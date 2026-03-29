package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type milestoneService interface {
	CreateMilestone(ctx context.Context, milestone *model.Milestone) error
	GetMilestone(ctx context.Context, id uuid.UUID) (*model.Milestone, error)
	ListMilestones(ctx context.Context, projectID uuid.UUID) ([]*model.Milestone, error)
	UpdateMilestone(ctx context.Context, milestone *model.Milestone) error
	DeleteMilestone(ctx context.Context, id uuid.UUID) error
	GetCompletionMetrics(ctx context.Context, milestoneID uuid.UUID) (model.MilestoneMetrics, error)
}

type MilestoneHandler struct{ service milestoneService }

func NewMilestoneHandler(service milestoneService) *MilestoneHandler {
	return &MilestoneHandler{service: service}
}

func (h *MilestoneHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	milestones, err := h.service.ListMilestones(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListMilestones)
	}
	dtos := make([]model.MilestoneDTO, 0, len(milestones))
	for _, milestone := range milestones {
		metrics, metricsErr := h.service.GetCompletionMetrics(c.Request().Context(), milestone.ID)
		if metricsErr != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadMilestoneMetrics)
		}
		metricsCopy := metrics
		dtos = append(dtos, milestone.ToDTO(&metricsCopy))
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *MilestoneHandler) Create(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	req := new(model.CreateMilestoneRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	targetDate, err := parseOptionalTimeString(req.TargetDate)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTargetDate)
	}
	milestone := &model.Milestone{
		ProjectID:   projectID,
		Name:        req.Name,
		TargetDate:  targetDate,
		Status:      req.Status,
		Description: req.Description,
	}
	if milestone.Status == "" {
		milestone.Status = model.MilestoneStatusPlanned
	}
	if err := h.service.CreateMilestone(c.Request().Context(), milestone); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateMilestone)
	}
	return c.JSON(http.StatusCreated, milestone.ToDTO(nil))
}

func (h *MilestoneHandler) Update(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	milestoneID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMilestoneID)
	}
	milestone, err := h.service.GetMilestone(c.Request().Context(), milestoneID)
	if err != nil || milestone == nil || milestone.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgMilestoneNotFound)
	}
	req := new(model.UpdateMilestoneRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Name != nil {
		milestone.Name = *req.Name
	}
	if req.TargetDate != nil {
		targetDate, parseErr := parseOptionalTimeString(req.TargetDate)
		if parseErr != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTargetDate)
		}
		milestone.TargetDate = targetDate
	}
	if req.Status != nil {
		milestone.Status = *req.Status
	}
	if req.Description != nil {
		milestone.Description = *req.Description
	}
	if err := h.service.UpdateMilestone(c.Request().Context(), milestone); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateMilestone)
	}
	metrics, _ := h.service.GetCompletionMetrics(c.Request().Context(), milestone.ID)
	metricsCopy := metrics
	return c.JSON(http.StatusOK, milestone.ToDTO(&metricsCopy))
}

func (h *MilestoneHandler) Delete(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	milestoneID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMilestoneID)
	}
	milestone, err := h.service.GetMilestone(c.Request().Context(), milestoneID)
	if err != nil || milestone == nil || milestone.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgMilestoneNotFound)
	}
	if err := h.service.DeleteMilestone(c.Request().Context(), milestoneID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteMilestone)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "milestone deleted"})
}
