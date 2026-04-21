package repository

import (
	"context"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AgentTeamRepository struct {
	db *gorm.DB
}

type teamSummaryEnvelope struct {
	team    *model.AgentTeam
	summary *model.AgentTeamSummaryDTO
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

func (r *AgentTeamRepository) GetTeamSummary(ctx context.Context, id uuid.UUID) (*model.AgentTeamSummaryDTO, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	team, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get agent team summary: %w", err)
	}

	summaries, err := r.buildTeamSummaries(ctx, []*model.AgentTeam{team})
	if err != nil {
		return nil, fmt.Errorf("get agent team summary: %w", err)
	}
	if len(summaries) == 0 {
		return nil, ErrNotFound
	}
	return summaries[0], nil
}

func (r *AgentTeamRepository) ListByProject(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeam, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var records []agentTeamRecord
	if err := query.Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent teams by project: %w", err)
	}
	teams := make([]*model.AgentTeam, len(records))
	for i := range records {
		teams[i] = records[i].toModel()
	}
	return teams, nil
}

func (r *AgentTeamRepository) ListTeamSummaries(ctx context.Context, projectID uuid.UUID, status string) ([]*model.AgentTeamSummaryDTO, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	teams, err := r.ListByProject(ctx, projectID, status)
	if err != nil {
		return nil, fmt.Errorf("list agent team summaries by project: %w", err)
	}

	return r.buildTeamSummaries(ctx, teams)
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

func (r *AgentTeamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).Delete(&agentTeamRecord{}, "id = ? AND status IN ?", id, []string{"completed", "failed", "cancelled"})
	if result.Error != nil {
		return fmt.Errorf("delete agent team: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AgentTeamRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateTeamRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.TotalBudgetUsd != nil {
		updates["total_budget_usd"] = *req.TotalBudgetUsd
	}
	if len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = gorm.Expr("NOW()")
	if err := r.db.WithContext(ctx).Model(&agentTeamRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update agent team: %w", err)
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

func (r *AgentTeamRepository) SetWorkflowExecutionID(ctx context.Context, id uuid.UUID, execID uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentTeamRecord{}).Where("id = ?", id).Updates(map[string]any{
		"workflow_execution_id": execID,
		"updated_at":            gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("set workflow execution ID: %w", err)
	}
	return nil
}

func (r *AgentTeamRepository) buildTeamSummaries(ctx context.Context, teams []*model.AgentTeam) ([]*model.AgentTeamSummaryDTO, error) {
	summaries := make([]*model.AgentTeamSummaryDTO, 0, len(teams))
	if len(teams) == 0 {
		return summaries, nil
	}

	taskTitles, err := r.loadTaskTitles(ctx, teams)
	if err != nil {
		return nil, err
	}

	teamIDs := make([]uuid.UUID, 0, len(teams))
	envelopes := make(map[uuid.UUID]*teamSummaryEnvelope, len(teams))
	for _, team := range teams {
		summary := &model.AgentTeamSummaryDTO{
			AgentTeamDTO: team.ToDTO(),
			TaskTitle:    taskTitles[team.TaskID],
			CoderRuns:    make([]model.AgentRunDTO, 0),
		}
		teamIDs = append(teamIDs, team.ID)
		envelopes[team.ID] = &teamSummaryEnvelope{
			team:    team,
			summary: summary,
		}
		summaries = append(summaries, summary)
	}

	var runRecords []agentRunRecord
	if err := r.db.WithContext(ctx).
		Where("team_id IN ?", teamIDs).
		Order("created_at ASC").
		Find(&runRecords).Error; err != nil {
		return nil, fmt.Errorf("list team runs for summaries: %w", err)
	}

	for _, runRecord := range runRecords {
		if runRecord.TeamID == nil {
			continue
		}
		envelope := envelopes[*runRecord.TeamID]
		if envelope == nil {
			continue
		}

		switch runRecord.TeamRole {
		case model.TeamRolePlanner:
			if (envelope.team.PlannerRunID != nil && runRecord.ID == *envelope.team.PlannerRunID) ||
				envelope.summary.PlannerStatus == "" {
				envelope.summary.PlannerStatus = runRecord.Status
			}
		case model.TeamRoleCoder:
			envelope.summary.CoderRuns = append(envelope.summary.CoderRuns, runRecord.toModel().ToDTO())
			envelope.summary.CoderTotal++
			if runRecord.Status == model.AgentRunStatusCompleted {
				envelope.summary.CoderCompleted++
			}
		case model.TeamRoleReviewer:
			if (envelope.team.ReviewerRunID != nil && runRecord.ID == *envelope.team.ReviewerRunID) ||
				envelope.summary.ReviewerStatus == "" {
				envelope.summary.ReviewerStatus = runRecord.Status
			}
		}
	}

	return summaries, nil
}

func (r *AgentTeamRepository) loadTaskTitles(ctx context.Context, teams []*model.AgentTeam) (map[uuid.UUID]string, error) {
	taskIDs := make([]uuid.UUID, 0, len(teams))
	for _, team := range teams {
		taskIDs = append(taskIDs, team.TaskID)
	}

	type taskTitleRow struct {
		ID    uuid.UUID `gorm:"column:id"`
		Title string    `gorm:"column:title"`
	}

	var rows []taskTitleRow
	if err := r.db.WithContext(ctx).
		Model(&taskRecord{}).
		Select("id, title").
		Where("id IN ?", taskIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list task titles for team summaries: %w", err)
	}

	titles := make(map[uuid.UUID]string, len(rows))
	for _, row := range rows {
		titles[row.ID] = row.Title
	}
	return titles, nil
}
