package app

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/ai/actions"
	"github.com/vishalss1/argus/internal/ai/memory"
	"github.com/vishalss1/argus/internal/ai/query"
	"github.com/vishalss1/argus/internal/config"
	commanddomain "github.com/vishalss1/argus/internal/domain/command"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	devicedomain "github.com/vishalss1/argus/internal/domain/device"
	incidentdomain "github.com/vishalss1/argus/internal/domain/incident"
	otadomain "github.com/vishalss1/argus/internal/domain/ota"
	policydomain "github.com/vishalss1/argus/internal/domain/policy"
	ruledomain "github.com/vishalss1/argus/internal/domain/rule"
	shadowdomain "github.com/vishalss1/argus/internal/domain/shadow"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/domain/usage"
	"github.com/vishalss1/argus/internal/domain/workspace"
	"github.com/vishalss1/argus/internal/domain/session"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
	"github.com/vishalss1/argus/internal/infrastructure/embedding"
	"github.com/vishalss1/argus/internal/infrastructure/kafka"
	segmentio "github.com/segmentio/kafka-go"
	"github.com/vishalss1/argus/internal/infrastructure/minio"
	"github.com/vishalss1/argus/internal/infrastructure/mqtt"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
	goredis "github.com/redis/go-redis/v9"
	transporthandler "github.com/vishalss1/argus/internal/transport/http/handler"
	transportrouter "github.com/vishalss1/argus/internal/transport/http/router"
	transportws "github.com/vishalss1/argus/internal/transport/websocket"
)

type Server struct {
	db            *sql.DB
	kafkaProducer *kafka.Producer
	mqttClient    *mqtt.Client
	httpServer    *http.Server
	websocketHub  *transportws.Hub
	tlsCertFile   string
	tlsKeyFile    string
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	runAPI        bool
}

type redisAlertLimiter struct {
	client *redis.Client
	ttl    time.Duration
}

func (l *redisAlertLimiter) Allow(ctx context.Context, ruleID string, deviceID string) bool {
	key := fmt.Sprintf("alert:cooldown:%s:%s", ruleID, deviceID)
	return l.client.Client().SetNX(ctx, key, "1", l.ttl).Val()
}

func Bootstrap() (*Server, error) {
	cfg := config.Load()
	appCtx, cancel := context.WithCancel(context.Background())

	database, err := postgres.InitDB(cfg.DatabaseURL)
	if err != nil {
		cancel()
		return nil, err
	}

	redisClient, err := redis.New(appCtx, redis.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	minioClient, err := minio.New(appCtx, minio.Config{
		Endpoint:        cfg.MinIOEndpoint,
		PublicURL:       cfg.MinIOPublicURL,
		AccessKeyID:     cfg.MinIOAccessKey,
		SecretAccessKey: cfg.MinIOSecretKey,
		Bucket:          cfg.MinIOBucket,
		UseSSL:          cfg.MinIOUseSSL,
	})
	if err != nil {
		cancel()
		return nil, err
	}

	var kafkaProducer *kafka.Producer
	if len(cfg.KafkaBrokers) > 0 {
		kafkaProducer, err = kafka.NewProducer(kafka.Config{
			Brokers:        cfg.KafkaBrokers,
			TelemetryTopic: cfg.KafkaTelemetryTopic,
			CommandTopic:   cfg.KafkaCommandTopic,
		})
		if err != nil {
			cancel()
			return nil, err
		}
	}

	websocketHub := transportws.NewHub()
	realtime := &realtimePublisher{hub: websocketHub}

	deviceRepository := postgres.NewDeviceRepository(database)
	deviceService := devicedomain.NewService(deviceRepository)
	deviceService.SetEventPublisher(realtime)
	presenceService := devicedomain.NewPresenceService(deviceService)
	deviceHandler := transporthandler.NewDeviceHandler(deviceService)

	telemetryRepository := postgres.NewTelemetryRepository(database)
	var finalTelemetryRepo telemetrydomain.Repository = telemetryRepository
	if kafkaProducer != nil {
		finalTelemetryRepo = kafka.NewTelemetryRepository(telemetryRepository, kafkaProducer)
	}
	telemetryService := telemetrydomain.NewService(finalTelemetryRepo)
	telemetryService.SetEventPublisher(realtime)

	redisTelemetryRepo := redis.NewTelemetryRepository(redisClient, 300*time.Second)

	exportDir := "./data/exports"
	exportService, err := kafka.NewExportService(cfg.KafkaBrokers, cfg.KafkaTelemetryTopic, exportDir, "http://localhost:"+cfg.Port) // Simplified base URL
	if err != nil {
		log.Printf("Warning: failed to initialize export service: %v", err)
	}

	telemetryHandler := transporthandler.NewTelemetryHandler(telemetryService, exportService, redisTelemetryRepo, exportDir)

	shadowRepository := redis.NewShadowRepository(redisClient)
	shadowService := shadowdomain.NewService(shadowRepository)
	shadowHandler := transporthandler.NewShadowHandler(shadowService)

	embeddingProvider := embedding.NewOllamaProvider(cfg.OllamaBaseURL, cfg.OllamaEmbedModel)
	vectorStore := postgres.NewVectorStore(database)
	memoryEmbedding := memory.NewEmbeddingService(embeddingProvider, vectorStore)

	eventRepository := postgres.NewEventRepository(database)
	contextRepository := postgres.NewContextRepository(database)
	contextService := ctxdomain.NewService(contextRepository)
	contextService.OnRecord = func(ctx context.Context, mem ctxdomain.OperationalMemory) {
		if err := memoryEmbedding.EmbedMemory(ctx, mem); err != nil {
			log.Printf("failed to embed operational memory: %v", err)
		}
	}
	memoryManager := memory.NewManager(contextService)

	commandRepository := postgres.NewCommandRepository(database)
	var finalCommandRepo commanddomain.Repository = commandRepository
	if kafkaProducer != nil {
		finalCommandRepo = kafka.NewCommandRepository(commandRepository, kafkaProducer)
	}
	commandService := commanddomain.NewService(finalCommandRepo)
	commandService.OnResult = func(ctx context.Context, cmd commanddomain.Command) {
		realtime.PublishCommandUpdate(ctx, cmd)
		if err := memoryManager.SummarizeCommand(ctx, cmd); err != nil {
			log.Printf("failed to summarize command: %v", err)
		}
	}
	commandHandler := transporthandler.NewCommandHandler(commandService)

	otaRepository := postgres.NewOTARepository(database)
	otaService := otadomain.NewService(otaRepository, minioClient)
	firmwareSigner, err := otadomain.NewFirmwareSigner(otadomain.SigningConfig{
		RequireSignatures: cfg.OTARequireSignatures,
		KeyID:             cfg.OTASigningKeyID,
		PrivateKeyB64:     cfg.OTASigningPrivateKey,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	otaService.SetFirmwareSigner(firmwareSigner)
	otaService.SetEventPublisher(realtime)
	otaService.OnResult = func(ctx context.Context, dep otadomain.Deployment) {
		if err := memoryManager.SummarizeDeployment(ctx, dep); err != nil {
			log.Printf("failed to summarize deployment: %v", err)
		}
	}
	otaHandler := transporthandler.NewOTAHandler(otaService)

	ruleRepository := postgres.NewRuleRepository(database)
	ruleService := ruledomain.NewService(ruleRepository)
	if kafkaProducer != nil {
		ruleService.SetPublisher(kafkaProducer)
	}
	ruleService.SetLimiter(&redisAlertLimiter{
		client: redisClient,
		ttl:    time.Duration(cfg.AlertCooldownSeconds) * time.Second,
	})
	ruleHandler := transporthandler.NewRuleHandler(ruleService)

	policyRepository := postgres.NewPolicyRepository(database)
	policyService := policydomain.NewService(policyRepository)
	actionEngine := actions.NewEngine(commandService, policyService)

	incidentRepository := postgres.NewIncidentRepository(database)
	incidentService := incidentdomain.NewService(incidentRepository)
	incidentService.OnResolved = func(ctx context.Context, inc incidentdomain.Incident) {
		if err := memoryManager.SummarizeIncident(ctx, inc); err != nil {
			log.Printf("failed to summarize incident: %v", err)
		}
	}

	aiProvider := ai.NewGroqProvider(cfg.GroqAPIKey, cfg.GroqModel, cfg.GroqBaseURL)
	queryEngine := query.NewEngine(embeddingProvider, aiProvider, vectorStore, eventRepository, incidentRepository, contextRepository)

	aiHandler := transporthandler.NewAIHandler(eventRepository, incidentService, contextService, queryEngine, actionEngine, policyService)

	findingRepository := postgres.NewFindingRepository(database)
	findingHandler := transporthandler.NewFindingHandler(findingRepository)

	fleetRepository := postgres.NewFleetRepository(database)
	fleetHandler := transporthandler.NewFleetHandler(fleetRepository)

	workspaceRepository := postgres.NewWorkspaceRepository(database)
	workspaceService := workspace.NewService(workspaceRepository)
	workspaceHandler := transporthandler.NewWorkspaceHandler(workspaceService)

	usageRepository := postgres.NewUsageRepository(database)
	usageService := usage.NewService(usageRepository)

	sessionRepository := postgres.NewSessionRepository(database)
	sessionService := session.NewService(sessionRepository)
	sessionManager := session.NewManager(sessionService, usageService, redisClient, workspaceRepository, findingRepository)
	sessionHandler := transporthandler.NewSessionHandler(sessionService, sessionManager, exportDir)

	// Recover any RUNNING sessions to Redis hot-state
	if err := sessionManager.RecoverActiveSessions(appCtx); err != nil {
		log.Printf("Warning: failed to recover active sessions: %v", err)
	}

	var mqttClient *mqtt.Client
	if cfg.MQTTBrokerURL != "" {
		mqttClient, err = mqtt.New(mqtt.Config{
			BrokerURL:      cfg.MQTTBrokerURL,
			ClientID:       cfg.MQTTClientID,
			TelemetryTopic: cfg.MQTTTelemetryTopic,
			StateTopic:     cfg.MQTTStateTopic,
		}, telemetryService, presenceService, commandService, otaService)
		if err != nil {
			cancel()
			return nil, err
		}

		if err := mqttClient.Start(); err != nil {
			log.Printf("failed to start mqtt client: %v", err)
		}
	}

	websocketHandler := transportws.NewHandler(websocketHub)
	router := transportrouter.New(deviceHandler, telemetryHandler, shadowHandler, commandHandler, otaHandler, ruleHandler, aiHandler, findingHandler, fleetHandler, workspaceHandler, sessionHandler, websocketHandler)

	if kafkaProducer != nil && mqttClient != nil {
		go startCommandDispatcher(appCtx, cfg, mqttClient)
	}

	profiles := strings.Split(cfg.WorkerProfiles, ",")
	hasProfile := func(p string) bool {
		for _, profile := range profiles {
			trimmed := strings.TrimSpace(profile)
			if trimmed == "all" || trimmed == p {
				return true
			}
		}
		return false
	}

	server := &Server{
		db:            database,
		kafkaProducer: kafkaProducer,
		mqttClient:    mqttClient,
		httpServer: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				// Disable Post-Quantum Key Exchange (ML-KEM) for better IoT compatibility
				CurvePreferences: []tls.CurveID{tls.CurveP256, tls.X25519},
			},
		},
		websocketHub: websocketHub,
		tlsCertFile:  cfg.HTTPSTLSCertFile,
		tlsKeyFile:   cfg.HTTPSTLSKeyFile,
		cancel:       cancel,
		runAPI:       hasProfile("api"),
	}

	wgCount := 0
	if hasProfile("api") {
		wgCount += 4 // websocketHub, monitorPresence, monitorOTATimeouts, startSessionReaper
	}
	if hasProfile("telemetry") && len(cfg.KafkaBrokers) > 0 {
		wgCount++
	}
	if hasProfile("summary") {
		wgCount++
	}
	if hasProfile("alerts") && len(cfg.KafkaBrokers) > 0 {
		wgCount++
	}

	server.wg.Add(wgCount)

	if hasProfile("api") {
		go func() {
			defer server.wg.Done()
			websocketHub.Run(appCtx)
		}()

		go func() {
			defer server.wg.Done()
			monitorPresence(appCtx, presenceService, cfg.HeartbeatTimeout, cfg.HeartbeatInterval)
		}()

		go func() {
			defer server.wg.Done()
			monitorOTATimeouts(appCtx, otaService)
		}()

		go func() {
			defer server.wg.Done()
			startSessionReaper(appCtx, sessionManager, cfg.SessionStaleTimeoutHours)
		}()
	}

	if len(cfg.KafkaBrokers) > 0 {
		if hasProfile("telemetry") {
			go func() {
				defer server.wg.Done()
				startTelemetryLiveConsumer(appCtx, cfg, redisTelemetryRepo, redisClient, kafkaProducer, deviceService)
			}()
		}
		if hasProfile("alerts") {
			go func() {
				defer server.wg.Done()
				startAlertConsumer(appCtx, cfg, ruleRepository, kafkaProducer)
			}()
		}
	}
	
	if hasProfile("summary") {
		go func() {
			defer server.wg.Done()
			startFleetSummaryService(appCtx, fleetRepository, findingRepository, redisTelemetryRepo, redisClient)
		}()
	}

	_ = actionEngine

	return server, nil
}

func startSessionReaper(ctx context.Context, manager *session.Manager, timeoutHours int) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	timeout := time.Duration(timeoutHours) * time.Hour

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := manager.CleanupStaleSessions(ctx, timeout)
			if err != nil {
				if !errors.Is(ctx.Err(), context.Canceled) {
					log.Printf("[SESSION REAPER] failed: %v", err)
				}
				continue
			}
			if count > 0 {
				log.Printf("[SESSION REAPER] marked %d stale session(s) as FAILED", count)
			}
		}
	}
}

func monitorOTATimeouts(ctx context.Context, service *otadomain.Service) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deployments, err := service.MarkTimedOut(ctx)
			if err != nil {
				if !errors.Is(ctx.Err(), context.Canceled) {
					log.Printf("ota timeout monitor failed: %v", err)
				}
				continue
			}
			if len(deployments) > 0 {
				log.Printf("ota timeout monitor marked %d deployment(s) timed out", len(deployments))
			}
		}
	}
}

func (s *Server) Start() error {
	if !s.runAPI {
		log.Printf("[SERVER] HTTP API disabled via profiles. Background workers running...")
		// Block until context cancellation (handled by Close or signals in main)
		// We'll just hang here instead of ListenAndServe
		select {}
	}

	tlsEnabled := s.tlsCertFile != "" && s.tlsKeyFile != ""

	log.Printf("[SERVER] starting server")
	log.Printf("[SERVER] listening address: %s", s.httpServer.Addr)
	log.Printf("[SERVER] TLS enabled: %v", tlsEnabled)

	if tlsEnabled {
		log.Printf("[SERVER] TLS cert file: %s", s.tlsCertFile)
		log.Printf("[SERVER] TLS key file: %s", s.tlsKeyFile)
		if err := s.httpServer.ListenAndServeTLS(s.tlsCertFile, s.tlsKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	if s.tlsCertFile != "" || s.tlsKeyFile != "" {
		return errors.New("both HTTPS_TLS_CERT_FILE and HTTPS_TLS_KEY_FILE must be set to enable native TLS")
	}

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Close() error {
	s.cancel()
	s.wg.Wait()

	if s.mqttClient != nil {
		s.mqttClient.Close()
	}
	if s.kafkaProducer != nil {
		s.kafkaProducer.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
	return s.httpServer.Close()
}

func startCommandDispatcher(ctx context.Context, cfg *config.Config, mqttClient *mqtt.Client) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaCommandTopic,
		GroupID: "argus-command-dispatcher",
	})
	defer consumer.Close()

	log.Printf("[DISPATCHER] starting command bridge: Kafka(%s) -> MQTT", cfg.KafkaCommandTopic)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[DISPATCHER] fetch error: %v", err)
				continue
			}

			var cmd commanddomain.Command
			if err := json.Unmarshal(msg.Value, &cmd); err != nil {
				log.Printf("[DISPATCHER] decode error: %v", err)
				consumer.CommitMessages(ctx, msg)
				continue
			}

			// Format MQTT topic: argus/devices/{id}/commands
			topic := fmt.Sprintf("argus/devices/%s/commands", cmd.DeviceID)

			err = mqttClient.Publish(topic, 1, false, map[string]interface{}{
				"id":      cmd.ID,
				"type":    cmd.Type,
				"payload": cmd.Payload,
				"sent_at": cmd.SentAt,
			})

			if err != nil {
				log.Printf("[DISPATCHER] publish error: %v", err)
			}

			consumer.CommitMessages(ctx, msg)
		}
	}
}

func monitorPresence(ctx context.Context, s *devicedomain.PresenceService, timeout time.Duration, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			devices, err := s.MarkStaleOffline(ctx, timeout)
			if err != nil {
				if !errors.Is(ctx.Err(), context.Canceled) {
					log.Printf("heartbeat monitor failed: %v", err)
				}
				continue
			}

			if len(devices) > 0 {
				log.Printf("heartbeat fallback monitor marked %d device(s) offline", len(devices))
			}
		}
	}
}

func startTelemetryLiveConsumer(ctx context.Context, cfg *config.Config, telemetryRepo *redis.TelemetryRepository, redisClient *redis.Client, kafkaProducer *kafka.Producer, deviceService *devicedomain.Service) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTelemetryTopic,
		GroupID: "argus-telemetry-live-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[LIVE CONSUMER] started, consuming topic: %s", cfg.KafkaTelemetryTopic)

	// Pre-load Lua scripts to prevent NOSCRIPT errors inside pipelines
	if err := updateMinMaxScript.Load(ctx, redisClient.Client()).Err(); err != nil {
		log.Printf("[LIVE CONSUMER] failed to load updateMinMaxScript: %v", err)
	}
	if err := updateRollupMinMaxScript.Load(ctx, redisClient.Client()).Err(); err != nil {
		log.Printf("[LIVE CONSUMER] failed to load updateRollupMinMaxScript: %v", err)
	}
	if err := updateGPSDistanceScript.Load(ctx, redisClient.Client()).Err(); err != nil {
		log.Printf("[LIVE CONSUMER] failed to load updateGPSDistanceScript: %v", err)
	}

	// In-memory cache for device workspace and active session
	type cacheEntry struct {
		val       string
		expiresAt time.Time
	}
	var (
		cacheMu    sync.RWMutex
		localCache = make(map[string]cacheEntry)
	)

	getLocalCache := func(key string) (string, bool) {
		cacheMu.RLock()
		defer cacheMu.RUnlock()
		entry, ok := localCache[key]
		if !ok || time.Now().After(entry.expiresAt) {
			return "", false
		}
		return entry.val, true
	}

	setLocalCache := func(key string, val string, ttl time.Duration) {
		cacheMu.Lock()
		defer cacheMu.Unlock()
		localCache[key] = cacheEntry{
			val:       val,
			expiresAt: time.Now().Add(ttl),
		}
	}

	var batch []segmentio.Message
	const maxBatchSize = 1000
	const maxBatchWait = 1 * time.Second

	commitTimer := time.NewTimer(maxBatchWait)
	defer commitTimer.Stop()

	flushBatch := func(flushCtx context.Context) {
		defer func() {
			if !commitTimer.Stop() {
				select {
				case <-commitTimer.C:
				default:
				}
			}
			commitTimer.Reset(maxBatchWait)
		}()

		if len(batch) == 0 {
			return
		}
		start := time.Now()
		err := consumer.CommitMessages(flushCtx, batch...)
		duration := time.Since(start).Seconds()
		session.TelemetryConsumerCommitDurationSeconds.Observe(duration)
		if err != nil {
			log.Printf("[LIVE CONSUMER] failed to commit batch of %d messages: %v", len(batch), err)
		} else {
			session.TelemetryConsumerBatchCommitsTotal.Inc()
			log.Printf("[LIVE CONSUMER] successfully committed batch of %d messages in %.4fs", len(batch), duration)
		}
		batch = batch[:0]
	}

	var redisBatch []segmentio.Message
	pipe := redisClient.Client().Pipeline()

	flushRedisBatch := func(flushCtx context.Context) {
		if len(redisBatch) == 0 {
			return
		}
		pipeStart := time.Now()
		cmds, execErr := pipe.Exec(flushCtx)
		pipeDuration := time.Since(pipeStart).Seconds()

		// Observability metrics
		session.TelemetryRedisPipelineDurationSeconds.Observe(pipeDuration)
		session.TelemetryPipelineMessagesTotal.Add(float64(len(redisBatch)))
		session.RedisPipelineBatchesTotal.Inc()
		session.RedisPipelineCommandsTotal.Add(float64(len(cmds)))

		if execErr != nil {
			log.Printf("[LIVE CONSUMER] error executing pipeline batch of %d: %v", len(redisBatch), execErr)
			session.TelemetryConsumerProcessingFailuresTotal.Add(float64(len(redisBatch)))
			if kafkaProducer != nil {
				for _, m := range redisBatch {
					_ = kafkaProducer.PublishDLQ(flushCtx, cfg.KafkaTelemetryTopic, m.Key, m.Value, execErr.Error())
				}
			}
		}

		batch = append(batch, redisBatch...)
		redisBatch = redisBatch[:0]

		if len(batch) >= maxBatchSize {
			flushBatch(flushCtx)
		}
	}

	msgChan := make(chan segmentio.Message, 2000)
	errChan := make(chan error, 1)

	go func() {
		for {
			msg, err := consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				errChan <- err
				time.Sleep(1 * time.Second)
				continue
			}
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[LIVE CONSUMER] context cancelled, flushing pending offsets...")
			flushRedisBatch(context.Background())
			flushBatch(context.Background())
			return
		case err := <-errChan:
			log.Printf("[LIVE CONSUMER] fetch error: %v", err)
		case msg := <-msgChan:
			var t telemetrydomain.Telemetry
			if err := json.Unmarshal(msg.Value, &t); err != nil {
				log.Printf("[LIVE CONSUMER] decode error: %v", err)
				session.TelemetryConsumerDroppedMessagesTotal.Inc()
				// Drop invalid messages
				_ = consumer.CommitMessages(ctx, msg)
				continue
			}

			session.TelemetryConsumerMessagesTotal.Inc()

			// Determine device workspace and active session
			var workspaceID string
			var sessionID string
			wsKey := fmt.Sprintf("device:%s:workspace", t.DeviceID)

			if cachedWS, ok := getLocalCache(wsKey); ok {
				workspaceID = cachedWS
			} else {
				cachedWS, err := redisClient.Client().Get(ctx, wsKey).Result()
				if err == nil && cachedWS != "" {
					workspaceID = cachedWS
					setLocalCache(wsKey, workspaceID, 10*time.Second)
				} else {
					dev, err := deviceService.GetByID(ctx, t.DeviceID)
					if err == nil && dev != nil && dev.WorkspaceID != nil {
						workspaceID = *dev.WorkspaceID
						_ = redisClient.Client().Set(ctx, wsKey, workspaceID, 24*time.Hour).Err()
						setLocalCache(wsKey, workspaceID, 10*time.Second)
					}
				}
			}

			if workspaceID != "" {
				sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
				if sID, ok := getLocalCache(sessionKey); ok {
					sessionID = sID
				} else {
					sID, err := redisClient.Client().Get(ctx, sessionKey).Result()
					if err == nil && sID != "" {
						sessionID = sID
						setLocalCache(sessionKey, sessionID, 2*time.Second)
					}
				}
			}

			if sessionID != "" {
				// 1. Set latest telemetry in Redis using pipeline
				if err := telemetryRepo.SetLatestPipeline(ctx, pipe, t.DeviceID, t); err != nil {
					log.Printf("[LIVE CONSUMER] failed to pipeline SetLatest: %v", err)
				}

				// 2. Accumulate metrics in the same Redis pipeline
				accumulateSessionMetrics(ctx, pipe, sessionID, t.DeviceID, t)

				redisBatch = append(redisBatch, msg)
				if len(redisBatch) >= 100 {
					flushRedisBatch(ctx)
				}
			} else {
				batch = append(batch, msg)
				if len(batch) >= maxBatchSize {
					flushBatch(ctx)
				}
			}
		case <-commitTimer.C:
			flushRedisBatch(ctx)
			flushBatch(ctx)
		}
	}
}

func toFloat(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

var updateMinMaxScript = goredis.NewScript(`
	local val = tonumber(ARGV[1])
	local minKey = KEYS[1]
	local maxKey = KEYS[2]

	-- update min
	local curMin = redis.call('get', minKey)
	if not curMin or val < tonumber(curMin) then
		redis.call('set', minKey, val)
	end

	-- update max
	local curMax = redis.call('get', maxKey)
	if not curMax or val > tonumber(curMax) then
		redis.call('set', maxKey, val)
	end
	return 1
`)

var updateRollupMinMaxScript = goredis.NewScript(`
	local hashKey = KEYS[1]
	local minField = ARGV[1]
	local maxField = ARGV[2]
	local val = tonumber(ARGV[3])

	-- update min
	local curMin = redis.call('hget', hashKey, minField)
	if not curMin or curMin == "" or val < tonumber(curMin) then
		redis.call('hset', hashKey, minField, val)
	end

	-- update max
	local curMax = redis.call('hget', hashKey, maxField)
	if not curMax or curMax == "" or val > tonumber(curMax) then
		redis.call('hset', hashKey, maxField, val)
	end
	return 1
`)

var updateGPSDistanceScript = goredis.NewScript(`
	local gpsKey = KEYS[1]
	local sessionDistKey = KEYS[2]
	local deviceDistKey = KEYS[3]
	local latVal = tonumber(ARGV[1])
	local lonVal = tonumber(ARGV[2])

	local prevLat = nil
	local prevLon = nil
	
	local latStr = redis.call('hget', gpsKey, 'lat')
	local lonStr = redis.call('hget', gpsKey, 'lon')
	if latStr and lonStr then
		prevLat = tonumber(latStr)
		prevLon = tonumber(lonStr)
	end

	local dist = 0.0
	if prevLat and prevLon then
		local lat1 = prevLat * 3.141592653589793 / 180.0
		local lon1 = prevLon * 3.141592653589793 / 180.0
		local lat2 = latVal * 3.141592653589793 / 180.0
		local lon2 = lonVal * 3.141592653589793 / 180.0
		
		local dLat = lat2 - lat1
		local dLon = lon2 - lon1
		
		local a = math.sin(dLat/2) * math.sin(dLat/2) + math.cos(lat1) * math.cos(lat2) * math.sin(dLon/2) * math.sin(dLon/2)
		local c = 2 * math.atan2(math.sqrt(a), math.sqrt(1-a))
		dist = 6371.0 * c
		
		redis.call('incrbyfloat', sessionDistKey, dist)
		redis.call('incrbyfloat', deviceDistKey, dist)
	end

	redis.call('hset', gpsKey, 'lat', ARGV[1], 'lon', ARGV[2])
	return dist
`)

func accumulateSessionMetrics(ctx context.Context, pipe goredis.Pipeliner, sessionID, deviceID string, t telemetrydomain.Telemetry) {
	var metrics map[string]interface{}
	if err := json.Unmarshal(t.Metrics, &metrics); err != nil {
		return
	}

	// Track sample count (session-wide)
	countKey := fmt.Sprintf("session:%s:metrics:count", sessionID)
	pipe.Incr(ctx, countKey)

	// Track sample count (per-device)
	devSampleKey := fmt.Sprintf("session:%s:device:%s:sample_count", sessionID, deviceID)
	pipe.Incr(ctx, devSampleKey)

	// Track device participation
	devsKey := fmt.Sprintf("session:%s:devices", sessionID)
	pipe.SAdd(ctx, devsKey, deviceID)

	// Track device first/last seen
	firstSeenKey := fmt.Sprintf("session:%s:device:%s:first_seen", sessionID, deviceID)
	pipe.SetNX(ctx, firstSeenKey, time.Now().Unix(), 0)
	lastSeenKey := fmt.Sprintf("session:%s:device:%s:last_seen", sessionID, deviceID)
	pipe.Set(ctx, lastSeenKey, time.Now().Unix(), 0)

	// Rollup minute key
	minuteStr := t.RecordedAt.Truncate(time.Minute).Format(time.RFC3339)
	rollupMinutesKey := fmt.Sprintf("session:%s:device:%s:rollup_minutes", sessionID, deviceID)
	pipe.SAdd(ctx, rollupMinutesKey, minuteStr)
	rollupKey := fmt.Sprintf("session:%s:device:%s:rollup:%s", sessionID, deviceID, minuteStr)
	pipe.HIncrBy(ctx, rollupKey, "sample_count", 1)

	// Update battery stats
	var batteryVal float64
	hasBattery := false
	if v, ok := metrics["battery_level"]; ok {
		if f, ok := toFloat(v); ok {
			batteryVal = f
			hasBattery = true
		}
	} else if v, ok := metrics["battery"]; ok {
		if f, ok := toFloat(v); ok {
			batteryVal = f
			hasBattery = true
		}
	}
	if hasBattery {
		// Session aggregates
		pipe.IncrByFloat(ctx, fmt.Sprintf("session:%s:metrics:battery:sum", sessionID), batteryVal)
		pipe.Incr(ctx, fmt.Sprintf("session:%s:metrics:battery:count", sessionID))
		updateMinMaxScript.Run(ctx, pipe, []string{
			fmt.Sprintf("session:%s:metrics:battery:min", sessionID),
			fmt.Sprintf("session:%s:metrics:battery:max", sessionID),
		}, batteryVal)

		// Per-device aggregates
		pipe.IncrByFloat(ctx, fmt.Sprintf("session:%s:device:%s:battery:sum", sessionID, deviceID), batteryVal)
		pipe.Incr(ctx, fmt.Sprintf("session:%s:device:%s:battery:count", sessionID, deviceID))
		updateMinMaxScript.Run(ctx, pipe, []string{
			fmt.Sprintf("session:%s:device:%s:battery:min", sessionID, deviceID),
			fmt.Sprintf("session:%s:device:%s:battery:max", sessionID, deviceID),
		}, batteryVal)

		// Telemetry Rollup
		pipe.HIncrByFloat(ctx, rollupKey, "battery:sum", batteryVal)
		pipe.HIncrBy(ctx, rollupKey, "battery:count", 1)
		updateRollupMinMaxScript.Run(ctx, pipe, []string{rollupKey}, "battery:min", "battery:max", batteryVal)
	}

	// Update temperature stats
	var tempVal float64
	hasTemp := false
	if v, ok := metrics["temp_c"]; ok {
		if f, ok := toFloat(v); ok {
			tempVal = f
			hasTemp = true
		}
	} else if v, ok := metrics["temperature"]; ok {
		if f, ok := toFloat(v); ok {
			tempVal = f
			hasTemp = true
		}
	} else if v, ok := metrics["temp"]; ok {
		if f, ok := toFloat(v); ok {
			tempVal = f
			hasTemp = true
		}
	}
	if hasTemp {
		// Session aggregates
		pipe.IncrByFloat(ctx, fmt.Sprintf("session:%s:metrics:temp:sum", sessionID), tempVal)
		pipe.Incr(ctx, fmt.Sprintf("session:%s:metrics:temp:count", sessionID))
		updateMinMaxScript.Run(ctx, pipe, []string{
			fmt.Sprintf("session:%s:metrics:temp:min", sessionID),
			fmt.Sprintf("session:%s:metrics:temp:max", sessionID),
		}, tempVal)

		// Per-device aggregates
		pipe.IncrByFloat(ctx, fmt.Sprintf("session:%s:device:%s:temp:sum", sessionID, deviceID), tempVal)
		pipe.Incr(ctx, fmt.Sprintf("session:%s:device:%s:temp:count", sessionID, deviceID))
		updateMinMaxScript.Run(ctx, pipe, []string{
			fmt.Sprintf("session:%s:device:%s:temp:min", sessionID, deviceID),
			fmt.Sprintf("session:%s:device:%s:temp:max", sessionID, deviceID),
		}, tempVal)

		// Telemetry Rollup
		pipe.HIncrByFloat(ctx, rollupKey, "temp:sum", tempVal)
		pipe.HIncrBy(ctx, rollupKey, "temp:count", 1)
		updateRollupMinMaxScript.Run(ctx, pipe, []string{rollupKey}, "temp:min", "temp:max", tempVal)
	}

	// Update signal stats (RSSI / signal strength)
	var rssiVal float64
	hasRSSI := false
	if v, ok := metrics["rssi_dbm"]; ok {
		if f, ok := toFloat(v); ok {
			rssiVal = f
			hasRSSI = true
		}
	} else if v, ok := metrics["rssi"]; ok {
		if f, ok := toFloat(v); ok {
			rssiVal = f
			hasRSSI = true
		}
	}
	if hasRSSI {
		// Per-device aggregates
		pipe.IncrByFloat(ctx, fmt.Sprintf("session:%s:device:%s:signal:sum", sessionID, deviceID), rssiVal)
		pipe.Incr(ctx, fmt.Sprintf("session:%s:device:%s:signal:count", sessionID, deviceID))
		updateMinMaxScript.Run(ctx, pipe, []string{
			fmt.Sprintf("session:%s:device:%s:signal:min", sessionID, deviceID),
			fmt.Sprintf("session:%s:device:%s:signal:max", sessionID, deviceID),
		}, rssiVal)

		// Telemetry Rollup
		pipe.HIncrByFloat(ctx, rollupKey, "signal:sum", rssiVal)
		pipe.HIncrBy(ctx, rollupKey, "signal:count", 1)
		updateRollupMinMaxScript.Run(ctx, pipe, []string{rollupKey}, "signal:min", "signal:max", rssiVal)
	}

	// Update uptime metrics
	var uptimeVal float64
	hasUptime := false
	if v, ok := metrics["uptime_s"]; ok {
		if f, ok := toFloat(v); ok {
			uptimeVal = f
			hasUptime = true
		}
	} else if v, ok := metrics["uptime"]; ok {
		if f, ok := toFloat(v); ok {
			uptimeVal = f
			hasUptime = true
		}
	}
	if hasUptime {
		updateMinMaxScript.Run(ctx, pipe, []string{
			fmt.Sprintf("session:%s:device:%s:uptime_s:min", sessionID, deviceID),
			fmt.Sprintf("session:%s:device:%s:uptime_s:max", sessionID, deviceID),
		}, uptimeVal)
	}

	// Update distance travelled (Haversine formula based on lat/lon via Lua)
	var latVal, lonVal float64
	hasGPS := false
	if vLat, okLat := metrics["latitude"]; okLat {
		if vLon, okLon := metrics["longitude"]; okLon {
			if fLat, ok1 := toFloat(vLat); ok1 {
				if fLon, ok2 := toFloat(vLon); ok2 {
					latVal = fLat
					lonVal = fLon
					hasGPS = true
				}
			}
		}
	} else if vLat, okLat := metrics["lat"]; okLat {
		if vLon, okLon := metrics["lon"]; okLon {
			if fLat, ok1 := toFloat(vLat); ok1 {
				if fLon, ok2 := toFloat(vLon); ok2 {
					latVal = fLat
					lonVal = fLon
					hasGPS = true
				}
			}
		} else if vLon, okLon := metrics["lng"]; okLon {
			if fLat, ok1 := toFloat(vLat); ok1 {
				if fLon, ok2 := toFloat(vLon); ok2 {
					latVal = fLat
					lonVal = fLon
					hasGPS = true
				}
			}
		}
	}

	if hasGPS {
		gpsKey := fmt.Sprintf("session:%s:device:%s:last_gps", sessionID, deviceID)
		sessionDistKey := fmt.Sprintf("session:%s:metrics:distance", sessionID)
		deviceDistKey := fmt.Sprintf("session:%s:device:%s:distance", sessionID, deviceID)
		updateGPSDistanceScript.Run(ctx, pipe, []string{gpsKey, sessionDistKey, deviceDistKey}, latVal, lonVal)
	}
}

func updateRollupMinMax(ctx context.Context, r *redis.Client, hashKey, minField, maxField string, val float64) {
	// Dummy function to keep any old references happy, though we don't call it.
}

type deviceState struct {
	LastSeen    time.Time
	HealthScore int
	RiskScore   float64
}

func startFleetSummaryService(ctx context.Context, fleetRepo *postgres.FleetRepository, findingRepo *postgres.FindingRepository, redisRepo *redis.TelemetryRepository, redisClient *redis.Client) {
	log.Printf("[FLEET SUMMARY] started, using Redis for aggregation")

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Enforce singleton execution via Redis Lock
			ok, err := redisClient.Client().SetNX(ctx, "lock:fleet-summary", "1", 4*time.Minute).Result()
			if err != nil || !ok {
				continue // Another instance is running the aggregation
			}

			telemetries, err := redisRepo.GetAllLatest(ctx)
			if err != nil {
				log.Printf("[FLEET SUMMARY] failed to get latest telemetries: %v", err)
				continue
			}

			states := make(map[string]deviceState)
			for _, t := range telemetries {
				findings, err := findingRepo.ListByDevice(ctx, t.DeviceID)
				var health int = 100
				var risk float64 = 0.0
				if err == nil && len(findings) > 0 {
					health = findings[0].HealthScore
					risk = findings[0].RiskScore
				}
				states[t.DeviceID] = deviceState{
					LastSeen:    time.Now(), // Assuming redis values are fresh within TTL
					HealthScore: health,
					RiskScore:   risk,
				}
			}

			summary := calculateSummary(states)
			if summary != nil {
				if _, err := fleetRepo.Create(ctx, *summary); err != nil {
					log.Printf("[FLEET SUMMARY] persist error: %v", err)
				} else {
					log.Printf("[FLEET SUMMARY] snapshot persisted: %d active, %d high-risk", summary.ActiveDevices, summary.HighRiskDevices)
				}
			}
		}
	}
}

func calculateSummary(states map[string]deviceState) *telemetrydomain.FleetSummary {
	if len(states) == 0 {
		return nil
	}

	active := 0
	offline := 0
	var totalHealth float64
	var totalRisk float64
	highRisk := 0

	now := time.Now()
	for _, s := range states {
		if now.Sub(s.LastSeen) < 10*time.Minute {
			active++
			totalHealth += float64(s.HealthScore)
			totalRisk += s.RiskScore
			if s.RiskScore > 0.7 {
				highRisk++
			}
		} else {
			offline++
		}
	}

	if active == 0 {
		return nil
	}

	return &telemetrydomain.FleetSummary{
		ID:              uuid.New().String(),
		ActiveDevices:   active,
		OfflineDevices:  offline,
		AvgHealthScore:  totalHealth / float64(active),
		AvgRiskScore:    totalRisk / float64(active),
		HighRiskDevices: highRisk,
		Metadata:        json.RawMessage("{}"),
		CreatedAt:       time.Now(),
	}
}

func startAlertConsumer(ctx context.Context, cfg *config.Config, ruleRepo *postgres.RuleRepository, kafkaProducer *kafka.Producer) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   "alerts.generated",
		GroupID: "argus-alert-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[ALERT CONSUMER] started, consuming topic: alerts.generated")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[ALERT CONSUMER] fetch error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			var a ruledomain.Alert
			if err := json.Unmarshal(msg.Value, &a); err != nil {
				log.Printf("[ALERT CONSUMER] decode error: %v", err)
				consumer.CommitMessages(ctx, msg)
				continue
			}

			err = withRetry(3, func() error {
				_, err := ruleRepo.CreateAlert(ctx, a)
				return err
			})

			if err != nil {
				log.Printf("[ALERT CONSUMER] permanent error persisting alert: %v", err)
				if kafkaProducer != nil {
					_ = kafkaProducer.PublishDLQ(ctx, "alerts.generated", msg.Key, msg.Value, err.Error())
				}
			}

			consumer.CommitMessages(ctx, msg)
		}
	}
}

func withRetry(maxRetries int, fn func() error) error {
	var err error
	delay := 100 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
		delay *= 2 // exponential backoff
	}
	return err
}
