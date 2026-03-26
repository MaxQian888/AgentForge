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

type entityLinkTestValidator struct {
	validator *validator.Validate
}

func (v *entityLinkTestValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type mockEntityLinkHandlerService struct {
	links []*model.EntityLink
}

func (m *mockEntityLinkHandlerService) CreateLink(_ context.Context, input *service.CreateEntityLinkInput) (*model.EntityLink, error) {
	link := &model.EntityLink{
		ID:         uuid.New(),
		ProjectID:  input.ProjectID,
		SourceType: input.SourceType,
		SourceID:   input.SourceID,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		LinkType:   input.LinkType,
		CreatedBy:  input.CreatedBy,
	}
	m.links = append(m.links, link)
	return link, nil
}

func (m *mockEntityLinkHandlerService) DeleteLink(_ context.Context, linkID uuid.UUID) error {
	filtered := make([]*model.EntityLink, 0, len(m.links))
	for _, link := range m.links {
		if link.ID != linkID {
			filtered = append(filtered, link)
		}
	}
	m.links = filtered
	return nil
}

func (m *mockEntityLinkHandlerService) ListLinksForEntity(_ context.Context, projectID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.EntityLink, error) {
	result := make([]*model.EntityLink, 0)
	for _, link := range m.links {
		if link.ProjectID != projectID {
			continue
		}
		if (link.SourceType == entityType && link.SourceID == entityID) || (link.TargetType == entityType && link.TargetID == entityID) {
			cloned := *link
			result = append(result, &cloned)
		}
	}
	return result, nil
}

func TestEntityLinkHandlerCRUD(t *testing.T) {
	e := echo.New()
	e.Validator = &entityLinkTestValidator{validator: validator.New()}
	projectID := uuid.New()
	userID := uuid.New()
	taskID := uuid.New()
	pageID := uuid.New()
	svc := &mockEntityLinkHandlerService{}
	h := NewEntityLinkHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/id/links", strings.NewReader(`{"sourceType":"task","sourceId":"`+taskID.String()+`","targetType":"wiki_page","targetId":"`+pageID.String()+`","linkType":"requirement"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})

	if err := h.Create(c); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create() status = %d", rec.Code)
	}

	var created model.EntityLinkDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created link: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/id/links?source_type=task&source_id="+taskID.String(), nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	if err := h.List(listCtx); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List() status = %d", listRec.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/id/links/"+created.ID, nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetParamNames("linkId")
	deleteCtx.SetParamValues(created.ID)
	if err := h.Delete(deleteCtx); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("Delete() status = %d", deleteRec.Code)
	}
}
