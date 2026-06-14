package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	segmentio "github.com/segmentio/kafka-go"
	"github.com/vishalss1/argus/shared/common"
	"github.com/vishalss1/argus/telemetry/internal/domain/telemetry"
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
	AlertTopic     string
	DLQTopic       string
	IncidentTopic  string
}

type Producer struct {
	telemetryWriter *segmentio.Writer
	alertWriter     *segmentio.Writer
	dlqWriter       *segmentio.Writer
	incidentWriter  *segmentio.Writer
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
	if config.AlertTopic == "" {
		config.AlertTopic = "alerts.generated"
	}
	if config.DLQTopic == "" {
		config.DLQTopic = "argus.dlq"
	}
	if config.IncidentTopic == "" {
		config.IncidentTopic = "telemetry.incidents"
	}

	log.Printf("[KAFKA] initializing producer with brokers: %v, telemetry topic: %s, command topic: %s, alert topic: %s, dlq topic: %s, incident topic: %s", config.Brokers, config.TelemetryTopic, config.CommandTopic, config.AlertTopic, config.DLQTopic, config.IncidentTopic)

	return &Producer{
		telemetryWriter: &segmentio.Writer{
			Addr:                   segmentio.TCP(config.Brokers...),
			Topic:                  config.TelemetryTopic,
			Balancer:               &segmentio.Hash{},
			AllowAutoTopicCreation: true,
			Async:                  false,
		},
		alertWriter: &segmentio.Writer{
			Addr:                   segmentio.TCP(config.Brokers...),
			Topic:                  config.AlertTopic,
			Balancer:               &segmentio.Hash{},
			AllowAutoTopicCreation: true,
			Async:                  false,
		},
		dlqWriter: &segmentio.Writer{
			Addr:                   segmentio.TCP(config.Brokers...),
			Topic:                  config.DLQTopic,
			Balancer:               &segmentio.Hash{},
			AllowAutoTopicCreation: true,
			Async:                  false,
		},
		incidentWriter: &segmentio.Writer{
			Addr:                   segmentio.TCP(config.Brokers...),
			Topic:                  config.IncidentTopic,
			Balancer:               &segmentio.Hash{},
			AllowAutoTopicCreation: true,
			Async:                  false,
		},
	}, nil
}

func (p *Producer) PublishTelemetry(ctx context.Context, event telemetry.Telemetry) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal telemetry event: %w", err)
	}

	err = p.telemetryWriter.WriteMessages(ctx, segmentio.Message{
		Key:     []byte(event.DeviceID),
		Value:   payload,
		Headers: getCorrelationHeader(ctx),
		Time:    time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("write telemetry event: %w", err)
	}

	return nil
}

func (p *Producer) PublishAlert(ctx context.Context, alert any) error {
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}

	// Try to extract device_id for the key if it's a map or has the field
	key := ""
	if m, ok := alert.(map[string]any); ok {
		if id, ok := m["device_id"].(string); ok {
			key = id
		}
	}

	err = p.alertWriter.WriteMessages(ctx, segmentio.Message{
		Key:     []byte(key),
		Value:   payload,
		Headers: getCorrelationHeader(ctx),
		Time:    time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("write alert: %w", err)
	}

	return nil
}

type IncidentEvent struct {
	DeviceID     string    `json:"device_id"`
	SessionID    string    `json:"session_id"`
	IncidentType string    `json:"incident_type"`
	Metric       string    `json:"metric"`
	Severity     string    `json:"severity"`
	Score        float64   `json:"score"`
	Status       string    `json:"status"`
	Timestamp    time.Time `json:"timestamp"`
}

func (p *Producer) PublishIncident(ctx context.Context, event IncidentEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal incident event: %w", err)
	}

	err = p.incidentWriter.WriteMessages(ctx, segmentio.Message{
		Key:     []byte(event.DeviceID),
		Value:   payload,
		Headers: getCorrelationHeader(ctx),
		Time:    time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("write incident event: %w", err)
	}

	return nil
}

func (p *Producer) PublishDLQ(ctx context.Context, sourceTopic string, key []byte, value []byte, errorMsg string) error {
	dlqPayload := map[string]any{
		"source_topic": sourceTopic,
		"error":        errorMsg,
		"failed_at":    time.Now().UTC(),
		"original_key": string(key),
		"payload":      json.RawMessage(value), // Assuming payload is JSON
	}

	// If the value isn't valid JSON, fallback to raw string
	if !json.Valid(value) {
		dlqPayload["payload"] = string(value)
	}

	payload, err := json.Marshal(dlqPayload)
	if err != nil {
		return fmt.Errorf("marshal dlq payload: %w", err)
	}

	err = p.dlqWriter.WriteMessages(ctx, segmentio.Message{
		Key:   key,
		Value: payload,
		Time:  time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("write dlq: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	if p == nil {
		return nil
	}

	if p.telemetryWriter != nil {
		_ = p.telemetryWriter.Close()
	}

	if p.alertWriter != nil {
		_ = p.alertWriter.Close()
	}
	if p.dlqWriter != nil {
		_ = p.dlqWriter.Close()
	}
	if p.incidentWriter != nil {
		_ = p.incidentWriter.Close()
	}

	return nil
}
