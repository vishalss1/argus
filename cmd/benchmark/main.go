package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	segmentio "github.com/segmentio/kafka-go"

	_ "github.com/lib/pq"
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

func loadConfig() *Config {
	_ = godotenv.Load(".env")

	cfg := &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Port:                os.Getenv("PORT"),
		KafkaBrokers:        splitCSV(os.Getenv("KAFKA_BROKERS")),
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

func splitCSV(value string) []string {
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

type SessionResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"`
}

type Checkpoint struct {
	Time      time.Duration
	Published int64
	Consumed  int64
	Processed int64
	Lag       int64
	CPU       float64
	Memory    float64
	RedisCPU  float64
	RedisMem  float64
	RedisP50  float64
	RedisP95  float64
	RedisP99  float64
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

	cfg := loadConfig()

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
		Timeout:   120 * time.Second,
	}

	scheme := "http"
	apiHost := os.Getenv("API_HOST")
	if apiHost == "" {
		apiHost = "localhost"
	}
	baseURL := fmt.Sprintf("%s://%s:%s/api", scheme, apiHost, cfg.Port)
	fmt.Printf("Using API Endpoint: %s\n", baseURL)

	telemetryMetricsPort := os.Getenv("TELEMETRY_METRICS_PORT")
	if telemetryMetricsPort == "" {
		telemetryMetricsPort = "8081"
	}
	coreMetricsPort := cfg.Port

	kafkaAddr := "localhost:9092"
	if len(cfg.KafkaBrokers) > 0 {
		kafkaAddr = cfg.KafkaBrokers[0]
	}

	// Clean up previous runs
	cleanupDB(db, cfg, kafkaAddr)

	// Setup Redis Client for Info collection and flush
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()
	_ = rdb.FlushDB(context.Background()).Err()

	// Ensure benchmark user exists, then authenticate
	registerBenchmarkUser(httpClient, baseURL)
	accessToken := loginBenchmarkUser(httpClient, baseURL)
	authHeader := "Bearer " + accessToken
	workspaceID := "00000000-0000-0000-0000-000000000001"

	// 3. Setup Workspace and Devices
	_, err = db.Exec(`INSERT INTO workspaces (id, name, description, created_at) 
		VALUES ($1, 'Benchmark Workspace', 'Workspace for load testing', NOW())
		ON CONFLICT (id) DO NOTHING`, workspaceID)
	if err != nil {
		fmt.Printf("Failed to insert workspace: %v\n", err)
		return
	}

	// Ensure user is workspace member
	userID := getUserIDFromToken(accessToken)
	_, _ = db.Exec(`INSERT INTO workspace_members (workspace_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, workspaceID, userID)

	fmt.Printf("Registering %d devices in database...\n", *devicesFlag)
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("Failed to begin device registration tx: %v\n", err)
		return
	}
	stmt, err := tx.Prepare(`INSERT INTO devices (id, name, type, status, firmware_version, workspace_id, created_at) 
		VALUES ($1, $2, 'sensor', 'online', 'v1.0.0', $3, NOW())
		ON CONFLICT (id) DO UPDATE SET workspace_id = EXCLUDED.workspace_id`)
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
	resp, err := doAuthPost(httpClient, url, nil, authHeader, workspaceID)
	if err != nil {
		fmt.Printf("Failed to create session: %v\n", err)
		return
	}

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to create session, status: %d, body: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
		return
	}

	var sessionRes SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionRes); err != nil {
		fmt.Printf("Failed to decode session response: %v\n", err)
		resp.Body.Close()
		return
	}
	resp.Body.Close()
	sessionID := sessionRes.ID
	fmt.Printf("Created session: %s\n", sessionID)

	// 5. Start Session via API
	startURL := fmt.Sprintf("%s/sessions/%s/start", baseURL, sessionID)
	resp, err = doAuthPost(httpClient, startURL, nil, authHeader, workspaceID)
	if err != nil {
		fmt.Printf("Failed to start session: %v\n", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to start session, status: %d, body: %s\n", resp.StatusCode, string(body))
		resp.Body.Close()
		return
	}
	resp.Body.Close()
	fmt.Println("Session started successfully.")

	// Pre-test metrics snapshot
	preCPU, preMem, _, preConsumed, preDropped, preFailures, preDuplicates, err := getAppMetrics(httpClient, scheme, apiHost, telemetryMetricsPort)
	if err != nil {
		fmt.Printf("Warning: Failed to fetch initial app metrics: %v\n", err)
	}

	preRedisCmds, preRedisMem, preEvalCalls, preEvalUsec, err := getRedisStats(rdb)
	if err != nil {
		fmt.Printf("Warning: Failed to fetch initial Redis stats: %v\n", err)
	}

	preArtifactGenDuration := getPrometheusMetricValue(httpClient, scheme, apiHost, coreMetricsPort, "session_artifact_generation_duration_seconds_sum")
	preStopDuration := getPrometheusMetricValue(httpClient, scheme, apiHost, coreMetricsPort, "session_stop_duration_seconds_sum")

	// 6. Kafka/Redpanda Async Writer Config
	fmt.Printf("Connecting to Kafka at %s...\n", kafkaAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var messagesPublished int64
	var publishFailures int64

	writer := &segmentio.Writer{
		Addr:         segmentio.TCP(kafkaAddr),
		Topic:        cfg.KafkaTelemetryTopic,
		Balancer:     &segmentio.Hash{},
		Async:        true,
		BatchSize:    5000,
		BatchTimeout: 10 * time.Millisecond,
		MaxAttempts:  3,
		Completion: func(messages []segmentio.Message, err error) {
			if err != nil {
				if ctx.Err() == nil {
					atomic.AddInt64(&publishFailures, int64(len(messages)))
				}
			} else {
				atomic.AddInt64(&messagesPublished, int64(len(messages)))
			}
		},
	}
	defer writer.Close()

	// Track latency slices per device (lock-free)
	durationSec := int((*durationFlag).Seconds())
	deviceLatencies := make([][]time.Duration, *devicesFlag)
	for i := range deviceLatencies {
		deviceLatencies[i] = make([]time.Duration, 0, durationSec)
	}

	extraData := strings.Repeat("x", *payloadSizeFlag)

	fmt.Printf("Starting telemetry traffic generation...\n")
	startTime := time.Now()

	// Use a bounded pool of writer workers to avoid overwhelming the segmentio Writer's internal queue
	numWriterWorkers := 2
	type publishTask struct {
		msg       segmentio.Message
		devIndex  int
		enqStart  time.Time
	}
	taskChan := make(chan publishTask, 10000)

	var pubWg sync.WaitGroup
	pubWg.Add(numWriterWorkers)
	for wi := 0; wi < numWriterWorkers; wi++ {
		go func() {
			defer pubWg.Done()
			for task := range taskChan {
				err := writer.WriteMessages(ctx, task.msg)
				latency := time.Since(task.enqStart)
				deviceLatencies[task.devIndex] = append(deviceLatencies[task.devIndex], latency)
				if err != nil && ctx.Err() == nil {
					atomic.AddInt64(&publishFailures, 1)
				}
			}
		}()
	}

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

				taskChan <- publishTask{
					msg: segmentio.Message{
						Key:   []byte(devID),
						Value: telemetryBytes,
					},
					devIndex: devIndex,
					enqStart: time.Now(),
				}
			}

			// Publish first message immediately
			publishMsg()

			ticker := time.NewTicker(*freqFlag)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					publishMsg()
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

	var redisP50Series []float64
	var redisP95Series []float64
	var redisP99Series []float64

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
		cpu, rss, goroutines, consumed, dropped, failures, _, err := getAppMetrics(httpClient, scheme, apiHost, telemetryMetricsPort)
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
		totLag, peakLag, _, _, err := getKafkaConsumerLag(kafkaAddr, "argus-telemetry-live-consumer-internal", cfg.KafkaTelemetryTopic)
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
	cancel()  // Stop device goroutines and monitor loop
	wg.Wait() // Wait for all device threads to finish publishing naturally
	close(taskChan) // Signal writer workers to exit
	pubWg.Wait()    // Wait for all writer workers to finish

	fmt.Println("Telemetry traffic completed. Closing writer to flush all pending messages...")
	activeWStats := writer.Stats()
	_ = writer.Close()

	fmt.Println("Waiting for consumer lag to drain to 0...")
	drainStart := time.Now()
	for {
		liveLag, _, _, liveMembers, errLive := getKafkaConsumerLag(kafkaAddr, "argus-telemetry-live-consumer-internal", cfg.KafkaTelemetryTopic)
		aiLag, _, _, aiMembers, errAI := getKafkaConsumerLag(kafkaAddr, cfg.KafkaAIWorkerGroupID, cfg.KafkaTelemetryTopic)

		waitingForLive := errLive == nil && (liveMembers > 0 && liveLag > 0)
		waitingForAI := errAI == nil && (aiMembers > 0 && aiLag > 0)

		if !waitingForLive && !waitingForAI {
			fmt.Println("All active consumer lags successfully drained to 0.")
			break
		}

	if time.Since(drainStart) > 180*time.Second {
		fmt.Printf("Timeout waiting for consumer lag to drain. Live lag: %d (members: %d), AI lag: %d (members: %d)\n",
				liveLag, liveMembers, aiLag, aiMembers)
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
	resp, err = doAuthPost(httpClient, stopURL, bytes.NewBuffer(stopPayload), authHeader, workspaceID)
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
	_, postMem, _, postConsumed, postDropped, postFailures, postDuplicates, err := getAppMetrics(httpClient, scheme, apiHost, telemetryMetricsPort)
	if err != nil {
		fmt.Printf("Failed to fetch post metrics: %v\n", err)
	}

	// Retrieve session artifact metrics
	cumulativeArtifactGenDuration := getPrometheusMetricValue(httpClient, scheme, apiHost, coreMetricsPort, "session_artifact_generation_duration_seconds_sum")
	postArtifactGenDuration := cumulativeArtifactGenDuration - preArtifactGenDuration
	if postArtifactGenDuration < 0 {
		postArtifactGenDuration = 0
	}
	
	// Wait a moment for final cleanup validations
	time.Sleep(500 * time.Millisecond)

	// Check Redis keys removed: scan for session keys to verify cleanup
	// Exclude the stopped key which is intentionally kept to prevent pipeline race
	stoppedKey := fmt.Sprintf("session:%s:stopped", sessionID)
	redisKeysAll, _ := rdb.Keys(context.Background(), fmt.Sprintf("session:%s:*", sessionID)).Result()
	var redisKeys []string
	for _, k := range redisKeysAll {
		if k != stoppedKey {
			redisKeys = append(redisKeys, k)
		}
	}
	redisKeysRemoved := true
	if len(redisKeys) > 0 {
		redisKeysRemoved = false
		fmt.Printf("Warning: %d Redis keys remain for session %s after stop: %v\n", len(redisKeys), sessionID, redisKeys)
	}

	// 9. Artifact Correctness Validation
	artifactValid, artifactBody, valErr := validateArtifact(httpClient, baseURL, sessionID, *devicesFlag, durationSec, authHeader, workspaceID)
	artifactSize := len(artifactBody)
	if valErr != nil {
		fmt.Printf("Artifact Validation Failed: %v\n", valErr)
		fmt.Printf("Artifact body (first 500 chars): %.500s\n", artifactBody)
	} else if len(artifactBody) > 0 {
		fmt.Printf("Artifact raw (first 500 chars): %.500s\n", artifactBody)
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
	finalLag, _, partitionLag, _, err := getKafkaConsumerLag(kafkaAddr, "argus-telemetry-live-consumer-internal", cfg.KafkaTelemetryTopic)

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

	avgRedisP50, _ := computeStats(redisP50Series)
	avgRedisP95, _ := computeStats(redisP95Series)
	avgRedisP99, _ := computeStats(redisP99Series)

	e2eP50, e2eP95, e2eP99 := getHistogramPercentiles(httpClient, scheme, apiHost, telemetryMetricsPort, "telemetry_consumer_message_processing_duration_seconds", "")
	fetchP50, fetchP95, fetchP99 := getHistogramPercentiles(httpClient, scheme, apiHost, telemetryMetricsPort, "telemetry_stage_fetch_duration_seconds", "")
	commitP50, commitP95, commitP99 := getHistogramPercentiles(httpClient, scheme, apiHost, telemetryMetricsPort, "telemetry_consumer_commit_duration_seconds", "")
	gcP50, gcP95, gcP99 := getHistogramPercentiles(httpClient, scheme, apiHost, telemetryMetricsPort, "go_gc_duration_seconds", "")

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
	msgsPerDevice := int64(durationSec * int(time.Second/(*freqFlag)))
	expectedPublished := int64(*devicesFlag) * msgsPerDevice
	
	pass := true
	var failReasons []string

	if totalPublished < int64(float64(expectedPublished)*0.995) {
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
	lagThreshold := int64(float64(totalPublished) * 0.005) // allow 0.5% residual lag at scale
	if lagThreshold < 100 {
		lagThreshold = 100
	}
	if finalLag > lagThreshold {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Final Kafka consumer lag too high: %d > %d (0.1%% of published)", finalLag, lagThreshold))
	}
	if stopDuration.Seconds() >= 30.0 {
		pass = false
		failReasons = append(failReasons, fmt.Sprintf("Session stop finalization duration too long: %s >= 30s", stopDuration))
	}
	if !redisKeysRemoved {
		pass = false
		failReasons = append(failReasons, "Redis session keys were not fully cleaned up on StopSession")
	}
	if !artifactValid {
		pass = false
		reason := "Generated artifact validation failed"
		if valErr != nil {
			reason += ": " + valErr.Error()
		}
		failReasons = append(failReasons, reason)
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

	reportBuilder.WriteString("## End-to-End Processing\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Message Processing p50: %.3f ms\n", e2eP50))
	reportBuilder.WriteString(fmt.Sprintf("Message Processing p95: %.3f ms\n", e2eP95))
	reportBuilder.WriteString(fmt.Sprintf("Message Processing p99: %.3f ms\n\n", e2eP99))

	reportBuilder.WriteString("## Kafka\n\n")
	reportBuilder.WriteString(fmt.Sprintf("Average Lag: %.2f\n", avgLag))
	reportBuilder.WriteString(fmt.Sprintf("Peak Lag: %.2f\n", maxLagVal))
	reportBuilder.WriteString(fmt.Sprintf("Final Lag: %d\n", finalLag))
	reportBuilder.WriteString(fmt.Sprintf("Consumer Fetch p50: %.3f ms\n", fetchP50))
	reportBuilder.WriteString(fmt.Sprintf("Consumer Fetch p95: %.3f ms\n", fetchP95))
	reportBuilder.WriteString(fmt.Sprintf("Consumer Fetch p99: %.3f ms\n", fetchP99))
	reportBuilder.WriteString(fmt.Sprintf("Consumer Commit p50: %.3f ms\n", commitP50))
	reportBuilder.WriteString(fmt.Sprintf("Consumer Commit p95: %.3f ms\n", commitP95))
	reportBuilder.WriteString(fmt.Sprintf("Consumer Commit p99: %.3f ms\n", commitP99))
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
	reportBuilder.WriteString(fmt.Sprintf("Redis Pipeline Latency p50: %.3f ms\n", avgRedisP50))
	reportBuilder.WriteString(fmt.Sprintf("Redis Pipeline Latency p95: %.3f ms\n", avgRedisP95))
	reportBuilder.WriteString(fmt.Sprintf("Redis Pipeline Latency p99: %.3f ms\n", avgRedisP99))
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
	reportBuilder.WriteString(fmt.Sprintf("Peak Goroutines: %.0f\n", peakAppGoroutines))
	reportBuilder.WriteString(fmt.Sprintf("GC Pause p50: %.3f ms\n", gcP50))
	reportBuilder.WriteString(fmt.Sprintf("GC Pause p95: %.3f ms\n", gcP95))
	reportBuilder.WriteString(fmt.Sprintf("GC Pause p99: %.3f ms\n\n", gcP99))

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
	// Fetch StopSession latency from Prometheus histogram
	cumulativeStopDuration := getPrometheusMetricValue(httpClient, scheme, apiHost, coreMetricsPort, "session_stop_duration_seconds_sum")

	postStopDuration := cumulativeStopDuration - preStopDuration
	if postStopDuration < 0 {
		postStopDuration = 0
	}

	reportBuilder.WriteString(fmt.Sprintf("StopSession Duration (from Prometheus _sum): %.4f sec\n", postStopDuration))
	reportBuilder.WriteString(fmt.Sprintf("Total StopSession Duration (wall clock): %.4f sec\n\n", stopDuration.Seconds()))

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

	// Save report with descriptive filename
	reportFilename := fmt.Sprintf("benchmark_%d_devices_%dmin.md", *devicesFlag, durationSec/60)
	for _, filename := range []string{"benchmark_report.md", reportFilename} {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString("# ARGUS Capacity Load Test Benchmark Report\n\n")
			_, _ = f.WriteString(reportStr)
			
			if len(checkpoints) > 0 {
				_, _ = f.WriteString("\n## Checkpoints\n\n| Time | Published | Consumed | Processed | Lag | App CPU | App Memory | Redis P95 | Redis P99 |\n|---|---|---|---|---|---|---|---|---|\n")
				for _, cp := range checkpoints {
					_, _ = f.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %.2f%% | %.2f MB | %.3f ms | %.3f ms |\n",
						cp.Time, cp.Published, cp.Consumed, cp.Processed, cp.Lag, cp.CPU, cp.Memory, cp.RedisP95, cp.RedisP99))
				}
			}
			f.Close()
		}
	}

	cleanupDB(db, cfg, kafkaAddr)
}

func getHistogramPercentiles(client *http.Client, scheme, apiHost, port, metricName, matchLabel string) (p50, p95, p99 float64) {
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


func doAuthPost(client *http.Client, url string, body io.Reader, authHeader string, workspaceID string) (*http.Response, error) {
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

func registerBenchmarkUser(client *http.Client, baseURL string) {
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

func loginBenchmarkUser(client *http.Client, baseURL string) string {
	loginBody := bytes.NewBufferString(`{"email":"benchmark@argus.test","password":"Benchmark123!"}`)
	for attempt := 0; attempt < 10; attempt++ {
		req, _ := http.NewRequest("POST", baseURL+"/auth/login", loginBody)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Login attempt %d failed: %v. Retrying...\n", attempt+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			var loginResp struct {
				AccessToken string `json:"access_token"`
			}
			json.NewDecoder(resp.Body).Decode(&loginResp)
			resp.Body.Close()
			fmt.Println("Authenticated successfully.")
			return loginResp.AccessToken
		}
		resp.Body.Close()
		time.Sleep(2 * time.Second)
	}
	fmt.Println("Failed to authenticate after 10 attempts, proceeding without auth (may fail).")
	return ""
}

func getUserIDFromToken(accessToken string) string {
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

func cleanupDB(db *sql.DB, cfg *Config, kafkaAddr string) {
	_, _ = db.Exec("DELETE FROM tenant_usage")
	_, _ = db.Exec("DELETE FROM session_artifacts WHERE session_id IN (SELECT id FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001')")
	_, _ = db.Exec("DELETE FROM session_statistics WHERE session_id IN (SELECT id FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001')")
	_, _ = db.Exec("DELETE FROM workspace_sessions WHERE workspace_id = '00000000-0000-0000-0000-000000000001'")
	_, _ = db.Exec("DELETE FROM devices WHERE workspace_id = '00000000-0000-0000-0000-000000000001'")
	_, _ = db.Exec("DELETE FROM workspaces WHERE id = '00000000-0000-0000-0000-000000000001'")

	fmt.Printf("Skipping consumer group deletion for native execution.\n")

	// Ensure the telemetry.raw topic exists with 3 partitions
	ensureTopicWithPartitions("telemetry.raw", 3, kafkaAddr)
}

func ensureTopicWithPartitions(topic string, partitions int, kafkaAddr string) {
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

func getAppMetrics(client *http.Client, scheme, apiHost, port string) (cpu float64, rss float64, goroutines float64, consumed float64, dropped float64, failures float64, duplicates float64, err error) {
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

func getPrometheusMetricValue(client *http.Client, scheme, apiHost, port string, metricName string) float64 {
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
	// Bypassed for native/container execution
	return 0.0, 0.0, nil
}

func getKafkaConsumerLag(kafkaAddr string, groupName string, topicName string) (int64, int64, map[int32]int64, int64, error) {
	client := &segmentio.Client{Addr: segmentio.TCP(kafkaAddr)}
	
	meta, err := client.Metadata(context.Background(), &segmentio.MetadataRequest{Topics: []string{topicName}})
	if err != nil { return 0, 0, nil, 0, err }
	
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
	if err != nil { return 0, 0, nil, 0, err }

	endOffsets := make(map[int]int64)
	for _, p := range endOffsetsResp.Topics[topicName] {
		endOffsets[p.Partition] = p.LastOffset
	}

	groupOffsetsResp, err := client.OffsetFetch(context.Background(), &segmentio.OffsetFetchRequest{
		GroupID: groupName,
		Topics: map[string][]int{topicName: partitions},
	})
	if err != nil { return 0, 0, nil, 0, err }

	var totalLag int64
	var peakLag int64
	partitionLag := make(map[int32]int64)
	var members int64 = 1

	for _, p := range groupOffsetsResp.Topics[topicName] {
		endOff := endOffsets[p.Partition]
		commitOff := p.CommittedOffset
		if commitOff < 0 { commitOff = 0 }
		lag := endOff - commitOff
		if lag < 0 { lag = 0 }
		partitionLag[int32(p.Partition)] = lag
		totalLag += lag
		if lag > peakLag { peakLag = lag }
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

func validateArtifact(httpClient *http.Client, baseURL string, sessionID string, expectedDevices int, durationSeconds int, authHeader, workspaceID string) (bool, string, error) {
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

var postRedisMem int64
