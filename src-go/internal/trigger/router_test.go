package trigger_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/trigger"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockListRepo struct {
	triggers map[model.TriggerSource][]*model.WorkflowTrigger
}

func (m *mockListRepo) ListEnabledBySource(_ context.Context, src model.TriggerSource) ([]*model.WorkflowTrigger, error) {
	return m.triggers[src], nil
}

type starterCall struct {
	WorkflowID uuid.UUID
	Opts       service.StartOptions
}

type mockStarter struct {
	calls   []starterCall
	err     error
	errOnce bool // if true, err fires once then clears
}

func (m *mockStarter) StartExecution(_ context.Context, workflowID uuid.UUID, _ *uuid.UUID, opts service.StartOptions) (*model.WorkflowExecution, error) {
	m.calls = append(m.calls, starterCall{workflowID, opts})
	if m.err != nil {
		fire := m.err
		if m.errOnce {
			m.err = nil
		}
		return nil, fire
	}
	return &model.WorkflowExecution{ID: uuid.New(), WorkflowID: workflowID}, nil
}

// stubIdem is a simple in-memory idempotency store for unit tests.
type stubIdem struct {
	seenKeys map[string]bool
}

func (s *stubIdem) SeenWithin(_ context.Context, key string, _ time.Duration) (bool, error) {
	if s.seenKeys == nil {
		s.seenKeys = map[string]bool{}
	}
	if s.seenKeys[key] {
		return true, nil
	}
	s.seenKeys[key] = true
	return false, nil
}

func newStubIdem() *stubIdem {
	return &stubIdem{seenKeys: map[string]bool{}}
}

// nopIdem never records or deduplicates anything.
func nopIdem() *stubIdem {
	return &stubIdem{}
}

// ---------------------------------------------------------------------------
// Helper to build WorkflowTrigger rows
// ---------------------------------------------------------------------------

func makeIMTrigger(workflowID uuid.UUID, cfgMap map[string]any, inputMapping map[string]any) *model.WorkflowTrigger {
	cfgBytes, _ := json.Marshal(cfgMap)
	mappingBytes, _ := json.Marshal(inputMapping)
	return &model.WorkflowTrigger{
		ID:           uuid.New(),
		WorkflowID:   workflowID,
		Source:       model.TriggerSourceIM,
		Config:       json.RawMessage(cfgBytes),
		InputMapping: json.RawMessage(mappingBytes),
		Enabled:      true,
	}
}

func makeScheduleTrigger(workflowID uuid.UUID) *model.WorkflowTrigger {
	cfgBytes, _ := json.Marshal(map[string]any{})
	mappingBytes, _ := json.Marshal(map[string]any{})
	return &model.WorkflowTrigger{
		ID:           uuid.New(),
		WorkflowID:   workflowID,
		Source:       model.TriggerSourceSchedule,
		Config:       json.RawMessage(cfgBytes),
		InputMapping: json.RawMessage(mappingBytes),
		Enabled:      true,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// 1. TestRouter_IM_MatchesAndStartsExecution — single IM trigger with
// platform/command filters; event matches → starter called once with correct
// WorkflowID, Seed (rendered pr_url), and TriggeredBy == &trigger.ID.
func TestRouter_IM_MatchesAndStartsExecution(t *testing.T) {
	wfID := uuid.New()
	trig := makeIMTrigger(wfID,
		map[string]any{
			"platform": "slack",
			"command":  "/review",
		},
		map[string]any{
			"pr_url": "{{$event.pr_url}}",
		},
	)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	starter := &mockStarter{}
	router := trigger.NewRouter(repo, starter, nopIdem())

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "slack",
			"command":  "/review",
			"pr_url":   "https://github.com/org/repo/pull/42",
		},
	}

	n, err := router.Route(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 execution started, got %d", n)
	}
	if len(starter.calls) != 1 {
		t.Fatalf("expected 1 starter call, got %d", len(starter.calls))
	}
	call := starter.calls[0]
	if call.WorkflowID != wfID {
		t.Errorf("expected workflowID %s, got %s", wfID, call.WorkflowID)
	}
	if call.Opts.TriggeredBy == nil || *call.Opts.TriggeredBy != trig.ID {
		t.Errorf("expected TriggeredBy %s, got %v", trig.ID, call.Opts.TriggeredBy)
	}
	if call.Opts.Seed["pr_url"] != "https://github.com/org/repo/pull/42" {
		t.Errorf("expected pr_url in seed, got %v", call.Opts.Seed)
	}
}

// 2. TestRouter_IM_NoMatchReturnsZero — event platform/command don't match → 0 started.
func TestRouter_IM_NoMatchReturnsZero(t *testing.T) {
	wfID := uuid.New()
	trig := makeIMTrigger(wfID,
		map[string]any{
			"platform": "slack",
			"command":  "/review",
		},
		map[string]any{},
	)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	starter := &mockStarter{}
	router := trigger.NewRouter(repo, starter, nopIdem())

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "teams",
			"command":  "/deploy",
		},
	}

	n, err := router.Route(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 executions started, got %d", n)
	}
	if len(starter.calls) != 0 {
		t.Fatalf("expected 0 starter calls, got %d", len(starter.calls))
	}
}

// 3. TestRouter_IM_RegexFilter — match_regex filter.
func TestRouter_IM_RegexFilter(t *testing.T) {
	wfID := uuid.New()
	trig := makeIMTrigger(wfID,
		map[string]any{
			"match_regex": "^/review .+",
		},
		map[string]any{},
	)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}

	t.Run("matches", func(t *testing.T) {
		starter := &mockStarter{}
		router := trigger.NewRouter(repo, starter, nopIdem())
		ev := trigger.Event{
			Source: model.TriggerSourceIM,
			Data:   map[string]any{"content": "/review https://github.com/pr/1"},
		}
		n, err := router.Route(context.Background(), ev)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 1 {
			t.Errorf("expected 1 execution, got %d", n)
		}
	})

	t.Run("no_match", func(t *testing.T) {
		starter := &mockStarter{}
		router := trigger.NewRouter(repo, starter, nopIdem())
		ev := trigger.Event{
			Source: model.TriggerSourceIM,
			Data:   map[string]any{"content": "/deploy"},
		}
		n, err := router.Route(context.Background(), ev)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 executions, got %d", n)
		}
	})
}

// 4. TestRouter_IM_ChatAllowlist — chat_id not in allowlist → not matched.
func TestRouter_IM_ChatAllowlist(t *testing.T) {
	wfID := uuid.New()
	trig := makeIMTrigger(wfID,
		map[string]any{
			"chat_allowlist": []string{"chat-a", "chat-b"},
		},
		map[string]any{},
	)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	starter := &mockStarter{}
	router := trigger.NewRouter(repo, starter, nopIdem())

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"chat_id": "chat-c"},
	}
	n, err := router.Route(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 executions for chat-c, got %d", n)
	}
}

// 5. TestRouter_Idempotency_SkipsDuplicates — same event fired twice; second is deduped.
func TestRouter_Idempotency_SkipsDuplicates(t *testing.T) {
	wfID := uuid.New()
	cfgBytes, _ := json.Marshal(map[string]any{})
	mappingBytes, _ := json.Marshal(map[string]any{})
	trig := &model.WorkflowTrigger{
		ID:                     uuid.New(),
		WorkflowID:             wfID,
		Source:                 model.TriggerSourceIM,
		Config:                 json.RawMessage(cfgBytes),
		InputMapping:           json.RawMessage(mappingBytes),
		IdempotencyKeyTemplate: "{{$event.id}}",
		DedupeWindowSeconds:    60,
		Enabled:                true,
	}

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	idem := newStubIdem()
	starter := &mockStarter{}
	router := trigger.NewRouter(repo, starter, idem)

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"id": "event-123"},
	}

	// First call: should start 1 execution.
	n1, err := router.Route(context.Background(), ev)
	if err != nil {
		t.Fatalf("first route error: %v", err)
	}
	if n1 != 1 {
		t.Errorf("first call: expected 1 started, got %d", n1)
	}

	// Second call with same event: should skip (duplicate).
	n2, err := router.Route(context.Background(), ev)
	if err != nil {
		t.Fatalf("second route error: %v", err)
	}
	if n2 != 0 {
		t.Errorf("second call: expected 0 started (duplicate), got %d", n2)
	}
}

// 6. TestRouter_InputMappingRendersWholeAndEmbedded — whole vs embedded template rendering.
func TestRouter_InputMappingRendersWholeAndEmbedded(t *testing.T) {
	wfID := uuid.New()
	trig := makeIMTrigger(wfID,
		map[string]any{},
		map[string]any{
			"pr_url": "{{$event.args.0}}",
			"msg":    "PR {{$event.args.0}} ready",
		},
	)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	starter := &mockStarter{}
	router := trigger.NewRouter(repo, starter, nopIdem())

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"args": []any{"https://github.com/org/repo/pull/99"},
		},
	}

	n, err := router.Route(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 execution, got %d", n)
	}

	seed := starter.calls[0].Opts.Seed

	// Whole-template: should preserve the string value from the array element.
	prURL, ok := seed["pr_url"].(string)
	if !ok || prURL != "https://github.com/org/repo/pull/99" {
		t.Errorf("pr_url whole-template: expected string value 'https://github.com/org/repo/pull/99', got %T %v", seed["pr_url"], seed["pr_url"])
	}

	// Embedded: should be stringified in the surrounding string.
	msg, ok := seed["msg"].(string)
	if !ok || msg != "PR https://github.com/org/repo/pull/99 ready" {
		t.Errorf("msg embedded: expected 'PR https://github.com/org/repo/pull/99 ready', got %v", seed["msg"])
	}
}

// 7. TestRouter_Schedule_AlwaysMatches — schedule trigger always fires.
func TestRouter_Schedule_AlwaysMatches(t *testing.T) {
	wfID := uuid.New()
	trig := makeScheduleTrigger(wfID)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceSchedule: {trig},
	}}
	starter := &mockStarter{}
	router := trigger.NewRouter(repo, starter, nopIdem())

	ev := trigger.Event{
		Source: model.TriggerSourceSchedule,
		Data:   map[string]any{"now": "2026-04-19T00:00:00Z"},
	}

	n, err := router.Route(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 execution for schedule trigger, got %d", n)
	}
}

// 8. TestRouter_StarterError_RecordedButContinues — error on first trigger doesn't
// abort the second; started=1, error non-nil.
func TestRouter_StarterError_RecordedButContinues(t *testing.T) {
	wfID1 := uuid.New()
	wfID2 := uuid.New()

	trig1 := makeIMTrigger(wfID1, map[string]any{}, map[string]any{})
	trig2 := makeIMTrigger(wfID2, map[string]any{}, map[string]any{})

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig1, trig2},
	}}

	sentinelErr := errors.New("starter failed")
	starter := &mockStarter{
		err:     sentinelErr,
		errOnce: true, // error fires once on the first call, then clears
	}
	router := trigger.NewRouter(repo, starter, nopIdem())

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{},
	}

	n, err := router.Route(context.Background(), ev)
	if err == nil {
		t.Fatal("expected an error but got nil")
	}
	if n != 1 {
		t.Errorf("expected 1 successful execution, got %d", n)
	}
	if len(starter.calls) != 2 {
		t.Errorf("expected 2 starter calls (both attempted), got %d", len(starter.calls))
	}
}
