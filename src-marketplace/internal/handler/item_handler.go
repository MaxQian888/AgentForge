package handler

import (
	"errors"
	"net/http"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/agentforge/marketplace/internal/repository"
	"github.com/agentforge/marketplace/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ItemHandler handles marketplace item endpoints.
type ItemHandler struct {
	svc *service.MarketplaceService
}

// NewItemHandler creates a new ItemHandler.
func NewItemHandler(svc *service.MarketplaceService) *ItemHandler {
	return &ItemHandler{svc: svc}
}

// List handles GET /api/v1/items
func (h *ItemHandler) List(c echo.Context) error {
	var q model.ListItemsQuery
	if err := c.Bind(&q); err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid query parameters")
	}
	resp, err := h.svc.ListItems(c.Request().Context(), q)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, resp)
}

// Featured handles GET /api/v1/items/featured
func (h *ItemHandler) Featured(c echo.Context) error {
	items, err := h.svc.GetFeatured(c.Request().Context())
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, items)
}

// Search handles GET /api/v1/items/search
func (h *ItemHandler) Search(c echo.Context) error {
	q := c.QueryParam("q")
	if q == "" {
		return localizedError(c, http.StatusBadRequest, "Query parameter 'q' is required")
	}
	items, err := h.svc.Search(c.Request().Context(), q)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, items)
}

// Get handles GET /api/v1/items/:id
func (h *ItemHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	item, err := h.svc.GetItem(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, item)
}

// Publish handles POST /api/v1/items
func (h *ItemHandler) Publish(c echo.Context) error {
	var req model.CreateItemRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return localizedError(c, http.StatusUnprocessableEntity, err.Error())
	}
	authorID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, "Unauthorized")
	}
	authorName := claimsUserName(c)

	item, err := h.svc.PublishItem(c.Request().Context(), authorID, authorName, req)
	if err != nil {
		if errors.Is(err, service.ErrSlugTaken) {
			return localizedError(c, http.StatusConflict, "A item with this slug and type already exists")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusCreated, item)
}

// UpdateMeta handles PATCH /api/v1/items/:id
func (h *ItemHandler) UpdateMeta(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	var req model.UpdateItemRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid request body")
	}
	requesterID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, "Unauthorized")
	}

	item, err := h.svc.UpdateItem(c.Request().Context(), id, requesterID, req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		if errors.Is(err, service.ErrNotItemOwner) {
			return localizedError(c, http.StatusForbidden, "You are not the owner of this item")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, item)
}

// Delete handles DELETE /api/v1/items/:id
func (h *ItemHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	requesterID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, "Unauthorized")
	}

	if err := h.svc.DeleteItem(c.Request().Context(), id, requesterID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		if errors.Is(err, service.ErrNotItemOwner) {
			return localizedError(c, http.StatusForbidden, "You are not the owner of this item")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.NoContent(http.StatusNoContent)
}
