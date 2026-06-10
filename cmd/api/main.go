package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/vishalss1/argus/internal/app"
)

// @title ARGUS API
// @version 1.0
// @description Distributed fleet monitoring and control API.
// @BasePath /
func main() {
	server, err := app.Bootstrap()
	if err != nil {
		log.Fatalf("failed to bootstrap app: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start pprof HTTP server for profiling
	pprofServer := &http.Server{Addr: ":6060", Handler: nil}
	go func() {
		log.Printf("[PPROF] starting pprof server on :6060")
		if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[PPROF] pprof server error: %v", err)
		}
	}()

	go func() {
		if err := server.Start(); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	sig := <-sigChan
	log.Printf("Received termination signal %v, shutting down gracefully...", sig)
	
	pprofServer.Close()
	if err := server.Close(); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
	log.Println("Server gracefully stopped.")
}
