package mods

import (
	"context"
	"testing"

	eb "github.com/agentforge/server/internal/eventbus"
)

func TestEnrich_MetaSpanID_AddedWhenMissing(t *testing.T) {
	enrich := NewEnrich()
	ev := eb.NewEvent("core.test", "test:source", "test:target")
	pc := &eb.PipelineCtx{
		SpanID: "span-123",
	}

	result, err := enrich.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	spanID := eb.GetString(result, eb.MetaSpanID)
	if spanID != "span-123" {
		t.Errorf("Expected SpanID to be 'span-123', got %q", spanID)
	}
}

func TestEnrich_MetaSpanID_KeptWhenAlreadySet(t *testing.T) {
	enrich := NewEnrich()
	ev := eb.NewEvent("core.test", "test:source", "test:target")
	eb.SetString(ev, eb.MetaSpanID, "span-existing")
	pc := &eb.PipelineCtx{
		SpanID: "span-123",
	}

	result, err := enrich.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	spanID := eb.GetString(result, eb.MetaSpanID)
	if spanID != "span-existing" {
		t.Errorf("Expected SpanID to remain 'span-existing', got %q", spanID)
	}
}

func TestEnrich_EmptySpanIDInContext(t *testing.T) {
	enrich := NewEnrich()
	ev := eb.NewEvent("core.test", "test:source", "test:target")
	pc := &eb.PipelineCtx{
		SpanID: "",
	}

	result, err := enrich.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	spanID := eb.GetString(result, eb.MetaSpanID)
	if spanID != "" {
		t.Errorf("Expected SpanID to remain empty, got %q", spanID)
	}
}

func TestEnrich_MetaSpanID_Unchanged(t *testing.T) {
	enrich := NewEnrich()
	ev := eb.NewEvent("core.test", "test:source", "test:target")
	pc := &eb.PipelineCtx{
		SpanID: "span-456",
	}

	// No span ID in event initially
	result, err := enrich.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	spanID := eb.GetString(result, eb.MetaSpanID)
	if spanID != "span-456" {
		t.Errorf("Expected SpanID to be 'span-456', got %q", spanID)
	}

	// Set it and call again with a different span ID - should be unchanged
	pc.SpanID = "span-999"
	result2, err := enrich.Transform(context.Background(), result, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	spanID = eb.GetString(result2, eb.MetaSpanID)
	if spanID != "span-456" {
		t.Errorf("Expected SpanID to remain 'span-456', got %q", spanID)
	}
}

func TestEnrich_Name(t *testing.T) {
	enrich := NewEnrich()
	if enrich.Name() != "core.enrich" {
		t.Errorf("Expected Name to be 'core.enrich', got %q", enrich.Name())
	}
}

func TestEnrich_Intercepts(t *testing.T) {
	enrich := NewEnrich()
	intercepts := enrich.Intercepts()
	if len(intercepts) != 1 || intercepts[0] != "*" {
		t.Errorf("Expected Intercepts to be [*], got %v", intercepts)
	}
}

func TestEnrich_Priority(t *testing.T) {
	enrich := NewEnrich()
	if enrich.Priority() != 10 {
		t.Errorf("Expected Priority to be 10, got %d", enrich.Priority())
	}
}

func TestEnrich_Mode(t *testing.T) {
	enrich := NewEnrich()
	if enrich.Mode() != eb.ModeTransform {
		t.Errorf("Expected Mode to be ModeTransform, got %v", enrich.Mode())
	}
}
