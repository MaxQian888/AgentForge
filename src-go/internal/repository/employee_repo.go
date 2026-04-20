// Package repository — employee_repo.go persists employees and their skill bindings.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EmployeeFilter holds optional filters for ListByProject.
type EmployeeFilter struct {
	State *model.EmployeeState
}

// EmployeeRepository handles persistence for Employee aggregates.
type EmployeeRepository struct {
	db *gorm.DB
}

// NewEmployeeRepository returns a new EmployeeRepository backed by db.
func NewEmployeeRepository(db *gorm.DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

// ---------------------------------------------------------------------------
// Record types
// ---------------------------------------------------------------------------

type employeeRecord struct {
	ID           uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID    uuid.UUID  `gorm:"column:project_id"`
	Name         string     `gorm:"column:name"`
	DisplayName  string     `gorm:"column:display_name"`
	RoleID       string     `gorm:"column:role_id"`
	RuntimePrefs jsonText   `gorm:"column:runtime_prefs;type:jsonb"`
	Config       jsonText   `gorm:"column:config;type:jsonb"`
	State        string     `gorm:"column:state"`
	CreatedBy    *uuid.UUID `gorm:"column:created_by"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
}

func (employeeRecord) TableName() string { return "employees" }

type employeeSkillRecord struct {
	EmployeeID uuid.UUID `gorm:"column:employee_id;primaryKey"`
	SkillPath  string    `gorm:"column:skill_path;primaryKey"`
	AutoLoad   bool      `gorm:"column:auto_load"`
	Overrides  jsonText  `gorm:"column:overrides;type:jsonb"`
	AddedAt    time.Time `gorm:"column:added_at"`
}

func (employeeSkillRecord) TableName() string { return "employee_skills" }

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

func newEmployeeRecord(e *model.Employee) *employeeRecord {
	if e == nil {
		return nil
	}
	return &employeeRecord{
		ID:           e.ID,
		ProjectID:    e.ProjectID,
		Name:         e.Name,
		DisplayName:  e.DisplayName,
		RoleID:       e.RoleID,
		RuntimePrefs: newJSONText(rawMessageToString(e.RuntimePrefs), "{}"),
		Config:       newJSONText(rawMessageToString(e.Config), "{}"),
		State:        string(e.State),
		CreatedBy:    e.CreatedBy,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

func (r *employeeRecord) toModel() *model.Employee {
	if r == nil {
		return nil
	}
	return &model.Employee{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		Name:         r.Name,
		DisplayName:  r.DisplayName,
		RoleID:       r.RoleID,
		RuntimePrefs: json.RawMessage(r.RuntimePrefs.String("{}")),
		Config:       json.RawMessage(r.Config.String("{}")),
		State:        model.EmployeeState(r.State),
		CreatedBy:    r.CreatedBy,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func newEmployeeSkillRecord(employeeID uuid.UUID, s model.EmployeeSkill) *employeeSkillRecord {
	return &employeeSkillRecord{
		EmployeeID: employeeID,
		SkillPath:  s.SkillPath,
		AutoLoad:   s.AutoLoad,
		Overrides:  newJSONText(rawMessageToString(s.Overrides), "{}"),
		AddedAt:    s.AddedAt,
	}
}

func (r *employeeSkillRecord) toModel() model.EmployeeSkill {
	return model.EmployeeSkill{
		EmployeeID: r.EmployeeID,
		SkillPath:  r.SkillPath,
		AutoLoad:   r.AutoLoad,
		Overrides:  json.RawMessage(r.Overrides.String("{}")),
		AddedAt:    r.AddedAt,
	}
}

// rawMessageToString converts json.RawMessage to string for jsonText, returning
// "" when nil so newJSONText falls back to its default.
func rawMessageToString(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	return string(raw)
}

// ---------------------------------------------------------------------------
// Repository methods
// ---------------------------------------------------------------------------

// Create inserts a new Employee row. Returns ErrEmployeeNameConflict when the
// (project_id, name) unique constraint fires.
func (r *EmployeeRepository) Create(ctx context.Context, e *model.Employee) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if err := r.db.WithContext(ctx).Create(newEmployeeRecord(e)).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrEmployeeNameConflict
		}
		return fmt.Errorf("create employee: %w", err)
	}
	return nil
}

// Get fetches a single Employee by ID and hydrates its Skills.
func (r *EmployeeRepository) Get(ctx context.Context, id uuid.UUID) (*model.Employee, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec employeeRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error; err != nil {
		if normalized := normalizeRepositoryError(err); errors.Is(normalized, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get employee: %w", err)
	}
	emp := rec.toModel()
	skills, err := r.ListSkills(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get employee skills: %w", err)
	}
	emp.Skills = skills
	return emp, nil
}

// ListByProject returns all employees for a project, ordered newest-first.
// Skills are NOT hydrated; callers who need them should call Get per row.
func (r *EmployeeRepository) ListByProject(ctx context.Context, projectID uuid.UUID, filter EmployeeFilter) ([]*model.Employee, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if filter.State != nil {
		q = q.Where("state = ?", string(*filter.State))
	}
	var records []employeeRecord
	if err := q.Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list employees by project: %w", err)
	}
	out := make([]*model.Employee, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

// Update writes mutable fields back to the row identified by e.ID.
func (r *EmployeeRepository) Update(ctx context.Context, e *model.Employee) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"display_name":  e.DisplayName,
		"role_id":       e.RoleID,
		"runtime_prefs": newJSONText(rawMessageToString(e.RuntimePrefs), "{}"),
		"config":        newJSONText(rawMessageToString(e.Config), "{}"),
		"updated_at":    time.Now().UTC(),
	}
	res := r.db.WithContext(ctx).
		Model(&employeeRecord{}).
		Where("id = ?", e.ID).
		Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("update employee: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetState transitions an employee to a new lifecycle state.
// Returns ErrNotFound when no row matched the ID.
func (r *EmployeeRepository) SetState(ctx context.Context, id uuid.UUID, state model.EmployeeState) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).
		Model(&employeeRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"state":      string(state),
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("set employee state: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete hard-deletes an employee row. FK cascade handles employee_skills.
// Returns ErrNotFound when no row matched.
func (r *EmployeeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&employeeRecord{})
	if res.Error != nil {
		return fmt.Errorf("delete employee: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// AddSkill upserts a skill binding for the given employee.
// A second call with the same (employee_id, skill_path) updates auto_load and overrides.
func (r *EmployeeRepository) AddSkill(ctx context.Context, employeeID uuid.UUID, s model.EmployeeSkill) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	s.EmployeeID = employeeID
	rec := newEmployeeSkillRecord(employeeID, s)
	if rec.AddedAt.IsZero() {
		rec.AddedAt = time.Now().UTC()
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "employee_id"},
				{Name: "skill_path"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"auto_load", "overrides"}),
		}).
		Create(rec).Error; err != nil {
		return fmt.Errorf("add employee skill: %w", err)
	}
	return nil
}

// RemoveSkill deletes a single skill binding by composite key.
func (r *EmployeeRepository) RemoveSkill(ctx context.Context, employeeID uuid.UUID, skillPath string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).
		Where("employee_id = ? AND skill_path = ?", employeeID, skillPath).
		Delete(&employeeSkillRecord{}).Error; err != nil {
		return fmt.Errorf("remove employee skill: %w", err)
	}
	return nil
}

// ListSkills returns all skill bindings for an employee, ordered by added_at ASC.
func (r *EmployeeRepository) ListSkills(ctx context.Context, employeeID uuid.UUID) ([]model.EmployeeSkill, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []employeeSkillRecord
	if err := r.db.WithContext(ctx).
		Where("employee_id = ?", employeeID).
		Order("added_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list employee skills: %w", err)
	}
	out := make([]model.EmployeeSkill, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isUniqueViolation returns true when err represents a PostgreSQL unique
// constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
