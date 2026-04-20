package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type fakeEmployeeRunsRepo struct {
	rows    []repository.EmployeeRunRow
	err     error
	gotID   uuid.UUID
	gotKind repository.EmployeeRunKind
	gotPage int
	gotSize int
}

func (f *fakeEmployeeRunsRepo) ListByEmployee(_ context.Context, id uuid.UUID, kind repository.EmployeeRunKind, page, size int) ([]repository.EmployeeRunRow, error) {
	f.gotID, f.gotKind, f.gotPage, f.gotSize = id, kind, page, size
	return f.rows, f.err
}

func TestEmployeeRunsHandler_List_DefaultsAndShape(t *testing.T) {
	e := echo.New()
	empID := uuid.New()
	started := time.Now().Add(-2 * time.Minute)
	completed := started.Add(45 * time.Second)
	repo := &fakeEmployeeRunsRepo{rows: []repository.EmployeeRunRow{{
		Kind: "workflow", ID: uuid.New().String(), Name: "echo-flow",
		Status: "completed", StartedAt: &started, CompletedAt: &completed,
		RefURL: "/workflow/runs/abc",
	}}}
	h := NewEmployeeRunsHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/employees/"+empID.String()+"/runs", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(empID.String())

	if err := h.List(c); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.gotID != empID || repo.gotKind != repository.EmployeeRunKindAll || repo.gotPage != 1 || repo.gotSize != 20 {
		t.Fatalf("repo args: id=%s kind=%s page=%d size=%d", repo.gotID, repo.gotKind, repo.gotPage, repo.gotSize)
	}

	var body struct {
		Items []map[string]any `json:"items"`
		Page  int              `json:"page"`
		Size  int              `json:"size"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0]["kind"] != "workflow" {
		t.Fatalf("body items: %+v", body.Items)
	}
}

func TestEmployeeRunsHandler_List_BadID(t *testing.T) {
	e := echo.New()
	h := NewEmployeeRunsHandler(&fakeEmployeeRunsRepo{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")
	_ = h.List(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
}
