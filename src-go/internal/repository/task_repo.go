package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/react-go-quick-starter/server/internal/model"
)

type TaskRepository struct {
	db DBTX
}

func NewTaskRepository(db DBTX) *TaskRepository {
	return &TaskRepository{db: db}
}

const taskColumns = `tasks.id, tasks.project_id, tasks.parent_id, tasks.sprint_id, tasks.title, tasks.description, tasks.status, tasks.priority,
	tasks.assignee_id, tasks.assignee_type, tasks.reporter_id, tasks.labels, tasks.budget_usd, tasks.spent_usd,
	tasks.agent_branch, tasks.agent_worktree, tasks.agent_session_id, tasks.pr_url, tasks.pr_number,
	tasks.blocked_by, tasks.planned_start_at, tasks.planned_end_at, tasks.created_at, tasks.updated_at, tasks.completed_at,
	tps.last_activity_at, tps.last_activity_source, tps.last_transition_at, tps.health_status, tps.risk_reason,
	tps.risk_since_at, tps.last_alert_state, tps.last_alert_at, tps.last_recovered_at, tps.created_at, tps.updated_at`

func scanTask(row interface{ Scan(dest ...any) error }) (*model.Task, error) {
	t := &model.Task{}
	var (
		assigneeType       *string
		blockedByIDs       []uuid.UUID
		lastActivityAt     *time.Time
		lastActivitySource *string
		lastTransitionAt   *time.Time
		healthStatus       *string
		riskReason         *string
		riskSinceAt        *time.Time
		lastAlertState     *string
		lastAlertAt        *time.Time
		lastRecoveredAt    *time.Time
		progressCreatedAt  *time.Time
		progressUpdatedAt  *time.Time
	)
	err := row.Scan(
		&t.ID, &t.ProjectID, &t.ParentID, &t.SprintID, &t.Title, &t.Description,
		&t.Status, &t.Priority, &t.AssigneeID, &assigneeType, &t.ReporterID,
		&t.Labels, &t.BudgetUsd, &t.SpentUsd, &t.AgentBranch, &t.AgentWorktree,
		&t.AgentSessionID, &t.PRUrl, &t.PRNumber, &blockedByIDs, &t.PlannedStartAt, &t.PlannedEndAt,
		&t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
		&lastActivityAt, &lastActivitySource, &lastTransitionAt, &healthStatus, &riskReason,
		&riskSinceAt, &lastAlertState, &lastAlertAt, &lastRecoveredAt, &progressCreatedAt, &progressUpdatedAt,
	)
	if err == nil {
		if assigneeType != nil {
			t.AssigneeType = *assigneeType
		}
		t.BlockedBy = make([]string, 0, len(blockedByIDs))
		for _, id := range blockedByIDs {
			t.BlockedBy = append(t.BlockedBy, id.String())
		}
	}
	if err == nil && lastActivityAt != nil && lastActivitySource != nil && lastTransitionAt != nil && healthStatus != nil && riskReason != nil && lastAlertState != nil && progressCreatedAt != nil && progressUpdatedAt != nil {
		t.Progress = &model.TaskProgressSnapshot{
			TaskID:             t.ID,
			LastActivityAt:     *lastActivityAt,
			LastActivitySource: *lastActivitySource,
			LastTransitionAt:   *lastTransitionAt,
			HealthStatus:       *healthStatus,
			RiskReason:         *riskReason,
			RiskSinceAt:        riskSinceAt,
			LastAlertState:     *lastAlertState,
			LastAlertAt:        lastAlertAt,
			LastRecoveredAt:    lastRecoveredAt,
			CreatedAt:          *progressCreatedAt,
			UpdatedAt:          *progressUpdatedAt,
		}
	}
	return t, err
}

func (r *TaskRepository) Create(ctx context.Context, task *model.Task) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	blockedByIDs, err := normalizeTaskBlockedBy(task.BlockedBy)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	query := `
		INSERT INTO tasks (id, project_id, parent_id, sprint_id, title, description, status, priority,
			assignee_id, assignee_type, reporter_id, labels, budget_usd, spent_usd,
			agent_branch, agent_worktree, agent_session_id, pr_url, pr_number,
			blocked_by, planned_start_at, planned_end_at, created_at, updated_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,NOW(),NOW(),$23)
	`
	_, err = r.db.Exec(ctx, query,
		task.ID, task.ProjectID, task.ParentID, task.SprintID, task.Title, task.Description,
		task.Status, task.Priority, task.AssigneeID, nullableString(task.AssigneeType), task.ReporterID,
		task.Labels, task.BudgetUsd, task.SpentUsd, task.AgentBranch, task.AgentWorktree,
		task.AgentSessionID, task.PRUrl, task.PRNumber, blockedByIDs, task.PlannedStartAt, task.PlannedEndAt, task.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + taskColumns + ` FROM tasks LEFT JOIN task_progress_snapshots tps ON tps.task_id = tasks.id WHERE tasks.id = $1`
	t, err := scanTask(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("get task by id: %w", err)
	}
	return t, nil
}

func (r *TaskRepository) GetByPRURL(ctx context.Context, prURL string) (*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT ` + taskColumns + ` FROM tasks LEFT JOIN task_progress_snapshots tps ON tps.task_id = tasks.id WHERE tasks.pr_url = $1 ORDER BY tasks.updated_at DESC LIMIT 1`
	t, err := scanTask(r.db.QueryRow(ctx, query, prURL))
	if err != nil {
		return nil, fmt.Errorf("get task by pr url: %w", err)
	}
	return t, nil
}

func (r *TaskRepository) List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseUnavailable
	}

	var where []string
	var args []any
	argN := 1

	where = append(where, fmt.Sprintf("tasks.project_id = $%d", argN))
	args = append(args, projectID)
	argN++

	if q.Status != "" {
		where = append(where, fmt.Sprintf("tasks.status = $%d", argN))
		args = append(args, q.Status)
		argN++
	}
	if q.AssigneeID != "" {
		where = append(where, fmt.Sprintf("tasks.assignee_id = $%d", argN))
		args = append(args, q.AssigneeID)
		argN++
	}
	if q.SprintID != "" {
		where = append(where, fmt.Sprintf("tasks.sprint_id = $%d", argN))
		args = append(args, q.SprintID)
		argN++
	}
	if q.Priority != "" {
		where = append(where, fmt.Sprintf("tasks.priority = $%d", argN))
		args = append(args, q.Priority)
		argN++
	}
	if q.Search != "" {
		where = append(where, fmt.Sprintf("(tasks.title ILIKE $%d OR tasks.description ILIKE $%d)", argN, argN))
		args = append(args, "%"+q.Search+"%")
		argN++
	}

	whereClause := "WHERE " + strings.Join(where, " AND ")

	// Count total.
	countQuery := "SELECT COUNT(*) FROM tasks " + whereClause
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	// Sort.
	sort := "tasks.created_at DESC"
	if q.Sort != "" {
		sort = q.Sort
	}

	// Pagination.
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	listQuery := fmt.Sprintf("SELECT %s FROM tasks LEFT JOIN task_progress_snapshots tps ON tps.task_id = tasks.id %s ORDER BY %s LIMIT %d OFFSET %d",
		taskColumns, whereClause, sort, limit, offset)

	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, total, rows.Err()
}

func (r *TaskRepository) ListBySprint(ctx context.Context, projectID uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := `SELECT ` + taskColumns + ` FROM tasks
		LEFT JOIN task_progress_snapshots tps ON tps.task_id = tasks.id
		WHERE tasks.project_id = $1 AND tasks.sprint_id = $2
		ORDER BY tasks.created_at ASC`
	rows, err := r.db.Query(ctx, query, projectID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("list sprint tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]*model.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sprint task: %w", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	blockedByIDs, err := normalizeOptionalTaskBlockedBy(req.BlockedBy)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	query := `UPDATE tasks SET
		title = COALESCE($1, title),
		description = COALESCE($2, description),
		priority = COALESCE($3, priority),
		budget_usd = COALESCE($4, budget_usd),
		sprint_id = CASE
			WHEN $5::text IS NULL THEN sprint_id
			ELSE NULLIF($5::text, '')::uuid
		END,
		planned_start_at = CASE
			WHEN $6::text IS NULL THEN planned_start_at
			ELSE NULLIF($6::text, '')::timestamptz
		END,
		planned_end_at = CASE
			WHEN $7::text IS NULL THEN planned_end_at
			ELSE NULLIF($7::text, '')::timestamptz
		END,
		blocked_by = CASE
			WHEN $8::uuid[] IS NULL THEN blocked_by
			ELSE $8::uuid[]
		END,
		updated_at = NOW()
		WHERE id = $9`
	_, err = r.db.Exec(
		ctx,
		query,
		req.Title,
		req.Description,
		req.Priority,
		req.BudgetUsd,
		req.SprintID,
		req.PlannedStartAt,
		req.PlannedEndAt,
		blockedByIDs,
		id,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

func (r *TaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	_, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func (r *TaskRepository) TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	// Get current status first.
	var currentStatus string
	err := r.db.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1`, id).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("get task status: %w", err)
	}
	if err := model.ValidateTransition(currentStatus, newStatus); err != nil {
		return err
	}

	query := `UPDATE tasks SET status = $1, updated_at = NOW() WHERE id = $2`
	if newStatus == model.TaskStatusDone {
		query = `UPDATE tasks SET status = $1, updated_at = NOW(), completed_at = NOW() WHERE id = $2`
	}
	_, err = r.db.Exec(ctx, query, newStatus, id)
	if err != nil {
		return fmt.Errorf("transition task status: %w", err)
	}
	return nil
}

func (r *TaskRepository) UpdateAssignee(ctx context.Context, id uuid.UUID, assigneeID uuid.UUID, assigneeType string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE tasks SET assignee_id = $1, assignee_type = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.Exec(ctx, query, assigneeID, assigneeType, id)
	if err != nil {
		return fmt.Errorf("update task assignee: %w", err)
	}
	return nil
}

func (r *TaskRepository) UpdateRuntime(ctx context.Context, id uuid.UUID, branch, worktreePath, sessionID string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE tasks SET
		agent_branch = $1,
		agent_worktree = $2,
		agent_session_id = $3,
		updated_at = NOW()
		WHERE id = $4`
	_, err := r.db.Exec(ctx, query, branch, worktreePath, sessionID, id)
	if err != nil {
		return fmt.Errorf("update task runtime: %w", err)
	}
	return nil
}

func (r *TaskRepository) ClearRuntime(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE tasks SET
		agent_branch = '',
		agent_worktree = '',
		agent_session_id = '',
		updated_at = NOW()
		WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("clear task runtime: %w", err)
	}
	return nil
}

func (r *TaskRepository) UpdateSpent(ctx context.Context, id uuid.UUID, spentUsd float64, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE tasks SET
		spent_usd = $1,
		status = CASE
			WHEN $2 = '' THEN status
			ELSE $2
		END,
		updated_at = NOW()
		WHERE id = $3`
	_, err := r.db.Exec(ctx, query, spentUsd, status, id)
	if err != nil {
		return fmt.Errorf("update task spent: %w", err)
	}
	return nil
}

func (r *TaskRepository) ListOpenForProgress(ctx context.Context) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := `SELECT ` + taskColumns + ` FROM tasks
		LEFT JOIN task_progress_snapshots tps ON tps.task_id = tasks.id
		WHERE tasks.status NOT IN ($1, $2)
		ORDER BY tasks.updated_at DESC`
	rows, err := r.db.Query(ctx, query, model.TaskStatusDone, model.TaskStatusCancelled)
	if err != nil {
		return nil, fmt.Errorf("list open tasks for progress: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task for progress: %w", err)
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) HasChildren(ctx context.Context, parentID uuid.UUID) (bool, error) {
	if r.db == nil {
		return false, ErrDatabaseUnavailable
	}
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM tasks WHERE parent_id = $1`, parentID).Scan(&count); err != nil {
		return false, fmt.Errorf("count child tasks: %w", err)
	}
	return count > 0, nil
}

func (r *TaskRepository) ListDependents(ctx context.Context, blockerID uuid.UUID) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	query := `SELECT ` + taskColumns + ` FROM tasks
		LEFT JOIN task_progress_snapshots tps ON tps.task_id = tasks.id
		WHERE $1 = ANY(tasks.blocked_by)
		ORDER BY tasks.updated_at DESC`
	rows, err := r.db.Query(ctx, query, blockerID)
	if err != nil {
		return nil, fmt.Errorf("list dependents: %w", err)
	}
	defer rows.Close()

	dependents := make([]*model.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dependent task: %w", err)
		}
		dependents = append(dependents, task)
	}
	return dependents, rows.Err()
}

func (r *TaskRepository) CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if len(inputs) == 0 {
		return []*model.Task{}, nil
	}

	type beginner interface {
		Begin(context.Context) (pgx.Tx, error)
	}

	var (
		executor DBTX = r.db
		tx       pgx.Tx
		err      error
		commit   bool
	)
	if b, ok := r.db.(beginner); ok {
		tx, err = b.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin child task transaction: %w", err)
		}
		executor = tx
		defer func() {
			if !commit {
				_ = tx.Rollback(ctx)
			}
		}()
	}

	created := make([]*model.Task, 0, len(inputs))
	query := `
		INSERT INTO tasks (id, project_id, parent_id, sprint_id, title, description, status, priority,
			assignee_id, assignee_type, reporter_id, labels, budget_usd, spent_usd,
			agent_branch, agent_worktree, agent_session_id, pr_url, pr_number,
			blocked_by, planned_start_at, planned_end_at, created_at, updated_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,NOW(),NOW(),$23)
	`

	for _, input := range inputs {
		task := &model.Task{
			ID:          uuid.New(),
			ProjectID:   input.ProjectID,
			ParentID:    &input.ParentID,
			SprintID:    input.SprintID,
			Title:       input.Title,
			Description: input.Description,
			Status:      model.TaskStatusInbox,
			Priority:    input.Priority,
			ReporterID:  input.ReporterID,
			Labels:      append([]string(nil), input.Labels...),
			BudgetUsd:   input.BudgetUSD,
			BlockedBy:   []string{},
		}
		blockedByIDs, err := normalizeTaskBlockedBy(task.BlockedBy)
		if err != nil {
			return nil, fmt.Errorf("create child task: %w", err)
		}

		_, err = executor.Exec(ctx, query,
			task.ID, task.ProjectID, task.ParentID, task.SprintID, task.Title, task.Description,
			task.Status, task.Priority, task.AssigneeID, nullableString(task.AssigneeType), task.ReporterID,
			task.Labels, task.BudgetUsd, task.SpentUsd, task.AgentBranch, task.AgentWorktree,
			task.AgentSessionID, task.PRUrl, task.PRNumber, blockedByIDs, task.PlannedStartAt, task.PlannedEndAt, task.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("create child task: %w", err)
		}
		created = append(created, task)
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit child task transaction: %w", err)
		}
		commit = true
	}

	return created, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func normalizeOptionalTaskBlockedBy(blockedBy *[]string) (any, error) {
	if blockedBy == nil {
		return nil, nil
	}
	return normalizeTaskBlockedBy(*blockedBy)
}

// TaskDateCount holds a count for a specific date.
type TaskDateCount struct {
	Date  time.Time
	Count int
}

func (r *TaskRepository) CountCompletedByDateRange(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]TaskDateCount, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT COALESCE(completed_at, updated_at)::date AS d, COUNT(*) AS cnt
		FROM tasks
		WHERE status = 'done'
		AND COALESCE(completed_at, updated_at) BETWEEN $1 AND $2`
	args := []interface{}{from, to}
	if projectID != nil {
		query += ` AND project_id = $3`
		args = append(args, *projectID)
	}
	query += ` GROUP BY d ORDER BY d`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count completed by date range: %w", err)
	}
	defer rows.Close()

	var results []TaskDateCount
	for rows.Next() {
		var tc TaskDateCount
		if err := rows.Scan(&tc.Date, &tc.Count); err != nil {
			return nil, fmt.Errorf("scan task date count: %w", err)
		}
		results = append(results, tc)
	}
	return results, rows.Err()
}

func normalizeTaskBlockedBy(blockedBy []string) ([]uuid.UUID, error) {
	if len(blockedBy) == 0 {
		return []uuid.UUID{}, nil
	}

	ids := make([]uuid.UUID, 0, len(blockedBy))
	for _, rawID := range blockedBy {
		trimmed := strings.TrimSpace(rawID)
		if trimmed == "" {
			return nil, fmt.Errorf("blockedBy contains an empty task id")
		}
		parsedID, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, fmt.Errorf("blockedBy contains an invalid task id %q: %w", rawID, err)
		}
		ids = append(ids, parsedID)
	}
	return ids, nil
}
