package trigger_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/employee"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/trigger"
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

type engineCall struct {
	Trigger *model.WorkflowTrigger
	Seed    map[string]any
}

type mockEngine struct {
	kind    model.TriggerTargetKind
	calls   []engineCall
	err     error
	errOnce bool
}

func newMockEngine(kind model.TriggerTargetKind) *mockEngine {
	return &mockEngine{kind: kind}
}

func (m *mockEngine) Kind() model.TriggerTargetKind { return m.kind }

func (m *mockEngine) Start(_ context.Context, trig *model.WorkflowTrigger, seed map[string]any) (trigger.TriggerRun, error) {
	m.calls = append(m.calls, engineCall{Trigger: trig, Seed: seed})
	if m.err != nil {
		fire := m.err
		if m.errOnce {
			m.err = nil
		}
		return trigger.TriggerRun{}, fire
	}
	return trigger.TriggerRun{Engine: m.kind, RunID: uuid.New()}, nil
}

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
// Helpers
// ---------------------------------------------------------------------------

func makeIMTrigger(workflowID uuid.UUID, cfgMap map[string]any, inputMapping map[string]any) *model.WorkflowTrigger {
	cfgBytes, _ := json.Marshal(cfgMap)
	mappingBytes, _ := json.Marshal(inputMapping)
	wfID := workflowID
	return &model.WorkflowTrigger{
		ID:           uuid.New(),
		WorkflowID:   &wfID,
		Source:       model.TriggerSourceIM,
		TargetKind:   model.TriggerTargetDAG,
		Config:       json.RawMessage(cfgBytes),
		InputMapping: json.RawMessage(mappingBytes),
		Enabled:      true,
	}
}

func makePluginIMTrigger(pluginID string, cfgMap map[string]any, inputMapping map[string]any) *model.WorkflowTrigger {
	cfgBytes, _ := json.Marshal(cfgMap)
	mappingBytes, _ := json.Marshal(inputMapping)
	return &model.WorkflowTrigger{
		ID:           uuid.New(),
		PluginID:     pluginID,
		Source:       model.TriggerSourceIM,
		TargetKind:   model.TriggerTargetPlugin,
		Config:       json.RawMessage(cfgBytes),
		InputMapping: json.RawMessage(mappingBytes),
		Enabled:      true,
	}
}

func makeScheduleTrigger(workflowID uuid.UUID) *model.WorkflowTrigger {
	cfgBytes, _ := json.Marshal(map[string]any{})
	mappingBytes, _ := json.Marshal(map[string]any{})
	wfID := workflowID
	return &model.WorkflowTrigger{
		ID:           uuid.New(),
		WorkflowID:   &wfID,
		Source:       model.TriggerSourceSchedule,
		TargetKind:   model.TriggerTargetDAG,
		Config:       json.RawMessage(cfgBytes),
		InputMapping: json.RawMessage(mappingBytes),
		Enabled:      true,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// DAG: single IM trigger with platform/command filters; event matches → engine
// called once with correct trigger ref + rendered seed; outcome is Started.
func TestRouter_IM_DAG_MatchesAndStartsExecution(t *testing.T) {
	wfID := uuid.New()
	trig := makeIMTrigger(wfID,
		map[string]any{"platform": "slack", "command": "/review"},
		map[string]any{"pr_url": "{{$event.pr_url}}"},
	)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	dag := newMockEngine(model.TriggerTargetDAG)
	router := trigger.NewRouter(repo, nopIdem(), dag)

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "slack",
			"command":  "/review",
			"pr_url":   "https://github.com/org/repo/pull/42",
		},
	}

	outcomes, err := router.RouteWithOutcomes(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Status != trigger.OutcomeStarted {
		t.Fatalf("expected 1 started outcome, got %+v", outcomes)
	}
	if outcomes[0].TargetKind != model.TriggerTargetDAG {
		t.Errorf("expected target_kind=dag on outcome, got %s", outcomes[0].TargetKind)
	}
	if outcomes[0].RunID == nil {
		t.Errorf("expected outcome to carry a run id")
	}
	if len(dag.calls) != 1 {
		t.Fatalf("expected 1 DAG engine call, got %d", len(dag.calls))
	}
	call := dag.calls[0]
	if call.Trigger.ID != trig.ID {
		t.Errorf("engine received wrong trigger: got %s, want %s", call.Trigger.ID, trig.ID)
	}
	if call.Seed["pr_url"] != "https://github.com/org/repo/pull/42" {
		t.Errorf("expected pr_url in seed, got %v", call.Seed)
	}
}

// Plugin-target trigger dispatches through the plugin adapter.
func TestRouter_IM_Plugin_MatchesAndStartsPluginRun(t *testing.T) {
	trig := makePluginIMTrigger("workflow-plugin-x",
		map[string]any{"platform": "slack", "command": "/review"},
		map[string]any{"pr_url": "{{$event.pr_url}}"},
	)

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	pluginEng := newMockEngine(model.TriggerTargetPlugin)
	router := trigger.NewRouter(repo, nopIdem(),
		newMockEngine(model.TriggerTargetDAG), pluginEng)

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data: map[string]any{
			"platform": "slack",
			"command":  "/review",
			"pr_url":   "https://github.com/org/repo/pull/42",
		},
	}

	outcomes, err := router.RouteWithOutcomes(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Status != trigger.OutcomeStarted {
		t.Fatalf("expected 1 started outcome, got %+v", outcomes)
	}
	if outcomes[0].TargetKind != model.TriggerTargetPlugin {
		t.Errorf("expected target_kind=plugin, got %s", outcomes[0].TargetKind)
	}
	if len(pluginEng.calls) != 1 {
		t.Fatalf("expected 1 plugin engine call, got %d", len(pluginEng.calls))
	}
	if pluginEng.calls[0].Trigger.PluginID != "workflow-plugin-x" {
		t.Errorf("adapter received wrong plugin id: %s", pluginEng.calls[0].Trigger.PluginID)
	}
}

// Unknown target kind → failed_unknown_target outcome, no run started.
func TestRouter_IM_UnknownTargetKind_FailsStructured(t *testing.T) {
	trig := makeIMTrigger(uuid.New(),
		map[string]any{"platform": "slack", "command": "/review"},
		map[string]any{},
	)
	trig.TargetKind = model.TriggerTargetKind("webhook-runtime") // not registered

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	dag := newMockEngine(model.TriggerTargetDAG)
	router := trigger.NewRouter(repo, nopIdem(), dag)

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"platform": "slack", "command": "/review"},
	}

	outcomes, err := router.RouteWithOutcomes(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outcomes))
	}
	if outcomes[0].Status != trigger.OutcomeFailedUnknownTarget {
		t.Errorf("expected failed_unknown_target, got %s", outcomes[0].Status)
	}
	if outcomes[0].RunID != nil {
		t.Errorf("expected no run id on failed outcome")
	}
	if len(dag.calls) != 0 {
		t.Errorf("DAG engine should not have been called")
	}
}

// Idempotency collides across engines: two triggers (DAG + plugin) share a key;
// only one fires, other is skipped_idempotent.
func TestRouter_Idempotency_SharedKeyAcrossEngines(t *testing.T) {
	dagTrig := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	dagTrig.IdempotencyKeyTemplate = "{{$event.id}}"
	dagTrig.DedupeWindowSeconds = 60

	pluginTrig := makePluginIMTrigger("plug", map[string]any{}, map[string]any{})
	pluginTrig.IdempotencyKeyTemplate = "{{$event.id}}"
	pluginTrig.DedupeWindowSeconds = 60

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {dagTrig, pluginTrig},
	}}
	dagEng := newMockEngine(model.TriggerTargetDAG)
	pluginEng := newMockEngine(model.TriggerTargetPlugin)
	router := trigger.NewRouter(repo, newStubIdem(), dagEng, pluginEng)

	ev := trigger.Event{Source: model.TriggerSourceIM, Data: map[string]any{"id": "shared"}}

	outcomes, err := router.RouteWithOutcomes(context.Background(), ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(outcomes))
	}

	started := 0
	skipped := 0
	for _, o := range outcomes {
		switch o.Status {
		case trigger.OutcomeStarted:
			started++
		case trigger.OutcomeSkippedIdempotent:
			skipped++
		default:
			t.Errorf("unexpected status: %s", o.Status)
		}
	}
	if started != 1 || skipped != 1 {
		t.Errorf("expected exactly one started + one idempotent skip; got started=%d skipped=%d", started, skipped)
	}
	if len(dagEng.calls)+len(pluginEng.calls) != 1 {
		t.Errorf("expected exactly one engine invocation in total, got dag=%d plugin=%d",
			len(dagEng.calls), len(pluginEng.calls))
	}
}

// Two triggers with identical input_mapping targeting different engines
// receive identical seeds from the same event.
func TestRouter_InputMappingParityAcrossEngines(t *testing.T) {
	dagTrig := makeIMTrigger(uuid.New(), map[string]any{},
		map[string]any{"url": "{{$event.url}}"})
	pluginTrig := makePluginIMTrigger("plug", map[string]any{},
		map[string]any{"url": "{{$event.url}}"})

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {dagTrig, pluginTrig},
	}}
	dagEng := newMockEngine(model.TriggerTargetDAG)
	pluginEng := newMockEngine(model.TriggerTargetPlugin)
	router := trigger.NewRouter(repo, nopIdem(), dagEng, pluginEng)

	ev := trigger.Event{Source: model.TriggerSourceIM, Data: map[string]any{"url": "https://x/y"}}

	if _, err := router.RouteWithOutcomes(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dagEng.calls) != 1 || len(pluginEng.calls) != 1 {
		t.Fatalf("expected both engines to fire once, got dag=%d plugin=%d",
			len(dagEng.calls), len(pluginEng.calls))
	}
	if dagEng.calls[0].Seed["url"] != pluginEng.calls[0].Seed["url"] {
		t.Errorf("seed mismatch across engines: dag=%v plugin=%v",
			dagEng.calls[0].Seed, pluginEng.calls[0].Seed)
	}
}

// Back-compat: Route() returns integer count of started dispatches.
func TestRouter_Route_ReturnsStartedCount(t *testing.T) {
	trig := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	router := trigger.NewRouter(repo, nopIdem(), newMockEngine(model.TriggerTargetDAG))

	n, err := router.Route(context.Background(), trigger.Event{Source: model.TriggerSourceIM, Data: map[string]any{}})
	if err != nil || n != 1 {
		t.Fatalf("expected 1 started, got %d err=%v", n, err)
	}
}

// Regex + allowlist filters still apply.
func TestRouter_IM_RegexFilter(t *testing.T) {
	trig := makeIMTrigger(uuid.New(),
		map[string]any{"match_regex": "^/review .+"},
		map[string]any{},
	)
	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}

	t.Run("matches", func(t *testing.T) {
		eng := newMockEngine(model.TriggerTargetDAG)
		router := trigger.NewRouter(repo, nopIdem(), eng)
		n, err := router.Route(context.Background(), trigger.Event{
			Source: model.TriggerSourceIM,
			Data:   map[string]any{"content": "/review https://github.com/pr/1"},
		})
		if err != nil || n != 1 {
			t.Errorf("expected 1 started, got %d err=%v", n, err)
		}
	})

	t.Run("no_match", func(t *testing.T) {
		eng := newMockEngine(model.TriggerTargetDAG)
		router := trigger.NewRouter(repo, nopIdem(), eng)
		n, err := router.Route(context.Background(), trigger.Event{
			Source: model.TriggerSourceIM,
			Data:   map[string]any{"content": "/deploy"},
		})
		if err != nil || n != 0 {
			t.Errorf("expected 0 started, got %d err=%v", n, err)
		}
	})
}

func TestRouter_IM_ChatAllowlist(t *testing.T) {
	trig := makeIMTrigger(uuid.New(),
		map[string]any{"chat_allowlist": []string{"chat-a", "chat-b"}},
		map[string]any{},
	)
	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	eng := newMockEngine(model.TriggerTargetDAG)
	router := trigger.NewRouter(repo, nopIdem(), eng)

	n, err := router.Route(context.Background(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"chat_id": "chat-c"},
	})
	if err != nil || n != 0 {
		t.Errorf("expected 0 started for chat-c, got %d err=%v", n, err)
	}
}

// Whole-template vs embedded rendering parity.
func TestRouter_InputMappingRendersWholeAndEmbedded(t *testing.T) {
	trig := makeIMTrigger(uuid.New(),
		map[string]any{},
		map[string]any{
			"pr_url": "{{$event.args.0}}",
			"msg":    "PR {{$event.args.0}} ready",
		},
	)
	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	eng := newMockEngine(model.TriggerTargetDAG)
	router := trigger.NewRouter(repo, nopIdem(), eng)

	ev := trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"args": []any{"https://github.com/org/repo/pull/99"}},
	}
	if _, err := router.Route(context.Background(), ev); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(eng.calls) != 1 {
		t.Fatalf("expected 1 engine call, got %d", len(eng.calls))
	}
	seed := eng.calls[0].Seed
	if seed["pr_url"] != "https://github.com/org/repo/pull/99" {
		t.Errorf("whole-template pr_url mismatch: got %v", seed["pr_url"])
	}
	if seed["msg"] != "PR https://github.com/org/repo/pull/99 ready" {
		t.Errorf("embedded-template msg mismatch: got %v", seed["msg"])
	}
}

// Schedule-source triggers always match.
func TestRouter_Schedule_AlwaysMatches(t *testing.T) {
	trig := makeScheduleTrigger(uuid.New())
	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceSchedule: {trig},
	}}
	router := trigger.NewRouter(repo, nopIdem(), newMockEngine(model.TriggerTargetDAG))

	n, err := router.Route(context.Background(), trigger.Event{
		Source: model.TriggerSourceSchedule,
		Data:   map[string]any{"now": "2026-04-19T00:00:00Z"},
	})
	if err != nil || n != 1 {
		t.Errorf("expected 1 scheduled dispatch, got %d err=%v", n, err)
	}
}

// ---------------------------------------------------------------------------
// Attribution guard tests (Section 3.5)
// ---------------------------------------------------------------------------

type stubAttributionGuard struct {
	// key: employeeID -> error to return from ValidateNotArchived.
	// empty error means the employee validates successfully.
	errs  map[uuid.UUID]error
	calls []uuid.UUID
}

func (s *stubAttributionGuard) ValidateNotArchived(_ context.Context, employeeID uuid.UUID) error {
	s.calls = append(s.calls, employeeID)
	if s.errs == nil {
		return nil
	}
	return s.errs[employeeID]
}

// Archived acting-employee: router emits OutcomeFailedActingEmployee, no
// engine call, idempotency key not consumed so a retry is possible.
func TestRouter_ActingEmployee_ArchivedBlocksDispatch(t *testing.T) {
	empID := uuid.New()
	trig := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	trig.ActingEmployeeID = &empID
	trig.IdempotencyKeyTemplate = "{{$event.id}}"
	trig.DedupeWindowSeconds = 60

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	eng := newMockEngine(model.TriggerTargetDAG)
	idem := newStubIdem()
	guard := &stubAttributionGuard{errs: map[uuid.UUID]error{empID: employee.ErrEmployeeArchived}}
	router := trigger.NewRouter(repo, idem, eng).WithAttributionGuard(guard)

	outcomes, err := router.RouteWithOutcomes(context.Background(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{"id": "evt-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Status != trigger.OutcomeFailedActingEmployee {
		t.Fatalf("expected OutcomeFailedActingEmployee, got %+v", outcomes)
	}
	if len(eng.calls) != 0 {
		t.Errorf("engine should not have been called; got %d", len(eng.calls))
	}
	// Idempotency key must NOT have been consumed on guard failure.
	if idem.seenKeys["evt-1"] {
		t.Errorf("idempotency key should not have been consumed on guard failure")
	}
}

// Paused acting-employee dispatches successfully — paused is a scheduler
// concern, not an identity concern (design decision 2).
func TestRouter_ActingEmployee_PausedPermitsDispatch(t *testing.T) {
	empID := uuid.New()
	trig := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	trig.ActingEmployeeID = &empID

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	eng := newMockEngine(model.TriggerTargetDAG)
	// Guard returns nil (paused is accepted).
	guard := &stubAttributionGuard{}
	router := trigger.NewRouter(repo, nopIdem(), eng).WithAttributionGuard(guard)

	outcomes, err := router.RouteWithOutcomes(context.Background(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Status != trigger.OutcomeStarted {
		t.Fatalf("expected OutcomeStarted, got %+v", outcomes)
	}
	if len(eng.calls) != 1 {
		t.Fatalf("expected 1 engine call, got %d", len(eng.calls))
	}
	if eng.calls[0].Trigger.ActingEmployeeID == nil || *eng.calls[0].Trigger.ActingEmployeeID != empID {
		t.Errorf("engine adapter received wrong acting employee id: %v", eng.calls[0].Trigger.ActingEmployeeID)
	}
}

// Null acting-employee: guard is not invoked; dispatch proceeds normally.
func TestRouter_ActingEmployee_NullSkipsGuard(t *testing.T) {
	trig := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	// No ActingEmployeeID on trigger.

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	eng := newMockEngine(model.TriggerTargetDAG)
	guard := &stubAttributionGuard{}
	router := trigger.NewRouter(repo, nopIdem(), eng).WithAttributionGuard(guard)

	if _, err := router.RouteWithOutcomes(context.Background(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(guard.calls) != 0 {
		t.Errorf("expected guard not to be called for nil acting employee; got %d calls", len(guard.calls))
	}
	if len(eng.calls) != 1 {
		t.Errorf("expected 1 engine call, got %d", len(eng.calls))
	}
}

// Unknown acting-employee id: guard returns ErrEmployeeNotFound; router emits
// failed_acting_employee outcome.
func TestRouter_ActingEmployee_UnknownIDBlocksDispatch(t *testing.T) {
	empID := uuid.New()
	trig := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	trig.ActingEmployeeID = &empID

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	eng := newMockEngine(model.TriggerTargetDAG)
	guard := &stubAttributionGuard{errs: map[uuid.UUID]error{empID: employee.ErrEmployeeNotFound}}
	router := trigger.NewRouter(repo, nopIdem(), eng).WithAttributionGuard(guard)

	outcomes, err := router.RouteWithOutcomes(context.Background(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Status != trigger.OutcomeFailedActingEmployee {
		t.Fatalf("expected OutcomeFailedActingEmployee, got %+v", outcomes)
	}
	if len(eng.calls) != 0 {
		t.Errorf("engine must not fire when guard rejects employee")
	}
}

// Happy path: active employee dispatches and the adapter receives the id.
func TestRouter_ActingEmployee_ActiveForwardsToAdapter(t *testing.T) {
	empID := uuid.New()
	trig := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	trig.ActingEmployeeID = &empID

	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig},
	}}
	eng := newMockEngine(model.TriggerTargetDAG)
	router := trigger.NewRouter(repo, nopIdem(), eng).
		WithAttributionGuard(&stubAttributionGuard{})

	outcomes, err := router.RouteWithOutcomes(context.Background(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].Status != trigger.OutcomeStarted {
		t.Fatalf("expected OutcomeStarted, got %+v", outcomes)
	}
	if len(eng.calls) != 1 {
		t.Fatalf("expected 1 engine call, got %d", len(eng.calls))
	}
	got := eng.calls[0].Trigger.ActingEmployeeID
	if got == nil || *got != empID {
		t.Errorf("engine adapter did not receive acting employee id: %v", got)
	}
}

// Engine error on one trigger does not abort the next; outcome records the failure.
func TestRouter_EngineError_RecordedButContinues(t *testing.T) {
	trig1 := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	trig2 := makeIMTrigger(uuid.New(), map[string]any{}, map[string]any{})
	repo := &mockListRepo{triggers: map[model.TriggerSource][]*model.WorkflowTrigger{
		model.TriggerSourceIM: {trig1, trig2},
	}}

	sentinel := errors.New("engine down")
	eng := newMockEngine(model.TriggerTargetDAG)
	eng.err = sentinel
	eng.errOnce = true
	router := trigger.NewRouter(repo, nopIdem(), eng)

	outcomes, err := router.RouteWithOutcomes(context.Background(), trigger.Event{
		Source: model.TriggerSourceIM,
		Data:   map[string]any{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(outcomes))
	}

	var sawStart, sawFail bool
	for _, o := range outcomes {
		if o.Status == trigger.OutcomeStarted {
			sawStart = true
		}
		if o.Status == trigger.OutcomeFailedEngineStart {
			sawFail = true
			if o.Reason != sentinel.Error() {
				t.Errorf("expected reason=%q on failed outcome, got %q", sentinel.Error(), o.Reason)
			}
		}
	}
	if !sawStart || !sawFail {
		t.Errorf("expected both started + failed outcomes; got started=%v failed=%v", sawStart, sawFail)
	}
}
