package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestCostHandler_GetStats_Default(t *testing.T) {
	// With nil db, ListActive will return ErrDatabaseUnavailable
	repo := repository.NewAgentRunRepository(nil)
	h := handler.NewCostHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/cost", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStats(c); err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	// With nil db, should get 500
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestCostHandler_GetStats_InvalidProjectId(t *testing.T) {
	repo := repository.NewAgentRunRepository(nil)
	h := handler.NewCostHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/cost?projectId=invalid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStats(c); err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Message != "invalid projectId" {
		t.Fatalf("message = %q", body.Message)
	}
}

func TestCostHandler_GetStats_InvalidSprintId(t *testing.T) {
	repo := repository.NewAgentRunRepository(nil)
	h := handler.NewCostHandler(repo)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/cost?sprintId=nope", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStats(c); err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
