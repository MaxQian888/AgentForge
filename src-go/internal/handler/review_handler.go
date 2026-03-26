package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

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
	ListAll(ctx context.Context, status, riskLevel string, limit int) ([]*model.Review, error)
	IngestCIResult(ctx context.Context, req *model.CIReviewRequest) (*model.Review, error)
	RequestHumanApproval(ctx context.Context, id uuid.UUID) error
	RouteFixRequest(ctx context.Context, id uuid.UUID) error
}

type ReviewAggregationService interface {
	MarkFalsePositive(ctx context.Context, reviewID uuid.UUID, findingIndex int, reason string) error
}

type ReviewHandler struct {
	service     ReviewService
	aggregation ReviewAggregationService
}

func NewReviewHandler(svc ReviewService) *ReviewHandler {
	return &ReviewHandler{service: svc}
}

func (h *ReviewHandler) WithAggregationService(agg ReviewAggregationService) *ReviewHandler {
	h.aggregation = agg
	return h
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

func (h *ReviewHandler) ListAll(c echo.Context) error {
	status := c.QueryParam("status")
	riskLevel := c.QueryParam("riskLevel")
	limit := 50
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	reviews, err := h.service.ListAll(c.Request().Context(), status, riskLevel, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
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

func (h *ReviewHandler) IngestCIResult(c echo.Context) error {
	req := new(model.CIReviewRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	review, err := h.service.IngestCIResult(c.Request().Context(), req)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, review.ToDTO())
}

func (h *ReviewHandler) RequestChanges(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid review ID"})
	}

	review, err := h.service.Complete(c.Request().Context(), id, &model.CompleteReviewRequest{
		RiskLevel:      model.ReviewRiskLevelMedium,
		Recommendation: model.ReviewRecommendationRequestChanges,
	})
	if err != nil {
		return h.handleServiceError(c, err)
	}

	if routeErr := h.service.RouteFixRequest(c.Request().Context(), id); routeErr != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: routeErr.Error()})
	}

	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) MarkFalsePositive(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid review ID"})
	}

	if h.aggregation == nil {
		return c.JSON(http.StatusNotImplemented, model.ErrorResponse{Message: "aggregation service not available"})
	}

	req := new(model.MarkFalsePositiveRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	if err := h.aggregation.MarkFalsePositive(c.Request().Context(), id, req.FindingIndex, req.Reason); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
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
