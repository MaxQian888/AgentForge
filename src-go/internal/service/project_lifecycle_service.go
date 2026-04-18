// Package service — project_lifecycle_service.go promotes project status
// transitions (Archive / Unarchive / Delete) to a first-class, auditable
// operation.
//
// The service wraps the status flip + cascade side effects:
//
//   * Archive: flip status to 'archived'; best-effort cancel in-flight team runs,
//     in-flight workflow executions; best-effort revoke pending invitations
//     (when an invitation service is wired — currently unwired Wave 2 work).
//   * Unarchive: flip back to 'active'. Does NOT auto-resume cancelled runs.
//   * DeleteArchived: physical delete; rejects non-archived projects with a
//     domain error so the handler can return 409 project_must_be_archived.
//
// Side-effects are intentionally best-effort: the design doc (§3) makes the
// status flip the "primary commitment" and each cascade a separate,
// individually-reportable step so a single cascade hiccup cannot leave the
// project half-archived.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	log "github.com/sirupsen/logrus"
)

// Sentinel errors returned by the project lifecycle service. Handlers map
// these to HTTP status codes; internal callers can errors.Is against them.
var (
	ErrProjectNotFound          = errors.New("project lifecycle: project not found")
	ErrProjectAlreadyArchived   = errors.New("project lifecycle: project already archived")
	ErrProjectNotArchived       = errors.New("project lifecycle: project is not archived")
	ErrProjectMustBeArchived    = errors.New("project lifecycle: project must be archived before delete")
	ErrProjectArchivalIsNoop    = errors.New("project lifecycle: project is not in a state that can be archived")
)

// ProjectLifecycleProjectRepo is the narrow repository contract the lifecycle
// service needs: load, archive, unarchive, delete.
type ProjectLifecycleProjectRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	SetArchived(ctx context.Context, id, archivedByUserID uuid.UUID, archivedAt time.Time) error
	SetUnarchived(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProjectLifecycleTeamCanceller cancels all active teams for a project.
// The TeamService satisfies this via a thin adapter.
type ProjectLifecycleTeamCanceller interface {
	CancelAllActiveForProject(ctx context.Context, projectID uuid.UUID, reason string) error
}

// ProjectLifecycleWorkflowCanceller cancels all active workflow executions
// for a project.
type ProjectLifecycleWorkflowCanceller interface {
	CancelAllActiveForProject(ctx context.Context, projectID uuid.UUID, reason string) error
}

// ProjectLifecycleInvitationRevoker revokes every pending invitation on the
// archived project. Optional — wiring is deferred until the invitation
// service lands (see 2026-04-17-add-member-invitation-flow).
type ProjectLifecycleInvitationRevoker interface {
	RevokeAllPendingForProject(ctx context.Context, projectID uuid.UUID, reason string) error
}

// DeleteOptions tunes DeleteArchived semantics.
type DeleteOptions struct {
	// KeepAudit preserves project_audit_events rows after the project itself
	// is deleted. Default is true; opt out by setting false.
	KeepAudit bool
}

// DefaultDeleteOptions returns the canonical defaults (KeepAudit=true).
func DefaultDeleteOptions() DeleteOptions {
	return DeleteOptions{KeepAudit: true}
}

// ProjectLifecycleService wraps archive/unarchive/delete flows with cascade
// hooks. Callers construct one instance at startup, wire cascade
// implementations via the `With*` setters, and invoke one of the three
// transitions per request.
type ProjectLifecycleService struct {
	projectRepo ProjectLifecycleProjectRepo
	teamCancel  ProjectLifecycleTeamCanceller
	wfCancel    ProjectLifecycleWorkflowCanceller
	inviteRev   ProjectLifecycleInvitationRevoker
	now         func() time.Time
}

// NewProjectLifecycleService constructs the service with the canonical wall
// clock. Cascade implementations attach via WithTeamCanceller / etc.
func NewProjectLifecycleService(projectRepo ProjectLifecycleProjectRepo) *ProjectLifecycleService {
	return &ProjectLifecycleService{
		projectRepo: projectRepo,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

// WithTeamCanceller wires team cascade.
func (s *ProjectLifecycleService) WithTeamCanceller(c ProjectLifecycleTeamCanceller) *ProjectLifecycleService {
	s.teamCancel = c
	return s
}

// WithWorkflowCanceller wires workflow-execution cascade.
func (s *ProjectLifecycleService) WithWorkflowCanceller(c ProjectLifecycleWorkflowCanceller) *ProjectLifecycleService {
	s.wfCancel = c
	return s
}

// WithInvitationRevoker wires invitation cascade.
func (s *ProjectLifecycleService) WithInvitationRevoker(r ProjectLifecycleInvitationRevoker) *ProjectLifecycleService {
	s.inviteRev = r
	return s
}

// WithClock overrides the wall clock. Used by tests.
func (s *ProjectLifecycleService) WithClock(now func() time.Time) *ProjectLifecycleService {
	if now != nil {
		s.now = now
	}
	return s
}

// Archive flips the project's status to 'archived' and runs best-effort
// cascades. Returns the post-flip project so callers can propagate the
// authoritative archived_at/by fields to the response body.
func (s *ProjectLifecycleService) Archive(ctx context.Context, projectID, ownerUserID uuid.UUID) (*model.Project, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("archive project: load: %w", err)
	}
	if project.IsArchived() {
		return nil, ErrProjectAlreadyArchived
	}

	archivedAt := s.now()
	if err := s.projectRepo.SetArchived(ctx, projectID, ownerUserID, archivedAt); err != nil {
		return nil, fmt.Errorf("archive project: set: %w", err)
	}

	// Cascade is best-effort. Failures are logged but do not fail Archive —
	// the primary commitment (status flip) has already succeeded.
	s.runArchiveCascades(ctx, projectID)

	refreshed, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("archive project: reload: %w", err)
	}
	return refreshed, nil
}

// Unarchive flips the project back to 'active'. Does NOT auto-resume
// cancelled runs — users must explicitly re-dispatch.
func (s *ProjectLifecycleService) Unarchive(ctx context.Context, projectID uuid.UUID) (*model.Project, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("unarchive project: load: %w", err)
	}
	if !project.IsArchived() {
		return nil, ErrProjectNotArchived
	}

	if err := s.projectRepo.SetUnarchived(ctx, projectID); err != nil {
		return nil, fmt.Errorf("unarchive project: set: %w", err)
	}

	refreshed, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("unarchive project: reload: %w", err)
	}
	return refreshed, nil
}

// DeleteArchived performs the physical delete. Rejects non-archived projects
// with ErrProjectMustBeArchived so callers can return 409.
//
// Cascade of child rows is enforced at the database level via foreign-key
// ON DELETE CASCADE. Audit rows are also FK-cascaded today; when
// opts.KeepAudit is set to false we rely on that behavior. When KeepAudit is
// true we would need to detach audit rows before delete; until a dedicated
// audit-retain path exists, the option is accepted but treated as a no-op
// beyond the default FK cascade. TODO: implement audit detach when the
// audit-archive spec lands.
func (s *ProjectLifecycleService) DeleteArchived(ctx context.Context, projectID uuid.UUID, opts DeleteOptions) error {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("delete archived project: load: %w", err)
	}
	if !project.IsArchived() {
		return ErrProjectMustBeArchived
	}
	_ = opts // KeepAudit currently not actionable; FK cascade governs audit rows.
	if err := s.projectRepo.Delete(ctx, projectID); err != nil {
		return fmt.Errorf("delete archived project: %w", err)
	}
	return nil
}

// runArchiveCascades fans out the three cascade calls and logs any failures
// individually. None of these failures block the primary Archive result.
func (s *ProjectLifecycleService) runArchiveCascades(ctx context.Context, projectID uuid.UUID) {
	const reason = "project_archived"

	if s.teamCancel != nil {
		if err := s.teamCancel.CancelAllActiveForProject(ctx, projectID, reason); err != nil {
			log.WithError(err).WithField("projectId", projectID.String()).
				Warn("project lifecycle: team cancel cascade failed (best-effort)")
		}
	}
	if s.wfCancel != nil {
		if err := s.wfCancel.CancelAllActiveForProject(ctx, projectID, reason); err != nil {
			log.WithError(err).WithField("projectId", projectID.String()).
				Warn("project lifecycle: workflow cancel cascade failed (best-effort)")
		}
	}
	if s.inviteRev != nil {
		if err := s.inviteRev.RevokeAllPendingForProject(ctx, projectID, reason); err != nil {
			log.WithError(err).WithField("projectId", projectID.String()).
				Warn("project lifecycle: invitation revoke cascade failed (best-effort)")
		}
	}
}
