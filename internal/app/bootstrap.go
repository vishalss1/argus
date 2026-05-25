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
	commanddomain "github.com/vishalss1/argus/internal/domain/command"
	devicedomain "github.com/vishalss1/argus/internal/domain/device"
	otadomain "github.com/vishalss1/argus/internal/domain/ota"
	ruledomain "github.com/vishalss1/argus/internal/domain/rule"
	shadowdomain "github.com/vishalss1/argus/internal/domain/shadow"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/kafka"
	minioinfra "github.com/vishalss1/argus/internal/infrastructure/minio"
	"github.com/vishalss1/argus/internal/infrastructure/mqtt"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
	redisinfra "github.com/vishalss1/argus/internal/infrastructure/redis"
	rulesinfra "github.com/vishalss1/argus/internal/infrastructure/rules"
	transporthandler "github.com/vishalss1/argus/internal/transport/http/handler"
	transportrouter "github.com/vishalss1/argus/internal/transport/http/router"
	transportws "github.com/vishalss1/argus/internal/transport/websocket"
)

type Server struct {
	httpServer    *http.Server
	db            *sql.DB
	kafkaProducer *kafka.Producer
	mqttClient    *mqtt.Client
	redisClient   *redisinfra.Client
	minioClient   *minioinfra.Client
	websocketHub  *transportws.Hub
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func Bootstrap() (*Server, error) {
	cfg := config.Load()

	database, err := postgres.InitDB(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	deviceRepository := postgres.NewDeviceRepository(database)
	deviceService := devicedomain.NewService(deviceRepository)
	deviceService.SetProvisioningConfig(devicedomain.ProvisioningConfig{
		MQTTBrokerURL:        cfg.ProvisioningBroker,
		MQTTTelemetryPattern: cfg.MQTTTelemetryTopic,
	})
	websocketHub := transportws.NewHub()
	realtime := &realtimePublisher{hub: websocketHub}
	deviceService.SetEventPublisher(realtime)
	deviceHandler := transporthandler.NewDeviceHandler(deviceService)

	ruleRepository := postgres.NewRuleRepository(database)
	ruleService := ruledomain.NewService(ruleRepository)
	ruleHandler := transporthandler.NewRuleHandler(ruleService)

	telemetryRepository := telemetrydomain.Repository(postgres.NewTelemetryRepository(database))
	telemetryRepository = rulesinfra.NewTelemetryRepository(telemetryRepository, ruleService)
	var kafkaProducer *kafka.Producer
	if len(cfg.KafkaBrokers) > 0 {
		kafkaProducer, err = kafka.NewProducer(kafka.Config{
			Brokers:        cfg.KafkaBrokers,
			TelemetryTopic: cfg.KafkaTelemetryTopic,
			CommandTopic:   cfg.KafkaCommandTopic,
		})
		if err != nil {
			return nil, err
		}
		telemetryRepository = kafka.NewTelemetryRepository(telemetryRepository, kafkaProducer)
	}
	telemetryService := telemetrydomain.NewService(telemetryRepository)
	telemetryService.SetEventPublisher(realtime)
	telemetryHandler := transporthandler.NewTelemetryHandler(telemetryService)

	commandRepository := commanddomain.Repository(postgres.NewCommandRepository(database))
	if kafkaProducer != nil {
		commandRepository = kafka.NewCommandRepository(commandRepository, kafkaProducer)
	}
	commandService := commanddomain.NewService(commandRepository)
	commandHandler := transporthandler.NewCommandHandler(commandService)

	redisClient, err := redisinfra.New(context.Background(), redisinfra.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		return nil, err
	}
	shadowRepository := redisinfra.NewShadowRepository(redisClient)
	shadowService := shadowdomain.NewService(shadowRepository)
	shadowHandler := transporthandler.NewShadowHandler(shadowService)

	minioClient, err := minioinfra.New(context.Background(), minioinfra.Config{
		Endpoint:        cfg.MinIOEndpoint,
		AccessKeyID:     cfg.MinIOAccessKey,
		SecretAccessKey: cfg.MinIOSecretKey,
		Bucket:          cfg.MinIOBucket,
		UseSSL:          cfg.MinIOUseSSL,
	})
	if err != nil {
		return nil, err
	}
	otaRepository := postgres.NewOTARepository(database)
	otaService := otadomain.NewService(otaRepository, minioClient)
	otaHandler := transporthandler.NewOTAHandler(otaService)

	var mqttClient *mqtt.Client
	if cfg.MQTTBrokerURL != "" {
		mqttClient, err = mqtt.New(mqtt.Config{
			BrokerURL:      cfg.MQTTBrokerURL,
			ClientID:       cfg.MQTTClientID,
			TelemetryTopic: cfg.MQTTTelemetryTopic,
		}, telemetryService)
		if err != nil {
			return nil, err
		}
	}

	websocketHandler := transportws.NewHandler(websocketHub)
	router := transportrouter.New(deviceHandler, telemetryHandler, shadowHandler, commandHandler, otaHandler, ruleHandler, websocketHandler)
	appCtx, cancel := context.WithCancel(context.Background())
	server := &Server{
		db:            database,
		kafkaProducer: kafkaProducer,
		mqttClient:    mqttClient,
		redisClient:   redisClient,
		minioClient:   minioClient,
		websocketHub:  websocketHub,
		cancel:        cancel,
		httpServer: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
		},
	}
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		websocketHub.Run(appCtx)
	}()
	server.startHeartbeatMonitor(appCtx, deviceService, cfg.HeartbeatInterval, cfg.HeartbeatTimeout)

	return server, nil
}

func (s *Server) Start() error {
	if s.mqttClient != nil {
		if err := s.mqttClient.Start(); err != nil {
			return err
		}
	}

	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	if s.httpServer != nil {
		_ = s.httpServer.Close()
	}
	if s.websocketHub != nil {
		s.websocketHub.Close()
	}
	if s.mqttClient != nil {
		s.mqttClient.Close()
	}
	if s.kafkaProducer != nil {
		_ = s.kafkaProducer.Close()
	}
	if s.redisClient != nil {
		_ = s.redisClient.Close()
	}
	if s.db == nil {
		return nil
	}

	return s.db.Close()
}

func (s *Server) startHeartbeatMonitor(ctx context.Context, service *devicedomain.Service, interval time.Duration, timeout time.Duration) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		runHeartbeatMonitor(ctx, service, timeout)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runHeartbeatMonitor(ctx, service, timeout)
			}
		}
	}()
}

func runHeartbeatMonitor(ctx context.Context, service *devicedomain.Service, timeout time.Duration) {
	devices, err := service.MarkStaleOffline(ctx, timeout)
	if err != nil {
		if !errors.Is(ctx.Err(), context.Canceled) {
			log.Printf("heartbeat monitor failed: %v", err)
		}
		return
	}

	log.Printf("heartbeat monitor marked %d device(s) offline", len(devices))
}
