// Package benchshared holds helpers common to the benchmark and benchmark-e2e tools.
package benchshared

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	segmentio "github.com/segmentio/kafka-go"
)

type Config struct {
	DatabaseURL          string
	Port                 string
	KafkaBrokers         []string
	KafkaTelemetryTopic  string
	KafkaAIWorkerGroupID string
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	HTTPSTLSCertFile     string
	HTTPSTLSKeyFile      string
}

func LoadConfig() *Config {
	_ = godotenv.Load(".env")

	cfg := &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Port:                os.Getenv("PORT"),
		KafkaBrokers:        SplitCSV(os.Getenv("KAFKA_BROKERS")),
		KafkaTelemetryTopic: os.Getenv("KAFKA_TELEMETRY_TOPIC"),
		RedisAddr:           os.Getenv("REDIS_ADDR"),
		RedisPassword:       os.Getenv("REDIS_PASSWORD"),
		HTTPSTLSCertFile:    os.Getenv("HTTPS_TLS_CERT_FILE"),
		HTTPSTLSKeyFile:     os.Getenv("HTTPS_TLS_KEY_FILE"),
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.KafkaTelemetryTopic == "" {
		cfg.KafkaTelemetryTopic = "argus.telemetry"
	}
	cfg.KafkaAIWorkerGroupID = os.Getenv("KAFKA_AI_WORKER_GROUP_ID")
	if cfg.KafkaAIWorkerGroupID == "" {
		cfg.KafkaAIWorkerGroupID = "argus-ai-worker"
	}
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}
	if redisDB := strings.TrimSpace(os.Getenv("REDIS_DB")); redisDB != "" {
		if parsed, err := strconv.Atoi(redisDB); err == nil {
			cfg.RedisDB = parsed
		}
	}
	return cfg
}

func SplitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	var values []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func GetHistogramPercentiles(client *http.Client, scheme, apiHost, port, metricName, matchLabel string) (p50, p95, p99 float64) {
	url := fmt.Sprintf("%s://%s:%s/metrics", scheme, apiHost, port)
	resp, err := client.Get(url)
	if err != nil {
		return 0, 0, 0
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0
	}

	type bucket struct {
		le    float64
		count float64
	}
	var buckets []bucket

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, metricName+"_bucket") {
			continue
		}
		if matchLabel != "" && !strings.Contains(line, matchLabel) {
			continue
		}

		// Example: metricName_bucket{...,le="0.0001"} 123
		idxLE := strings.Index(line, "le=\"")
		if idxLE == -1 {
			continue
		}
		endLE := strings.Index(line[idxLE+4:], "\"")
		if endLE == -1 {
			continue
		}
		leStr := line[idxLE+4 : idxLE+4+endLE]
		le, _ := strconv.ParseFloat(leStr, 64)

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		count, _ := strconv.ParseFloat(fields[1], 64)

		buckets = append(buckets, bucket{le: le, count: count})
	}

	if len(buckets) == 0 {
		return 0, 0, 0
	}

	// Sort buckets by LE
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].le < buckets[j].le })

	totalCount := buckets[len(buckets)-1].count
	if totalCount == 0 {
		return 0, 0, 0
	}

	calc := func(percentile float64) float64 {
		target := percentile * totalCount
		for i, b := range buckets {
			if b.count >= target {
				if i == 0 {
					return b.le * 1000.0 // ms
				}
				prev := buckets[i-1]
				ratio := (target - prev.count) / (b.count - prev.count)
				val := prev.le + (b.le-prev.le)*ratio
				return val * 1000.0 // ms
			}
		}
		return buckets[len(buckets)-1].le * 1000.0
	}

	return calc(0.50), calc(0.95), calc(0.99)
}

func DoAuthPost(client *http.Client, url string, body io.Reader, authHeader string, workspaceID string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Workspace-ID", workspaceID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

func RegisterBenchmarkUser(client *http.Client, baseURL string) {
	registerBody := bytes.NewBufferString(`{"email":"benchmark@argus.test","password":"Benchmark123!","name":"Benchmark User"}`)
	req, _ := http.NewRequest("POST", baseURL+"/auth/register", registerBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Register attempt failed (may already exist): %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		fmt.Println("Registered benchmark user.")
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Register response (user may already exist): %s\n", string(body))
	}
}

func GetUserIDFromToken(accessToken string) string {
	if accessToken == "" {
		return ""
	}
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return ""
	}
	// Fix base64 padding
	encoded := parts[1]
	switch len(encoded) % 4 {
	case 2:
		encoded += "=="
	case 3:
		encoded += "="
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	var claims struct {
		UserID string `json:"user_id"`
		Sub    string `json:"sub"`
	}
	json.Unmarshal(decoded, &claims)
	if claims.UserID != "" {
		return claims.UserID
	}
	return claims.Sub
}

func CleanupDB(db *sql.DB, cfg *Config, kafkaAddr string) {
	_, _ = db.Exec("DELETE FROM tenant_usage")
	_, _ = db.Exec("DELETE FROM session_artifacts WHERE session_id IN (SELECT id FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001')")
	_, _ = db.Exec("DELETE FROM session_statistics WHERE session_id IN (SELECT id FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001')")
	_, _ = db.Exec("DELETE FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001'")
	_, _ = db.Exec("DELETE FROM devices WHERE workspace_id = '00000000-0000-0000-0000-000000000001'")
	_, _ = db.Exec("DELETE FROM workspaces WHERE id = '00000000-0000-0000-0000-000000000001'")

	fmt.Printf("Skipping consumer group deletion for native execution.\n")

	// Ensure the telemetry.raw topic exists with 3 partitions
	EnsureTopicWithPartitions("telemetry.raw", 3, kafkaAddr)
}

func EnsureTopicWithPartitions(topic string, partitions int, kafkaAddr string) {
	client := &segmentio.Client{
		Addr: segmentio.TCP(kafkaAddr),
	}

	resp, err := client.CreateTopics(context.Background(), &segmentio.CreateTopicsRequest{
		Topics: []segmentio.TopicConfig{
			{
				Topic:             topic,
				NumPartitions:     partitions,
				ReplicationFactor: 1,
			},
		},
	})

	if err != nil {
		fmt.Printf("Failed to create topic %s natively: %v\n", topic, err)
		return
	}

	for _, t := range resp.Errors {
		if t != nil && t.Error() != "topic already exists" {
			fmt.Printf("Warning: Kafka returned error for topic creation: %v\n", t)
		}
	}
	fmt.Printf("Topic %s ensured with %d partitions.\n", topic, partitions)
}

func GetAppMetrics(client *http.Client, scheme, apiHost, port string) (cpu float64, rss float64, goroutines float64, consumed float64, dropped float64, failures float64, duplicates float64, err error) {
	url := fmt.Sprintf("%s://%s:%s/metrics", scheme, apiHost, port)
	resp, err := client.Get(url)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, err
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		if idx := strings.Index(name, "{"); idx != -1 {
			name = name[:idx]
		}
		var val float64
		_, scanErr := fmt.Sscanf(parts[1], "%f", &val)
		if scanErr != nil {
			continue
		}

		switch name {
		case "process_cpu_seconds_total":
			cpu = val
		case "process_resident_memory_bytes":
			rss = val
		case "go_goroutines":
			goroutines = val
		case "telemetry_consumer_messages_total":
			consumed = val
		case "telemetry_consumer_dropped_messages_total":
			dropped = val
		case "telemetry_consumer_processing_failures_total":
			failures = val
		case "telemetry_consumer_duplicate_messages_total":
			duplicates = val
		}
	}
	return
}

func GetPrometheusMetricValue(client *http.Client, scheme, apiHost, port string, metricName string) float64 {
	url := fmt.Sprintf("%s://%s:%s/metrics", scheme, apiHost, port)
	resp, err := client.Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		if idx := strings.Index(name, "{"); idx != -1 {
			name = name[:idx]
		}
		if name == metricName {
			var val float64
			if _, err := fmt.Sscanf(parts[1], "%f", &val); err == nil {
				return val
			}
		}
	}
	return 0
}

func GetRedisStats(rdb *goredis.Client) (totalCommands int64, usedMemory int64, evalshaCalls int64, evalshaUsec int64, err error) {
	ctx := context.Background()
	stats, err := rdb.Info(ctx, "stats").Result()
	if err == nil {
		lines := strings.Split(stats, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "total_commands_processed:") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					if val, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
						totalCommands = val
					}
				}
			}
		}
	}

	memory, err := rdb.Info(ctx, "memory").Result()
	if err == nil {
		lines := strings.Split(memory, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "used_memory:") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					if val, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
						usedMemory = val
					}
				}
			}
		}
	}

	commandstats, err := rdb.Info(ctx, "commandstats").Result()
	if err == nil {
		lines := strings.Split(commandstats, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "cmdstat_evalsha:") {
				parts := strings.Split(strings.TrimPrefix(line, "cmdstat_evalsha:"), ",")
				for _, part := range parts {
					subparts := strings.Split(part, "=")
					if len(subparts) == 2 {
						key := strings.TrimSpace(subparts[0])
						val := strings.TrimSpace(subparts[1])
						if key == "calls" {
							if v, err := strconv.ParseInt(val, 10, 64); err == nil {
								evalshaCalls = v
							}
						} else if key == "usec" {
							if v, err := strconv.ParseInt(val, 10, 64); err == nil {
								evalshaUsec = v
							}
						}
					}
				}
			}
		}
	}
	return
}

func GetKafkaConsumerLag(kafkaAddr string, groupName string, topicName string) (int64, int64, map[int32]int64, int64, error) {
	client := &segmentio.Client{Addr: segmentio.TCP(kafkaAddr)}

	meta, err := client.Metadata(context.Background(), &segmentio.MetadataRequest{Topics: []string{topicName}})
	if err != nil {
		return 0, 0, nil, 0, err
	}

	var partitions []int
	for _, t := range meta.Topics {
		if t.Name == topicName {
			for _, p := range t.Partitions {
				partitions = append(partitions, p.ID)
			}
		}
	}

	var reqReqs []segmentio.OffsetRequest
	for _, p := range partitions {
		reqReqs = append(reqReqs, segmentio.OffsetRequest{Partition: p, Timestamp: segmentio.LastOffset})
	}
	endOffsetsResp, err := client.ListOffsets(context.Background(), &segmentio.ListOffsetsRequest{
		Topics: map[string][]segmentio.OffsetRequest{topicName: reqReqs},
	})
	if err != nil {
		return 0, 0, nil, 0, err
	}

	endOffsets := make(map[int]int64)
	for _, p := range endOffsetsResp.Topics[topicName] {
		endOffsets[p.Partition] = p.LastOffset
	}

	groupOffsetsResp, err := client.OffsetFetch(context.Background(), &segmentio.OffsetFetchRequest{
		GroupID: groupName,
		Topics:  map[string][]int{topicName: partitions},
	})
	if err != nil {
		return 0, 0, nil, 0, err
	}

	var totalLag int64
	var peakLag int64
	partitionLag := make(map[int32]int64)
	var members int64 = 1

	for _, p := range groupOffsetsResp.Topics[topicName] {
		endOff := endOffsets[p.Partition]
		commitOff := p.CommittedOffset
		if commitOff < 0 {
			commitOff = 0
		}
		lag := endOff - commitOff
		if lag < 0 {
			lag = 0
		}
		partitionLag[int32(p.Partition)] = lag
		totalLag += lag
		if lag > peakLag {
			peakLag = lag
		}
	}

	if totalLag == 0 && len(partitionLag) > 0 {
		var sum int64
		for _, l := range partitionLag {
			sum += l
		}
		totalLag = sum
	}

	return totalLag, peakLag, partitionLag, members, nil
}

func ValidateArtifact(httpClient *http.Client, baseURL string, sessionID string, expectedDevices int, durationSeconds int, authHeader, workspaceID string) (bool, string, error) {
	url := fmt.Sprintf("%s/sessions/%s/artifact", baseURL, sessionID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("X-Workspace-ID", workspaceID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to fetch artifact: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, "", fmt.Errorf("artifact status %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		SessionID       string `json:"session_id"`
		DeviceSummaries map[string]struct {
			DeviceID    string `json:"device_id"`
			SampleCount int    `json:"sample_count"`
		} `json:"device_summaries"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("failed to read artifact body: %w", err)
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return false, "", fmt.Errorf("failed to decode artifact JSON: %w", err)
	}

	minExpectedDevices := expectedDevices
	if expectedDevices > 500 {
		minExpectedDevices = int(float64(expectedDevices) * 0.95)
	} else {
		minExpectedDevices = int(float64(expectedDevices) * 0.98)
	}
	if len(payload.DeviceSummaries) < minExpectedDevices {
		return false, string(body), fmt.Errorf("device count mismatch: got %d, expected at least %d", len(payload.DeviceSummaries), minExpectedDevices)
	}

	var lowSamplesCount int
	var avgSampleCount float64
	for devID, summary := range payload.DeviceSummaries {
		avgSampleCount += float64(summary.SampleCount)
		minExpected := int(float64(durationSeconds) * 0.90)
		if summary.SampleCount < minExpected {
			lowSamplesCount++
			fmt.Printf("Device %s has low sample count: %d (expected >= %d)\n", devID, summary.SampleCount, minExpected)
		}
	}
	avgSampleCount /= float64(expectedDevices)

	if lowSamplesCount > expectedDevices/10 {
		return false, string(body), fmt.Errorf("too many devices with low sample count: %d devices, average sample count: %.2f", lowSamplesCount, avgSampleCount)
	}

	return true, string(body), nil
}

func DetectMemoryLeak(rssValues []float64) (bool, string) {
	if len(rssValues) < 5 {
		return false, "not enough samples to detect leak"
	}
	monotonic := true
	for i := 1; i < len(rssValues); i++ {
		if rssValues[i] <= rssValues[i-1] {
			if (rssValues[i-1] - rssValues[i]) > 0.05*rssValues[i-1] {
				monotonic = false
				break
			}
		}
	}

	if !monotonic {
		return false, "memory stabilized or fluctuated"
	}

	firstVal := rssValues[0]
	lastVal := rssValues[len(rssValues)-1]
	growthPercent := ((lastVal - firstVal) / firstVal) * 100.0

	if growthPercent > 50.0 {
		halfIdx := len(rssValues) / 2
		growthFirstHalf := rssValues[halfIdx] - rssValues[0]
		growthSecondHalf := rssValues[len(rssValues)-1] - rssValues[halfIdx]
		if growthSecondHalf > 0.8*growthFirstHalf {
			return true, fmt.Sprintf("continuous monotonic memory growth: %.2f%% growth overall, early growth: %.2f MB, late growth: %.2f MB", growthPercent, growthFirstHalf, growthSecondHalf)
		}
	}
	return false, "memory growth stabilized"
}
