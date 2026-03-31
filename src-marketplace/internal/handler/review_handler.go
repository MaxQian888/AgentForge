package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/agentforge/marketplace/internal/repository"
	"github.com/agentforge/marketplace/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ReviewHandler handles marketplace item review endpoints.
type ReviewHandler struct {
	svc *service.MarketplaceService
}

// NewReviewHandler creates a new ReviewHandler.
func NewReviewHandler(svc *service.MarketplaceService) *ReviewHandler {
	return &ReviewHandler{svc: svc}
}

// List handles GET /api/v1/items/:id/reviews
func (h *ReviewHandler) List(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}

	limit := 20
	offset := 0
	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	reviews, err := h.svc.GetReviews(c.Request().Context(), id, limit, offset)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, reviews)
}

// Upsert handles POST /api/v1/items/:id/reviews
func (h *ReviewHandler) Upsert(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}

	var req model.CreateReviewRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return localizedError(c, http.StatusUnprocessableEntity, err.Error())
	}

	userID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, "Unauthorized")
	}
	userName := claimsUserName(c)

	review, err := h.svc.UpsertReview(c.Request().Context(), id, userID, userName, req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, review)
}

// DeleteMine handles DELETE /api/v1/items/:id/reviews/me
func (h *ReviewHandler) DeleteMine(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}

	userID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, "Unauthorized")
	}

	if err := h.svc.DeleteReview(c.Request().Context(), id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Review not found")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.NoContent(http.StatusNoContent)
}
