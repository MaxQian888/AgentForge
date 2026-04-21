package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
)

// LogCreator is the narrow slice of LogService the ingest handler depends on.
type LogCreator interface {
	CreateLog(ctx context.Context, in model.CreateLogInput) (*model.Log, error)
}

type IngestHandler struct{ svc LogCreator }

func NewIngestHandler(svc LogCreator) *IngestHandler { return &IngestHandler{svc: svc} }

type ingestRequest struct {
	ProjectID string         `json:"projectId"`
	Tab       string         `json:"tab"`
	Level     string         `json:"level"`
	Source    string         `json:"source"`
	Summary   string         `json:"summary"`
	Detail    map[string]any `json:"detail,omitempty"`
	EventType string         `json:"eventType,omitempty"`
	Action    string         `json:"action,omitempty"`
}

var allowedIngestLevels = map[string]struct{}{
	model.LogLevelDebug: {},
	model.LogLevelInfo:  {},
	model.LogLevelWarn:  {},
	model.LogLevelError: {},
}

// Ingest accepts either a single ingestRequest JSON object or a JSON array of them.
// All entries are written to the logs table with trace_id merged into detail.
func (h *IngestHandler) Ingest(c echo.Context) error {
	raw, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "read body"})
	}

	batch, err := parseIngestBody(raw)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	for _, req := range batch {
		if err := validateIngestRequest(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
	}

	traceID := applog.TraceID(c.Request().Context())
	for _, req := range batch {
		in, err := buildCreateLogInput(&req, traceID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if _, err := h.svc.CreateLog(c.Request().Context(), in); err != nil {
			if errors.Is(err, context.Canceled) {
				return c.NoContent(http.StatusRequestTimeout)
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "ingest failed"})
		}
	}
	return c.NoContent(http.StatusAccepted)
}

func parseIngestBody(raw []byte) ([]ingestRequest, error) {
	trimmed := firstNonSpace(raw)
	if trimmed == '[' {
		var arr []ingestRequest
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, errors.New("invalid json array")
		}
		return arr, nil
	}
	var one ingestRequest
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, errors.New("invalid json")
	}
	return []ingestRequest{one}, nil
}

func firstNonSpace(raw []byte) byte {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b
		}
	}
	return 0
}

func validateIngestRequest(req *ingestRequest) error {
	if _, ok := allowedIngestLevels[req.Level]; !ok {
		return errors.New("invalid level")
	}
	if req.Summary == "" {
		return errors.New("summary required")
	}
	if req.Tab == "" {
		req.Tab = model.LogTabSystem
	}
	return nil
}

func buildCreateLogInput(req *ingestRequest, traceID string) (model.CreateLogInput, error) {
	detail := req.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	if traceID != "" {
		detail["trace_id"] = traceID
	}
	if req.Source != "" {
		detail["source"] = req.Source
	}

	var projectID uuid.UUID
	if req.ProjectID != "" {
		id, err := uuid.Parse(req.ProjectID)
		if err != nil {
			return model.CreateLogInput{}, errors.New("invalid projectId")
		}
		projectID = id
	}

	return model.CreateLogInput{
		ProjectID: projectID,
		Tab:       req.Tab,
		Level:     req.Level,
		ActorType: "service",
		ActorID:   req.Source,
		EventType: req.EventType,
		Action:    req.Action,
		Summary:   req.Summary,
		Detail:    detail,
	}, nil
}
