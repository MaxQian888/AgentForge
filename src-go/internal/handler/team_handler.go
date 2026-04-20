package handler

import (
	"context"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type TeamRuntimeService interface {
	StartTeam(ctx context.Context, input service.StartTeamInput) (*model.AgentTeam, error)
	GetSummary(ctx context.Context, teamID uuid.UUID) (*model.AgentTeamSummaryDTO, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeam, error)
	ListSummaries(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeamSummaryDTO, error)
	CancelTeam(ctx context.Context, teamID uuid.UUID) error
	RetryTeam(ctx context.Context, teamID uuid.UUID) error
	DeleteTeam(ctx context.Context, teamID uuid.UUID) error
	UpdateTeam(ctx context.Context, teamID uuid.UUID, req *model.UpdateTeamRequest) (*model.AgentTeam, error)
	ListArtifacts(ctx context.Context, teamID uuid.UUID) ([]model.TeamArtifactDTO, error)
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
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	req := new(StartTeamRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	taskID, err := uuid.Parse(req.TaskID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	memberID, err := uuid.Parse(req.MemberID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemberID)
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
			return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToStartTeam)
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
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTeamID)
	}

	summary, err := h.service.GetSummary(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgTeamNotFound)
	}
	return c.JSON(http.StatusOK, summary)
}

func (h *TeamHandler) List(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	projectID, err := uuid.Parse(c.QueryParam("projectId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidOrMissingProjectID)
	}

	status := c.QueryParam("status")
	summaries, err := h.service.ListSummaries(c.Request().Context(), projectID, status)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListTeams)
	}

	return c.JSON(http.StatusOK, summaries)
}

func (h *TeamHandler) Cancel(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTeamID)
	}

	if err := h.service.CancelTeam(c.Request().Context(), id); err != nil {
		switch err {
		case service.ErrTeamNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case service.ErrTeamNotActive:
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		default:
			return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToCancelTeam)
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
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTeamID)
	}

	if err := h.service.RetryTeam(c.Request().Context(), id); err != nil {
		switch err {
		case service.ErrTeamNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		default:
			return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToRetryTeam)
		}
	}

	summary, summaryErr := h.service.GetSummary(c.Request().Context(), id)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusOK, summary)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "retrying"})
}

func (h *TeamHandler) Delete(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTeamID)
	}

	if err := h.service.DeleteTeam(c.Request().Context(), id); err != nil {
		switch err {
		case service.ErrTeamNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		case service.ErrTeamNotActive:
			return localizedError(c, http.StatusConflict, i18n.MsgTeamStillActive)
		default:
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteTeam)
		}
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *TeamHandler) Update(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTeamID)
	}

	req := new(model.UpdateTeamRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}

	team, err := h.service.UpdateTeam(c.Request().Context(), id, req)
	if err != nil {
		switch err {
		case service.ErrTeamNotFound:
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		default:
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
		}
	}

	summary, summaryErr := h.service.GetSummary(c.Request().Context(), team.ID)
	if summaryErr == nil && summary != nil {
		return c.JSON(http.StatusOK, summary)
	}
	return c.JSON(http.StatusOK, team.ToDTO())
}

func (h *TeamHandler) ListArtifacts(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgTeamServiceUnavailable)
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTeamID)
	}

	artifacts, err := h.service.ListArtifacts(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgTeamNotFound)
	}
	return c.JSON(http.StatusOK, artifacts)
}
