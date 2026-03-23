package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type TaskRepository struct {
	db DBTX
}

func NewTaskRepository(db DBTX) *TaskRepository {
	return &TaskRepository{db: db}
}

const taskColumns = `id, project_id, parent_id, sprint_id, title, description, status, priority,
	assignee_id, assignee_type, reporter_id, labels, budget_usd, spent_usd,
	agent_branch, agent_worktree, agent_session_id, pr_url, pr_number,
	blocked_by, created_at, updated_at, completed_at`

func scanTask(row interface{ Scan(dest ...any) error }) (*model.Task, error) {
	t := &model.Task{}
	err := row.Scan(
		&t.ID, &t.ProjectID, &t.ParentID, &t.SprintID, &t.Title, &t.Description,
		&t.Status, &t.Priority, &t.AssigneeID, &t.AssigneeType, &t.ReporterID,
		&t.Labels, &t.BudgetUsd, &t.SpentUsd, &t.AgentBranch, &t.AgentWorktree,
		&t.AgentSessionID, &t.PRUrl, &t.PRNumber, &t.BlockedBy,
		&t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
	)
	return t, err
}

func (r *TaskRepository) Create(ctx context.Context, task *model.Task) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `
		INSERT INTO tasks (id, project_id, parent_id, sprint_id, title, description, status, priority,
			assignee_id, assignee_type, reporter_id, labels, budget_usd, spent_usd,
			agent_branch, agent_worktree, agent_session_id, pr_url, pr_number,
			blocked_by, created_at, updated_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,NOW(),NOW(),$21)
	`
	_, err := r.db.Exec(ctx, query,
		task.ID, task.ProjectID, task.ParentID, task.SprintID, task.Title, task.Description,
		task.Status, task.Priority, task.AssigneeID, task.AssigneeType, task.ReporterID,
		task.Labels, task.BudgetUsd, task.SpentUsd, task.AgentBranch, task.AgentWorktree,
		task.AgentSessionID, task.PRUrl, task.PRNumber, task.BlockedBy, task.CompletedAt,
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
	query := `SELECT ` + taskColumns + ` FROM tasks WHERE id = $1`
	t, err := scanTask(r.db.QueryRow(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("get task by id: %w", err)
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

	where = append(where, fmt.Sprintf("project_id = $%d", argN))
	args = append(args, projectID)
	argN++

	if q.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argN))
		args = append(args, q.Status)
		argN++
	}
	if q.AssigneeID != "" {
		where = append(where, fmt.Sprintf("assignee_id = $%d", argN))
		args = append(args, q.AssigneeID)
		argN++
	}
	if q.SprintID != "" {
		where = append(where, fmt.Sprintf("sprint_id = $%d", argN))
		args = append(args, q.SprintID)
		argN++
	}
	if q.Priority != "" {
		where = append(where, fmt.Sprintf("priority = $%d", argN))
		args = append(args, q.Priority)
		argN++
	}
	if q.Search != "" {
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", argN, argN))
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
	sort := "created_at DESC"
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

	listQuery := fmt.Sprintf("SELECT %s FROM tasks %s ORDER BY %s LIMIT %d OFFSET %d",
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

func (r *TaskRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	query := `UPDATE tasks SET
		title = COALESCE($1, title),
		description = COALESCE($2, description),
		priority = COALESCE($3, priority),
		budget_usd = COALESCE($4, budget_usd),
		updated_at = NOW()
		WHERE id = $5`
	_, err := r.db.Exec(ctx, query, req.Title, req.Description, req.Priority, req.BudgetUsd, id)
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
