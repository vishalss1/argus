package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Launcher struct {
	BaseURL string
	CACert  string
	BinPath string
	WorkDir string
}

func (l *Launcher) ProvisionAndStart(ctx context.Context, count int) ([]*exec.Cmd, error) {
	var cmds []*exec.Cmd

	for i := 0; i < count; i++ {
		envFile := filepath.Join(l.WorkDir, fmt.Sprintf("device_%d.env", i))
		
		// 1. Provision device
		// We execute from project root
		provCmd := exec.CommandContext(ctx, "go", "run", "sdk_tests/bootstrap/bootstrap.go",
			"--base-url="+l.BaseURL,
			"--ca-cert="+l.CACert,
			"--output="+envFile)
		
		if out, err := provCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to provision device %d: %v\nOutput: %s", i, err, string(out))
		}

		// 2. Read env file
		envVars, err := readEnvFile(envFile)
		if err != nil {
			return nil, err
		}

		// 3. Start Simulator
		simCmd := exec.CommandContext(ctx, l.BinPath)
		simCmd.Env = append(os.Environ(), envVars...)
		
		// Redirect output to files to avoid log spam, can inspect later
		logFile, err := os.Create(filepath.Join(l.WorkDir, fmt.Sprintf("sim_%d.log", i)))
		if err == nil {
			simCmd.Stdout = logFile
			simCmd.Stderr = logFile
		}

		if err := simCmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start simulator %d: %v", i, err)
		}
		cmds = append(cmds, simCmd)
	}

	return cmds, nil
}

func readEnvFile(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var envs []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// e.g. export FOO=BAR -> FOO=BAR
			line = strings.TrimPrefix(line, "export ")
			envs = append(envs, line)
		}
	}
	return envs, nil
}
