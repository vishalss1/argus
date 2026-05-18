package main

import (
	"log"
	"net/http"

	"github.com/vishalss1/argus/src/db"
	"github.com/vishalss1/argus/src/internal/config"
	"github.com/vishalss1/argus/src/internal/handler"
	"github.com/vishalss1/argus/src/internal/repository"
	"github.com/vishalss1/argus/src/internal/router"
	"github.com/vishalss1/argus/src/internal/service"
)

func main() {
	cfg := config.Load()

	database, err := db.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer database.Close()

	deviceRepository := repository.NewPostgreDeviceRepository(database)
	deviceService := service.NewDeviceService(deviceRepository)
	deviceHandler := handler.NewDeviceHandler(deviceService)

	telemetryRepository := repository.NewPostgreTelemetryRepository(database)
	telemetryService := service.NewTelemetryService(telemetryRepository)
	telemetryHandler := handler.NewTelemetryHandler(telemetryService)

	httpRouter := router.New(deviceHandler, telemetryHandler)

	addr := ":" + cfg.Port
	log.Printf("argus api listening on %s", addr)
	if err := http.ListenAndServe(addr, httpRouter); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
