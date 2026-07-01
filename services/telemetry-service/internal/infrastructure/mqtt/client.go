package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"github.com/vishalss1/argus/telemetry/internal/domain/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/kafka"
)

type Config struct {
	BrokerURL      string
	ClientID       string
	TelemetryTopic string
}

type Client struct {
	client           paho.Client
	cfg              Config
	telemetryService *telemetry.Service
	kafkaProducer    *kafka.Producer
	logger           *zap.Logger
	messageCh        chan paho.Message
	mu               sync.Mutex
	wg               sync.WaitGroup
	done             chan struct{}
}

func New(cfg Config, telemetryService *telemetry.Service, kafkaProducer *kafka.Producer, logger *zap.Logger) (*Client, error) {
	messageCh := make(chan paho.Message, 65536)

	opts := paho.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetConnectionLostHandler(func(client paho.Client, err error) {
			logger.Error("MQTT connection lost", zap.Error(err))
		}).
		SetOnConnectHandler(func(client paho.Client) {
			logger.Info("MQTT connected", zap.String("broker", cfg.BrokerURL))
		})

	client := paho.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", token.Error())
	}

	return &Client{
		client:           client,
		cfg:              cfg,
		telemetryService: telemetryService,
		kafkaProducer:    kafkaProducer,
		logger:           logger,
		messageCh:        messageCh,
		done:             make(chan struct{}),
	}, nil
}

func (c *Client) Start() error {
	// ponytail: subscribe QoS 0 for telemetry ingestion
	token := c.client.Subscribe(c.cfg.TelemetryTopic, 0, c.messageHandler)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("mqtt subscribe: %w", token.Error())
	}

	c.logger.Info("MQTT subscribed", zap.String("topic", c.cfg.TelemetryTopic))

	// ponytail: minimum viable workers, match kafka default
	for i := 0; i < 64; i++ {
		c.wg.Add(1)
		go c.worker()
	}

	return nil
}

func (c *Client) messageHandler(client paho.Client, msg paho.Message) {
	select {
	case c.messageCh <- msg:
	default:
		c.logger.Warn("MQTT message channel full, dropping message",
			zap.String("topic", msg.Topic()))
	}
}

func (c *Client) worker() {
	defer c.wg.Done()
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.messageCh:
			c.handleMessage(msg)
		}
	}
}

func (c *Client) handleMessage(msg paho.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Extract device ID from topic: "argus/devices/{deviceID}/telemetry"
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 3 {
		c.logger.Warn("invalid topic format", zap.String("topic", msg.Topic()))
		return
	}
	deviceID := parts[2]

	var payload struct {
		RecordedAt *time.Time      `json:"recorded_at"`
		Metrics    json.RawMessage `json:"metrics"`
	}
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		c.logger.Warn("telemetry decode failed", zap.String("device", deviceID), zap.Error(err))
		return
	}

	input := telemetry.CreateInput{
		RecordedAt: payload.RecordedAt,
		Metrics:    payload.Metrics,
	}

	entity, err := c.telemetryService.Ingest(ctx, deviceID, input)
	if err != nil {
		c.logger.Warn("failed to ingest telemetry",
			zap.String("device_id", deviceID), zap.Error(err))
		return
	}

	if c.kafkaProducer != nil && entity != nil {
		if err := c.kafkaProducer.PublishTelemetry(ctx, *entity); err != nil {
			c.logger.Warn("failed to publish telemetry to kafka",
				zap.String("device_id", deviceID), zap.Error(err))
		}
	}
}

func (c *Client) Close() {
	close(c.done)
	c.client.Disconnect(1000)
	c.wg.Wait()
}
