package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL         string
	Port                string
	MQTTBrokerURL       string
	MQTTClientID        string
	MQTTStateTopic      string
	MQTTTelemetryTopic  string
	KafkaBrokers        []string
	KafkaTelemetryTopic string
	KafkaCommandTopic   string
	KafkaAIWorkerGroupID string
	RedisAddr           string
	RedisPassword       string
	RedisDB             int
	MinIOEndpoint       string
	MinIOAccessKey      string
	MinIOSecretKey      string
	MinIOBucket         string
	MinIOUseSSL         bool
	HeartbeatTimeout    time.Duration
	HeartbeatInterval   time.Duration
	ProvisioningBroker  string
	GroqAPIKey          string
	GroqModel           string
	GroqBaseURL         string
	OllamaBaseURL       string
	OllamaEmbedModel    string
	OllamaReasonModel   string
}

func Load() *Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Println(".env file not found (continuing)")
	}

	cfg := &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Port:                os.Getenv("PORT"),
		MQTTBrokerURL:       os.Getenv("MQTT_BROKER_URL"),
		MQTTClientID:        os.Getenv("MQTT_CLIENT_ID"),
		MQTTStateTopic:      os.Getenv("MQTT_STATE_TOPIC"),
		MQTTTelemetryTopic:  os.Getenv("MQTT_TELEMETRY_TOPIC"),
		KafkaBrokers:        splitCSV(os.Getenv("KAFKA_BROKERS")),
		KafkaTelemetryTopic: os.Getenv("KAFKA_TELEMETRY_TOPIC"),
		KafkaCommandTopic:   os.Getenv("KAFKA_COMMAND_TOPIC"),
		RedisAddr:           os.Getenv("REDIS_ADDR"),
		RedisPassword:       os.Getenv("REDIS_PASSWORD"),
		MinIOEndpoint:       os.Getenv("MINIO_ENDPOINT"),
		MinIOAccessKey:      os.Getenv("MINIO_ACCESS_KEY"),
		MinIOSecretKey:      os.Getenv("MINIO_SECRET_KEY"),
		MinIOBucket:         os.Getenv("MINIO_BUCKET"),
		GroqAPIKey:          os.Getenv("GROQ_API_KEY"),
		GroqModel:           os.Getenv("GROQ_MODEL"),
		GroqBaseURL:         os.Getenv("GROQ_BASE_URL"),
		OllamaBaseURL:       os.Getenv("OLLAMA_BASE_URL"),
		OllamaEmbedModel:    os.Getenv("OLLAMA_EMBED_MODEL"),
		OllamaReasonModel:   os.Getenv("OLLAMA_REASON_MODEL"),
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
		cfg.MQTTStateTopic = "devices/+/state"
	}
	if cfg.MQTTTelemetryTopic == "" {
		cfg.MQTTTelemetryTopic = "devices/+/telemetry"
	}
	if cfg.KafkaTelemetryTopic == "" {
		cfg.KafkaTelemetryTopic = "argus.telemetry"
	}
	if cfg.KafkaCommandTopic == "" {
		cfg.KafkaCommandTopic = "argus.commands"
	}
	cfg.KafkaAIWorkerGroupID = os.Getenv("KAFKA_AI_WORKER_GROUP_ID")
	if cfg.KafkaAIWorkerGroupID == "" {
		cfg.KafkaAIWorkerGroupID = "argus-ai-worker"
	}
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}
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
	if cfg.OllamaReasonModel == "" {
		cfg.OllamaReasonModel = "qwen2.5:7b-instruct"
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
