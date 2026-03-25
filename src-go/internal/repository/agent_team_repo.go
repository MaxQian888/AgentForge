package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type AgentTeamRepository struct {
	db *gorm.DB
}

func NewAgentTeamRepository(db *gorm.DB) *AgentTeamRepository {
	return &AgentTeamRepository{db: db}
}

func (r *AgentTeamRepository) Create(ctx context.Context, team *model.AgentTeam) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newAgentTeamRecord(team)).Error; err != nil {
		return fmt.Errorf("create agent team: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record agentTeamRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get agent team by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *AgentTeamRepository) GetByTask(ctx context.Context, taskID uuid.UUID) (*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record agentTeamRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at DESC").Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get agent team by task: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *AgentTeamRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []agentTeamRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent teams by project: %w", err)
	}
	teams := make([]*model.AgentTeam, len(records))
	for i := range records {
		teams[i] = records[i].toModel()
	}
	return teams, nil
}

func (r *AgentTeamRepository) ListActive(ctx context.Context) ([]*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []agentTeamRecord
	if err := r.db.WithContext(ctx).Where("status IN ?", []string{"pending", "planning", "executing", "reviewing"}).Order("created_at").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list active agent teams: %w", err)
	}
	teams := make([]*model.AgentTeam, len(records))
	for i := range records {
		teams[i] = records[i].toModel()
	}
	return teams, nil
}

func (r *AgentTeamRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentTeamRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update agent team status: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) UpdateStatusWithError(ctx context.Context, id uuid.UUID, status, errorMessage string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentTeamRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"error_message": errorMessage,
		"updated_at":    gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update agent team status with error: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) UpdateSpent(ctx context.Context, id uuid.UUID, spent float64) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentTeamRecord{}).Where("id = ?", id).Updates(map[string]any{
		"total_spent_usd": spent,
		"updated_at":      gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update agent team spent: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) SetPlannerRun(ctx context.Context, id uuid.UUID, plannerRunID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentTeamRecord{}).Where("id = ?", id).Updates(map[string]any{
		"planner_run_id": plannerRunID,
		"updated_at":     gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("set agent team planner run: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) SetReviewerRun(ctx context.Context, id uuid.UUID, reviewerRunID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentTeamRecord{}).Where("id = ?", id).Updates(map[string]any{
		"reviewer_run_id": reviewerRunID,
		"updated_at":      gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("set agent team reviewer run: %w", err)
	}
	return nil
}
