package liveartifact

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type stubAgentRunReader struct {
	run *model.AgentRun
	err error
}

func (s *stubAgentRunReader) GetByID(_ context.Context, _ uuid.UUID) (*model.AgentRun, error) {
	return s.run, s.err
}

func viewerPrincipal() model.PrincipalContext {
	return model.PrincipalContext{UserID: uuid.New(), ProjectRole: "viewer"}
}

func refFor(id uuid.UUID) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"kind": "agent_run", "id": id.String()})
	return b
}

func flattenBlocks(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatalf("projection not a block array: %v", err)
	}
	var sb strings.Builder
	for _, b := range blocks {
		content, ok := b["content"].([]any)
		if !ok {
			continue
		}
		for _, item := range content {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := m["text"].(string); ok {
				sb.WriteString(s)
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

func TestAgentRunProjectorRunningRun(t *testing.T) {
	runID := uuid.New()
	run := &model.AgentRun{
		ID:        runID,
		TaskID:    uuid.New(),
		Status:    model.AgentRunStatusRunning,
		Runtime:   "claude_code",
		Provider:  "anthropic",
		Model:     "claude-opus-4",
		StartedAt: time.Now().Add(-2 * time.Minute),
	}
	p := NewAgentRunProjector(&stubAgentRunReader{run: run})

	res, err := p.Project(context.Background(), viewerPrincipal(), uuid.New(), refFor(runID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s (diag=%s)", res.Status, res.Diagnostics)
	}
	text := flattenBlocks(t, res.Projection)
	if !strings.Contains(text, "Status: running") {
		t.Fatalf("projection missing running status: %s", text)
	}
	if !strings.Contains(text, "Duration: —") {
		t.Fatalf("projection should show em-dash duration for running run: %s", text)
	}
	if res.TTLHint == nil || *res.TTLHint != 30*time.Second {
		t.Fatalf("want 30s TTL hint, got %v", res.TTLHint)
	}
}

func TestAgentRunProjectorCompletedRun(t *testing.T) {
	runID := uuid.New()
	started := time.Now().Add(-1*time.Minute - 5*time.Second)
	completed := started.Add(65 * time.Second)
	run := &model.AgentRun{
		ID:           runID,
		TaskID:       uuid.New(),
		Status:       model.AgentRunStatusCompleted,
		Runtime:      "codex",
		Provider:     "openai",
		Model:        "gpt-5",
		StartedAt:    started,
		CompletedAt:  &completed,
		CostUsd:      0.1234,
		InputTokens:  100,
		OutputTokens: 250,
		TurnCount:    4,
	}
	p := NewAgentRunProjector(&stubAgentRunReader{run: run})

	res, err := p.Project(context.Background(), viewerPrincipal(), uuid.New(), refFor(runID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("want StatusOK, got %s", res.Status)
	}
	text := flattenBlocks(t, res.Projection)
	if !strings.Contains(text, "1m 5s") {
		t.Fatalf("projection missing 1m 5s duration: %s", text)
	}
	if !strings.Contains(text, "$0.1234") {
		t.Fatalf("projection missing cost formatting: %s", text)
	}
}

func TestAgentRunProjectorViewOptsParsing(t *testing.T) {
	runID := uuid.New()
	run := &model.AgentRun{
		ID:        runID,
		TaskID:    uuid.New(),
		Status:    model.AgentRunStatusRunning,
		Runtime:   "claude_code",
		Provider:  "anthropic",
		Model:     "claude-opus",
		StartedAt: time.Now(),
	}
	p := NewAgentRunProjector(&stubAgentRunReader{run: run})

	// show_log_lines > 0 -> placeholder paragraph present.
	optsDefault, _ := json.Marshal(map[string]any{"show_log_lines": 10, "show_steps": true})
	res, err := p.Project(context.Background(), viewerPrincipal(), uuid.New(), refFor(runID), optsDefault)
	if err != nil || res.Status != StatusOK {
		t.Fatalf("default opts: unexpected status %s err %v", res.Status, err)
	}
	if !strings.Contains(flattenBlocks(t, res.Projection), "log feed not yet wired") {
		t.Fatalf("expected log placeholder when show_log_lines=10")
	}

	// show_log_lines == 0 -> placeholder absent.
	optsZero, _ := json.Marshal(map[string]any{"show_log_lines": 0, "show_steps": true})
	res, _ = p.Project(context.Background(), viewerPrincipal(), uuid.New(), refFor(runID), optsZero)
	if strings.Contains(flattenBlocks(t, res.Projection), "log feed not yet wired") {
		t.Fatalf("expected log placeholder absent when show_log_lines=0")
	}

	// invalid show_log_lines clamps to default (10) -> placeholder present.
	optsBad, _ := json.Marshal(map[string]any{"show_log_lines": 999})
	res, _ = p.Project(context.Background(), viewerPrincipal(), uuid.New(), refFor(runID), optsBad)
	if !strings.Contains(flattenBlocks(t, res.Projection), "log feed not yet wired") {
		t.Fatalf("expected invalid log lines to clamp to default and keep placeholder")
	}

	// show_steps false -> steps paragraph omitted.
	optsNoSteps, _ := json.Marshal(map[string]any{"show_steps": false, "show_log_lines": 10})
	res, _ = p.Project(context.Background(), viewerPrincipal(), uuid.New(), refFor(runID), optsNoSteps)
	if strings.Contains(flattenBlocks(t, res.Projection), "Steps:") {
		t.Fatalf("expected steps paragraph absent when show_steps=false")
	}
}

func TestAgentRunProjectorForbidden(t *testing.T) {
	runID := uuid.New()
	run := &model.AgentRun{ID: runID, StartedAt: time.Now()}
	p := NewAgentRunProjector(&stubAgentRunReader{run: run})

	pc := model.PrincipalContext{UserID: uuid.New(), ProjectRole: ""}
	res, err := p.Project(context.Background(), pc, uuid.New(), refFor(runID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusForbidden {
		t.Fatalf("want StatusForbidden, got %s", res.Status)
	}
	if len(res.Projection) != 0 {
		t.Fatalf("forbidden result must not leak projection JSON: %s", res.Projection)
	}
}

func TestAgentRunProjectorNotFound(t *testing.T) {
	runID := uuid.New()
	p := NewAgentRunProjector(&stubAgentRunReader{run: nil, err: errors.New("record not found")})

	res, err := p.Project(context.Background(), viewerPrincipal(), uuid.New(), refFor(runID), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Heuristic: nil run -> StatusNotFound (regardless of whether err is set).
	if res.Status != StatusNotFound {
		t.Fatalf("want StatusNotFound, got %s", res.Status)
	}
	if len(res.Projection) != 0 {
		t.Fatalf("not-found result must not carry projection JSON")
	}
}

func TestAgentRunProjectorSubscribeScoped(t *testing.T) {
	p := NewAgentRunProjector(&stubAgentRunReader{})
	runID := uuid.New()
	topics := p.Subscribe(refFor(runID))
	if len(topics) != 6 {
		t.Fatalf("want 6 topics, got %d", len(topics))
	}
	wantEvents := map[string]struct{}{
		"agent.started":     {},
		"agent.completed":   {},
		"agent.failed":      {},
		"agent.output":      {},
		"agent.progress":    {},
		"agent.cost_update": {},
	}
	for _, topic := range topics {
		if _, ok := wantEvents[topic.Event]; !ok {
			t.Errorf("unexpected event %q", topic.Event)
		}
		if topic.Scope["agent_run_id"] != runID.String() {
			t.Errorf("topic %q missing agent_run_id scope: %+v", topic.Event, topic.Scope)
		}
	}
}

func TestAgentRunProjectorSubscribeInvalidRef(t *testing.T) {
	p := NewAgentRunProjector(&stubAgentRunReader{})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Subscribe panicked on invalid ref: %v", r)
		}
	}()
	topics := p.Subscribe(json.RawMessage(`{"kind":"agent_run","id":"not-a-uuid"}`))
	if topics == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(topics) != 0 {
		t.Fatalf("expected empty topic slice, got %d", len(topics))
	}

	topics = p.Subscribe(nil)
	if topics == nil || len(topics) != 0 {
		t.Fatalf("expected non-nil empty slice on nil ref, got %v", topics)
	}
}
