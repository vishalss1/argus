package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
}

func Load() *Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Println(".env file not found (continuing)")
	}

	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        os.Getenv("PORT"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg
}
