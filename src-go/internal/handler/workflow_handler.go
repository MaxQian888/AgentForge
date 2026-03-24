package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type workflowRepository interface {
	GetByProject(ctx context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error)
	Upsert(ctx context.Context, projectID uuid.UUID, transitions json.RawMessage, triggers json.RawMessage) (*model.WorkflowConfig, error)
}

type WorkflowHandler struct {
	repo workflowRepository
}

func NewWorkflowHandler(repo workflowRepository) *WorkflowHandler {
	return &WorkflowHandler{repo: repo}
}

func (h *WorkflowHandler) Get(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	wf, err := h.repo.GetByProject(c.Request().Context(), projectID)
	if err != nil {
		// Return default empty config if not found
		return c.JSON(http.StatusOK, model.WorkflowConfigDTO{
			ProjectID:   projectID.String(),
			Transitions: make(map[string][]string),
			Triggers:    make([]model.WorkflowTrigger, 0),
		})
	}
	return c.JSON(http.StatusOK, wf.ToDTO())
}

func (h *WorkflowHandler) Put(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)

	req := new(model.UpdateWorkflowRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}

	transitionsJSON, err := json.Marshal(req.Transitions)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid transitions"})
	}
	triggersJSON, err := json.Marshal(req.Triggers)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid triggers"})
	}

	wf, err := h.repo.Upsert(c.Request().Context(), projectID, transitionsJSON, triggersJSON)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to save workflow config"})
	}

	return c.JSON(http.StatusOK, wf.ToDTO())
}
