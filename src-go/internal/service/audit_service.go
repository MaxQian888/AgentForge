// Package service — audit_service.go is the policy layer between event
// emission points and persistence. It validates ActionID against the RBAC
// matrix, sanitizes the payload, and forwards to the AuditSink for
// best-effort writes.
//
// AuditService never blocks the caller's main path: RecordEvent always
// returns quickly. Persistence happens asynchronously inside the sink.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// ErrUnknownAuditActionID indicates the caller passed an ActionID not
// declared in the canonical RBAC matrix. We fail closed here so accidental
// typos do not silently land rows under undeclared actions.
var ErrUnknownAuditActionID = errors.New("audit: action_id is not in the canonical ActionID enum")

// ErrInvalidAuditResourceType indicates the caller passed a resource_type
// outside the documented enum. The DB CHECK constraint also enforces this,
// but service-level rejection produces a clearer error path.
var ErrInvalidAuditResourceType = errors.New("audit: resource_type is not in the canonical enum")

// AuditEventSink is the consumer interface RecordEvent forwards to. The
// real implementation is the eventbus AuditSink with retry queue + spill;
// tests substitute a synchronous in-memory sink.
type AuditEventSink interface {
	Enqueue(ctx context.Context, event *model.AuditEvent)
}

// ActionIDValidator returns true when the given action_id is declared in
// the canonical RBAC matrix. The audit service depends on this contract
// (rather than importing middleware directly) to avoid an import cycle —
// middleware/jwt.go already imports service.Claims.
type ActionIDValidator func(actionID string) bool

// AuditEventReader is the read-side contract used by the query API.
type AuditEventReader interface {
	GetByID(ctx context.Context, projectID, eventID uuid.UUID) (*model.AuditEvent, error)
	List(
		ctx context.Context,
		projectID uuid.UUID,
		filters model.AuditEventQueryFilters,
		cursor string,
		limit int,
	) ([]*model.AuditEvent, string, error)
}

// AuditService is the seam handlers and the eventbus consumer use.
type AuditService struct {
	sink            AuditEventSink
	reader          AuditEventReader
	validateAction  ActionIDValidator
}

// NewAuditService constructs the service. validate is called for every
// RecordEvent; pass appMiddleware-backed validator at wiring time.
// A nil validator is permitted — the service then accepts any non-empty
// action_id (used by tests that don't want to wire the matrix).
func NewAuditService(sink AuditEventSink, reader AuditEventReader, validate ActionIDValidator) *AuditService {
	return &AuditService{sink: sink, reader: reader, validateAction: validate}
}

// RecordEvent validates and enqueues a single audit event. Sanitizes the
// payload in-place before handing off so the sink never sees raw secrets.
//
// Returns nil for invalid ActionIDs / ResourceTypes the caller should NOT
// silently drop — the service prefers explicit feedback at the emission
// site over silent loss. Once enqueued, persistence failures are handled
// by the sink's retry/spill machinery.
func (s *AuditService) RecordEvent(ctx context.Context, event *model.AuditEvent) error {
	if event == nil {
		return fmt.Errorf("audit: event is nil")
	}
	if event.ActionID == "" {
		return fmt.Errorf("%w: empty action_id", ErrUnknownAuditActionID)
	}
	if s.validateAction != nil && !s.validateAction(event.ActionID) {
		return fmt.Errorf("%w: %q", ErrUnknownAuditActionID, event.ActionID)
	}
	if !model.IsValidAuditResourceType(event.ResourceType) {
		return fmt.Errorf("%w: %q", ErrInvalidAuditResourceType, event.ResourceType)
	}
	// Sanitize once at the boundary; the sink and persistence layer never
	// re-touch the payload.
	event.PayloadSnapshotJSON = SanitizeAuditPayloadJSON(event.PayloadSnapshotJSON)
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if s.sink != nil {
		s.sink.Enqueue(ctx, event)
	}
	return nil
}

// Query passes through to the reader. Lives on the service so handlers
// don't take a direct dependency on the repository.
func (s *AuditService) Query(
	ctx context.Context,
	projectID uuid.UUID,
	filters model.AuditEventQueryFilters,
	cursor string,
	limit int,
) ([]*model.AuditEvent, string, error) {
	if s.reader == nil {
		return nil, "", fmt.Errorf("audit: query reader not wired")
	}
	return s.reader.List(ctx, projectID, filters, cursor, limit)
}

// GetByID returns a single event detail. Same handler-facing contract as
// the repo but goes through the service for potential future hooks
// (caching, cross-event correlation, etc).
func (s *AuditService) GetByID(ctx context.Context, projectID, eventID uuid.UUID) (*model.AuditEvent, error) {
	if s.reader == nil {
		return nil, fmt.Errorf("audit: query reader not wired")
	}
	return s.reader.GetByID(ctx, projectID, eventID)
}
