package health

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/vishalss1/argus/core/internal/infrastructure/mqtt"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
)

func CheckDependencies(ctx context.Context, db *sql.DB, redisClient *redis.Client, mqttClient *mqtt.Client) error {
	var errs []string

	if err := db.PingContext(ctx); err != nil {
		errs = append(errs, "postgres: "+err.Error())
	}
	
	if err := redisClient.Client().Ping(ctx).Err(); err != nil {
		errs = append(errs, "redis: "+err.Error())
	}
	
	if mqttClient != nil && !mqttClient.IsConnected() {
		errs = append(errs, "mqtt: not connected")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}
