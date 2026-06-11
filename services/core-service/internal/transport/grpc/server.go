package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	commanddomain "github.com/vishalss1/argus/core/internal/domain/command"
	devicedomain "github.com/vishalss1/argus/core/internal/domain/device"
	policydomain "github.com/vishalss1/argus/core/internal/domain/policy"
	sessiondomain "github.com/vishalss1/argus/core/internal/domain/session"
	usagedomain "github.com/vishalss1/argus/core/internal/domain/usage"
	workspacedomain "github.com/vishalss1/argus/core/internal/domain/workspace"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
	pb "github.com/vishalss1/argus/shared/proto/core"
)

type Server struct {
	pb.UnimplementedCoreServiceServer
	deviceRepo      devicedomain.Repository
	workspaceRepo   workspacedomain.Repository
	sessionService  *sessiondomain.Service
	sessionManager  *sessiondomain.Manager
	commandService  *commanddomain.Service
	policyService   *policydomain.Service
	usageService    *usagedomain.Service
	redisClient     *redis.Client
}

func NewServer(
	deviceRepo devicedomain.Repository,
	workspaceRepo workspacedomain.Repository,
	sessionService *sessiondomain.Service,
	sessionManager *sessiondomain.Manager,
	commandService *commanddomain.Service,
	policyService *policydomain.Service,
	usageService *usagedomain.Service,
	redisClient *redis.Client,
) *Server {
	return &Server{
		deviceRepo:     deviceRepo,
		workspaceRepo:  workspaceRepo,
		sessionService: sessionService,
		sessionManager: sessionManager,
		commandService: commandService,
		policyService:  policyService,
		usageService:   usageService,
		redisClient:    redisClient,
	}
}

func (s *Server) GetDeviceContext(ctx context.Context, req *pb.GetDeviceContextRequest) (*pb.DeviceContextResponse, error) {
	if req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}

	// 1. Fetch Device
	dev, err := s.deviceRepo.GetByID(ctx, req.DeviceId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "device not found: %v", err)
	}

	var workspaceID, workspaceName, workspaceDesc string
	if dev.WorkspaceID != nil {
		workspaceID = *dev.WorkspaceID

		// 2. Fetch Workspace
		ws, err := s.workspaceRepo.Get(ctx, workspaceID)
		if err == nil && ws != nil {
			workspaceName = ws.Name
			if ws.Description != "" {
				workspaceDesc = ws.Description
			}
		}
	}

	// 3. Fetch Active Session
	var activeSessionID, activeSessionStatus string
	var activeSessionStartedAt *timestamppb.Timestamp

	if workspaceID != "" {
		activeKey := fmt.Sprintf("workspace:%s:active_session", workspaceID)
		sID, err := s.redisClient.Client().Get(ctx, activeKey).Result()
		if err == nil && sID != "" {
			sess, err := s.sessionService.Get(ctx, sID)
			if err == nil && sess != nil {
				activeSessionID = sess.ID
				activeSessionStatus = string(sess.Status)
				if sess.StartedAt != nil {
					activeSessionStartedAt = timestamppb.New(*sess.StartedAt)
				}
			}
		}
	}

	// 4. Fetch Tenant limits / Usage (mocked/queried plan)
	tenantID := "00000000-0000-0000-0000-000000000000" // Standard fallback tenant
	billingPlan := "FREE"
	var devicesUsed, maxDevicesAllowed int32 = 0, 10

	// Map metadata JSON
	metadataBytes, _ := json.Marshal(dev.Metadata)

	var lastSeenProto *timestamppb.Timestamp
	if dev.LastSeen != nil {
		lastSeenProto = timestamppb.New(*dev.LastSeen)
	}

	return &pb.DeviceContextResponse{
		DeviceId:                   dev.ID,
		Name:                       dev.Name,
		Type:                       dev.Type,
		FirmwareVersion:            dev.FirmwareVersion,
		Status:                     dev.Status,
		MetadataJson:               string(metadataBytes),
		LastSeen:                   lastSeenProto,
		CreatedAt:                  timestamppb.New(dev.CreatedAt),
		UpdatedAt:                  timestamppb.New(dev.UpdatedAt),
		WorkspaceId:                workspaceID,
		WorkspaceName:              workspaceName,
		WorkspaceDescription:       workspaceDesc,
		ActiveSessionId:            activeSessionID,
		ActiveSessionStatus:        activeSessionStatus,
		ActiveSessionStartedAt:     activeSessionStartedAt,
		TenantId:                   tenantID,
		BillingPlan:                billingPlan,
		DevicesUsed:                devicesUsed,
		MaxDevicesAllowed:          maxDevicesAllowed,
	}, nil
}

func (s *Server) ExecuteCommand(ctx context.Context, req *pb.ExecuteCommandRequest) (*pb.ExecuteCommandResponse, error) {
	if req.DeviceId == "" || req.CommandType == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id and command_type are required")
	}

	var payload json.RawMessage
	if req.PayloadJson != "" {
		payload = json.RawMessage(req.PayloadJson)
	} else {
		payload = json.RawMessage(`{}`)
	}

	cmdInput := commanddomain.SendInput{
		Type:    req.CommandType,
		Payload: payload,
	}

	// If sessionID is provided, verify and pass session context
	cmd, err := s.commandService.Send(ctx, req.DeviceId, cmdInput)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create/dispatch command: %v", err)
	}

	return &pb.ExecuteCommandResponse{
		CommandId: cmd.ID,
		Status:    string(cmd.Status),
		IssuedAt:  timestamppb.New(cmd.CreatedAt),
	}, nil
}

func (s *Server) GetPolicy(ctx context.Context, req *pb.GetPolicyRequest) (*pb.PolicyResponse, error) {
	if req.Action == "" || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "action and device_id are required")
	}

	policy, err := s.policyService.GetPolicyByAction(ctx, policydomain.ActionType(req.Action))
	if err != nil {
		if errors.Is(err, policydomain.ErrPolicyNotFound) {
			return nil, status.Errorf(codes.NotFound, "policy not found for action %s", req.Action)
		}
		return nil, status.Errorf(codes.Internal, "get policy failed: %v", err)
	}

	return &pb.PolicyResponse{
		PolicyId:              policy.ID,
		Action:                string(policy.Action),
		AllowedDevices:        policy.AllowedDevices,
		RequiresApproval:      policy.RequiresApproval,
		MaxPerDay:             int32(policy.MaxPerDay),
		CurrentExecutionsToday: 0, // Mocked/calculated from execution logs
	}, nil
}

func (s *Server) RecordExecution(ctx context.Context, req *pb.RecordExecutionRequest) (*pb.RecordExecutionResponse, error) {
	if req.Action == "" || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "action and device_id are required")
	}

	var err error

	if req.Id != "" {
		// Update existing execution record status/approved_by
		if req.Status == "approved" {
			err = s.policyService.ApproveAction(ctx, req.Id, req.ApprovedBy)
		} else if req.Status == "executed" {
			err = s.policyService.MarkExecuted(ctx, req.Id)
		} else if req.Status == "failed" {
			err = s.policyService.MarkFailed(ctx, req.Id)
		} else {
			err = s.policyService.ApproveAction(ctx, req.Id, req.ApprovedBy)
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update execution status: %v", err)
		}
		return &pb.RecordExecutionResponse{
			Id:     req.Id,
			Status: req.Status,
		}, nil
	}

	// Create new execution record
	var incidentID *string
	if req.IncidentId != "" {
		incidentID = &req.IncidentId
	}
	record := policydomain.ExecutionRecord{
		Action:      policydomain.ActionType(req.Action),
		DeviceID:    req.DeviceId,
		IncidentID:  incidentID,
		Status:      req.Status,
		SuggestedBy: req.SuggestedBy,
		Metadata:    req.Metadata,
	}

	created, err := s.policyService.CreateRecord(ctx, record)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create execution record: %v", err)
	}

	return &pb.RecordExecutionResponse{
		Id:     created.ID,
		Status: created.Status,
	}, nil
}

