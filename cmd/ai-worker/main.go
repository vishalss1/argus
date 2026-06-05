package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/vishalss1/argus/internal/ai/analytics"
	"github.com/vishalss1/argus/internal/config"
	telemetrydomain "github.com/vishalss1/argus/internal/domain/telemetry"
	"github.com/vishalss1/argus/internal/infrastructure/kafka"
	"github.com/vishalss1/argus/internal/infrastructure/redis"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Redis Client
	redisClient, err := redis.New(ctx, redis.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		log.Fatalf("failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize Kafka Producer
	kafkaProducer, err := kafka.NewProducer(kafka.Config{
		Brokers:        cfg.KafkaBrokers,
		TelemetryTopic: cfg.KafkaTelemetryTopic,
		CommandTopic:   cfg.KafkaCommandTopic,
		IncidentTopic:  cfg.KafkaIncidentTopic,
	})
	if err != nil {
		log.Fatalf("failed to initialize Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	// Initialize AI Analytics Engine with Redis and Kafka
	analyticsEngine := analytics.NewEngine(redisClient.Client(), kafkaProducer)

	// Initialize Kafka Consumer for raw telemetry
	consumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTelemetryTopic,
		GroupID: cfg.KafkaAIWorkerGroupID,
	})
	defer consumer.Close()

	log.Printf("AI Worker started, consuming topic: %s", cfg.KafkaTelemetryTopic)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down AI Worker...")
		cancel()
	}()

	// Worker pool size
	const numWorkers = 16
	
	// Channels
	msgChan := make(chan segmentio.Message, 2000)
	commitChan := make(chan segmentio.Message, 2000)
	errChan := make(chan error, 1)

	// Spawn Workers
	for w := 1; w <= numWorkers; w++ {
		go func(workerID int) {
			for msg := range msgChan {
				var t telemetrydomain.Telemetry
				if err := json.Unmarshal(msg.Value, &t); err != nil {
					log.Printf("[AI WORKER-%d] decode error: %v", workerID, err)
					// Commit directly since it's invalid
					_ = consumer.CommitMessages(ctx, msg)
					continue
				}

				if err := analyticsEngine.Analyze(ctx, t); err != nil {
					log.Printf("[AI WORKER-%d] analytics error: %v", workerID, err)
				}

				select {
				case commitChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}(w)
	}

	// Fetcher Goroutine
	go func() {
		for {
			msg, err := consumer.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				errChan <- err
				time.Sleep(1 * time.Second)
				continue
			}
			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Committer/Reporter Loop
	var batch []segmentio.Message
	const maxBatchSize = 100
	commitInterval := 100 * time.Millisecond
	commitTimer := time.NewTicker(commitInterval)
	defer commitTimer.Stop()

	flushBatch := func(flushCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := consumer.CommitMessages(flushCtx, batch...); err != nil {
			log.Printf("[AI WORKER] failed to commit batch of %d messages: %v", len(batch), err)
		}
		batch = batch[:0]
	}

	var msgCount int64

	for {
		select {
		case <-ctx.Done():
			log.Println("[AI WORKER] context cancelled, flushing pending offsets...")
			close(msgChan)
			flushBatch(context.Background())
			return
		case err := <-errChan:
			log.Printf("[AI WORKER] fetch error: %v", err)
		case msg := <-commitChan:
			msgCount++
			if msgCount%500 == 0 {
				log.Printf("[AI WORKER] processed %d messages, pending commit: %d", msgCount, len(batch))
			}

			batch = append(batch, msg)
			if len(batch) >= maxBatchSize {
				flushBatch(ctx)
			}
		case <-commitTimer.C:
			flushBatch(ctx)
		}
	}
}
