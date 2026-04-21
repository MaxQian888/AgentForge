package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	eventbus "github.com/agentforge/server/internal/eventbus"
	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/ws"
	"github.com/google/uuid"
)

type logRepository interface {
	Create(ctx context.Context, log *model.Log) error
	List(ctx context.Context, req model.LogListRequest) ([]model.Log, int64, error)
}

// LogService handles log entry creation and retrieval.
type LogService struct {
	repo logRepository
	hub  *ws.Hub
	bus  eventbus.Publisher
}

// NewLogService creates a new LogService.
func NewLogService(repo logRepository, hub *ws.Hub, bus eventbus.Publisher) *LogService {
	return &LogService{repo: repo, hub: hub, bus: bus}
}

// CreateLog persists a new log entry and broadcasts it over WebSocket.
func (s *LogService) CreateLog(ctx context.Context, input model.CreateLogInput) (*model.Log, error) {
	if input.Tab == "" {
		input.Tab = model.LogTabSystem
	}
	if input.Level == "" {
		input.Level = model.LogLevelInfo
	}

	if tid := applog.TraceID(ctx); tid != "" {
		if input.Detail == nil {
			input.Detail = map[string]any{}
		}
		input.Detail["trace_id"] = tid
	}

	detailJSON, _ := json.Marshal(input.Detail)
	if detailJSON == nil {
		detailJSON = []byte("{}")
	}

	entry := &model.Log{
		ID:           uuid.New(),
		ProjectID:    input.ProjectID,
		Tab:          input.Tab,
		Level:        input.Level,
		ActorType:    input.ActorType,
		ActorID:      input.ActorID,
		AgentID:      input.AgentID,
		SessionID:    input.SessionID,
		EventType:    input.EventType,
		Action:       input.Action,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		Summary:      input.Summary,
		Detail:       detailJSON,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("create log: %w", err)
	}

	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventLogCreated, entry.ProjectID.String(), entry.ToDTO())

	return entry, nil
}

// ListLogs returns a paginated list of log entries.
func (s *LogService) ListLogs(ctx context.Context, req model.LogListRequest) (*model.LogListResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 50
	}

	logs, total, err := s.repo.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list logs: %w", err)
	}

	items := make([]model.LogDTO, len(logs))
	for i := range logs {
		items[i] = logs[i].ToDTO()
	}

	return &model.LogListResponse{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}
