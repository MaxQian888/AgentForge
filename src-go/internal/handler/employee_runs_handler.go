package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// employeeRunsRepo is the narrow read-side contract that EmployeeRunsHandler
// depends on. In production it is satisfied by *repository.EmployeeRunsRepository.
type employeeRunsRepo interface {
	ListByEmployee(ctx context.Context, employeeID uuid.UUID, kind repository.EmployeeRunKind, page, size int) ([]repository.EmployeeRunRow, error)
}

// EmployeeRunsHandler serves GET /api/v1/employees/:id/runs.
type EmployeeRunsHandler struct {
	repo employeeRunsRepo
}

// NewEmployeeRunsHandler returns a new EmployeeRunsHandler backed by the
// given repository.
func NewEmployeeRunsHandler(repo employeeRunsRepo) *EmployeeRunsHandler {
	return &EmployeeRunsHandler{repo: repo}
}

// employeeRunsResponse wraps the page so the FE can advance pagination
// without a separate count query (HasMore is derived by the FE: rows length
// == size means a next page may exist).
type employeeRunsResponse struct {
	Items []repository.EmployeeRunRow `json:"items"`
	Page  int                         `json:"page"`
	Size  int                         `json:"size"`
	Kind  string                      `json:"kind"`
}

// List handles GET /api/v1/employees/:id/runs?type=&page=&size=
//
// Query params:
//
//	type   one of "all" (default), "workflow", "agent"
//	page   1-indexed, defaults to 1
//	size   1..200, defaults to 20
func (h *EmployeeRunsHandler) List(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeID)
	}

	kind := repository.EmployeeRunKindAll
	switch c.QueryParam("type") {
	case "workflow":
		kind = repository.EmployeeRunKindWorkflow
	case "agent":
		kind = repository.EmployeeRunKindAgent
	case "", "all":
		// keep default
	default:
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidEmployeeRequest)
	}

	page := 1
	if v := c.QueryParam("page"); v != "" {
		if parsed, perr := strconv.Atoi(v); perr == nil {
			page = parsed
		}
	}
	size := 20
	if v := c.QueryParam("size"); v != "" {
		if parsed, perr := strconv.Atoi(v); perr == nil {
			size = parsed
		}
	}

	rows, err := h.repo.ListByEmployee(c.Request().Context(), id, kind, page, size)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListAgentRuns)
	}

	return c.JSON(http.StatusOK, employeeRunsResponse{
		Items: rows,
		Page:  page,
		Size:  size,
		Kind:  string(kind),
	})
}
