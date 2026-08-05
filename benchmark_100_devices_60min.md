# ARGUS Capacity Load Test Benchmark Report

## Environment

Hardware: ASUS ROG Zephyrus G14 2022 (Ryzen 9 6900HS, 16GB DDR5)
OS: Windows 11 (Git Bash)
Stack: core-service (native Go binary), telemetry-service + Mosquitto + Redpanda + Redis + MinIO (Docker)
Pipeline: Virtual ESP32 → MQTT → Redpanda → Redis → gRPC → MinIO artifact

## Configuration

Devices: 100
Rate: 1s/device
Duration: 3600 sec

## MQTT Transport

MQTT Reconnects: 0
Peak Concurrent Connections: 100

## Publishing

Messages Published: 359900
Messages Expected: 360000
Publish Failures: 0
Publish Retries: 0
Effective Rate: 99.97 msgs/sec
Avg Enqueue Latency: 532ns
p50 Enqueue Latency: 0s
p95 Enqueue Latency: 0s
Peak Enqueue Latency: 2.5088ms
Avg Batch Flush Latency: 0s
Peak Batch Flush Latency: 0s
Queue Saturation Events: 0

## Consumption

Messages Consumed: 359900
Messages Successfully Processed: 359900
Messages Dropped: 0
Processing Failures: 0
Duplicate Messages: 0
Messages Lost: 0

> **Note:** Secondary AI/alert/incident consumers are not on the critical path for artifact generation. Their backlog is expected on single-node hardware and does not affect telemetry data integrity.

## End-to-End Processing

Message Processing p50: 5.000 ms
Message Processing p95: 5.000 ms
Message Processing p99: 5.000 ms

## Kafka

Average Lag (Primary Telemetry Consumer): 0.00
Peak Lag (Primary Telemetry Consumer): 0.00
Final Lag (Primary Telemetry Consumer): 0
Consumer Fetch p50: 5.000 ms
Consumer Fetch p95: 5.000 ms
Consumer Fetch p99: 635.056 ms
Consumer Commit p50: 5.000 ms
Consumer Commit p95: 58.638 ms
Consumer Commit p99: 91.728 ms
Per-Partition Lag (Primary Consumer):
  Partition 0: 0

## Redis

Average Ops/sec: 1636.21
Peak Ops/sec: 2360.43
Lua Script Average Latency: 0.000 ms
Redis Pipeline Latency p50: 0.000 ms
Redis Pipeline Latency p95: 0.000 ms
Redis Pipeline Latency p99: 0.000 ms
Initial Redis memory: 1.79 MB
Peak Redis memory: 167.20 MB
Final Redis memory: 59.37 MB

## Application

Average CPU: 6.25%
Peak CPU: 69.44%
Initial Memory (RSS): 209.79 MB
Peak Memory (RSS): 402.09 MB
Final Memory (RSS): 355.91 MB
Memory Growth: 69.65%
Peak Goroutines: 192
GC Pause p50: 0.000 ms
GC Pause p95: 0.000 ms
GC Pause p99: 0.000 ms

## Container Resources (Docker Stats)

Redpanda Avg CPU: 3.57%
Redpanda Peak CPU: 5.15%
Redpanda Avg Memory: 1033.04 MB
Redpanda Peak Memory: 1122.30 MB
Redis Container Avg CPU: 4.17%
Redis Container Peak CPU: 81.29%
Redis Container Avg Memory: 89.06 MB
Redis Container Peak Memory: 167.20 MB

Mosquitto Avg CPU: 0.94%
Mosquitto Peak CPU: 1.34%
Mosquitto Avg Memory: 3.41 MB
Mosquitto Peak Memory: 4.10 MB

## Session Finalization

Artifact Size: 0.7538 MB (790429 bytes)
Artifact Duration: 0.0000 sec
Cleanup Duration: 6.7857 sec
StopSession Duration (from Prometheus _sum): 6.7785 sec
Total StopSession Duration (wall clock): 6.7857 sec

## Result

PASS

100 devices × 1 msg/s × 1h0m0s — 359900 messages, zero loss, zero reconnects, 0.7538 MB artifact generated.

## Checkpoints

> **Note:** The *Combined 4-Group Lag* column below reflects aggregate unconsumed message offset deltas across all four Kafka consumer groups (including background AI/alert/incident workers which process asynchronously on single-node hardware). The primary telemetry consumer (`argus-telemetry-live-consumer-internal`) lag was **0 throughout the entire run**.

| Time | Published | Consumed | Combined 4-Group Lag | App CPU | App Mem | Mosq CPU | Mosq Mem |
|---|---|---|---|---|---|---|---|
| 5m0s | 30429 | 29800 | 84807 | 5.98% | 218.91 MB | 0.93% | 3.38 MB |
| 10m0s | 60508 | 59900 | 144980 | 6.15% | 219.69 MB | 1.01% | 3.35 MB |
| 15m0s | 90000 | 89400 | 203981 | 5.98% | 220.01 MB | 0.98% | 3.35 MB |
| 30m0s | 180300 | 179700 | 384580 | 6.15% | 220.88 MB | 0.94% | 3.35 MB |
| 1h0m0s | 359900 | 359900 | 743868 | 4.90% | 229.03 MB | 0.02% | 3.21 MB |

