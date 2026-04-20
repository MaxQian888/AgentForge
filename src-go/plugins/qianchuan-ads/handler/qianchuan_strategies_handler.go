package qchandler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/plugins/qianchuan-ads/strategy"
	"github.com/react-go-quick-starter/server/internal/repository"
	qcservice "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/service"
)

// QianchuanStrategyService is the narrow seam this handler consumes; tests
// can mock it without touching the GORM repo.
type QianchuanStrategyService interface {
	Create(ctx context.Context, in qcservice.QianchuanStrategyCreateInput) (*strategy.QianchuanStrategy, error)
	Update(ctx context.Context, id uuid.UUID, yamlSource string) (*strategy.QianchuanStrategy, error)
	Publish(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*strategy.QianchuanStrategy, error)
	List(ctx context.Context, projectID uuid.UUID) ([]*strategy.QianchuanStrategy, error)
	TestRun(ctx context.Context, id uuid.UUID, snapshot map[string]any) (*qcservice.TestRunResult, error)
}

// QianchuanStrategiesHandler exposes the strategy library REST API.
type QianchuanStrategiesHandler struct {
	svc QianchuanStrategyService
}

// NewQianchuanStrategiesHandler constructs a new handler.
func NewQianchuanStrategiesHandler(svc QianchuanStrategyService) *QianchuanStrategiesHandler {
	return &QianchuanStrategiesHandler{svc: svc}
}

// QianchuanStrategyDTO is the wire shape for a single strategy. Field names
// follow the project's camelCase JSON convention.
type QianchuanStrategyDTO struct {
	ID          string  `json:"id"`
	ProjectID   *string `json:"projectId"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	YAMLSource  string  `json:"yamlSource"`
	ParsedSpec  string  `json:"parsedSpec"`
	Version     int     `json:"version"`
	Status      string  `json:"status"`
	CreatedBy   string  `json:"createdBy"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	IsSystem    bool    `json:"isSystem"`
}

func toDTO(s *strategy.QianchuanStrategy) QianchuanStrategyDTO {
	dto := QianchuanStrategyDTO{
		ID:          s.ID.String(),
		Name:        s.Name,
		Description: s.Description,
		YAMLSource:  s.YAMLSource,
		ParsedSpec:  s.ParsedSpec,
		Version:     s.Version,
		Status:      s.Status,
		CreatedBy:   s.CreatedBy.String(),
		CreatedAt:   s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   s.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		IsSystem:    s.IsSystem(),
	}
	if s.ProjectID != nil {
		pid := s.ProjectID.String()
		dto.ProjectID = &pid
	}
	return dto
}

// errorBody is the JSON wrapper for ANY error response from this handler.
// On parse errors the inner error is a strategy.StrategyParseError so the
// FE can place Monaco markers without re-parsing the message.
type errorBody struct {
	Error any `json:"error"`
}

// List returns all strategies visible to the project (project-scoped + system seeds).
// Query params: ?status=draft|published|archived narrows the result.
func (h *QianchuanStrategiesHandler) List(c echo.Context) error {
	pid, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid project id"})
	}
	rows, err := h.svc.List(c.Request().Context(), pid)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorBody{Error: err.Error()})
	}
	statusFilter := strings.TrimSpace(c.QueryParam("status"))
	out := make([]QianchuanStrategyDTO, 0, len(rows))
	for _, r := range rows {
		if statusFilter != "" && r.Status != statusFilter {
			continue
		}
		out = append(out, toDTO(r))
	}
	return c.JSON(http.StatusOK, out)
}

// Get returns a single strategy by ID.
func (h *QianchuanStrategiesHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid strategy id"})
	}
	row, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, toDTO(row))
}

// CreateRequest is the body for POST .../strategies.
type qianchuanCreateRequest struct {
	YAMLSource string `json:"yamlSource"`
}

// Create accepts YAML source, parses + validates, and persists a new draft.
// Returns 400 with a structured StrategyParseError on parse failure.
func (h *QianchuanStrategiesHandler) Create(c echo.Context) error {
	pid, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid project id"})
	}
	userID, err := claimsUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, errorBody{Error: "auth required"})
	}
	var req qianchuanCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid request body"})
	}
	row, err := h.svc.Create(c.Request().Context(), qcservice.QianchuanStrategyCreateInput{
		ProjectID:  &pid,
		YAMLSource: req.YAMLSource,
		CreatedBy:  *userID,
	})
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, toDTO(row))
}

// Update overwrites a draft row with re-parsed YAML.
type qianchuanUpdateRequest struct {
	YAMLSource string `json:"yamlSource"`
}

func (h *QianchuanStrategiesHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid strategy id"})
	}
	var req qianchuanUpdateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid request body"})
	}
	row, err := h.svc.Update(c.Request().Context(), id, req.YAMLSource)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, toDTO(row))
}

// Publish flips draft -> published.
func (h *QianchuanStrategiesHandler) Publish(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid strategy id"})
	}
	if err := h.svc.Publish(c.Request().Context(), id); err != nil {
		return mapServiceError(c, err)
	}
	row, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, toDTO(row))
}

// Archive flips published -> archived.
func (h *QianchuanStrategiesHandler) Archive(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid strategy id"})
	}
	if err := h.svc.Archive(c.Request().Context(), id); err != nil {
		return mapServiceError(c, err)
	}
	row, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, toDTO(row))
}

// Delete hard-deletes a draft row.
func (h *QianchuanStrategiesHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid strategy id"})
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return mapServiceError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// TestRun runs a dry-run evaluation against a user-supplied snapshot.
type qianchuanTestRunRequest struct {
	Snapshot map[string]any `json:"snapshot"`
}

func (h *QianchuanStrategiesHandler) TestRun(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid strategy id"})
	}
	var req qianchuanTestRunRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorBody{Error: "invalid request body"})
	}
	res, err := h.svc.TestRun(c.Request().Context(), id, req.Snapshot)
	if err != nil {
		return mapServiceError(c, err)
	}
	return c.JSON(http.StatusOK, res)
}

// mapServiceError converts the service-layer error vocabulary into HTTP
// responses. StrategyParseError preserves its structure so the FE editor
// can render markers; ErrStrategy* maps to 409; ErrNotFound to 404; system
// read-only to 403.
func mapServiceError(c echo.Context, err error) error {
	var spe *strategy.StrategyParseError
	if errors.As(err, &spe) {
		return c.JSON(http.StatusBadRequest, errorBody{Error: spe})
	}
	if errors.Is(err, repository.ErrNotFound) {
		return c.JSON(http.StatusNotFound, errorBody{Error: "not found"})
	}
	if errors.Is(err, qcservice.ErrStrategySystemReadOnly) {
		return c.JSON(http.StatusForbidden, errorBody{Error: err.Error()})
	}
	if errors.Is(err, qcservice.ErrStrategyImmutable) || errors.Is(err, qcservice.ErrStrategyInvalidTransition) {
		return c.JSON(http.StatusConflict, errorBody{Error: err.Error()})
	}
	return c.JSON(http.StatusInternalServerError, errorBody{Error: err.Error()})
}

// RegisterQianchuanStrategyRoutes mounts the route table onto the v1 group.
// Project-scoped read+create live under /projects/:pid/qianchuan/strategies;
// per-row mutations live under /qianchuan/strategies/:id.
func RegisterQianchuanStrategyRoutes(v1 *echo.Group, h *QianchuanStrategiesHandler, mws ...echo.MiddlewareFunc) {
	v1.GET("/projects/:pid/qianchuan/strategies", h.List, mws...)
	v1.POST("/projects/:pid/qianchuan/strategies", h.Create, mws...)
	v1.GET("/qianchuan/strategies/:id", h.Get, mws...)
	v1.PATCH("/qianchuan/strategies/:id", h.Update, mws...)
	v1.POST("/qianchuan/strategies/:id/publish", h.Publish, mws...)
	v1.POST("/qianchuan/strategies/:id/archive", h.Archive, mws...)
	v1.DELETE("/qianchuan/strategies/:id", h.Delete, mws...)
	v1.POST("/qianchuan/strategies/:id/test", h.TestRun, mws...)
}
