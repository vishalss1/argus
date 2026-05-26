package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/command"
	"github.com/vishalss1/argus/internal/domain/policy"
)

type Engine struct {
	commandService *command.Service
	policyService  *policy.Service
}

func NewEngine(commandService *command.Service, policyService *policy.Service) *Engine {
	return &Engine{
		commandService: commandService,
		policyService:  policyService,
	}
}

func (e *Engine) ProcessSuggestion(ctx context.Context, deviceID string, incidentID *string, actionType policy.ActionType, metadata string) (*policy.ExecutionRecord, error) {
	// 1. Validate Action against Policy
	allowed, requiresApproval, err := e.policyService.ValidateAction(ctx, actionType, deviceID)
	if err != nil {
		return nil, fmt.Errorf("policy validation failed: %w", err)
	}

	if !allowed {
		return nil, fmt.Errorf("action %s is not allowed for device %s", actionType, deviceID)
	}

	// 2. Create execution record
	status := "pending"
	if !requiresApproval {
		status = "approved"
	}

	record := policy.ExecutionRecord{
		ID:          uuid.New().String(),
		Action:      actionType,
		DeviceID:    deviceID,
		IncidentID:  incidentID,
		Status:      status,
		SuggestedBy: "argus_ai",
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}

	created, err := e.policyService.CreateRecord(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	// 3. If no approval required, execute immediately
	if !requiresApproval {
		if err := e.Execute(ctx, created.ID); err != nil {
			return created, fmt.Errorf("immediate execution failed: %w", err)
		}
	}

	return created, nil
}

func (e *Engine) Execute(ctx context.Context, recordID string) error {
	record, err := e.policyService.GetRecord(ctx, recordID)
	if err != nil {
		return fmt.Errorf("failed to get execution record: %w", err)
	}

	if record.Status != "approved" {
		return fmt.Errorf("action is not approved for execution (status: %s)", record.Status)
	}

	// Map action type to actual device command
	var cmdName string
	var payload map[string]interface{}

	switch record.Action {
	case policy.ActionReboot:
		cmdName = "reboot"
	case policy.ActionRestartService:
		cmdName = "restart_service"
	case policy.ActionUpdateConfig:
		cmdName = "update_config"
	case policy.ActionRollbackFirmware:
		cmdName = "rollback_firmware"
	default:
		return fmt.Errorf("unsupported action type for execution: %s", record.Action)
	}

	var jsonPayload json.RawMessage
	if payload != nil {
		jsonPayload, _ = json.Marshal(payload)
	}

	_, err = e.commandService.Send(ctx, record.DeviceID, command.SendInput{
		Type:    cmdName,
		Payload: jsonPayload,
	})

	if err != nil {
		e.policyService.MarkFailed(ctx, recordID)
		return fmt.Errorf("send command failed: %w", err)
	}

	return e.policyService.MarkExecuted(ctx, recordID)
}
