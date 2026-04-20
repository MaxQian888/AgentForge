package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EmployeeRunKind narrows the UNION query in ListByEmployee.
type EmployeeRunKind string

const (
	EmployeeRunKindAll      EmployeeRunKind = "all"
	EmployeeRunKindWorkflow EmployeeRunKind = "workflow"
	EmployeeRunKindAgent    EmployeeRunKind = "agent"
)

// EmployeeRunRow is the unified DTO returned by ListByEmployee. One row
// represents either a workflow_executions row (Kind="workflow") or an
// agent_runs row (Kind="agent"); the Name / RefURL fields are pre-rendered
// so the FE can drill down without extra joins.
type EmployeeRunRow struct {
	Kind        string     `json:"kind"` // "workflow" | "agent"
	ID          string     `json:"id"`
	Name        string     `json:"name"` // workflow_definitions.name OR roles.id (agent_runs.role_id)
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	DurationMs  *int64     `json:"durationMs,omitempty"`
	RefURL      string     `json:"refUrl"`
}

// EmployeeRunsRepository serves the per-employee unified runs feed.
type EmployeeRunsRepository struct {
	db *gorm.DB
}

// NewEmployeeRunsRepository returns a new EmployeeRunsRepository backed by db.
func NewEmployeeRunsRepository(db *gorm.DB) *EmployeeRunsRepository {
	return &EmployeeRunsRepository{db: db}
}

func normalizeRunsPage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func normalizeRunsSize(size int) int {
	if size <= 0 {
		return 20
	}
	if size > 200 {
		return 200
	}
	return size
}

// unifiedRunRow is the SQL projection target for the UNION query.
type unifiedRunRow struct {
	Kind        string     `gorm:"column:kind"`
	ID          uuid.UUID  `gorm:"column:id"`
	Name        string     `gorm:"column:name"`
	Status      string     `gorm:"column:status"`
	StartedAt   *time.Time `gorm:"column:started_at"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
}

// ListByEmployee returns workflow_executions ∪ agent_runs filtered by
// employee, ordered started_at DESC, with offset-based pagination.
//
// The UNION is evaluated as a subquery so the outer ORDER BY / LIMIT /
// OFFSET applies to the combined result set, not to each leg
// independently. workflow_executions.acting_employee_id and
// agent_runs.employee_id are both nullable; rows with NULL are excluded
// by the WHERE clauses on each leg.
func (r *EmployeeRunsRepository) ListByEmployee(ctx context.Context, employeeID uuid.UUID, kind EmployeeRunKind, page, size int) ([]EmployeeRunRow, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	page = normalizeRunsPage(page)
	size = normalizeRunsSize(size)
	offset := (page - 1) * size

	// Build per-leg SELECTs, then UNION ALL based on kind.
	wfSQL := `
            SELECT 'workflow' AS kind,
                   we.id        AS id,
                   COALESCE(wd.name, we.workflow_id::text) AS name,
                   we.status    AS status,
                   we.started_at AS started_at,
                   we.completed_at AS completed_at
              FROM workflow_executions we
              LEFT JOIN workflow_definitions wd ON wd.id = we.workflow_id
             WHERE we.acting_employee_id = ?`

	arSQL := `
            SELECT 'agent' AS kind,
                   ar.id     AS id,
                   COALESCE(NULLIF(ar.role_id, ''), 'agent') AS name,
                   ar.status AS status,
                   ar.started_at AS started_at,
                   ar.completed_at AS completed_at
              FROM agent_runs ar
             WHERE ar.employee_id = ?`

	var sqlText string
	var args []any
	switch kind {
	case EmployeeRunKindWorkflow:
		sqlText = wfSQL + ` ORDER BY started_at DESC NULLS LAST, id DESC LIMIT ? OFFSET ?`
		args = []any{employeeID, size, offset}
	case EmployeeRunKindAgent:
		sqlText = arSQL + ` ORDER BY started_at DESC NULLS LAST, id DESC LIMIT ? OFFSET ?`
		args = []any{employeeID, size, offset}
	default: // EmployeeRunKindAll or unknown
		sqlText = `
                SELECT * FROM (
                ` + wfSQL + `
                    UNION ALL
                ` + arSQL + `
                ) u
                ORDER BY started_at DESC NULLS LAST, id DESC
                LIMIT ? OFFSET ?`
		args = []any{employeeID, employeeID, size, offset}
	}

	var rows []unifiedRunRow
	if err := r.db.WithContext(ctx).Raw(sqlText, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list employee runs: %w", err)
	}

	out := make([]EmployeeRunRow, 0, len(rows))
	for _, row := range rows {
		er := EmployeeRunRow{
			Kind:        row.Kind,
			ID:          row.ID.String(),
			Name:        row.Name,
			Status:      row.Status,
			StartedAt:   row.StartedAt,
			CompletedAt: row.CompletedAt,
		}
		if row.StartedAt != nil && row.CompletedAt != nil {
			d := row.CompletedAt.Sub(*row.StartedAt).Milliseconds()
			er.DurationMs = &d
		}
		switch row.Kind {
		case "workflow":
			er.RefURL = "/workflow/runs/" + row.ID.String()
		case "agent":
			er.RefURL = "/agents?run=" + row.ID.String()
		}
		out = append(out, er)
	}
	return out, nil
}
