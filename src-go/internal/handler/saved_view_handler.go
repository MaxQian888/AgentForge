package handler

import (
	"context"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type savedViewService interface {
	CreateView(ctx context.Context, view *model.SavedView) error
	GetView(ctx context.Context, id uuid.UUID) (*model.SavedView, error)
	UpdateView(ctx context.Context, view *model.SavedView) error
	DeleteView(ctx context.Context, id uuid.UUID) error
	ListAccessibleViews(ctx context.Context, projectID uuid.UUID, userID uuid.UUID, roles []string) ([]*model.SavedView, error)
	SetDefaultView(ctx context.Context, projectID uuid.UUID, viewID uuid.UUID) error
}

type savedViewMemberLookup interface {
	GetByUserAndProject(ctx context.Context, userID, projectID uuid.UUID) (*model.Member, error)
}

type SavedViewHandler struct {
	service      savedViewService
	memberLookup savedViewMemberLookup
}

func NewSavedViewHandler(service savedViewService, memberLookup savedViewMemberLookup) *SavedViewHandler {
	return &SavedViewHandler{service: service, memberLookup: memberLookup}
}

func (h *SavedViewHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	userID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgAuthRequired)
	}
	roles := []string{}
	if h.memberLookup != nil {
		if member, memberErr := h.memberLookup.GetByUserAndProject(c.Request().Context(), *userID, projectID); memberErr == nil && member != nil && member.Role != "" {
			roles = append(roles, member.Role)
		}
	}
	views, err := h.service.ListAccessibleViews(c.Request().Context(), projectID, *userID, roles)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListSavedViews)
	}
	dtos := make([]model.SavedViewDTO, 0, len(views))
	for _, view := range views {
		dtos = append(dtos, view.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *SavedViewHandler) Create(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	userID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgAuthRequired)
	}
	req := new(model.CreateSavedViewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	ownerID, err := parseOptionalUUIDString(req.OwnerID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidOwnerID)
	}
	if ownerID == nil && len(req.SharedWith) == 0 {
		ownerID = userID
	}
	view := &model.SavedView{
		ProjectID:  projectID,
		Name:       req.Name,
		OwnerID:    ownerID,
		IsDefault:  req.IsDefault,
		SharedWith: string(req.SharedWith),
		Config:     string(req.Config),
	}
	if err := h.service.CreateView(c.Request().Context(), view); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateSavedView)
	}
	if req.IsDefault {
		_ = h.service.SetDefaultView(c.Request().Context(), projectID, view.ID)
	}
	return c.JSON(http.StatusCreated, view.ToDTO())
}

func (h *SavedViewHandler) Update(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	viewID, err := uuid.Parse(c.Param("vid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidViewID)
	}
	view, err := h.service.GetView(c.Request().Context(), viewID)
	if err != nil || view == nil || view.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgSavedViewNotFound)
	}
	req := new(model.UpdateSavedViewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Name != nil {
		view.Name = *req.Name
	}
	if req.OwnerID != nil {
		ownerID, parseErr := parseOptionalUUIDString(req.OwnerID)
		if parseErr != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidOwnerID)
		}
		view.OwnerID = ownerID
	}
	if req.IsDefault != nil {
		view.IsDefault = *req.IsDefault
	}
	if len(req.SharedWith) > 0 {
		view.SharedWith = string(req.SharedWith)
	}
	if len(req.Config) > 0 {
		view.Config = string(req.Config)
	}
	if err := h.service.UpdateView(c.Request().Context(), view); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateSavedView)
	}
	if view.IsDefault {
		_ = h.service.SetDefaultView(c.Request().Context(), projectID, view.ID)
	}
	return c.JSON(http.StatusOK, view.ToDTO())
}

func (h *SavedViewHandler) Delete(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	viewID, err := uuid.Parse(c.Param("vid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidViewID)
	}
	view, err := h.service.GetView(c.Request().Context(), viewID)
	if err != nil || view == nil || view.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgSavedViewNotFound)
	}
	if err := h.service.DeleteView(c.Request().Context(), viewID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteSavedView)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "saved view deleted"})
}

func (h *SavedViewHandler) SetDefault(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	viewID, err := uuid.Parse(c.Param("vid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidViewID)
	}
	if err := h.service.SetDefaultView(c.Request().Context(), projectID, viewID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSetDefaultView)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "default view updated"})
}
