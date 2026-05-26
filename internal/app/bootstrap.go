package app

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/vishalss1/argus/internal/config"
	"github.com/vishalss1/argus/internal/ai/memory"
	"github.com/vishalss1/argus/internal/ai/query"
	commanddomain "github.com/vishalss1/argus/internal/domain/command"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	devicedomain "github.com/vishalss1/argus/internal/domain/device"
	incidentdomain "github.com/vishalss1/argus/internal/domain/incident"
	otadomain "github.com/vishalss1/argus/internal/domain/ota"
	ruledomain "github.com/vishalss1/argus/internal/domain/rule"
	shadowdomain "github.com/vishalss1/argus/internal/domain/shadow"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
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
	cancel        context.CancelFunc
	wg            sync.WaitGroup
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

	deviceRepository := postgres.NewDeviceRepository(database)
	deviceService := devicedomain.NewService(deviceRepository)
	presenceService := devicedomain.NewPresenceService(deviceService)
	deviceHandler := transporthandler.NewDeviceHandler(deviceService)

	telemetryRepository := postgres.NewTelemetryRepository(database)
	var finalTelemetryRepo telemetrydomain.Repository = telemetryRepository
	if kafkaProducer != nil {
		finalTelemetryRepo = kafka.NewTelemetryRepository(telemetryRepository, kafkaProducer)
	}
	telemetryService := telemetrydomain.NewService(finalTelemetryRepo)
	telemetryHandler := transporthandler.NewTelemetryHandler(telemetryService)

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
		if err := memoryManager.SummarizeCommand(ctx, cmd); err != nil {
			log.Printf("failed to summarize command: %v", err)
		}
	}
	commandHandler := transporthandler.NewCommandHandler(commandService)

	otaRepository := postgres.NewOTARepository(database)
	otaService := otadomain.NewService(otaRepository, minioClient)
	otaService.OnResult = func(ctx context.Context, dep otadomain.Deployment) {
		if err := memoryManager.SummarizeDeployment(ctx, dep); err != nil {
			log.Printf("failed to summarize deployment: %v", err)
		}
	}
	otaHandler := transporthandler.NewOTAHandler(otaService)

	ruleRepository := postgres.NewRuleRepository(database)
	ruleService := ruledomain.NewService(ruleRepository)
	ruleHandler := transporthandler.NewRuleHandler(ruleService)

	incidentRepository := postgres.NewIncidentRepository(database)
	incidentService := incidentdomain.NewService(incidentRepository)
	incidentService.OnResolved = func(ctx context.Context, inc incidentdomain.Incident) {
		if err := memoryManager.SummarizeIncident(ctx, inc); err != nil {
			log.Printf("failed to summarize incident: %v", err)
		}
	}

	aiProvider := ai.NewGroqProvider(cfg.GroqAPIKey, cfg.GroqModel, cfg.GroqBaseURL)
	queryEngine := query.NewEngine(embeddingProvider, aiProvider, vectorStore, eventRepository, incidentRepository, contextRepository)

	aiHandler := transporthandler.NewAIHandler(eventRepository, incidentService, contextService, queryEngine)

	var mqttClient *mqtt.Client
	if cfg.MQTTBrokerURL != "" {
		mqttClient, err = mqtt.New(mqtt.Config{
			BrokerURL:      cfg.MQTTBrokerURL,
			ClientID:       cfg.MQTTClientID,
			TelemetryTopic: cfg.MQTTTelemetryTopic,
			StateTopic:     cfg.MQTTStateTopic,
		}, telemetryService, presenceService)
		if err != nil {
			cancel()
			return nil, err
		}
	}

	websocketHandler := transportws.NewHandler(websocketHub)
	router := transportrouter.New(deviceHandler, telemetryHandler, shadowHandler, commandHandler, otaHandler, ruleHandler, aiHandler, websocketHandler)

	server := &Server{
		db:            database,
		kafkaProducer: kafkaProducer,
		mqttClient:    mqttClient,
		httpServer: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
		},
		websocketHub: websocketHub,
		cancel:       cancel,
	}

	server.wg.Add(2)
	go func() {
		defer server.wg.Done()
		websocketHub.Run(appCtx)
	}()

	go func() {
		defer server.wg.Done()
		monitorPresence(appCtx, presenceService, cfg.HeartbeatTimeout, cfg.HeartbeatInterval)
	}()

	return server, nil
}

func (s *Server) Start() error {
	log.Printf("starting server on %s", s.httpServer.Addr)
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
