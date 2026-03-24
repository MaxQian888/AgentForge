package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type ReviewService interface {
	Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error)
	Complete(ctx context.Context, id uuid.UUID, req *model.CompleteReviewRequest) (*model.Review, error)
	Approve(ctx context.Context, id uuid.UUID, comment string) (*model.Review, error)
	Reject(ctx context.Context, id uuid.UUID, reason, comment string) (*model.Review, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error)
}

type ReviewHandler struct {
	service ReviewService
}

func NewReviewHandler(svc ReviewService) *ReviewHandler {
	return &ReviewHandler{service: svc}
}

func (h *ReviewHandler) Trigger(c echo.Context) error {
	req := new(model.TriggerReviewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	review, err := h.service.Trigger(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusAccepted, review.ToDTO())
}

func (h *ReviewHandler) Complete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid review ID"})
	}

	req := new(model.CompleteReviewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	review, err := h.service.Complete(c.Request().Context(), id, req)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid review ID"})
	}

	review, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) ListByTask(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}

	reviews, err := h.service.GetByTask(c.Request().Context(), taskID)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	dtos := make([]model.ReviewDTO, 0, len(reviews))
	for _, review := range reviews {
		dtos = append(dtos, review.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *ReviewHandler) Approve(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid review ID"})
	}

	req := new(model.ApproveReviewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}

	review, err := h.service.Approve(c.Request().Context(), id, req.Comment)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) Reject(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid review ID"})
	}

	req := new(model.RejectReviewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	review, err := h.service.Reject(c.Request().Context(), id, req.Reason, req.Comment)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) handleServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrReviewNotFound):
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "review not found"})
	case errors.Is(err, service.ErrReviewTaskNotFound):
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "task not found"})
	default:
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
}
