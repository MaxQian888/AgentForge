package service

import (
	"context"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type MemoryAPIService struct {
	memory   *MemoryService
	explorer *MemoryExplorerService
}

func NewMemoryAPIService(memory *MemoryService, explorer *MemoryExplorerService) *MemoryAPIService {
	return &MemoryAPIService{memory: memory, explorer: explorer}
}

func (s *MemoryAPIService) Store(ctx context.Context, input StoreMemoryInput) (*model.AgentMemory, error) {
	return s.memory.Store(ctx, input)
}

func (s *MemoryAPIService) Update(ctx context.Context, input UpdateMemoryInput) (*model.AgentMemory, error) {
	return s.memory.Update(ctx, input)
}

func (s *MemoryAPIService) Search(ctx context.Context, query MemoryExplorerQuery) ([]model.AgentMemoryDTO, error) {
	return s.explorer.Search(ctx, query)
}

func (s *MemoryAPIService) Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID, roleID string) (*model.AgentMemoryDetailDTO, error) {
	return s.explorer.Get(ctx, projectID, id, roleID)
}

func (s *MemoryAPIService) Stats(ctx context.Context, query MemoryExplorerQuery) (*model.MemoryExplorerStatsDTO, error) {
	return s.explorer.Stats(ctx, query)
}

func (s *MemoryAPIService) ExportEpisodic(ctx context.Context, query MemoryExplorerQuery) (*EpisodicMemoryExport, error) {
	return s.explorer.ExportEpisodic(ctx, query)
}

func (s *MemoryAPIService) BulkDelete(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID, roleID string) (int64, error) {
	return s.explorer.BulkDelete(ctx, projectID, ids, roleID)
}

func (s *MemoryAPIService) CleanupEpisodic(ctx context.Context, input MemoryCleanupInput) (int64, error) {
	return s.explorer.CleanupEpisodic(ctx, input)
}

func (s *MemoryAPIService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.memory.Delete(ctx, id)
}
