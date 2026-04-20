package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func newArchivedTestContext(project *model.Project) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/projects/x/members", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if project != nil {
		c.Set(ProjectContextKey, project)
	}
	return c, rec
}

func TestArchivedGuardAllowsReadOnlyAction(t *testing.T) {
	now := time.Now().UTC()
	owner := uuid.New()
	project := &model.Project{
		ID:               uuid.New(),
		Status:           model.ProjectStatusArchived,
		ArchivedAt:       &now,
		ArchivedByUserID: &owner,
	}
	c, rec := newArchivedTestContext(project)

	called := false
	handler := ArchivedGuard(ActionProjectRead)(func(echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !called {
		t.Errorf("expected downstream handler to run for .read action")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestArchivedGuardBlocksWriteAction(t *testing.T) {
	now := time.Now().UTC()
	owner := uuid.New()
	project := &model.Project{
		ID:               uuid.New(),
		Status:           model.ProjectStatusArchived,
		ArchivedAt:       &now,
		ArchivedByUserID: &owner,
	}
	c, rec := newArchivedTestContext(project)

	called := false
	handler := ArchivedGuard(ActionTaskCreate)(func(echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if called {
		t.Errorf("downstream handler should not run for write action on archived project")
	}
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestArchivedGuardAllowsUnarchiveWhitelist(t *testing.T) {
	project := &model.Project{ID: uuid.New(), Status: model.ProjectStatusArchived}
	c, rec := newArchivedTestContext(project)

	called := false
	handler := ArchivedGuard(ActionProjectUnarchive)(func(echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !called {
		t.Errorf("expected unarchive action to be whitelisted")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestArchivedGuardSkipsWhenProjectActive(t *testing.T) {
	project := &model.Project{ID: uuid.New(), Status: model.ProjectStatusActive}
	c, rec := newArchivedTestContext(project)

	called := false
	handler := ArchivedGuard(ActionTaskCreate)(func(echo.Context) error {
		called = true
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !called {
		t.Errorf("expected downstream handler to run on active project")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestArchivedProjectWriteGuardGroupLevel(t *testing.T) {
	project := &model.Project{ID: uuid.New(), Status: model.ProjectStatusArchived}

	cases := []struct {
		name           string
		method         string
		path           string
		expectCallDown bool
	}{
		{"read GET allowed", http.MethodGet, "/projects/:pid/tasks", true},
		{"write POST blocked", http.MethodPost, "/projects/:pid/tasks", false},
		{"write DELETE blocked", http.MethodDelete, "/projects/:pid/tasks/:tid", false},
		{"whitelisted unarchive POST allowed", http.MethodPost, "/projects/:pid/unarchive", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(tc.method, "/projects/x"+tc.path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set(ProjectContextKey, project)
			c.SetPath("/projects/:pid" + tc.path)
			called := false
			handler := ArchivedProjectWriteGuard(ArchivedProjectWriteGuardConfig{
				WhitelistedSuffixes: []string{"/unarchive"},
			})(func(echo.Context) error {
				called = true
				return c.NoContent(http.StatusOK)
			})
			if err := handler(c); err != nil {
				t.Fatalf("handler returned error: %v", err)
			}
			if called != tc.expectCallDown {
				t.Errorf("expected downstream call=%v, got %v", tc.expectCallDown, called)
			}
		})
	}
}
