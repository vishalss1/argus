package app

import (
	"database/sql"
	"net/http"

	"github.com/vishalss1/argus/internal/config"
	devicedomain "github.com/vishalss1/argus/internal/domain/device"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
	transporthandler "github.com/vishalss1/argus/internal/transport/http/handler"
	transportrouter "github.com/vishalss1/argus/internal/transport/http/router"
)

type Server struct {
	httpServer *http.Server
	db         *sql.DB
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

	telemetryRepository := postgres.NewTelemetryRepository(database)
	telemetryService := telemetrydomain.NewService(telemetryRepository)
	telemetryHandler := transporthandler.NewTelemetryHandler(telemetryService)

	router := transportrouter.New(deviceHandler, telemetryHandler)

	return &Server{
		db: database,
		httpServer: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
		},
	}, nil
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}

	return s.db.Close()
}
