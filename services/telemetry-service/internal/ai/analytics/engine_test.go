package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/vishalss1/argus/telemetry/internal/domain/telemetry"
)

func TestEngine_Analyze(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to run miniredis: %v", err)
	}
	defer mr.Close()

	rdb := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	engine := NewEngine(rdb, nil)
	ctx := context.Background()

	// Set up workspace and active session in Redis
	deviceID := "device-1"
	workspaceID := "workspace-1"
	sessionID := "session-1"

	rdb.Set(ctx, fmt.Sprintf("device:%s:workspace", deviceID), workspaceID, 0)
	rdb.Set(ctx, fmt.Sprintf("workspace:%s:active_session", workspaceID), sessionID, 0)

	// Feed 10 normal telemetry messages
	for i := 0; i < 10; i++ {
		metrics := map[string]interface{}{
			"temperature": 25.0,
			"battery":     80.0,
		}
		metricsBytes, _ := json.Marshal(metrics)

		tel := telemetry.Telemetry{
			DeviceID:   deviceID,
			Metrics:    json.RawMessage(metricsBytes),
			RecordedAt: time.Now().Add(time.Duration(i) * time.Second),
			CreatedAt:  time.Now().Add(time.Duration(i) * time.Second),
		}

		err := engine.Analyze(ctx, tel)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Verify raw telemetry partitions are written in Redis
	hourStr := time.Now().UTC().Format("2006-01-02-15")
	activeHours, err := rdb.SMembers(ctx, fmt.Sprintf("session:%s:hours", sessionID)).Result()
	if err != nil {
		t.Fatalf("failed to get active hours: %v", err)
	}
	if len(activeHours) != 1 || activeHours[0] != hourStr {
		t.Errorf("expected active hour %s, got %v", hourStr, activeHours)
	}

	historyKey := fmt.Sprintf("session:%s:hour:%s:device:%s:telemetry_history", sessionID, hourStr, deviceID)
	cnt, err := rdb.ZCard(ctx, historyKey).Result()
	if err != nil {
		t.Fatalf("failed to get telemetry history card: %v", err)
	}
	if cnt != 10 {
		t.Errorf("expected ZSET count 10, got %d", cnt)
	}
}
