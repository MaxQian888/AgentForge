package handler_test

import (
	"context"
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

type savedViewServiceMock struct {
	views     []*model.SavedView
	created   *model.SavedView
	defaultID uuid.UUID
}

func (m *savedViewServiceMock) CreateView(_ context.Context, view *model.SavedView) error {
	if view.ID == uuid.Nil {
		view.ID = uuid.New()
	}
	m.created = view
	return nil
}
func (m *savedViewServiceMock) GetView(_ context.Context, _ uuid.UUID) (*model.SavedView, error) {
	return nil, nil
}
func (m *savedViewServiceMock) UpdateView(_ context.Context, _ *model.SavedView) error { return nil }
func (m *savedViewServiceMock) DeleteView(_ context.Context, _ uuid.UUID) error        { return nil }
func (m *savedViewServiceMock) ListAccessibleViews(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ []string) ([]*model.SavedView, error) {
	return m.views, nil
}
func (m *savedViewServiceMock) SetDefaultView(_ context.Context, _ uuid.UUID, viewID uuid.UUID) error {
	m.defaultID = viewID
	return nil
}

type savedViewMemberLookupMock struct {
	member *model.Member
}

func (m *savedViewMemberLookupMock) GetByUserAndProject(_ context.Context, _, _ uuid.UUID) (*model.Member, error) {
	return m.member, nil
}

func TestSavedViewHandler_ListAndCreate(t *testing.T) {
	projectID := uuid.New()
	userID := uuid.New()
	viewID := uuid.New()
	now := time.Now().UTC()
	svc := &savedViewServiceMock{
		views: []*model.SavedView{{
			ID:         viewID,
			ProjectID:  projectID,
			Name:       "My View",
			OwnerID:    &userID,
			IsDefault:  true,
			SharedWith: `{}`,
			Config:     `{"layout":"table"}`,
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
	}
	memberLookup := &savedViewMemberLookupMock{member: &model.Member{Role: "reviewer"}}

	e := echo.New()
	e.Validator = &customFieldValidator{validator: validator.New()}
	claims := &service.Claims{UserID: userID.String()}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/views", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	listCtx.Set(appMiddleware.JWTContextKey, claims)

	h := handler.NewSavedViewHandler(svc, memberLookup)
	if err := h.List(listCtx); err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/views", strings.NewReader(`{"name":"Review Queue","config":{"layout":"list"},"sharedWith":{"roleIds":["reviewer"]},"isDefault":true}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	createCtx.Set(appMiddleware.JWTContextKey, claims)

	if err := h.Create(createCtx); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}
	if svc.created == nil || svc.created.Name != "Review Queue" {
		t.Fatalf("unexpected created view: %+v", svc.created)
	}
	if svc.defaultID == uuid.Nil {
		t.Fatal("expected SetDefaultView to be called")
	}
}
