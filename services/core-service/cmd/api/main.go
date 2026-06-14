package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vishalss1/argus/core/internal/app"
	"github.com/vishalss1/argus/shared/common"
)

// @title ARGUS API
// @version 1.0
// @description Distributed fleet monitoring and control API.
// @BasePath /
func main() {
	common.InitLogger("core-service")
	server, err := app.Bootstrap()
	if err != nil {
		log.Fatalf("failed to bootstrap app: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	sig := <-sigChan
	log.Printf("Received termination signal %v, shutting down gracefully...", sig)
	
	if err := server.Close(); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	log.Println("Server gracefully stopped.")
}
