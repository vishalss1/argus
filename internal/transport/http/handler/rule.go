package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vishalss1/argus/internal/domain/rule"
	"github.com/vishalss1/argus/internal/transport/http/dto"
)

type RuleHandler struct {
	service *rule.Service
}

func NewRuleHandler(service *rule.Service) *RuleHandler {
	return &RuleHandler{service: service}
}

// CreateRule godoc
// @Summary Create telemetry rule
// @Tags rules
// @Accept json
// @Produce json
// @Param request body dto.CreateRuleRequest true "Rule payload"
// @Success 201 {object} rule.Rule
// @Failure 400 {object} dto.ErrorResponse
// @Router /rules [post]
func (h *RuleHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entity, err := h.service.Create(r.Context(), rule.CreateInput{
		Name:      req.Name,
		Metric:    req.Metric,
		Operator:  req.Operator,
		Threshold: req.Threshold,
		Enabled:   req.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, entity)
}

// ListRules godoc
// @Summary List telemetry rules
// @Tags rules
// @Produce json
// @Success 200 {array} rule.Rule
// @Failure 500 {object} dto.ErrorResponse
// @Router /rules [get]
func (h *RuleHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}

	writeJSON(w, http.StatusOK, rules)
}

// GetRule godoc
// @Summary Get telemetry rule
// @Tags rules
// @Produce json
// @Param ruleID path string true "Rule ID"
// @Success 200 {object} rule.Rule
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /rules/{ruleID} [get]
func (h *RuleHandler) GetRule(w http.ResponseWriter, r *http.Request, id string) {
	entity, err := h.service.Get(r.Context(), id)
	if errors.Is(err, rule.ErrRuleNotFound) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
}

// UpdateRule godoc
// @Summary Update telemetry rule
// @Tags rules
// @Accept json
// @Produce json
// @Param ruleID path string true "Rule ID"
// @Param request body dto.UpdateRuleRequest true "Rule update payload"
// @Success 200 {object} rule.Rule
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /rules/{ruleID} [put]
func (h *RuleHandler) UpdateRule(w http.ResponseWriter, r *http.Request, id string) {
	var req dto.UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	entity, err := h.service.Update(r.Context(), id, rule.UpdateInput{
		Name:      req.Name,
		Metric:    req.Metric,
		Operator:  req.Operator,
		Threshold: req.Threshold,
		Enabled:   req.Enabled,
	})
	if errors.Is(err, rule.ErrRuleNotFound) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, entity)
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
	err := h.service.Delete(r.Context(), id)
	if errors.Is(err, rule.ErrRuleNotFound) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAlerts godoc
// @Summary List rule alerts
// @Tags rules
// @Produce json
// @Success 200 {array} rule.Alert
// @Failure 500 {object} dto.ErrorResponse
// @Router /alerts [get]
func (h *RuleHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.service.ListAlerts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list alerts")
		return
	}

	writeJSON(w, http.StatusOK, alerts)
}
