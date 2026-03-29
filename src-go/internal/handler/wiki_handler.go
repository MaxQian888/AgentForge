package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type WikiHandler struct {
	service wikiHandlerService
}

type wikiHandlerService interface {
	GetSpaceByProjectID(ctx echo.Context, projectID uuid.UUID) (*model.WikiSpace, error)
	CreatePage(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID, title string, parentID *uuid.UUID, content string, createdBy *uuid.UUID) (*model.WikiPage, error)
	GetPageTree(ctx echo.Context, spaceID uuid.UUID) ([]*model.WikiPage, error)
	GetPage(ctx echo.Context, pageID uuid.UUID) (*model.WikiPage, error)
	GetPageContext(ctx echo.Context, pageID uuid.UUID) (*model.WikiSpace, *model.WikiPage, error)
	UpdatePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, title string, content string, contentText string, updatedBy *uuid.UUID, expectedUpdatedAt *time.Time) (*model.WikiPage, error)
	DeletePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID) error
	MovePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, newParentID *uuid.UUID, sortOrder int) (*model.WikiPage, error)
	ListVersions(ctx echo.Context, pageID uuid.UUID) ([]*model.PageVersion, error)
	CreateVersion(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, name string, createdBy *uuid.UUID) (*model.PageVersion, error)
	GetVersion(ctx echo.Context, versionID uuid.UUID) (*model.PageVersion, error)
	RestoreVersion(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, versionID uuid.UUID, updatedBy *uuid.UUID) (*model.WikiPage, *model.PageVersion, error)
	ListComments(ctx echo.Context, pageID uuid.UUID) ([]*model.PageComment, error)
	CreateComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, body string, anchorBlockID *string, parentCommentID *uuid.UUID, createdBy *uuid.UUID, mentions string) (*model.PageComment, error)
	ResolveComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) (*model.PageComment, error)
	ReopenComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) (*model.PageComment, error)
	DeleteComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) error
	SeedBuiltInTemplates(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID) ([]*model.WikiPage, error)
	CreateTemplateFromPage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, name string, category string, createdBy *uuid.UUID) (*model.WikiPage, error)
	CreatePageFromTemplate(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID, templateID uuid.UUID, parentID *uuid.UUID, title string, createdBy *uuid.UUID) (*model.WikiPage, error)
	ListTemplates(ctx echo.Context, spaceID uuid.UUID) ([]*model.WikiPage, error)
	AddFavorite(ctx echo.Context, pageID, userID uuid.UUID) error
	RemoveFavorite(ctx echo.Context, pageID, userID uuid.UUID) error
	ListFavorites(ctx echo.Context, userID uuid.UUID) ([]*model.PageFavorite, error)
	SetPinned(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, pinned bool, updatedBy *uuid.UUID) error
	TouchRecentAccess(ctx echo.Context, pageID, userID uuid.UUID) error
	ListRecentAccess(ctx echo.Context, userID uuid.UUID, limit int) ([]*model.PageRecentAccess, error)
}

// Note: use context.Context-compatible methods from service.WikiService via adapters below.

type wikiHandlerServiceAdapter struct {
	svc *service.WikiService
}

func NewWikiHandler(svc *service.WikiService) *WikiHandler {
	return &WikiHandler{service: &wikiHandlerServiceAdapter{svc: svc}}
}

func (a *wikiHandlerServiceAdapter) GetSpaceByProjectID(ctx echo.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	return a.svc.GetSpaceByProjectID(ctx.Request().Context(), projectID)
}
func (a *wikiHandlerServiceAdapter) CreatePage(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID, title string, parentID *uuid.UUID, content string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	return a.svc.CreatePage(ctx.Request().Context(), projectID, spaceID, title, parentID, content, createdBy)
}
func (a *wikiHandlerServiceAdapter) GetPageTree(ctx echo.Context, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	return a.svc.GetPageTree(ctx.Request().Context(), spaceID)
}
func (a *wikiHandlerServiceAdapter) GetPage(ctx echo.Context, pageID uuid.UUID) (*model.WikiPage, error) {
	return a.svc.GetPage(ctx.Request().Context(), pageID)
}
func (a *wikiHandlerServiceAdapter) GetPageContext(ctx echo.Context, pageID uuid.UUID) (*model.WikiSpace, *model.WikiPage, error) {
	return a.svc.GetPageContext(ctx.Request().Context(), pageID)
}
func (a *wikiHandlerServiceAdapter) UpdatePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, title string, content string, contentText string, updatedBy *uuid.UUID, expectedUpdatedAt *time.Time) (*model.WikiPage, error) {
	return a.svc.UpdatePage(ctx.Request().Context(), projectID, pageID, title, content, contentText, updatedBy, expectedUpdatedAt)
}
func (a *wikiHandlerServiceAdapter) DeletePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID) error {
	return a.svc.DeletePage(ctx.Request().Context(), projectID, pageID)
}
func (a *wikiHandlerServiceAdapter) MovePage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, newParentID *uuid.UUID, sortOrder int) (*model.WikiPage, error) {
	return a.svc.MovePage(ctx.Request().Context(), projectID, pageID, newParentID, sortOrder)
}
func (a *wikiHandlerServiceAdapter) ListVersions(ctx echo.Context, pageID uuid.UUID) ([]*model.PageVersion, error) {
	return a.svc.ListVersions(ctx.Request().Context(), pageID)
}
func (a *wikiHandlerServiceAdapter) CreateVersion(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, name string, createdBy *uuid.UUID) (*model.PageVersion, error) {
	return a.svc.CreateVersion(ctx.Request().Context(), projectID, pageID, name, createdBy)
}
func (a *wikiHandlerServiceAdapter) GetVersion(ctx echo.Context, versionID uuid.UUID) (*model.PageVersion, error) {
	return a.svc.GetVersion(ctx.Request().Context(), versionID)
}
func (a *wikiHandlerServiceAdapter) RestoreVersion(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, versionID uuid.UUID, updatedBy *uuid.UUID) (*model.WikiPage, *model.PageVersion, error) {
	return a.svc.RestoreVersion(ctx.Request().Context(), projectID, pageID, versionID, updatedBy)
}
func (a *wikiHandlerServiceAdapter) ListComments(ctx echo.Context, pageID uuid.UUID) ([]*model.PageComment, error) {
	return a.svc.ListComments(ctx.Request().Context(), pageID)
}
func (a *wikiHandlerServiceAdapter) CreateComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, body string, anchorBlockID *string, parentCommentID *uuid.UUID, createdBy *uuid.UUID, mentions string) (*model.PageComment, error) {
	return a.svc.CreateComment(ctx.Request().Context(), projectID, pageID, body, anchorBlockID, parentCommentID, createdBy, mentions)
}
func (a *wikiHandlerServiceAdapter) ResolveComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) (*model.PageComment, error) {
	return a.svc.ResolveComment(ctx.Request().Context(), projectID, pageID, commentID)
}
func (a *wikiHandlerServiceAdapter) ReopenComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) (*model.PageComment, error) {
	return a.svc.ReopenComment(ctx.Request().Context(), projectID, pageID, commentID)
}
func (a *wikiHandlerServiceAdapter) DeleteComment(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, commentID uuid.UUID) error {
	return a.svc.DeleteComment(ctx.Request().Context(), projectID, pageID, commentID)
}
func (a *wikiHandlerServiceAdapter) SeedBuiltInTemplates(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	return a.svc.SeedBuiltInTemplates(ctx.Request().Context(), projectID, spaceID)
}
func (a *wikiHandlerServiceAdapter) CreateTemplateFromPage(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, name string, category string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	return a.svc.CreateTemplateFromPage(ctx.Request().Context(), projectID, pageID, name, category, createdBy)
}
func (a *wikiHandlerServiceAdapter) CreatePageFromTemplate(ctx echo.Context, projectID uuid.UUID, spaceID uuid.UUID, templateID uuid.UUID, parentID *uuid.UUID, title string, createdBy *uuid.UUID) (*model.WikiPage, error) {
	return a.svc.CreatePageFromTemplate(ctx.Request().Context(), projectID, spaceID, templateID, parentID, title, createdBy)
}
func (a *wikiHandlerServiceAdapter) ListTemplates(ctx echo.Context, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	return a.svc.ListTemplates(ctx.Request().Context(), spaceID)
}
func (a *wikiHandlerServiceAdapter) AddFavorite(ctx echo.Context, pageID, userID uuid.UUID) error {
	return a.svc.AddFavorite(ctx.Request().Context(), pageID, userID)
}
func (a *wikiHandlerServiceAdapter) RemoveFavorite(ctx echo.Context, pageID, userID uuid.UUID) error {
	return a.svc.RemoveFavorite(ctx.Request().Context(), pageID, userID)
}
func (a *wikiHandlerServiceAdapter) ListFavorites(ctx echo.Context, userID uuid.UUID) ([]*model.PageFavorite, error) {
	return a.svc.ListFavorites(ctx.Request().Context(), userID)
}
func (a *wikiHandlerServiceAdapter) SetPinned(ctx echo.Context, projectID uuid.UUID, pageID uuid.UUID, pinned bool, updatedBy *uuid.UUID) error {
	return a.svc.SetPinned(ctx.Request().Context(), projectID, pageID, pinned, updatedBy)
}
func (a *wikiHandlerServiceAdapter) TouchRecentAccess(ctx echo.Context, pageID, userID uuid.UUID) error {
	return a.svc.TouchRecentAccess(ctx.Request().Context(), pageID, userID)
}
func (a *wikiHandlerServiceAdapter) ListRecentAccess(ctx echo.Context, userID uuid.UUID, limit int) ([]*model.PageRecentAccess, error) {
	return a.svc.ListRecentAccess(ctx.Request().Context(), userID, limit)
}

func (h *WikiHandler) ListPages(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	space, err := h.service.GetSpaceByProjectID(c, projectID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWikiSpaceNotFound)
	}
	pages, err := h.service.GetPageTree(c, space.ID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListWikiPages)
	}
	return c.JSON(http.StatusOK, buildWikiTree(pages))
}

func (h *WikiHandler) CreatePage(c echo.Context) error {
	req := new(model.CreateWikiPageRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	projectID := appMiddleware.GetProjectID(c)
	space, err := h.service.GetSpaceByProjectID(c, projectID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWikiSpaceNotFound)
	}
	parentID, err := parseOptionalUUID(req.ParentID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidParentID)
	}
	userID := currentUserID(c)
	page, err := h.service.CreatePage(c, projectID, space.ID, req.Title, parentID, req.Content, userID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateWikiPage)
	}
	return c.JSON(http.StatusCreated, page.ToDTO())
}

func (h *WikiHandler) GetPage(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	page, err := h.service.GetPage(c, pageID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWikiPageNotFound)
	}
	userID := currentUserID(c)
	if userID != nil {
		_ = h.service.TouchRecentAccess(c, pageID, *userID)
	}
	return c.JSON(http.StatusOK, page.ToDTO())
}

func (h *WikiHandler) GetPageContext(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	space, page, err := h.service.GetPageContext(c, pageID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWikiPageNotFound)
	}
	userID := currentUserID(c)
	if userID != nil {
		_ = h.service.TouchRecentAccess(c, pageID, *userID)
	}
	return c.JSON(http.StatusOK, model.WikiPageContextDTO{
		ProjectID: space.ProjectID.String(),
		Page:      page.ToDTO(),
	})
}

func (h *WikiHandler) UpdatePage(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.UpdateWikiPageRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	expectedUpdatedAt, err := parseOptionalTime(req.ExpectedUpdatedAt)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidExpectedUpdatedAt)
	}
	page, err := h.service.UpdatePage(c, appMiddleware.GetProjectID(c), pageID, req.Title, req.Content, req.ContentText, currentUserID(c), expectedUpdatedAt)
	if err != nil {
		if errors.Is(err, service.ErrWikiPageConflict) {
			return localizedError(c, http.StatusConflict, i18n.MsgWikiPageHasNewerChanges)
		}
		if errors.Is(err, service.ErrWikiPageNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgWikiPageNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateWikiPage)
	}
	return c.JSON(http.StatusOK, page.ToDTO())
}

func (h *WikiHandler) DeletePage(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	if err := h.service.DeletePage(c, appMiddleware.GetProjectID(c), pageID); err != nil {
		if errors.Is(err, service.ErrWikiPageNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgWikiPageNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteWikiPage)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "wiki page deleted"})
}

func (h *WikiHandler) MovePage(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.MoveWikiPageRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	parentID, err := parseOptionalUUID(req.ParentID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidParentID)
	}
	page, err := h.service.MovePage(c, appMiddleware.GetProjectID(c), pageID, parentID, req.SortOrder)
	if err != nil {
		if errors.Is(err, service.ErrWikiCircularMove) {
			return localizedError(c, http.StatusBadRequest, i18n.MsgCannotMoveIntoDescendant)
		}
		if errors.Is(err, service.ErrWikiPageNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgWikiPageNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToMoveWikiPage)
	}
	return c.JSON(http.StatusOK, page.ToDTO())
}

func (h *WikiHandler) ListVersions(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	versions, err := h.service.ListVersions(c, pageID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListPageVersions)
	}
	payload := make([]model.PageVersionDTO, 0, len(versions))
	for _, version := range versions {
		payload = append(payload, version.ToDTO())
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *WikiHandler) CreateVersion(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.CreatePageVersionRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	version, err := h.service.CreateVersion(c, appMiddleware.GetProjectID(c), pageID, req.Name, currentUserID(c))
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreatePageVersion)
	}
	return c.JSON(http.StatusCreated, version.ToDTO())
}

func (h *WikiHandler) GetVersion(c echo.Context) error {
	versionID, err := uuid.Parse(c.Param("vid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidVersionID)
	}
	version, err := h.service.GetVersion(c, versionID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgPageVersionNotFound)
	}
	return c.JSON(http.StatusOK, version.ToDTO())
}

func (h *WikiHandler) RestoreVersion(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	versionID, err := uuid.Parse(c.Param("vid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidVersionID)
	}
	page, version, err := h.service.RestoreVersion(c, appMiddleware.GetProjectID(c), pageID, versionID, currentUserID(c))
	if err != nil {
		if errors.Is(err, service.ErrPageVersionNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgPageVersionNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRestorePageVersion)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"page":    page.ToDTO(),
		"version": version.ToDTO(),
	})
}

func (h *WikiHandler) ListComments(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	comments, err := h.service.ListComments(c, pageID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListPageComments)
	}
	payload := make([]model.PageCommentDTO, 0, len(comments))
	for _, comment := range comments {
		payload = append(payload, comment.ToDTO())
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *WikiHandler) CreateComment(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.CreatePageCommentRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	parentCommentID, err := parseOptionalUUID(req.ParentCommentID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidParentCommentID)
	}
	comment, err := h.service.CreateComment(c, appMiddleware.GetProjectID(c), pageID, req.Body, req.AnchorBlockID, parentCommentID, currentUserID(c), req.Mentions)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreatePageComment)
	}
	return c.JSON(http.StatusCreated, comment.ToDTO())
}

func (h *WikiHandler) UpdateComment(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	commentID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidCommentID)
	}
	req := new(model.UpdatePageCommentRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Resolved == nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgResolvedFlagRequired)
	}
	var comment *model.PageComment
	if *req.Resolved {
		comment, err = h.service.ResolveComment(c, appMiddleware.GetProjectID(c), pageID, commentID)
	} else {
		comment, err = h.service.ReopenComment(c, appMiddleware.GetProjectID(c), pageID, commentID)
	}
	if err != nil {
		if errors.Is(err, service.ErrPageCommentNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgPageCommentNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdatePageComment)
	}
	return c.JSON(http.StatusOK, comment.ToDTO())
}

func (h *WikiHandler) DeleteComment(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	commentID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidCommentID)
	}
	if err := h.service.DeleteComment(c, appMiddleware.GetProjectID(c), pageID, commentID); err != nil {
		if errors.Is(err, service.ErrPageCommentNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgPageCommentNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeletePageComment)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "page comment deleted"})
}

func (h *WikiHandler) ListTemplates(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	space, err := h.service.GetSpaceByProjectID(c, projectID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWikiSpaceNotFound)
	}
	templates, err := h.service.ListTemplates(c, space.ID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListWikiTemplates)
	}
	payload := make([]model.WikiPageDTO, 0, len(templates))
	for _, template := range templates {
		payload = append(payload, template.ToDTO())
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *WikiHandler) CreateTemplateFromPage(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.CreateTemplateFromPageRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	template, err := h.service.CreateTemplateFromPage(c, appMiddleware.GetProjectID(c), pageID, req.Name, req.Category, currentUserID(c))
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateTemplate)
	}
	return c.JSON(http.StatusCreated, template.ToDTO())
}

func (h *WikiHandler) CreatePageFromTemplate(c echo.Context) error {
	req := new(model.CreatePageFromTemplateRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	projectID := appMiddleware.GetProjectID(c)
	space, err := h.service.GetSpaceByProjectID(c, projectID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgWikiSpaceNotFound)
	}
	templateID, err := uuid.Parse(req.TemplateID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTemplateID)
	}
	parentID, err := parseOptionalUUID(req.ParentID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidParentID)
	}
	page, err := h.service.CreatePageFromTemplate(c, projectID, space.ID, templateID, parentID, req.Title, currentUserID(c))
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreatePageFromTemplate)
	}
	return c.JSON(http.StatusCreated, page.ToDTO())
}

func (h *WikiHandler) ListFavorites(c echo.Context) error {
	userID := currentUserID(c)
	if userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgMissingUserContext)
	}
	favorites, err := h.service.ListFavorites(c, *userID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListPageFavorites)
	}
	payload := make([]model.PageFavoriteDTO, 0, len(favorites))
	for _, favorite := range favorites {
		payload = append(payload, favorite.ToDTO())
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *WikiHandler) ToggleFavorite(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.ToggleFavoriteRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	userID := currentUserID(c)
	if userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgMissingUserContext)
	}
	if req.Favorite {
		err = h.service.AddFavorite(c, pageID, *userID)
	} else {
		err = h.service.RemoveFavorite(c, pageID, *userID)
	}
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateFavoriteState)
	}
	return c.JSON(http.StatusOK, map[string]bool{"favorite": req.Favorite})
}

func (h *WikiHandler) ListRecentAccess(c echo.Context) error {
	userID := currentUserID(c)
	if userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgMissingUserContext)
	}
	accesses, err := h.service.ListRecentAccess(c, *userID, 20)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListRecentPages)
	}
	payload := make([]model.PageRecentAccessDTO, 0, len(accesses))
	for _, access := range accesses {
		payload = append(payload, access.ToDTO())
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *WikiHandler) TogglePinned(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.TogglePinnedRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := h.service.SetPinned(c, appMiddleware.GetProjectID(c), pageID, req.Pinned, currentUserID(c)); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdatePinnedState)
	}
	return c.JSON(http.StatusOK, map[string]bool{"pinned": req.Pinned})
}

func buildWikiTree(pages []*model.WikiPage) []model.WikiPageTreeNodeDTO {
	nodes := make(map[string]*model.WikiPageTreeNodeDTO, len(pages))
	rootIDs := make([]string, 0)
	for _, page := range pages {
		if page == nil {
			continue
		}
		dto := model.WikiPageTreeNodeDTO{WikiPageDTO: page.ToDTO(), Children: []model.WikiPageTreeNodeDTO{}}
		nodes[page.ID.String()] = &dto
	}
	for _, page := range pages {
		if page == nil {
			continue
		}
		node := nodes[page.ID.String()]
		if page.ParentID == nil {
			rootIDs = append(rootIDs, page.ID.String())
			continue
		}
		parent := nodes[page.ParentID.String()]
		if parent == nil {
			rootIDs = append(rootIDs, page.ID.String())
			continue
		}
		parent.Children = append(parent.Children, *node)
	}
	roots := make([]model.WikiPageTreeNodeDTO, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		if root := nodes[rootID]; root != nil {
			roots = append(roots, rebuildTreeNode(*root, nodes))
		}
	}
	return roots
}

func rebuildTreeNode(node model.WikiPageTreeNodeDTO, nodes map[string]*model.WikiPageTreeNodeDTO) model.WikiPageTreeNodeDTO {
	if len(node.Children) == 0 {
		return node
	}
	children := make([]model.WikiPageTreeNodeDTO, 0, len(node.Children))
	for _, child := range node.Children {
		if stored := nodes[child.ID]; stored != nil {
			children = append(children, rebuildTreeNode(*stored, nodes))
		} else {
			children = append(children, child)
		}
	}
	node.Children = children
	return node
}

func currentUserID(c echo.Context) *uuid.UUID {
	claims, err := appMiddleware.GetClaims(c)
	if err != nil {
		return nil
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil
	}
	return &userID
}

func parseOptionalUUID(value *string) (*uuid.UUID, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(*value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalTime(value *string) (*time.Time, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
