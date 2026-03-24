package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeMemberRepo struct {
	members []*model.Member
}

func (f *fakeMemberRepo) Create(context.Context, *model.Member) error {
	return nil
}

func (f *fakeMemberRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.Member, error) {
	results := make([]*model.Member, 0, len(f.members))
	for _, member := range f.members {
		if member != nil && member.ProjectID == projectID {
			results = append(results, member)
		}
	}
	return results, nil
}

func (f *fakeMemberRepo) Update(context.Context, uuid.UUID, *model.UpdateMemberRequest) error {
	return nil
}

func (f *fakeMemberRepo) GetByID(context.Context, uuid.UUID) (*model.Member, error) {
	return nil, nil
}

func TestMemberHandlerListIncludesAgentConfig(t *testing.T) {
	projectID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()

	repo := &fakeMemberRepo{
		members: []*model.Member{
			{
				ID:          memberID,
				ProjectID:   projectID,
				Name:        "Review Bot",
				Type:        "agent",
				Role:        "code-reviewer",
				AgentConfig: `{"roleId":"frontend-developer","runtime":"codex"}`,
				Skills:      []string{"review", "security"},
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewMemberHandler(repo)
	if err := h.List(ctx); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) != 1 {
		t.Fatalf("len(response) = %d, want 1", len(response))
	}

	if response[0]["agentConfig"] != `{"roleId":"frontend-developer","runtime":"codex"}` {
		t.Fatalf("agentConfig = %#v", response[0]["agentConfig"])
	}

	skills, ok := response[0]["skills"].([]any)
	if !ok || len(skills) != 2 {
		t.Fatalf("skills = %#v, want 2 entries", response[0]["skills"])
	}
}
