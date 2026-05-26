package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/internal/domain/policy"
	"github.com/vishalss1/argus/internal/domain/command"
)

type Engine struct {
	commandService *command.Service
	policies       []policy.Policy
}

func NewEngine(commandService *command.Service) *Engine {
	return &Engine{
		commandService: commandService,
		// Hardcoded policies for now
		policies: []policy.Policy{
			{
				Action:           policy.ActionReboot,
				RequiresApproval: true,
			},
			{
				Action:           policy.ActionRestartService,
				RequiresApproval: false,
			},
		},
	}
}

func (e *Engine) ProcessSuggestion(ctx context.Context, deviceID string, actionType policy.ActionType, metadata string) (*policy.ExecutionRecord, error) {
	// 1. Find policy
	var targetPolicy *policy.Policy
	for _, p := range e.policies {
		if p.Action == actionType {
			targetPolicy = &p
			break
		}
	}

	if targetPolicy == nil {
		return nil, fmt.Errorf("no policy defined for action %s", actionType)
	}

	// 2. Create execution record
	record := &policy.ExecutionRecord{
		ID:          uuid.New().String(),
		Action:      actionType,
		DeviceID:    deviceID,
		Status:      "pending",
		SuggestedBy: "argus_ai",
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}

	// 3. Validate Policy
	if targetPolicy.RequiresApproval {
		// Just return the record as pending
		return record, nil
	}

	// 4. Execute Action
	if err := e.Execute(ctx, record); err != nil {
		record.Status = "failed"
		return record, fmt.Errorf("execution failed: %w", err)
	}

	record.Status = "executed"
	return record, nil
}

func (e *Engine) Execute(ctx context.Context, record *policy.ExecutionRecord) error {
	// Map action type to actual device command
	var cmdName string
	var payload map[string]interface{}

	switch record.Action {
	case policy.ActionReboot:
		cmdName = "reboot"
	case policy.ActionRestartService:
		cmdName = "restart_service"
	default:
		return fmt.Errorf("unsupported action type for execution: %s", record.Action)
	}

	var jsonPayload json.RawMessage
	if payload != nil {
		jsonPayload, _ = json.Marshal(payload)
	}

	_, err := e.commandService.Send(ctx, record.DeviceID, command.SendInput{
		Type:    cmdName,
		Payload: jsonPayload,
	})
	return err
}
