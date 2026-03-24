package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type TeamRuntimeService interface {
	StartTeam(ctx context.Context, input service.StartTeamInput) (*model.AgentTeam, error)
	GetSummary(ctx context.Context, teamID uuid.UUID) (*model.AgentTeamSummaryDTO, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error)
	CancelTeam(ctx context.Context, teamID uuid.UUID) error
	RetryTeam(ctx context.Context, teamID uuid.UUID) error
}

type TeamHandler struct {
	service TeamRuntimeService
}

func NewTeamHandler(service TeamRuntimeService) *TeamHandler {
	return &TeamHandler{service: service}
}

type StartTeamRequest struct {
	TaskID         string  `json:"taskId" validate:"required"`
	MemberID       string  `json:"memberId" validate:"required"`
	Name           string  `json:"name"`
	Strategy       string  `json:"strategy"`
	Runtime        string  `json:"runtime"`
	Provider       string  `json:"provider"`
	Model          string  `json:"model"`
	TotalBudgetUsd float64 `json:"totalBudgetUsd"`
}

func (h *TeamHandler) Start(c echo.Context) error {
	if h.service == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "team service unavailable"})
	}

	req := new(StartTeamRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	memberID, err := uuid.Parse(req.MemberID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid member ID"})
	}

	team, err := h.service.StartTeam(c.Request().Context(), service.StartTeamInput{
		TaskID:         taskID,
		MemberID:       memberID,
		Name:           req.Name,
		Strategy:       req.Strategy,
		Runtime:        req.Runtime,
		Provider:       req.Provider,
		Model:          req.Model,
		TotalBudgetUsd: req.TotalBudgetUsd,
	})
	if err != nil {
		switch err {
		case service.ErrTeamTaskNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case service.ErrTeamAlreadyActive:
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to start team"})
		}
	}

	summary, summaryErr := h.service.GetSummary(c.Request().Context(), team.ID)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusCreated, summary)
	}
	return c.JSON(http.StatusCreated, team.ToDTO())
}

func (h *TeamHandler) Get(c echo.Context) error {
	if h.service == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "team service unavailable"})
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid team ID"})
	}

	summary, err := h.service.GetSummary(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "team not found"})
	}
	return c.JSON(http.StatusOK, summary)
}

func (h *TeamHandler) List(c echo.Context) error {
	if h.service == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "team service unavailable"})
	}

	projectID, err := uuid.Parse(c.QueryParam("projectId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid or missing projectId query parameter"})
	}

	teams, err := h.service.ListByProject(c.Request().Context(), projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list teams"})
	}

	dtos := make([]model.AgentTeamDTO, 0, len(teams))
	for _, t := range teams {
		dtos = append(dtos, t.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *TeamHandler) Cancel(c echo.Context) error {
	if h.service == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "team service unavailable"})
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid team ID"})
	}

	if err := h.service.CancelTeam(c.Request().Context(), id); err != nil {
		switch err {
		case service.ErrTeamNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case service.ErrTeamNotActive:
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to cancel team"})
		}
	}

	summary, summaryErr := h.service.GetSummary(c.Request().Context(), id)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusOK, summary)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *TeamHandler) Retry(c echo.Context) error {
	if h.service == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "team service unavailable"})
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid team ID"})
	}

	if err := h.service.RetryTeam(c.Request().Context(), id); err != nil {
		switch err {
		case service.ErrTeamNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to retry team"})
		}
	}

	summary, summaryErr := h.service.GetSummary(c.Request().Context(), id)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusOK, summary)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "retrying"})
}
