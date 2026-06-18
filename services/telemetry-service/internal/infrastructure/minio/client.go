package minio

import (
	"context"
	"fmt"
	"io"
	"log"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Client struct {
	client *miniogo.Client
	bucket string
}

func NewClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	client, err := miniogo.New(endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	if bucket == "" {
		bucket = "argus-firmware"
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, miniogo.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create minio bucket: %w", err)
		}
	}

	log.Printf("[MINIO] telemetry service connected: endpoint=%s bucket=%s", endpoint, bucket)

	return &Client{client: client, bucket: bucket}, nil
}

func (c *Client) PutObject(ctx context.Context, objectKey string, reader io.Reader, sizeBytes int64, contentType string) error {
	_, err := c.client.PutObject(ctx, c.bucket, objectKey, reader, sizeBytes, miniogo.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("minio put object: %w", err)
	}
	return nil
}

func (c *Client) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, objectKey, miniogo.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio get object: %w", err)
	}
	return obj, nil
}

func (c *Client) DeleteObject(ctx context.Context, objectKey string) error {
	err := c.client.RemoveObject(ctx, c.bucket, objectKey, miniogo.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
