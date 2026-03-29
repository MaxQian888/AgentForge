package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type QueueAgentAdmissionRecord struct {
	ProjectID uuid.UUID
	TaskID    uuid.UUID
	MemberID  uuid.UUID
	Runtime   string
	Provider  string
	Model     string
	RoleID    string
	Priority  int
	BudgetUSD float64
	Reason    string
}

type AgentPoolQueueRepository struct {
	db      *gorm.DB
	mu      sync.RWMutex
	entries map[string]*model.AgentPoolQueueEntry
}

func NewAgentPoolQueueRepository(db ...*gorm.DB) *AgentPoolQueueRepository {
	var conn *gorm.DB
	if len(db) > 0 {
		conn = db[0]
	}
	return &AgentPoolQueueRepository{
		db:      conn,
		entries: make(map[string]*model.AgentPoolQueueEntry),
	}
}

func (r *AgentPoolQueueRepository) QueueAgentAdmission(ctx context.Context, input QueueAgentAdmissionRecord) (*model.AgentPoolQueueEntry, error) {
	entry := &model.AgentPoolQueueEntry{
		EntryID:   uuid.NewString(),
		ProjectID: input.ProjectID.String(),
		TaskID:    input.TaskID.String(),
		MemberID:  input.MemberID.String(),
		Status:    model.AgentPoolQueueStatusQueued,
		Reason:    input.Reason,
		Runtime:   input.Runtime,
		Provider:  input.Provider,
		Model:     input.Model,
		RoleID:    input.RoleID,
		Priority:  input.Priority,
		BudgetUSD: input.BudgetUSD,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.entries[entry.EntryID] = cloneAgentPoolQueueEntry(entry)
		return cloneAgentPoolQueueEntry(entry), nil
	}

	if err := r.db.WithContext(ctx).Create(newAgentPoolQueueEntryRecord(entry)).Error; err != nil {
		return nil, fmt.Errorf("queue agent admission: %w", err)
	}
	return entry, nil
}

func (r *AgentPoolQueueRepository) CountQueuedByProject(ctx context.Context, projectID uuid.UUID) (int, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		count := 0
		for _, entry := range r.entries {
			if entry.ProjectID == projectID.String() && entry.Status == model.AgentPoolQueueStatusQueued {
				count++
			}
		}
		return count, nil
	}

	var count int64
	if err := r.db.WithContext(ctx).
		Model(&agentPoolQueueEntryRecord{}).
		Where("project_id = ? AND status = ?", projectID.String(), model.AgentPoolQueueStatusQueued).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count queued entries: %w", err)
	}
	return int(count), nil
}

func (r *AgentPoolQueueRepository) ListAllQueued(ctx context.Context, limit int) ([]*model.AgentPoolQueueEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		entries := make([]*model.AgentPoolQueueEntry, 0, len(r.entries))
		for _, entry := range r.entries {
			if entry.Status != model.AgentPoolQueueStatusQueued {
				continue
			}
			entries = append(entries, cloneAgentPoolQueueEntry(entry))
		}
		slices.SortFunc(entries, compareQueueEntries)
		if len(entries) > limit {
			entries = entries[:limit]
		}
		return entries, nil
	}

	var rows []agentPoolQueueEntryRecord
	if err := r.db.WithContext(ctx).
		Where("status = ?", model.AgentPoolQueueStatusQueued).
		Order("priority DESC").
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list all queued entries: %w", err)
	}
	return toAgentPoolQueueEntries(rows), nil
}

func (r *AgentPoolQueueRepository) ListQueuedByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]*model.AgentPoolQueueEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		entries := make([]*model.AgentPoolQueueEntry, 0)
		for _, entry := range r.entries {
			if entry.ProjectID == projectID.String() && entry.Status == model.AgentPoolQueueStatusQueued {
				entries = append(entries, cloneAgentPoolQueueEntry(entry))
			}
		}
		slices.SortFunc(entries, compareQueueEntries)
		if len(entries) > limit {
			entries = entries[:limit]
		}
		return entries, nil
	}

	var rows []agentPoolQueueEntryRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND status = ?", projectID.String(), model.AgentPoolQueueStatusQueued).
		Order("priority DESC").
		Order("created_at ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list queued entries by project: %w", err)
	}
	return toAgentPoolQueueEntries(rows), nil
}

func (r *AgentPoolQueueRepository) ReserveNextQueuedByProject(ctx context.Context, projectID uuid.UUID) (*model.AgentPoolQueueEntry, error) {
	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		entries := make([]*model.AgentPoolQueueEntry, 0)
		for _, entry := range r.entries {
			if entry.ProjectID == projectID.String() && entry.Status == model.AgentPoolQueueStatusQueued {
				entries = append(entries, entry)
			}
		}
		slices.SortFunc(entries, compareQueueEntries)
		if len(entries) == 0 {
			return nil, nil
		}
		entries[0].Status = model.AgentPoolQueueStatusAdmitted
		entries[0].UpdatedAt = time.Now().UTC()
		return cloneAgentPoolQueueEntry(entries[0]), nil
	}

	var row agentPoolQueueEntryRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND status = ?", projectID.String(), model.AgentPoolQueueStatusQueued).
		Order("priority DESC").
		Order("created_at ASC").
		Take(&row).Error; err != nil {
		if errors.Is(normalizeRepositoryError(err), ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("reserve queued entry: %w", err)
	}
	entry := row.toModel()

	if err := r.db.WithContext(ctx).
		Model(&agentPoolQueueEntryRecord{}).
		Where("entry_id = ?", entry.EntryID).
		Updates(map[string]any{
			"status":     model.AgentPoolQueueStatusAdmitted,
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		return nil, fmt.Errorf("mark queued entry admitted: %w", err)
	}
	entry.Status = model.AgentPoolQueueStatusAdmitted
	entry.UpdatedAt = time.Now().UTC()
	return entry, nil
}

func (r *AgentPoolQueueRepository) CompleteQueuedEntry(ctx context.Context, entryID string, status model.AgentPoolQueueStatus, reason string, runID *uuid.UUID) error {
	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		entry, ok := r.entries[entryID]
		if !ok {
			return ErrNotFound
		}
		entry.Status = status
		entry.Reason = reason
		if runID != nil {
			value := runID.String()
			entry.AgentRunID = &value
		}
		entry.UpdatedAt = time.Now().UTC()
		return nil
	}

	updates := map[string]any{
		"status":     status,
		"reason":     reason,
		"updated_at": time.Now().UTC(),
	}
	if runID != nil {
		updates["agent_run_id"] = runID.String()
	} else {
		updates["agent_run_id"] = nil
	}
	result := r.db.WithContext(ctx).
		Model(&agentPoolQueueEntryRecord{}).
		Where("entry_id = ?", entryID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("complete queued entry: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AgentPoolQueueRepository) ListRecentByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]*model.AgentPoolQueueEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		entries := make([]*model.AgentPoolQueueEntry, 0)
		for _, entry := range r.entries {
			if entry.ProjectID == projectID.String() {
				entries = append(entries, cloneAgentPoolQueueEntry(entry))
			}
		}
		slices.SortFunc(entries, func(a, b *model.AgentPoolQueueEntry) int {
			switch {
			case a.UpdatedAt.After(b.UpdatedAt):
				return -1
			case a.UpdatedAt.Before(b.UpdatedAt):
				return 1
			default:
				return compareQueueEntries(a, b)
			}
		})
		if len(entries) > limit {
			entries = entries[:limit]
		}
		return entries, nil
	}

	var rows []agentPoolQueueEntryRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID.String()).
		Order("updated_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list recent queue entries by project: %w", err)
	}
	return toAgentPoolQueueEntries(rows), nil
}

func toAgentPoolQueueEntries(rows []agentPoolQueueEntryRecord) []*model.AgentPoolQueueEntry {
	entries := make([]*model.AgentPoolQueueEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, row.toModel())
	}
	return entries
}

func cloneAgentPoolQueueEntry(entry *model.AgentPoolQueueEntry) *model.AgentPoolQueueEntry {
	if entry == nil {
		return nil
	}
	cloned := *entry
	if entry.AgentRunID != nil {
		value := *entry.AgentRunID
		cloned.AgentRunID = &value
	}
	return &cloned
}

func compareQueueEntries(a, b *model.AgentPoolQueueEntry) int {
	switch {
	case a.Priority > b.Priority:
		return -1
	case a.Priority < b.Priority:
		return 1
	case a.CreatedAt.Before(b.CreatedAt):
		return -1
	case a.CreatedAt.After(b.CreatedAt):
		return 1
	default:
		return 0
	}
}
