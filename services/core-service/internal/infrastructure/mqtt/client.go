package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/vishalss1/argus/core/internal/domain/command"
	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/domain/ota"
)

type Config struct {
	BrokerURL      string
	ClientID       string
	StateTopic     string
}

type Client struct {
	client           paho.Client
	presenceService  *device.PresenceService
	commandService   *command.Service
	otaService       *ota.Service
	stateTopic       string
	resultTopic      string
	otaStatusTopic   string
	msgChan          chan paho.Message
	wg               sync.WaitGroup
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

type resultMessage struct {
	CommandID    string `json:"command_id,omitempty"`
	DeploymentID string `json:"deployment_id,omitempty"`
	Status       string `json:"status"` // ack or nack
	Message      string `json:"message"`
}

type otaStatusMessage struct {
	DeploymentID string `json:"deployment_id"`
	Status       string `json:"status"`
	Progress     *int   `json:"progress,omitempty"`
	Message      string `json:"message,omitempty"`
}

func New(config Config, presenceService *device.PresenceService, commandService *command.Service, otaService *ota.Service) (*Client, error) {
	if config.BrokerURL == "" {
		return nil, fmt.Errorf("mqtt broker url is required")
	}
	if config.ClientID == "" {
		config.ClientID = "argus-api"
	}
	if config.StateTopic == "" {
		config.StateTopic = "argus/devices/+/state"
	}
	resultTopic := "argus/devices/+/results"
	otaStatusTopic := "argus/devices/+/ota/status"

	mqttClient := &Client{
		presenceService:  presenceService,
		commandService:   commandService,
		otaService:       otaService,
		stateTopic:       config.StateTopic,
		resultTopic:      resultTopic,
		otaStatusTopic:   otaStatusTopic,
		// Non-blocking enqueue pattern: protects Paho's internal network reader loop from blocking
		// when worker processing slows down. Buffer capacity (16,384) accommodates extreme traffic spikes.
		// For QoS 1 messages (state/results/OTA), drops indicate extreme worker backlog exceeding channel capacity.
		msgChan:          make(chan paho.Message, 16384),
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
		SetDefaultPublishHandler(func(_ paho.Client, msg paho.Message) {
			select {
			case mqttClient.msgChan <- msg:
			default:
				log.Printf("[MQTT] message channel full (cap=%d), non-blocking drop for topic=%s", cap(mqttClient.msgChan), msg.Topic())
			}
		})

	mqttClient.client = paho.NewClient(options)
	return mqttClient, nil
}

func (c *Client) Start() error {
	for range 16 {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			for msg := range c.msgChan {
				c.dispatch(msg)
			}
		}()
	}

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
	close(c.msgChan)
	c.wg.Wait()
}

func (c *Client) IsConnected() bool {
	if c == nil || c.client == nil {
		return false
	}
	return c.client.IsConnected()
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

func (c *Client) dispatch(message paho.Message) {
	switch {
	case topicMatches(c.stateTopic, message.Topic()):
		c.handleStateMessage(message)
	case topicMatches(c.resultTopic, message.Topic()):
		c.handleResultMessage(message)
	case topicMatches(c.otaStatusTopic, message.Topic()):
		c.handleOTAStatusMessage(message)
	default:
		log.Printf("mqtt ignored unexpected topic: %s", message.Topic())
	}
}

func (c *Client) subscribe(client paho.Client) {
	if token := client.Subscribe(c.stateTopic, 1, func(_ paho.Client, msg paho.Message) {
		select {
		case c.msgChan <- msg:
		default:
			log.Printf("[MQTT] msgChan full — QoS 1 state message dropped (topic=%s, cap=%d)", msg.Topic(), cap(c.msgChan))
		}
	}); token.Wait() && token.Error() != nil {
		log.Printf("mqtt subscribe state topic failed: %v", token.Error())
		return
	}
	log.Printf("mqtt subscribed to %s", c.stateTopic)

	if token := client.Subscribe(c.resultTopic, 1, func(_ paho.Client, msg paho.Message) {
		select {
		case c.msgChan <- msg:
		default:
			log.Printf("[MQTT] msgChan full — QoS 1 result message dropped (topic=%s, cap=%d)", msg.Topic(), cap(c.msgChan))
		}
	}); token.Wait() && token.Error() != nil {
		log.Printf("mqtt subscribe result topic failed: %v", token.Error())
		return
	}
	log.Printf("mqtt subscribed to %s", c.resultTopic)

	if token := client.Subscribe(c.otaStatusTopic, 1, func(_ paho.Client, msg paho.Message) {
		select {
		case c.msgChan <- msg:
		default:
			log.Printf("[MQTT] msgChan full — QoS 1 ota status message dropped (topic=%s, cap=%d)", msg.Topic(), cap(c.msgChan))
		}
	}); token.Wait() && token.Error() != nil {
		log.Printf("mqtt subscribe ota status topic failed: %v", token.Error())
		return
	}
	log.Printf("mqtt subscribed to %s", c.otaStatusTopic)
}

func (c *Client) handleStateMessage(message paho.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rawID, err := deviceIDFromTopic(c.stateTopic, message.Topic())
	if err != nil {
		log.Printf("[MQTT] presence ignored: %v", err)
		return
	}

	// 1. Resolve UUID
	deviceEntity, err := c.presenceService.GetDeviceByIDOrHardwareID(ctx, rawID)
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

	_, err = c.presenceService.RecordState(ctx, deviceID, device.PresenceInput{
		Status:    device.PresenceStatus(payload.Status),
		Timestamp: timestamp,
		Metadata:  payload.Metadata,
	}, message.Retained())
	if err != nil {
		log.Printf("[MQTT] presence update failed for device %s: %v", deviceID, err)
		return
	}
}

func (c *Client) handleResultMessage(message paho.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rawID, err := deviceIDFromTopic(c.resultTopic, message.Topic())
	if err != nil {
		return
	}

	device, err := c.presenceService.GetDeviceByIDOrHardwareID(ctx, rawID)
	if err != nil {
		log.Printf("[MQTT] failed to resolve device for result: %v", err)
		return
	}

	var payload resultMessage
	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Printf("[MQTT] result decode failed: %v", err)
		return
	}

	status := strings.ToLower(payload.Status)

	if payload.CommandID != "" {
		input := command.ResultInput{Message: payload.Message}
		if status == "ack" {
			c.commandService.Ack(ctx, device.ID, payload.CommandID, input)
		} else {
			c.commandService.Nack(ctx, device.ID, payload.CommandID, input)
		}
		log.Printf("[MQTT] recorded command %s as %s for device %s", payload.CommandID, status, device.ID)
	}

	if payload.DeploymentID != "" {
		input := ota.ResultInput{Message: payload.Message}
		if status == "ack" {
			c.otaService.Ack(ctx, device.ID, payload.DeploymentID, input)
		} else {
			c.otaService.Nack(ctx, device.ID, payload.DeploymentID, input)
		}
		log.Printf("[MQTT] recorded deployment %s as %s for device %s", payload.DeploymentID, status, device.ID)
	}
}

func (c *Client) handleOTAStatusMessage(message paho.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rawID, err := deviceIDFromTopic(c.otaStatusTopic, message.Topic())
	if err != nil {
		return
	}

	device, err := c.presenceService.GetDeviceByIDOrHardwareID(ctx, rawID)
	if err != nil {
		log.Printf("[MQTT] failed to resolve device for ota status: %v", err)
		return
	}

	var payload otaStatusMessage
	if err := json.Unmarshal(message.Payload(), &payload); err != nil {
		log.Printf("[MQTT] ota status decode failed: %v", err)
		return
	}

	deployment, err := c.otaService.RecordProgress(ctx, device.ID, ota.ProgressInput{
		DeploymentID: payload.DeploymentID,
		Status:       payload.Status,
		Progress:     payload.Progress,
		Message:      payload.Message,
	})
	if err != nil {
		log.Printf("[MQTT] ota status update failed: %v", err)
		return
	}
	log.Printf("[MQTT] recorded ota deployment %s as %s for device %s", deployment.ID, deployment.Status, device.ID)
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

