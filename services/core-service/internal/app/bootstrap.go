package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	segmentio "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/vishalss1/argus/shared/common"

	"github.com/vishalss1/argus/core/internal/config"
	"github.com/vishalss1/argus/core/internal/domain/auth"
	"github.com/vishalss1/argus/core/internal/domain/certificate"
	commanddomain "github.com/vishalss1/argus/core/internal/domain/command"
	devicedomain "github.com/vishalss1/argus/core/internal/domain/device"
	otadomain "github.com/vishalss1/argus/core/internal/domain/ota"
	policydomain "github.com/vishalss1/argus/core/internal/domain/policy"
	sessiondomain "github.com/vishalss1/argus/core/internal/domain/session"
	shadowdomain "github.com/vishalss1/argus/core/internal/domain/shadow"
	telemetrydomain "github.com/vishalss1/argus/core/internal/domain/telemetry"
	workspacedomain "github.com/vishalss1/argus/core/internal/domain/workspace"
	"github.com/vishalss1/argus/core/internal/firmware"
	telemetrygrpc "github.com/vishalss1/argus/core/internal/infrastructure/grpc"
	"github.com/vishalss1/argus/core/internal/infrastructure/kafka"
	"github.com/vishalss1/argus/core/internal/infrastructure/minio"
	"github.com/vishalss1/argus/core/internal/infrastructure/mqtt"
	"github.com/vishalss1/argus/core/internal/infrastructure/postgres"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
	transporthandler "github.com/vishalss1/argus/core/internal/transport/http/handler"
	transportrouter "github.com/vishalss1/argus/core/internal/transport/http/router"
	transportws "github.com/vishalss1/argus/core/internal/transport/websocket"
	coregrpc "github.com/vishalss1/argus/core/internal/transport/grpc"
	pb "github.com/vishalss1/argus/shared/proto/core"
	telemetrypb "github.com/vishalss1/argus/shared/proto/telemetry"
)

type Server struct {
	db              *sql.DB
	kafkaProducer   *kafka.Producer
	mqttClient      *mqtt.Client
	httpServer      *http.Server
	grpcServer      *grpc.Server
	websocketHub    *transportws.Hub
	telemetryClient *telemetrygrpc.TelemetryClient
	tlsCertFile     string
	tlsKeyFile      string
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

func Bootstrap() (*Server, error) {
	cfg := config.Load()
	appCtx, cancel := context.WithCancel(context.Background())

	server := &Server{
		cancel: cancel,
	}

	database, err := postgres.InitDB(cfg.DatabaseURL)
	if err != nil {
		cancel()
		return nil, err
	}
	server.db = database

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
		server.kafkaProducer = kafkaProducer
	}

	// Initialize Telemetry gRPC Client
	telemetrySvcAddr := osGetEnv("TELEMETRY_SERVICE_GRPC_ADDR", "telemetry-service:50052")
	telemetryClient, err := telemetrygrpc.NewTelemetryClient(telemetrySvcAddr)
	if err != nil {
		log.Printf("[BOOTSTRAP] Warning: Failed to connect to Telemetry gRPC Service: %v. Running in disconnected mode.", err)
	} else {
		server.telemetryClient = telemetryClient
	}

	deviceRepository := postgres.NewDeviceRepository(database)
	websocketHub := transportws.NewHub(redisClient)
	server.websocketHub = websocketHub

	// Setup simple realtime publisher
	realtime := &realtimePublisherWrapper{hub: websocketHub}

	// Initialize Certificate Authority (uses device CA for issuing client certs)
	ca, err := certificate.NewCertificateAuthority(cfg.DeviceCARootCertFile, cfg.DeviceCAPrivateKeyFile)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize Certificate Authority: %w", err)
	}

	// Read device CA — used for server's client cert verification pool
	deviceCABytes, err := os.ReadFile(cfg.DeviceCARootCertFile)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to read device CA cert file: %w", err)
	}

	// Read server CA — used for firmware's ARGUS_ROOT_CA (server cert chain verification)
	serverCABytes, err := os.ReadFile(cfg.ServerCACertFile)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to read server CA cert file: %w", err)
	}

	// Verify that the server CA actually issues the server certificate
	serverCertBytes, err := os.ReadFile(cfg.HTTPSTLSCertFile)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to read server TLS cert file for validation: %w", err)
	}
	if err := firmware.ValidateCAIssuesServerCert(string(serverCABytes), string(serverCertBytes)); err != nil {
		cancel()
		return nil, err
	}

	httpPort, err := strconv.Atoi(cfg.Port)
	if err != nil {
		httpPort = 8080
	}

	mqttPort := 1883
	if cfg.MQTTBrokerURL != "" {
		if u, err := url.Parse(cfg.MQTTBrokerURL); err == nil {
			if p := u.Port(); p != "" {
				if parsed, err := strconv.Atoi(p); err == nil {
					mqttPort = parsed
				}
			}
		}
	}

	fwGen, err := firmware.NewGenerator(firmware.GeneratorConfig{
		ServerHost:             cfg.ServerHost,
		HTTPPort:               httpPort,
		MQTTPort:               mqttPort,
		RootCAPEM:              string(serverCABytes),
		OTASigningKeyID:        cfg.OTASigningKeyID,
		OTASigningPublicKeyB64: cfg.OTASigningPublicKeyB64,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize Firmware Generator: %w", err)
	}

	deviceService := devicedomain.NewService(deviceRepository)
	deviceService.SetEventPublisher(realtime)
	presenceService := devicedomain.NewPresenceService(deviceService)
	deviceHandler := transporthandler.NewDeviceHandler(deviceService, ca, fwGen)

	// Ingress Telemetry publishes directly to Kafka telemetry.raw
	var finalTelemetryRepo telemetrydomain.Repository = &kafka.NoopTelemetryRepository{}
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

	userRepo := postgres.NewUserRepository(database)
	tokenRepo := postgres.NewRefreshTokenRepository(database)
	auditRepo := postgres.NewAuditLogRepository(database)
	authService := auth.NewService(userRepo, tokenRepo, auditRepo, cfg.JWTSecret, cfg.JWTAccessExpiration, cfg.JWTRefreshExpiration)
	authHandler := transporthandler.NewAuthHandler(authService, redisClient)

	// Periodic Auth Token Cleanup Job (every 24 hours)
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-appCtx.Done():
				return
			case <-ticker.C:
				bgCtx, cancel := context.WithTimeout(appCtx, 10*time.Minute)
				_, _ = tokenRepo.DeleteExpiredOrRevoked(bgCtx)
				cancel()
			}
		}
	}()

	commandRepository := postgres.NewCommandRepository(database)
	var finalCommandRepo commanddomain.Repository = commandRepository
	if kafkaProducer != nil {
		finalCommandRepo = kafka.NewCommandRepository(commandRepository, kafkaProducer)
	}
	commandService := commanddomain.NewService(finalCommandRepo)
	commandService.OnResult = func(ctx context.Context, cmd commanddomain.Command) {
		websocketHub.Broadcast("command_update", cmd)
	}
	commandHandler := transporthandler.NewCommandHandler(commandService)

	otaRepository := postgres.NewOTARepository(database)
	otaService := otadomain.NewService(otaRepository, minioClient)
	otaService.MinioPublicURL = cfg.MinIOPublicURL
	firmwareSigner, err := otadomain.NewFirmwareSigner(otadomain.SigningConfig{
		RequireSignatures: cfg.OTARequireSignatures,
		KeyID:             cfg.OTASigningKeyID,
		PrivateKeyB64:     cfg.OTASigningPrivateKey,
	})
	if err == nil {
		otaService.SetFirmwareSigner(firmwareSigner)
	}
	otaService.SetEventPublisher(realtime)
	otaHandler := transporthandler.NewOTAHandler(otaService)

	policyRepository := postgres.NewPolicyRepository(database)
	policyService := policydomain.NewService(policyRepository)

	workspaceRepository := postgres.NewWorkspaceRepository(database)
	workspaceService := workspacedomain.NewService(workspaceRepository, redisClient)
	workspaceHandler := transporthandler.NewWorkspaceHandler(workspaceService, userRepo)

	sessionRepository := postgres.NewSessionRepository(database)
	sessionService := sessiondomain.NewService(sessionRepository)

	var telemetryGRPC telemetrypb.TelemetryIntelligenceServiceClient
	if telemetryClient != nil {
		telemetryGRPC = telemetryClient.Client()
	}
	sessionManager := sessiondomain.NewManager(sessionService, redisClient, workspaceRepository, telemetryGRPC, minioClient)
	sessionHandler := transporthandler.NewSessionHandler(sessionService, sessionManager)

	// Recover any RUNNING sessions to Redis hot-state
	_ = sessionManager.RecoverActiveSessions(appCtx)

	// Start telemetry export cleanup (every 12 hours)
	sessionManager.StartTelemetryExportCleaner(appCtx, cfg.TelemetryExportRetentionDays, 12*time.Hour)

	// Configure REST handlers to proxy rules/alerts/AI queries via gRPC client
	ruleHandler := transporthandler.NewRuleHandler(telemetryClient)
	aiHandler := transporthandler.NewAIHandler(telemetryClient, redisClient, cfg.AIQueryAPIKey, cfg.AIQueryRateLimit)

	var mqttClient *mqtt.Client
	if cfg.MQTTBrokerURL != "" {
		mqttClient, err = mqtt.New(mqtt.Config{
			BrokerURL:      cfg.MQTTBrokerURL,
			ClientID:       cfg.MQTTClientID,
			TelemetryTopic: cfg.MQTTTelemetryTopic,
			StateTopic:     cfg.MQTTStateTopic,
		}, telemetryService, presenceService, commandService, otaService)
		if err == nil {
			_ = mqttClient.Start()
			server.mqttClient = mqttClient
		}
	}

	websocketHandler := transportws.NewHandler(websocketHub, authService)
	router := transportrouter.New(
		deviceRepository,
		deviceHandler,
		telemetryHandler,
		shadowHandler,
		commandHandler,
		otaHandler,
		ruleHandler,
		aiHandler,
		workspaceHandler,
		sessionHandler,
		authHandler,
		authService,
		websocketHandler,
	)

	if kafkaProducer != nil && mqttClient != nil {
		go startCommandDispatcher(appCtx, cfg, mqttClient)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(deviceCABytes)

	server.httpServer = &http.Server{
		Addr: ":" + cfg.Port,
		Handler: router,
		TLSConfig: &tls.Config{
			MinVersion:       tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{tls.CurveP256, tls.X25519},
			ClientAuth:       tls.VerifyClientCertIfGiven,
			ClientCAs:        caCertPool,
		},
	}
	server.tlsCertFile = cfg.HTTPSTLSCertFile
	server.tlsKeyFile = cfg.HTTPSTLSKeyFile

	// Configure gRPC Server
	grpcPort := osGetEnv("GRPC_PORT", "50051")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		return nil, fmt.Errorf("grpc failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(correlationServerUnaryInterceptor),
	)
	coreGRPCServer := coregrpc.NewServer(deviceRepository, workspaceRepository, sessionService, sessionManager, commandService, policyService, redisClient)
	pb.RegisterCoreServiceServer(grpcServer, coreGRPCServer)

	// Register standard gRPC Health check server
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	server.grpcServer = grpcServer

	server.wg.Add(6)

	// WebSocket Hub
	go func() {
		defer server.wg.Done()
		websocketHub.Run(appCtx)
	}()

	// Heartbeat Presence Fallback Monitor
	go func() {
		defer server.wg.Done()
		monitorPresence(appCtx, presenceService, cfg.HeartbeatTimeout, cfg.HeartbeatInterval, redisClient)
	}()

	// OTA timeout monitor
	go func() {
		defer server.wg.Done()
		monitorOTATimeouts(appCtx, otaService)
	}()

	// Session Reaper
	go func() {
		defer server.wg.Done()
		startSessionReaper(appCtx, sessionManager, cfg.SessionStaleTimeoutHours, redisClient)
	}()

	// gRPC Server Runner
	go func() {
		defer server.wg.Done()
		log.Printf("[gRPC] Core Service gRPC Server listening on port %s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("[gRPC] Server error: %v", err)
		}
	}()

	// seed Redis device workspace mapping cache
	_ = seedDeviceWorkspaceCache(appCtx, redisClient, database)

	return server, nil
}

func (s *Server) Start() error {
	log.Printf("[SERVER] HTTP API listening on %s", s.httpServer.Addr)
	if s.tlsCertFile != "" && s.tlsKeyFile != "" {
		if err := s.httpServer.ListenAndServeTLS(s.tlsCertFile, s.tlsKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Close() error {
	s.cancel()
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	s.wg.Wait()

	if s.telemetryClient != nil {
		s.telemetryClient.Close()
	}
	if s.mqttClient != nil {
		s.mqttClient.Close()
	}
	if s.kafkaProducer != nil {
		s.kafkaProducer.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// Stubs & Wrappers for compilation

type realtimePublisherWrapper struct {
	hub *transportws.Hub
}

func (p *realtimePublisherWrapper) PublishDeviceUpdate(ctx context.Context, entity devicedomain.Device) {
	p.hub.Broadcast("device_update", entity)
}

func (p *realtimePublisherWrapper) PublishDevicePresence(ctx context.Context, event devicedomain.PresenceEvent) {
	p.hub.BroadcastPayload(event)
	// Integration event published asynchronously to Kafka telemetry.incidents on disconnection
	// Telemetry Service Incident Consumer will pick this up and store as historical event.
	// This maintains clean database segregation!
}

func (p *realtimePublisherWrapper) PublishTelemetry(ctx context.Context, entity telemetrydomain.Telemetry) {
	p.hub.Broadcast("telemetry", entity)
}

func (p *realtimePublisherWrapper) PublishCommandUpdate(ctx context.Context, entity commanddomain.Command) {
	p.hub.Broadcast("command_update", entity)
}

func (p *realtimePublisherWrapper) PublishOTAEvent(ctx context.Context, eventType string, deployment otadomain.Deployment) {
	p.hub.Broadcast(eventType, deployment)
}



func extractCorrelationID(headers []segmentio.Header) string {
	for _, h := range headers {
		if h.Key == "correlation_id" {
			return string(h.Value)
		}
	}
	return ""
}

func startCommandDispatcher(ctx context.Context, cfg *config.Config, mqttClient *mqtt.Client) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaCommandTopic,
		GroupID: "argus-command-dispatcher",
	})
	defer consumer.Close()

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
				continue
			}


			workerCtx := ctx
			if corrID := extractCorrelationID(msg.Headers); corrID != "" {
				workerCtx = common.WithCorrelationID(ctx, corrID)
			}

			var cmd commanddomain.Command
			if err := json.Unmarshal(msg.Value, &cmd); err == nil {
				topic := fmt.Sprintf("argus/devices/%s/commands", cmd.DeviceID)
				_ = mqttClient.Publish(topic, 1, false, map[string]interface{}{
					"id":      cmd.ID,
					"type":    cmd.Type,
					"payload": cmd.Payload,
					"sent_at": cmd.SentAt,
				})
			}
			consumer.CommitMessages(workerCtx, msg)
		}
	}
}

type TokenLock struct {
	client *redis.Client
	key    string
	token  string
	ttl    time.Duration
}

func NewTokenLock(client *redis.Client, key string, ttl time.Duration) *TokenLock {
	return &TokenLock{
		client: client,
		key:    key,
		token:  uuid.New().String(),
		ttl:    ttl,
	}
}

func (l *TokenLock) Acquire(ctx context.Context) (bool, error) {
	res, err := l.client.Client().SetNX(ctx, l.key, l.token, l.ttl).Result()
	if err != nil {
		common.RedisLockContention.WithLabelValues(l.key).Inc()
		return false, err
	}
	if !res {
		common.RedisLockContention.WithLabelValues(l.key).Inc()
	}
	return res, nil
}

func (l *TokenLock) Release(ctx context.Context) error {
	luaScript := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	_, err := l.client.Client().Eval(ctx, luaScript, []string{l.key}, l.token).Result()
	return err
}

func (l *TokenLock) Renew(ctx context.Context) (bool, error) {
	luaScript := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	res, err := l.client.Client().Eval(ctx, luaScript, []string{l.key}, l.token, int64(l.ttl/time.Millisecond)).Result()
	if err != nil {
		return false, err
	}
	return res.(int64) == 1, nil
}

func monitorPresence(ctx context.Context, s *devicedomain.PresenceService, timeout time.Duration, interval time.Duration, redisClient *redis.Client) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lockTTL := interval - (5 * time.Second)
			if lockTTL < 5*time.Second {
				lockTTL = interval
			}
			
			lock := NewTokenLock(redisClient, "lock:presence_monitor", lockTTL)
			acquired, err := lock.Acquire(ctx)
			if err != nil || !acquired {
				continue
			}
			
			jobCtx, cancelJob := context.WithCancel(ctx)
			watchdogDone := make(chan struct{})
			
			// Start watchdog
			go func() {
				defer close(watchdogDone)
				watchdogTicker := time.NewTicker(lockTTL / 3)
				defer watchdogTicker.Stop()
				for {
					select {
					case <-jobCtx.Done():
						return
					case <-watchdogTicker.C:
						renewed, err := lock.Renew(ctx)
						if err != nil || !renewed {
							log.Printf("[Lock Watchdog] presence_monitor lock renewal failed: %v", err)
							cancelJob()
							return
						}
					}
				}
			}()
			
			// Run job
			_, _ = s.MarkStaleOffline(jobCtx, timeout)
			
			// Stop watchdog
			cancelJob()
			<-watchdogDone
			
			// Release lock
			_ = lock.Release(ctx)
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
			_, _ = service.MarkTimedOut(ctx)
		}
	}
}

func startSessionReaper(ctx context.Context, manager *sessiondomain.Manager, timeoutHours int, redisClient *redis.Client) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	timeout := time.Duration(timeoutHours) * time.Hour
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lock := NewTokenLock(redisClient, "lock:session_reaper", 10*time.Minute)
			acquired, err := lock.Acquire(ctx)
			if err != nil || !acquired {
				continue
			}
			
			_, _ = manager.CleanupStaleSessions(ctx, timeout)
			
			_ = lock.Release(ctx)
		}
	}
}

func seedDeviceWorkspaceCache(ctx context.Context, r *redis.Client, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "SELECT id, workspace_id FROM devices WHERE workspace_id IS NOT NULL")
	if err != nil {
		return err
	}
	defer rows.Close()
	pipe := r.Client().Pipeline()
	for rows.Next() {
		var id, wsID string
		if err := rows.Scan(&id, &wsID); err == nil {
			pipe.Set(ctx, fmt.Sprintf("device:%s:workspace", id), wsID, 24*time.Hour)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

func osGetEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func correlationServerUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("x-correlation-id"); len(vals) > 0 {
			ctx = common.WithCorrelationID(ctx, vals[0])
		}
	}
	return handler(ctx, req)
}
