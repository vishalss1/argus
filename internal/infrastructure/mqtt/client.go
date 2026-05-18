package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type Config struct {
	BrokerURL      string
	ClientID       string
	TelemetryTopic string
}

type Client struct {
	client           paho.Client
	telemetryService *telemetry.Service
	telemetryTopic   string
}

type telemetryMessage struct {
	RecordedAt *time.Time      `json:"recorded_at,omitempty"`
	Metrics    json.RawMessage `json:"metrics"`
}

func New(config Config, telemetryService *telemetry.Service) (*Client, error) {
	if config.BrokerURL == "" {
		return nil, fmt.Errorf("mqtt broker url is required")
	}
	if config.ClientID == "" {
		config.ClientID = "argus-api"
	}
	if config.TelemetryTopic == "" {
		config.TelemetryTopic = "argus/devices/+/telemetry"
	}

	mqttClient := &Client{
		telemetryService: telemetryService,
		telemetryTopic:   config.TelemetryTopic,
	}

	options := paho.NewClientOptions().
		AddBroker(config.BrokerURL).
		SetClientID(config.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetDefaultPublishHandler(mqttClient.handleMessage)

	mqttClient.client = paho.NewClient(options)
	return mqttClient, nil
}

func (c *Client) Start() error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("connect mqtt broker: %w", token.Error())
	}

	if token := c.client.Subscribe(c.telemetryTopic, 1, c.handleMessage); token.Wait() && token.Error() != nil {
		return fmt.Errorf("subscribe mqtt telemetry topic: %w", token.Error())
	}

	log.Printf("mqtt subscribed to %s", c.telemetryTopic)
	return nil
}

func (c *Client) Close() {
	if c.client == nil || !c.client.IsConnected() {
		return
	}

	c.client.Disconnect(250)
}

func (c *Client) handleMessage(_ paho.Client, message paho.Message) {
	deviceID, err := deviceIDFromTelemetryTopic(c.telemetryTopic, message.Topic())
	if err != nil {
		log.Printf("mqtt telemetry ignored: %v", err)
		return
	}

	var payload telemetryMessage
	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Printf("mqtt telemetry decode failed for device %s: %v", deviceID, err)
		return
	}

	_, err = c.telemetryService.Ingest(context.Background(), deviceID, telemetry.CreateInput{
		RecordedAt: payload.RecordedAt,
		Metrics:    payload.Metrics,
	})
	if err != nil {
		log.Printf("mqtt telemetry ingest failed for device %s: %v", deviceID, err)
		return
	}
}

func deviceIDFromTelemetryTopic(pattern string, topic string) (string, error) {
	patternParts := strings.Split(pattern, "/")
	parts := strings.Split(topic, "/")
	if len(patternParts) != len(parts) {
		return "", fmt.Errorf("unexpected telemetry topic %q", topic)
	}

	deviceID := ""
	for i, patternPart := range patternParts {
		if patternPart == "+" {
			deviceID = parts[i]
			continue
		}
		if patternPart != parts[i] {
			return "", fmt.Errorf("unexpected telemetry topic %q", topic)
		}
	}

	if deviceID == "" {
		return "", fmt.Errorf("missing device id in telemetry topic %q", topic)
	}

	return deviceID, nil
}
