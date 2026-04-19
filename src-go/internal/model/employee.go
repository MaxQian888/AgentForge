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
	ID           uuid.UUID       `json:"id"`
	ProjectID    uuid.UUID       `json:"projectId"`
	Name         string          `json:"name"`
	DisplayName  string          `json:"displayName,omitempty"`
	RoleID       string          `json:"roleId"`
	RuntimePrefs json.RawMessage `json:"runtimePrefs"`
	Config       json.RawMessage `json:"config"`
	State        EmployeeState   `json:"state"`
	CreatedBy    *uuid.UUID      `json:"createdBy,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`

	Skills []EmployeeSkill `json:"skills,omitempty"`
}

// EmployeeSkill is an additional skill binding beyond the role manifest's declared skills.
type EmployeeSkill struct {
	EmployeeID uuid.UUID       `json:"employeeId"`
	SkillPath  string          `json:"skillPath"`
	AutoLoad   bool            `json:"autoLoad"`
	Overrides  json.RawMessage `json:"overrides,omitempty"`
	AddedAt    time.Time       `json:"addedAt"`
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
