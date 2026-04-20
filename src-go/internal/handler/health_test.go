// src-go/internal/handler/health_test.go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/server/internal/handler"
	"github.com/labstack/echo/v4"
)

func TestHealth_Returns200WithStatus(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handler.NewHealthHandler("test", "abc123", "2025-01-01", "test")
	if err := h.Health(c); err != nil {
		t.Fatalf("Health() error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field: want ok, got %q", body["status"])
	}
	if body["time"] == "" {
		t.Error("time field: want non-empty RFC3339 string, got empty")
	}
}

func TestHealthV1_IncludesVersionAndEnv(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handler.NewHealthHandler("1.0.0", "abc123", "2025-01-01", "test")
	if err := h.HealthV1(c); err != nil {
		t.Fatalf("HealthV1() error: %v", err)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["version"] != "1.0.0" {
		t.Errorf("version: want 1.0.0, got %q", body["version"])
	}
	if body["env"] != "test" {
		t.Errorf("env: want test, got %q", body["env"])
	}
}
