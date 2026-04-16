package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type wikiTestValidator struct {
	validator *validator.Validate
}

func (v *wikiTestValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type mockWikiHandlerService struct {
	space     *model.WikiSpace
	pages     map[uuid.UUID]*model.WikiPage
	versions  map[uuid.UUID]*model.PageVersion
	comments  map[uuid.UUID]*model.PageComment
	favorites []*model.PageFavorite
	recent    []*model.PageRecentAccess
}

func (m *mockWikiHandlerService) GetSpaceByProjectID(ctx echo.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	_ = ctx
	if m.space == nil || m.space.ProjectID != projectID {
		return nil, service.ErrWikiSpaceNotFound
	}
	cloned := *m.space
	return &cloned, nil
}
func (m *mockWikiHandlerService) GetSpaceByID(ctx echo.Context, spaceID uuid.UUID) (*model.WikiSpace, error) {
	_ = ctx
	if m.space == nil || m.space.ID != spaceID {
		return nil, service.ErrWikiSpaceNotFound
	}
	cloned := *m.space
	return &cloned, nil
}
func (m *mockWikiHandlerService) CreatePage(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID, title string, parentID *uuid.UUID, content string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	_ = ctx
	now := time.Now().UTC().Truncate(time.Second)
	page := &model.WikiPage{
		ID:          uuid.New(),
		SpaceID:     spaceID,
		ParentID:    parentID,
		Title:       title,
		Content:     content,
		ContentText: content,
		Path:        "/" + title,
		SortOrder:   0,
		CreatedBy:   createdBy,
		UpdatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if m.pages == nil {
		m.pages = map[uuid.UUID]*model.WikiPage{}
	}
	m.pages[page.ID] = page
	_ = projectID
	return page, nil
}
func (m *mockWikiHandlerService) CreateTemplate(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID, title string, category string, content string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	_ = ctx
	_ = projectID
	template := &model.WikiPage{
		ID:               uuid.New(),
		SpaceID:          spaceID,
		Title:            title,
		Content:          content,
		ContentText:      content,
		Path:             "/templates/custom/" + title,
		SortOrder:        0,
		IsTemplate:       true,
		TemplateCategory: category,
		IsSystem:         false,
		CreatedBy:        createdBy,
		UpdatedBy:        createdBy,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if m.pages == nil {
		m.pages = map[uuid.UUID]*model.WikiPage{}
	}
	m.pages[template.ID] = template
	return template, nil
}
func (m *mockWikiHandlerService) GetPageTree(ctx echo.Context, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	_ = ctx
	out := make([]*model.WikiPage, 0)
	for _, page := range m.pages {
		if page.SpaceID == spaceID {
			cloned := *page
			out = append(out, &cloned)
		}
	}
	return out, nil
}
func (m *mockWikiHandlerService) GetPage(ctx echo.Context, pageID uuid.UUID) (*model.WikiPage, error) {
	_ = ctx
	page, ok := m.pages[pageID]
	if !ok {
		return nil, service.ErrWikiPageNotFound
	}
	cloned := *page
	return &cloned, nil
}
func (m *mockWikiHandlerService) GetPageContext(ctx echo.Context, pageID uuid.UUID) (*model.WikiSpace, *model.WikiPage, error) {
	page, err := m.GetPage(ctx, pageID)
	if err != nil {
		return nil, nil, err
	}
	return m.space, page, nil
}
func (m *mockWikiHandlerService) UpdatePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, title string, content string, contentText string, updatedBy *uuid.UUID, expectedUpdatedAt *time.Time, templateCategory *string) (*model.WikiPage, error) {
	_ = ctx
	_ = projectID
	page, ok := m.pages[pageID]
	if !ok {
		return nil, service.ErrWikiPageNotFound
	}
	if page.IsTemplate && page.IsSystem {
		return nil, service.ErrWikiTemplateImmutable
	}
	if expectedUpdatedAt != nil && page.UpdatedAt.After(*expectedUpdatedAt) {
		return nil, service.ErrWikiPageConflict
	}
	page.Title = title
	page.Content = content
	page.ContentText = contentText
	if templateCategory != nil {
		page.TemplateCategory = *templateCategory
	}
	page.UpdatedBy = updatedBy
	page.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	cloned := *page
	return &cloned, nil
}
func (m *mockWikiHandlerService) DeletePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID) error {
	_ = ctx
	_ = projectID
	if page, ok := m.pages[pageID]; ok && page.IsTemplate && page.IsSystem {
		return service.ErrWikiTemplateImmutable
	}
	delete(m.pages, pageID)
	return nil
}
func (m *mockWikiHandlerService) MovePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, newParentID *uuid.UUID, sortOrder int) (*model.WikiPage, error) {
	_ = ctx
	_ = projectID
	page, ok := m.pages[pageID]
	if !ok {
		return nil, service.ErrWikiPageNotFound
	}
	if newParentID != nil && *newParentID == pageID {
		return nil, service.ErrWikiCircularMove
	}
	page.ParentID = newParentID
	page.SortOrder = sortOrder
	page.Path = "/" + page.ID.String()
	cloned := *page
	return &cloned, nil
}
func (m *mockWikiHandlerService) ListVersions(ctx echo.Context, pageID uuid.UUID) ([]*model.PageVersion, error) {
	_ = ctx
	out := make([]*model.PageVersion, 0)
	for _, version := range m.versions {
		if version.PageID == pageID {
			cloned := *version
			out = append(out, &cloned)
		}
	}
	return out, nil
}
func (m *mockWikiHandlerService) CreateVersion(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, name string, createdBy *uuid.UUID) (*model.PageVersion, error) {
	_ = ctx
	_ = projectID
	version := &model.PageVersion{
		ID:            uuid.New(),
		PageID:        pageID,
		VersionNumber: 1,
		Name:          name,
		Content:       "[]",
		CreatedBy:     createdBy,
		CreatedAt:     time.Now().UTC(),
	}
	if m.versions == nil {
		m.versions = map[uuid.UUID]*model.PageVersion{}
	}
	m.versions[version.ID] = version
	return version, nil
}
func (m *mockWikiHandlerService) GetVersion(ctx echo.Context, versionID uuid.UUID) (*model.PageVersion, error) {
	_ = ctx
	version, ok := m.versions[versionID]
	if !ok {
		return nil, service.ErrPageVersionNotFound
	}
	cloned := *version
	return &cloned, nil
}
func (m *mockWikiHandlerService) RestoreVersion(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, versionID uuid.UUID, updatedBy *uuid.UUID) (*model.WikiPage, *model.PageVersion, error) {
	_ = ctx
	_ = projectID
	_ = updatedBy
	page, ok := m.pages[pageID]
	if !ok {
		return nil, nil, service.ErrWikiPageNotFound
	}
	version, ok := m.versions[versionID]
	if !ok {
		return nil, nil, service.ErrPageVersionNotFound
	}
	page.Content = version.Content
	clonedPage := *page
	clonedVersion := *version
	return &clonedPage, &clonedVersion, nil
}
func (m *mockWikiHandlerService) ListComments(ctx echo.Context, pageID uuid.UUID) ([]*model.PageComment, error) {
	_ = ctx
	out := make([]*model.PageComment, 0)
	for _, comment := range m.comments {
		if comment.PageID == pageID {
			cloned := *comment
			out = append(out, &cloned)
		}
	}
	return out, nil
}
func (m *mockWikiHandlerService) CreateComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, body string, anchorBlockID *string, parentCommentID *uuid.UUID, createdBy *uuid.UUID, mentions string) (*model.PageComment, error) {
	_ = ctx
	_ = projectID
	comment := &model.PageComment{
		ID:              uuid.New(),
		PageID:          pageID,
		Body:            body,
		AnchorBlockID:   anchorBlockID,
		ParentCommentID: parentCommentID,
		CreatedBy:       createdBy,
		Mentions:        mentions,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if m.comments == nil {
		m.comments = map[uuid.UUID]*model.PageComment{}
	}
	m.comments[comment.ID] = comment
	return comment, nil
}
func (m *mockWikiHandlerService) ResolveComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) (*model.PageComment, error) {
	return m.setCommentResolved(ctx, projectID, pageID, commentID, true)
}
func (m *mockWikiHandlerService) ReopenComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) (*model.PageComment, error) {
	return m.setCommentResolved(ctx, projectID, pageID, commentID, false)
}
func (m *mockWikiHandlerService) setCommentResolved(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID, resolved bool) (*model.PageComment, error) {
	_ = ctx
	_ = projectID
	_ = pageID
	comment, ok := m.comments[commentID]
	if !ok {
		return nil, service.ErrPageCommentNotFound
	}
	if resolved {
		now := time.Now().UTC()
		comment.ResolvedAt = &now
	} else {
		comment.ResolvedAt = nil
	}
	cloned := *comment
	return &cloned, nil
}
func (m *mockWikiHandlerService) DeleteComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) error {
	_ = ctx
	_ = projectID
	_ = pageID
	delete(m.comments, commentID)
	return nil
}
func (m *mockWikiHandlerService) SeedBuiltInTemplates(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	_ = ctx
	_ = projectID
	template := &model.WikiPage{ID: uuid.New(), SpaceID: spaceID, Title: "PRD", IsTemplate: true, IsSystem: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if m.pages == nil {
		m.pages = map[uuid.UUID]*model.WikiPage{}
	}
	m.pages[template.ID] = template
	return []*model.WikiPage{template}, nil
}
func (m *mockWikiHandlerService) CreateTemplateFromPage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, name string, category string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	_ = ctx
	_ = projectID
	_ = pageID
	template := &model.WikiPage{ID: uuid.New(), SpaceID: m.space.ID, Title: name, TemplateCategory: category, IsTemplate: true, CreatedBy: createdBy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	m.pages[template.ID] = template
	return template, nil
}
func (m *mockWikiHandlerService) CreatePageFromTemplate(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID, templateID uuid.UUID, parentID *uuid.UUID, title string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	_ = ctx
	_ = projectID
	_ = templateID
	page := &model.WikiPage{ID: uuid.New(), SpaceID: spaceID, ParentID: parentID, Title: title, CreatedBy: createdBy, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	m.pages[page.ID] = page
	return page, nil
}
func (m *mockWikiHandlerService) ListTemplates(ctx echo.Context, spaceID uuid.UUID, query string, category string, source string) ([]*model.WikiPage, error) {
	_ = ctx
	out := make([]*model.WikiPage, 0)
	for _, page := range m.pages {
		if page.SpaceID == spaceID && page.IsTemplate {
			if category != "" && page.TemplateCategory != category {
				continue
			}
			templateSource := "custom"
			if page.IsSystem {
				templateSource = "system"
			}
			if source != "" && source != templateSource {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(page.Title+" "+page.TemplateCategory), strings.ToLower(query)) {
				continue
			}
			cloned := *page
			out = append(out, &cloned)
		}
	}
	return out, nil
}
func (m *mockWikiHandlerService) AddFavorite(ctx echo.Context, pageID, userID uuid.UUID) error {
	_ = ctx
	m.favorites = append(m.favorites, &model.PageFavorite{PageID: pageID, UserID: userID, CreatedAt: time.Now().UTC()})
	return nil
}
func (m *mockWikiHandlerService) RemoveFavorite(ctx echo.Context, pageID, userID uuid.UUID) error {
	_ = ctx
	next := make([]*model.PageFavorite, 0, len(m.favorites))
	for _, favorite := range m.favorites {
		if favorite.PageID == pageID && favorite.UserID == userID {
			continue
		}
		next = append(next, favorite)
	}
	m.favorites = next
	return nil
}
func (m *mockWikiHandlerService) ListFavorites(ctx echo.Context, userID uuid.UUID) ([]*model.PageFavorite, error) {
	_ = ctx
	out := make([]*model.PageFavorite, 0)
	for _, favorite := range m.favorites {
		if favorite.UserID == userID {
			cloned := *favorite
			out = append(out, &cloned)
		}
	}
	return out, nil
}
func (m *mockWikiHandlerService) SetPinned(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, pinned bool, updatedBy *uuid.UUID) error {
	_ = ctx
	_ = projectID
	_ = updatedBy
	page, ok := m.pages[pageID]
	if !ok {
		return service.ErrWikiPageNotFound
	}
	page.IsPinned = pinned
	return nil
}
func (m *mockWikiHandlerService) TouchRecentAccess(ctx echo.Context, pageID, userID uuid.UUID) error {
	_ = ctx
	m.recent = append(m.recent, &model.PageRecentAccess{PageID: pageID, UserID: userID, AccessedAt: time.Now().UTC()})
	return nil
}
func (m *mockWikiHandlerService) ListRecentAccess(ctx echo.Context, userID uuid.UUID, limit int) ([]*model.PageRecentAccess, error) {
	_ = ctx
	out := make([]*model.PageRecentAccess, 0)
	for _, access := range m.recent {
		if access.UserID == userID {
			cloned := *access
			out = append(out, &cloned)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func newWikiTestContext(method, target, body string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder, uuid.UUID, uuid.UUID) {
	e := echo.New()
	e.Validator = &wikiTestValidator{validator: validator.New()}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	projectID := uuid.New()
	userID := uuid.New()
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	return e, c, rec, projectID, userID
}

func TestWikiHandlerCRUDFlow(t *testing.T) {
	_, c, rec, projectID, userID := newWikiTestContext(http.MethodPost, "/api/v1/projects/id/wiki/pages", `{"title":"Docs Root","content":"[]"}`)
	space := &model.WikiSpace{ID: uuid.New(), ProjectID: projectID, CreatedAt: time.Now().UTC()}
	svc := &mockWikiHandlerService{space: space, pages: map[uuid.UUID]*model.WikiPage{}, versions: map[uuid.UUID]*model.PageVersion{}, comments: map[uuid.UUID]*model.PageComment{}}
	h := &WikiHandler{service: svc}

	if err := h.CreatePage(c); err != nil {
		t.Fatalf("CreatePage() error = %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreatePage() status = %d", rec.Code)
	}

	var created model.WikiPageDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created page: %v", err)
	}
	if created.Title != "Docs Root" {
		t.Fatalf("created title = %q", created.Title)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/id/wiki/pages/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	getCtx := c.Echo().NewContext(getReq, getRec)
	getCtx.SetParamNames("id")
	getCtx.SetParamValues(created.ID)
	getCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	getCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.GetPage(getCtx); err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("GetPage() status = %d", getRec.Code)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/projects/id/wiki/pages/"+created.ID, strings.NewReader(`{"title":"Docs Root Updated","content":"[]","contentText":"updated","expectedUpdatedAt":"`+created.UpdatedAt+`"}`))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateRec := httptest.NewRecorder()
	updateCtx := c.Echo().NewContext(updateReq, updateRec)
	updateCtx.SetParamNames("id")
	updateCtx.SetParamValues(created.ID)
	updateCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	updateCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.UpdatePage(updateCtx); err != nil {
		t.Fatalf("UpdatePage() error = %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("UpdatePage() status = %d", updateRec.Code)
	}
}

func TestWikiHandlerListVersionsCommentsAndTemplates(t *testing.T) {
	_, c, _, projectID, userID := newWikiTestContext(http.MethodGet, "/", "")
	space := &model.WikiSpace{ID: uuid.New(), ProjectID: projectID, CreatedAt: time.Now().UTC()}
	pageID := uuid.New()
	versionID := uuid.New()
	commentID := uuid.New()
	svc := &mockWikiHandlerService{
		space: space,
		pages: map[uuid.UUID]*model.WikiPage{
			pageID: {ID: pageID, SpaceID: space.ID, Title: "Page", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
		versions: map[uuid.UUID]*model.PageVersion{
			versionID: {ID: versionID, PageID: pageID, VersionNumber: 1, Name: "Initial", Content: "[]", CreatedAt: time.Now().UTC()},
		},
		comments: map[uuid.UUID]*model.PageComment{
			commentID: {ID: commentID, PageID: pageID, Body: "note", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
		favorites: []*model.PageFavorite{{PageID: pageID, UserID: userID, CreatedAt: time.Now().UTC()}},
		recent:    []*model.PageRecentAccess{{PageID: pageID, UserID: userID, AccessedAt: time.Now().UTC()}},
	}
	templateID := uuid.New()
	svc.pages[templateID] = &model.WikiPage{ID: templateID, SpaceID: space.ID, Title: "PRD", IsTemplate: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	h := &WikiHandler{service: svc}

	c.SetParamNames("id")
	c.SetParamValues(pageID.String())
	rec := httptest.NewRecorder()
	c.Response().Writer = rec
	if err := h.ListVersions(c); err != nil {
		t.Fatalf("ListVersions() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ListVersions() status = %d", rec.Code)
	}

	commentsReq := httptest.NewRequest(http.MethodGet, "/", nil)
	commentsRec := httptest.NewRecorder()
	commentsCtx := c.Echo().NewContext(commentsReq, commentsRec)
	commentsCtx.SetParamNames("id")
	commentsCtx.SetParamValues(pageID.String())
	commentsCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	commentsCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.ListComments(commentsCtx); err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if commentsRec.Code != http.StatusOK {
		t.Fatalf("ListComments() status = %d", commentsRec.Code)
	}

	templateReq := httptest.NewRequest(http.MethodGet, "/", nil)
	templateRec := httptest.NewRecorder()
	templateCtx := c.Echo().NewContext(templateReq, templateRec)
	templateCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	templateCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.ListTemplates(templateCtx); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	if templateRec.Code != http.StatusOK {
		t.Fatalf("ListTemplates() status = %d", templateRec.Code)
	}
}

func TestWikiHandlerCreateTemplateAndProtectBuiltInTemplates(t *testing.T) {
	_, c, _, projectID, userID := newWikiTestContext(http.MethodPost, "/", "")
	space := &model.WikiSpace{ID: uuid.New(), ProjectID: projectID, CreatedAt: time.Now().UTC()}
	systemTemplateID := uuid.New()
	svc := &mockWikiHandlerService{
		space: space,
		pages: map[uuid.UUID]*model.WikiPage{
			systemTemplateID: {
				ID:               systemTemplateID,
				SpaceID:          space.ID,
				Title:            "PRD",
				IsTemplate:       true,
				TemplateCategory: "prd",
				IsSystem:         true,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
			},
		},
		versions: map[uuid.UUID]*model.PageVersion{},
		comments: map[uuid.UUID]*model.PageComment{},
	}
	h := &WikiHandler{service: svc}

	createReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"title":"Runbook Template","category":"runbook","content":"[]"}`))
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	createCtx := c.Echo().NewContext(createReq, createRec)
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	createCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.CreateTemplate(createCtx); err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("CreateTemplate() status = %d", createRec.Code)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"title":"Mutated","content":"[]","contentText":"","expectedUpdatedAt":"2026-04-15T20:00:00Z","templateCategory":"custom"}`))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateRec := httptest.NewRecorder()
	updateCtx := c.Echo().NewContext(updateReq, updateRec)
	updateCtx.SetParamNames("id")
	updateCtx.SetParamValues(systemTemplateID.String())
	updateCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	updateCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.UpdatePage(updateCtx); err != nil {
		t.Fatalf("UpdatePage() error = %v", err)
	}
	if updateRec.Code != http.StatusForbidden {
		t.Fatalf("UpdatePage() status = %d, want %d", updateRec.Code, http.StatusForbidden)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/", nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := c.Echo().NewContext(deleteReq, deleteRec)
	deleteCtx.SetParamNames("id")
	deleteCtx.SetParamValues(systemTemplateID.String())
	deleteCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	deleteCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.DeletePage(deleteCtx); err != nil {
		t.Fatalf("DeletePage() error = %v", err)
	}
	if deleteRec.Code != http.StatusForbidden {
		t.Fatalf("DeletePage() status = %d, want %d", deleteRec.Code, http.StatusForbidden)
	}
}

func TestWikiHandlerFavoriteAndPinEndpoints(t *testing.T) {
	_, c, _, projectID, userID := newWikiTestContext(http.MethodPut, "/", "")
	space := &model.WikiSpace{ID: uuid.New(), ProjectID: projectID, CreatedAt: time.Now().UTC()}
	pageID := uuid.New()
	svc := &mockWikiHandlerService{
		space: space,
		pages: map[uuid.UUID]*model.WikiPage{
			pageID: {ID: pageID, SpaceID: space.ID, Title: "Page", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		},
	}
	h := &WikiHandler{service: svc}

	favoriteReq := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"favorite":true}`))
	favoriteReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	favoriteRec := httptest.NewRecorder()
	favoriteCtx := c.Echo().NewContext(favoriteReq, favoriteRec)
	favoriteCtx.SetParamNames("id")
	favoriteCtx.SetParamValues(pageID.String())
	favoriteCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	favoriteCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.ToggleFavorite(favoriteCtx); err != nil {
		t.Fatalf("ToggleFavorite() error = %v", err)
	}
	if favoriteRec.Code != http.StatusOK || len(svc.favorites) != 1 {
		t.Fatalf("ToggleFavorite() status=%d favorites=%d", favoriteRec.Code, len(svc.favorites))
	}

	pinReq := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"pinned":true}`))
	pinReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	pinRec := httptest.NewRecorder()
	pinCtx := c.Echo().NewContext(pinReq, pinRec)
	pinCtx.SetParamNames("id")
	pinCtx.SetParamValues(pageID.String())
	pinCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	pinCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.TogglePinned(pinCtx); err != nil {
		t.Fatalf("TogglePinned() error = %v", err)
	}
	if pinRec.Code != http.StatusOK || !svc.pages[pageID].IsPinned {
		t.Fatalf("TogglePinned() status=%d pinned=%v", pinRec.Code, svc.pages[pageID].IsPinned)
	}
}

func TestWikiHandlerGetPageContext(t *testing.T) {
	_, c, _, projectID, userID := newWikiTestContext(http.MethodGet, "/", "")
	space := &model.WikiSpace{ID: uuid.New(), ProjectID: projectID, CreatedAt: time.Now().UTC()}
	pageID := uuid.New()
	svc := &mockWikiHandlerService{
		space: space,
		pages: map[uuid.UUID]*model.WikiPage{
			pageID: {
				ID:        pageID,
				SpaceID:   space.ID,
				Title:     "Runbook",
				Path:      "/runbook",
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			},
		},
	}
	h := &WikiHandler{service: svc}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wiki/pages/"+pageID.String(), nil)
	rec := httptest.NewRecorder()
	ctx := c.Echo().NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(pageID.String())
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)
	ctx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})

	if err := h.GetPageContext(ctx); err != nil {
		t.Fatalf("GetPageContext() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("GetPageContext() status = %d", rec.Code)
	}

	var payload model.WikiPageContextDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal page context: %v", err)
	}
	if payload.ProjectID != projectID.String() {
		t.Fatalf("ProjectID = %q, want %q", payload.ProjectID, projectID.String())
	}
	if payload.Page.ID != pageID.String() {
		t.Fatalf("Page.ID = %q, want %q", payload.Page.ID, pageID.String())
	}
}
