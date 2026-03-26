package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type automationRuleRepoMock struct {
	rules     []*model.AutomationRule
	created   *model.AutomationRule
	updated   *model.AutomationRule
	deletedID uuid.UUID
}

func (m *automationRuleRepoMock) Create(_ context.Context, rule *model.AutomationRule) error {
	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}
	m.created = rule
	return nil
}
func (m *automationRuleRepoMock) GetByID(_ context.Context, id uuid.UUID) (*model.AutomationRule, error) {
	for _, rule := range m.rules {
		if rule.ID == id {
			return rule, nil
		}
	}
	return nil, nil
}
func (m *automationRuleRepoMock) ListByProject(_ context.Context, _ uuid.UUID) ([]*model.AutomationRule, error) {
	return m.rules, nil
}
func (m *automationRuleRepoMock) ListByProjectAndEvent(_ context.Context, _ uuid.UUID, eventType string) ([]*model.AutomationRule, error) {
	if eventType == "" {
		return m.rules, nil
	}
	filtered := make([]*model.AutomationRule, 0)
	for _, rule := range m.rules {
		if rule.EventType == eventType {
			filtered = append(filtered, rule)
		}
	}
	return filtered, nil
}
func (m *automationRuleRepoMock) Update(_ context.Context, rule *model.AutomationRule) error {
	m.updated = rule
	return nil
}
func (m *automationRuleRepoMock) Delete(_ context.Context, id uuid.UUID) error {
	m.deletedID = id
	return nil
}

type automationLogRepoMock struct {
	logs []*model.AutomationLog
}

func (m *automationLogRepoMock) ListByProject(_ context.Context, _ uuid.UUID, _ model.AutomationLogListQuery) ([]*model.AutomationLog, int, error) {
	return m.logs, len(m.logs), nil
}

func TestAutomationHandler_ListCreateAndLogs(t *testing.T) {
	projectID := uuid.New()
	ruleID := uuid.New()
	now := time.Now().UTC()
	ruleRepo := &automationRuleRepoMock{
		rules: []*model.AutomationRule{{
			ID:         ruleID,
			ProjectID:  projectID,
			Name:       "Notify done",
			Enabled:    true,
			EventType:  model.AutomationEventTaskStatusChanged,
			Conditions: `[{"field":"status","op":"eq","value":"done"}]`,
			Actions:    `[{"type":"send_notification"}]`,
			CreatedBy:  uuid.New(),
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
	}
	logRepo := &automationLogRepoMock{
		logs: []*model.AutomationLog{{
			ID:          uuid.New(),
			RuleID:      ruleID,
			EventType:   model.AutomationEventTaskStatusChanged,
			TriggeredAt: now,
			Status:      model.AutomationLogStatusSuccess,
			Detail:      `{"status":"ok"}`,
		}},
	}

	e := echo.New()
	e.Validator = &customFieldValidator{validator: validator.New()}
	h := handler.NewAutomationHandler(ruleRepo, logRepo)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/automations", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	if err := h.ListRules(listCtx); err != nil {
		t.Fatalf("ListRules() error: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/automations", strings.NewReader(`{"name":"Escalate","eventType":"task.field_changed","conditions":[],"actions":[]}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	createCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: uuid.New().String()})
	if err := h.CreateRule(createCtx); err != nil {
		t.Fatalf("CreateRule() error: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}
	if ruleRepo.created == nil || ruleRepo.created.EventType != model.AutomationEventTaskFieldChanged {
		t.Fatalf("unexpected created rule: %+v", ruleRepo.created)
	}

	logReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/automations/logs", nil)
	logRec := httptest.NewRecorder()
	logCtx := e.NewContext(logReq, logRec)
	logCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	if err := h.ListLogs(logCtx); err != nil {
		t.Fatalf("ListLogs() error: %v", err)
	}
	if logRec.Code != http.StatusOK {
		t.Fatalf("logs status = %d, want %d", logRec.Code, http.StatusOK)
	}
	var payload map[string]any
	if err := json.Unmarshal(logRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode logs payload: %v", err)
	}
	if payload["total"].(float64) != 1 {
		t.Fatalf("unexpected logs payload: %#v", payload)
	}
}
