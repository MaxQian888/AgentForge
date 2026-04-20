package handler

import (
	"context"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type entityLinkHandlerService interface {
	CreateLink(ctx context.Context, input *service.CreateEntityLinkInput) (*model.EntityLink, error)
	DeleteLink(ctx context.Context, linkID uuid.UUID) error
	ListLinksForEntity(ctx context.Context, projectID uuid.UUID, entityType string, entityID uuid.UUID) ([]*model.EntityLink, error)
}

type EntityLinkHandler struct {
	service entityLinkHandlerService
}

func NewEntityLinkHandler(service entityLinkHandlerService) *EntityLinkHandler {
	return &EntityLinkHandler{service: service}
}

func (h *EntityLinkHandler) Create(c echo.Context) error {
	req := new(model.CreateEntityLinkRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidSourceID)
	}
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTargetID)
	}
	createdBy := currentUserID(c)
	if createdBy == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgMissingUserContext)
	}
	link, err := h.service.CreateLink(c.Request().Context(), &service.CreateEntityLinkInput{
		ProjectID:     appMiddleware.GetProjectID(c),
		SourceType:    req.SourceType,
		SourceID:      sourceID,
		TargetType:    req.TargetType,
		TargetID:      targetID,
		LinkType:      req.LinkType,
		AnchorBlockID: req.AnchorBlockID,
		CreatedBy:     *createdBy,
	})
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateEntityLink)
	}
	return c.JSON(http.StatusCreated, link.ToDTO())
}

func (h *EntityLinkHandler) List(c echo.Context) error {
	sourceType := c.QueryParam("source_type")
	sourceIDRaw := c.QueryParam("source_id")
	if sourceType == "" || sourceIDRaw == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgSourceTypeAndIDRequired)
	}
	sourceID, err := uuid.Parse(sourceIDRaw)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidSourceID)
	}
	links, err := h.service.ListLinksForEntity(c.Request().Context(), appMiddleware.GetProjectID(c), sourceType, sourceID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListEntityLinks)
	}
	payload := make([]model.EntityLinkDTO, 0, len(links))
	for _, link := range links {
		payload = append(payload, link.ToDTO())
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *EntityLinkHandler) Delete(c echo.Context) error {
	linkID, err := uuid.Parse(c.Param("linkId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidLinkID)
	}
	if err := h.service.DeleteLink(c.Request().Context(), linkID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteEntityLink)
	}
	return c.NoContent(http.StatusNoContent)
}
