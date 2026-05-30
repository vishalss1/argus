package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/vishalss1/argus/internal/domain/device"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type Config struct {
	BrokerURL      string
	ClientID       string
	TelemetryTopic string
	StateTopic     string
}

type Client struct {
	client           paho.Client
	telemetryService *telemetry.Service
	presenceService  *device.PresenceService
	telemetryTopic   string
	stateTopic       string
}

type telemetryMessage struct {
	RecordedAt *time.Time      `json:"recorded_at,omitempty"`
	Metrics    json.RawMessage `json:"metrics"`
}

type stateMessage struct {
	Status    string         `json:"status"`
	Timestamp string         `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func New(config Config, telemetryService *telemetry.Service, presenceService *device.PresenceService) (*Client, error) {
	if config.BrokerURL == "" {
		return nil, fmt.Errorf("mqtt broker url is required")
	}
	if config.ClientID == "" {
		config.ClientID = "argus-api"
	}
	if config.TelemetryTopic == "" {
		config.TelemetryTopic = "devices/+/telemetry"
	}
	if config.StateTopic == "" {
		config.StateTopic = "devices/+/state"
	}

	mqttClient := &Client{
		telemetryService: telemetryService,
		presenceService:  presenceService,
		telemetryTopic:   config.TelemetryTopic,
		stateTopic:       config.StateTopic,
	}

	options := paho.NewClientOptions().
		AddBroker(config.BrokerURL).
		SetClientID(config.ClientID).
		SetCleanSession(false).
		SetKeepAlive(30 * time.Second).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			log.Printf("mqtt connection lost: %v", err)
		}).
		SetOnConnectHandler(func(client paho.Client) {
			log.Printf("mqtt connected: broker=%s client_id=%s", config.BrokerURL, config.ClientID)
			mqttClient.subscribe(client)
		}).
		SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
			log.Printf("mqtt reconnecting: broker=%s client_id=%s", config.BrokerURL, config.ClientID)
		}).
		SetDefaultPublishHandler(mqttClient.handleMessage)

	mqttClient.client = paho.NewClient(options)
	return mqttClient, nil
}

func (c *Client) Start() error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("connect mqtt broker: %w", token.Error())
	}

	return nil
}

func (c *Client) Close() {
	if c.client == nil || !c.client.IsConnected() {
		return
	}

	c.client.Disconnect(250)
}

func (c *Client) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal mqtt payload: %w", err)
	}

	log.Printf("[MQTT] publishing to %s: %s", topic, string(data))
	token := c.client.Publish(topic, qos, retained, data)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt publish: %w", token.Error())
	}

	return nil
}

func (c *Client) handleMessage(_ paho.Client, message paho.Message) {
	switch {
	case topicMatches(c.stateTopic, message.Topic()):
		c.handleStateMessage(message)
	case topicMatches(c.telemetryTopic, message.Topic()):
		c.handleTelemetryMessage(message)
	default:
		log.Printf("mqtt ignored unexpected topic: %s", message.Topic())
	}
}

func (c *Client) subscribe(client paho.Client) {
	if token := client.Subscribe(c.stateTopic, 1, c.handleMessage); token.Wait() && token.Error() != nil {
		log.Printf("mqtt subscribe state topic failed: %v", token.Error())
		return
	}
	log.Printf("mqtt subscribed to %s", c.stateTopic)

	if token := client.Subscribe(c.telemetryTopic, 1, c.handleMessage); token.Wait() && token.Error() != nil {
		log.Printf("mqtt subscribe telemetry topic failed: %v", token.Error())
		return
	}
	log.Printf("mqtt subscribed to %s", c.telemetryTopic)
}

func (c *Client) handleTelemetryMessage(message paho.Message) {
	log.Printf("[MQTT DEBUG] incoming telemetry | topic: %s | payload: %s", message.Topic(), string(message.Payload()))
	rawID, err := deviceIDFromTopic(c.telemetryTopic, message.Topic())
	if err != nil {
		log.Printf("[MQTT DEBUG] topic match failed: %v", err)
		return
	}

	// 1. Resolve UUID (topic might contain Hardware ID)
	var deviceID string
	device, err := c.presenceService.GetDeviceByIDOrHardwareID(context.Background(), rawID)
	if err != nil {
		log.Printf("[MQTT DEBUG] device resolution failed for %q: %v. Ensure device is provisioned.", rawID, err)
		return
	}
	deviceID = device.ID
	log.Printf("[MQTT DEBUG] resolved device: %s (%s)", device.Name, deviceID)

	// 2. Decode Payload
	var payload telemetryMessage
	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Printf("[MQTT DEBUG] JSON decode failed: %v", err)
		return
	}

	// 3. Ingest
	_, err = c.telemetryService.Ingest(context.Background(), deviceID, telemetry.CreateInput{
		RecordedAt: payload.RecordedAt,
		Metrics:    payload.Metrics,
	})
	if err != nil {
		log.Printf("[MQTT DEBUG] ingestion failed: %v", err)
		return
	}
	log.Printf("[MQTT DEBUG] telemetry successfully ingested and broadcasted")
}

func (c *Client) handleStateMessage(message paho.Message) {
	rawID, err := deviceIDFromTopic(c.stateTopic, message.Topic())
	if err != nil {
		log.Printf("[MQTT] presence ignored: %v", err)
		return
	}

	// 1. Resolve UUID
	deviceEntity, err := c.presenceService.GetDeviceByIDOrHardwareID(context.Background(), rawID)
	if err != nil {
		log.Printf("[MQTT] failed to resolve device %s for state: %v", rawID, err)
		return
	}
	deviceID := deviceEntity.ID

	// 2. Decode Payload
	var payload stateMessage
	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Printf("[MQTT] presence decode failed for device %s: %v", deviceID, err)
		return
	}

	timestamp := time.Now().UTC()
	if strings.TrimSpace(payload.Timestamp) != "" {
		parsed, err := time.Parse(time.RFC3339, payload.Timestamp)
		if err != nil {
			log.Printf("[MQTT] presence invalid timestamp for device %s: %v", deviceID, err)
			return
		}
		timestamp = parsed.UTC()
	}

	_, err = c.presenceService.RecordState(context.Background(), deviceID, device.PresenceInput{
		Status:    device.PresenceStatus(payload.Status),
		Timestamp: timestamp,
		Metadata:  payload.Metadata,
	}, message.Retained())
	if err != nil {
		log.Printf("[MQTT] presence update failed for device %s: %v", deviceID, err)
		return
	}
	log.Printf("[MQTT] successfully updated state for device: %s (%s)", deviceID, payload.Status)
}

func topicMatches(pattern string, topic string) bool {
	_, err := deviceIDFromTopic(pattern, topic)
	return err == nil
}

func deviceIDFromTopic(pattern string, topic string) (string, error) {
	patternParts := strings.Split(pattern, "/")
	parts := strings.Split(topic, "/")
	if len(patternParts) != len(parts) {
		return "", fmt.Errorf("unexpected mqtt topic %q", topic)
	}

	deviceID := ""
	for i, patternPart := range patternParts {
		if patternPart == "+" {
			deviceID = parts[i]
			continue
		}
		if patternPart != parts[i] {
			return "", fmt.Errorf("unexpected mqtt topic %q", topic)
		}
	}

	if deviceID == "" {
		return "", fmt.Errorf("missing device id in mqtt topic %q", topic)
	}

	return deviceID, nil
}
