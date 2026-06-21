package config

import (
	"encoding/base64"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL          string
	Port                 string
	MQTTBrokerURL        string
	MQTTClientID         string
	MQTTStateTopic       string
	MQTTTelemetryTopic   string
	KafkaBrokers         []string
	KafkaTelemetryTopic  string
	KafkaCommandTopic    string
	KafkaAIWorkerGroupID string
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	MinIOEndpoint        string
	MinIOPublicURL       string
	MinIOAccessKey       string
	MinIOSecretKey       string
	MinIOBucket          string
	MinIOUseSSL          bool
	OTARequireSignatures bool
	OTASigningKeyID      string
	OTASigningPrivateKey string
	HTTPSTLSCertFile     string
	HTTPSTLSKeyFile      string
	HeartbeatTimeout     time.Duration
	HeartbeatInterval    time.Duration
	ProvisioningBroker   string
	GroqAPIKey           string
	GroqModel            string
	GroqBaseURL          string
	OllamaBaseURL        string
	OllamaEmbedModel     string
	WorkerProfiles       string
	AlertCooldownSeconds int
	SessionStaleTimeoutHours int
	KafkaDLQTopic        string
	KafkaIncidentTopic   string
	RAGSimilarityThreshold float32
	EmbeddingQueueSize   int
	EmbeddingWorkers     int
	AIQueryAPIKey        string
	AIQueryRateLimit     int
	AIRetentionDays      int
	JWTSecret                 string
	JWTAccessExpiration       time.Duration
	JWTRefreshExpiration      time.Duration
	AuthAuditLogRetentionDays int
	TelemetryExportRetentionDays int
	DeviceCARootCertFile      string
	DeviceCAPrivateKeyFile    string
	ServerCACertFile          string
	OTASigningPublicKeyB64    string
	ServerHost                string
}

func Load() *Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Println(".env file not found (continuing)")
	}

	cfg := &Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		Port:                 os.Getenv("PORT"),
		MQTTBrokerURL:        os.Getenv("MQTT_BROKER_URL"),
		MQTTClientID:         os.Getenv("MQTT_CLIENT_ID"),
		MQTTStateTopic:       os.Getenv("MQTT_STATE_TOPIC"),
		MQTTTelemetryTopic:   os.Getenv("MQTT_TELEMETRY_TOPIC"),
		KafkaBrokers:         splitCSV(os.Getenv("KAFKA_BROKERS")),
		KafkaTelemetryTopic:  os.Getenv("KAFKA_TELEMETRY_TOPIC"),
		KafkaCommandTopic:    os.Getenv("KAFKA_COMMAND_TOPIC"),
		KafkaDLQTopic:        os.Getenv("KAFKA_DLQ_TOPIC"),
		KafkaIncidentTopic:   os.Getenv("KAFKA_INCIDENT_TOPIC"),
		RedisAddr:            os.Getenv("REDIS_ADDR"),
		RedisPassword:        os.Getenv("REDIS_PASSWORD"),
		MinIOEndpoint:        os.Getenv("MINIO_ENDPOINT"),
		MinIOPublicURL:       os.Getenv("MINIO_PUBLIC_URL"),
		MinIOAccessKey:       os.Getenv("MINIO_ACCESS_KEY"),
		MinIOSecretKey:       os.Getenv("MINIO_SECRET_KEY"),
		MinIOBucket:          os.Getenv("MINIO_BUCKET"),
		OTASigningKeyID:      os.Getenv("OTA_SIGNING_KEY_ID"),
		OTASigningPrivateKey: os.Getenv("OTA_SIGNING_PRIVATE_KEY_B64"),
		HTTPSTLSCertFile:     os.Getenv("HTTPS_TLS_CERT_FILE"),
		HTTPSTLSKeyFile:      os.Getenv("HTTPS_TLS_KEY_FILE"),
		GroqAPIKey:           os.Getenv("GROQ_API_KEY"),
		GroqModel:            os.Getenv("GROQ_MODEL"),
		GroqBaseURL:          os.Getenv("GROQ_BASE_URL"),
		OllamaBaseURL:        os.Getenv("OLLAMA_BASE_URL"),
		OllamaEmbedModel:     os.Getenv("OLLAMA_EMBED_MODEL"),
		WorkerProfiles:       os.Getenv("WORKER_PROFILES"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.MQTTClientID == "" {
		cfg.MQTTClientID = "argus-api"
	}
	if cfg.MQTTStateTopic == "" {
		cfg.MQTTStateTopic = "argus/devices/+/state"
	}
	if cfg.MQTTTelemetryTopic == "" {
		cfg.MQTTTelemetryTopic = "argus/devices/+/telemetry"
	}
	if cfg.KafkaTelemetryTopic == "" {
		cfg.KafkaTelemetryTopic = "argus.telemetry"
	}
	if cfg.KafkaCommandTopic == "" {
		cfg.KafkaCommandTopic = "argus.commands"
	}
	if cfg.KafkaDLQTopic == "" {
		cfg.KafkaDLQTopic = "argus.dlq"
	}
	if cfg.KafkaIncidentTopic == "" {
		cfg.KafkaIncidentTopic = "telemetry.incidents"
	}
	cfg.KafkaAIWorkerGroupID = os.Getenv("KAFKA_AI_WORKER_GROUP_ID")
	if cfg.KafkaAIWorkerGroupID == "" {
		cfg.KafkaAIWorkerGroupID = "argus-ai-worker"
	}

	if cfg.WorkerProfiles == "" {
		cfg.WorkerProfiles = "all"
	}

	cooldownStr := os.Getenv("ALERT_COOLDOWN_SECONDS")
	if cooldownStr != "" {
		parsed, err := strconv.Atoi(cooldownStr)
		if err == nil && parsed >= 0 {
			cfg.AlertCooldownSeconds = parsed
		}
	} else {
		cfg.AlertCooldownSeconds = 900 // 15 minutes default
	}

	staleStr := os.Getenv("SESSION_STALE_TIMEOUT_HOURS")
	if staleStr != "" {
		parsed, err := strconv.Atoi(staleStr)
		if err == nil && parsed > 0 {
			cfg.SessionStaleTimeoutHours = parsed
		}
	} else {
		cfg.SessionStaleTimeoutHours = 24 // 24 hours default
	}

	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}
	log.Printf("Loaded Redis Address: %s", cfg.RedisAddr)
	if redisDB := strings.TrimSpace(os.Getenv("REDIS_DB")); redisDB != "" {
		parsed, err := strconv.Atoi(redisDB)
		if err != nil {
			log.Fatal("REDIS_DB must be an integer")
		}
		cfg.RedisDB = parsed
	}
	if cfg.MinIOEndpoint == "" {
		cfg.MinIOEndpoint = "localhost:9000"
	}
	if cfg.MinIOAccessKey == "" {
		cfg.MinIOAccessKey = "argus"
	}
	if cfg.MinIOSecretKey == "" {
		cfg.MinIOSecretKey = "arguspassword"
	}
	if cfg.MinIOBucket == "" {
		cfg.MinIOBucket = "argus-firmware"
	}
	cfg.MinIOUseSSL = parseBool(os.Getenv("MINIO_USE_SSL"))
	cfg.OTARequireSignatures = parseBool(os.Getenv("OTA_REQUIRE_SIGNATURES"))
	cfg.HeartbeatTimeout = parseDurationSeconds("HEARTBEAT_TIMEOUT_SECONDS", 45)
	if strings.TrimSpace(os.Getenv("HEARTBEAT_INTERVAL_SECONDS")) != "" {
		cfg.HeartbeatInterval = parseDurationSeconds("HEARTBEAT_INTERVAL_SECONDS", 30)
	} else {
		cfg.HeartbeatInterval = parseDurationSeconds("HEARTBEAT_MONITOR_INTERVAL_SECONDS", 30)
	}
	cfg.ProvisioningBroker = os.Getenv("PROVISIONING_MQTT_BROKER_URL")
	if cfg.ProvisioningBroker == "" {
		cfg.ProvisioningBroker = cfg.MQTTBrokerURL
	}

	if cfg.GroqModel == "" {
		cfg.GroqModel = "llama-3.3-70b-versatile"
	}
	if cfg.GroqBaseURL == "" {
		cfg.GroqBaseURL = "https://api.groq.com/openai/v1"
	}

	if cfg.OllamaBaseURL == "" {
		cfg.OllamaBaseURL = "http://localhost:11434"
	}
	if cfg.OllamaEmbedModel == "" {
		cfg.OllamaEmbedModel = "nomic-embed-text"
	}

	thresholdStr := os.Getenv("RAG_SIMILARITY_THRESHOLD")
	if thresholdStr != "" {
		if val, err := strconv.ParseFloat(thresholdStr, 32); err == nil {
			cfg.RAGSimilarityThreshold = float32(val)
		} else {
			cfg.RAGSimilarityThreshold = 0.5
		}
	} else {
		cfg.RAGSimilarityThreshold = 0.5
	}

	queueSizeStr := os.Getenv("EMBEDDING_QUEUE_SIZE")
	if queueSizeStr != "" {
		if val, err := strconv.Atoi(queueSizeStr); err == nil && val > 0 {
			cfg.EmbeddingQueueSize = val
		} else {
			cfg.EmbeddingQueueSize = 10000
		}
	} else {
		cfg.EmbeddingQueueSize = 10000
	}

	workersStr := os.Getenv("EMBEDDING_WORKERS")
	if workersStr != "" {
		if val, err := strconv.Atoi(workersStr); err == nil && val > 0 {
			cfg.EmbeddingWorkers = val
		} else {
			cfg.EmbeddingWorkers = 8
		}
	} else {
		cfg.EmbeddingWorkers = 8
	}

	cfg.AIQueryAPIKey = os.Getenv("AI_QUERY_API_KEY")

	rateLimitStr := os.Getenv("AI_QUERY_RATE_LIMIT")
	if rateLimitStr != "" {
		if val, err := strconv.Atoi(rateLimitStr); err == nil && val >= 0 {
			cfg.AIQueryRateLimit = val
		} else {
			cfg.AIQueryRateLimit = 10 // default 10 requests per minute
		}
	} else {
		cfg.AIQueryRateLimit = 10
	}

	retentionDaysStr := os.Getenv("AI_RETENTION_DAYS")
	if retentionDaysStr != "" {
		if val, err := strconv.Atoi(retentionDaysStr); err == nil && val > 0 {
			cfg.AIRetentionDays = val
		} else {
			cfg.AIRetentionDays = 30 // default 30 days
		}
	} else {
		cfg.AIRetentionDays = 30
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "default-super-secure-secret-key-change-this-in-prod"
	}

	cfg.JWTAccessExpiration = parseDurationWithDays("JWT_ACCESS_EXPIRY", 15*time.Minute)
	cfg.JWTRefreshExpiration = parseDurationWithDays("JWT_REFRESH_EXPIRY", 30*24*time.Hour)

	auditRetentionDaysStr := os.Getenv("AUTH_AUDIT_LOG_RETENTION_DAYS")
	if auditRetentionDaysStr != "" {
		if val, err := strconv.Atoi(auditRetentionDaysStr); err == nil && val > 0 {
			cfg.AuthAuditLogRetentionDays = val
		} else {
			cfg.AuthAuditLogRetentionDays = 90
		}
	} else {
		cfg.AuthAuditLogRetentionDays = 90
	}

	telemetryRetentionDaysStr := os.Getenv("ARGUS_TELEMETRY_EXPORT_RETENTION_DAYS")
	if telemetryRetentionDaysStr != "" {
		if val, err := strconv.Atoi(telemetryRetentionDaysStr); err == nil && val > 0 {
			cfg.TelemetryExportRetentionDays = val
		} else {
			cfg.TelemetryExportRetentionDays = 7
		}
	} else {
		cfg.TelemetryExportRetentionDays = 7
	}

	cfg.DeviceCARootCertFile = os.Getenv("DEVICE_CA_ROOT_CERT_FILE")
	if cfg.DeviceCARootCertFile == "" {
		cfg.DeviceCARootCertFile = "certs/root-ca.pem"
	}

	cfg.DeviceCAPrivateKeyFile = os.Getenv("DEVICE_CA_PRIVATE_KEY_FILE")
	if cfg.DeviceCAPrivateKeyFile == "" {
		cfg.DeviceCAPrivateKeyFile = "certs/root-ca.key"
	}

	cfg.ServerCACertFile = os.Getenv("SERVER_CA_CERT_FILE")
	if cfg.ServerCACertFile == "" {
		cfg.ServerCACertFile = "certs/ca.pem"
	}

	cfg.ServerHost = os.Getenv("ARGUS_SERVER_HOST")
	if cfg.ServerHost == "" {
		cfg.ServerHost = "localhost"
	}

	if cfg.OTASigningPrivateKey != "" {
		privBytes, err := base64.StdEncoding.DecodeString(cfg.OTASigningPrivateKey)
		if err == nil && len(privBytes) == 64 {
			pubKeyBytes := privBytes[32:]
			cfg.OTASigningPublicKeyB64 = base64.StdEncoding.EncodeToString(pubKeyBytes)
		} else {
			log.Println("Warning: invalid OTA_SIGNING_PRIVATE_KEY_B64 length or decoding error:", err)
		}
	}

	return cfg
}

func parseDurationSeconds(key string, defaultSeconds int) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return time.Duration(defaultSeconds) * time.Second
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		log.Fatalf("%s must be a positive integer", key)
	}

	return time.Duration(seconds) * time.Second
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}

	return values
}

func parseDurationWithDays(key string, defaultDuration time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultDuration
	}

	if strings.HasSuffix(value, "d") {
		daysStr := strings.TrimSuffix(value, "d")
		days, err := strconv.Atoi(daysStr)
		if err == nil && days > 0 {
			return time.Duration(days) * 24 * time.Hour
		}
	}

	parsed, err := time.ParseDuration(value)
	if err == nil && parsed > 0 {
		return parsed
	}

	log.Printf("Warning: invalid duration for %s, using default %v", key, defaultDuration)
	return defaultDuration
}

