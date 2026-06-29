#!/bin/bash

# Default arguments
DEVICES=100
FREQ="1s"
DURATION="1m"
PAYLOAD=256

# Parse arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        -Devices) DEVICES="$2"; shift ;;
        -Freq) FREQ="$2"; shift ;;
        -Duration) DURATION="$2"; shift ;;
        -Payload) PAYLOAD="$2"; shift ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

echo "Starting infrastructure services..."
docker compose --env-file ./.env -f deployments/compose/docker-compose.yml up -d postgres redis mosquitto redpanda minio telemetry-service

echo "Waiting for postgres..."
PG_READY=false
for i in {1..30}; do
    PG_STATUS=$(docker exec argus-postgres pg_isready -U argus 2>&1)
    if [[ "$PG_STATUS" == *"accepting connections"* ]]; then
        echo "Postgres is ready."
        PG_READY=true
        break
    fi
    sleep 2
done

if [ "$PG_READY" = false ]; then
    echo "ERROR: Postgres failed to start."
    exit 1
fi

echo "Waiting for telemetry-service..."
TELEMETRY_READY=false
for i in {1..15}; do
    if curl -s -f http://localhost:8081/healthz > /dev/null; then
        echo "Telemetry service is ready."
        TELEMETRY_READY=true
        break
    fi
    sleep 2
done

if [ "$TELEMETRY_READY" = false ]; then
    echo "ERROR: Telemetry service failed to start."
    exit 1
fi

echo "Building API server and benchmark client..."
cd /d/argus 2>/dev/null || cd d:/argus 2>/dev/null || cd /mnt/d/argus 2>/dev/null || cd D:/argus
go build -o api_server.exe ./services/core-service/cmd/api/main.go > /dev/null 2>&1
go build -o benchmark-e2e.exe ./cmd/benchmark-e2e/main.go > /dev/null 2>&1

echo "Cleaning up previous API server..."
killall api_server.exe 2>/dev/null || pkill -f api_server.exe 2>/dev/null || true
echo "Stopping argus-api docker container to free port 8080..."
docker stop argus-api > /dev/null 2>&1
sleep 15

echo "Resetting consumer group offsets..."
for i in {1..3}; do
    RESULT=$(docker exec argus-redpanda rpk group delete argus-telemetry-live-consumer-internal 2>&1)
    if [[ "$RESULT" == *"OK"* ]]; then
        break
    fi
    sleep 3
done

echo "Starting API server..."
export TELEMETRY_SERVICE_GRPC_ADDR=localhost:50052
./api_server.exe &
API_PID=$!
echo "API server PID: $API_PID"

echo "Waiting for server to be ready..."
sleep 12

if ! kill -0 $API_PID 2>/dev/null; then
    echo "ERROR: API server failed to start"
    exit 1
fi

echo "API server running. Starting benchmark..."
./benchmark-e2e.exe -devices "$DEVICES" -freq "$FREQ" -duration "$DURATION" -payload "$PAYLOAD"

echo "Stopping API server..."
kill $API_PID 2>/dev/null

echo "Restarting argus-api docker container..."
docker start argus-api > /dev/null 2>&1
echo "Done"
