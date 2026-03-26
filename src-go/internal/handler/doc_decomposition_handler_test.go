package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type docDecompositionTestValidator struct {
	validator *validator.Validate
}

func (v *docDecompositionTestValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type mockDocDecompositionHandlerService struct{}

func (m *mockDocDecompositionHandlerService) DecomposeTasksFromBlocks(_ context.Context, projectID uuid.UUID, pageID uuid.UUID, blockIDs []string, parentTaskID *uuid.UUID, createdBy *uuid.UUID) (*model.DecomposeTasksFromPageResponse, error) {
	_ = projectID
	_ = parentTaskID
	_ = createdBy
	return &model.DecomposeTasksFromPageResponse{
		PageID:   pageID.String(),
		BlockIDs: append([]string(nil), blockIDs...),
		Tasks: []model.TaskDTO{
			{ID: uuid.New().String(), Title: "Generated"},
		},
	}, nil
}

type notFoundDocDecompositionHandlerService struct{}

func (m *notFoundDocDecompositionHandlerService) DecomposeTasksFromBlocks(_ context.Context, projectID uuid.UUID, pageID uuid.UUID, blockIDs []string, parentTaskID *uuid.UUID, createdBy *uuid.UUID) (*model.DecomposeTasksFromPageResponse, error) {
	return nil, service.ErrWikiPageNotFound
}

func TestDocDecompositionHandlerDecompose(t *testing.T) {
	e := echo.New()
	e.Validator = &docDecompositionTestValidator{validator: validator.New()}
	projectID := uuid.New()
	pageID := uuid.New()
	userID := uuid.New()
	h := NewDocDecompositionHandler(&mockDocDecompositionHandlerService{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/id/wiki/pages/id/decompose-tasks", strings.NewReader(`{"blockIds":["block-a"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(pageID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})

	if err := h.Decompose(c); err != nil {
		t.Fatalf("Decompose() error = %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var body model.DecomposeTasksFromPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(body.Tasks))
	}
}

func TestDocDecompositionHandlerDecomposeReturnsNotFoundForForeignPage(t *testing.T) {
	e := echo.New()
	e.Validator = &docDecompositionTestValidator{validator: validator.New()}
	projectID := uuid.New()
	pageID := uuid.New()
	h := NewDocDecompositionHandler(&notFoundDocDecompositionHandlerService{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/id/wiki/pages/id/decompose-tasks", strings.NewReader(`{"blockIds":["block-a"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(pageID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	if err := h.Decompose(c); err != nil {
		t.Fatalf("Decompose() error = %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
