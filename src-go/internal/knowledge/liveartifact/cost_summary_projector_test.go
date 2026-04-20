package liveartifact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// stubCostRunReader is an in-memory costRunReader fixture.
type stubCostRunReader struct {
	runs []*model.AgentRun
	err  error
}

func (s *stubCostRunReader) ListByProject(_ context.Context, _ uuid.UUID) ([]*model.AgentRun, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.runs, nil
}

// --- helpers ---

func mustEditor() model.PrincipalContext {
	return model.PrincipalContext{UserID: uuid.New(), ProjectRole: "editor"}
}

func mustViewer() model.PrincipalContext {
	return model.PrincipalContext{UserID: uuid.New(), ProjectRole: "viewer"}
}

func mustTargetRef(t *testing.T, start, end time.Time, runtime, provider, memberID string) json.RawMessage {
	t.Helper()
	body := map[string]any{
		"kind": string(KindCostSummary),
		"filter": map[string]any{
			"range_start": start.Format(time.RFC3339),
			"range_end":   end.Format(time.RFC3339),
		},
	}
	f := body["filter"].(map[string]any)
	if runtime != "" {
		f["runtime"] = runtime
	}
	if provider != "" {
		f["provider"] = provider
	}
	if memberID != "" {
		f["member_id"] = memberID
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal target_ref: %v", err)
	}
	return raw
}

func mustViewOpts(t *testing.T, groupBy string, topN int) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"group_by": groupBy, "top_n": topN})
	if err != nil {
		t.Fatalf("marshal view_opts: %v", err)
	}
	return raw
}

func run(startedAt time.Time, runtime, provider string, memberID uuid.UUID, cost float64) *model.AgentRun {
	return &model.AgentRun{
		ID:        uuid.New(),
		MemberID:  memberID,
		Runtime:   runtime,
		Provider:  provider,
		StartedAt: startedAt,
		CostUsd:   cost,
	}
}

// decodes the projection fragment to a slice of block maps.
func decodeBlocks(t *testing.T, raw json.RawMessage) []map[string]any {
	t.Helper()
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		t.Fatalf("decode projection: %v", err)
	}
	return blocks
}

// blockText returns the first inline text of the block, if any.
func blockText(b map[string]any) string {
	content, ok := b["content"].([]any)
	if !ok {
		return ""
	}
	if len(content) == 0 {
		return ""
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		return ""
	}
	s, _ := first["text"].(string)
	return s
}

// --- tests ---

func TestCostSummaryProjector_GroupByRuntime(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-2 * time.Hour)
	end := now.Add(-1 * time.Minute)
	member := uuid.New()

	reader := &stubCostRunReader{runs: []*model.AgentRun{
		run(start.Add(10*time.Minute), "claude_code", "anthropic", member, 0.50),
		run(start.Add(20*time.Minute), "claude_code", "anthropic", member, 0.25),
		run(start.Add(30*time.Minute), "codex", "openai", member, 1.00),
	}}

	p := NewCostSummaryProjector(reader)
	res, err := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "runtime", 5),
	)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("status=%s diag=%s", res.Status, res.Diagnostics)
	}
	blocks := decodeBlocks(t, res.Projection)

	// collect group rows (paragraphs that contain "runtime=")
	var rows []string
	for _, b := range blocks {
		text := blockText(b)
		if strings.HasPrefix(text, "runtime=") {
			rows = append(rows, text)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 group rows, got %d (blocks=%+v)", len(rows), blocks)
	}
	// codex ($1.00) should be first, claude_code ($0.75) second.
	if !strings.HasPrefix(rows[0], "runtime=codex") {
		t.Fatalf("first row should be codex, got %q", rows[0])
	}
	if !strings.HasPrefix(rows[1], "runtime=claude_code") {
		t.Fatalf("second row should be claude_code, got %q", rows[1])
	}
}

func TestCostSummaryProjector_GroupByProvider(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-3 * time.Hour)
	end := now.Add(-1 * time.Minute)
	member := uuid.New()

	reader := &stubCostRunReader{runs: []*model.AgentRun{
		run(start.Add(1*time.Minute), "claude_code", "anthropic", member, 1.00),
		run(start.Add(2*time.Minute), "codex", "openai", member, 2.00),
		run(start.Add(3*time.Minute), "codex", "openai", member, 3.00),
	}}

	p := NewCostSummaryProjector(reader)
	res, _ := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "provider", 5),
	)
	if res.Status != StatusOK {
		t.Fatalf("status=%s diag=%s", res.Status, res.Diagnostics)
	}
	blocks := decodeBlocks(t, res.Projection)
	var rows []string
	for _, b := range blocks {
		text := blockText(b)
		if strings.HasPrefix(text, "provider=") {
			rows = append(rows, text)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 group rows, got %d", len(rows))
	}
	if !strings.HasPrefix(rows[0], "provider=openai") {
		t.Fatalf("first row should be openai, got %q", rows[0])
	}
}

func TestCostSummaryProjector_GroupByMember(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-3 * time.Hour)
	end := now.Add(-1 * time.Minute)
	m1 := uuid.New()
	m2 := uuid.New()

	reader := &stubCostRunReader{runs: []*model.AgentRun{
		run(start.Add(1*time.Minute), "claude_code", "anthropic", m1, 0.10),
		run(start.Add(2*time.Minute), "claude_code", "anthropic", m2, 2.00),
	}}

	p := NewCostSummaryProjector(reader)
	res, _ := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "member", 5),
	)
	if res.Status != StatusOK {
		t.Fatalf("status=%s", res.Status)
	}
	blocks := decodeBlocks(t, res.Projection)
	var rows []string
	for _, b := range blocks {
		text := blockText(b)
		if strings.HasPrefix(text, "member=") {
			rows = append(rows, text)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 group rows, got %d", len(rows))
	}
	if !strings.Contains(rows[0], m2.String()) {
		t.Fatalf("first row should be m2 (higher cost), got %q", rows[0])
	}
}

func TestCostSummaryProjector_DeltaComputation(t *testing.T) {
	now := time.Now().UTC()
	// windows of 1 hour each.
	end := now.Add(-1 * time.Minute)
	start := end.Add(-1 * time.Hour)
	priorStart := start.Add(-1 * time.Hour)
	member := uuid.New()

	reader := &stubCostRunReader{runs: []*model.AgentRun{
		// prior window: totaling $1.00
		run(priorStart.Add(1*time.Minute), "claude_code", "anthropic", member, 0.40),
		run(priorStart.Add(2*time.Minute), "claude_code", "anthropic", member, 0.60),
		// current window: totaling $1.50
		run(start.Add(1*time.Minute), "claude_code", "anthropic", member, 0.50),
		run(start.Add(2*time.Minute), "claude_code", "anthropic", member, 1.00),
	}}

	p := NewCostSummaryProjector(reader)
	res, _ := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "", 5),
	)
	if res.Status != StatusOK {
		t.Fatalf("status=%s diag=%s", res.Status, res.Diagnostics)
	}
	blocks := decodeBlocks(t, res.Projection)
	var totalLine string
	for _, b := range blocks {
		text := blockText(b)
		if strings.HasPrefix(text, "Total:") {
			totalLine = text
		}
	}
	if totalLine == "" {
		t.Fatalf("no Total line found; blocks=%+v", blocks)
	}
	if !strings.Contains(totalLine, "$1.5000") {
		t.Fatalf("expected current total $1.5000 in %q", totalLine)
	}
	if !strings.Contains(totalLine, "+0.5000") {
		t.Fatalf("expected delta +0.5000 in %q", totalLine)
	}
	if !strings.Contains(totalLine, "+50.0%") {
		t.Fatalf("expected delta_pct +50.0%% in %q", totalLine)
	}
}

func TestCostSummaryProjector_DeltaPriorZero(t *testing.T) {
	now := time.Now().UTC()
	end := now.Add(-1 * time.Minute)
	start := end.Add(-1 * time.Hour)
	member := uuid.New()

	reader := &stubCostRunReader{runs: []*model.AgentRun{
		// Only runs in the current window; prior window is empty.
		run(start.Add(1*time.Minute), "claude_code", "anthropic", member, 0.75),
	}}

	p := NewCostSummaryProjector(reader)
	res, _ := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "", 5),
	)
	if res.Status != StatusOK {
		t.Fatalf("status=%s diag=%s", res.Status, res.Diagnostics)
	}
	blocks := decodeBlocks(t, res.Projection)
	var totalLine string
	for _, b := range blocks {
		text := blockText(b)
		if strings.HasPrefix(text, "Total:") {
			totalLine = text
		}
	}
	// Zero prior + non-zero current -> "n/a" per formatDeltaPct contract.
	if !strings.Contains(totalLine, "(n/a)") {
		t.Fatalf("expected delta_pct n/a when prior is zero, got %q", totalLine)
	}
}

func TestCostSummaryProjector_Forbidden(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	end := now.Add(-1 * time.Minute)
	member := uuid.New()

	reader := &stubCostRunReader{runs: []*model.AgentRun{
		run(start.Add(1*time.Minute), "claude_code", "anthropic", member, 9.99),
	}}

	p := NewCostSummaryProjector(reader)
	res, err := p.Project(
		context.Background(),
		mustViewer(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "runtime", 5),
	)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if res.Status != StatusForbidden {
		t.Fatalf("want forbidden, got %s", res.Status)
	}
	if len(res.Projection) != 0 {
		t.Fatalf("projection must not leak on forbidden; got %s", string(res.Projection))
	}
}

func TestCostSummaryProjector_ZeroSpendWindow(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-2 * time.Hour)
	end := now.Add(-1 * time.Hour)

	reader := &stubCostRunReader{runs: nil}

	p := NewCostSummaryProjector(reader)
	res, _ := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "runtime", 5),
	)
	if res.Status != StatusOK {
		t.Fatalf("status=%s", res.Status)
	}
	blocks := decodeBlocks(t, res.Projection)
	var found bool
	var sawTotal bool
	for _, b := range blocks {
		text := blockText(b)
		if strings.Contains(text, "No cost recorded in this window.") {
			found = true
		}
		if strings.HasPrefix(text, "Total:") {
			sawTotal = true
		}
	}
	if !found {
		t.Fatalf("expected no-cost paragraph in %+v", blocks)
	}
	if sawTotal {
		t.Fatalf("zero-spend window must skip the delta/total line")
	}
}

func TestCostSummaryProjector_PartialWindowEndInFuture(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour) // future
	member := uuid.New()

	reader := &stubCostRunReader{runs: []*model.AgentRun{
		run(now.Add(-10*time.Minute), "claude_code", "anthropic", member, 0.42),
	}}

	p := NewCostSummaryProjector(reader)
	res, err := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "", 5),
	)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	// Choice: end_in_future is clamped to now, not rejected.
	if res.Status != StatusOK {
		t.Fatalf("want ok (clamped), got %s diag=%s", res.Status, res.Diagnostics)
	}
	blocks := decodeBlocks(t, res.Projection)
	var totalLine string
	for _, b := range blocks {
		text := blockText(b)
		if strings.HasPrefix(text, "Total:") {
			totalLine = text
		}
	}
	if !strings.Contains(totalLine, "$0.4200") {
		t.Fatalf("expected clamped-window run to be counted, got %q", totalLine)
	}
}

func TestCostSummaryProjector_InvalidRange(t *testing.T) {
	now := time.Now().UTC()
	// end <= start
	start := now.Add(-1 * time.Hour)
	end := now.Add(-2 * time.Hour)

	reader := &stubCostRunReader{}
	p := NewCostSummaryProjector(reader)
	res, err := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "", 5),
	)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if res.Status != StatusDegraded {
		t.Fatalf("want degraded, got %s", res.Status)
	}
	if res.Diagnostics == "" {
		t.Fatalf("degraded projection must set Diagnostics")
	}
}

func TestCostSummaryProjector_InvalidRangeStartInFuture(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(1 * time.Hour)
	end := now.Add(2 * time.Hour)

	reader := &stubCostRunReader{}
	p := NewCostSummaryProjector(reader)
	res, _ := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "", 5),
	)
	if res.Status != StatusDegraded {
		t.Fatalf("want degraded when range_start is in the future, got %s", res.Status)
	}
}

func TestCostSummaryProjector_SubscribeWithoutMember(t *testing.T) {
	now := time.Now().UTC()
	ref := mustTargetRef(t, now.Add(-1*time.Hour), now, "", "", "")
	p := NewCostSummaryProjector(&stubCostRunReader{})

	topics := p.Subscribe(ref)
	if len(topics) != 2 {
		t.Fatalf("want 2 topics, got %d", len(topics))
	}
	wantEvents := map[string]bool{"agent.cost_update": false, "team.cost_update": false}
	for _, tp := range topics {
		if _, ok := wantEvents[tp.Event]; !ok {
			t.Fatalf("unexpected event %q", tp.Event)
		}
		wantEvents[tp.Event] = true
		if got, ok := tp.Scope["project_id"]; !ok || got != ScopeAssetProject {
			t.Fatalf("scope project_id: want sentinel %q, got %q (present=%v)", ScopeAssetProject, got, ok)
		}
		if _, hasMember := tp.Scope["member_id"]; hasMember {
			t.Fatalf("scope must not contain member_id when target_ref omits it")
		}
	}
	for name, seen := range wantEvents {
		if !seen {
			t.Fatalf("missing topic for event %q", name)
		}
	}
}

func TestCostSummaryProjector_SubscribeWithMember(t *testing.T) {
	now := time.Now().UTC()
	member := uuid.New()
	ref := mustTargetRef(t, now.Add(-1*time.Hour), now, "", "", member.String())
	p := NewCostSummaryProjector(&stubCostRunReader{})

	topics := p.Subscribe(ref)
	if len(topics) != 2 {
		t.Fatalf("want 2 topics, got %d", len(topics))
	}
	for _, tp := range topics {
		if got := tp.Scope["project_id"]; got != ScopeAssetProject {
			t.Fatalf("scope project_id: want sentinel, got %q", got)
		}
		if got := tp.Scope["member_id"]; got != member.String() {
			t.Fatalf("scope member_id: want %q, got %q", member.String(), got)
		}
	}
}

// Sanity check: the projector reports its kind + role correctly (used by
// the registry + coordinator wiring once routes.go is updated).
func TestCostSummaryProjector_KindAndRole(t *testing.T) {
	p := NewCostSummaryProjector(&stubCostRunReader{})
	if p.Kind() != KindCostSummary {
		t.Fatalf("kind=%s want %s", p.Kind(), KindCostSummary)
	}
	if p.RequiredRole() != RoleEditor {
		t.Fatalf("required role=%s want %s", p.RequiredRole(), RoleEditor)
	}
}

// Guard: the reader error path degrades the projection without leaking.
func TestCostSummaryProjector_ReaderError(t *testing.T) {
	now := time.Now().UTC()
	start := now.Add(-1 * time.Hour)
	end := now.Add(-1 * time.Minute)

	reader := &stubCostRunReader{err: fmt.Errorf("db down")}
	p := NewCostSummaryProjector(reader)
	res, _ := p.Project(
		context.Background(),
		mustEditor(),
		uuid.New(),
		mustTargetRef(t, start, end, "", "", ""),
		mustViewOpts(t, "", 5),
	)
	if res.Status != StatusDegraded {
		t.Fatalf("want degraded on reader error, got %s", res.Status)
	}
	if res.Diagnostics == "" {
		t.Fatalf("degraded must set Diagnostics")
	}
}
