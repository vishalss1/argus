package main

import (
	"log"

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
	defer server.Close()

	if err := server.Start(); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
