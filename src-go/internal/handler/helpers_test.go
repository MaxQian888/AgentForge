package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func TestClaimsUserID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	c.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})

	got, err := claimsUserID(c)
	if err != nil {
		t.Fatalf("claimsUserID() error = %v", err)
	}
	if got == nil || *got != userID {
		t.Fatalf("claimsUserID() = %#v, want %s", got, userID)
	}
}

func TestParseOptionalUUIDString(t *testing.T) {
	valid := "  " + uuid.MustParse("11111111-1111-1111-1111-111111111111").String() + "  "
	blank := "   "
	invalid := "not-a-uuid"

	if got, err := parseOptionalUUIDString(nil); err != nil || got != nil {
		t.Fatalf("parseOptionalUUIDString(nil) = %#v, %v; want nil, nil", got, err)
	}
	if got, err := parseOptionalUUIDString(&blank); err != nil || got != nil {
		t.Fatalf("parseOptionalUUIDString(blank) = %#v, %v; want nil, nil", got, err)
	}
	if got, err := parseOptionalUUIDString(&valid); err != nil || got == nil || got.String() != strings.TrimSpace(valid) {
		t.Fatalf("parseOptionalUUIDString(valid) = %#v, %v", got, err)
	}
	if _, err := parseOptionalUUIDString(&invalid); err == nil {
		t.Fatal("parseOptionalUUIDString(invalid) expected error")
	}
}

func TestParseOptionalTimeString(t *testing.T) {
	validTime := time.Date(2026, 3, 30, 16, 0, 0, 0, time.UTC).Format(time.RFC3339)
	valid := "  " + validTime + "  "
	blank := " "
	invalid := "2026-03-30"

	if got, err := parseOptionalTimeString(nil); err != nil || got != nil {
		t.Fatalf("parseOptionalTimeString(nil) = %#v, %v; want nil, nil", got, err)
	}
	if got, err := parseOptionalTimeString(&blank); err != nil || got != nil {
		t.Fatalf("parseOptionalTimeString(blank) = %#v, %v; want nil, nil", got, err)
	}
	if got, err := parseOptionalTimeString(&valid); err != nil || got == nil || got.UTC().Format(time.RFC3339) != validTime {
		t.Fatalf("parseOptionalTimeString(valid) = %#v, %v", got, err)
	}
	if _, err := parseOptionalTimeString(&invalid); err == nil {
		t.Fatal("parseOptionalTimeString(invalid) expected error")
	}
}

func TestInferPreflightBudgetScopeAndContainsBudgetScope(t *testing.T) {
	if got := inferPreflightBudgetScope("Task budget exceeded due to retries"); got != "task" {
		t.Fatalf("inferPreflightBudgetScope(task) = %q, want task", got)
	}
	if got := inferPreflightBudgetScope("SPRINT budget warning"); got != "sprint" {
		t.Fatalf("inferPreflightBudgetScope(sprint) = %q, want sprint", got)
	}
	if got := inferPreflightBudgetScope("  project budget blocked "); got != "project" {
		t.Fatalf("inferPreflightBudgetScope(project) = %q, want project", got)
	}
	if got := inferPreflightBudgetScope(""); got != "" {
		t.Fatalf("inferPreflightBudgetScope(empty) = %q, want empty", got)
	}
	if got := inferPreflightBudgetScope("capacity reached"); got != "" {
		t.Fatalf("inferPreflightBudgetScope(unmatched) = %q, want empty", got)
	}

	if !containsBudgetScope(" Task budget warning ", "task") {
		t.Fatal("containsBudgetScope(task) = false, want true")
	}
	if containsBudgetScope("task_budget_warning", "task") {
		t.Fatal("containsBudgetScope(task_budget_warning) = true, want false")
	}
}
