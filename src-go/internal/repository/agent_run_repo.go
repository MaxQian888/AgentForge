package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type AgentRunRepository struct {
	db *gorm.DB
}

func NewAgentRunRepository(db *gorm.DB) *AgentRunRepository {
	return &AgentRunRepository{db: db}
}

func (r *AgentRunRepository) Create(ctx context.Context, run *model.AgentRun) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newAgentRunRecord(run)).Error; err != nil {
		return fmt.Errorf("create agent run: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record agentRunRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get agent run by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *AgentRunRepository) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []agentRunRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("get agent runs by task: %w", err)
	}
	runs := make([]*model.AgentRun, len(records))
	for i := range records {
		runs[i] = records[i].toModel()
	}
	return runs, nil
}

func (r *AgentRunRepository) ListActive(ctx context.Context) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []agentRunRecord
	if err := r.db.WithContext(ctx).Where("status IN ?", []string{"starting", "running", "paused"}).Order("created_at").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list active agent runs: %w", err)
	}
	runs := make([]*model.AgentRun, len(records))
	for i := range records {
		runs[i] = records[i].toModel()
	}
	return runs, nil
}

func (r *AgentRunRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentRunRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":     status,
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update agent run status: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []agentRunRecord
	if err := r.db.WithContext(ctx).
		Joins("JOIN tasks t ON t.id = agent_runs.task_id").
		Where("t.project_id = ?", projectID).
		Order("agent_runs.created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent runs by project: %w", err)
	}
	runs := make([]*model.AgentRun, len(records))
	for i := range records {
		runs[i] = records[i].toModel()
	}
	return runs, nil
}

func (r *AgentRunRepository) ListBySprint(ctx context.Context, sprintID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []agentRunRecord
	if err := r.db.WithContext(ctx).
		Joins("JOIN tasks t ON t.id = agent_runs.task_id").
		Where("t.sprint_id = ?", sprintID).
		Order("agent_runs.created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent runs by sprint: %w", err)
	}
	runs := make([]*model.AgentRun, len(records))
	for i := range records {
		runs[i] = records[i].toModel()
	}
	return runs, nil
}

func (r *AgentRunRepository) ListByTeam(ctx context.Context, teamID uuid.UUID) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []agentRunRecord
	if err := r.db.WithContext(ctx).Where("team_id = ?", teamID).Order("created_at").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent runs by team: %w", err)
	}
	runs := make([]*model.AgentRun, len(records))
	for i := range records {
		runs[i] = records[i].toModel()
	}
	return runs, nil
}

func (r *AgentRunRepository) ListByRole(ctx context.Context, roleID string, limit int) ([]*model.AgentRun, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	var records []agentRunRecord
	if err := r.db.WithContext(ctx).
		Where("role_id = ?", roleID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list agent runs by role: %w", err)
	}
	runs := make([]*model.AgentRun, len(records))
	for i := range records {
		runs[i] = records[i].toModel()
	}
	return runs, nil
}

func (r *AgentRunRepository) SetTeamFields(ctx context.Context, id uuid.UUID, teamID uuid.UUID, teamRole string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentRunRecord{}).Where("id = ?", id).Updates(map[string]any{
		"team_id":    teamID,
		"team_role":  teamRole,
		"updated_at": gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("set agent run team fields: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) UpdateCost(ctx context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int, costAccounting *model.CostAccountingSnapshot) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"input_tokens":      inputTokens,
		"output_tokens":     outputTokens,
		"cache_read_tokens": cacheReadTokens,
		"cost_usd":          costUsd,
		"turn_count":        turnCount,
		"updated_at":        gorm.Expr("NOW()"),
	}
	updates["cost_accounting"] = mustMarshalAgentRunCostAccounting(costAccounting)
	if err := r.db.WithContext(ctx).Model(&agentRunRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update agent run cost: %w", err)
	}
	return nil
}

func (r *AgentRunRepository) UpdateStructuredOutput(ctx context.Context, id uuid.UUID, output json.RawMessage) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&agentRunRecord{}).Where("id = ?", id).Update("structured_output", string(output)).Error; err != nil {
		return fmt.Errorf("update agent run structured output: %w", err)
	}
	return nil
}

// --- Raw SQL for complex aggregates ---

func (r *AgentRunRepository) AggregateByProject(ctx context.Context, projectID uuid.UUID) (*model.CostSummaryDTO, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT
		COALESCE(SUM(ar.cost_usd), 0),
		COALESCE(SUM(ar.input_tokens), 0),
		COALESCE(SUM(ar.output_tokens), 0),
		COALESCE(SUM(ar.cache_read_tokens), 0),
		COALESCE(SUM(ar.turn_count), 0),
		COUNT(ar.id)
		FROM agent_runs ar
		JOIN tasks t ON ar.task_id = t.id
		WHERE t.project_id = $1`
	s := &model.CostSummaryDTO{}
	err := r.db.WithContext(ctx).Raw(query, projectID).Row().Scan(
		&s.TotalCostUsd, &s.TotalInputTokens, &s.TotalOutputTokens,
		&s.TotalCacheReadTokens, &s.TotalTurns, &s.RunCount,
	)
	if err != nil {
		return nil, fmt.Errorf("aggregate cost by project: %w", err)
	}
	return s, nil
}

// AgentPerformanceRow holds aggregated performance data for a role.
type AgentPerformanceRow struct {
	RoleID         string
	TotalRuns      int
	CompletedRuns  int
	AvgCostUsd     float64
	AvgDurationSec float64
	TotalCostUsd   float64
}

func (r *AgentRunRepository) AggregatePerformance(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]AgentPerformanceRow, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT
		ar.role_id,
		COUNT(ar.id) AS total_runs,
		COUNT(ar.id) FILTER (WHERE ar.status = 'completed') AS completed_runs,
		COALESCE(AVG(ar.cost_usd), 0) AS avg_cost,
		COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(ar.completed_at, NOW()) - ar.started_at))), 0) AS avg_duration,
		COALESCE(SUM(ar.cost_usd), 0) AS total_cost
	FROM agent_runs ar`
	args := []interface{}{from, to}
	if projectID != nil {
		query += ` JOIN tasks t ON ar.task_id = t.id WHERE ar.created_at BETWEEN $1 AND $2 AND t.project_id = $3`
		args = append(args, *projectID)
	} else {
		query += ` WHERE ar.created_at BETWEEN $1 AND $2`
	}
	query += ` GROUP BY ar.role_id ORDER BY total_runs DESC`

	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("aggregate agent performance: %w", err)
	}
	defer rows.Close()

	var results []AgentPerformanceRow
	for rows.Next() {
		var row AgentPerformanceRow
		if err := rows.Scan(&row.RoleID, &row.TotalRuns, &row.CompletedRuns, &row.AvgCostUsd, &row.AvgDurationSec, &row.TotalCostUsd); err != nil {
			return nil, fmt.Errorf("scan agent performance row: %w", err)
		}
		results = append(results, row)
	}
	return results, rows.Err()
}
