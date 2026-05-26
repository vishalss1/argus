package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vishalss1/argus/internal/ai/semantic"
	"github.com/vishalss1/argus/internal/config"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/kafka"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
)

func main() {
	cfg := config.Load()

	db, err := postgres.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	eventRepo := postgres.NewEventRepository(db)
	semanticEngine := semantic.NewEngine(eventRepo)

	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTelemetryTopic,
		GroupID: cfg.KafkaAIWorkerGroupID,
	})
	defer consumer.Close()

	log.Printf("AI Worker started, consuming topic: %s", cfg.KafkaTelemetryTopic)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down AI Worker...")
		cancel()
	}()

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
				log.Printf("failed to fetch message: %v", err)
				continue
			}

			var t telemetrydomain.Telemetry
			if err := json.Unmarshal(msg.Value, &t); err != nil {
				log.Printf("failed to unmarshal telemetry: %v", err)
				consumer.CommitMessages(ctx, msg)
				continue
			}

			events, err := semanticEngine.AnalyzeTelemetry(ctx, t)
			if err != nil {
				log.Printf("failed to analyze telemetry: %v", err)
			} else if len(events) > 0 {
				log.Printf("generated %d semantic events for device %s", len(events), t.DeviceID)
			}

			if err := consumer.CommitMessages(ctx, msg); err != nil {
				log.Printf("failed to commit message: %v", err)
			}
		}
	}
}
