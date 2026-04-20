package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ReviewService interface {
	Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error)
	Complete(ctx context.Context, id uuid.UUID, req *model.CompleteReviewRequest) (*model.Review, error)
	ApproveReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error)
	RequestChangesReview(ctx context.Context, id uuid.UUID, actor, comment string) (*model.Review, error)
	RejectReview(ctx context.Context, id uuid.UUID, actor, reason, comment string) (*model.Review, error)
	MarkFalsePositive(ctx context.Context, reviewID uuid.UUID, actor string, findingIDs []string, reason string) (*model.Review, error)
	Approve(ctx context.Context, id uuid.UUID, comment string) (*model.Review, error)
	Reject(ctx context.Context, id uuid.UUID, reason, comment string) (*model.Review, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error)
	ListAll(ctx context.Context, status, riskLevel string, limit int) ([]*model.Review, error)
	IngestCIResult(ctx context.Context, req *model.CIReviewRequest) (*model.Review, error)
	RequestHumanApproval(ctx context.Context, id uuid.UUID) error
}

type ReviewHandler struct {
	service ReviewService
}

func NewReviewHandler(svc ReviewService) *ReviewHandler {
	return &ReviewHandler{service: svc}
}

func (h *ReviewHandler) WithAggregationService(_ any) *ReviewHandler {
	return h
}

func (h *ReviewHandler) Trigger(c echo.Context) error {
	req := new(model.TriggerReviewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidReviewID)
	}

	req := new(model.CompleteReviewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidReviewID)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidReviewID)
	}

	req := new(model.ApproveReviewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	review, err := h.service.ApproveReview(c.Request().Context(), id, resolveReviewActor(c), req.Comment)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) Reject(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidReviewID)
	}

	req := new(model.RejectReviewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	review, err := h.service.RejectReview(c.Request().Context(), id, resolveReviewActor(c), req.Reason, req.Comment)
	if err != nil {
		return h.handleServiceError(c, err)
	}
	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) IngestCIResult(c echo.Context) error {
	req := new(model.CIReviewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidReviewID)
	}

	req := new(model.RequestChangesReviewRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	review, err := h.service.RequestChangesReview(c.Request().Context(), id, resolveReviewActor(c), req.Comment)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) MarkFalsePositive(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidReviewID)
	}

	req := new(model.MarkFalsePositiveRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	review, err := h.service.MarkFalsePositive(c.Request().Context(), id, resolveReviewActor(c), req.FindingIDs, req.Reason)
	if err != nil {
		return h.handleServiceError(c, err)
	}

	return c.JSON(http.StatusOK, review.ToDTO())
}

func (h *ReviewHandler) handleServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrReviewNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgReviewNotFound)
	case errors.Is(err, service.ErrReviewTaskNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgTaskNotFound)
	case errors.Is(err, service.ErrReviewInvalidTransition):
		return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
	default:
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}
}

func resolveReviewActor(c echo.Context) string {
	if c == nil {
		return "api"
	}
	keys := []string{"user_id", "userId", "uid", "sub"}
	for _, key := range keys {
		raw := c.Get(key)
		value, ok := raw.(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "api"
}
