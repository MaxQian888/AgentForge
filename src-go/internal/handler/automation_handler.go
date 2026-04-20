package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type automationRuleRepository interface {
	Create(ctx context.Context, rule *model.AutomationRule) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AutomationRule, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AutomationRule, error)
	ListByProjectAndEvent(ctx context.Context, projectID uuid.UUID, eventType string) ([]*model.AutomationRule, error)
	Update(ctx context.Context, rule *model.AutomationRule) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type automationLogRepository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID, query model.AutomationLogListQuery) ([]*model.AutomationLog, int, error)
}

type AutomationHandler struct {
	rules automationRuleRepository
	logs  automationLogRepository
}

func NewAutomationHandler(rules automationRuleRepository, logs automationLogRepository) *AutomationHandler {
	return &AutomationHandler{rules: rules, logs: logs}
}

func (h *AutomationHandler) ListRules(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	eventType := c.QueryParam("eventType")
	var (
		rules []*model.AutomationRule
		err   error
	)
	if eventType != "" {
		rules, err = h.rules.ListByProjectAndEvent(c.Request().Context(), projectID, eventType)
	} else {
		rules, err = h.rules.ListByProject(c.Request().Context(), projectID)
	}
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListAutomationRules)
	}
	dtos := make([]model.AutomationRuleDTO, 0, len(rules))
	for _, rule := range rules {
		dtos = append(dtos, rule.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *AutomationHandler) CreateRule(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	userID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgAuthRequired)
	}
	req := new(model.CreateAutomationRuleRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if err := service.ValidateAutomationActions(req.Actions); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       req.Name,
		EventType:  req.EventType,
		Conditions: string(req.Conditions),
		Actions:    string(req.Actions),
		CreatedBy:  *userID,
		Enabled:    true,
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if err := h.rules.Create(c.Request().Context(), rule); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateAutomationRule)
	}
	return c.JSON(http.StatusCreated, rule.ToDTO())
}

func (h *AutomationHandler) UpdateRule(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	ruleID, err := uuid.Parse(c.Param("rid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRuleID)
	}
	rule, err := h.rules.GetByID(c.Request().Context(), ruleID)
	if err != nil || rule == nil || rule.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgAutomationRuleNotFound)
	}
	req := new(model.UpdateAutomationRuleRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.EventType != nil {
		rule.EventType = *req.EventType
	}
	if len(req.Conditions) > 0 {
		rule.Conditions = string(req.Conditions)
	}
	if len(req.Actions) > 0 {
		if err := service.ValidateAutomationActions(req.Actions); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
		}
		rule.Actions = string(req.Actions)
	}
	if err := h.rules.Update(c.Request().Context(), rule); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateAutomationRule)
	}
	return c.JSON(http.StatusOK, rule.ToDTO())
}

func (h *AutomationHandler) DeleteRule(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	ruleID, err := uuid.Parse(c.Param("rid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRuleID)
	}
	rule, err := h.rules.GetByID(c.Request().Context(), ruleID)
	if err != nil || rule == nil || rule.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgAutomationRuleNotFound)
	}
	if err := h.rules.Delete(c.Request().Context(), ruleID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteAutomationRule)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "automation rule deleted"})
}

func (h *AutomationHandler) ListLogs(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	query := model.AutomationLogListQuery{
		EventType: c.QueryParam("eventType"),
		Status:    c.QueryParam("status"),
		Page:      page,
		Limit:     limit,
	}
	logs, total, err := h.logs.ListByProject(c.Request().Context(), projectID, query)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListAutomationLogs)
	}
	dtos := make([]model.AutomationLogDTO, 0, len(logs))
	for _, entry := range logs {
		dtos = append(dtos, entry.ToDTO())
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items": dtos,
		"total": total,
		"page":  query.Page,
		"limit": query.Limit,
	})
}
