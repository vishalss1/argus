package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	commanddomain "github.com/vishalss1/argus/core/internal/domain/command"
	"github.com/vishalss1/argus/core/internal/domain/telemetry"
	"github.com/vishalss1/argus/shared/common"
)

func getCorrelationHeader(ctx context.Context) []segmentio.Header {
	if corrID, ok := common.GetCorrelationID(ctx); ok {
		return []segmentio.Header{
			{
				Key:   "correlation_id",
				Value: []byte(corrID),
			},
		}
	}
	return nil
}

type Config struct {
	Brokers        []string
	TelemetryTopic string
	CommandTopic   string
}

type Producer struct {
	commandWriter *segmentio.Writer
}

func NewProducer(config Config) (*Producer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are required")
	}
	if config.TelemetryTopic == "" {
		config.TelemetryTopic = "argus.telemetry"
	}
	if config.CommandTopic == "" {
		config.CommandTopic = "argus.commands"
	}

	log.Printf("[KAFKA] initializing producer with brokers: %v, command topic: %s", config.Brokers, config.CommandTopic)

	return &Producer{
		commandWriter: &segmentio.Writer{
			Addr:                   segmentio.TCP(config.Brokers...),
			Topic:                  config.CommandTopic,
			Balancer:               &segmentio.Hash{},
			AllowAutoTopicCreation: true,
			Async:                  false,
			BatchSize:              1000,
			BatchTimeout:           50 * time.Millisecond,
		},
	}, nil
}

// PublishTelemetry is intentionally a no-op on Core Service.
// Telemetry Ingestion Service ingests device telemetry via MQTT and produces directly to Kafka.
// Core Service consumes telemetry from Kafka solely to broadcast updates over WebSockets.
func (p *Producer) PublishTelemetry(ctx context.Context, event telemetry.Telemetry) error {
	return nil
}

func (p *Producer) PublishCommand(ctx context.Context, event commanddomain.Command) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal command event: %w", err)
	}

	err = p.commandWriter.WriteMessages(ctx, segmentio.Message{
		Key:     []byte(event.DeviceID),
		Value:   payload,
		Headers: getCorrelationHeader(ctx),
		Time:    time.Now().UTC(),
	})
	if err != nil {
		log.Printf("[KAFKA] failed to write command event: %v", err)
		return fmt.Errorf("write command event: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	if p == nil {
		return nil
	}

	if p.commandWriter != nil {
		_ = p.commandWriter.Close()
	}

	return nil
}

