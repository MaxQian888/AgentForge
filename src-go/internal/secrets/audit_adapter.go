package secrets

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
)

// AuditEventEmitter is the narrow contract auditServiceAdapter needs.
// service.AuditService satisfies it via RecordEvent.
type AuditEventEmitter interface {
	Record(ctx context.Context, e *model.AuditEvent) error
}

type auditServiceAdapter struct{ sink AuditEventEmitter }

// NewAuditServiceAdapter wraps an AuditEventEmitter so it implements
// AuditRecorder for the secrets Service.
func NewAuditServiceAdapter(sink AuditEventEmitter) AuditRecorder {
	return &auditServiceAdapter{sink: sink}
}

func (a *auditServiceAdapter) Record(ctx context.Context, projectID uuid.UUID, action, resourceID, payload string, actor *uuid.UUID) {
	if a.sink == nil {
		return
	}
	ev := &model.AuditEvent{
		ProjectID:           projectID,
		OccurredAt:          time.Now().UTC(),
		ActorUserID:         actor,
		ActionID:            action,
		ResourceType:        model.AuditResourceTypeSecret,
		ResourceID:          resourceID,
		PayloadSnapshotJSON: payload,
	}
	_ = a.sink.Record(ctx, ev)
}
