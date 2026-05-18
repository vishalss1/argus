package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	Port               string
	MQTTBrokerURL      string
	MQTTClientID       string
	MQTTTelemetryTopic string
}

func Load() *Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Println(".env file not found (continuing)")
	}

	cfg := &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		Port:               os.Getenv("PORT"),
		MQTTBrokerURL:      os.Getenv("MQTT_BROKER_URL"),
		MQTTClientID:       os.Getenv("MQTT_CLIENT_ID"),
		MQTTTelemetryTopic: os.Getenv("MQTT_TELEMETRY_TOPIC"),
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

	return cfg
}
