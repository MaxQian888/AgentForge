package employee

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

// Attribution-guard errors are returned by AttributionGuard methods so callers
// (trigger registrar, dispatch router, step router) can translate them into
// structured non-success outcomes without matching on string content.
var (
	// ErrEmployeeNotFound is returned when an acting-employee identifier does
	// not resolve to any employees row.
	ErrEmployeeNotFound = errors.New("acting employee not found")

	// ErrEmployeeCrossProject is returned when an acting-employee identifier
	// resolves to a row whose project_id differs from the referencing
	// workflow / trigger / plugin's project_id.
	ErrEmployeeCrossProject = errors.New("acting employee belongs to a different project")
)

// AttributionGuard validates that a candidate acting-employee reference is
// safe to use as a run-level attribution default. It exposes two distinct
// check points:
//
//   - ValidateForProject: author-time (trigger sync, node-config save, plugin
//     manifest validation). Rejects cross-project or unknown references.
//     Archived employees are rejected here as well because author-time use of
//     an archived identity is a mistake.
//
//   - ValidateNotArchived: dispatch-time (router, step executor). A trigger
//     saved against an active employee could become stale if the employee is
//     archived later; dispatch-time validation catches that race. Paused
//     employees are deliberately permitted — "paused" is a scheduler concern,
//     not an identity concern (see change design decision 2).
//
// The guard is deliberately a thin wrapper over Repository.Get so it can be
// constructed with the same dependency the Service already holds, avoiding a
// cyclic import between package employee and its consumers.
type AttributionGuard struct {
	repo Repository
}

// NewAttributionGuard constructs a guard backed by the given repository.
// In production this is the same *repository.EmployeeRepository the Service
// uses; tests may substitute an in-memory fake implementing Repository.
func NewAttributionGuard(repo Repository) *AttributionGuard {
	return &AttributionGuard{repo: repo}
}

// ValidateForProject enforces the author-time invariants:
//
//  1. The employee row exists.
//  2. The employee belongs to projectID.
//  3. The employee is not archived.
//
// Returns ErrEmployeeNotFound, ErrEmployeeCrossProject, or ErrEmployeeArchived
// respectively on failure. Paused employees are accepted.
func (g *AttributionGuard) ValidateForProject(ctx context.Context, employeeID uuid.UUID, projectID uuid.UUID) error {
	emp, err := g.lookup(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp.ProjectID != projectID {
		return fmt.Errorf("%w: employee %s is in project %s, expected %s",
			ErrEmployeeCrossProject, emp.ID, emp.ProjectID, projectID)
	}
	if emp.State == model.EmployeeStateArchived {
		return ErrEmployeeArchived
	}
	return nil
}

// ValidateNotArchived enforces the dispatch-time invariant that the employee
// still exists and is not archived. It does NOT check project scope — the
// registrar already bound the trigger to a project at author time, so any
// cross-project reference would have been rejected there.
//
// Paused employees are accepted. Dispatching "as" a paused employee is a
// legitimate attribution choice (see design decision 2).
func (g *AttributionGuard) ValidateNotArchived(ctx context.Context, employeeID uuid.UUID) error {
	emp, err := g.lookup(ctx, employeeID)
	if err != nil {
		return err
	}
	if emp.State == model.EmployeeStateArchived {
		return ErrEmployeeArchived
	}
	return nil
}

// lookup fetches the employee, translating repository-layer ErrNotFound into
// the guard's domain-level ErrEmployeeNotFound.
func (g *AttributionGuard) lookup(ctx context.Context, employeeID uuid.UUID) (*model.Employee, error) {
	emp, err := g.repo.Get(ctx, employeeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrEmployeeNotFound
		}
		return nil, fmt.Errorf("resolve acting employee %s: %w", employeeID, err)
	}
	return emp, nil
}
