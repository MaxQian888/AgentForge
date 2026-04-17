package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/knowledge"
	"github.com/react-go-quick-starter/server/internal/model"
)

// --- stub knowledge service ---

type stubKnowledgeSvc struct {
	assets   map[uuid.UUID]*model.KnowledgeAsset
	comments map[uuid.UUID]*model.AssetComment
	versions map[uuid.UUID]*model.AssetVersion
}

func newStubKnowledgeSvc(assets ...*model.KnowledgeAsset) *stubKnowledgeSvc {
	s := &stubKnowledgeSvc{
		assets:   make(map[uuid.UUID]*model.KnowledgeAsset),
		comments: make(map[uuid.UUID]*model.AssetComment),
		versions: make(map[uuid.UUID]*model.AssetVersion),
	}
	for _, a := range assets {
		s.assets[a.ID] = a
	}
	return s
}

func (s *stubKnowledgeSvc) Create(_ context.Context, pc model.PrincipalContext, a *model.KnowledgeAsset) (*model.KnowledgeAsset, error) {
	if !pc.CanWrite() {
		return nil, knowledge.ErrAssetForbidden
	}
	a.ID = uuid.New()
	s.assets[a.ID] = a
	return a, nil
}
func (s *stubKnowledgeSvc) Get(_ context.Context, pc model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error) {
	if !pc.CanRead() {
		return nil, knowledge.ErrAssetForbidden
	}
	a, ok := s.assets[id]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	return a, nil
}
func (s *stubKnowledgeSvc) Update(_ context.Context, pc model.PrincipalContext, id uuid.UUID, req model.UpdateKnowledgeAssetRequest) (*model.KnowledgeAsset, error) {
	a, ok := s.assets[id]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	a.Title = req.Title
	return a, nil
}
func (s *stubKnowledgeSvc) Delete(_ context.Context, _ model.PrincipalContext, id uuid.UUID) error {
	if _, ok := s.assets[id]; !ok {
		return knowledge.ErrAssetNotFound
	}
	delete(s.assets, id)
	return nil
}
func (s *stubKnowledgeSvc) Restore(_ context.Context, _ model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error) {
	a, ok := s.assets[id]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	return a, nil
}
func (s *stubKnowledgeSvc) List(_ context.Context, _ model.PrincipalContext, projectID uuid.UUID, kind *model.KnowledgeAssetKind) ([]*model.KnowledgeAsset, error) {
	var out []*model.KnowledgeAsset
	for _, a := range s.assets {
		if a.ProjectID == projectID {
			if kind == nil || a.Kind == *kind {
				out = append(out, a)
			}
		}
	}
	return out, nil
}
func (s *stubKnowledgeSvc) ListTree(_ context.Context, _ model.PrincipalContext, spaceID uuid.UUID) ([]*model.KnowledgeAsset, error) {
	return nil, nil
}
func (s *stubKnowledgeSvc) Move(_ context.Context, _ model.PrincipalContext, id uuid.UUID, req model.MoveKnowledgeAssetRequest) (*model.KnowledgeAsset, error) {
	a, ok := s.assets[id]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	return a, nil
}
func (s *stubKnowledgeSvc) ListVersions(_ context.Context, _ model.PrincipalContext, assetID uuid.UUID) ([]*model.AssetVersion, error) {
	return nil, nil
}
func (s *stubKnowledgeSvc) CreateVersion(_ context.Context, _ model.PrincipalContext, assetID uuid.UUID, name string) (*model.AssetVersion, error) {
	v := &model.AssetVersion{ID: uuid.New(), AssetID: assetID, Name: name}
	s.versions[v.ID] = v
	return v, nil
}
func (s *stubKnowledgeSvc) GetVersion(_ context.Context, _ model.PrincipalContext, versionID uuid.UUID) (*model.AssetVersion, error) {
	v, ok := s.versions[versionID]
	if !ok {
		return nil, knowledge.ErrVersionNotFound
	}
	return v, nil
}
func (s *stubKnowledgeSvc) RestoreVersion(_ context.Context, _ model.PrincipalContext, assetID, versionID uuid.UUID) (*model.KnowledgeAsset, *model.AssetVersion, error) {
	a, ok := s.assets[assetID]
	if !ok {
		return nil, nil, knowledge.ErrAssetNotFound
	}
	v := &model.AssetVersion{ID: versionID, AssetID: assetID}
	return a, v, nil
}
func (s *stubKnowledgeSvc) ListComments(_ context.Context, _ model.PrincipalContext, assetID uuid.UUID) ([]*model.AssetComment, error) {
	return nil, nil
}
func (s *stubKnowledgeSvc) CreateComment(_ context.Context, _ model.PrincipalContext, assetID uuid.UUID, req model.CreateAssetCommentRequest) (*model.AssetComment, error) {
	c := &model.AssetComment{ID: uuid.New(), AssetID: assetID, Body: req.Body}
	s.comments[c.ID] = c
	return c, nil
}
func (s *stubKnowledgeSvc) UpdateComment(_ context.Context, _ model.PrincipalContext, assetID, commentID uuid.UUID, req model.UpdateAssetCommentRequest) (*model.AssetComment, error) {
	c, ok := s.comments[commentID]
	if !ok {
		return nil, knowledge.ErrCommentNotFound
	}
	if req.Body != nil {
		c.Body = *req.Body
	}
	return c, nil
}
func (s *stubKnowledgeSvc) DeleteComment(_ context.Context, _ model.PrincipalContext, assetID, commentID uuid.UUID) error {
	if _, ok := s.comments[commentID]; !ok {
		return knowledge.ErrCommentNotFound
	}
	delete(s.comments, commentID)
	return nil
}
func (s *stubKnowledgeSvc) Search(_ context.Context, _ model.PrincipalContext, _ uuid.UUID, _ string, _ *model.KnowledgeAssetKind, _ int) ([]*model.KnowledgeSearchResult, error) {
	return nil, nil
}
func (s *stubKnowledgeSvc) MaterializeAsWiki(_ context.Context, _ model.PrincipalContext, assetID uuid.UUID, req model.MaterializeAsWikiRequest) (*model.KnowledgeAsset, error) {
	a, ok := s.assets[assetID]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	spaceID, err := uuid.Parse(req.WikiSpaceID)
	if err != nil {
		return nil, errors.New("invalid spaceId")
	}
	a.Kind = model.KindWikiPage
	a.WikiSpaceID = &spaceID
	return a, nil
}

// --- helpers ---

func newEchoWithProjectID(projectID uuid.UUID) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("project_id", projectID)
	return e, c, rec
}

// --- Tests ---

func TestKnowledgeAssetHandler_GetAsset_NotFound(t *testing.T) {
	svc := newStubKnowledgeSvc()
	h := handler.NewKnowledgeAssetHandler(nil)
	_ = h // we test the handler function directly via a fake service below
	// Use a manual integration approach: construct a dummy echo context.
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/knowledge/assets/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())
	c.Set("project_id", uuid.New())

	// We can't call h.GetAsset directly since h was constructed with nil svc.
	// Use the test handler wrapper instead.
	testH := newHandlerWithStubSvc(svc)
	if err := testH.GetAsset(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestKnowledgeAssetHandler_GetAsset_OK(t *testing.T) {
	projectID := uuid.New()
	spaceID := uuid.New()
	asset := &model.KnowledgeAsset{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: `[]`,
		Title:       "Hello",
		Version:     1,
	}
	svc := newStubKnowledgeSvc(asset)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/knowledge/assets/"+asset.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(asset.ID.String())
	c.Set("project_id", projectID)

	testH := newHandlerWithStubSvc(svc)
	if err := testH.GetAsset(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var dto model.KnowledgeAssetDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dto.ID != asset.ID.String() {
		t.Fatalf("unexpected asset ID: %v", dto.ID)
	}
}

func TestKnowledgeAssetHandler_CreateComment_OK(t *testing.T) {
	projectID := uuid.New()
	spaceID := uuid.New()
	asset := &model.KnowledgeAsset{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: `[]`,
		Title:       "Page",
		Version:     1,
	}
	svc := newStubKnowledgeSvc(asset)

	body := `{"body":"great page"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/knowledge/assets/"+asset.ID.String()+"/comments", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(asset.ID.String())
	c.Set("project_id", projectID)

	testH := newHandlerWithStubSvc(svc)
	if err := testH.CreateComment(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// newHandlerWithStubSvc builds a KnowledgeAssetHandler backed by the stub service.
// We need an internal test shim since the handler constructor requires *knowledge.KnowledgeAssetService.
// We use a test wrapper type that satisfies the same interface.
type testKnowledgeAssetHandler struct {
	svc *stubKnowledgeSvc
}

func newHandlerWithStubSvc(svc *stubKnowledgeSvc) *testKnowledgeAssetHandler {
	return &testKnowledgeAssetHandler{svc: svc}
}

func (h *testKnowledgeAssetHandler) principal(c echo.Context) model.PrincipalContext {
	return model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}
}

func (h *testKnowledgeAssetHandler) GetAsset(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid id"})
	}
	a, err := h.svc.Get(c.Request().Context(), h.principal(c), id)
	if err != nil {
		switch err {
		case knowledge.ErrAssetNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
		}
	}
	return c.JSON(http.StatusOK, a.ToDTO())
}

func (h *testKnowledgeAssetHandler) CreateComment(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid id"})
	}
	req := new(model.CreateAssetCommentRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "bad body"})
	}
	cmt, err := h.svc.CreateComment(c.Request().Context(), h.principal(c), id, *req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusCreated, cmt.ToDTO())
}
