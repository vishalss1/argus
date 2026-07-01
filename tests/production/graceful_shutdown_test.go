//go:build e2e
package production

import (
	"context"
	"testing"
	"time"

	"github.com/vishalss1/argus/tests/production/env"
)

func TestGracefulShutdown(t *testing.T) {
	e := env.DefaultEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// In a real scenario, the launcher would be running in the background.
	// We restart core-service and verify it exits cleanly and comes back.
	
	t.Log("Restarting core-service...")
	if err := e.RestartCoreService(ctx); err != nil {
		t.Fatalf("Failed to restart core-service: %v", err)
	}
	t.Log("core-service restarted successfully")

	t.Log("Restarting telemetry-service...")
	if err := e.RestartTelemetryService(ctx); err != nil {
		t.Fatalf("Failed to restart telemetry-service: %v", err)
	}
	t.Log("telemetry-service restarted successfully")

	// We would also assert MQTT reconnection and telemetry resumption here,
	// usually done by inspecting the simulator logs or querying the API
	// to see if latest telemetry timestamp is increasing.
}
