package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeAuditSink struct {
	mu       sync.Mutex
	received []*model.AuditEvent
}

func (f *fakeAuditSink) Enqueue(_ context.Context, event *model.AuditEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = append(f.received, event)
}

func (f *fakeAuditSink) events() []*model.AuditEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*model.AuditEvent(nil), f.received...)
}

func newFakeAuditSvc(validate ActionIDValidator) (*AuditService, *fakeAuditSink) {
	sink := &fakeAuditSink{}
	return NewAuditService(sink, nil, validate), sink
}

func allowAllValidator(string) bool { return true }

func TestAuditService_RecordEvent_RejectsUnknownActionID(t *testing.T) {
	svc, sink := newFakeAuditSvc(func(actionID string) bool {
		return actionID == "task.create"
	})

	err := svc.RecordEvent(context.Background(), &model.AuditEvent{
		ProjectID:    uuid.New(),
		ActionID:     "made.up.action",
		ResourceType: model.AuditResourceTypeTask,
	})
	if !errors.Is(err, ErrUnknownAuditActionID) {
		t.Fatalf("expected ErrUnknownAuditActionID, got %v", err)
	}
	if len(sink.events()) != 0 {
		t.Errorf("sink should be empty after rejected action; got %d events", len(sink.events()))
	}
}

func TestAuditService_RecordEvent_RejectsUnknownResourceType(t *testing.T) {
	svc, sink := newFakeAuditSvc(allowAllValidator)
	err := svc.RecordEvent(context.Background(), &model.AuditEvent{
		ProjectID:    uuid.New(),
		ActionID:     "task.create",
		ResourceType: "not-a-resource",
	})
	if !errors.Is(err, ErrInvalidAuditResourceType) {
		t.Fatalf("expected ErrInvalidAuditResourceType, got %v", err)
	}
	if len(sink.events()) != 0 {
		t.Errorf("sink should be empty; got %d events", len(sink.events()))
	}
}

func TestAuditService_RecordEvent_SanitizesPayloadBeforeEnqueue(t *testing.T) {
	svc, sink := newFakeAuditSvc(allowAllValidator)
	err := svc.RecordEvent(context.Background(), &model.AuditEvent{
		ProjectID:           uuid.New(),
		ActionID:            "member.role.change",
		ResourceType:        model.AuditResourceTypeMember,
		PayloadSnapshotJSON: `{"access_token":"leaked","member":"alice"}`,
	})
	if err != nil {
		t.Fatalf("RecordEvent error = %v", err)
	}
	events := sink.events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event enqueued, got %d", len(events))
	}
	payload := events[0].PayloadSnapshotJSON
	if contains(payload, "leaked") {
		t.Errorf("sensitive token should be redacted in enqueued payload: %s", payload)
	}
	if !contains(payload, "alice") {
		t.Errorf("non-sensitive field should survive sanitization: %s", payload)
	}
	if !contains(payload, AuditRedactionMarker) {
		t.Errorf("expected redaction marker in enqueued payload: %s", payload)
	}
}

func TestAuditService_RecordEvent_AssignsIDWhenMissing(t *testing.T) {
	svc, sink := newFakeAuditSvc(allowAllValidator)
	err := svc.RecordEvent(context.Background(), &model.AuditEvent{
		ProjectID:    uuid.New(),
		ActionID:     "project.read",
		ResourceType: model.AuditResourceTypeProject,
	})
	if err != nil {
		t.Fatalf("RecordEvent error = %v", err)
	}
	events := sink.events()
	if len(events) != 1 || events[0].ID == uuid.Nil {
		t.Errorf("expected enqueued event to have non-nil ID; got %+v", events)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 && stringIndex(haystack, needle) >= 0
}

func stringIndex(haystack, needle string) int {
	// Avoid pulling strings into this file; existing tests don't.
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
