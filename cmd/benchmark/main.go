package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	goredis "github.com/redis/go-redis/v9"
	segmentio "github.com/segmentio/kafka-go"
	"github.com/vishalss1/argus/internal/config"

	_ "github.com/lib/pq"
)

type SessionResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"`
}

type Checkpoint struct {
	Time       time.Duration
	Published  int64
	Consumed   int64
	Processed  int64
	Lag        int64
	CPU        float64
	Memory     float64
	RedisCPU   float64
	RedisMem   float64
}

func main() {
	devicesFlag := flag.Int("devices", 1000, "Number of simulated devices")
	durationFlag := flag.Duration("duration", 60*time.Minute, "Duration of the load test")
	freqFlag := flag.Duration("freq", 1*time.Second, "Publish interval per device")
	payloadSizeFlag := flag.Int("payload", 256, "Extra payload size in bytes")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Printf("ARGUS Sustained Load System Capacity Benchmark\n")
	fmt.Printf("Simulated Devices: %d\n", *devicesFlag)
	fmt.Printf("Duration:          %s\n", *durationFlag)
	fmt.Printf("Publish Interval:  %s\n", *freqFlag)
	fmt.Printf("Extra Payload:     %d bytes\n", *payloadSizeFlag)
	fmt.Println("==================================================")

	cfg := config.Load()

	// 1. Database Connection
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		return
	}
	defer db.Close()

	// 2. HTTP Client with TLS skip
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   15 * time.Second,
	}

	scheme := "http"
	if cfg.HTTPSTLSCertFile != "" && cfg.HTTPSTLSKeyFile != "" {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://localhost:%s/api", scheme, cfg.Port)
	fmt.Printf("Using API Endpoint: %s\n", baseURL)

	// Clean up previous runs
	cleanupDB(db)

	// Setup Redis Client for Info collection and flush
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()
	_ = rdb.FlushDB(context.Background()).Err()

	// 3. Setup Workspace and Devices
	workspaceID := "00000000-0000-0000-0000-000000000001"
	_, err = db.Exec(`INSERT INTO workspaces (id, name, description, created_at) 
		VALUES ($1, 'Benchmark Workspace', 'Workspace for load testing', NOW())
		ON CONFLICT (id) DO NOTHING`, workspaceID)
	if err != nil {
		fmt.Printf("Failed to insert workspace: %v\n", err)
		return
	}

	fmt.Printf("Registering %d devices in database...\n", *devicesFlag)
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("Failed to begin device registration tx: %v\n", err)
		return
	}
	stmt, err := tx.Prepare(`INSERT INTO devices (id, name, type, status, firmware_version, workspace_id, created_at) 
		VALUES ($1, $2, 'sensor', 'online', 'v1.0.0', $3, NOW())
		ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		fmt.Printf("Failed to prepare device registration: %v\n", err)
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for i := 0; i < *devicesFlag; i++ {
		deviceID := fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
		_, err = stmt.Exec(deviceID, deviceID, workspaceID)
		if err != nil {
			fmt.Printf("Failed to register device %s: %v\n", deviceID, err)
			tx.Rollback()
			return
		}
	}
	if err := tx.Commit(); err != nil {
		fmt.Printf("Failed to commit device registrations: %v\n", err)
		return
	}

	// 4. Create Session via API
	url := fmt.Sprintf("%s/workspaces/%s/sessions", baseURL, workspaceID)
	resp, err := httpClient.Post(url, "application/json", nil)
	if err != nil {
		fmt.Printf("Failed to create session: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to create session, status: %d, body: %s\n", resp.StatusCode, string(body))
		return
	}

	var sessionRes SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionRes); err != nil {
		fmt.Printf("Failed to decode session response: %v\n", err)
		return
	}
	sessionID := sessionRes.ID
	fmt.Printf("Created session: %s\n", sessionID)

	// 5. Start Session via API
	startURL := fmt.Sprintf("%s/sessions/%s/start", baseURL, sessionID)
	resp, err = httpClient.Post(startURL, "application/json", nil)
	if err != nil {
		fmt.Printf("Failed to start session: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to start session, status: %d, body: %s\n", resp.StatusCode, string(body))
		return
	}
	fmt.Println("Session started successfully.")

	// Pre-test metrics snapshot
	preCPU, preMem, _, preConsumed, preDropped, preFailures, preDuplicates, err := getAppMetrics(httpClient, scheme, cfg.Port)
	if err != nil {
		fmt.Printf("Warning: Failed to fetch initial app metrics: %v\n", err)
	}

	preRedisCmds, preRedisMem, preEvalCalls, preEvalUsec, err := getRedisStats(rdb)
	if err != nil {
		fmt.Printf("Warning: Failed to fetch initial Redis stats: %v\n", err)
	}

	preArtifactGenDuration := getPrometheusMetricValue(httpClient, scheme, cfg.Port, "session_artifact_generation_duration_seconds_sum")

	// 6. Kafka/Redpanda Async Writer Config
	kafkaAddr := "localhost:9092"
	if len(cfg.KafkaBrokers) > 0 {
		kafkaAddr = cfg.KafkaBrokers[0]
	}
	fmt.Printf("Connecting to Kafka at %s...\n", kafkaAddr)

	var messagesPublished int64
	var publishFailures int64

	writer := &segmentio.Writer{
		Addr:         segmentio.TCP(kafkaAddr),
		Topic:        cfg.KafkaTelemetryTopic,
		Balancer:     &segmentio.Hash{},
		Async:        true,
		BatchSize:    5000,
		BatchTimeout: 10 * time.Millisecond,
		Completion: func(messages []segmentio.Message, err error) {
			if err != nil {
				atomic.AddInt64(&publishFailures, int64(len(messages)))
			} else {
				atomic.AddInt64(&messagesPublished, int64(len(messages)))
			}
		},
	}
	defer writer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Track latency slices per device (lock-free)
	durationSec := int((*durationFlag).Seconds())
	deviceLatencies := make([][]time.Duration, *devicesFlag)
	for i := range deviceLatencies {
		deviceLatencies[i] = make([]time.Duration, 0, durationSec)
	}

	extraData := strings.Repeat("x", *payloadSizeFlag)

	fmt.Printf("Starting telemetry traffic generation...\n")
	startTime := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < *devicesFlag; i++ {
		deviceID := fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
		wg.Add(1)
		go func(devID string, devIndex int) {
			defer wg.Done()

			lat := 37.7749
			lon := -122.4194

			publishMsg := func() {
				lat += 0.0001
				lon += 0.0001

				metricsVal := map[string]interface{}{
					"battery_level": 98.5 - float64(time.Since(startTime).Seconds()*0.01),
					"temperature":   24.5 + float64(time.Since(startTime).Seconds()*0.005),
					"rssi":          -65.0,
					"uptime":        int(time.Since(startTime).Seconds()),
					"latitude":      lat,
					"longitude":     lon,
					"extra":         extraData,
				}
				metricsBytes, _ := json.Marshal(metricsVal)

				telemetryMsg := map[string]interface{}{
					"device_id":   devID,
					"metrics":     json.RawMessage(metricsBytes),
					"recorded_at": time.Now().UTC().Format(time.RFC3339),
				}
				telemetryBytes, _ := json.Marshal(telemetryMsg)

				enqueueStart := time.Now()
				err := writer.WriteMessages(ctx, segmentio.Message{
					Key:   []byte(devID),
					Value: telemetryBytes,
				})
				latency := time.Since(enqueueStart)

				deviceLatencies[devIndex] = append(deviceLatencies[devIndex], latency)

				if err != nil {
					atomic.AddInt64(&publishFailures, 1)
				}
			}

			// Publish first message immediately
			publishMsg()
			publishedCount := 1

			if publishedCount >= durationSec {
				return
			}

			ticker := time.NewTicker(*freqFlag)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					publishMsg()
					publishedCount++
					if publishedCount >= durationSec {
						return
					}
				}
			}
		}(deviceID, i)
	}

	// 7. Polling loop (every 5 seconds) to track all resource and throughput statistics
	monitorTicker := time.NewTicker(5 * time.Second)
	defer monitorTicker.Stop()

	// Monitoring Time-Series Lists
	var appCPUSeries []float64
	var appRSSSeries []float64
	var appGoroutinesSeries []float64

	var kafkaCPUSeries []float64
	var kafkaMemSeries []float64

	var redisCPUSeries []float64
	var redisMemSeries []float64
	var redisOpsSeries []float64
	var redisUsedMemorySeries []float64
	var redisLuaLatencies []float64

	var totalLagSeries []float64
	var peakLagSeries []float64

	var checkpoints []Checkpoint
	nextCheckpointIdx := 0
	checkpointDurations := []time.Duration{
		5 * time.Minute,
		10 * time.Minute,
		15 * time.Minute,
		30 * time.Minute,
		60 * time.Minute,
	}

	// For App CPU Calculation
	prevCPUTime := preCPU
	prevCPUQueryTime := startTime

	// For Redis Ops Calculation
	prevRedisCmds := preRedisCmds

	// For Redis Lua Latency Calculation
	prevEvalCalls := preEvalCalls
	prevEvalUsec := preEvalUsec

	var lastConsumed, lastDropped, lastFailures float64

	// Helper to handle monitoring updates
	runMonitorTick := func() {
		nowQueryTime := time.Now()
		elapsedSecs := nowQueryTime.Sub(prevCPUQueryTime).Seconds()
		if elapsedSecs <= 0 {
			elapsedSecs = 1.0
		}

		// A. App Stats
		cpu, rss, goroutines, consumed, dropped, failures, _, err := getAppMetrics(httpClient, scheme, cfg.Port)
		if err == nil {
			cpuDelta := cpu - prevCPUTime
			cpuUtil := (cpuDelta / elapsedSecs) * 100.0
			appCPUSeries = append(appCPUSeries, cpuUtil)
			appRSSSeries = append(appRSSSeries, rss/(1024.0*1024.0)) // MB
			appGoroutinesSeries = append(appGoroutinesSeries, goroutines)
			lastConsumed = consumed
			lastDropped = dropped
			lastFailures = failures

			prevCPUTime = cpu
			prevCPUQueryTime = nowQueryTime
		}

		// B. Redis Stats from Client
		rCmds, rMem, rEvalCalls, rEvalUsec, err := getRedisStats(rdb)
		if err == nil {
			redisCmdsDelta := rCmds - prevRedisCmds
			redisOpsSec := float64(redisCmdsDelta) / elapsedSecs
			redisOpsSeries = append(redisOpsSeries, redisOpsSec)
			redisUsedMemorySeries = append(redisUsedMemorySeries, float64(rMem)/(1024.0*1024.0)) // MB

			evalCallsDelta := rEvalCalls - prevEvalCalls
			evalUsecDelta := rEvalUsec - prevEvalUsec
			if evalCallsDelta > 0 {
				luaLatencyMS := float64(evalUsecDelta) / float64(evalCallsDelta) / 1000.0 // us to ms
				redisLuaLatencies = append(redisLuaLatencies, luaLatencyMS)
			}

			prevRedisCmds = rCmds
			prevEvalCalls = rEvalCalls
			prevEvalUsec = rEvalUsec
		}

		// C. Docker Stats for Containers
		kCPU, kMem, err := parseDockerStats("argus-redpanda")
		if err == nil {
			kafkaCPUSeries = append(kafkaCPUSeries, kCPU)
			kafkaMemSeries = append(kafkaMemSeries, kMem)
		}
		redCPU, redMem, err := parseDockerStats("argus-redis")
		if err == nil {
			redisCPUSeries = append(redisCPUSeries, redCPU)
			redisMemSeries = append(redisMemSeries, redMem)
		}

		// D. Kafka Consumer Group Lag Description
		totLag, peakLag, _, err := getKafkaConsumerLag("argus-telemetry-live-consumer-internal", cfg.KafkaTelemetryTopic)
		if err == nil {
			totalLagSeries = append(totalLagSeries, float64(totLag))
			peakLagSeries = append(peakLagSeries, float64(peakLag))
		}

		// E. Checkpoints
		elapsedRun := time.Since(startTime)
		if nextCheckpointIdx < len(checkpointDurations) && elapsedRun >= checkpointDurations[nextCheckpointIdx] {
			targetDur := checkpointDurations[nextCheckpointIdx]
			fmt.Printf("--- CHECKPOINT %s ---\n", targetDur)
			fmt.Printf("  Published: %d\n", atomic.LoadInt64(&messagesPublished))
			fmt.Printf("  Consumed:  %d\n", int64(lastConsumed-preConsumed))
			
			processedVal := int64(lastConsumed - preConsumed) - int64(lastDropped - preDropped) - int64(lastFailures - preFailures)
			fmt.Printf("  Processed: %d\n", processedVal)
			fmt.Printf("  Group Lag: %d\n", totLag)
			
			var curAppCPU, curAppMem float64
			if len(appCPUSeries) > 0 {
				curAppCPU = appCPUSeries[len(appCPUSeries)-1]
				curAppMem = appRSSSeries[len(appRSSSeries)-1]
				fmt.Printf("  App CPU:   %.2f%%\n", curAppCPU)
				fmt.Printf("  App Mem:   %.2f MB\n", curAppMem)
			}
			
			var curRedCPU, curRedMem float64
			if len(redisCPUSeries) > 0 {
				curRedCPU = redisCPUSeries[len(redisCPUSeries)-1]
				curRedMem = redisMemSeries[len(redisMemSeries)-1]
				fmt.Printf("  Redis CPU: %.2f%%\n", curRedCPU)
				fmt.Printf("  Redis Mem: %.2f MB\n", curRedMem)
			}
			
			checkpoints = append(checkpoints, Checkpoint{
				Time:      targetDur,
				Published: atomic.LoadInt64(&messagesPublished),
				Consumed:  int64(lastConsumed - preConsumed),
				Processed:  processedVal,
				Lag:       totLag,
				CPU:       curAppCPU,
				Memory:    curAppMem,
				RedisCPU:  curRedCPU,
				RedisMem:  curRedMem,
			})
			nextCheckpointIdx++
		}
	}

	// Run monitoring ticker in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-monitorTicker.C:
				runMonitorTick()
			}
		}
	}()

	// Let the test run for the configured duration
	time.Sleep(*durationFlag)
	wg.Wait() // Wait for all device threads to finish publishing naturally
	cancel()  // Stop monitor loop

	fmt.Println("Telemetry traffic completed. Closing writer to flush all pending messages...")
	activeWStats := writer.Stats()
	_ = writer.Close()

	fmt.Println("Waiting for consumer lag to drain to 0...")
	drainStart := time.Now()
	for {
		lag, _, _, err := getKafkaConsumerLag("argus-telemetry-live-consumer-internal", cfg.KafkaTelemetryTopic)
		if err == nil && lag == 0 {
			fmt.Println("Consumer lag successfully drained to 0.")
			break
		}
		if time.Since(drainStart) > 10*time.Second {
			fmt.Printf("Timeout waiting for consumer lag to drain. Current lag: %d\n", lag)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Final metrics run
	runMonitorTick()

	// 8. Session Finalization Benchmarking
	fmt.Println("Stopping session...")
	stopURL := fmt.Sprintf("%s/sessions/%s/stop", baseURL, sessionID)
	stopPayload := []byte(`{"success":true}`)

	stopStart := time.Now()
	resp, err = httpClient.Post(stopURL, "application/json", bytes.NewBuffer(stopPayload))
	stopDuration := time.Since(stopStart)
	if err != nil {
		fmt.Printf("Failed to stop session: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to stop session, status: %d, body: %s\n", resp.StatusCode, string(body))
		return
	}
	fmt.Printf("Session stopped successfully in %s.\n", stopDuration)

	// Fetch final Prometheus metrics
	_, postMem, _, postConsumed, postDropped, postFailures, postDuplicates, err := getAppMetrics(httpClient, scheme, cfg.Port)
	if err != nil {
		fmt.Printf("Failed to fetch post metrics: %v\n", err)
	}

	// Retrieve session artifact metrics
	cumulativeArtifactGenDuration := getPrometheusMetricValue(httpClient, scheme, cfg.Port, "session_artifact_generation_duration_seconds_sum")
	postArtifactGenDuration := cumulativeArtifactGenDuration - preArtifactGenDuration
	if postArtifactGenDuration < 0 {
		postArtifactGenDuration = 0
	}
	
	// Wait a moment for final cleanup validations
	time.Sleep(500 * time.Millisecond)

	// Check Redis keys removed: scan for session keys to verify cleanup
	redisKeys, _ := rdb.Keys(context.Background(), fmt.Sprintf("session:%s:*", sessionID)).Result()
	redisKeysRemoved := true
	if len(redisKeys) > 0 {
		redisKeysRemoved = false
		fmt.Printf("Warning: %d Redis keys remain for session %s after stop: %v\n", len(redisKeys), sessionID, redisKeys)
	}

	// 9. Artifact Correctness Validation
	artifactValid, artifactBody, valErr := validateArtifact(httpClient, baseURL, sessionID, *devicesFlag, durationSec)
	var artifactSize int
	if valErr != nil {
		fmt.Printf("Artifact Validation Failed: %v\n", valErr)
	} else {
		artifactSize = len(artifactBody)
	}

	// 10. Compute Aggregates & Statistics
	totalPublished := atomic.LoadInt64(&messagesPublished)
	totalDropped := postDropped - preDropped
	totalFailures := postFailures - preFailures
	totalDuplicates := postDuplicates - preDuplicates

	// Ingress consumption delta from Prometheus
	totalConsumed := postConsumed - preConsumed
	
	successfullyProcessed := totalConsumed - totalFailures - totalDropped

	// Wait, we also check final Kafka Lag
	finalLag, _, partitionLag, err := getKafkaConsumerLag("argus-telemetry-live-consumer-internal", cfg.KafkaTelemetryTopic)

	// Enqueue Latency Aggregation
	var allLatencies []time.Duration
	var totalLatencySum time.Duration
	for _, lList := range deviceLatencies {
		allLatencies = append(allLatencies, lList...)
		for _, latVal := range lList {
			totalLatencySum += latVal
		}
	}
	sort.Slice(allLatencies, func(i, j int) bool { return allLatencies[i] < allLatencies[j] })

	var avgEnqueue time.Duration
	var p50Enqueue time.Duration
	var p95Enqueue time.Duration
	var peakEnqueue time.Duration
	if len(allLatencies) > 0 {
		avgEnqueue = totalLatencySum / time.Duration(len(allLatencies))
		p50Enqueue = allLatencies[len(allLatencies)/2]
		p95Enqueue = allLatencies[int(float64(len(allLatencies))*0.95)]
		peakEnqueue = allLatencies[len(allLatencies)-1]
	}

	// Writer stats
	wStats := writer.Stats()
	writerRetries := wStats.Retries
	batchFlushAvg := wStats.BatchTime.Avg
	batchFlushMax := wStats.BatchTime.Max
	
	// Estimate queue saturation events
	var saturationEvents int64
	if activeWStats.QueueCapacity > 0 && activeWStats.QueueLength >= activeWStats.QueueCapacity {
		saturationEvents++
	}

	// Averages and Peaks
	var avgAppCPU, peakAppCPU, peakAppMem, peakAppGoroutines float64
	var avgRedisCPU, peakRedisCPU, avgRedisMem, peakRedisMem, avgRedisOps, peakRedisOps float64
	var avgKafkaCPU, peakKafkaCPU, avgKafkaMem, peakKafkaMem float64
	var avgLag, maxLagVal float64

	computeStats := func(series []float64) (avg float64, peak float64) {
		if len(series) == 0 {
			return 0, 0
		}
		var sum float64
		for _, v := range series {
			sum += v
			if v > peak {
				peak = v
			}
		}
		return sum / float64(len(series)), peak
	}

	avgAppCPU, peakAppCPU = computeStats(appCPUSeries)
	_, peakAppMem = computeStats(appRSSSeries)
	_, peakAppGoroutines = computeStats(appGoroutinesSeries)
	
	avgRedisCPU, peakRedisCPU = computeStats(redisCPUSeries)
	avgRedisMem, peakRedisMem = computeStats(redisMemSeries)
	avgRedisOps, peakRedisOps = computeStats(redisOpsSeries)
	
	avgKafkaCPU, peakKafkaCPU = computeStats(kafkaCPUSeries)
	avgKafkaMem, peakKafkaMem = computeStats(kafkaMemSeries)

	avgLag, maxLagVal = computeStats(totalLagSeries)

	// Lua Stats
	var avgLuaLatency float64
	if len(redisLuaLatencies) > 0 {
		var sum float64
		for _, l := range redisLuaLatencies {
			sum += l
		}
		avgLuaLatency = sum / float64(len(redisLuaLatencies))
	}

	// Memory Leak Validation
	initialRSS := preMem / (1024.0 * 1024.0) // MB
	finalRSS := postMem / (1024.0 * 1024.0)  // MB
	peakRSS := peakAppMem
	if finalRSS > peakRSS {
		peakRSS = finalRSS
	}
	growthPercent := 0.0
	if initialRSS > 0 {
		growthPercent = ((finalRSS - initialRSS) / initialRSS) * 100.0
	}

	isLeak, leakMsg := detectMemoryLeak(appRSSSeries)

	// Success Verdict
	expectedPublished := int64(*devicesFlag) * int64(durationSec)
	
	pass := true
	var failReasons []string

	if totalPublished < expectedPublished {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Target publish rate not sustained: published %d, expected %d", totalPublished, expectedPublished))
	}
	if int64(totalConsumed) != totalPublished {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Message loss: Published %d != Consumed %d", totalPublished, int64(totalConsumed)))
	}
	if int64(successfullyProcessed) != totalPublished {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Ingress processing error: Successfully processed %d != Published %d", int64(successfullyProcessed), totalPublished))
	}
	if totalDropped > 0 {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Dropped messages: %d", int64(totalDropped)))
	}
	if totalFailures > 0 {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Processing failures: %d", int64(totalFailures)))
	}
	if finalLag > 0 {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Non-zero final Kafka consumer lag: %d", finalLag))
	}
	if stopDuration.Seconds() >= 5.0 {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Session stop finalization duration too long: %s >= 5s", stopDuration))
	}
	if !redisKeysRemoved {
		pass = false
		failReasons = append(failReasons, "Redis session keys were not fully cleaned up on StopSession")
	}
	if !artifactValid {
		pass = false
		failReasons = append(failReasons, "Generated artifact validation failed")
	}
	if isLeak {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Memory leak detected: %s", leakMsg))
	}

	verdict := "PASS"
	if !pass {
		verdict = "FAIL"
	}

	// 11. Final Report Generation in exact markdown format
	reportBuilder := strings.Builder{}
	reportBuilder.WriteString("## Configuration\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Devices: %d\n", *devicesFlag))
	reportBuilder.WriteString(fmt.Sprintf("Rate: %s/device\n", *freqFlag))
	reportBuilder.WriteString(fmt.Sprintf("Duration: %d sec\n\n", durationSec))

	reportBuilder.WriteString("## Publishing\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Messages Published: %d\n", totalPublished))
	reportBuilder.WriteString(fmt.Sprintf("Messages Expected: %d\n", expectedPublished))
	reportBuilder.WriteString(fmt.Sprintf("Publish Failures: %d\n", publishFailures))
	reportBuilder.WriteString(fmt.Sprintf("Publish Retries: %d\n", writerRetries))
	reportBuilder.WriteString(fmt.Sprintf("Effective Rate: %.2f msgs/sec\n", float64(totalPublished)/float64(durationSec)))
	reportBuilder.WriteString(fmt.Sprintf("Avg Enqueue Latency: %s\n", avgEnqueue))
	reportBuilder.WriteString(fmt.Sprintf("p50 Enqueue Latency: %s\n", p50Enqueue))
	reportBuilder.WriteString(fmt.Sprintf("p95 Enqueue Latency: %s\n", p95Enqueue))
	reportBuilder.WriteString(fmt.Sprintf("Peak Enqueue Latency: %s\n", peakEnqueue))
	reportBuilder.WriteString(fmt.Sprintf("Avg Batch Flush Latency: %s\n", batchFlushAvg))
	reportBuilder.WriteString(fmt.Sprintf("Peak Batch Flush Latency: %s\n", batchFlushMax))
	reportBuilder.WriteString(fmt.Sprintf("Queue Saturation Events: %d\n\n", saturationEvents))

	reportBuilder.WriteString("## Consumption\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Messages Consumed: %d\n", int64(totalConsumed)))
	reportBuilder.WriteString(fmt.Sprintf("Messages Successfully Processed: %d\n", int64(successfullyProcessed)))
	reportBuilder.WriteString(fmt.Sprintf("Messages Dropped: %d\n", int64(totalDropped)))
	reportBuilder.WriteString(fmt.Sprintf("Processing Failures: %d\n", int64(totalFailures)))
	reportBuilder.WriteString(fmt.Sprintf("Duplicate Messages: %d\n", int64(totalDuplicates)))
	reportBuilder.WriteString(fmt.Sprintf("Messages Lost: %d\n\n", totalPublished-int64(totalConsumed)))

	reportBuilder.WriteString("## Kafka\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Average Lag: %.2f\n", avgLag))
	reportBuilder.WriteString(fmt.Sprintf("Peak Lag: %.2f\n", maxLagVal))
	reportBuilder.WriteString(fmt.Sprintf("Final Lag: %d\n", finalLag))
	reportBuilder.WriteString("Per-Partition Lag:\n")
	for pID, lVal := range partitionLag {
		reportBuilder.WriteString(fmt.Sprintf("  Partition %d: %d\n", pID, lVal))
	}
	reportBuilder.WriteString("\n")

	// Query post-test Redis stats for report
	_, postRedisMem, _, _, _ = getRedisStats(rdb)

	reportBuilder.WriteString("## Redis\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Average Ops/sec: %.2f\n", avgRedisOps))
	reportBuilder.WriteString(fmt.Sprintf("Peak Ops/sec: %.2f\n", peakRedisOps))
	reportBuilder.WriteString(fmt.Sprintf("Lua Script Average Latency: %.3f ms\n", avgLuaLatency))
	reportBuilder.WriteString(fmt.Sprintf("Initial Redis memory: %.2f MB\n", float64(preRedisMem)/(1024.0*1024.0)))
	reportBuilder.WriteString(fmt.Sprintf("Peak Redis memory: %.2f MB\n", peakRedisMem))
	reportBuilder.WriteString(fmt.Sprintf("Final Redis memory: %.2f MB\n\n", float64(postRedisMem)/(1024.0*1024.0)))

	reportBuilder.WriteString("## Application\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Average CPU: %.2f%%\n", avgAppCPU))
	reportBuilder.WriteString(fmt.Sprintf("Peak CPU: %.2f%%\n", peakAppCPU))
	reportBuilder.WriteString(fmt.Sprintf("Initial Memory (RSS): %.2f MB\n", initialRSS))
	reportBuilder.WriteString(fmt.Sprintf("Peak Memory (RSS): %.2f MB\n", peakRSS))
	reportBuilder.WriteString(fmt.Sprintf("Final Memory (RSS): %.2f MB\n", finalRSS))
	reportBuilder.WriteString(fmt.Sprintf("Memory Growth: %.2f%%\n", growthPercent))
	reportBuilder.WriteString(fmt.Sprintf("Peak Goroutines: %.0f\n\n", peakAppGoroutines))

	reportBuilder.WriteString("## Container Resources (Docker Stats)\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Redpanda Avg CPU: %.2f%%\n", avgKafkaCPU))
	reportBuilder.WriteString(fmt.Sprintf("Redpanda Peak CPU: %.2f%%\n", peakKafkaCPU))
	reportBuilder.WriteString(fmt.Sprintf("Redpanda Avg Memory: %.2f MB\n", avgKafkaMem))
	reportBuilder.WriteString(fmt.Sprintf("Redpanda Peak Memory: %.2f MB\n", peakKafkaMem))
	reportBuilder.WriteString(fmt.Sprintf("Redis Container Avg CPU: %.2f%%\n", avgRedisCPU))
	reportBuilder.WriteString(fmt.Sprintf("Redis Container Peak CPU: %.2f%%\n", peakRedisCPU))
	reportBuilder.WriteString(fmt.Sprintf("Redis Container Avg Memory: %.2f MB\n", avgRedisMem))
	reportBuilder.WriteString(fmt.Sprintf("Redis Container Peak Memory: %.2f MB\n\n", peakRedisMem))

	reportBuilder.WriteString("## Session Finalization\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Artifact Size: %.4f MB (%d bytes)\n", float64(artifactSize)/(1024.0*1024.0), artifactSize))
	reportBuilder.WriteString(fmt.Sprintf("Artifact Duration: %.4f sec\n", postArtifactGenDuration))
	reportBuilder.WriteString(fmt.Sprintf("Cleanup Duration: %.4f sec\n", stopDuration.Seconds()-postArtifactGenDuration))
	reportBuilder.WriteString(fmt.Sprintf("Total StopSession Duration: %.4f sec\n\n", stopDuration.Seconds()))

	reportBuilder.WriteString("## Result\n\n")
	reportBuilder.WriteString(fmt.Sprintf("%s\n", verdict))
	if !pass {
		reportBuilder.WriteString("\nFailure Reasons:\n")
		for _, reason := range failReasons {
			reportBuilder.WriteString(fmt.Sprintf("- %s\n", reason))
		}
	}

	reportStr := reportBuilder.String()
	fmt.Println(reportStr)

	// Save to benchmark_report.md and benchmark_report_1000_devices_15min.md
	for _, filename := range []string{"benchmark_report.md", "benchmark_report_1000_devices_15min.md"} {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString("# ARGUS Capacity Load Test Benchmark Report\n\n")
			_, _ = f.WriteString(reportStr)
			
			if len(checkpoints) > 0 {
				_, _ = f.WriteString("\n## Checkpoints\n\n| Time | Published | Consumed | Processed | Lag | App CPU | App Memory | Redis CPU | Redis Memory |\n|---|---|---|---|---|---|---|---|---|\n")
				for _, cp := range checkpoints {
					_, _ = f.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %.2f%% | %.2f MB | %.2f%% | %.2f MB |\n",
						cp.Time, cp.Published, cp.Consumed, cp.Processed, cp.Lag, cp.CPU, cp.Memory, cp.RedisCPU, cp.RedisMem))
				}
			}
			f.Close()
		}
	}

	cleanupDB(db)
}

func cleanupDB(db *sql.DB) {
	_, _ = db.Exec("DELETE FROM tenant_usage")
	_, _ = db.Exec("DELETE FROM workspace_artifacts WHERE workspace_id = '00000000-0000-0000-0000-000000000001'")
	_, _ = db.Exec("DELETE FROM workspace_reports WHERE session_id IN (SELECT id FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001')")
	_, _ = db.Exec("DELETE FROM workspace_session_statistics WHERE session_id IN (SELECT id FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001')")
	_, _ = db.Exec("DELETE FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001'")
	_, _ = db.Exec("DELETE FROM devices WHERE workspace_id = '00000000-0000-0000-0000-000000000001'")
	_, _ = db.Exec("DELETE FROM workspaces WHERE id = '00000000-0000-0000-0000-000000000001'")

	// Check if topic exists and has 16 partitions
	descCmdInit := exec.Command("docker", "exec", "argus-redpanda", "rpk", "topic", "describe", "telemetry.raw")
	var descOutInit bytes.Buffer
	descCmdInit.Stdout = &descOutInit
	if errDesc := descCmdInit.Run(); errDesc == nil && strings.Contains(descOutInit.String(), "PARTITIONS  16") {
		fmt.Println("Topic telemetry.raw already exists with 16 partitions. Skipping recreate to prevent group rebalance.")
		return
	}

	// Recreate Kafka topic to purge all messages and offsets cleanly
	fmt.Println("Purging Kafka topic telemetry.raw...")
	delCmd := exec.Command("docker", "exec", "argus-redpanda", "rpk", "topic", "delete", "telemetry.raw")
	_ = delCmd.Run()
	
	time.Sleep(1 * time.Second)

	created := false
	for attempt := 1; attempt <= 5; attempt++ {
		createCmd := exec.Command("docker", "exec", "argus-redpanda", "rpk", "topic", "create", "telemetry.raw", "-p", "16")
		var errOut bytes.Buffer
		createCmd.Stderr = &errOut
		createCmd.Stdout = &errOut
		err := createCmd.Run()
		outStr := errOut.String()
		if err == nil || strings.Contains(outStr, "TOPIC_ALREADY_EXISTS") {
			// Check if it already has 16 partitions
			descCmd := exec.Command("docker", "exec", "argus-redpanda", "rpk", "topic", "describe", "telemetry.raw")
			var descOut bytes.Buffer
			descCmd.Stdout = &descOut
			if errDesc := descCmd.Run(); errDesc == nil {
				if strings.Contains(descOut.String(), "PARTITIONS  16") {
					fmt.Println("Topic telemetry.raw already exists with 16 partitions.")
					created = true
					break
				}
			}
			if err == nil {
				created = true
				break
			}
		}
		fmt.Printf("Attempt %d to create topic failed: %s. Retrying...\n", attempt, strings.TrimSpace(outStr))
		time.Sleep(1 * time.Second)
	}
	if !created {
		fmt.Println("Error: Failed to create telemetry.raw topic with 16 partitions after 5 attempts!")
	}

	time.Sleep(5 * time.Second)
}

func getAppMetrics(client *http.Client, scheme, port string) (cpu float64, rss float64, goroutines float64, consumed float64, dropped float64, failures float64, duplicates float64, err error) {
	url := fmt.Sprintf("%s://localhost:%s/metrics", scheme, port)
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

func getPrometheusMetricValue(client *http.Client, scheme, port string, metricName string) float64 {
	url := fmt.Sprintf("%s://localhost:%s/metrics", scheme, port)
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

func getRedisStats(rdb *goredis.Client) (totalCommands int64, usedMemory int64, evalshaCalls int64, evalshaUsec int64, err error) {
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

func parseDockerStats(containerName string) (float64, float64, error) {
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.CPUPerc}}|{{.MemUsage}}", containerName)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, 0, err
	}
	s := strings.TrimSpace(out.String())
	parts := strings.Split(s, "|")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected output: %s", s)
	}
	cpuStr := strings.TrimSpace(strings.ReplaceAll(parts[0], "%", ""))
	cpu, err := strconv.ParseFloat(cpuStr, 64)
	if err != nil {
		return 0, 0, err
	}
	memStr := parts[1]
	if idx := strings.Index(memStr, "/"); idx != -1 {
		memStr = memStr[:idx]
	}
	memStr = strings.TrimSpace(memStr)

	var multiplier float64 = 1.0
	if strings.HasSuffix(memStr, "GiB") {
		multiplier = 1024.0
		memStr = strings.TrimSuffix(memStr, "GiB")
	} else if strings.HasSuffix(memStr, "GB") {
		multiplier = 1024.0
		memStr = strings.TrimSuffix(memStr, "GB")
	} else if strings.HasSuffix(memStr, "MiB") {
		multiplier = 1.0
		memStr = strings.TrimSuffix(memStr, "MiB")
	} else if strings.HasSuffix(memStr, "MB") {
		multiplier = 1.0
		memStr = strings.TrimSuffix(memStr, "MB")
	} else if strings.HasSuffix(memStr, "KiB") {
		multiplier = 1.0 / 1024.0
		memStr = strings.TrimSuffix(memStr, "KiB")
	} else if strings.HasSuffix(memStr, "KB") {
		multiplier = 1.0 / 1024.0
		memStr = strings.TrimSuffix(memStr, "KB")
	} else if strings.HasSuffix(memStr, "B") {
		multiplier = 1.0 / (1024.0 * 1024.0)
		memStr = strings.TrimSuffix(memStr, "B")
	}
	memStr = strings.TrimSpace(memStr)
	mem, err := strconv.ParseFloat(memStr, 64)
	if err != nil {
		return cpu, 0, err
	}
	return cpu, mem * multiplier, nil
}

func getKafkaConsumerLag(groupName string, topicName string) (int64, int64, map[int32]int64, error) {
	cmd := exec.Command("docker", "exec", "argus-redpanda", "rpk", "group", "describe", groupName)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 0, 0, nil, err
	}
	lines := strings.Split(out.String(), "\n")
	var totalLag int64
	var peakLag int64
	partitionLag := make(map[int32]int64)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TOTAL-LAG") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if val, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					totalLag = val
				}
			}
		}

		if strings.HasPrefix(line, topicName) {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				partVal, err1 := strconv.ParseInt(fields[1], 10, 32)
				lagVal, err2 := strconv.ParseInt(fields[5], 10, 64)
				if err1 == nil && err2 == nil {
					partitionLag[int32(partVal)] = lagVal
					if lagVal > peakLag {
						peakLag = lagVal
					}
				}
			}
		}
	}

	if totalLag == 0 && len(partitionLag) > 0 {
		var sum int64
		for _, l := range partitionLag {
			sum += l
		}
		totalLag = sum
	}

	return totalLag, peakLag, partitionLag, nil
}

func validateArtifact(httpClient *http.Client, baseURL string, sessionID string, expectedDevices int, durationSeconds int) (bool, string, error) {
	url := fmt.Sprintf("%s/sessions/%s/artifact", baseURL, sessionID)
	resp, err := httpClient.Get(url)
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

	if len(payload.DeviceSummaries) != expectedDevices {
		return false, string(body), fmt.Errorf("device count mismatch: got %d, expected %d", len(payload.DeviceSummaries), expectedDevices)
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

func detectMemoryLeak(rssValues []float64) (bool, string) {
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

	if growthPercent > 35.0 {
		halfIdx := len(rssValues) / 2
		growthFirstHalf := rssValues[halfIdx] - rssValues[0]
		growthSecondHalf := rssValues[len(rssValues)-1] - rssValues[halfIdx]
		if growthSecondHalf > 0.8*growthFirstHalf {
			return true, fmt.Sprintf("continuous monotonic memory growth: %.2f%% growth overall, early growth: %.2f MB, late growth: %.2f MB", growthPercent, growthFirstHalf, growthSecondHalf)
		}
	}
	return false, "memory growth stabilized"
}

var postRedisMem int64
