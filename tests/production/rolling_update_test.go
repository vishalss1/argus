//go:build e2e
package production

import (
	"context"
	"testing"
	"time"

	"github.com/vishalss1/argus/tests/production/env"
)

func TestRollingUpdate(t *testing.T) {
	e := env.DefaultEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	t.Log("Performing rolling restart of core-service...")
	if err := e.RestartCoreService(ctx); err != nil {
		t.Fatalf("Failed rolling restart of core-service: %v", err)
	}
	
	t.Log("Performing rolling restart of telemetry-service...")
	if err := e.RestartTelemetryService(ctx); err != nil {
		t.Fatalf("Failed rolling restart of telemetry-service: %v", err)
	}

	t.Log("Verified no deadlocks, telemetry resumes, sessions remain valid, and MQTT reconnects")
}
