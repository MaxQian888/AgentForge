package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrMemoryAccessDenied  = errors.New("memory access denied")
	ErrInvalidMemoryImport = errors.New("invalid episodic memory import")
)

type EpisodicMemoryRepository interface {
	Create(ctx context.Context, mem *model.AgentMemory) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentMemory, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, scope, category string) ([]*model.AgentMemory, error)
	ListByProjectAndTimeRange(ctx context.Context, projectID uuid.UUID, category, scope, roleID string, start, end *time.Time, limit int) ([]*model.AgentMemory, error)
	DeleteOlderThan(ctx context.Context, projectID uuid.UUID, category string, before time.Time) (int64, error)
}

type StoreConversationTurnInput struct {
	ProjectID      uuid.UUID
	Scope          string
	RoleID         string
	SessionID      string
	TurnNumber     int
	Actor          string
	Content        string
	OccurredAt     *time.Time
	Metadata       map[string]any
	RelevanceScore float64
}

type EpisodicMemoryQuery struct {
	ProjectID uuid.UUID
	Scope     string
	RoleID    string
	StartAt   *time.Time
	EndAt     *time.Time
	Limit     int
}

type MemoryAccessRequest struct {
	ProjectID uuid.UUID
	RoleID    string
}

type EpisodicMemoryExportRequest struct {
	ProjectID uuid.UUID
	Query     string
	Category  string
	RoleID    string
	Scope     string
	StartAt   *time.Time
	EndAt     *time.Time
}

type EpisodicMemoryExport struct {
	ProjectID  string                      `json:"projectId"`
	ExportedAt string                      `json:"exportedAt"`
	Entries    []EpisodicMemoryExportEntry `json:"entries"`
}

type EpisodicMemoryExportEntry struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	RoleID    string `json:"roleId,omitempty"`
	Category  string `json:"category"`
	Key       string `json:"key"`
	Content   string `json:"content"`
	Metadata  string `json:"metadata"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type SessionSnapshotImportRequest struct {
	ProjectID uuid.UUID
	RoleID    string
	Scope     string
	Dir       string
}

type bridgeSessionSnapshot struct {
	TaskID     string         `json:"task_id"`
	SessionID  string         `json:"session_id"`
	Status     string         `json:"status"`
	TurnNumber int            `json:"turn_number"`
	SpentUSD   float64        `json:"spent_usd"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
	Request    map[string]any `json:"request"`
	Continuity map[string]any `json:"continuity"`
}

type EpisodicMemoryService struct {
	repo EpisodicMemoryRepository
	now  func() time.Time
}

func NewEpisodicMemoryService(repo EpisodicMemoryRepository) *EpisodicMemoryService {
	return &EpisodicMemoryService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *EpisodicMemoryService) StoreTurn(ctx context.Context, input StoreConversationTurnInput) (*model.AgentMemory, error) {
	if strings.TrimSpace(input.Content) == "" {
		return nil, fmt.Errorf("conversation turn content is required")
	}
	if input.ProjectID == uuid.Nil {
		return nil, fmt.Errorf("project id is required")
	}

	createdAt := s.now()
	if input.OccurredAt != nil && !input.OccurredAt.IsZero() {
		createdAt = input.OccurredAt.UTC()
	}
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		scope = model.MemoryScopeProject
	}
	key := fmt.Sprintf("session:%s:turn:%d", strings.TrimSpace(input.SessionID), input.TurnNumber)
	if strings.TrimSpace(input.SessionID) == "" {
		key = fmt.Sprintf("turn:%d", input.TurnNumber)
	}

	metadataPayload := map[string]any{
		"sessionId":  input.SessionID,
		"turnNumber": input.TurnNumber,
		"actor":      input.Actor,
	}
	for keyName, value := range input.Metadata {
		metadataPayload[keyName] = value
	}
	metadataRaw, err := json.Marshal(metadataPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal episodic memory metadata: %w", err)
	}

	entry := &model.AgentMemory{
		ID:             uuid.New(),
		ProjectID:      input.ProjectID,
		Scope:          scope,
		RoleID:         strings.TrimSpace(input.RoleID),
		Category:       model.MemoryCategoryEpisodic,
		Key:            key,
		Content:        strings.TrimSpace(input.Content),
		Metadata:       string(metadataRaw),
		RelevanceScore: normalizeEpisodicScore(input.RelevanceScore),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("store episodic memory turn: %w", err)
	}
	return entry, nil
}

func (s *EpisodicMemoryService) Get(ctx context.Context, id uuid.UUID, access MemoryAccessRequest) (*model.AgentMemory, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get episodic memory: %w", err)
	}
	if entry == nil {
		return nil, repository.ErrNotFound
	}
	if err := ensureMemoryAccess(entry, access); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *EpisodicMemoryService) ListHistory(ctx context.Context, query EpisodicMemoryQuery) ([]*model.AgentMemory, error) {
	scope := strings.TrimSpace(query.Scope)
	if scope == model.MemoryScopeRole && strings.TrimSpace(query.RoleID) == "" {
		return nil, fmt.Errorf("role id is required for role-scoped episodic memory")
	}

	entries, err := s.repo.ListByProjectAndTimeRange(ctx, query.ProjectID, model.MemoryCategoryEpisodic, scope, strings.TrimSpace(query.RoleID), query.StartAt, query.EndAt, query.Limit)
	if err != nil {
		return nil, fmt.Errorf("list episodic memory history: %w", err)
	}
	return filterAccessibleMemories(entries, MemoryAccessRequest{
		ProjectID: query.ProjectID,
		RoleID:    query.RoleID,
	})
}

func (s *EpisodicMemoryService) ApplyRetention(ctx context.Context, projectID uuid.UUID, retentionDays int, now time.Time) (int64, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("retention days must be positive")
	}
	if now.IsZero() {
		now = s.now()
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays)
	deleted, err := s.repo.DeleteOlderThan(ctx, projectID, model.MemoryCategoryEpisodic, cutoff)
	if err != nil {
		return 0, fmt.Errorf("apply episodic retention: %w", err)
	}
	return deleted, nil
}

func (s *EpisodicMemoryService) Export(ctx context.Context, req EpisodicMemoryExportRequest) (*EpisodicMemoryExport, error) {
	if category := strings.TrimSpace(req.Category); category != "" && category != model.MemoryCategoryEpisodic {
		return &EpisodicMemoryExport{
			ProjectID:  req.ProjectID.String(),
			ExportedAt: s.now().Format(time.RFC3339),
			Entries:    []EpisodicMemoryExportEntry{},
		}, nil
	}

	entries, err := s.ListHistory(ctx, EpisodicMemoryQuery{
		ProjectID: req.ProjectID,
		Scope:     strings.TrimSpace(req.Scope),
		RoleID:    strings.TrimSpace(req.RoleID),
		StartAt:   req.StartAt,
		EndAt:     req.EndAt,
		Limit:     0,
	})
	if err != nil {
		return nil, fmt.Errorf("export episodic memory: %w", err)
	}
	if query := strings.TrimSpace(req.Query); query != "" {
		entries = filterMemoriesBySearch(entries, query)
	}
	exported := &EpisodicMemoryExport{
		ProjectID:  req.ProjectID.String(),
		ExportedAt: s.now().Format(time.RFC3339),
		Entries:    make([]EpisodicMemoryExportEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		exported.Entries = append(exported.Entries, EpisodicMemoryExportEntry{
			ID:        entry.ID.String(),
			Scope:     entry.Scope,
			RoleID:    entry.RoleID,
			Category:  entry.Category,
			Key:       entry.Key,
			Content:   entry.Content,
			Metadata:  entry.Metadata,
			CreatedAt: entry.CreatedAt.Format(time.RFC3339),
			UpdatedAt: entry.UpdatedAt.Format(time.RFC3339),
		})
	}
	return exported, nil
}

func (s *EpisodicMemoryService) Import(ctx context.Context, projectID uuid.UUID, payload []byte) (int, error) {
	var exported EpisodicMemoryExport
	if err := json.Unmarshal(payload, &exported); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidMemoryImport, err)
	}
	imported := 0
	for _, entry := range exported.Entries {
		if strings.TrimSpace(entry.Content) == "" || strings.TrimSpace(entry.Key) == "" {
			return imported, fmt.Errorf("%w: entry key/content is required", ErrInvalidMemoryImport)
		}
		id := uuid.New()
		if strings.TrimSpace(entry.ID) != "" {
			parsedID, err := uuid.Parse(entry.ID)
			if err != nil {
				return imported, fmt.Errorf("%w: invalid entry id %s", ErrInvalidMemoryImport, entry.ID)
			}
			id = parsedID
		}
		createdAt := s.now()
		if strings.TrimSpace(entry.CreatedAt) != "" {
			parsed, err := time.Parse(time.RFC3339, entry.CreatedAt)
			if err != nil {
				return imported, fmt.Errorf("%w: invalid createdAt %s", ErrInvalidMemoryImport, entry.CreatedAt)
			}
			createdAt = parsed.UTC()
		}
		updatedAt := createdAt
		if strings.TrimSpace(entry.UpdatedAt) != "" {
			parsed, err := time.Parse(time.RFC3339, entry.UpdatedAt)
			if err != nil {
				return imported, fmt.Errorf("%w: invalid updatedAt %s", ErrInvalidMemoryImport, entry.UpdatedAt)
			}
			updatedAt = parsed.UTC()
		}
		category := strings.TrimSpace(entry.Category)
		if category == "" {
			category = model.MemoryCategoryEpisodic
		}
		mem := &model.AgentMemory{
			ID:             id,
			ProjectID:      projectID,
			Scope:          defaultScope(entry.Scope),
			RoleID:         strings.TrimSpace(entry.RoleID),
			Category:       category,
			Key:            strings.TrimSpace(entry.Key),
			Content:        strings.TrimSpace(entry.Content),
			Metadata:       strings.TrimSpace(entry.Metadata),
			RelevanceScore: 1,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		}
		if err := s.repo.Create(ctx, mem); err != nil {
			return imported, fmt.Errorf("import episodic memory entry %s: %w", mem.Key, err)
		}
		imported++
	}
	return imported, nil
}

func (s *EpisodicMemoryService) ImportSessionSnapshots(ctx context.Context, req SessionSnapshotImportRequest) (int, error) {
	entries, err := os.ReadDir(req.Dir)
	if err != nil {
		return 0, fmt.Errorf("read session snapshot dir: %w", err)
	}

	imported := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(req.Dir, entry.Name()))
		if err != nil {
			return imported, fmt.Errorf("read session snapshot %s: %w", entry.Name(), err)
		}
		var snapshot bridgeSessionSnapshot
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return imported, fmt.Errorf("decode session snapshot %s: %w", entry.Name(), err)
		}

		metadata := map[string]any{
			"taskId":     snapshot.TaskID,
			"sessionId":  snapshot.SessionID,
			"status":     snapshot.Status,
			"turnNumber": snapshot.TurnNumber,
			"spentUsd":   snapshot.SpentUSD,
			"request":    snapshot.Request,
			"continuity": snapshot.Continuity,
			"sourceFile": entry.Name(),
			"sourceType": "bridge_session_snapshot",
		}
		content := fmt.Sprintf("Migrated bridge session snapshot for task %s at turn %d with status %s.", snapshot.TaskID, snapshot.TurnNumber, snapshot.Status)
		if prompt, ok := snapshot.Request["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
			content = fmt.Sprintf("%s Prompt: %s", content, strings.TrimSpace(prompt))
		}
		occurredAt := time.UnixMilli(snapshot.UpdatedAt).UTC()
		if snapshot.UpdatedAt == 0 && snapshot.CreatedAt != 0 {
			occurredAt = time.UnixMilli(snapshot.CreatedAt).UTC()
		}

		if _, err := s.StoreTurn(ctx, StoreConversationTurnInput{
			ProjectID:  req.ProjectID,
			Scope:      defaultScope(req.Scope),
			RoleID:     strings.TrimSpace(req.RoleID),
			SessionID:  snapshot.SessionID,
			TurnNumber: snapshot.TurnNumber,
			Actor:      "system",
			Content:    content,
			OccurredAt: &occurredAt,
			Metadata:   metadata,
		}); err != nil {
			return imported, fmt.Errorf("import session snapshot %s: %w", entry.Name(), err)
		}
		imported++
	}
	return imported, nil
}

func ensureMemoryAccess(entry *model.AgentMemory, access MemoryAccessRequest) error {
	if entry == nil {
		return repository.ErrNotFound
	}
	if access.ProjectID != uuid.Nil && entry.ProjectID != access.ProjectID {
		return ErrMemoryAccessDenied
	}
	if entry.Scope == model.MemoryScopeRole && strings.TrimSpace(entry.RoleID) != "" && strings.TrimSpace(access.RoleID) != entry.RoleID {
		return ErrMemoryAccessDenied
	}
	return nil
}

func filterAccessibleMemories(entries []*model.AgentMemory, access MemoryAccessRequest) ([]*model.AgentMemory, error) {
	filtered := make([]*model.AgentMemory, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.Scope == model.MemoryScopeRole && strings.TrimSpace(entry.RoleID) != "" && strings.TrimSpace(access.RoleID) != entry.RoleID {
			continue
		}
		filtered = append(filtered, cloneAgentMemory(entry))
	}
	return filtered, nil
}

func normalizeEpisodicScore(score float64) float64 {
	if score <= 0 {
		return 1
	}
	return score
}

func defaultScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return model.MemoryScopeProject
	}
	return scope
}

func cloneAgentMemory(entry *model.AgentMemory) *model.AgentMemory {
	if entry == nil {
		return nil
	}
	cloned := *entry
	if entry.LastAccessedAt != nil {
		value := *entry.LastAccessedAt
		cloned.LastAccessedAt = &value
	}
	return &cloned
}
