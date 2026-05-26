# ARGUS

ARGUS is a Go-based modular monolith for fleet monitoring and device control. It provides a REST API for devices, telemetry, commands, shadows, OTA firmware, and rules, with optional MQTT, Kafka/Redpanda, Redis, and MinIO integrations.

## What It Does

- Device registry with CRUD
- Heartbeat ingestion and last-seen tracking
- Telemetry ingestion over HTTP and MQTT
- Command creation, retrieval, ACK/NACK tracking
- Device shadow state in Redis
- OTA firmware upload, storage, and deployment manifests
- Telemetry rule evaluation and alert generation
- Prometheus metrics, Swagger docs, and Grafana/Loki support

## Stack

- Go
- PostgreSQL
- Redis
- MinIO
- MQTT/Mosquitto
- Kafka/Redpanda
- Prometheus
- Grafana
- Loki

## Project Structure

```text
cmd/api/                Application entrypoint
internal/app/           Bootstrap and wiring
internal/config/        Environment loading and defaults
internal/domain/        Domain models, services, interfaces, errors
internal/infrastructure/ PostgreSQL, Redis, MQTT, Kafka, MinIO, rules, logging
internal/transport/http/ HTTP router, handlers, DTOs, middleware
migrations/             Database schema migrations
docs/swagger/           Generated OpenAPI/Swagger output
deployments/compose/    Local Docker Compose setup and observability config
```

## Architecture

The code follows a layered approach:

- HTTP handlers decode requests and call services
- Domain services contain business rules and validation
- Repository interfaces live in the domain layer
- Infrastructure packages implement those interfaces
- `internal/app/bootstrap.go` wires everything together

Telemetry and command flows can optionally be decorated with Kafka publishing. Telemetry can also be ingested from MQTT. Shadow state is stored in Redis, and firmware artifacts are stored in MinIO.

## Runtime Behavior

- `cmd/api/main.go` starts the application
- `internal/config/config.go` loads `.env` and environment variables
- `internal/infrastructure/postgres/postgres.go` connects to PostgreSQL and runs migrations
- `internal/transport/http/router/router.go` registers all routes
- `internal/transport/http/middleware/metrics.go` exposes Prometheus metrics

## HTTP API

### Health and Docs

- `GET /healthz`
- `GET /metrics`
- `GET /docs`

### Devices

- `GET /devices`
- `POST /devices`
- `GET /devices/{deviceID}`
- `PUT /devices/{deviceID}`
- `DELETE /devices/{deviceID}`
- `POST /devices/{deviceID}/heartbeat`

### Telemetry

- `POST /devices/{deviceID}/telemetry`

### Commands

- `GET /devices/{deviceID}/commands`
- `POST /devices/{deviceID}/commands`
- `GET /devices/{deviceID}/commands/{commandID}`
- `POST /devices/{deviceID}/commands/{commandID}/ack`
- `POST /devices/{deviceID}/commands/{commandID}/nack`

### Shadows

- `GET /devices/{deviceID}/shadow`
- `PUT /devices/{deviceID}/shadow/desired`
- `PUT /devices/{deviceID}/shadow/reported`

### OTA

- `GET /ota/firmware`
- `POST /ota/firmware`
- `GET /ota/firmware/{firmwareID}`
- `GET /devices/{deviceID}/ota`
- `POST /devices/{deviceID}/ota`
- `GET /devices/{deviceID}/ota/{deploymentID}/manifest`
- `POST /devices/{deviceID}/ota/{deploymentID}/ack`
- `POST /devices/{deviceID}/ota/{deploymentID}/nack`

### Rules

- `GET /rules`
- `POST /rules`
- `GET /rules/{ruleID}`
- `PUT /rules/{ruleID}`
- `DELETE /rules/{ruleID}`
- `GET /alerts`

## Local Development

### Prerequisites

- Go 1.26.3 or newer
- PostgreSQL
- Redis
- MinIO
- Optional: MQTT broker
- Optional: Kafka-compatible broker

### Environment

Create a `.env` file in the repository root with at least:

```env
DATABASE_URL=postgres://argus:argus@localhost:5432/argus?sslmode=disable
PORT=8080
```

Common optional variables:

```env
MQTT_BROKER_URL=tcp://localhost:1883
MQTT_CLIENT_ID=argus-api
MQTT_STATE_TOPIC=devices/+/state
MQTT_TELEMETRY_TOPIC=devices/+/telemetry
KAFKA_BROKERS=localhost:9092
KAFKA_TELEMETRY_TOPIC=argus.telemetry
KAFKA_COMMAND_TOPIC=argus.commands
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=argus
MINIO_SECRET_KEY=arguspassword
MINIO_BUCKET=argus-firmware
MINIO_USE_SSL=false
HEARTBEAT_INTERVAL_SECONDS=30
HEARTBEAT_TIMEOUT_SECONDS=45
```

## MQTT Device Presence

ARGUS uses MQTT retained state messages and Last Will and Testament for near-realtime device presence. Devices should publish telemetry and commands on:

- `devices/{deviceId}/state`
- `devices/{deviceId}/telemetry`
- `devices/{deviceId}/events`
- `devices/{deviceId}/commands`

On MQTT connect, an ESP32 should use keepalive `10` seconds, configure an LWT on `devices/{deviceId}/state` with QoS 1, retain enabled, and payload `{"status":"offline","timestamp":"<iso8601>"}`. After a successful connect it should publish `{"status":"online","timestamp":"<iso8601>"}` to the same state topic with QoS 1 and retain enabled.

The backend subscribes to `devices/+/state` with a persistent MQTT session. Mosquitto retained messages rebuild the in-memory presence cache on backend startup, and presence changes are pushed immediately to WebSocket clients as `{"type":"device_presence","deviceId":"esp32-01","status":"offline","timestamp":"2026-05-26T10:00:00Z"}`. Heartbeat timeout is retained only as a safety fallback for stale devices; it is not the primary online/offline detector.

For Mosquitto, enable persistence:

```conf
persistence true
```

### Run With Docker Compose

The repository includes a local stack under `deployments/compose/`:

```bash
docker compose -f deployments/compose/docker-compose.yml up -d
```

This brings up PostgreSQL, Redis, MinIO, Redpanda, Mosquitto, Prometheus, Loki, Promtail, and Grafana.

### Run The API

```bash
go run ./cmd/api
```

The API listens on `PORT` and serves Swagger at `/docs`.

## Database Migrations

Migrations live in `migrations/` and are applied at startup by the PostgreSQL bootstrap path.

## Swagger

The Swagger/OpenAPI output is generated from handler annotations.

Regenerate docs with:

```bash
make swagger
```

## Observability

- Prometheus metrics are exposed at `/metrics`
- Loki and Promtail are configured under `deployments/compose/`
- Grafana dashboards and datasources are provisioned automatically from `deployments/compose/grafana/`

## Notes

- The `Dockerfile` is currently empty.
- `docs/swagger/` is generated output and should not be edited manually.
- `report.md` contains a detailed codebase walkthrough if you need the full wiring and file-by-file explanation.
