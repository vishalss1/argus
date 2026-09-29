package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/domain/telemetry"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCompactor_Compact(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to run miniredis: %v", err)
	}
	defer mr.Close()

	rdb := goredis.NewClient(&goredis.Options{
		Addr: mr.Addr(),
	})
	defer rdb.Close()

	compactor := NewCompactor(rdb, nil, 1*time.Minute)
	ctx := context.Background()

	sessionID := "session-1"
	deviceID := "device-1"
	expiredHour := "2020-01-01-12" // Way in the past, definitely expired

	// 1. Mark session active
	rdb.SAdd(ctx, "sessions:active", sessionID)

	// 2. Track expired hour
	rdb.SAdd(ctx, fmt.Sprintf("session:%s:hours", sessionID), expiredHour)

	// 3. Put raw telemetry in the expired hour's partition
	partitionKey := fmt.Sprintf("session:%s:hour:%s:device:%s:telemetry_history", sessionID, expiredHour, deviceID)

	for i := 0; i < 5; i++ {
		metrics := map[string]interface{}{
			"temperature": float64(20 + i), // 20, 21, 22, 23, 24 -> avg: 22, min: 20, max: 24, count: 5
		}
		metricsBytes, _ := json.Marshal(metrics)

		tel := telemetry.Telemetry{
			DeviceID:   deviceID,
			Metrics:    json.RawMessage(metricsBytes),
			RecordedAt: time.Date(2020, 1, 1, 12, i, 0, 0, time.UTC),
			CreatedAt:  time.Date(2020, 1, 1, 12, i, 0, 0, time.UTC),
		}
		telBytes, _ := json.Marshal(tel)

		rdb.ZAdd(ctx, partitionKey, goredis.Z{
			Score:  float64(tel.RecordedAt.UnixMilli()),
			Member: string(telBytes),
		})
	}

	// 4. Run Compact
	compactor.Compact(ctx)

	// 5. Raw telemetry must be RETAINED when the MinIO archive upload cannot be
	//    performed. The compactor is built with a nil MinIO client here, and it
	//    deliberately skips deletion rather than dropping the only copy of the
	//    data. This is the data-loss guard from 8651d66; this test previously
	//    asserted the opposite and had been failing since that fix landed.
	exists, err := rdb.Exists(ctx, partitionKey).Result()
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if exists == 0 {
		t.Errorf("expected partition key %s to be retained when MinIO is unavailable", partitionKey)
	}

	// 6. The hour must likewise stay in the hours set so the next compaction
	//    pass retries the archive instead of forgetting the hour.
	hours, err := rdb.SMembers(ctx, fmt.Sprintf("session:%s:hours", sessionID)).Result()
	if err != nil {
		t.Fatalf("failed to read hours: %v", err)
	}
	found := false
	for _, h := range hours {
		if h == expiredHour {
			found = true
		}
	}
	if !found {
		t.Errorf("expected hour %s to be retained in hours set when MinIO is unavailable", expiredHour)
	}

	// 7. Verify summaries are generated in Redis Hash
	summaryHashKey := fmt.Sprintf("session:%s:hourly_summaries", sessionID)
	field := fmt.Sprintf("device:%s:hour:%s", deviceID, expiredHour)
	summaryJSON, err := rdb.HGet(ctx, summaryHashKey, field).Result()
	if err != nil {
		t.Fatalf("failed to get summary hash field: %v", err)
	}

	var list pb.HourlySummaryList
	if err := protojson.Unmarshal([]byte(summaryJSON), &list); err != nil {
		t.Fatalf("failed to unmarshal summary list: %v", err)
	}

	if len(list.Summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(list.Summaries))
	}

	sum := list.Summaries[0]
	if sum.Metric != "temperature" {
		t.Errorf("expected metric temperature, got %s", sum.Metric)
	}
	if sum.SampleCount != 5 {
		t.Errorf("expected sample count 5, got %d", sum.SampleCount)
	}
	if sum.Min != 20 {
		t.Errorf("expected min 20, got %f", sum.Min)
	}
	if sum.Max != 24 {
		t.Errorf("expected max 24, got %f", sum.Max)
	}
	if sum.Average != 22 {
		t.Errorf("expected average 22, got %f", sum.Average)
	}
}
