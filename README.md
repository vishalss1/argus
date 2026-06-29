<div align="center">

# ⚡ ARGUS

### Distributed IoT Fleet Monitoring & Control Platform

A production-grade system spanning **Go microservices**, a **C++ ESP32 SDK**, and a **React dashboard** — handling provisioning, telemetry, command dispatch, OTA firmware updates, digital twins, and local AI inference with cryptographic security at every layer.

[![E2E CI](https://github.com/Vishalss1/argus/actions/workflows/e2e-ci.yml/badge.svg)](https://github.com/Vishalss1/argus/actions/workflows/e2e-ci.yml)
[![Go CI](https://github.com/Vishalss1/argus/actions/workflows/go-ci.yml/badge.svg)](https://github.com/Vishalss1/argus/actions/workflows/go-ci.yml)
[![SDK CI](https://github.com/Vishalss1/argus/actions/workflows/sdk-ci.yml/badge.svg)](https://github.com/Vishalss1/argus/actions/workflows/sdk-ci.yml)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![React](https://img.shields.io/badge/React_18-TypeScript-61DAFB?style=flat&logo=react)
![ESP32](https://img.shields.io/badge/ESP32-C++17-E7352C?style=flat&logo=esp32)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL+pgvector-16-4169E1?style=flat&logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat&logo=redis)
![Kafka](https://img.shields.io/badge/Redpanda-Kafka_Engine-E50695?style=flat)
![MQTT](https://img.shields.io/badge/MQTT-Mosquitto-660066?style=flat)
![ONNX](https://img.shields.io/badge/ONNX_Runtime-Local_AI-005CED?style=flat)
![Docker](https://img.shields.io/badge/Docker-10_Containers-2496ED?style=flat&logo=docker)
![Kubernetes](https://img.shields.io/badge/Kubernetes-Kustomize-326CE5?style=flat&logo=kubernetes)
![License](https://img.shields.io/badge/License-MIT-22c55e?style=flat)

</div>

---

## Table of Contents

[What Is ARGUS](#what-is-argus) · [Engineering Decisions](#engineering-decisions) · [Architecture](#architecture) · [Features](#features) · [OTA Security Model](#ota-security-model) · [CI/CD](#cicd-pipelines) · [Load Tests](#load-test-results) · [Tech Stack](#tech-stack) · [Quick Start](#quick-start) · [API Endpoints](#api-endpoints) · [Testing](#testing)

---

## What Is ARGUS

Most IoT projects stop at "sensor sends data to a dashboard." ARGUS goes further — every ESP32 device is a remotely observable, controllable, and versioned compute node with cryptographic identity, fleet-wide OTA deployments, and local AI that detects anomalies without sending a single byte to an external embedding API.

Built as a learning project that grew into a real distributed system. Two Go microservices — one for fleet management and device control, one for high-volume telemetry ingestion and AI analytics — wired together with Kafka, MQTT, gRPC, and PostgreSQL, containerised in Docker with full Kubernetes manifests.

---

## Engineering Decisions

A few design choices that shaped how ARGUS works.

**Fleet-wide OTA with per-device identity.** Provisioning generates a unique `config_{id}.ino` sketch for each device — UUID, API key, mTLS certs, and OTA public key written to NVS flash. After that, an identical firmware binary deploys to every device in the fleet. No per-device values compiled in. Identity survives updates.

**ESP32 C++ that tests without hardware.** A POSIX/OpenSSL HAL layer stubs out `WiFiClientSecure`, `PubSubClient`, `HTTPClient`, `Preferences`, and `Update.h` using real TCP sockets. The SDK compiles and runs on Linux x86_64, making actual HTTPS and MQTT connections to a Dockerised backend. libsodium and mbedTLS run natively. Ed25519 OTA verification is real, not mocked — which is what makes CI possible.

**Auditable command lifecycle.** Commands move through `PENDING → DISPATCHED → ACKED / NACKED / TIMEOUT` — written to Postgres, published to Kafka, consumed by an MQTT bridge, delivered over Mosquitto, with the device response flowing back through MQTT → Kafka → Postgres. A background goroutine sweeps unacknowledged commands to `TIMED_OUT`. Every transition is recorded. Core Service handles only low-volume MQTT traffic (state, commands, OTA) while Telemetry Service independently ingests high-volume device telemetry directly from the broker.

**Four-layer OTA verification.** Before a single byte touches flash: TLS cert pinning prevents MITM, SHA-256 checksum is verified chunk-by-chunk, Ed25519 signature (libsodium) is validated against the provisioned public key, and key ID is checked against what was written at provisioning. Any failure aborts the update and publishes a NACK.

**Dual authentication.** Each device gets an ECDSA client certificate signed by an internal CA (validated at TLS handshake) and a per-device API key (SHA-256 hashed, prefix-indexed, validated on every HTTP request). Both layers are independent — either alone is sufficient to reject an unauthorised device.

**Local AI, no external APIs.** The telemetry service runs `all-MiniLM-L6-v2` via ONNX Runtime for 384-dim embeddings entirely in-process. Vectors land in pgvector. RAG queries route across device, fleet, historical, and incident handlers before hitting Groq for reasoning. Anomaly detection runs EWMA + Welford's online algorithm on live streams.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│               Frontend  (React 18 + TypeScript)             │
│           nginx  ──►  /api proxy  ──►  core-service         │
└────────────────────────────┬────────────────────────────────┘
                             │ HTTP/REST + WebSocket
                 ┌───────────┴────────────┐
                 │     Core Service       │ ◄──gRPC──► ┌──────────────────────────┐
                 │       Go  :8080        │            │    Telemetry Service     │
                 │   gRPC server :50051   │            │      Go  :8081           │
                 │                        │            │    gRPC server :50052     │
                 │  Device registry       │            │                          │
                 │  Fleet management      │            │  MQTT subscriber         │
                 │  Command dispatch      │            │  (argus/+/telemetry)     │
                 │  OTA lifecycle         │            │  Kafka consumer          │
                 │  Shadow state          │            │  Anomaly detection       │
                 │  Internal CA           │            │  ONNX embeddings         │
                 │  JWT auth              │            │  pgvector RAG            │
                 │  WebSocket broadcast   │            │  Rules + alerts          │
                 └──┬───┬───┬────────────┘            └──┬───┬───┬───┬───────────┘
                    │   │   │                             │   │   │   │
                   PG Redis MQTT                      PG Redis Kafka MinIO
                  +vec  (state,     Mosquitto           +vec        /RP
                       commands,
                       OTA only)
                        │                                    │
                        │ MQTT                               │ Kafka
                 ┌──────┴──────────────┐          ┌──────────┴──────────┐
                 │    ESP32 Devices    │          │  telemetry.raw      │
                 │   (argus_sdk C++)   │          │  (Telemetry → Core  │
                 │  NVS identity       │          │   for WS broadcast) │
                 │  mTLS + API key     │          └─────────────────────┘
                 │  Ed25519 OTA verify │
                 │  State machine      │
                 └─────────────────────┘
```

| Channel | Flow | Purpose |
|:--------|:-----|:--------|
| gRPC | Core ↔ Telemetry | AI queries, snapshots, rules, incidents, device resolution |
| MQTT `argus/+/telemetry` | Device → Telemetry | High-volume telemetry ingestion (direct from broker, QoS 0) |
| MQTT `argus/+/state` | Device → Core | Device state updates, LWT presence |
| MQTT `argus/+/results` | Device → Core | Command result payloads |
| MQTT `argus/+/ota/status` | Device → Core | OTA progress updates |
| Kafka `telemetry.raw` | Telemetry → Core | Ingested telemetry for WebSocket broadcast to frontend |
| Kafka `telemetry.incidents` | Telemetry → Core | Anomaly incidents |


---

## Features

| Feature | What It Does |
|:--------|:-------------|
| **Fleet Provisioning** | Create N devices from one API call. Each gets a unique config sketch with UUID, API key, mTLS certs, and OTA public key — all written to ESP32 NVS flash. |
| **OTA Firmware Deployment** | Fleet-wide or per-device. Four-layer security: TLS cert pinning, SHA-256 checksum, Ed25519 signature, key ID validation. Corrupted binaries never reach flash. |
| **Command & Control** | Full `PENDING → DISPATCHED → ACKED / NACKED / TIMEOUT` lifecycle. Postgres audit trail, Kafka distribution, MQTT delivery, WebSocket push to dashboard. |
| **Digital Twins** | Redis-backed desired/reported state per device. Drift detection flags when the two diverge. |
| **Local AI Inference** | ONNX Runtime runs `all-MiniLM-L6-v2` in-process — no external API. pgvector semantic search, event deduplication, RAG across four handler types. |
| **Anomaly Detection** | EWMA + Welford's online algorithm on live telemetry streams. Real-time Z-score anomalies trigger alerts through the rules engine. |
| **RAG Query Engine** | Routes queries across device, fleet, historical, and incident knowledge bases. Groq provides reasoning over local vector context. |
| **Session Lifecycle** | All telemetry scoped to sessions: start → run → stop. On session end: device summaries, metric aggregates, hourly rollups, and incident archives as MinIO artifacts. |
| **16-Page Dashboard** | Landing · Login · Workspaces · Fleet Overview · Devices · Device Detail · Sessions · Telemetry · Commands · OTA · Alerts · AI Insights · Observability · Docs |
| **Observability Stack** | Prometheus metrics, Grafana dashboards, Loki + Promtail log aggregation, pre-configured alerting rules — all in Docker. |
| **Multi-Workspace** | Workspace isolation with per-workspace JWT scoping, rate limiting, and resource ownership. |
| **Dual Authentication** | ECDSA client certificates (mTLS) + per-device API keys (SHA-256 hashed). Both layers are independent and sufficient on their own. |

---

## OTA Security Model

```
Layer 1 — TLS + Certificate Pinning
          ESP32 pins the server cert SHA-256 fingerprint at compile time.
          A rogue CA cannot intercept the connection.

Layer 2 — SHA-256 Checksum
          Computed server-side on upload. Verified chunk-by-chunk during download.

Layer 3 — Ed25519 Digital Signature  (libsodium)
          Server signs the SHA-256 hex string with the fleet private key.
          Device verifies using the public key written to NVS at provisioning.
          Key ID in the manifest is checked against the key ID in firmware.

Layer 4 — Abort on Any Failure
          Wrong checksum · bad signature · unknown key ID
          → update aborted, NACK published, existing firmware untouched.
```

Keypairs generated by `cmd/ota-keygen/`. Public key lives in NVS. Private key is environment config only, never in the binary.

---

## CI/CD Pipelines

Three pipelines. All three must be green before `master` merges.

**`e2e-ci.yml`** — full stack, triggers on `master`
```
1.  Generate TLS certs + Ed25519 OTA keypair
2.  Start 10 infrastructure containers via Docker Compose
3.  Provision test device via Go bootstrap script  (real API calls)
4.  Build C++ SDK test runner natively  (CMake + libsodium + OpenSSL)
5.  Run simulator for 120s  — telemetry 5s, heartbeat 30s, OTA poll 60s

    ✅  Telemetry  — asserted via mosquitto_sub
    ✅  Commands   — ping dispatched, "Pong from hardware" asserted on /results
    ✅  OTA        — firmware uploaded and signed, simulator receives manifest,
                    verifies SHA-256 + Ed25519, flash write and ACK asserted
```

**`go-ci.yml`** — backend + frontend, triggers on `master`
Both Go services built with ONNX Runtime. Full unit test suite with `-race`. React TypeScript build validated.

**`sdk-ci.yml`** — ESP32 SDK, triggers on `feat/sdk-dev`
Arduino-cli compile check against ESP32 core. Native C++ integration test (120s) against a live backend over real HTTPS + MQTT.


---

## Load Test Results

Full reports: [`benchmark_10_devices_20min.md`](./benchmark_10_devices_20min.md) · [`benchmark_100_devices_60min.md`](./benchmark_100_devices_60min.md)

> Hardware: ASUS ROG Zephyrus G14 2022 (Ryzen 9 6900HS, 16 GB DDR5) — core-service as a native Go binary, all infra in Docker.
> Pipeline: Virtual ESP32 → MQTT (Telemetry Service) → Kafka → Core Service WebSocket → Frontend.
> MQTT topic split: Core Service handles `state`, `results`, `ota/status`. Telemetry Service handles `telemetry` independently.

| | 10 devices · 20 min | 100 devices · 60 min |
|:--|:--|:--|
| Messages | 11,990 / 12,000 | 359,900 / 360,000 |
| Loss | **0** | **0** |
| MQTT reconnects | **0** | **0** |
| Publish failures | 0 | 0 |
| Effective rate | 9.99 msg/s | 99.97 msg/s |
| Avg enqueue latency | 21.6 µs | **532 ns** |
| Primary Kafka lag | 0 throughout | 0 throughout |
| App avg CPU | 1.88% | 6.25% |
| App peak RSS | 228 MB | 402 MB |
| Artifact generated | 0.06 MB | 0.75 MB |
| Result | ✅ PASS | ✅ PASS |

At 100 devices the avg enqueue latency dropped to 532 ns from 21.6 µs at 10 devices — Kafka's producer batching becomes more efficient as throughput increases.

---

## Tech Stack

| Domain | Technologies |
|:-------|:-------------|
| **Backend** | Go 1.22+, gRPC + Protobuf, chi router, Swagger/OpenAPI |
| **Frontend** | React 18, TypeScript 5, Vite, TanStack Query & Table, Recharts, Framer Motion |
| **Device SDK** | C++17, Arduino Core for ESP32, libsodium, mbedTLS, PubSubClient, ArduinoJson |
| **Data** | PostgreSQL 16 + pgvector, Redis 7 |
| **Messaging** | Eclipse Mosquitto (MQTT 3.1.1), Redpanda (Kafka-compatible) |
| **Storage** | MinIO — firmware artifacts + presigned URLs |
| **AI / ML** | ONNX Runtime (local embeddings), Groq API (LLM reasoning) |
| **Security** | Ed25519, SHA-256, ECDSA mTLS, JWT RS256, TLS cert pinning |
| **Observability** | Prometheus, Grafana, Loki, Promtail |
| **Deploy** | Docker Compose (10 containers), Kubernetes manifests + Kustomize, nginx |
| **CI/CD** | GitHub Actions, CMake, arduino-cli, race detector |

---

## Project Structure

```
argus/
├── services/
│   ├── core-service/            # Go API  — HTTP :8080, gRPC :50051
│   │   └── internal/
│   │       ├── domain/          # Device, fleet, OTA, session, cert, policy
│   │       ├── transport/       # HTTP handlers, gRPC, auth + mTLS middleware
│   │       ├── infrastructure/  # Postgres, Redis, Kafka, MQTT, MinIO
│   │       └── firmware/        # Arduino .ino template generation
│   └── telemetry-service/       # AI + analytics  — HTTP :8081, gRPC :50052
│       └── internal/
│           ├── ai/              # Anomaly, embeddings, RAG, actions, memory
│           ├── domain/          # Telemetry service, rule engine
│           └── infrastructure/  # MQTT (direct ingestion), Kafka, Redis, Postgres+pgvector, ONNX, Groq
├── shared/                      # Shared Go modules + Protobuf definitions
├── frontend/                    # React 18 SPA — 16 pages
├── argus_sdk/src/               # C++ ESP32 SDK
│                                # argus · argus_nvs · argus_http · argus_mqtt
│                                # argus_ota · argus_security · argus_state_machine
│                                # argus_rollback · argus_time · argus_diag
├── sdk_tests/
│   ├── hal/                     # POSIX/OpenSSL HAL stubs
│   ├── bootstrap/               # Go device provisioning tool
│   └── ci/                      # CI compose override + cert generation
├── deployments/
│   ├── compose/                 # Docker Compose — 10 containers
│   └── k8s/                     # Kubernetes manifests + Kustomize
├── cmd/
│   ├── ota-keygen/              # Ed25519 keypair generator
│   └── gen-certs/               # TLS cert generator
├── benchmark_10_devices_20min.md
└── benchmark_100_devices_60min.md
```


---

## Quick Start

```bash
# 1. Generate OTA signing keys
cd sdk_tests/bootstrap
go run bootstrap.go --generate-ota-keys --ota-keys-output=../../.env

# 2. Start infrastructure
docker compose -f deployments/compose/docker-compose.yml up -d

# 3. Run services
go run ./services/core-service/cmd/api        # :8080
go run ./services/telemetry-service/cmd       # :8081

# 4. Frontend
cd frontend && npm install && npm run dev

# Swagger UI → http://localhost:8080/docs
```

---

## API Endpoints

| Method | Path | Description |
|:-------|:-----|:------------|
| `POST` | `/auth/register` `/auth/login` | JWT access + refresh tokens |
| `POST` | `/fleets/` | Create fleet — provisions N devices, returns zip of config sketches |
| `POST` | `/fleets/{id}/ota` | Deploy firmware to every device in fleet |
| `GET` | `/fleets/{id}/firmware` | Download `fleet_firmware.ino` for customisation |
| `POST` | `/devices/{id}/commands` | Dispatch command |
| `GET/PUT` | `/devices/{id}/shadow` | Read / set device shadow state |
| `POST` | `/devices/{id}/ota` | Single-device OTA deploy |
| `POST` | `/ota/firmware` | Upload firmware — triggers Ed25519 signing |
| `GET` | `/ai/query` | RAG query over device / fleet / incident knowledge base |
| `GET` | `/docs` | Swagger UI |

---

## Testing

```bash
# Unit tests
go test -v -race ./services/core-service/...
go test -v -race ./services/telemetry-service/...

# SDK — native C++, no hardware needed
cmake sdk_tests/ -B sdk_tests/build && cmake --build sdk_tests/build
./sdk_tests/build/argus_sdk_test

# Full E2E locally
bash sdk_tests/ci/gen_certs.sh
docker compose -f deployments/compose/docker-compose.yml \
               -f sdk_tests/ci/docker-compose.ci.yml up -d --build
cd sdk_tests/bootstrap && go run bootstrap.go \
    --base-url=https://localhost:8080 \
    --ca-cert=../../certs/root-ca.pem --output=device.env
cmake sdk_tests/ -B sdk_tests/build && cmake --build sdk_tests/build
set -a && source sdk_tests/bootstrap/device.env && set +a
./sdk_tests/build/argus_sdk_test
```

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Run the full test suite (`go test -race ./...` + E2E)
4. Submit a pull request against `master`

All three CI pipelines must pass before merge.

---

<div align="center">

**Built by [Vishal Shetagar](https://github.com/vishalss1)**

*Go · React · C++ · gRPC · MQTT · Kafka · Redis · PostgreSQL · ONNX · Ed25519 · ESP32*

[![GitHub](https://img.shields.io/badge/GitHub-vishalss1-181717?style=flat&logo=github)](https://github.com/vishalss1)

</div>
