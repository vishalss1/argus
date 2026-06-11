package handler

import (
	"encoding/json"
	"net/http"

	"github.com/vishalss1/argus/core/internal/transport/http/dto"
	telemetrygrpc "github.com/vishalss1/argus/core/internal/infrastructure/grpc"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
)

type RuleHandler struct {
	telemetryClient *telemetrygrpc.TelemetryClient
}

func NewRuleHandler(telemetryClient *telemetrygrpc.TelemetryClient) *RuleHandler {
	return &RuleHandler{telemetryClient: telemetryClient}
}

// CreateRule godoc
// @Summary Create telemetry rule
// @Tags rules
// @Accept json
// @Produce json
// @Param request body dto.CreateRuleRequest true "Rule payload"
// @Success 201 {object} pb.RuleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /rules [post]
func (h *RuleHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	resp, err := h.telemetryClient.Client().ConfigureRule(r.Context(), &pb.ConfigureRuleRequest{
		RuleId:    "", // Create new
		Name:      req.Name,
		Metric:    req.Metric,
		Operator:  req.Operator,
		Threshold: req.Threshold,
		Enabled:   enabled,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

// ListRules godoc
// @Summary List telemetry rules
// @Tags rules
// @Produce json
// @Success 200 {array} pb.RuleResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /rules [get]
func (h *RuleHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	resp, err := h.telemetryClient.Client().ListRules(r.Context(), &pb.ListRulesRequest{
		EnabledOnly: false,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list rules: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp.Rules)
}

// GetRule godoc
// @Summary Get telemetry rule
// @Tags rules
// @Produce json
// @Param ruleID path string true "Rule ID"
// @Success 200 {object} pb.RuleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /rules/{ruleID} [get]
func (h *RuleHandler) GetRule(w http.ResponseWriter, r *http.Request, id string) {
	// Filter ListRules to find the specific ID
	resp, err := h.telemetryClient.Client().ListRules(r.Context(), &pb.ListRulesRequest{
		EnabledOnly: false,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	for _, rule := range resp.Rules {
		if rule.Id == id {
			writeJSON(w, http.StatusOK, rule)
			return
		}
	}

	writeError(w, http.StatusNotFound, "rule not found")
}

// UpdateRule godoc
// @Summary Update telemetry rule
// @Tags rules
// @Accept json
// @Produce json
// @Param ruleID path string true "Rule ID"
// @Param request body dto.UpdateRuleRequest true "Rule update payload"
// @Success 200 {object} pb.RuleResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /rules/{ruleID} [put]
func (h *RuleHandler) UpdateRule(w http.ResponseWriter, r *http.Request, id string) {
	var req dto.UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Fetch existing first to fill in blanks
	resp, err := h.telemetryClient.Client().ListRules(r.Context(), &pb.ListRulesRequest{
		EnabledOnly: false,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var existing *pb.RuleResponse
	for _, rule := range resp.Rules {
		if rule.Id == id {
			existing = rule
			break
		}
	}

	if existing == nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	metric := existing.Metric
	if req.Metric != nil {
		metric = *req.Metric
	}
	operator := existing.Operator
	if req.Operator != nil {
		operator = *req.Operator
	}
	threshold := existing.Threshold
	if req.Threshold != nil {
		threshold = *req.Threshold
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	updated, err := h.telemetryClient.Client().ConfigureRule(r.Context(), &pb.ConfigureRuleRequest{
		RuleId:    id,
		Name:      name,
		Metric:    metric,
		Operator:  operator,
		Threshold: threshold,
		Enabled:   enabled,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteRule godoc
// @Summary Delete telemetry rule
// @Tags rules
// @Param ruleID path string true "Rule ID"
// @Success 204
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /rules/{ruleID} [delete]
func (h *RuleHandler) DeleteRule(w http.ResponseWriter, r *http.Request, id string) {
	_, err := h.telemetryClient.Client().DeleteRule(r.Context(), &pb.DeleteRuleRequest{
		RuleId: id,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		// Assume error means not found or bad request
		writeError(w, http.StatusNotFound, "rule not found or deletion failed: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAlerts godoc
// @Summary List rule alerts
// @Tags rules
// @Produce json
// @Success 200 {array} pb.AlertResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /alerts [get]
func (h *RuleHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	// Call GetRulesWithAlerts with limit and offset (defaults)
	resp, err := h.telemetryClient.Client().GetRulesWithAlerts(r.Context(), &pb.GetRulesWithAlertsRequest{
		WorkspaceId: "", // Retrieve all
		Limit:       100,
		Offset:      0,
	})
	if err != nil {
		if isGrpcOrBreakerError(err) {
			writeError(w, http.StatusServiceUnavailable, "Telemetry service is unavailable: "+err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list alerts: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp.Alerts)
}
