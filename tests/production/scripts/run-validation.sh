#!/bin/bash
set -e

echo "Starting Production Validation Tests..."

# Build the tests to ensure no compilation errors
# Optional: Set up environment
if [ "$KUBERNETES_TEST" = "true" ]; then
    echo "Running in Kubernetes environment (kind)"
else
    echo "Running in Docker Compose environment"
fi

# We collect metrics such as cpu/memory without failing
echo "Monitoring resource usage in background..."
docker stats --no-stream > resource_metrics.txt || true

# Run Go race detector on the main codebase
echo "Running race detector on main codebase..."
go test -race ./...

# Check for OOMKilled containers if using Docker
if [ "$KUBERNETES_TEST" != "true" ]; then
    echo "Checking for OOMKilled containers..."
    OOM_COUNT=$(docker ps -a --format '{{.Status}}' | grep -c 'OOMKilled' || true)
    if [ "$OOM_COUNT" -gt 0 ]; then
        echo "Error: Found OOMKilled containers!"
        exit 1
    fi
fi

echo "Production validation completed successfully!"
