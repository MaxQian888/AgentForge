package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
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
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	sourceID, err := uuid.Parse(req.SourceID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid source ID"})
	}
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid target ID"})
	}
	createdBy := currentUserID(c)
	if createdBy == nil {
		return c.JSON(http.StatusUnauthorized, model.ErrorResponse{Message: "missing user context"})
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
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create entity link"})
	}
	return c.JSON(http.StatusCreated, link.ToDTO())
}

func (h *EntityLinkHandler) List(c echo.Context) error {
	sourceType := c.QueryParam("source_type")
	sourceIDRaw := c.QueryParam("source_id")
	if sourceType == "" || sourceIDRaw == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "source_type and source_id are required"})
	}
	sourceID, err := uuid.Parse(sourceIDRaw)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid source ID"})
	}
	links, err := h.service.ListLinksForEntity(c.Request().Context(), appMiddleware.GetProjectID(c), sourceType, sourceID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list entity links"})
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
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid link ID"})
	}
	if err := h.service.DeleteLink(c.Request().Context(), linkID); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to delete entity link"})
	}
	return c.NoContent(http.StatusNoContent)
}
