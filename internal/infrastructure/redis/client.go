package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Addr     string
	Password string
	DB       int
}

type Client struct {
	client *goredis.Client
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	return &Client{client: client}, nil
}

func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}

	return c.client.Close()
}
