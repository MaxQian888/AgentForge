package secrets_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/secrets"
)

type capturedSink struct{ events []*model.AuditEvent }

func (c *capturedSink) Record(_ context.Context, e *model.AuditEvent) error {
	c.events = append(c.events, e)
	return nil
}

func TestAuditServiceAdapter_EmitsResourceTypeSecret(t *testing.T) {
	sink := &capturedSink{}
	rec := secrets.NewAuditServiceAdapter(sink)

	proj := uuid.New()
	actor := uuid.New()
	rec.Record(context.Background(), proj, "secret.create", "GITHUB_TOKEN", `{"name":"GITHUB_TOKEN","op":"secret.create"}`, &actor)

	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink.events))
	}
	ev := sink.events[0]
	if ev.ResourceType != model.AuditResourceTypeSecret {
		t.Errorf("resource_type = %s want secret", ev.ResourceType)
	}
	if ev.ActionID != "secret.create" {
		t.Errorf("action_id = %s", ev.ActionID)
	}
	if ev.ResourceID != "GITHUB_TOKEN" {
		t.Errorf("resource_id = %s", ev.ResourceID)
	}
}
