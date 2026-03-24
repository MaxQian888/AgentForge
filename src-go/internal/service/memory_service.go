package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// MemoryRepository defines persistence for agent memories.
type MemoryRepository interface {
	Create(ctx context.Context, mem *model.AgentMemory) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentMemory, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, scope, category string) ([]*model.AgentMemory, error)
	Search(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]*model.AgentMemory, error)
	IncrementAccess(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// StoreMemoryInput is the input for storing a new memory entry.
type StoreMemoryInput struct {
	ProjectID      uuid.UUID `json:"projectId"`
	Scope          string    `json:"scope"`
	RoleID         string    `json:"roleId"`
	Category       string    `json:"category"`
	Key            string    `json:"key"`
	Content        string    `json:"content"`
	Metadata       string    `json:"metadata"`
	RelevanceScore float64   `json:"relevanceScore"`
}

type MemoryService struct {
	repo MemoryRepository
}

func NewMemoryService(repo MemoryRepository) *MemoryService {
	return &MemoryService{repo: repo}
}

// Store creates a new memory entry.
func (s *MemoryService) Store(ctx context.Context, input StoreMemoryInput) (*model.AgentMemory, error) {
	mem := &model.AgentMemory{
		ID:             uuid.New(),
		ProjectID:      input.ProjectID,
		Scope:          input.Scope,
		RoleID:         input.RoleID,
		Category:       input.Category,
		Key:            input.Key,
		Content:        input.Content,
		Metadata:       input.Metadata,
		RelevanceScore: input.RelevanceScore,
	}
	if strings.TrimSpace(mem.Scope) == "" {
		mem.Scope = model.MemoryScopeProject
	}
	if strings.TrimSpace(mem.Category) == "" {
		mem.Category = model.MemoryCategoryEpisodic
	}
	if mem.RelevanceScore <= 0 {
		mem.RelevanceScore = 1.0
	}

	if err := s.repo.Create(ctx, mem); err != nil {
		return nil, fmt.Errorf("store memory: %w", err)
	}
	return mem, nil
}

// Search finds memories matching a query string.
func (s *MemoryService) Search(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]model.AgentMemoryDTO, error) {
	memories, err := s.repo.Search(ctx, projectID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}

	dtos := make([]model.AgentMemoryDTO, 0, len(memories))
	for _, m := range memories {
		_ = s.repo.IncrementAccess(ctx, m.ID)
		dtos = append(dtos, m.ToDTO())
	}
	return dtos, nil
}

// InjectContext fetches recent relevant memories and formats them as system prompt context.
func (s *MemoryService) InjectContext(ctx context.Context, projectID uuid.UUID, roleID string) (string, error) {
	memories, err := s.repo.ListByProject(ctx, projectID, "", "")
	if err != nil {
		return "", fmt.Errorf("inject context: %w", err)
	}

	if len(memories) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Project Memory Context\n")
	count := 0
	for _, m := range memories {
		if count >= 10 {
			break
		}
		// Filter by role if specified
		if strings.TrimSpace(roleID) != "" && m.RoleID != "" && m.RoleID != roleID && m.Scope == model.MemoryScopeRole {
			continue
		}
		sb.WriteString(fmt.Sprintf("- [%s/%s] %s: %s\n", m.Scope, m.Category, m.Key, m.Content))
		_ = s.repo.IncrementAccess(ctx, m.ID)
		count++
	}

	if count == 0 {
		return "", nil
	}
	return sb.String(), nil
}

// Delete removes a memory entry by ID.
func (s *MemoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// RecordTeamLearnings extracts key learnings from a completed team run and stores them.
func (s *MemoryService) RecordTeamLearnings(ctx context.Context, projectID uuid.UUID, team *model.AgentTeam, runs []*model.AgentRun) error {
	if team == nil {
		return nil
	}

	var totalCost float64
	var totalTurns int
	coderCount := 0
	for _, r := range runs {
		totalCost += r.CostUsd
		totalTurns += r.TurnCount
		if r.TeamRole == model.TeamRoleCoder {
			coderCount++
		}
	}

	content := fmt.Sprintf("Team '%s' completed with strategy '%s'. %d coder(s), total cost $%.4f, %d turns.",
		team.Name, team.Strategy, coderCount, totalCost, totalTurns)

	_, err := s.Store(ctx, StoreMemoryInput{
		ProjectID:      projectID,
		Scope:          model.MemoryScopeProject,
		Category:       model.MemoryCategoryEpisodic,
		Key:            fmt.Sprintf("team-completion-%s", team.ID.String()[:8]),
		Content:        content,
		RelevanceScore: 0.8,
	})
	return err
}
