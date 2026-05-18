package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL         string
	Port                string
	MQTTBrokerURL       string
	MQTTClientID        string
	MQTTTelemetryTopic  string
	KafkaBrokers        []string
	KafkaTelemetryTopic string
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
		MQTTTelemetryTopic:  os.Getenv("MQTT_TELEMETRY_TOPIC"),
		KafkaBrokers:        splitCSV(os.Getenv("KAFKA_BROKERS")),
		KafkaTelemetryTopic: os.Getenv("KAFKA_TELEMETRY_TOPIC"),
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
	if cfg.MQTTTelemetryTopic == "" {
		cfg.MQTTTelemetryTopic = "argus/devices/+/telemetry"
	}
	if cfg.KafkaTelemetryTopic == "" {
		cfg.KafkaTelemetryTopic = "argus.telemetry"
	}

	return cfg
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
