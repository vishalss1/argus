package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vishalss1/argus/shared/common"
	"github.com/vishalss1/argus/telemetry/internal/app"
)

func main() {
	common.InitLogger("telemetry-service")
	server, err := app.Bootstrap()
	if err != nil {
		log.Fatalf("failed to bootstrap Telemetry Service: %v", err)
	}

	log.Println("Telemetry Service started successfully.")

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Telemetry Service...")
	if err := server.Close(); err != nil {
		log.Printf("error closing Telemetry Service: %v", err)
	}
	log.Println("Telemetry Service stopped.")
}
