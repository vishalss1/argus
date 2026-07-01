package env

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Environment abstracts infrastructure operations for the production tests.
type Environment interface {
	RestartCoreService(ctx context.Context) error
	RestartTelemetryService(ctx context.Context) error
	RestartComponent(ctx context.Context, component string) error
	GetCoreServiceURL() string
	GetTelemetryServiceURL() string
	GetRedisURL() string
}

// DockerComposeEnv implements Environment for docker-compose.
type DockerComposeEnv struct {
	ComposeProject string
}

func (d *DockerComposeEnv) RestartCoreService(ctx context.Context) error {
	return d.RestartComponent(ctx, "core-service")
}

func (d *DockerComposeEnv) RestartTelemetryService(ctx context.Context) error {
	return d.RestartComponent(ctx, "telemetry-service")
}

func (d *DockerComposeEnv) RestartComponent(ctx context.Context, component string) error {
	composeFiles := []string{"-f", "../../deployments/compose/docker-compose.yml"}
	if extraFile := os.Getenv("COMPOSE_FILE_EXTRA"); extraFile != "" {
		composeFiles = append(composeFiles, "-f", extraFile)
	}

	args := []string{"compose", "-p", d.ComposeProject}
	args = append(args, composeFiles...)
	args = append(args, "restart", component)
	
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart %s: %v, output: %s", component, err, string(output))
	}
	return nil
}

func (d *DockerComposeEnv) GetCoreServiceURL() string {
	return "http://localhost:8080"
}

func (d *DockerComposeEnv) GetTelemetryServiceURL() string {
	return "http://localhost:8081"
}

func (d *DockerComposeEnv) GetRedisURL() string {
	return "localhost:6379"
}

// KindEnv implements Environment for Kubernetes (kind).
type KindEnv struct {
	Namespace string
}

func (k *KindEnv) RestartCoreService(ctx context.Context) error {
	return k.RestartComponent(ctx, "deployment/core-service")
}

func (k *KindEnv) RestartTelemetryService(ctx context.Context) error {
	return k.RestartComponent(ctx, "deployment/telemetry-service")
}

func (k *KindEnv) RestartComponent(ctx context.Context, component string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "restart", component, "-n", k.Namespace)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart %s: %v, output: %s", component, err, string(output))
	}
	// Wait for rollout to finish
	waitCmd := exec.CommandContext(ctx, "kubectl", "rollout", "status", component, "-n", k.Namespace)
	waitOutput, err := waitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to wait for rollout of %s: %v, output: %s", component, err, string(waitOutput))
	}
	return nil
}

func (k *KindEnv) GetCoreServiceURL() string {
	// Assumes port-forward or ingress is already set up in tests or CI
	return "http://localhost:8080"
}

func (k *KindEnv) GetTelemetryServiceURL() string {
	return "http://localhost:8081"
}

func (k *KindEnv) GetRedisURL() string {
	return "localhost:6379"
}

// DefaultEnv returns an appropriate Environment implementation based on ENV vars.
func DefaultEnv() Environment {
	if strings.ToLower(os.Getenv("KUBERNETES_TEST")) == "true" {
		ns := os.Getenv("K8S_NAMESPACE")
		if ns == "" {
			ns = "argus"
		}
		return &KindEnv{Namespace: ns}
	}
	
	proj := os.Getenv("COMPOSE_PROJECT_NAME")
	if proj == "" {
		proj = "argus"
	}
	return &DockerComposeEnv{ComposeProject: proj}
}
