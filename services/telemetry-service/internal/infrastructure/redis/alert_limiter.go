package redis

import (
	"context"
	"fmt"
)

type AlertLimiter struct {
	client *Client
}

func NewAlertLimiter(client *Client) *AlertLimiter {
	return &AlertLimiter{client: client}
}

func (l *AlertLimiter) Allow(ctx context.Context, ruleID string, deviceID string) bool {
	cooldownKey := fmt.Sprintf("alert:cooldown:%s:%s", ruleID, deviceID)
	exists, err := l.client.Client().Exists(ctx, cooldownKey).Result()
	if err != nil {
		// If Redis has an error, default to allowing to avoid missing critical alerts
		return true
	}
	return exists == 0
}
