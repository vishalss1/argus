package app

import (
	"database/sql"
	"net/http"

	"github.com/vishalss1/argus/internal/config"
	devicedomain "github.com/vishalss1/argus/internal/domain/device"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/kafka"
	"github.com/vishalss1/argus/internal/infrastructure/mqtt"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
	transporthandler "github.com/vishalss1/argus/internal/transport/http/handler"
	transportrouter "github.com/vishalss1/argus/internal/transport/http/router"
)

type Server struct {
	httpServer    *http.Server
	db            *sql.DB
	kafkaProducer *kafka.Producer
	mqttClient    *mqtt.Client
}

func Bootstrap() (*Server, error) {
	cfg := config.Load()

	database, err := postgres.InitDB(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	deviceRepository := postgres.NewDeviceRepository(database)
	deviceService := devicedomain.NewService(deviceRepository)
	deviceHandler := transporthandler.NewDeviceHandler(deviceService)

	telemetryRepository := telemetrydomain.Repository(postgres.NewTelemetryRepository(database))
	var kafkaProducer *kafka.Producer
	if len(cfg.KafkaBrokers) > 0 {
		kafkaProducer, err = kafka.NewProducer(kafka.Config{
			Brokers:        cfg.KafkaBrokers,
			TelemetryTopic: cfg.KafkaTelemetryTopic,
		})
		if err != nil {
			return nil, err
		}
		telemetryRepository = kafka.NewTelemetryRepository(telemetryRepository, kafkaProducer)
	}
	telemetryService := telemetrydomain.NewService(telemetryRepository)
	telemetryHandler := transporthandler.NewTelemetryHandler(telemetryService)

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

	router := transportrouter.New(deviceHandler, telemetryHandler)

	return &Server{
		db:            database,
		kafkaProducer: kafkaProducer,
		mqttClient:    mqttClient,
		httpServer: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
		},
	}, nil
}

func (s *Server) Start() error {
	if s.mqttClient != nil {
		if err := s.mqttClient.Start(); err != nil {
			return err
		}
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) Close() error {
	if s.mqttClient != nil {
		s.mqttClient.Close()
	}
	if s.kafkaProducer != nil {
		_ = s.kafkaProducer.Close()
	}
	if s.db == nil {
		return nil
	}

	return s.db.Close()
}
