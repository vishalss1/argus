package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type Config struct {
	Brokers        []string
	TelemetryTopic string
}

type Producer struct {
	writer *segmentio.Writer
}

func NewProducer(config Config) (*Producer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("kafka brokers are required")
	}
	if config.TelemetryTopic == "" {
		config.TelemetryTopic = "argus.telemetry"
	}

	return &Producer{
		writer: &segmentio.Writer{
			Addr:     segmentio.TCP(config.Brokers...),
			Topic:    config.TelemetryTopic,
			Balancer: &segmentio.Hash{},
		},
	}, nil
}

func (p *Producer) PublishTelemetry(ctx context.Context, event telemetry.Telemetry) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal telemetry event: %w", err)
	}

	err = p.writer.WriteMessages(ctx, segmentio.Message{
		Key:   []byte(event.DeviceID),
		Value: payload,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("write telemetry event: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}

	return p.writer.Close()
}
