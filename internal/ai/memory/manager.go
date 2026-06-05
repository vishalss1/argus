package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/command"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/domain/ota"
)

type Manager struct {
	contextService *ctxdomain.Service
}

func NewManager(contextService *ctxdomain.Service) *Manager {
	return &Manager{
		contextService: contextService,
	}
}

func (m *Manager) SummarizeDeployment(ctx context.Context, dep ota.Deployment) error {
	data, err := json.Marshal(map[string]interface{}{
		"deployment_id": dep.ID,
		"artifact_id":   dep.ArtifactID,
		"status":        dep.Status,
		"message":       dep.ResultMessage,
	})
	if err != nil {
		return fmt.Errorf("marshal deployment data: %w", err)
	}

	_, err = m.contextService.RecordMemory(ctx, ctxdomain.OperationalMemory{
		DeviceID:  &dep.DeviceID,
		Type:      ctxdomain.MemoryTypeDeployment,
		Summary:   fmt.Sprintf("Deployment Finished: %s (Status: %s)", dep.ArtifactID, dep.Status),
		Data:      data,
		Timestamp: time.Now(),
	})
	return err
}

func (m *Manager) SummarizeCommand(ctx context.Context, cmd command.Command) error {
	data, err := json.Marshal(map[string]interface{}{
		"command_id": cmd.ID,
		"type":       cmd.Type,
		"status":     cmd.Status,
		"message":    cmd.ResultMessage,
	})
	if err != nil {
		return fmt.Errorf("marshal command data: %w", err)
	}

	_, err = m.contextService.RecordMemory(ctx, ctxdomain.OperationalMemory{
		DeviceID:  &cmd.DeviceID,
		Type:      ctxdomain.MemoryTypeCommandOutcome,
		Summary:   fmt.Sprintf("Command Completed: %s (Status: %s)", cmd.Type, cmd.Status),
		Data:      data,
		Timestamp: time.Now(),
	})
	return err
}


