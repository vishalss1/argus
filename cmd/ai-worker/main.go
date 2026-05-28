package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vishalss1/argus/internal/ai/anomaly"
	"github.com/vishalss1/argus/internal/ai/correlation"
	"github.com/vishalss1/argus/internal/ai/memory"
	"github.com/vishalss1/argus/internal/ai/semantic"
	"github.com/vishalss1/argus/internal/config"
	anomalydomain "github.com/vishalss1/argus/internal/domain/anomaly"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	eventdomain "github.com/vishalss1/argus/internal/domain/event"
	incidentdomain "github.com/vishalss1/argus/internal/domain/incident"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/embedding"
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

	embeddingProvider := embedding.NewOllamaProvider(cfg.OllamaBaseURL, cfg.OllamaEmbedModel)
	vectorStore := postgres.NewVectorStore(db)
	semanticEmbedding := semantic.NewEmbeddingService(embeddingProvider, vectorStore)
	correlationEmbedding := correlation.NewEmbeddingService(embeddingProvider, vectorStore)
	memoryEmbedding := memory.NewEmbeddingService(embeddingProvider, vectorStore)

	eventRepo := postgres.NewEventRepository(db)
	semanticEngine := semantic.NewEngine(eventRepo)

	anomalyEngine := anomaly.NewEngine(func(ctx context.Context, a anomalydomain.Anomaly) {
		// Convert anomaly to semantic event
		ev := eventdomain.Event{
			ID:              a.ID,
			DeviceID:        a.DeviceID,
			Type:            string(a.Type),
			Severity:        eventdomain.Severity(a.Severity),
			Title:           a.Title,
			Summary:         a.Summary,
			Source:          "anomaly_engine",
			ConfidenceScore: a.ConfidenceScore,
			Metadata:        a.Metadata,
			CreatedAt:       a.CreatedAt,
		}
		if _, err := eventRepo.Create(ctx, ev); err == nil {
			log.Printf("[AI WORKER] anomaly detected and persisted: %s (Device: %s)", ev.Title, ev.DeviceID)
			if err := semanticEmbedding.EmbedEvent(ctx, ev); err != nil {
				log.Printf("[AI WORKER] failed to embed anomaly event: %v", err)
			} else {
				log.Printf("[AI WORKER] anomaly event embedded successfully: %s", ev.ID)
			}
		} else {
			log.Printf("[AI WORKER] failed to persist anomaly event: %v", err)
		}
	})

	contextRepo := postgres.NewContextRepository(db)
	contextService := ctxdomain.NewService(contextRepo)
	contextService.OnRecord = func(ctx context.Context, mem ctxdomain.OperationalMemory) {
		if err := memoryEmbedding.EmbedMemory(ctx, mem); err != nil {
			log.Printf("failed to embed operational memory: %v", err)
		}
	}
	memoryManager := memory.NewManager(contextService)

	incidentRepo := postgres.NewIncidentRepository(db)
	incidentService := incidentdomain.NewService(incidentRepo)
	incidentService.OnCreated = func(ctx context.Context, inc incidentdomain.Incident) {
		if err := correlationEmbedding.EmbedIncident(ctx, inc); err != nil {
			log.Printf("failed to embed incident: %v", err)
		}
	}
	incidentService.OnResolved = func(ctx context.Context, inc incidentdomain.Incident) {
		if err := memoryManager.SummarizeIncident(ctx, inc); err != nil {
			log.Printf("failed to summarize incident: %v", err)
		}
	}

	correlationEngine := correlation.NewEngine(incidentService, eventRepo)

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

			if err := anomalyEngine.Analyze(ctx, t); err != nil {
				log.Printf("failed to run anomaly detection: %v", err)
			}

			events, err := semanticEngine.AnalyzeTelemetry(ctx, t)
			if err != nil {
				log.Printf("[AI WORKER] failed to analyze telemetry: %v", err)
			} else if len(events) > 0 {
				log.Printf("[AI WORKER] generated %d semantic events for device %s", len(events), t.DeviceID)
				for _, ev := range events {
					log.Printf("[AI WORKER] processing event: %s (%s)", ev.Title, ev.Type)
					if err := semanticEmbedding.EmbedEvent(ctx, ev); err != nil {
						log.Printf("[AI WORKER] failed to embed semantic event: %v", err)
					} else {
						log.Printf("[AI WORKER] semantic event embedded successfully: %s", ev.ID)
					}
					if err := correlationEngine.Correlate(ctx, ev); err != nil {
						log.Printf("[AI WORKER] failed to correlate event: %v", err)
					}
				}
			}

			if err := consumer.CommitMessages(ctx, msg); err != nil {
				log.Printf("failed to commit message: %v", err)
			}
		}
	}
}
