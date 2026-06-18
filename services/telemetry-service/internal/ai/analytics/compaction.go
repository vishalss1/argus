package analytics

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/domain/telemetry"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/minio"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Compactor struct {
	redisClient *goredis.Client
	minioClient *minio.Client
	interval    time.Duration
}

func NewCompactor(redisClient *goredis.Client, minioClient *minio.Client, interval time.Duration) *Compactor {
	return &Compactor{
		redisClient: redisClient,
		minioClient: minioClient,
		interval:    interval,
	}
}

func (c *Compactor) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				c.Compact(ctx)
			}
		}
	}()
}

func (c *Compactor) Compact(ctx context.Context) {
	sessions, err := c.redisClient.SMembers(ctx, "sessions:active").Result()
	if err != nil || len(sessions) == 0 {
		return
	}

	nowHour := time.Now().UTC().Format("2006-01-02-15")

	for _, sessionID := range sessions {
		hours, err := c.redisClient.SMembers(ctx, fmt.Sprintf("session:%s:hours", sessionID)).Result()
		if err != nil {
			continue
		}

		for _, hour := range hours {
			if hour >= nowHour {
				continue // Skip the current or future hours
			}

			// Find all device keys matching: session:<session_id>:hour:<hour>:device:*:telemetry_history
			var keys []string
			var cursor uint64
			for {
				var scanKeys []string
				var scanErr error
				matchPattern := fmt.Sprintf("session:%s:hour:%s:device:*:telemetry_history", sessionID, hour)
				scanKeys, cursor, scanErr = c.redisClient.Scan(ctx, cursor, matchPattern, 100).Result()
				if scanErr != nil {
					break
				}
				keys = append(keys, scanKeys...)
				if cursor == 0 {
					break
				}
			}

			// Accumulate all samples for the hour for MinIO archiving
			type archiveSample struct {
				Timestamp string             `json:"timestamp"`
				DeviceID  string             `json:"device_id"`
				Metrics   map[string]float64 `json:"metrics"`
			}
			var hourSamples []archiveSample
			allMetricKeys := make(map[string]bool)

			for _, key := range keys {
				parts := strings.Split(key, ":")
				if len(parts) < 6 {
					continue
				}
				deviceID := parts[5]

				items, err := c.redisClient.ZRange(ctx, key, 0, -1).Result()
				if err != nil || len(items) == 0 {
					continue
				}

				type metricTracker struct {
					vals  []float64
					times []time.Time
				}
				trackers := make(map[string]*metricTracker)

				for _, item := range items {
					var sample telemetry.Telemetry
					if err := json.Unmarshal([]byte(item), &sample); err != nil {
						continue
					}

					var metrics map[string]interface{}
					if err := json.Unmarshal(sample.Metrics, &metrics); err != nil {
						continue
					}

					numerics := make(map[string]float64)
					binaries := make(map[string]bool)
					categoricals := make(map[string]string)
					flattenMetrics(metrics, "", numerics, binaries, categoricals)

					recordedAt := sample.RecordedAt
					if recordedAt.IsZero() {
						recordedAt = sample.CreatedAt
					}
					if recordedAt.IsZero() {
						recordedAt = time.Now().UTC()
					}

					for mName, mVal := range numerics {
						tr, ok := trackers[mName]
						if !ok {
							tr = &metricTracker{}
							trackers[mName] = tr
						}
						tr.vals = append(tr.vals, mVal)
						tr.times = append(tr.times, recordedAt)
					}

					// Collect for archive
					flatMetrics := make(map[string]float64)
					for k, v := range numerics {
						flatMetrics[k] = v
						allMetricKeys[k] = true
					}
					ts := recordedAt.UTC().Format(time.RFC3339)
					hourSamples = append(hourSamples, archiveSample{
						Timestamp: ts,
						DeviceID:  deviceID,
						Metrics:   flatMetrics,
					})
				}

				var summaries []*pb.HourlySummary
				for mName, tr := range trackers {
					n := len(tr.vals)
					if n == 0 {
						continue
					}
					minVal := tr.vals[0]
					maxVal := tr.vals[0]
					sumVal := 0.0
					for _, v := range tr.vals {
						if v < minVal {
							minVal = v
						}
						if v > maxVal {
							maxVal = v
						}
						sumVal += v
					}
					avg := sumVal / float64(n)
					varSum := 0.0
					for _, v := range tr.vals {
						varSum += (v - avg) * (v - avg)
					}
					variance := varSum / float64(n)
					stdDev := math.Sqrt(variance)

					firstTs := tr.times[0]
					lastTs := tr.times[n-1]

					summaries = append(summaries, &pb.HourlySummary{
						DeviceId:       deviceID,
						Hour:           hour,
						Metric:         mName,
						SampleCount:    int32(n),
						Min:            minVal,
						Max:            maxVal,
						Average:        avg,
						Variance:       variance,
						Stddev:         stdDev,
						FirstTimestamp: timestamppb.New(firstTs),
						LastTimestamp:  timestamppb.New(lastTs),
					})
				}

				if len(summaries) > 0 {
					listMsg := &pb.HourlySummaryList{Summaries: summaries}
					data, err := protojson.Marshal(listMsg)
					if err == nil {
						field := fmt.Sprintf("device:%s:hour:%s", deviceID, hour)
						c.redisClient.HSet(ctx, fmt.Sprintf("session:%s:hourly_summaries", sessionID), field, string(data)).Err()
					}
				}

				// Delete the raw hourly key
				c.redisClient.Del(ctx, key).Err()
			}

			// Archive accumulated hour samples to MinIO before removing from hours set
			if len(hourSamples) > 0 && c.minioClient != nil {
				sort.Slice(hourSamples, func(i, j int) bool {
					return hourSamples[i].Timestamp < hourSamples[j].Timestamp
				})

				prefix := fmt.Sprintf("session-exports/%s", sessionID)
				jsonKey := fmt.Sprintf("%s/telemetry-%s.json.gz", prefix, hour)
				csvKey := fmt.Sprintf("%s/telemetry-%s.csv.gz", prefix, hour)

				// Build JSON
				jsonData, _ := json.Marshal(hourSamples)

				// Build CSV
				var csvBuf bytes.Buffer
				csvWriter := csv.NewWriter(&csvBuf)
				var metricKeys []string
				for k := range allMetricKeys {
					metricKeys = append(metricKeys, k)
				}
				sort.Strings(metricKeys)
				header := append([]string{"timestamp", "device_id"}, metricKeys...)
				_ = csvWriter.Write(header)
				for _, s := range hourSamples {
					row := []string{s.Timestamp, s.DeviceID}
					for _, mk := range metricKeys {
						row = append(row, strconv.FormatFloat(s.Metrics[mk], 'f', -1, 64))
					}
					_ = csvWriter.Write(row)
				}
				csvWriter.Flush()

				// Upload JSON.gz
				{
					var gzBuf bytes.Buffer
					gzWriter := gzip.NewWriter(&gzBuf)
					_, _ = gzWriter.Write(jsonData)
					_ = gzWriter.Close()
					_ = c.minioClient.PutObject(ctx, jsonKey, &gzBuf, int64(gzBuf.Len()), "application/gzip")
				}

				// Upload CSV.gz
				{
					var gzBuf bytes.Buffer
					gzWriter := gzip.NewWriter(&gzBuf)
					_, _ = gzWriter.Write(csvBuf.Bytes())
					_ = gzWriter.Close()
					_ = c.minioClient.PutObject(ctx, csvKey, &gzBuf, int64(gzBuf.Len()), "application/gzip")
				}

				// Append to export manifest
				manifestEntry, _ := json.Marshal(map[string]string{
					"hour":     hour,
					"json_key": jsonKey,
					"csv_key":  csvKey,
				})
				c.redisClient.LPush(ctx, fmt.Sprintf("session:%s:export_paths", sessionID), string(manifestEntry))
			}

			// Remove the hour from the hours set
			c.redisClient.SRem(ctx, fmt.Sprintf("session:%s:hours", sessionID), hour).Err()
		}
	}
}
