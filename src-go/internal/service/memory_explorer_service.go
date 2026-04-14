package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type MemoryExplorerQuery struct {
	ProjectID uuid.UUID
	Query     string
	Scope     string
	Category  string
	RoleID    string
	StartAt   *time.Time
	EndAt     *time.Time
	Limit     int
}

type MemoryCleanupInput struct {
	ProjectID     uuid.UUID
	Scope         string
	RoleID        string
	Before        *time.Time
	RetentionDays int
}

type MemoryExplorerRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentMemory, error)
	ListFiltered(ctx context.Context, projectID uuid.UUID, filter model.AgentMemoryFilter) ([]*model.AgentMemory, error)
	IncrementAccess(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteMany(ctx context.Context, ids []uuid.UUID) (int64, error)
}

type memoryExplorerEpisodicRuntime interface {
	Get(ctx context.Context, id uuid.UUID, access MemoryAccessRequest) (*model.AgentMemory, error)
	ListHistory(ctx context.Context, query EpisodicMemoryQuery) ([]*model.AgentMemory, error)
	Export(ctx context.Context, req EpisodicMemoryExportRequest) (*EpisodicMemoryExport, error)
}

type MemoryExplorerService struct {
	repo     MemoryExplorerRepository
	episodic memoryExplorerEpisodicRuntime
	now      func() time.Time
}

func NewMemoryExplorerService(repo MemoryExplorerRepository) *MemoryExplorerService {
	return &MemoryExplorerService{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *MemoryExplorerService) WithEpisodic(episodic memoryExplorerEpisodicRuntime) *MemoryExplorerService {
	s.episodic = episodic
	return s
}

func (s *MemoryExplorerService) Search(ctx context.Context, query MemoryExplorerQuery) ([]model.AgentMemoryDTO, error) {
	entries, err := s.listEntries(ctx, query)
	if err != nil {
		return nil, err
	}
	dtos := make([]model.AgentMemoryDTO, 0, len(entries))
	for _, entry := range entries {
		_ = s.repo.IncrementAccess(ctx, entry.ID)
		dtos = append(dtos, entry.ToDTO())
	}
	return dtos, nil
}

func (s *MemoryExplorerService) Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID, roleID string) (*model.AgentMemoryDetailDTO, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("memory explorer repository is required")
	}
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get memory detail: %w", err)
	}
	if entry.Category == model.MemoryCategoryEpisodic && s.episodic != nil {
		entry, err = s.episodic.Get(ctx, id, MemoryAccessRequest{ProjectID: projectID, RoleID: roleID})
		if err != nil {
			return nil, fmt.Errorf("get memory detail: %w", err)
		}
	} else if err := ensureExplorerAccess(entry, MemoryAccessRequest{ProjectID: projectID, RoleID: roleID}); err != nil {
		return nil, err
	}
	_ = s.repo.IncrementAccess(ctx, entry.ID)
	dto := entry.ToDetailDTO()
	return &dto, nil
}

func (s *MemoryExplorerService) Stats(ctx context.Context, query MemoryExplorerQuery) (*model.MemoryExplorerStatsDTO, error) {
	statsQuery := query
	statsQuery.Limit = 0
	entries, err := s.listEntries(ctx, statsQuery)
	if err != nil {
		return nil, err
	}
	stats := &model.MemoryExplorerStatsDTO{
		ByCategory: make(map[string]int),
		ByScope:    make(map[string]int),
	}
	var oldestCreated *time.Time
	var newestCreated *time.Time
	var lastAccessed *time.Time
	for _, entry := range entries {
		stats.TotalCount++
		stats.ApproxStorageBytes += len(entry.Key) + len(entry.Content) + len(entry.Metadata)
		stats.ByCategory[entry.Category]++
		stats.ByScope[entry.Scope]++
		if oldestCreated == nil || entry.CreatedAt.Before(*oldestCreated) {
			value := entry.CreatedAt
			oldestCreated = &value
		}
		if newestCreated == nil || entry.CreatedAt.After(*newestCreated) {
			value := entry.CreatedAt
			newestCreated = &value
		}
		if entry.LastAccessedAt != nil && (lastAccessed == nil || entry.LastAccessedAt.After(*lastAccessed)) {
			value := entry.LastAccessedAt.UTC()
			lastAccessed = &value
		}
	}
	if oldestCreated != nil {
		stats.OldestCreatedAt = oldestCreated.Format(time.RFC3339)
	}
	if newestCreated != nil {
		stats.NewestCreatedAt = newestCreated.Format(time.RFC3339)
	}
	if lastAccessed != nil {
		stats.LastAccessedAt = lastAccessed.Format(time.RFC3339)
	}
	return stats, nil
}

func (s *MemoryExplorerService) ExportEpisodic(ctx context.Context, query MemoryExplorerQuery) (*EpisodicMemoryExport, error) {
	if s.episodic == nil {
		return nil, fmt.Errorf("episodic memory explorer is not configured")
	}
	return s.episodic.Export(ctx, EpisodicMemoryExportRequest{
		ProjectID: query.ProjectID,
		Query:     strings.TrimSpace(query.Query),
		Category:  strings.TrimSpace(query.Category),
		RoleID:    strings.TrimSpace(query.RoleID),
		Scope:     strings.TrimSpace(query.Scope),
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
	})
}

func (s *MemoryExplorerService) BulkDelete(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID, roleID string) (int64, error) {
	if s.repo == nil {
		return 0, fmt.Errorf("memory explorer repository is required")
	}
	if len(ids) == 0 {
		return 0, nil
	}
	allowed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		entry, err := s.repo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			return 0, fmt.Errorf("load memory for bulk delete: %w", err)
		}
		if err := ensureExplorerAccess(entry, MemoryAccessRequest{ProjectID: projectID, RoleID: roleID}); err != nil {
			continue
		}
		allowed = append(allowed, id)
	}
	deleted, err := s.repo.DeleteMany(ctx, allowed)
	if err != nil {
		return 0, fmt.Errorf("bulk delete memories: %w", err)
	}
	return deleted, nil
}

func (s *MemoryExplorerService) CleanupEpisodic(ctx context.Context, input MemoryCleanupInput) (int64, error) {
	if s.episodic == nil {
		return 0, fmt.Errorf("episodic memory explorer is not configured")
	}
	before, err := resolveCleanupCutoff(input, s.now)
	if err != nil {
		return 0, err
	}
	entries, err := s.episodic.ListHistory(ctx, EpisodicMemoryQuery{
		ProjectID: input.ProjectID,
		Scope:     strings.TrimSpace(input.Scope),
		RoleID:    strings.TrimSpace(input.RoleID),
		EndAt:     before,
		Limit:     0,
	})
	if err != nil {
		return 0, fmt.Errorf("cleanup episodic memories: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	deleted, err := s.repo.DeleteMany(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("cleanup episodic memories: %w", err)
	}
	return deleted, nil
}

func (s *MemoryExplorerService) listEntries(ctx context.Context, query MemoryExplorerQuery) ([]*model.AgentMemory, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("memory explorer repository is required")
	}
	if strings.TrimSpace(query.Category) == model.MemoryCategoryEpisodic && s.episodic != nil {
		search := strings.TrimSpace(query.Query)
		episodicLimit := query.Limit
		if search != "" {
			episodicLimit = 0
		}
		entries, err := s.episodic.ListHistory(ctx, EpisodicMemoryQuery{
			ProjectID: query.ProjectID,
			Scope:     strings.TrimSpace(query.Scope),
			RoleID:    strings.TrimSpace(query.RoleID),
			StartAt:   query.StartAt,
			EndAt:     query.EndAt,
			Limit:     episodicLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("search memories: %w", err)
		}
		if search != "" {
			entries = filterMemoriesBySearch(entries, search)
			entries = limitExplorerEntries(entries, query.Limit)
		}
		return entries, nil
	}
	entries, err := s.repo.ListFiltered(ctx, query.ProjectID, model.AgentMemoryFilter{
		Query:    strings.TrimSpace(query.Query),
		Scope:    strings.TrimSpace(query.Scope),
		Category: strings.TrimSpace(query.Category),
		RoleID:   strings.TrimSpace(query.RoleID),
		StartAt:  query.StartAt,
		EndAt:    query.EndAt,
		Limit:    query.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	return filterAccessibleExplorerEntries(entries, MemoryAccessRequest{ProjectID: query.ProjectID, RoleID: query.RoleID}), nil
}

func limitExplorerEntries(entries []*model.AgentMemory, limit int) []*model.AgentMemory {
	if limit <= 0 || len(entries) <= limit {
		return entries
	}
	return entries[:limit]
}

func ensureExplorerAccess(entry *model.AgentMemory, access MemoryAccessRequest) error {
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

func filterAccessibleExplorerEntries(entries []*model.AgentMemory, access MemoryAccessRequest) []*model.AgentMemory {
	filtered := make([]*model.AgentMemory, 0, len(entries))
	for _, entry := range entries {
		if ensureExplorerAccess(entry, access) != nil {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func filterMemoriesBySearch(entries []*model.AgentMemory, query string) []*model.AgentMemory {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return entries
	}
	filtered := make([]*model.AgentMemory, 0, len(entries))
	for _, entry := range entries {
		searchBody := strings.ToLower(entry.Key + "\n" + entry.Content + "\n" + entry.Metadata)
		if strings.Contains(searchBody, query) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func resolveCleanupCutoff(input MemoryCleanupInput, now func() time.Time) (*time.Time, error) {
	if input.Before != nil && !input.Before.IsZero() {
		value := input.Before.UTC()
		return &value, nil
	}
	if input.RetentionDays > 0 {
		value := now().UTC().AddDate(0, 0, -input.RetentionDays)
		return &value, nil
	}
	return nil, fmt.Errorf("cleanup before or retentionDays is required")
}
