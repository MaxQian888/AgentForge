// Package employee implements the EmployeeService: CRUD, skill bindings,
// lifecycle state transitions, and a stub Invoke seam for Task 7.
package employee

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

// Repository is the persistence interface the service needs.
// In production it is satisfied by *repository.EmployeeRepository.
type Repository interface {
	Create(ctx context.Context, e *model.Employee) error
	Get(ctx context.Context, id uuid.UUID) (*model.Employee, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, filter repository.EmployeeFilter) ([]*model.Employee, error)
	Update(ctx context.Context, e *model.Employee) error
	SetState(ctx context.Context, id uuid.UUID, state model.EmployeeState) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddSkill(ctx context.Context, employeeID uuid.UUID, s model.EmployeeSkill) error
	RemoveSkill(ctx context.Context, employeeID uuid.UUID, skillPath string) error
	ListSkills(ctx context.Context, employeeID uuid.UUID) ([]model.EmployeeSkill, error)
}

// RoleRegistry is the role-existence check the service needs.
// In production it is satisfied by *role.Registry (.Has(string) bool).
type RoleRegistry interface {
	Has(roleID string) bool
}

// AgentSpawner is the Task-7 seam. In Task 6 it is passed in but not called
// (Invoke returns a not-implemented error). Tests pass nil.
type AgentSpawner interface {
	SpawnForEmployee(ctx context.Context, in SpawnForEmployeeInput) (*model.AgentRun, error)
}

// SpawnForEmployeeInput carries parameters for spawning an agent run on behalf
// of an Employee. It is defined here so Task 7 can fill in Invoke without
// touching the interface boundary.
type SpawnForEmployeeInput struct {
	EmployeeID           uuid.UUID
	TaskID               uuid.UUID
	MemberID             uuid.UUID
	Runtime              string
	Provider             string
	Model                string
	RoleID               string
	BudgetUsd            float64
	SystemPromptOverride string
	ExtraSkills          []model.EmployeeSkill
}

// CreateInput holds validated fields for creating a new Employee.
type CreateInput struct {
	ProjectID    uuid.UUID
	Name         string
	DisplayName  string
	RoleID       string
	RuntimePrefs json.RawMessage
	Config       json.RawMessage
	CreatedBy    *uuid.UUID
	Skills       []model.EmployeeSkill
}

// UpdateInput holds the mutable fields the caller wants to change.
// Only non-nil pointer fields are applied; json.RawMessage nil means no change.
type UpdateInput struct {
	DisplayName  *string
	RoleID       *string
	RuntimePrefs json.RawMessage
	Config       json.RawMessage
}

// InvokeInput carries the parameters for triggering an Employee as a workflow node.
type InvokeInput struct {
	EmployeeID     uuid.UUID
	TaskID         uuid.UUID
	ExecutionID    uuid.UUID
	NodeID         string
	Prompt         string
	Context        map[string]any
	BudgetOverride *float64
}

// InvokeResult is returned by a successful Invoke.
type InvokeResult struct {
	AgentRunID uuid.UUID
}

// Service is the Employee domain service.
type Service struct {
	repo         Repository
	roles        RoleRegistry
	agentSpawner AgentSpawner
}

// NewService constructs a Service with its dependencies injected.
func NewService(repo Repository, roles RoleRegistry, spawner AgentSpawner) *Service {
	return &Service{repo: repo, roles: roles, agentSpawner: spawner}
}

// Create validates that the role exists, inserts the Employee with State =
// EmployeeStateActive, then inserts any Skills provided in CreateInput.
//
// Skill errors are wrapped and returned but the Employee row is NOT rolled back
// on failure — skill insertion is best-effort. Callers that require atomicity
// should treat a partial-skill error as a signal to delete or re-patch the
// Employee.
func (s *Service) Create(ctx context.Context, in CreateInput) (*model.Employee, error) {
	if !s.roles.Has(in.RoleID) {
		return nil, ErrRoleNotFound
	}

	e := &model.Employee{
		ID:           uuid.New(),
		ProjectID:    in.ProjectID,
		Name:         in.Name,
		DisplayName:  in.DisplayName,
		RoleID:       in.RoleID,
		RuntimePrefs: in.RuntimePrefs,
		Config:       in.Config,
		State:        model.EmployeeStateActive,
		CreatedBy:    in.CreatedBy,
	}

	if err := s.repo.Create(ctx, e); err != nil {
		if errors.Is(err, repository.ErrEmployeeNameConflict) {
			return nil, ErrEmployeeNameExists
		}
		return nil, fmt.Errorf("create employee: %w", err)
	}

	for _, sk := range in.Skills {
		if err := s.repo.AddSkill(ctx, e.ID, sk); err != nil {
			// Best-effort: Employee row is already committed. Return the error so
			// the caller is aware that skills may be incomplete.
			return nil, fmt.Errorf("add skill %q after employee create: %w", sk.SkillPath, err)
		}
	}

	return e, nil
}

// Get returns the Employee with Skills hydrated (repo.Get already does this).
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Employee, error) {
	return s.repo.Get(ctx, id)
}

// ListByProject passes through to the repo. Skills are NOT hydrated in list results.
func (s *Service) ListByProject(ctx context.Context, projectID uuid.UUID, f repository.EmployeeFilter) ([]*model.Employee, error) {
	return s.repo.ListByProject(ctx, projectID, f)
}

// Update fetches the current Employee, applies non-nil / non-empty fields from
// UpdateInput, validates the new RoleID if changed, persists via repo.Update,
// and returns the fresh state from repo.Get.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*model.Employee, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if in.RoleID != nil {
		if !s.roles.Has(*in.RoleID) {
			return nil, ErrRoleNotFound
		}
		e.RoleID = *in.RoleID
	}
	if in.DisplayName != nil {
		e.DisplayName = *in.DisplayName
	}
	if in.RuntimePrefs != nil {
		e.RuntimePrefs = in.RuntimePrefs
	}
	if in.Config != nil {
		e.Config = in.Config
	}

	if err := s.repo.Update(ctx, e); err != nil {
		return nil, fmt.Errorf("update employee: %w", err)
	}

	return s.repo.Get(ctx, id)
}

// SetState passes through to repo.SetState.
func (s *Service) SetState(ctx context.Context, id uuid.UUID, state model.EmployeeState) error {
	return s.repo.SetState(ctx, id, state)
}

// Delete passes through to repo.Delete.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// AddSkill passes through to repo.AddSkill.
func (s *Service) AddSkill(ctx context.Context, employeeID uuid.UUID, sk model.EmployeeSkill) error {
	return s.repo.AddSkill(ctx, employeeID, sk)
}

// RemoveSkill passes through to repo.RemoveSkill.
func (s *Service) RemoveSkill(ctx context.Context, employeeID uuid.UUID, skillPath string) error {
	return s.repo.RemoveSkill(ctx, employeeID, skillPath)
}

// Invoke resolves the Employee's runtime preferences and skills, then delegates
// to agentSpawner.SpawnForEmployee to create an agent run on its behalf.
func (s *Service) Invoke(ctx context.Context, in InvokeInput) (*InvokeResult, error) {
	emp, err := s.repo.Get(ctx, in.EmployeeID)
	if err != nil {
		return nil, err // pass through repository.ErrNotFound etc.
	}
	switch emp.State {
	case model.EmployeeStateArchived:
		return nil, ErrEmployeeArchived
	case model.EmployeeStatePaused:
		return nil, ErrEmployeePaused
	}

	prefs, err := decodePrefs(emp.RuntimePrefs)
	if err != nil {
		return nil, fmt.Errorf("decode runtime prefs: %w", err)
	}
	skills, err := s.repo.ListSkills(ctx, emp.ID)
	if err != nil {
		return nil, fmt.Errorf("list employee skills: %w", err)
	}
	systemPromptOverride := extractSystemPromptOverride(emp.Config)

	budget := prefs.BudgetUsd
	if budget <= 0 {
		budget = 5.0 // same default as the LLMAgentHandler
	}
	if in.BudgetOverride != nil && *in.BudgetOverride > 0 {
		budget = *in.BudgetOverride
	}

	run, err := s.agentSpawner.SpawnForEmployee(ctx, SpawnForEmployeeInput{
		EmployeeID:           emp.ID,
		TaskID:               in.TaskID,
		MemberID:             uuid.Nil, // employee-sourced runs are not attributed to a human member
		Runtime:              prefs.Runtime,
		Provider:             prefs.Provider,
		Model:                prefs.Model,
		RoleID:               emp.RoleID,
		BudgetUsd:            budget,
		SystemPromptOverride: systemPromptOverride,
		ExtraSkills:          skills,
	})
	if err != nil {
		return nil, err
	}
	return &InvokeResult{AgentRunID: run.ID}, nil
}

func decodePrefs(raw json.RawMessage) (model.RuntimePrefs, error) {
	var p model.RuntimePrefs
	if len(raw) == 0 {
		return p, nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	return p, nil
}

func extractSystemPromptOverride(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if v, ok := m["system_prompt_override"].(string); ok {
		return v
	}
	return ""
}
