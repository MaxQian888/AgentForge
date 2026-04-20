package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type logService interface {
	CreateLog(ctx context.Context, input model.CreateLogInput) (*model.Log, error)
	ListLogs(ctx context.Context, req model.LogListRequest) (*model.LogListResponse, error)
}

// LogHandler handles HTTP requests for log entries.
type LogHandler struct {
	service logService
}

// NewLogHandler creates a new LogHandler.
func NewLogHandler(svc logService) *LogHandler {
	return &LogHandler{service: svc}
}

// List returns a paginated list of log entries for a project.
func (h *LogHandler) List(c echo.Context) error {
	pidStr := c.Param("pid")
	projectID, err := uuid.Parse(pidStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid project id"})
	}

	tab := c.QueryParam("tab")
	if tab != "" && tab != model.LogTabAgent && tab != model.LogTabSystem {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid tab, must be 'agent' or 'system'"})
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	level := c.QueryParam("level")
	search := c.QueryParam("search")

	req := model.LogListRequest{
		ProjectID: projectID,
		Tab:       tab,
		Page:      page,
		PageSize:  pageSize,
		Level:     level,
		Search:    search,
	}

	if from := c.QueryParam("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			req.From = &t
		}
	}
	if to := c.QueryParam("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			req.To = &t
		}
	}

	resp, err := h.service.ListLogs(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list logs"})
	}

	return c.JSON(http.StatusOK, resp)
}

// Create inserts a new log entry for a project.
func (h *LogHandler) Create(c echo.Context) error {
	pidStr := c.Param("pid")
	projectID, err := uuid.Parse(pidStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid project id"})
	}

	var body struct {
		Tab          string         `json:"tab"`
		Level        string         `json:"level"`
		ActorType    string         `json:"actorType"`
		ActorID      string         `json:"actorId"`
		AgentID      *string        `json:"agentId"`
		SessionID    string         `json:"sessionId"`
		EventType    string         `json:"eventType"`
		Action       string         `json:"action"`
		ResourceType string         `json:"resourceType"`
		ResourceID   string         `json:"resourceId"`
		Summary      string         `json:"summary"`
		Detail       map[string]any `json:"detail"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}

	input := model.CreateLogInput{
		ProjectID:    projectID,
		Tab:          body.Tab,
		Level:        body.Level,
		ActorType:    body.ActorType,
		ActorID:      body.ActorID,
		SessionID:    body.SessionID,
		EventType:    body.EventType,
		Action:       body.Action,
		ResourceType: body.ResourceType,
		ResourceID:   body.ResourceID,
		Summary:      body.Summary,
		Detail:       body.Detail,
	}
	if body.AgentID != nil {
		if aid, err := uuid.Parse(*body.AgentID); err == nil {
			input.AgentID = &aid
		}
	}

	entry, err := h.service.CreateLog(c.Request().Context(), input)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create log"})
	}

	dto := entry.ToDTO()
	return c.JSON(http.StatusCreated, dto)
}
