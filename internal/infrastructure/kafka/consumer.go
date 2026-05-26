package kafka

import (
	"context"

	segmentio "github.com/segmentio/kafka-go"
)

type ConsumerConfig struct {
	Brokers []string
	Topic   string
	GroupID string
}

type Consumer struct {
	reader *segmentio.Reader
}

func NewConsumer(config ConsumerConfig) *Consumer {
	return &Consumer{
		reader: segmentio.NewReader(segmentio.ReaderConfig{
			Brokers:  config.Brokers,
			Topic:    config.Topic,
			GroupID:  config.GroupID,
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
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
