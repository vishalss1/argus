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
	sessionManager := session.NewManager(sessionService, usageService, redisClient)
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
				startTelemetryLiveConsumer(appCtx, cfg, redisTelemetryRepo, redisClient, kafkaProducer)
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

func startTelemetryLiveConsumer(ctx context.Context, cfg *config.Config, telemetryRepo *redis.TelemetryRepository, redisClient *redis.Client, kafkaProducer *kafka.Producer) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTelemetryTopic,
		GroupID: "argus-telemetry-live-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[LIVE CONSUMER] started, consuming topic: %s", cfg.KafkaTelemetryTopic)

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
				log.Printf("[LIVE CONSUMER] fetch error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			var t telemetrydomain.Telemetry
			if err := json.Unmarshal(msg.Value, &t); err != nil {
				log.Printf("[LIVE CONSUMER] decode error: %v", err)
				consumer.CommitMessages(ctx, msg) // Drop invalid messages
				continue
			}

			err = withRetry(3, func() error {
				return telemetryRepo.SetLatest(ctx, t.DeviceID, t)
			})

			if err != nil {
				log.Printf("[LIVE CONSUMER] permanent error for device %s: %v", t.DeviceID, err)
				if kafkaProducer != nil {
					_ = kafkaProducer.PublishDLQ(ctx, cfg.KafkaTelemetryTopic, msg.Key, msg.Value, err.Error())
				}
			}

			consumer.CommitMessages(ctx, msg)
		}
	}
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
