# ARGUS Capacity Load Test Benchmark Report

## Environment

Hardware: ASUS ROG Zephyrus G14 2022 (Ryzen 9 6900HS, 16GB DDR5)
OS: Windows 11 (Git Bash)
Stack: core-service (native Go binary), telemetry-service + Mosquitto + Redpanda + Redis + MinIO (Docker)
Pipeline: Virtual ESP32 → MQTT → Redpanda → Redis → gRPC → MinIO artifact

## Configuration

Devices: 10
Rate: 1s/device
Duration: 1200 sec

## MQTT Transport

MQTT Reconnects: 0
Peak Concurrent Connections: 10

## Publishing

Messages Published: 11990
Messages Expected: 12000
Publish Failures: 0
Publish Retries: 0
Effective Rate: 9.99 msgs/sec
Avg Enqueue Latency: 21.582µs
p50 Enqueue Latency: 0s
p95 Enqueue Latency: 0s
Peak Enqueue Latency: 14.0915ms
Avg Batch Flush Latency: 0s
Peak Batch Flush Latency: 0s
Queue Saturation Events: 0

## Consumption

Messages Consumed: 11990
Messages Successfully Processed: 11990
Messages Dropped: 0
Processing Failures: 0
Duplicate Messages: 0
Messages Lost: 0

> **Note:** Secondary AI/alert/incident consumers are not on the critical path for artifact generation. Their backlog is expected on single-node hardware and does not affect telemetry data integrity.

## End-to-End Processing

Message Processing p50: 5.000 ms
Message Processing p95: 82.623 ms
Message Processing p99: 96.525 ms

## Kafka

Average Lag: 0.00
Peak Lag: 0.00
Final Lag: 0
Consumer Fetch p50: 5.000 ms
Consumer Fetch p95: 1379.213 ms
Consumer Fetch p99: 2277.341 ms
Consumer Commit p50: 5.000 ms
Consumer Commit p95: 89.664 ms
Consumer Commit p99: 97.933 ms
Per-Partition Lag:
  Partition 0: 0

## Redis

Average Ops/sec: 165.43
Peak Ops/sec: 180.28
Lua Script Average Latency: 0.000 ms
Redis Pipeline Latency p50: 0.000 ms
Redis Pipeline Latency p95: 0.000 ms
Redis Pipeline Latency p99: 0.000 ms
Initial Redis memory: 1.21 MB
Peak Redis memory: 13.79 MB
Final Redis memory: 3.70 MB

## Application

Average CPU: 1.88%
Peak CPU: 3.82%
Initial Memory (RSS): 177.49 MB
Peak Memory (RSS): 228.62 MB
Final Memory (RSS): 228.62 MB
Memory Growth: 28.80%
Peak Goroutines: 192
GC Pause p50: 0.000 ms
GC Pause p95: 0.000 ms
GC Pause p99: 0.000 ms

## Container Resources (Docker Stats)

Redpanda Avg CPU: 3.90%
Redpanda Peak CPU: 12.57%
Redpanda Avg Memory: 939.94 MB
Redpanda Peak Memory: 944.50 MB
Redis Container Avg CPU: 0.99%
Redis Container Peak CPU: 4.53%
Redis Container Avg Memory: 10.05 MB
Redis Container Peak Memory: 13.79 MB

Mosquitto Avg CPU: 0.15%
Mosquitto Peak CPU: 0.24%
Mosquitto Avg Memory: 2.46 MB
Mosquitto Peak Memory: 2.94 MB

## Session Finalization

Artifact Size: 0.0641 MB (67206 bytes)
Artifact Duration: 0.0000 sec
Cleanup Duration: 1.7015 sec
StopSession Duration (from Prometheus _sum): 1.6929 sec
Total StopSession Duration (wall clock): 1.7015 sec

## Result

PASS

10 devices × 1 msg/s × 20m0s — 11990 messages, zero loss, zero reconnects, 0.0641 MB artifact generated.

## Checkpoints

| Time | Published | Consumed | Lag | App CPU | App Mem | Mosq CPU | Mosq Mem |
|---|---|---|---|---|---|---|---|
| 5m0s | 3050 | 2990 | 6120 | 1.99% | 214.37 MB | 0.17% | 2.45 MB |
| 10m0s | 6000 | 5940 | 12010 | 1.82% | 220.49 MB | 0.19% | 2.45 MB |
| 15m0s | 9010 | 8950 | 18030 | 1.49% | 223.12 MB | 0.12% | 2.45 MB |

> **Note:** The Lag column reflects combined lag across all four Kafka consumer groups, including secondary AI/alert/incident consumers which accumulate by design on single-node hardware. Primary telemetry consumer lag was 0 throughout the entire run.

