package kafka

import (
	"context"
	"time"

	segmentio "github.com/segmentio/kafka-go"
)

type ConsumerConfig struct {
	Brokers      []string
	Topic        string
	GroupID      string
	MinBytes     int
	MaxBytes     int
	MaxWait      time.Duration
	QueueCapacity int
}

func (c ConsumerConfig) WithDefaults() ConsumerConfig {
	if c.MinBytes <= 0 {
		c.MinBytes = 1e3 // 1KB
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = 1e6 // 1MB
	}
	if c.MaxWait <= 0 {
		c.MaxWait = 100 * time.Millisecond
	}
	if c.QueueCapacity <= 0 {
		c.QueueCapacity = 10000
	}
	return c
}

type Consumer struct {
	reader *segmentio.Reader
}

func NewConsumer(config ConsumerConfig) *Consumer {
	config = config.WithDefaults()
	return &Consumer{
		reader: segmentio.NewReader(segmentio.ReaderConfig{
			Brokers:       config.Brokers,
			Topic:         config.Topic,
			GroupID:       config.GroupID,
			MinBytes:      config.MinBytes,
			MaxBytes:      config.MaxBytes,
			MaxWait:       config.MaxWait,
			QueueCapacity: config.QueueCapacity,
			StartOffset:   segmentio.LastOffset,
		}),
	}
}

func (c *Consumer) FetchMessage(ctx context.Context) (segmentio.Message, error) {
	return c.reader.FetchMessage(ctx)
}

func (c *Consumer) CommitMessages(ctx context.Context, msgs ...segmentio.Message) error {
	return c.reader.CommitMessages(ctx, msgs...)
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
