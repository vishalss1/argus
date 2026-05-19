package minio

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
}

type Client struct {
	client *miniogo.Client
	bucket string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("minio endpoint is required")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("minio access key is required")
	}
	if cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("minio secret key is required")
	}
	if cfg.Bucket == "" {
		cfg.Bucket = "argus-firmware"
	}

	client, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, miniogo.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create minio bucket: %w", err)
		}
	}

	return &Client{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (c *Client) PutFirmware(ctx context.Context, objectKey string, reader io.Reader, sizeBytes int64, contentType string) error {
	_, err := c.client.PutObject(ctx, c.bucket, objectKey, reader, sizeBytes, miniogo.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put firmware object: %w", err)
	}

	return nil
}

func (c *Client) FirmwareURL(ctx context.Context, objectKey string, filename string, expires time.Duration) (string, error) {
	query := make(url.Values)
	if filename != "" {
		query.Set("response-content-disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	}

	url, err := c.client.PresignedGetObject(ctx, c.bucket, objectKey, expires, query)
	if err != nil {
		return "", fmt.Errorf("sign firmware url: %w", err)
	}

	return url.String(), nil
}
