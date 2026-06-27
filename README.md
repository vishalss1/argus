<div align="center">

# ⚡ ARGUS

### Distributed IoT Fleet Monitoring & Control Platform

![E2E CI](https://github.com/Vishalss1/argus/actions/workflows/e2e-ci.yml/badge.svg)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![React](https://img.shields.io/badge/React_18-61DAFB?style=flat&logo=react)
![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?style=flat&logo=typescript)
![ESP32](https://img.shields.io/badge/ESP32-C++-E7352C?style=flat)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat&logo=postgresql)
![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat&logo=redis)
![Kafka](https://img.shields.io/badge/Redpanda-Kafka-00ADD8?style=flat)
![MQTT](https://img.shields.io/badge/MQTT-Mosquitto-660066?style=flat)
![MinIO](https://img.shields.io/badge/MinIO-Object_Storage-C62828?style=flat)
![ONNX](https://img.shields.io/badge/ONNX_Runtime-AI_Inference-005CED?style=flat)
![gRPC](https://img.shields.io/badge/gRPC-Inter_Service-000000?style=flat)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)
![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?style=flat&logo=kubernetes)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

<br/>

**A production-grade distributed platform spanning Go microservices, a C++ ESP32 SDK, and a React dashboard — with full CI including a native C++ device simulator that exercises the actual SDK code against a live backend without hardware.**

[Architecture](#%EF%B8%8F-architecture) • [What Makes This Hard](#-what-makes-this-hard) • [Features](#-features) • [CI/CD](#-cicd-pipelines) • [Security](#-ota-security-model) • [Quick Start](#-quick-start) • [API Docs](#-api-documentation)

</div>

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                        Frontend (React 18 + TypeScript)                      │
│                    nginx → /api proxy → core-service:8080                    │
└─────────────────────────┬────────────────────────────────────────────────────┘
                          │ HTTP/REST + WebSocket
                          ▼
┌────────────────────────────────────┐   gRPC    ┌─────────────────────────────┐
│        Core Service (Go :8080)     │◄─────────►│   Telemetry Service (Go)    │
│        gRPC Server (:50051)        │           │   HTTP (:8081) gRPC (:50052)│
│                                    │           │                             │
│  • Device Registry & Fleet Mgmt   │           │  • Kafka Telemetry Consumer  │
│  • Command Dispatch (Kafka→MQTT)  │           │  • EWMA Anomaly Detection    │
│  • OTA Lifecycle & Ed25519 Signing│           │  • Statistical Z-Scores      │
│  • Shadow State (Redis)           │           │  • AI Query Engine (Groq)    │
│  • JWT Auth & Workspace Isolation │           │  • Local ONNX Embeddings     │
│  • Session Management & Lifecycle │           │  • pgvector Semantic Search  │
│  • Firmware Template Generation   │           │  • Rules Engine & Alerts     │
│  • Certificate Authority (ECDSA)  │           │  • Operational Memory        │
│  • API Key Management             │           │  • Action Engine (gated)     │
└──┬──┬──┬──┬──────────────────────┘           └──┬──┬──┬──────────────────────┘
   │  │  │  │                                      │  │  │
   ▼  ▼  ▼  ▼                                      ▼  ▼  ▼
 ┌────┐┌─────┐┌──────┐┌─────┐               ┌─────┐┌─────┐┌──────┐┌─────┐
 │ PG ││Redis││MQTT  ││Kafka│               │ PG  ││Redis││Kafka ││MinIO│
 │+vec││     ││Mosq. ││/RP  │               │+vec ││     ││/RP   ││     │
 └────┘└─────┘└──────┘└─────┘               └─────┘└─────┘└──────┘└─────┘

                                                     ┌─────────────────────┐
                                                     │    ESP32 Devices    │
                                                     │   (argus_sdk/ C++)  │
                                                     │                     │
                                                     │  NVS identity layer │
                                                     │  mTLS + API key auth│
                                                     │  Ed25519 OTA verify │
                                                     │  State machine loop │
                                                     └─────────────────────┘
```

### Inter-Service Communication

| Channel | Direction | Purpose |
|:--------|:----------|:--------|
| **gRPC** | Core ↔ Telemetry | AI queries, snapshots, rules, incidents, policy checks |
| **Kafka** | Core → Telemetry | Raw telemetry events (`telemetry.raw`), command dispatch |
| **Kafka** | Telemetry → Core | Incident events (`telemetry.incidents`) |
| **MQTT** | Device ↔ Core | Telemetry publish, command subscribe, LWT presence |
| **gRPC Health** | Both services | `grpc_health_v1` with correlation ID propagation |

---

## 🔩 What Makes This Hard

Most IoT demos connect a device to a broker. ARGUS is a fleet management system — the non-trivial parts are in the architecture, not the connectivity.

**Two-firmware NVS architecture** — Fleet provisioning generates a per-device `config_{deviceID}.ino` provisioning sketch that writes device identity (UUID, API key, mTLS certs, OTA public key) to ESP32 NVS flash. Subsequent OTA updates deploy an identical fleet binary across all N devices — no per-device values compiled in. One binary, infinite devices. Identity survives OTA.

**Native host build harness for ESP32 C++** — The SDK is C++ targeting the ESP32. Rather than flash hardware on every change, a POSIX/OpenSSL HAL stub layer implements `WiFiClientSecure`, `PubSubClient`, `HTTPClient`, `Preferences`, and `Update.h` using real TCP sockets. The SDK C++ compiles and runs on Linux x86\_64, making real HTTPS + MQTT connections to a Dockerised backend. libsodium and mbedTLS run natively — Ed25519 verification is real, not mocked.

**Three-tier CI split** — `go-ci.yml` covers backend/frontend build and unit tests. `sdk-ci.yml` runs arduino-cli compile check + the native SDK simulator against a live backend (120s, all timers fire). `e2e-ci.yml` on master validates the full system: provisions a test device, dispatches commands, asserts MQTT acks, uploads a firmware artifact, deploys OTA, and verifies the simulator receives and cryptographically validates the binary. All three gates must pass before master merges.

**4-layer OTA cryptographic verification** — TLS cert pinning → SHA-256 checksum → Ed25519 signature on the checksum hex string (libsodium) → partition write abort on any failure. Signing key ID is validated against the key baked into firmware at provisioning. A corrupt or tampered binary is rejected before a single byte touches flash.

**Per-device mTLS + API key dual auth** — Each device is issued a unique ECDSA client certificate signed by an internal CA. The backend validates the client cert chain at the TLS layer. Additionally, a per-device API key (SHA-256 hashed, prefix-indexed) is validated on every HTTP request via `DeviceAuth` middleware. Both layers operate independently — either is sufficient to reject an unauthorized device.

**Stateful command lifecycle across two brokers** — Commands move through `PENDING → DISPATCHED → ACKED/NACKED/TIMEOUT`. They're written to Postgres, published to Kafka, consumed by the MQTT bridge, delivered over Mosquitto, and the device's response flows back through MQTT → Kafka → Postgres state update. A background goroutine marks unacknowledged commands as `TIMED_OUT`. The full audit trail is preserved.

**Local AI inference pipeline** — Telemetry service runs `all-MiniLM-L6-v2` via ONNX Runtime for 384-dim embeddings — no external API call. Vectors stored in pgvector. RAG queries use a planner/router pattern across device, fleet, historical, and incident handlers before hitting Groq for reasoning. Anomaly detection runs EWMA + Welford's online algorithm on live telemetry streams.

---

## ✨ Features

### 🏭 Fleet Management & Device Lifecycle

| Feature | Description |
|:--------|:------------|
| **Fleet Provisioning** | Create a fleet → instantly provision N devices with per-device `config_{deviceID}.ino` sketches |
| **Two-Firmware Model** | Provisioning sketch writes identity to NVS → fleet firmware reads from NVS. One binary, N devices. |
| **Fleet OTA** | `POST /fleets/{id}/ota` deploys a firmware artifact to all N devices simultaneously. Log-and-continue on per-device failure. |
| **Device Registry** | Full CRUD with workspace isolation, pagination, and API key management |
| **Real-Time Presence** | MQTT Last Will and Testament (LWT) for instant online/offline detection + heartbeat stale monitoring |
| **Workspace Isolation** | Multi-tenant architecture — devices, sessions, fleets, and rules scoped per workspace |

### 🔌 Command & Control

| Feature | Description |
|:--------|:------------|
| **Command Lifecycle** | `PENDING → DISPATCHED → ACKED/NACKED/TIMEOUT` with full audit trail in Postgres |
| **Kafka Bridge** | Commands flow through Kafka for durable, traceable dispatch with correlation IDs |
| **MQTT Transport** | Real-time command delivery over Mosquitto with auto-reconnect and exponential backoff |
| **WebSocket Broadcast** | Command status updates pushed to the frontend in real-time |
| **Policy Gating** | AI-suggested actions validated against policies with approval workflows and daily rate limits |

### 📡 Telemetry & Analytics

| Feature | Description |
|:--------|:------------|
| **Dual-Path Ingestion** | MQTT (primary) and HTTP (fallback) telemetry paths, both funneling through Kafka |
| **Real-Time Streaming** | Kafka consumer processes telemetry batches with sub-second latency |
| **Redis Caching** | Latest telemetry per device cached with 5-minute TTL for fast dashboard reads |
| **Session Scoping** | All telemetry tied to sessions — start, run, stop, export as artifacts |
| **Telemetry Artifacts** | On session end: device summaries, metric aggregates, hourly summaries, incidents archive |

### 🤖 AI & Intelligence Engine

| Feature | Description |
|:--------|:------------|
| **EWMA Anomaly Detection** | Exponentially Weighted Moving Average on live telemetry streams — flags deviations in real-time |
| **Statistical Z-Scores** | Welford's online algorithm for running mean/variance on thermal and connectivity metrics |
| **Local Embeddings** | `all-MiniLM-L6-v2` via ONNX Runtime (384-dim vectors) — no external API calls for embedding inference |
| **Vector Search** | pgvector in PostgreSQL for semantic similarity search and event deduplication |
| **RAG Query Engine** | Planner → Router → Device/Fleet/Historical/Incident handlers → Groq LLM reasoning |
| **Operational Memory** | AI records deployments, commands, incidents — builds a retrievable knowledge base over time |
| **Root Cause Analysis** | AI-generated RCA with suggested remediation actions |
| **Action Engine** | Policy-gated AI actions with audit trail, approval workflow, and rate limiting |

### 🔐 Security & Authentication

| Feature | Description |
|:--------|:------------|
| **Internal CA** | ECDSA root CA signs and verifies per-device mTLS client certificates |
| **Dual Auth** | Devices authenticate via mTLS client cert OR per-device API key (prefix + SHA-256 hash) |
| **JWT Auth** | User authentication with access/refresh token rotation and revocation |
| **Ed25519 OTA Signatures** | Firmware artifacts signed server-side — devices verify before any byte touches flash |
| **SHA-256 Checksums** | Every firmware artifact integrity-verified end-to-end |
| **Certificate Pinning** | ESP32 SDK pins server cert SHA-256 fingerprint — prevents MITM even under rogue CA |

### 🔄 OTA Firmware Updates

| Feature | Description |
|:--------|:------------|
| **Single Device Deploy** | Push firmware to one device with full status tracking |
| **Fleet-Wide Deploy** | Deploy to every device in a fleet — log-and-continue on per-device failure |
| **MinIO Storage** | Firmware artifacts stored in MinIO with presigned time-limited download URLs |
| **State Machine** | `pending → downloading → installing → completed/failed/timed_out` |
| **Timeout Monitor** | Background goroutine marks stale OTA deployments as `timed_out` every 60 seconds |
| **Firmware Generation** | Go templates generate Arduino `.ino` files with injected credentials and config |
| **OTA Rollback** | ESP32 SDK rolls back to previous partition on boot failure |

### 🏗️ Digital Twins (Shadow State)

| Feature | Description |
|:--------|:------------|
| **Desired vs. Reported** | Redis-backed state synchronization — operator sets desired, device reports actual |
| **Drift Detection** | Automatic detection when desired state differs from reported state |
| **Device-Side Sync** | ESP32 SDK reads desired state, applies changes, pushes reported state back |
| **Persistent Storage** | Shadow state stored in Redis with no TTL — survives restarts |

### 🖥️ Frontend Dashboard

| Feature | Description |
|:--------|:------------|
| **16 Pages** | Landing, Login, Register, Workspaces, Fleet Overview, Devices, Device Detail, Sessions, Session Reports, Telemetry, Commands, OTA, Alerts, AI Insights, Observability, Documentation |
| **Real-Time Updates** | WebSocket connection for live telemetry, command status, and presence changes |
| **Rich Charts** | Recharts-powered telemetry visualization with time-series data |
| **Responsive Design** | Framer Motion animations, modern SaaS UI patterns |
| **Workspace Switcher** | Multi-workspace support with context switching |

### 📊 Observability

| Feature | Description |
|:--------|:------------|
| **Prometheus Metrics** | Core service exposes `/metrics` endpoint |
| **Grafana Dashboards** | Pre-configured with Prometheus + Loki datasources |
| **Loki + Promtail** | Centralized log aggregation from all containers |
| **Alerting Rules** | Prometheus alerting rules for infrastructure monitoring |

---

## 🔄 CI/CD Pipelines

Three automated pipelines enforce quality at every layer of the stack.

### E2E CI (`e2e-ci.yml`) — triggers on `master`

The most comprehensive test: spins up the entire stack and validates the system end-to-end.

```
1. Generate TLS certificates + Ed25519 OTA signing keypair
2. Start 10 infrastructure containers via Docker Compose
3. Provision a test device via the Go bootstrap script (real API calls)
4. Build the C++ SDK test runner natively (CMake + libsodium + OpenSSL)
5. Run simulator for 120s — all SDK timers fire (telemetry 5s, heartbeat 30s, OTA poll 60s)

Validations:
  ✅ Telemetry   — device publishes MQTT, CI asserts receipt via mosquitto_sub
  ✅ Commands    — dispatches ping command, asserts "Pong from hardware" on /results
  ✅ OTA         — uploads firmware binary, deploys deployment record,
                   simulator receives manifest, verifies SHA-256 + Ed25519 signature,
                   asserts flash write and ACK publish

6. Cleanup test device and artifact
7. docker compose down
```

### Go & React CI (`go-ci.yml`) — triggers on `master`

- **Backend**: Builds both Go services (including ONNX Runtime integration), runs all unit tests with `-race`
- **Frontend**: `npm ci` + `npm run build` — validates full TypeScript compilation

### SDK CI (`sdk-ci.yml`) — triggers on `feat/sdk-dev`

- **Arduino Compile Check**: Installs ESP32 core, compiles the full SDK library with arduino-cli
- **Integration Test**: Builds the C++ test runner natively, runs 120s against a live backend stack with real HTTPS + MQTT + OTA poll

---

## 🔒 OTA Security Model

Every OTA update goes through a **4-layer cryptographic verification pipeline** before a single byte touches flash:

```
┌─────────────────────────────────────────────────────────────────┐
│                       OTA Security Pipeline                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Layer 1 — TLS + Certificate Pinning                           │
│            Device pins server cert SHA-256 fingerprint          │
│            All communication over pinned TLS connection         │
│                                                                 │
│  Layer 2 — SHA-256 Checksum                                    │
│            Backend computes hash on firmware upload             │
│            Device verifies chunk-by-chunk during download       │
│                                                                 │
│  Layer 3 — Ed25519 Digital Signature                           │
│            Backend signs the SHA-256 hex string with Ed25519   │
│            Device verifies with provisioned public key          │
│            Key ID validated against firmware-embedded key ID    │
│                                                                 │
│  Layer 4 — Abort on Any Failure                                │
│            Corrupt binary, wrong key, tampered manifest         │
│            → update rejected, nack published, firmware intact   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key provisioning**: Ed25519 keypairs generated via `cmd/ota-keygen/`. Public key embedded in device NVS during provisioning. Private key held server-side in environment config only.

---

## 🛠️ Technology Stack

| Domain | Technologies |
|:-------|:-------------|
| **Backend** | Go 1.22+, gRPC + Protobuf, chi router, Swagger/OpenAPI |
| **Frontend** | React 18, TypeScript, Vite, TanStack Query & Table, Recharts, Framer Motion |
| **Device SDK** | C++17, Arduino Core for ESP32, libsodium, mbedTLS, PubSubClient, ArduinoJson |
| **Databases** | PostgreSQL 16 + pgvector, Redis 7 |
| **Message Brokers** | Eclipse Mosquitto (MQTT 3.1.1), Redpanda (Kafka-compatible) |
| **Object Storage** | MinIO (firmware artifact storage + presigned URLs) |
| **AI / ML** | ONNX Runtime (local `all-MiniLM-L6-v2` embeddings), Groq API (LLM reasoning) |
| **Security** | Ed25519 signing, SHA-256 checksums, ECDSA mTLS, JWT RS256, cert pinning |
| **Observability** | Prometheus, Grafana, Loki, Promtail |
| **Deployment** | Docker Compose (10 containers), Kubernetes manifests, nginx reverse proxy |
| **CI/CD** | GitHub Actions — 3 pipelines (E2E, Go+React, SDK) |

---

## 📁 Project Structure

```
argus/
├── services/
│   ├── core-service/          # Primary Go API
│   │   ├── cmd/api/           # Entrypoint (HTTP :8080 + gRPC :50051)
│   │   └── internal/
│   │       ├── domain/        # Business logic (device, fleet, ota, session, certificate, policy)
│   │       ├── transport/     # HTTP handlers, gRPC server, middleware (auth, mTLS, workspace)
│   │       ├── infrastructure/# Postgres, Redis, Kafka, MQTT, MinIO clients
│   │       ├── firmware/      # Arduino .ino template generation (provision + fleet firmware)
│   │       └── config/        # Environment configuration
│   └── telemetry-service/     # Dedicated AI + analytics service
│       ├── cmd/               # Entrypoint (HTTP :8081 + gRPC :50052)
│       └── internal/
│           ├── ai/            # Anomaly detection, embeddings, RAG, actions, operations
│           ├── domain/        # Telemetry, rules, events, anomalies
│           ├── infrastructure/# Kafka, Redis, Postgres+pgvector, ONNX Runtime, Groq
│           └── transport/     # gRPC server
├── shared/                    # Shared Go modules + Protobuf definitions
│   └── proto/                 # gRPC service definitions (core ↔ telemetry)
├── frontend/                  # React 18 + TypeScript SPA (16 pages)
├── argus_sdk/                 # C++ ESP32 SDK
│   └── src/                   # argus.cpp, argus_nvs.cpp, argus_http.cpp, argus_mqtt.cpp,
│                              # argus_ota.cpp, argus_security.cpp, argus_state_machine.cpp,
│                              # argus_rollback.cpp, argus_time.cpp, argus_diag.cpp
├── sdk_tests/                 # Native C++ test harness
│   ├── hal/                   # POSIX/OpenSSL HAL stubs (WiFiClientSecure, PubSubClient, etc.)
│   ├── bootstrap/             # Go device provisioning tool
│   ├── ci/                    # CI compose override + cert generation
│   ├── config_shim.cpp        # Extern symbol definitions from env vars (native build)
│   └── main.cpp               # Test runner: argusBegin → argusLoop → assert
├── deployments/
│   ├── compose/               # Docker Compose (10 containers + observability stack)
│   └── k8s/                   # Kubernetes manifests (namespace, deployments, ingress)
├── cmd/
│   ├── ota-keygen/            # Ed25519 key pair generator
│   └── gen-certs/             # TLS certificate generator
├── migrations/                # PostgreSQL migration files (golang-migrate)
└── .github/workflows/         # CI/CD (E2E, Go+React, SDK)
```

---

## 🚀 Quick Start

### Prerequisites

- Go 1.22+, Node.js 20+, Docker & Docker Compose
- Arduino CLI + CMake (for SDK development)

### 1. Generate OTA Signing Keys

```bash
cd sdk_tests/bootstrap
go run bootstrap.go --generate-ota-keys --ota-keys-output=../../.env
```

### 2. Start Infrastructure (10 containers)

```bash
docker compose -f deployments/compose/docker-compose.yml up -d
```

Brings up: PostgreSQL + pgvector, Redis, Mosquitto MQTT, Redpanda (Kafka), MinIO, Prometheus, Grafana, Loki, Promtail.

### 3. Run Backend Services

```bash
# Core service  (HTTP :8080 + gRPC :50051)
go run ./services/core-service/cmd/api

# Telemetry service  (HTTP :8081 + gRPC :50052)
go run ./services/telemetry-service/cmd
```

### 4. Run Frontend

```bash
cd frontend && npm install && npm run dev
```

### 5. Regenerate Swagger Docs

```bash
make swagger
```

---

## 📚 API Documentation

Swagger UI is available at `GET /docs` on the core service when running locally.

### Key Endpoints

| Method | Path | Description |
|:-------|:-----|:------------|
| `POST` | `/auth/register` | Register new user |
| `POST` | `/auth/login` | Login, returns JWT access + refresh tokens |
| `GET` | `/fleets/` | List fleets with device counts and stats |
| `POST` | `/fleets/` | Create fleet — provisions N devices, returns zip of `config_{id}.ino` sketches |
| `POST` | `/fleets/{id}/ota` | Deploy firmware artifact to all devices in fleet |
| `GET` | `/fleets/{id}/firmware` | Download `fleet_firmware.ino` source for user customization |
| `POST` | `/fleets/{id}/devices` | Add devices to an existing fleet |
| `GET` | `/devices/` | List devices with workspace-scoped filters |
| `POST` | `/devices/{id}/commands` | Dispatch command to device |
| `GET` | `/devices/{id}/shadow` | Get current device shadow state |
| `PUT` | `/devices/{id}/shadow/desired` | Set desired state for shadow sync |
| `POST` | `/devices/{id}/ota` | Deploy OTA firmware to a single device |
| `GET` | `/devices/{id}/ota/pending` | Device polls for pending OTA manifest |
| `POST` | `/ota/firmware` | Upload firmware artifact (triggers Ed25519 signing) |
| `GET` | `/ai/query` | RAG-based AI query over device/fleet/incident knowledge base |
| `POST` | `/ai/actions/{id}/approve` | Approve a policy-gated AI-suggested action |
| `GET` | `/sessions/` | List sessions |
| `GET` | `/docs` | Swagger UI |

---

## 🧪 Testing

### Unit Tests (Go)

```bash
cd services/core-service && go test -v -race ./...
cd services/telemetry-service && go test -v -race ./...
cd shared && go test -v -race ./...
```

### SDK Tests (C++ Native — no hardware required)

```bash
cmake sdk_tests/ -B sdk_tests/build
cmake --build sdk_tests/build
./sdk_tests/build/argus_sdk_test
```

### E2E Tests (Full Stack)

The `e2e-ci.yml` pipeline runs a complete integration test against the real backend stack with a C++ device simulator — testing telemetry ingestion, command dispatch, and OTA cryptographic verification end-to-end. Run locally with:

```bash
bash sdk_tests/ci/gen_certs.sh
docker compose -f deployments/compose/docker-compose.yml \
               -f sdk_tests/ci/docker-compose.ci.yml up -d --build
cd sdk_tests/bootstrap && go run bootstrap.go --base-url=https://localhost:8080 \
    --ca-cert=../../certs/root-ca.pem --output=device.env
cmake sdk_tests/ -B sdk_tests/build && cmake --build sdk_tests/build
set -a && source sdk_tests/bootstrap/device.env && set +a
./sdk_tests/build/argus_sdk_test
```

---

## 📦 Kubernetes Deployment

Kubernetes manifests are available in `deployments/k8s/`:

```bash
kubectl apply -f deployments/k8s/namespace.yaml
kubectl apply -f deployments/k8s/
```

Includes: namespace, core-service, telemetry-service, frontend, PostgreSQL, Redis, Redpanda, and ingress configuration via Kustomize.

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`feat/your-feature`)
3. Commit with conventional commits
4. Push and open a Pull Request — CI runs E2E, Go+React, and SDK tests automatically

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">

**Built by [Vishal Shetagar](https://github.com/vishalss1)**

*Go • React • C++ • gRPC • MQTT • Kafka • Redis • PostgreSQL • ONNX • Ed25519 • ESP32*

</div>