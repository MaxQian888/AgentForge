package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EmployeeState string

const (
	EmployeeStateActive   EmployeeState = "active"
	EmployeeStatePaused   EmployeeState = "paused"
	EmployeeStateArchived EmployeeState = "archived"
)

// Employee is a persistent capability carrier scoped to one project.
// It binds a role manifest, optional extra skills, runtime preferences,
// and lifecycle state. AgentRuns executed on its behalf carry
// EmployeeID for memory isolation and history attribution.
type Employee struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	ProjectID    uuid.UUID       `db:"project_id" json:"projectId"`
	Name         string          `db:"name" json:"name"`
	DisplayName  string          `db:"display_name" json:"displayName,omitempty"`
	RoleID       string          `db:"role_id" json:"roleId"`
	RuntimePrefs json.RawMessage `db:"runtime_prefs" json:"runtimePrefs"`
	Config       json.RawMessage `db:"config" json:"config"`
	State        EmployeeState   `db:"state" json:"state"`
	CreatedBy    *uuid.UUID      `db:"created_by" json:"createdBy,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updatedAt"`

	// Skills is populated on-demand by the repository (e.g., Get) from the
	// employee_skills join table. It is NOT a column on the employees row
	// and will be silently dropped on Employee upserts.
	Skills []EmployeeSkill `json:"skills,omitempty"`
}

// EmployeeSkill is an additional skill binding beyond the role manifest's declared skills.
type EmployeeSkill struct {
	EmployeeID uuid.UUID       `db:"employee_id" json:"employeeId"`
	SkillPath  string          `db:"skill_path" json:"skillPath"`
	AutoLoad   bool            `db:"auto_load" json:"autoLoad"`
	Overrides  json.RawMessage `db:"overrides" json:"overrides,omitempty"`
	AddedAt    time.Time       `db:"added_at" json:"addedAt"`
}

// RuntimePrefs is the typed shape we expect inside Employee.RuntimePrefs.
// Persisted as JSON to allow schema evolution without migrations.
type RuntimePrefs struct {
	Runtime   string  `json:"runtime,omitempty"`
	Provider  string  `json:"provider,omitempty"`
	Model     string  `json:"model,omitempty"`
	BudgetUsd float64 `json:"budgetUsd,omitempty"`
	MaxTurns  int     `json:"maxTurns,omitempty"`
}
