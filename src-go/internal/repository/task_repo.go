package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/pkg/database"
	"gorm.io/gorm"
)

type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) DB() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.db
}

func (r *TaskRepository) WithDB(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// attachProgress batch-loads progress snapshots for the given tasks and attaches them.
func (r *TaskRepository) attachProgress(ctx context.Context, tasks []*model.Task) error {
	if len(tasks) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	var snapshots []taskProgressSnapshotRecord
	if err := r.db.WithContext(ctx).Where("task_id IN ?", ids).Find(&snapshots).Error; err != nil {
		return err
	}
	snapshotMap := make(map[uuid.UUID]*model.TaskProgressSnapshot, len(snapshots))
	for i := range snapshots {
		snapshotMap[snapshots[i].TaskID] = snapshots[i].toModel()
	}
	for _, t := range tasks {
		t.Progress = snapshotMap[t.ID]
	}
	return nil
}

func (r *TaskRepository) Create(ctx context.Context, task *model.Task) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if _, err := normalizeTaskBlockedBy(task.BlockedBy); err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	record := newTaskRecord(task)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record taskRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get task by id: %w", normalizeRepositoryError(err))
	}
	task := record.toModel()
	var progress taskProgressSnapshotRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", id).Take(&progress).Error; err == nil {
		task.Progress = progress.toModel()
	}
	return task, nil
}

func (r *TaskRepository) GetByPRURL(ctx context.Context, prURL string) (*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record taskRecord
	if err := r.db.WithContext(ctx).Where("pr_url = ?", prURL).Order("updated_at DESC").Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get task by pr url: %w", normalizeRepositoryError(err))
	}
	task := record.toModel()
	var progress taskProgressSnapshotRecord
	if err := r.db.WithContext(ctx).Where("task_id = ?", record.ID).Take(&progress).Error; err == nil {
		task.Progress = progress.toModel()
	}
	return task, nil
}

func (r *TaskRepository) List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseUnavailable
	}

	query := r.db.WithContext(ctx).Model(&taskRecord{}).Where("tasks.project_id = ?", projectID)
	if q.Status != "" {
		query = query.Where("tasks.status = ?", q.Status)
	}
	if q.AssigneeID != "" {
		query = query.Where("tasks.assignee_id = ?", q.AssigneeID)
	}
	if q.SprintID != "" {
		query = query.Where("tasks.sprint_id = ?", q.SprintID)
	}
	if q.Priority != "" {
		query = query.Where("tasks.priority = ?", q.Priority)
	}
	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("(tasks.title ILIKE ? OR tasks.description ILIKE ?)", search, search)
	}

	sort := "tasks.created_at DESC"
	if q.Sort != "" {
		sort = q.Sort
	}

	var err error
	query, sort, err = applyTaskCustomFieldQuery(query, q, sort)
	if err != nil {
		return nil, 0, fmt.Errorf("apply custom field query: %w", err)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	var records []taskRecord
	if err := query.Order(sort).Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}

	tasks := make([]*model.Task, 0, len(records))
	for i := range records {
		tasks = append(tasks, records[i].toModel())
	}
	if err := r.attachProgress(ctx, tasks); err != nil {
		return nil, 0, fmt.Errorf("attach progress: %w", err)
	}
	return tasks, int(total), nil
}

func applyTaskCustomFieldQuery(query *gorm.DB, q model.TaskListQuery, sort string) (*gorm.DB, string, error) {
	for index, filter := range q.CustomFieldFilters {
		if strings.TrimSpace(filter.FieldDefID) == "" {
			return nil, "", fmt.Errorf("custom field filter %d missing fieldDefId", index)
		}
		alias := fmt.Sprintf("cf_filter_%d", index)
		query = query.Joins(
			fmt.Sprintf("LEFT JOIN custom_field_values %s ON %s.task_id = tasks.id AND %s.field_def_id = ?", alias, alias, alias),
			filter.FieldDefID,
		)
		normalizedExpr := fmt.Sprintf("REPLACE(CAST(%s.value AS TEXT), '\"', '')", alias)
		switch strings.ToLower(strings.TrimSpace(filter.Op)) {
		case "", "eq":
			query = query.Where(fmt.Sprintf("%s = ?", normalizedExpr), filter.Value)
		case "ne":
			query = query.Where(fmt.Sprintf("%s <> ?", normalizedExpr), filter.Value)
		case "gt":
			query = query.Where(fmt.Sprintf("%s > ?", normalizedExpr), filter.Value)
		case "gte":
			query = query.Where(fmt.Sprintf("%s >= ?", normalizedExpr), filter.Value)
		case "lt":
			query = query.Where(fmt.Sprintf("%s < ?", normalizedExpr), filter.Value)
		case "lte":
			query = query.Where(fmt.Sprintf("%s <= ?", normalizedExpr), filter.Value)
		case "contains":
			query = query.Where(fmt.Sprintf("LOWER(%s) LIKE ?", normalizedExpr), "%"+strings.ToLower(filter.Value)+"%")
		default:
			return nil, "", fmt.Errorf("unsupported custom field filter op %q", filter.Op)
		}
	}

	if q.CustomFieldSort != nil && strings.TrimSpace(q.CustomFieldSort.FieldDefID) != "" {
		alias := "cf_sort"
		query = query.Joins(
			fmt.Sprintf("LEFT JOIN custom_field_values %s ON %s.task_id = tasks.id AND %s.field_def_id = ?", alias, alias, alias),
			q.CustomFieldSort.FieldDefID,
		)
		direction := strings.ToUpper(strings.TrimSpace(q.CustomFieldSort.Direction))
		if direction != "ASC" && direction != "DESC" {
			direction = "ASC"
		}
		sort = fmt.Sprintf("REPLACE(CAST(%s.value AS TEXT), '\"', '') %s, tasks.created_at DESC", alias, direction)
	}

	return query, sort, nil
}

func (r *TaskRepository) ListBySprint(ctx context.Context, projectID uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []taskRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND sprint_id = ?", projectID, sprintID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list sprint tasks: %w", err)
	}

	tasks := make([]*model.Task, 0, len(records))
	for i := range records {
		tasks = append(tasks, records[i].toModel())
	}
	if err := r.attachProgress(ctx, tasks); err != nil {
		return nil, fmt.Errorf("attach progress: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	updates := map[string]any{}

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.BudgetUsd != nil {
		updates["budget_usd"] = *req.BudgetUsd
	}
	if req.SprintID != nil {
		val := strings.TrimSpace(*req.SprintID)
		if val == "" {
			updates["sprint_id"] = nil
		} else {
			parsed, err := uuid.Parse(val)
			if err != nil {
				return fmt.Errorf("update task: invalid sprint_id %q: %w", val, err)
			}
			updates["sprint_id"] = parsed
		}
	}
	if req.MilestoneID != nil {
		val := strings.TrimSpace(*req.MilestoneID)
		if val == "" {
			updates["milestone_id"] = nil
		} else {
			parsed, err := uuid.Parse(val)
			if err != nil {
				return fmt.Errorf("update task: invalid milestone_id %q: %w", val, err)
			}
			updates["milestone_id"] = parsed
		}
	}
	if req.PlannedStartAt != nil {
		val := strings.TrimSpace(*req.PlannedStartAt)
		if val == "" {
			updates["planned_start_at"] = nil
		} else {
			t, err := time.Parse(time.RFC3339, val)
			if err != nil {
				return fmt.Errorf("update task: invalid planned_start_at %q: %w", val, err)
			}
			updates["planned_start_at"] = t
		}
	}
	if req.PlannedEndAt != nil {
		val := strings.TrimSpace(*req.PlannedEndAt)
		if val == "" {
			updates["planned_end_at"] = nil
		} else {
			t, err := time.Parse(time.RFC3339, val)
			if err != nil {
				return fmt.Errorf("update task: invalid planned_end_at %q: %w", val, err)
			}
			updates["planned_end_at"] = t
		}
	}
	if req.BlockedBy != nil {
		blockedByIDs, err := normalizeTaskBlockedBy(*req.BlockedBy)
		if err != nil {
			return fmt.Errorf("update task: %w", err)
		}
		updates["blocked_by"] = blockedByIDs
	}

	updates["updated_at"] = gorm.Expr("NOW()")

	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

func (r *TaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Delete(&taskRecord{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func (r *TaskRepository) TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	var current taskRecord
	if err := r.db.WithContext(ctx).Select("status").Where("id = ?", id).Take(&current).Error; err != nil {
		return fmt.Errorf("get task status: %w", normalizeRepositoryError(err))
	}
	if err := model.ValidateTransition(current.Status, newStatus); err != nil {
		return err
	}

	updates := map[string]any{
		"status":     newStatus,
		"updated_at": gorm.Expr("NOW()"),
	}
	if newStatus == model.TaskStatusDone {
		updates["completed_at"] = gorm.Expr("NOW()")
	}
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("transition task status: %w", err)
	}
	return nil
}

func (r *TaskRepository) UpdateAssignee(ctx context.Context, id uuid.UUID, assigneeID uuid.UUID, assigneeType string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("id = ?", id).Updates(map[string]any{
		"assignee_id":   assigneeID,
		"assignee_type": assigneeType,
		"updated_at":    gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update task assignee: %w", err)
	}
	return nil
}

func (r *TaskRepository) UpdateRuntime(ctx context.Context, id uuid.UUID, branch, worktreePath, sessionID string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("id = ?", id).Updates(map[string]any{
		"agent_branch":     branch,
		"agent_worktree":   worktreePath,
		"agent_session_id": sessionID,
		"updated_at":       gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("update task runtime: %w", err)
	}
	return nil
}

func (r *TaskRepository) ClearRuntime(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("id = ?", id).Updates(map[string]any{
		"agent_branch":     "",
		"agent_worktree":   "",
		"agent_session_id": "",
		"updated_at":       gorm.Expr("NOW()"),
	}).Error; err != nil {
		return fmt.Errorf("clear task runtime: %w", err)
	}
	return nil
}

func (r *TaskRepository) UpdateSpent(ctx context.Context, id uuid.UUID, spentUsd float64, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"spent_usd":  spentUsd,
		"updated_at": gorm.Expr("NOW()"),
	}
	if status != "" {
		updates["status"] = status
	}
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("update task spent: %w", err)
	}
	return nil
}

func (r *TaskRepository) ListOpenForProgress(ctx context.Context) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []taskRecord
	if err := r.db.WithContext(ctx).
		Where("status NOT IN ?", []string{model.TaskStatusDone, model.TaskStatusCancelled}).
		Order("updated_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list open tasks for progress: %w", err)
	}

	tasks := make([]*model.Task, 0, len(records))
	for i := range records {
		tasks = append(tasks, records[i].toModel())
	}
	if err := r.attachProgress(ctx, tasks); err != nil {
		return nil, fmt.Errorf("attach progress: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepository) HasChildren(ctx context.Context, parentID uuid.UUID) (bool, error) {
	if r.db == nil {
		return false, ErrDatabaseUnavailable
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&taskRecord{}).Where("parent_id = ?", parentID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("count child tasks: %w", err)
	}
	return count > 0, nil
}

func (r *TaskRepository) ListChildren(ctx context.Context, parentID uuid.UUID) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []taskRecord
	if err := r.db.WithContext(ctx).
		Where("parent_id = ?", parentID).
		Order("created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}

	tasks := make([]*model.Task, 0, len(records))
	for i := range records {
		tasks = append(tasks, records[i].toModel())
	}
	return tasks, nil
}

func (r *TaskRepository) ListDependents(ctx context.Context, blockerID uuid.UUID) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []taskRecord
	if err := r.db.WithContext(ctx).
		Where("? = ANY(blocked_by)", blockerID).
		Order("updated_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list dependents: %w", err)
	}

	tasks := make([]*model.Task, 0, len(records))
	for i := range records {
		tasks = append(tasks, records[i].toModel())
	}
	if err := r.attachProgress(ctx, tasks); err != nil {
		return nil, fmt.Errorf("attach progress: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepository) CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if len(inputs) == 0 {
		return []*model.Task{}, nil
	}

	created := make([]*model.Task, 0, len(inputs))

	err := database.WithTx(ctx, r.db, func(tx *gorm.DB) error {
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
			if _, err := normalizeTaskBlockedBy(task.BlockedBy); err != nil {
				return fmt.Errorf("create child task: %w", err)
			}
			record := newTaskRecord(task)
			if err := tx.WithContext(ctx).Create(record).Error; err != nil {
				return fmt.Errorf("create child task: %w", err)
			}
			created = append(created, task)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func normalizeOptionalTaskBlockedBy(blockedBy *[]string) (any, error) {
	if blockedBy == nil {
		return nil, nil
	}
	return normalizeTaskBlockedBy(*blockedBy)
}

type TaskDateCount struct {
	Date  time.Time
	Count int
}

type TaskDateCost struct {
	Date    time.Time
	Count   int
	CostUsd float64
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

	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
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

func (r *TaskRepository) SummarizeCompletedCostByDateRange(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]TaskDateCost, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	query := `SELECT COALESCE(completed_at, updated_at)::date AS d, COUNT(*) AS cnt, COALESCE(SUM(spent_usd), 0) AS total_cost
		FROM tasks
		WHERE status = 'done'
		AND COALESCE(completed_at, updated_at) BETWEEN $1 AND $2`
	args := []interface{}{from, to}
	if projectID != nil {
		query += ` AND project_id = $3`
		args = append(args, *projectID)
	}
	query += ` GROUP BY d ORDER BY d`

	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("summarize completed cost by date range: %w", err)
	}
	defer rows.Close()

	var results []TaskDateCost
	for rows.Next() {
		var tc TaskDateCost
		if err := rows.Scan(&tc.Date, &tc.Count, &tc.CostUsd); err != nil {
			return nil, fmt.Errorf("scan task date cost: %w", err)
		}
		results = append(results, tc)
	}
	return results, rows.Err()
}

// ErrTaskAncestorCycle is returned by GetAncestorRoot when a cycle is detected
// in the ParentID chain.
var ErrTaskAncestorCycle = errors.New("task ancestor cycle detected")

// GetAncestorRoot walks the ParentID chain from taskID up to the root task.
// Returns the task itself if ParentID is nil.
// Returns ErrTaskAncestorCycle if a cycle is encountered.
func (r *TaskRepository) GetAncestorRoot(ctx context.Context, taskID uuid.UUID) (*model.Task, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	seen := make(map[uuid.UUID]struct{})
	current := taskID
	for {
		if _, visited := seen[current]; visited {
			return nil, fmt.Errorf("get ancestor root: %w", ErrTaskAncestorCycle)
		}
		seen[current] = struct{}{}

		var record taskRecord
		if err := r.db.WithContext(ctx).Where("id = ?", current).Take(&record).Error; err != nil {
			return nil, fmt.Errorf("get ancestor root: %w", normalizeRepositoryError(err))
		}
		if record.ParentID == nil {
			return record.toModel(), nil
		}
		current = *record.ParentID
	}
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
