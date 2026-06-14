package actions

import (
	"context"
	"fmt"

	pb "github.com/vishalss1/argus/shared/proto/core"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/grpc"
)

type Engine struct {
	coreClient *grpc.CoreClient
}

func NewEngine(coreClient *grpc.CoreClient) *Engine {
	return &Engine{
		coreClient: coreClient,
	}
}

func (e *Engine) ProcessSuggestion(ctx context.Context, deviceID string, incidentID *string, actionType string, metadata string) (*pb.RecordExecutionResponse, error) {
	// 1. Validate Action against Policy via Core gRPC
	policyResp, err := e.coreClient.Client().GetPolicy(ctx, &pb.GetPolicyRequest{
		Action:   actionType,
		DeviceId: deviceID,
	})
	if err != nil {
		return nil, fmt.Errorf("policy validation failed: %w", err)
	}

	// 2. Map requires approval / status
	status := "pending"
	if !policyResp.RequiresApproval {
		status = "approved"
	}

	var incID string
	if incidentID != nil {
		incID = *incidentID
	}

	// 3. Create execution record via Core gRPC
	recordResp, err := e.coreClient.Client().RecordExecution(ctx, &pb.RecordExecutionRequest{
		Action:      actionType,
		DeviceId:    deviceID,
		IncidentId:  incID,
		Status:      status,
		SuggestedBy: "argus_ai",
		Metadata:    metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	// 4. If no approval required, execute immediately
	if !policyResp.RequiresApproval {
		if err := e.Execute(ctx, recordResp.Id, actionType, deviceID); err != nil {
			return recordResp, fmt.Errorf("immediate execution failed: %w", err)
		}
	}

	return recordResp, nil
}

func (e *Engine) Execute(ctx context.Context, recordID string, actionType string, deviceID string) error {
	// Map action type to actual device command
	var cmdName string

	switch actionType {
	case "reboot":
		cmdName = "reboot"
	case "restart_service":
		cmdName = "restart_service"
	case "update_config":
		cmdName = "update_config"
	case "rollback_firmware":
		cmdName = "rollback_firmware"
	default:
		return fmt.Errorf("unsupported action type for execution: %s", actionType)
	}

	_, err := e.coreClient.Client().ExecuteCommand(ctx, &pb.ExecuteCommandRequest{
		DeviceId:    deviceID,
		CommandType: cmdName,
		PayloadJson: "{}",
	})

	if err != nil {
		_, _ = e.coreClient.Client().RecordExecution(ctx, &pb.RecordExecutionRequest{
			Id:     recordID,
			Status: "failed",
		})
		return fmt.Errorf("send command failed: %w", err)
	}

	_, _ = e.coreClient.Client().RecordExecution(ctx, &pb.RecordExecutionRequest{
		Id:     recordID,
		Status: "executed",
	})
	return nil
}
