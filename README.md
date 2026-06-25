# ARGUS

![ARGUS CI/CD](https://github.com/Vishalss1/argus/actions/workflows/e2e-ci.yml/badge.svg)
![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)
![ESP32](https://img.shields.io/badge/ESP32-C++-E7352C?style=flat)

ARGUS is a comprehensive, distributed fleet monitoring and device control system designed for IoT devices. It treats every device as a remotely observable, controllable, and versioned compute node.

## 🚀 Features

- **Device Registry**: Complete CRUD capabilities for managing your device fleet.
- **Real-Time Presence**: Heartbeat ingestion and tracking via MQTT Last Will and Testament (LWT) for near-instant online/offline status.
- **Telemetry & AI**: Time-series telemetry ingestion (HTTP/MQTT) with AI-powered analysis powered by ONNX Runtime.
- **Command & Control**: Secure command dispatching, retrieval, and ACK/NACK lifecycle tracking.
- **Shadow State (Digital Twins)**: Desired vs. reported state synchronization using Redis.
- **Secure OTA Updates**: Robust Over-The-Air firmware deployments via MinIO. Artifacts are secured with SHA-256 and Ed25519 cryptographic signatures.
- **Rules Engine & Alerts**: Telemetry rule evaluation and dynamic alert generation.
- **Modern SaaS Dashboard**: A responsive, rich web interface built with React, Vite, and Framer Motion.

## 🏗️ Architecture & Components

ARGUS has evolved into a robust microservice-oriented architecture:

- **`services/core-service`**: The primary Go REST API handling devices, commands, shadows, OTA manifests, and rules.
- **`services/telemetry-service`**: Dedicated Go service for high-throughput telemetry ingestion and processing.
- **`frontend/`**: The web application (Argus SaaS) using React 18, TypeScript, Vite, TanStack Query/Table, and Recharts.
- **`argus_sdk/`**: The official C++ SDK for ESP32 devices, handling secure TLS communication, MQTT telemetry, commands, and OTA signature verification.
- **`deployments/compose/`**: A complete local development and observability stack.

## 🛠️ Technology Stack

| Domain | Technologies |
| :--- | :--- |
| **Backend** | Go (1.22+), ONNX Runtime |
| **Frontend** | React 18, TypeScript, Vite, Framer Motion, Recharts |
| **Device SDK** | C++, Arduino Core for ESP32 |
| **Database & Cache**| PostgreSQL, Redis |
| **Message Brokers** | Mosquitto (MQTT), Redpanda / Kafka |
| **Object Storage** | MinIO (for OTA payloads) |
| **Observability** | Prometheus, Grafana, Loki, Promtail |
| **API Specs** | Swagger / OpenAPI |

## 🔄 CI/CD Pipelines

Our `.github/workflows` enforce strict quality and correctness:

1. **E2E CI (`e2e-ci.yml`)**: Fully automated End-to-End integration tests. It spins up the entire backend stack, provisions a test device, and runs a C++ simulator to validate telemetry, command parsing, and OTA firmware cryptographic verification.
2. **Go and React CI (`go-ci.yml`)**: Builds backend services (with ONNX runtime), runs all Go unit and race tests, and builds the Vite React frontend.
3. **SDK CI (`sdk-ci.yml`)**: Validates Arduino compilation against the ESP32 core and runs native SDK integration tests against the backend services.

## 🔒 OTA Security Model

ARGUS takes device security seriously. Firmware artifacts are strictly verified before installation:
1. Devices download firmware over pinned HTTPS/TLS connections.
2. The payload must pass a `SHA-256` checksum verification.
3. The checksum is then verified against an **Ed25519 signature** signed by the ARGUS server. 
4. The update is aborted if any cryptographic check fails, preventing MITM or compromised storage attacks.

## 💻 Local Development

### Prerequisites
- Go 1.22+
- Node.js 20+ (for frontend)
- Docker & Docker Compose
- CMake & Arduino CLI (if developing the SDK)

### Quick Start

1. **Generate OTA Keys**:
   ```bash
   cd sdk_tests/bootstrap
   go run bootstrap.go --generate-ota-keys --ota-keys-output=../../.env
   ```

2. **Start the Infrastructure**:
   ```bash
   docker compose -f deployments/compose/docker-compose.yml up -d
   ```
   *Brings up PostgreSQL, Redis, Mosquitto, Redpanda, MinIO, and the observability stack.*

3. **Run the Backend Services**:
   ```bash
   go run ./services/core-service/cmd/api
   go run ./services/telemetry-service/cmd
   ```

4. **Run the Frontend**:
   ```bash
   cd frontend
   npm install
   npm run dev
   ```

### Documentation

- The API Swagger UI is exposed at `GET /docs` on the core service. To regenerate the Swagger docs, run `make swagger` in the root directory.
- Refer to `CLAUDE.md` for deep-dive architectural rules and conventions.
