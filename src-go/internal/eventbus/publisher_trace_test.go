package eventbus

import (
	"context"
	"testing"
	"time"

	applog "github.com/agentforge/server/internal/log"
)

// publishAndCollect publishes e via bus and returns the first event seen by a
// Subscribe channel, timing out after 200 ms.
func publishAndCollect(t *testing.T, bus *Bus, ctx context.Context, e *Event) *Event {
	t.Helper()
	subCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ch := bus.Subscribe(subCtx)
	if err := bus.Publish(ctx, e); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	select {
	case got, ok := <-ch:
		if !ok {
			t.Fatal("subscriber channel closed before receiving event")
		}
		return got
	case <-subCtx.Done():
		t.Fatal("timed out waiting for event from subscriber")
		return nil
	}
}

func TestPublish_AttachesTraceIDToMetadata(t *testing.T) {
	bus := NewBus()
	ctx := applog.WithTrace(context.Background(), "tr_bus000000000000000000000")
	e := NewEvent("task.created", "test", "target:1")
	got := publishAndCollect(t, bus, ctx, e)
	if got.Metadata[MetaTraceID] != "tr_bus000000000000000000000" {
		t.Fatalf("missing or wrong trace_id in metadata: %+v", got.Metadata)
	}
}

func TestPublish_PreservesPresetTraceID(t *testing.T) {
	bus := NewBus()
	ctx := applog.WithTrace(context.Background(), "tr_ctx000000000000000000000")
	e := NewEvent("task.updated", "test", "target:1")
	e.Metadata[MetaTraceID] = "tr_preset00000000000000000"
	got := publishAndCollect(t, bus, ctx, e)
	if got.Metadata[MetaTraceID] != "tr_preset00000000000000000" {
		t.Fatalf("preset trace_id must not be overwritten: %+v", got.Metadata)
	}
}

func TestPublish_NoTraceInCtx_MetadataUnchanged(t *testing.T) {
	bus := NewBus()
	// context has no trace; Metadata should not gain a trace_id key
	e := NewEvent("task.deleted", "test", "target:1")
	got := publishAndCollect(t, bus, context.Background(), e)
	if _, ok := got.Metadata[MetaTraceID]; ok {
		t.Fatalf("trace_id should not be set when context has none: %+v", got.Metadata)
	}
}
