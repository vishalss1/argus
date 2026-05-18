package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	commanddomain "github.com/vishalss1/argus/internal/domain/command"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type Config struct {
	Brokers        []string
	TelemetryTopic string
	CommandTopic   string
}

type Producer struct {
	telemetryWriter *segmentio.Writer
	commandWriter   *segmentio.Writer
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

	return &Producer{
		telemetryWriter: &segmentio.Writer{
			Addr:     segmentio.TCP(config.Brokers...),
			Topic:    config.TelemetryTopic,
			Balancer: &segmentio.Hash{},
		},
		commandWriter: &segmentio.Writer{
			Addr:     segmentio.TCP(config.Brokers...),
			Topic:    config.CommandTopic,
			Balancer: &segmentio.Hash{},
		},
	}, nil
}

func (p *Producer) PublishTelemetry(ctx context.Context, event telemetry.Telemetry) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal telemetry event: %w", err)
	}

	err = p.telemetryWriter.WriteMessages(ctx, segmentio.Message{
		Key:   []byte(event.DeviceID),
		Value: payload,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("write telemetry event: %w", err)
	}

	return nil
}

func (p *Producer) PublishCommand(ctx context.Context, event commanddomain.Command) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal command event: %w", err)
	}

	err = p.commandWriter.WriteMessages(ctx, segmentio.Message{
		Key:   []byte(event.DeviceID),
		Value: payload,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("write command event: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	if p == nil {
		return nil
	}

	if p.telemetryWriter != nil {
		if err := p.telemetryWriter.Close(); err != nil {
			return err
		}
	}
	if p.commandWriter != nil {
		return p.commandWriter.Close()
	}

	return nil
}
