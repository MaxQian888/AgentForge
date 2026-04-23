// Package trigger — crud_service.go is the orchestration layer for the
// Spec 1C trigger CRUD surface. It validates author-time references
// (workflow exists in same project, acting employee is active and same-
// project) and delegates persistence to the workflow_triggers repository.
//
// Lives in the trigger package (not internal/service) to avoid an import
// cycle: internal/trigger/engines.go depends on internal/service for the
// engine adapters, so the inverse direction is forbidden. Handlers reach
// this service as *trigger.CRUDService.
//
// Notable invariants enforced here (not at the repository layer):
//   - Create stamps CreatedVia=manual unconditionally so registrar-owned
//     rows can never be authored through this surface.
//   - Patch refuses to mutate WorkflowID, Source, and CreatedVia silently;
//     the handler is expected to reject the body fields explicitly.
//   - Delete refuses to remove dag_node-owned rows (those belong to the DAG
//     definition; users must edit the workflow to remove them).
package trigger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
)

// Sentinel errors returned by CRUDService. Handlers map these to the
// stable error codes defined in spec1 §10 (trigger:workflow_not_found,
// trigger:acting_employee_archived, trigger:cannot_delete_dag_managed).
var (
	ErrTriggerWorkflowNotFound       = errors.New("trigger:workflow_not_found")
	ErrTriggerActingEmployeeArchived = errors.New("trigger:acting_employee_archived")
	ErrTriggerCannotDeleteDAGManaged = errors.New("trigger:cannot_delete_dag_managed")
	ErrTriggerNotFound               = errors.New("trigger:not_found")
)

// triggerCRUDRepo is the persistence seam consumed by CRUDService.
// Satisfied in production by *repository.WorkflowTriggerRepository; tests
// substitute an in-memory mock.
type triggerCRUDRepo interface {
	Create(ctx context.Context, t *model.WorkflowTrigger) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowTrigger, error)
	Update(ctx context.Context, t *model.WorkflowTrigger) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByActingEmployee(ctx context.Context, employeeID uuid.UUID) ([]*model.WorkflowTrigger, error)
}

// workflowDefLookup is the read-side seam for resolving target workflows.
// Satisfied by *repository.WorkflowDefinitionRepository.
type workflowDefLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
}

// employeeLookup is the read-side seam for validating acting-employee
// references. Satisfied by *repository.EmployeeRepository.
type employeeLookup interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Employee, error)
}

// CRUDService is the orchestration layer for trigger CRUD.
type CRUDService struct {
	repo triggerCRUDRepo
	defs workflowDefLookup
	emps employeeLookup
}

// NewCRUDService wires the dependencies. Any of defs / emps may be nil
// in unit tests that don't exercise the corresponding validation branch;
// production wiring (routes.go) supplies all three.
func NewCRUDService(repo triggerCRUDRepo, defs workflowDefLookup, emps employeeLookup) *CRUDService {
	return &CRUDService{repo: repo, defs: defs, emps: emps}
}

// CreateTriggerInput is the typed shape consumed by Create. All fields are
// validated in-method; Source must be a known TriggerSource and the workflow
// must exist before the row is persisted.
type CreateTriggerInput struct {
	WorkflowID       uuid.UUID
	Source           model.TriggerSource
	Config           json.RawMessage
	InputMapping     json.RawMessage
	ActingEmployeeID *uuid.UUID
	DisplayName      string
	Description      string
	CreatedBy        *uuid.UUID
}

// Create persists a new manual trigger row. The workflow must exist in the
// same project as any acting-employee reference, and the acting employee
// (when set) must be active. CreatedVia is forced to 'manual' so the row
// is permanently isolated from the registrar's reaper.
func (s *CRUDService) Create(ctx context.Context, in CreateTriggerInput) (*model.WorkflowTrigger, error) {
	if s.defs == nil {
		return nil, fmt.Errorf("trigger service: workflow definition lookup not wired")
	}
	def, err := s.defs.GetByID(ctx, in.WorkflowID)
	if err != nil || def == nil {
		return nil, ErrTriggerWorkflowNotFound
	}
	if in.ActingEmployeeID != nil {
		if err := s.validateActingEmployee(ctx, *in.ActingEmployeeID, def.ProjectID); err != nil {
			return nil, err
		}
	}
	wfRef := in.WorkflowID
	tr := &model.WorkflowTrigger{
		WorkflowID:       &wfRef,
		ProjectID:        def.ProjectID,
		Source:           in.Source,
		TargetKind:       model.TriggerTargetDAG,
		Config:           normalizeJSON(in.Config),
		InputMapping:     normalizeJSON(in.InputMapping),
		ActingEmployeeID: in.ActingEmployeeID,
		DisplayName:      in.DisplayName,
		Description:      in.Description,
		CreatedVia:       model.TriggerCreatedViaManual,
		CreatedBy:        in.CreatedBy,
		Enabled:          true,
	}
	if err := s.repo.Create(ctx, tr); err != nil {
		return nil, fmt.Errorf("trigger service: create: %w", err)
	}
	return tr, nil
}

// PatchTriggerInput uses pointer fields so callers can express "do not
// touch" vs "set to zero value" unambiguously. ActingEmployeeID specifically
// accepts a double-pointer-style nil-vs-set pattern via the IncludeActingEmployeeID
// flag, since *uuid.UUID alone cannot distinguish "clear" from "no change".
type PatchTriggerInput struct {
	Config       json.RawMessage
	InputMapping json.RawMessage
	DisplayName  *string
	Description  *string
	Enabled      *bool

	// IncludeActingEmployeeID, when true, applies ActingEmployeeID (which may
	// be nil to clear the binding). When false, the existing value is kept.
	IncludeActingEmployeeID bool
	ActingEmployeeID        *uuid.UUID
}

// Patch loads the row by id and applies the non-nil fields. If
// ActingEmployeeID is being changed, it is re-validated against the
// workflow's project. Returns ErrTriggerNotFound when the row is missing.
//
// WorkflowID, Source, and CreatedVia are intentionally not part of the
// input shape; the handler is the contract enforcement boundary that
// rejects attempts to set them in the request body.
func (s *CRUDService) Patch(ctx context.Context, id uuid.UUID, in PatchTriggerInput) (*model.WorkflowTrigger, error) {
	tr, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTriggerNotFound
		}
		return nil, fmt.Errorf("trigger service: patch lookup: %w", err)
	}
	if len(in.Config) > 0 {
		tr.Config = normalizeJSON(in.Config)
	}
	if len(in.InputMapping) > 0 {
		tr.InputMapping = normalizeJSON(in.InputMapping)
	}
	if in.DisplayName != nil {
		tr.DisplayName = *in.DisplayName
	}
	if in.Description != nil {
		tr.Description = *in.Description
	}
	if in.Enabled != nil {
		tr.Enabled = *in.Enabled
	}
	if in.IncludeActingEmployeeID {
		if in.ActingEmployeeID != nil {
			if err := s.validateActingEmployee(ctx, *in.ActingEmployeeID, tr.ProjectID); err != nil {
				return nil, err
			}
		}
		tr.ActingEmployeeID = in.ActingEmployeeID
	}
	if err := s.repo.Update(ctx, tr); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTriggerNotFound
		}
		return nil, fmt.Errorf("trigger service: patch: %w", err)
	}
	return tr, nil
}

// Delete removes a row only when it is FE-authored (CreatedVia=manual).
// Rows materialized by the registrar (CreatedVia=dag_node) belong to a
// workflow definition and are removed by editing the DAG; attempting to
// delete one through the CRUD surface returns ErrTriggerCannotDeleteDAGManaged.
func (s *CRUDService) Delete(ctx context.Context, id uuid.UUID) error {
	tr, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTriggerNotFound
		}
		return fmt.Errorf("trigger service: delete lookup: %w", err)
	}
	if tr.CreatedVia != model.TriggerCreatedViaManual {
		return ErrTriggerCannotDeleteDAGManaged
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTriggerNotFound
		}
		return fmt.Errorf("trigger service: delete: %w", err)
	}
	return nil
}

// ListByEmployee returns every trigger row whose acting_employee_id is the
// given employee. Used by the FE employee-detail Triggers tab.
func (s *CRUDService) ListByEmployee(ctx context.Context, employeeID uuid.UUID) ([]*model.WorkflowTrigger, error) {
	rows, err := s.repo.ListByActingEmployee(ctx, employeeID)
	if err != nil {
		return nil, fmt.Errorf("trigger service: list by employee: %w", err)
	}
	if rows == nil {
		rows = []*model.WorkflowTrigger{}
	}
	return rows, nil
}

// Test evaluates the trigger's matchers and input mapping against a sample
// event payload WITHOUT dispatching. The idempotency store is NOT touched.
// Delegates to DryRun (the matching/rendering logic lives in the same
// package as the live router so they can never drift). Returns
// ErrTriggerNotFound when the row is missing.
func (s *CRUDService) Test(ctx context.Context, id uuid.UUID, event map[string]any) (*DryRunResult, error) {
	tr, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTriggerNotFound
		}
		return nil, fmt.Errorf("trigger service: test lookup: %w", err)
	}
	if s.defs != nil && tr.TargetKind == model.TriggerTargetDAG && tr.WorkflowID != nil {
		def, defErr := s.defs.GetByID(ctx, *tr.WorkflowID)
		if defErr != nil || def == nil {
			return &DryRunResult{
				Matched:       true,
				WouldDispatch: false,
				SkipReason:    "target_not_found",
			}, nil
		}
	}
	return DryRun(tr, event), nil
}

// validateActingEmployee enforces the same-project + active-state contract
// the registrar's author-time guard applies. The error sentinel is the same
// regardless of cause (cross-project / archived / unknown) because the FE
// surfaces a single i18n string for "this employee is not bindable".
func (s *CRUDService) validateActingEmployee(
	ctx context.Context,
	employeeID, projectID uuid.UUID,
) error {
	if s.emps == nil {
		// No lookup wired — accept (back-compat with tests that don't exercise this).
		return nil
	}
	emp, err := s.emps.Get(ctx, employeeID)
	if err != nil || emp == nil {
		return ErrTriggerActingEmployeeArchived
	}
	if emp.ProjectID != projectID {
		return ErrTriggerActingEmployeeArchived
	}
	if emp.State == model.EmployeeStateArchived {
		return ErrTriggerActingEmployeeArchived
	}
	return nil
}

// normalizeJSON coerces an empty/nil json.RawMessage into the canonical
// "{}" so downstream Postgres jsonb writes don't choke on NULL.
func normalizeJSON(in json.RawMessage) json.RawMessage {
	if len(in) == 0 {
		return json.RawMessage(`{}`)
	}
	return in
}
