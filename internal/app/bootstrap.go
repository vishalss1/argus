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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"


	"github.com/vishalss1/argus/internal/ai/actions"
	"github.com/vishalss1/argus/internal/ai/memory"
	"github.com/vishalss1/argus/internal/ai/query"
	"github.com/vishalss1/argus/internal/config"
	commanddomain "github.com/vishalss1/argus/internal/domain/command"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	devicedomain "github.com/vishalss1/argus/internal/domain/device"

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

	var finalTelemetryRepo telemetrydomain.Repository = &noopTelemetryRepository{}
	if kafkaProducer != nil {
		finalTelemetryRepo = kafka.NewTelemetryRepository(kafkaProducer)
	}
	telemetryService := telemetrydomain.NewService(finalTelemetryRepo)
	telemetryService.SetEventPublisher(realtime)

	redisTelemetryRepo := redis.NewTelemetryRepository(redisClient, 300*time.Second)

	telemetryHandler := transporthandler.NewTelemetryHandler(telemetryService, redisTelemetryRepo)

	shadowRepository := redis.NewShadowRepository(redisClient)
	shadowService := shadowdomain.NewService(shadowRepository)
	shadowHandler := transporthandler.NewShadowHandler(shadowService)

	embeddingProvider := embedding.NewOllamaProvider(cfg.OllamaBaseURL, cfg.OllamaEmbedModel)
	vectorStore := postgres.NewVectorStore(database)

	eventRepository := postgres.NewEventRepository(database)
	contextRepository := postgres.NewContextRepository(database)
	contextService := ctxdomain.NewService(contextRepository)
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

	aiProvider := ai.NewGroqProvider(cfg.GroqAPIKey, cfg.GroqModel, cfg.GroqBaseURL)
	queryEngine := query.NewEngine(embeddingProvider, aiProvider, vectorStore, eventRepository, contextRepository)

	aiHandler := transporthandler.NewAIHandler(eventRepository, contextService, queryEngine, actionEngine, policyService, redisClient)

	workspaceRepository := postgres.NewWorkspaceRepository(database)
	workspaceService := workspace.NewService(workspaceRepository, redisClient)
	workspaceHandler := transporthandler.NewWorkspaceHandler(workspaceService)

	usageRepository := postgres.NewUsageRepository(database)
	usageService := usage.NewService(usageRepository)

	sessionRepository := postgres.NewSessionRepository(database)
	sessionService := session.NewService(sessionRepository)
	sessionManager := session.NewManager(sessionService, usageService, redisClient, workspaceRepository)
	sessionHandler := transporthandler.NewSessionHandler(sessionService, sessionManager)

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
	router := transportrouter.New(deviceHandler, telemetryHandler, shadowHandler, commandHandler, otaHandler, ruleHandler, aiHandler, workspaceHandler, sessionHandler, websocketHandler)

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

	// Seed Redis device-to-workspace cache before starting consumers
	if err := seedDeviceWorkspaceCache(appCtx, redisClient, database); err != nil {
		log.Printf("Warning: failed to seed device workspace cache: %v", err)
	}

	if len(cfg.KafkaBrokers) > 0 {
		if hasProfile("telemetry") {
			go func() {
				defer server.wg.Done()
				startTelemetryLiveConsumer(appCtx, cfg, redisTelemetryRepo, redisClient, kafkaProducer)
			}()
		}
		if hasProfile("alerts") {
			go func() {
				defer server.wg.Done()
				startAlertConsumer(appCtx, cfg, ruleRepository, redisClient, kafkaProducer)
			}()
		}
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

func startTelemetryLiveConsumer(ctx context.Context, cfg *config.Config, telemetryRepo *redis.TelemetryRepository, redisClient *redis.Client, kafkaProducer *kafka.Producer) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTelemetryTopic,
		GroupID: "argus-telemetry-live-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[LIVE CONSUMER] started, consuming topic: %s", cfg.KafkaTelemetryTopic)



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





func startAlertConsumer(ctx context.Context, cfg *config.Config, ruleRepo *postgres.RuleRepository, redisClient *redis.Client, kafkaProducer *kafka.Producer) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   "alerts.generated",
		GroupID: "argus-alert-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[ALERT CONSUMER] started, consuming topic: alerts.generated")

	// Configuration with env overrides
	batchSize := 500
	if bsStr := os.Getenv("ALERT_BATCH_SIZE"); bsStr != "" {
		if bs, err := strconv.Atoi(bsStr); err == nil && bs > 0 {
			batchSize = bs
		}
	}

	flushInterval := 100 * time.Millisecond
	if fiStr := os.Getenv("ALERT_FLUSH_INTERVAL_MS"); fiStr != "" {
		if fi, err := strconv.Atoi(fiStr); err == nil && fi > 0 {
			flushInterval = time.Duration(fi) * time.Millisecond
		}
	}

	log.Printf("[ALERT CONSUMER] batch settings: maxBatchSize=%d, flushInterval=%v", batchSize, flushInterval)

	kafkaMsgChan := make(chan segmentio.Message, batchSize*2)

	// Fetcher Goroutine
	go func() {
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
				select {
				case kafkaMsgChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	var batchAlerts []ruledomain.Alert
	var batchMessages []segmentio.Message

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batchAlerts) == 0 {
			return
		}

		err := withRetry(3, func() error {
			return ruleRepo.CreateAlertsBatch(ctx, batchAlerts)
		})

		if err != nil {
			log.Printf("[ALERT CONSUMER] permanent error persisting alert batch (size %d): %v", len(batchAlerts), err)
			if kafkaProducer != nil {
				for idx := range batchAlerts {
					msg := batchMessages[idx]
					_ = kafkaProducer.PublishDLQ(ctx, "alerts.generated", msg.Key, msg.Value, err.Error())
				}
			}
		} else {
			// Set cooldowns in Redis only AFTER successful database persistence
			pipe := redisClient.Client().Pipeline()
			for _, alert := range batchAlerts {
				cooldownKey := fmt.Sprintf("alert:cooldown:%s:%s", alert.RuleID, alert.DeviceID)
				cooldownTTL := time.Duration(cfg.AlertCooldownSeconds) * time.Second
				pipe.Set(ctx, cooldownKey, "1", cooldownTTL)
			}
			if _, err := pipe.Exec(ctx); err != nil {
				log.Printf("[ALERT CONSUMER] redis error setting cooldowns: %v", err)
			}
		}

		// Commit all messages in the batch
		if err := consumer.CommitMessages(ctx, batchMessages...); err != nil {
			log.Printf("[ALERT CONSUMER] failed to commit messages: %v", err)
		}

		// Reset slices
		batchAlerts = batchAlerts[:0]
		batchMessages = batchMessages[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case msg := <-kafkaMsgChan:
			var a ruledomain.Alert
			if err := json.Unmarshal(msg.Value, &a); err != nil {
				log.Printf("[ALERT CONSUMER] decode error: %v", err)
				consumer.CommitMessages(ctx, msg)
				continue
			}

			// 1. In-memory deduplication within the current active batch
			duplicateInBatch := false
			for _, batched := range batchAlerts {
				if batched.RuleID == a.RuleID && batched.DeviceID == a.DeviceID {
					duplicateInBatch = true
					break
				}
			}
			if duplicateInBatch {
				log.Printf("[ALERT CONSUMER] dropping duplicate alert (in batch) for rule %s, device %s", a.RuleID, a.DeviceID)
				consumer.CommitMessages(ctx, msg)
				continue
			}

			// 2. Redis-backed check for cooldown (Exists check, set occurs post-write)
			cooldownKey := fmt.Sprintf("alert:cooldown:%s:%s", a.RuleID, a.DeviceID)
			exists, err := redisClient.Client().Exists(ctx, cooldownKey).Result()
			if err != nil {
				log.Printf("[ALERT CONSUMER] redis error checking cooldown: %v", err)
				exists = 0 // Fallback to write anyway if Redis has issues
			}

			if exists > 0 {
				log.Printf("[ALERT CONSUMER] dropping duplicate alert (in cooldown) for rule %s, device %s", a.RuleID, a.DeviceID)
				consumer.CommitMessages(ctx, msg)
				continue
			}

			batchAlerts = append(batchAlerts, a)
			batchMessages = append(batchMessages, msg)

			if len(batchAlerts) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
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

func seedDeviceWorkspaceCache(ctx context.Context, redisClient *redis.Client, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "SELECT id, workspace_id FROM devices WHERE workspace_id IS NOT NULL")
	if err != nil {
		return err
	}
	defer rows.Close()

	pipe := redisClient.Client().Pipeline()
	var count int
	for rows.Next() {
		var deviceID, workspaceID string
		if err := rows.Scan(&deviceID, &workspaceID); err != nil {
			return err
		}
		wsKey := fmt.Sprintf("device:%s:workspace", deviceID)
		pipe.Set(ctx, wsKey, workspaceID, 24*time.Hour)
		count++
	}

	if count > 0 {
		_, err = pipe.Exec(ctx)
		if err != nil {
			return err
		}
	}
	log.Printf("[CACHE SEED] successfully seeded %d device-to-workspace mapping(s) into Redis", count)
	return nil
}

type noopTelemetryRepository struct{}

func (noopTelemetryRepository) Create(ctx context.Context, t telemetrydomain.Telemetry) (*telemetrydomain.Telemetry, error) {
	return &t, nil
}
