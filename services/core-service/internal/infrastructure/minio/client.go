package minio

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint        string
	PublicURL       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
}

type Client struct {
	client        *miniogo.Client
	presignClient *miniogo.Client
	bucket        string
	endpoint      string
	publicURL     string
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

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:     credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure:    cfg.UseSSL,
		Transport: transport,
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

	publicEndpoint, publicSecure, publicURL, err := publicMinIOEndpoint(cfg)
	if err != nil {
		return nil, err
	}

	presignClient := client
	if publicEndpoint != cfg.Endpoint || publicSecure != cfg.UseSSL {
		// Use a custom transport to skip verification for the local self-signed proxy
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		presignClient, err = miniogo.New(publicEndpoint, &miniogo.Options{
			Creds:     credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
			Secure:    publicSecure,
			Transport: transport,
		})
		if err != nil {
			return nil, fmt.Errorf("create public minio presign client: %w", err)
		}
	}

	log.Printf("[MINIO] endpoint=%s bucket=%s use_ssl=%t", cfg.Endpoint, cfg.Bucket, cfg.UseSSL)
	log.Printf("[MINIO] public_url=%s", publicURL)
	if strings.Contains(publicURL, "://localhost") || strings.Contains(publicURL, "://127.0.0.1") {
		log.Printf("[MINIO] warning: public_url=%s is not reachable by external ESP32 devices", publicURL)
	}

	return &Client{
		client:        client,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
		endpoint:      cfg.Endpoint,
		publicURL:     publicURL,
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

	url, err := c.presignClient.PresignedGetObject(ctx, c.bucket, objectKey, expires, query)
	if err != nil {
		return "", fmt.Errorf("sign firmware url: %w", err)
	}

	log.Printf("[OTA] generated download_url=%s", url.String())
	return url.String(), nil
}

func (c *Client) GetFirmware(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, objectKey, miniogo.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get firmware object: %w", err)
	}
	return obj, nil
}

func (c *Client) RemoveFirmware(ctx context.Context, objectKey string) error {
	err := c.client.RemoveObject(ctx, c.bucket, objectKey, miniogo.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("remove firmware object: %w", err)
	}
	return nil
}

func publicMinIOEndpoint(cfg Config) (endpoint string, secure bool, publicURL string, err error) {
	rawPublicURL := strings.TrimSpace(cfg.PublicURL)
	if rawPublicURL == "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		return cfg.Endpoint, cfg.UseSSL, scheme + "://" + cfg.Endpoint, nil
	}

	parsed, err := url.Parse(rawPublicURL)
	if err != nil {
		return "", false, "", fmt.Errorf("parse MINIO_PUBLIC_URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false, "", fmt.Errorf("MINIO_PUBLIC_URL must start with http:// or https://")
	}
	if parsed.Host == "" {
		return "", false, "", fmt.Errorf("MINIO_PUBLIC_URL must include host")
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.Host, parsed.Scheme == "https", strings.TrimRight(parsed.String(), "/"), nil
}

