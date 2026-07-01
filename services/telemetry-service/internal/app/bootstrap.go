package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	segmentio "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"

	"github.com/vishalss1/argus/shared/common"

	pb "github.com/vishalss1/argus/shared/proto/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/ai/analytics"
	"github.com/vishalss1/argus/telemetry/internal/ai/memory"
	"github.com/vishalss1/argus/telemetry/internal/ai/query"
	"github.com/vishalss1/argus/telemetry/internal/config"
	ctxdomain "github.com/vishalss1/argus/telemetry/internal/domain/context"
	devicedomain "github.com/vishalss1/argus/telemetry/internal/domain/device"
	eventdomain "github.com/vishalss1/argus/telemetry/internal/domain/event"
	ruledomain "github.com/vishalss1/argus/telemetry/internal/domain/rule"
	telemetrydomain "github.com/vishalss1/argus/telemetry/internal/domain/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/ai"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/embedding"
	grpcinfra "github.com/vishalss1/argus/telemetry/internal/infrastructure/grpc"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/minio"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/mqtt"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/kafka"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/postgres"
	redisinfra "github.com/vishalss1/argus/telemetry/internal/infrastructure/redis"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/telemetry"
	telemetrygrpc "github.com/vishalss1/argus/telemetry/internal/transport/grpc"
	"go.uber.org/zap"
)

// ponytail: minimal repository that just returns the entity, as we publish via Kafka
type noopTelemetryRepo struct{}

func (n noopTelemetryRepo) Create(ctx context.Context, t telemetrydomain.Telemetry) (*telemetrydomain.Telemetry, error) {
	return &t, nil
}

type Server struct {
	db          *sql.DB
	redisClient *redisinfra.Client
	grpcServer  *grpc.Server
	httpServer  *http.Server
	coreClient    *grpcinfra.CoreClient
	embedProvider *embedding.LocalProvider
	kafkaProducer *kafka.Producer
	cancel        context.CancelFunc
	wg          sync.WaitGroup
	mqttClient  *mqtt.Client
}

func Bootstrap() (*Server, error) {
	cfg := config.Load()
	appCtx, cancel := context.WithCancel(context.Background())

	logger, _ := zap.NewProduction()
	
	// Initialize Telemetry
	_, err := telemetry.Init("telemetry-service")
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init telemetry: %w", err)
	}

	server := &Server{
		cancel: cancel,
	}

	database, err := postgres.InitDB(cfg.DatabaseURL)
	if err != nil {
		cancel()
		return nil, err
	}
	server.db = database

	redisClient, err := redisinfra.New(appCtx, redisinfra.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	server.redisClient = redisClient

	// Initialize MinIO client for telemetry export archiving
	var minioClient *minio.Client
	if cfg.MinIOEndpoint != "" {
		mc, err := minio.NewClient(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
		if err != nil {
			log.Printf("[BOOTSTRAP] Warning: failed to connect to MinIO: %v. Hourly telemetry archiving disabled.", err)
		} else {
			minioClient = mc
		}
	}

	// Start Telemetry Compactor background task
	compactor := analytics.NewCompactor(redisClient.Client(), minioClient, 1*time.Minute)
	compactor.Start(appCtx)

	coreSvcAddr := osGetEnv("CORE_SERVICE_GRPC_ADDR", "core-service:50051")
	coreClient, err := grpcinfra.NewCoreClient(coreSvcAddr)
	if err != nil {
		log.Printf("[BOOTSTRAP] Warning: Failed to connect to Core gRPC Service: %v. Running in disconnected mode.", err)
	} else {
		server.coreClient = coreClient
	}

	deviceRepo := grpcinfra.NewDeviceRepository(coreClient, redisClient)
	embedProvider, err := embedding.NewLocalProvider(cfg.Embedding.ModelPath, cfg.Embedding.Dimension)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init local embedding provider: %w", err)
	}
	server.embedProvider = embedProvider

	vectorStore := postgres.NewVectorStore(database)
	eventRepo := postgres.NewEventRepository(database)
	contextRepo := postgres.NewContextRepository(database)
	contextService := ctxdomain.NewService(contextRepo, deviceRepo)

	embedSvc := memory.NewEmbeddingService(embedProvider, vectorStore, cfg.EmbeddingQueueSize)
	embedSvc.StartWorkers(appCtx, cfg.EmbeddingWorkers, &server.wg)

	aiProvider := ai.NewGroqProvider(cfg.GroqAPIKey, cfg.GroqModel, cfg.GroqBaseURL)

	ruleRepo := postgres.NewRuleRepository(database)
	ruleService := ruledomain.NewService(ruleRepo, deviceRepo)

	var kafkaProducer *kafka.Producer
	if len(cfg.KafkaBrokers) > 0 {
		kafkaProducer, err = kafka.NewProducer(kafka.Config{
			Brokers:        cfg.KafkaBrokers,
			TelemetryTopic: cfg.KafkaTelemetryTopic,
			CommandTopic:   cfg.KafkaCommandTopic,
			IncidentTopic:  cfg.KafkaIncidentTopic,
		})
		if err != nil {
			cancel()
			return nil, err
		}
		server.kafkaProducer = kafkaProducer
		ruleService.SetPublisher(kafkaProducer)
	}
	ruleService.SetLimiter(redisinfra.NewAlertLimiter(redisClient))

	telemetryService := telemetrydomain.NewService(noopTelemetryRepo{})

	if cfg.MQTTBrokerURL != "" {
		mqttClient, err := mqtt.New(mqtt.Config{
			BrokerURL:      cfg.MQTTBrokerURL,
			ClientID:       cfg.MQTTClientID,
			TelemetryTopic: cfg.MQTTTelemetryTopic,
		}, telemetryService, kafkaProducer, logger)
		if err != nil {
			log.Printf("[MQTT] Failed to create MQTT client: %v", err)
		} else {
			if err := mqttClient.Start(); err != nil {
				log.Printf("[MQTT] Failed to start MQTT client: %v", err)
			} else {
				server.mqttClient = mqttClient
				log.Printf("[MQTT] Telemetry ingestion started")
			}
		}
	}

	planner := query.NewPlanner()
	snapshotBuilder := query.NewSnapshotBuilder(deviceRepo, eventRepo, contextRepo, redisClient)
	
	// Temporarily construct engine early for handlers that still need it for helpers
	queryEngine := query.NewEngine(planner, nil, deviceRepo, redisClient, aiProvider, logger)
	
	fleetService := redisinfra.NewFleetService(redisClient, deviceRepo)
	fleetHandler := query.NewFleetSummaryHandler(fleetService, aiProvider, logger)
	deviceHandler := query.NewDeviceHealthHandler(snapshotBuilder, queryEngine)
	incidentHandler := query.NewIncidentHandler(snapshotBuilder, queryEngine)
	historicalHandler := query.NewHistoricalAnalysisHandler(embedProvider, vectorStore, eventRepo, contextRepo, aiProvider, logger, float32(cfg.Embedding.SimilarityLimit))

	router := query.NewRouter(fleetHandler, deviceHandler, incidentHandler, historicalHandler)
	if err := router.Validate(); err != nil {
		cancel()
		return nil, fmt.Errorf("invalid query router configuration: %w", err)
	}

	// Now that router is built, rebuild engine with it
	queryEngine = query.NewEngine(planner, router, deviceRepo, redisClient, aiProvider, logger)

	grpcPort := osGetEnv("GRPC_PORT", "50052")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("grpc failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(correlationServerUnaryInterceptor),
		grpc.MaxRecvMsgSize(128*1024*1024),
		grpc.MaxSendMsgSize(128*1024*1024),
	)
	deviceService := redisinfra.NewDeviceService(redisClient)
	telemetryGRPCServer := telemetrygrpc.NewServer(queryEngine, eventRepo, contextService, ruleService, redisClient, deviceService, fleetService)
	pb.RegisterTelemetryIntelligenceServiceServer(grpcServer, telemetryGRPCServer)

	// Register standard gRPC Health check server
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	server.grpcServer = grpcServer

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		log.Printf("[gRPC] Telemetry Service gRPC Server listening on port %s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("[gRPC] Server error: %v", err)
		}
	}()

	// Expose HTTP server for Health check and Metrics
	httpPort := osGetEnv("PORT", "8081")
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	server.httpServer = httpServer

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		log.Printf("[HTTP] Telemetry Service HTTP Server listening on port %s (health & metrics)", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[HTTP] Server error: %v", err)
		}
	}()

	if len(cfg.KafkaBrokers) > 0 {
		analyticsEngine := analytics.NewEngine(redisClient.Client(), kafkaProducer)
		server.wg.Add(1)
		go func() {
			defer server.wg.Done()
			startAIWorker(appCtx, cfg, analyticsEngine)
		}()

		server.wg.Add(1)
		go func() {
			defer server.wg.Done()
			startIncidentConsumer(appCtx, cfg, eventRepo, embedSvc, kafkaProducer, deviceRepo)
		}()

		server.wg.Add(1)
		go func() {
			defer server.wg.Done()
			startAlertConsumer(appCtx, cfg, ruleRepo, redisClient, kafkaProducer, deviceRepo)
		}()

		redisTelemetryRepo := redisinfra.NewTelemetryRepository(redisClient, 300*time.Second)
		server.wg.Add(1)
		go func() {
			defer server.wg.Done()
			startTelemetryLiveConsumer(appCtx, cfg, redisTelemetryRepo, redisClient, kafkaProducer)
		}()
	}

	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		startCorrelationEngine(appCtx, redisClient, eventRepo, embedSvc, deviceRepo)
	}()


	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-appCtx.Done():
				return
			case <-ticker.C:
				lock := NewTokenLock(redisClient, "lock:embedding_backfill", 10*time.Minute)
				acquired, err := lock.Acquire(appCtx)
				if err != nil || !acquired {
					continue
				}
				_ = embedSvc.Backfill(appCtx, contextRepo, eventRepo)
				_ = lock.Release(appCtx)
			}
		}
	}()

	return server, nil
}

func (s *Server) Start() error {
	log.Printf("[SERVER] Telemetry & Intelligence Service background processes active.")
	select {}
}

func (s *Server) Close() error {
	s.cancel()

	var httpErr error
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpErr = s.httpServer.Shutdown(ctx)
	}

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	s.wg.Wait()

	if s.embedProvider != nil {
		_ = s.embedProvider.Close()
	}
	if s.mqttClient != nil {
		s.mqttClient.Close()
	}
	if s.kafkaProducer != nil {
		s.kafkaProducer.Close()
	}
	if s.coreClient != nil {
		s.coreClient.Close()
	}
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.db != nil {
		s.db.Close()
	}
	return httpErr
}

func extractCorrelationID(headers []segmentio.Header) string {
	for _, h := range headers {
		if h.Key == "correlation_id" {
			return string(h.Value)
		}
	}
	return ""
}

func startAIWorker(ctx context.Context, cfg *config.Config, analyticsEngine *analytics.Engine) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTelemetryTopic,
		GroupID: cfg.KafkaAIWorkerGroupID,
	})
	defer consumer.Close()

	log.Printf("[AI WORKER] started, consuming topic: %s", cfg.KafkaTelemetryTopic)

	const numWorkers = 16
	msgChan := make(chan segmentio.Message, 2000)
	commitChan := make(chan segmentio.Message, 2000)
	errChan := make(chan error, 1)

	var workerWg sync.WaitGroup
	for w := 1; w <= numWorkers; w++ {
		workerWg.Add(1)
		go func(workerID int) {
			defer workerWg.Done()
			for msg := range msgChan {
				workerCtx := ctx
				if corrID := extractCorrelationID(msg.Headers); corrID != "" {
					workerCtx = common.WithCorrelationID(ctx, corrID)
				}

				var t telemetrydomain.Telemetry
				if err := json.Unmarshal(msg.Value, &t); err != nil {
					log.Printf("[AI WORKER-%d] decode error: %v", workerID, err)
					_ = consumer.CommitMessages(workerCtx, msg)
					continue
				}

				if err := analyticsEngine.Analyze(workerCtx, t); err != nil {
					log.Printf("[AI WORKER-%d] analytics error: %v", workerID, err)
				}

				select {
				case commitChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}(w)
	}

	go func() {
		for {
			msg, err := consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case errChan <- err:
				default:
				}
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

	var batch []segmentio.Message
	const maxBatchSize = 100
	commitInterval := 100 * time.Millisecond
	commitTimer := time.NewTicker(commitInterval)
	defer commitTimer.Stop()

	flushBatch := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := consumer.CommitMessages(flushCtx, batch...); err != nil {
			log.Printf("[AI WORKER] failed to commit batch of %d messages: %v", len(batch), err)
		}
		batch = batch[:0]
	}

	var msgCount int64

	for {
		select {
		case <-ctx.Done():
			log.Println("[AI WORKER] context cancelled, flushing pending offsets...")
			close(msgChan)
			workerWg.Wait()
			flushBatch(context.Background())
			return
		case err := <-errChan:
			log.Printf("[AI WORKER] fetch error: %v", err)
		case msg := <-commitChan:
			msgCount++
			if msgCount%500 == 0 {
				log.Printf("[AI WORKER] processed %d messages, pending commit: %d", msgCount, len(batch))
			}
			batch = append(batch, msg)
			if len(batch) >= maxBatchSize {
				flushBatch(ctx)
			}
		case <-commitTimer.C:
			flushBatch(ctx)
		}
	}
}

const (
	telemetryLiveMsgChanSize     = 2000
	telemetryLiveCommitBatchSize = 100
	telemetryLiveCommitInterval  = 1 * time.Second
	telemetryLiveRedisBatchSize  = 100
	telemetryLiveWorkspaceTTL    = 60 * time.Second
	telemetryLiveSessionTTL      = 30 * time.Second
	telemetryLiveWorkers         = 32
)

func startTelemetryLiveConsumer(ctx context.Context, cfg *config.Config, telemetryRepo *redisinfra.TelemetryRepository, redisClient *redisinfra.Client, kafkaProducer *kafka.Producer) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTelemetryTopic,
		GroupID: "argus-telemetry-live-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[LIVE CONSUMER] started, consuming topic: %s", cfg.KafkaTelemetryTopic)

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

	msgChan := make(chan segmentio.Message, telemetryLiveMsgChanSize)
	commitChan := make(chan segmentio.Message, telemetryLiveMsgChanSize)
	errChan := make(chan error, 1)

	type pipelineItem struct {
		msg       segmentio.Message
		deviceID  string
		sessionID string
		telemetry telemetrydomain.Telemetry
	}
	pipelineChan := make(chan pipelineItem, telemetryLiveMsgChanSize)

	pipelineDone := make(chan struct{})
	go func() {
		defer close(pipelineDone)
		pipe := redisClient.Client().Pipeline()
		var redisBatch []segmentio.Message

		flushPipeline := func(flushCtx context.Context) {
			if len(redisBatch) == 0 {
				return
			}
			
			cmds, execErr := pipe.Exec(flushCtx)
			if execErr != nil {
				log.Printf("[LIVE CONSUMER] error executing pipeline batch of %d: %v", len(redisBatch), execErr)
				if kafkaProducer != nil {
					for _, m := range redisBatch {
						_ = kafkaProducer.PublishDLQ(flushCtx, cfg.KafkaTelemetryTopic, m.Key, m.Value, execErr.Error())
					}
				}
			}

			_ = cmds

			for _, m := range redisBatch {
				select {
				case commitChan <- m:
				case <-flushCtx.Done():
					return
				}
			}
			redisBatch = redisBatch[:0]
		}

		pipelineFlushTicker := time.NewTicker(500 * time.Millisecond)
		defer pipelineFlushTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				flushPipeline(context.Background())
				return
			case item := <-pipelineChan:
				payload, _ := json.Marshal(item.telemetry)
				if err := telemetryRepo.SessionTrackPipeline(ctx, pipe, item.sessionID, item.deviceID, time.Now().Unix(), payload); err != nil {
					log.Printf("[LIVE CONSUMER] failed to pipeline SessionTrack: %v", err)
				}
				redisBatch = append(redisBatch, item.msg)
				if len(redisBatch) >= telemetryLiveRedisBatchSize {
					flushPipeline(ctx)
				}
			case <-pipelineFlushTicker.C:
				flushPipeline(ctx)
			}
		}
	}()

	go func() {
		for {
			fetchStart := time.Now()
			msg, err := consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case errChan <- err:
				default:
				}
				time.Sleep(1 * time.Second)
				continue
			}
			TelemetryStageFetchDurationSeconds.Observe(time.Since(fetchStart).Seconds())
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	commitWorkerDone := make(chan struct{})
	go func() {
		defer close(commitWorkerDone)
		var commitBatch []segmentio.Message
		commitTicker := time.NewTicker(telemetryLiveCommitInterval)
		defer commitTicker.Stop()

		flushCommit := func(flushCtx context.Context) {
			if len(commitBatch) == 0 {
				return
			}
			commitStart := time.Now()
			err := consumer.CommitMessages(flushCtx, commitBatch...)
			TelemetryConsumerCommitDurationSeconds.Observe(time.Since(commitStart).Seconds())
			if err != nil {
				log.Printf("[LIVE CONSUMER] failed to commit batch of %d messages: %v", len(commitBatch), err)
			}
			commitBatch = commitBatch[:0]
		}

		for {
			select {
			case <-ctx.Done():
				flushCommit(context.Background())
				return
			case msg := <-commitChan:
				commitBatch = append(commitBatch, msg)
				if len(commitBatch) >= telemetryLiveCommitBatchSize {
					flushCommit(ctx)
				}
			case <-commitTicker.C:
				flushCommit(ctx)
			}
		}
	}()

	var workerWg sync.WaitGroup
	for w := 0; w < telemetryLiveWorkers; w++ {
		workerWg.Add(1)
		go func(workerID int) {
			defer workerWg.Done()
			for msg := range msgChan {
				msgStart := time.Now()
				workerCtx := ctx
				if corrID := extractCorrelationID(msg.Headers); corrID != "" {
					workerCtx = common.WithCorrelationID(ctx, corrID)
				}

				var t telemetrydomain.Telemetry
				if err := json.Unmarshal(msg.Value, &t); err != nil {
					log.Printf("[LIVE CONSUMER] decode error: %v", err)
					TelemetryConsumerDroppedMessagesTotal.Inc()
					select {
					case commitChan <- msg:
					case <-ctx.Done():
						return
					}
					continue
				}

				TelemetryConsumerMessagesTotal.Inc()

				if t.ID != "" {
					dedupKey := fmt.Sprintf("dedup:telemetry:%s", t.ID)
					res, err := redisClient.Client().SetNX(workerCtx, dedupKey, "1", cfg.TelemetryDedupTTL).Result()
					if err == nil && !res {
						log.Printf("[LIVE CONSUMER] duplicate telemetry message skipped: ID=%s, DeviceID=%s", t.ID, t.DeviceID)
						TelemetryConsumerDuplicateMessagesTotal.Inc()
						select {
						case commitChan <- msg:
						case <-ctx.Done():
							return
						}
						continue
					}
				}

				var workspaceID string
				var sessionID string
				wsKey := fmt.Sprintf("device:%s:workspace", t.DeviceID)

				if cachedWS, ok := getLocalCache(wsKey); ok {
					workspaceID = cachedWS
				} else {
					cachedWS, err := redisClient.Client().Get(workerCtx, wsKey).Result()
					if err == nil && cachedWS != "" {
						workspaceID = cachedWS
						setLocalCache(wsKey, workspaceID, telemetryLiveWorkspaceTTL)
					}
				}

				if workspaceID != "" {
					sessionKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
					if sID, ok := getLocalCache(sessionKey); ok {
						sessionID = sID
					} else {
						sID, err := redisClient.Client().Get(workerCtx, sessionKey).Result()
						if err == nil && sID != "" {
							sessionID = sID
							setLocalCache(sessionKey, sessionID, telemetryLiveSessionTTL)
						}
					}
				}

				if sessionID != "" {
					item := pipelineItem{
						msg:       msg,
						deviceID:  t.DeviceID,
						sessionID: sessionID,
						telemetry: t,
					}
					select {
					case pipelineChan <- item:
					case <-ctx.Done():
						return
					}
				} else {
					select {
					case commitChan <- msg:
					case <-ctx.Done():
						return
					}
				}

				TelemetryConsumerMessageProcessingDuration.Observe(time.Since(msgStart).Seconds())
			}
		}(w)
	}

	<-ctx.Done()
	close(msgChan)
	workerWg.Wait()
	close(pipelineChan)
	<-pipelineDone
	close(commitChan)
	<-commitWorkerDone
}

func startAlertConsumer(ctx context.Context, cfg *config.Config, ruleRepo *postgres.RuleRepository, redisClient *redisinfra.Client, kafkaProducer *kafka.Producer, deviceRepo devicedomain.Repository) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   "alerts.generated",
		GroupID: "argus-alert-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[ALERT CONSUMER] started, consuming topic: alerts.generated")

		batchSize := 100
	flushInterval := 500 * time.Millisecond
	kafkaMsgChan := make(chan segmentio.Message, 2000)

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

	var workerWg sync.WaitGroup
	for w := 1; w <= 32; w++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			var batchAlerts []ruledomain.Alert
			var batchMessages []segmentio.Message
			knownRules := make(map[string]struct{})
			deviceWorkspaces := make(map[string]string)
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
					log.Printf("[ALERT CONSUMER] permanent error persisting alert batch: %v", err)
				} else {
					pipe := redisClient.Client().Pipeline()
					for _, alert := range batchAlerts {
						cooldownKey := fmt.Sprintf("alert:cooldown:%s:%s", alert.RuleID, alert.DeviceID)
						cooldownTTL := time.Duration(cfg.AlertCooldownSeconds) * time.Second
						pipe.Set(ctx, cooldownKey, "1", cooldownTTL)
					}
					_, _ = pipe.Exec(ctx)
				}
				_ = consumer.CommitMessages(ctx, batchMessages...)
				batchAlerts = batchAlerts[:0]
				batchMessages = batchMessages[:0]
			}

			for {
				select {
				case <-ctx.Done():
					flush()
					return
				case msg := <-kafkaMsgChan:
					workerCtx := ctx
					if corrID := extractCorrelationID(msg.Headers); corrID != "" {
						workerCtx = common.WithCorrelationID(ctx, corrID)
					}

					var a ruledomain.Alert
					if err := json.Unmarshal(msg.Value, &a); err != nil {
						_ = consumer.CommitMessages(workerCtx, msg)
						continue
					}

					if _, cached := knownRules[a.RuleID]; !cached {
						ruleEntity, err := ruleRepo.GetRule(workerCtx, a.RuleID)
						if err == nil && ruleEntity != nil {
							knownRules[a.RuleID] = struct{}{}
						} else {
							autoRule := ruledomain.Rule{
								ID:        a.RuleID,
								Name:      fmt.Sprintf("Auto-Provisioned Rule %s", a.RuleID),
								Metric:    a.Metric,
								Operator:  a.Operator,
								Threshold: a.Threshold,
								Enabled:   true,
							}
							if _, err := ruleRepo.CreateRule(workerCtx, autoRule); err != nil {
								if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate key") {
									_ = consumer.CommitMessages(workerCtx, msg)
									continue
								}
							}
							knownRules[a.RuleID] = struct{}{}
						}
					}

					var wsID string
					var deviceEntity *devicedomain.Device
					var err error
					for {
						if cachedWsID, cached := deviceWorkspaces[a.DeviceID]; cached {
							wsID = cachedWsID
							break
						}
						deviceEntity, err = deviceRepo.GetByID(workerCtx, a.DeviceID)
						if err == nil && deviceEntity != nil {
							if deviceEntity.WorkspaceID != nil {
								wsID = *deviceEntity.WorkspaceID
							}
							deviceWorkspaces[a.DeviceID] = wsID
							break
						}
						if err != nil && isTransientError(err) {
							time.Sleep(5 * time.Second)
							continue
						}
						_ = consumer.CommitMessages(workerCtx, msg)
						break
					}
					if wsID == "" && err != nil && !isTransientError(err) {
						continue
					}
					a.WorkspaceID = wsID

					duplicateInBatch := false
					for _, batched := range batchAlerts {
						if batched.RuleID == a.RuleID && batched.DeviceID == a.DeviceID {
							duplicateInBatch = true
							break
						}
					}
					if duplicateInBatch {
						_ = consumer.CommitMessages(workerCtx, msg)
						continue
					}

					cooldownKey := fmt.Sprintf("alert:cooldown:%s:%s", a.RuleID, a.DeviceID)
					exists, err := redisClient.Client().Exists(workerCtx, cooldownKey).Result()
					if err == nil && exists > 0 {
						_ = consumer.CommitMessages(workerCtx, msg)
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
		}()
	}
	workerWg.Wait()
}

func startIncidentConsumer(ctx context.Context, cfg *config.Config, eventRepo eventdomain.Repository, embedSvc *memory.EmbeddingService, kafkaProducer *kafka.Producer, deviceRepo devicedomain.Repository) {
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaIncidentTopic,
		GroupID: "argus-incident-event-consumer-internal",
	})
	defer consumer.Close()

	log.Printf("[INCIDENT CONSUMER] started, consuming topic: %s", cfg.KafkaIncidentTopic)
	kafkaMsgChan := make(chan segmentio.Message, 2000)
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
					log.Printf("[INCIDENT CONSUMER] fetch error: %v", err)
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

	var workerWg sync.WaitGroup
	for w := 1; w <= 32; w++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			deviceWorkspaces := make(map[string]string)
			var batchMessages []segmentio.Message
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			flush := func() {
				if len(batchMessages) == 0 {
					return
				}
				_ = consumer.CommitMessages(ctx, batchMessages...)
				batchMessages = batchMessages[:0]
			}

			for {
				select {
				case <-ctx.Done():
					flush()
					return
				case msg := <-kafkaMsgChan:
					workerCtx := ctx
					if corrID := extractCorrelationID(msg.Headers); corrID != "" {
						workerCtx = common.WithCorrelationID(ctx, corrID)
					}

					var inc kafka.IncidentEvent
					if err := json.Unmarshal(msg.Value, &inc); err != nil {
						batchMessages = append(batchMessages, msg)
						continue
					}

					isSignificant := (inc.Status == "OPEN" && inc.Severity == "critical") || inc.Status == "CLOSE"
					if !isSignificant {
						batchMessages = append(batchMessages, msg)
						continue
					}

					var wsID string
					var deviceEntity *devicedomain.Device
					var lookupErr error
					for {
						if cachedWsID, cached := deviceWorkspaces[inc.DeviceID]; cached {
							wsID = cachedWsID
							break
						}
						deviceEntity, lookupErr = deviceRepo.GetByID(workerCtx, inc.DeviceID)
						if lookupErr == nil && deviceEntity != nil {
							if deviceEntity.WorkspaceID != nil {
								wsID = *deviceEntity.WorkspaceID
							}
							deviceWorkspaces[inc.DeviceID] = wsID
							break
						}
						if lookupErr != nil && isTransientError(lookupErr) {
							time.Sleep(5 * time.Second)
							continue
						}
						batchMessages = append(batchMessages, msg)
						break
					}
					if wsID == "" && lookupErr != nil && !isTransientError(lookupErr) {
						continue
					}

					severity := eventdomain.SeverityWarning
					if inc.Severity == "critical" {
						severity = eventdomain.SeverityCritical
					} else if inc.Status == "CLOSE" {
						severity = eventdomain.SeverityInfo
					}

					eventType := "incident_critical"
					title := "Critical Fleet Anomaly"
					summary := fmt.Sprintf("Critical anomaly detected on device %s for metric %s: type=%s, peak_score=%.2f", inc.DeviceID, inc.Metric, inc.IncidentType, inc.Score)
					if inc.Status == "CLOSE" {
						eventType = "incident_resolution"
						title = "Fleet Incident Resolved"
						summary = fmt.Sprintf("Incident for device %s metric %s resolved", inc.DeviceID, inc.Metric)
					}

					metadataBytes, _ := json.Marshal(map[string]any{
						"incident_type": inc.IncidentType,
						"metric":        inc.Metric,
						"score":         inc.Score,
						"session_id":    inc.SessionID,
						"status":        inc.Status,
					})

					newEvent := eventdomain.Event{
						ID:              uuid.New().String(),
						DeviceID:        inc.DeviceID,
						WorkspaceID:     wsID,
						Type:            eventType,
						Severity:        severity,
						Title:           title,
						Summary:         summary,
						Source:          "anomaly_worker",
						ConfidenceScore: 1.0,
						Metadata:        json.RawMessage(metadataBytes),
						CreatedAt:       inc.Timestamp,
					}

					var created *eventdomain.Event
					var createErr error
					err := withRetry(3, func() error {
						created, createErr = eventRepo.Create(workerCtx, newEvent)
						return createErr
					})

					if err == nil && embedSvc != nil {
						embedSvc.EnqueueEvent(*created)
					}

					batchMessages = append(batchMessages, msg)
					if len(batchMessages) >= 100 {
						flush()
					}
				case <-ticker.C:
					flush()
				}
			}
		}()
	}
	workerWg.Wait()
}

func startCorrelationEngine(ctx context.Context, redisClient *redisinfra.Client, eventRepo eventdomain.Repository, embedSvc *memory.EmbeddingService, deviceRepo devicedomain.Repository) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Println("[CORRELATION ENGINE] started, running every 1 minute")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lockTTL := 45 * time.Second
			lock := NewTokenLock(redisClient, "lock:correlation_engine", lockTTL)
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
							log.Printf("[Lock Watchdog] correlation_engine lock renewal failed: %v", err)
							cancelJob()
							return
						}
					}
				}
			}()

			runCorrelation(jobCtx, redisClient, eventRepo, embedSvc, deviceRepo)

			// Stop watchdog
			cancelJob()
			<-watchdogDone

			_ = lock.Release(ctx)
		}
	}
}

func runCorrelation(ctx context.Context, redisClient *redisinfra.Client, eventRepo eventdomain.Repository, embedSvc *memory.EmbeddingService, deviceRepo devicedomain.Repository) {
	rdb := redisClient.Client()

	activeSessions, err := rdb.SMembers(ctx, "sessions:active").Result()
	if err != nil || len(activeSessions) == 0 {
		return
	}

	for _, sessionID := range activeSessions {
		incidentsSetKey := fmt.Sprintf("session:%s:incidents", sessionID)
		incidentKeys, err := rdb.SMembers(ctx, incidentsSetKey).Result()
		if err != nil || len(incidentKeys) == 0 {
			continue
		}

		vals, err := rdb.MGet(ctx, incidentKeys...).Result()
		if err != nil {
			continue
		}

		type groupKey struct {
			incidentType string
			metric       string
		}

		groups := make(map[groupKey][]string)

		for _, v := range vals {
			vStr, ok := v.(string)
			if !ok || vStr == "" {
				continue
			}

			var inc struct {
				DeviceID     string `json:"device_id"`
				Metric       string `json:"metric"`
				IncidentType string `json:"incident_type"`
			}
			if err := json.Unmarshal([]byte(vStr), &inc); err == nil {
				k := groupKey{
					incidentType: inc.IncidentType,
					metric:       inc.Metric,
				}
				groups[k] = append(groups[k], inc.DeviceID)
			}
		}

		for k, devices := range groups {
			distinctDevices := make(map[string]bool)
			for _, dev := range devices {
				distinctDevices[dev] = true
			}

			if len(distinctDevices) >= 3 {
				deviceList := make([]string, 0, len(distinctDevices))
				for dev := range distinctDevices {
					deviceList = append(deviceList, dev)
				}

				timeWindow := time.Now().UTC().Truncate(15 * time.Minute).Format("1504")
				cooldownKey := fmt.Sprintf("finding:cooldown:%s:%s:%s:%s", sessionID, k.metric, k.incidentType, timeWindow)

				allowed := rdb.SetNX(ctx, cooldownKey, "1", 1*time.Hour).Val()
				if !allowed {
					continue
				}

				representativeDeviceID := deviceList[0]

				deviceEntity, err := deviceRepo.GetByID(ctx, representativeDeviceID)
				if err != nil || deviceEntity == nil {
					log.Printf("[CORRELATION ENGINE] Skipping fleet finding event: representative device %s not registered: %v", representativeDeviceID, err)
					continue
				}

				var wsID string
				if deviceEntity.WorkspaceID != nil {
					wsID = *deviceEntity.WorkspaceID
				}

				metadataBytes, _ := json.Marshal(map[string]any{
					"session_id":    sessionID,
					"metric":        k.metric,
					"incident_type": k.incidentType,
					"devices":       deviceList,
				})

				newEvent := eventdomain.Event{
					ID:              uuid.New().String(),
					DeviceID:        representativeDeviceID,
					WorkspaceID:     wsID,
					Type:            "fleet_finding",
					Severity:        eventdomain.SeverityCritical,
					Title:           "Fleet-Wide Anomaly Detected",
					Summary:         fmt.Sprintf("Fleet correlation: %d devices are experiencing '%s' anomalies on metric '%s'", len(deviceList), k.incidentType, k.metric),
					Source:          "correlation_engine",
					ConfidenceScore: 0.90,
					Metadata:        json.RawMessage(metadataBytes),
					CreatedAt:       time.Now().UTC(),
				}

				created, err := eventRepo.Create(ctx, newEvent)
				if err != nil {
					log.Printf("[CORRELATION ENGINE] Failed to log fleet finding: %v", err)
					continue
				}

				if embedSvc != nil {
					embedSvc.EnqueueEvent(*created)
				}
			}
		}
	}
}

func osGetEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
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
		delay *= 2
	}
	return err
}

type TokenLock struct {
	client *redisinfra.Client
	key    string
	token  string
	ttl    time.Duration
}

func NewTokenLock(client *redisinfra.Client, key string, ttl time.Duration) *TokenLock {
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

func correlationServerUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("x-correlation-id"); len(vals) > 0 {
			ctx = common.WithCorrelationID(ctx, vals[0])
		}
	}
	return handler(ctx, req)
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "circuit breaker") ||
		strings.Contains(errStr, "open") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "desc = transport") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "Service Unavailable")
}
