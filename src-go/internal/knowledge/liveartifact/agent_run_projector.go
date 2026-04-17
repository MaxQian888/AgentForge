package liveartifact

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// agentRunReader is the minimal dependency the agent-run projector needs.
// The concrete implementation is *repository.AgentRunRepository; we use
// an interface to keep the projector package-isolated and testable.
type agentRunReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error)
}

// AgentRunProjector renders an agent_run as a BlockNote JSON fragment.
//
// Project-ownership scoping note: this projector does not cross-check
// that the run's task belongs to the supplied projectID. The
// projection handler validates the hosting asset belongs to projectID
// before dispatching here, which is sufficient for the initial cut.
// A follow-up should plumb task-project lookups for defence-in-depth.
type AgentRunProjector struct {
	runs agentRunReader
}

// NewAgentRunProjector constructs the projector with its dependency.
func NewAgentRunProjector(runs agentRunReader) *AgentRunProjector {
	return &AgentRunProjector{runs: runs}
}

// Kind reports the discriminator this projector handles.
func (p *AgentRunProjector) Kind() LiveArtifactKind { return KindAgentRun }

// RequiredRole reports the minimum role tier for a successful projection.
func (p *AgentRunProjector) RequiredRole() Role { return RoleViewer }

// agentRunTargetRef is the target_ref shape this projector accepts.
type agentRunTargetRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// agentRunViewOpts is the view_opts shape this projector accepts.
type agentRunViewOpts struct {
	ShowLogLines *int  `json:"show_log_lines,omitempty"`
	ShowSteps    *bool `json:"show_steps,omitempty"`
}

// defaults for view opts.
const (
	defaultShowLogLines = 10
	defaultShowSteps    = true
)

// allowed values for show_log_lines per spec.
var allowedLogLines = map[int]struct{}{10: {}, 25: {}, 50: {}}

// Project runs the projection. See LiveArtifactProjector for contract.
func (p *AgentRunProjector) Project(
	ctx context.Context,
	principal model.PrincipalContext,
	_ uuid.UUID,
	targetRef json.RawMessage,
	viewOpts json.RawMessage,
) (ProjectionResult, error) {
	now := time.Now().UTC()

	if !PrincipalHasRole(principal, p.RequiredRole()) {
		return ProjectionResult{Status: StatusForbidden, ProjectedAt: now}, nil
	}

	ref, err := parseAgentRunTargetRef(targetRef)
	if err != nil {
		return ProjectionResult{
			Status:      StatusNotFound,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	showLogLines, showSteps := parseAgentRunViewOpts(viewOpts)

	run, err := p.runs.GetByID(ctx, ref)
	// Pragmatic heuristic: nil run -> not found; any other error ->
	// degraded. The repository wraps its not-found sentinel, but we
	// intentionally do not import internal/repository here.
	if run == nil {
		if err != nil {
			// Treat repo errors on a nil run as not_found for the
			// canonical missing-entity shape.
			return ProjectionResult{
				Status:      StatusNotFound,
				ProjectedAt: now,
				Diagnostics: err.Error(),
			}, nil
		}
		return ProjectionResult{Status: StatusNotFound, ProjectedAt: now}, nil
	}
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	fragment, err := renderAgentRunBlocks(run, showLogLines, showSteps)
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	ttl := 30 * time.Second
	return ProjectionResult{
		Status:      StatusOK,
		Projection:  fragment,
		ProjectedAt: now,
		TTLHint:     &ttl,
	}, nil
}

// Subscribe lists hub event topics that trigger a re-projection.
// Event names mirror the constants in internal/ws/events.go; we inline
// the strings here to avoid coupling the projector package to the hub.
func (p *AgentRunProjector) Subscribe(targetRef json.RawMessage) []EventTopic {
	empty := []EventTopic{}
	ref, err := parseAgentRunTargetRef(targetRef)
	if err != nil {
		return empty
	}
	scope := map[string]string{"agent_run_id": ref.String()}
	names := []string{
		"agent.started",
		"agent.completed",
		"agent.failed",
		"agent.output",
		"agent.progress",
		"agent.cost_update",
	}
	out := make([]EventTopic, 0, len(names))
	for _, name := range names {
		out = append(out, EventTopic{Event: name, Scope: copyScope(scope)})
	}
	return out
}

// --- helpers ---

func parseAgentRunTargetRef(raw json.RawMessage) (uuid.UUID, error) {
	if len(raw) == 0 {
		return uuid.Nil, fmt.Errorf("target_ref missing")
	}
	var ref agentRunTargetRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		return uuid.Nil, fmt.Errorf("target_ref invalid: %w", err)
	}
	if ref.Kind != "" && ref.Kind != string(KindAgentRun) {
		return uuid.Nil, fmt.Errorf("target_ref kind %q not agent_run", ref.Kind)
	}
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("target_ref id invalid: %w", err)
	}
	return id, nil
}

func parseAgentRunViewOpts(raw json.RawMessage) (showLogLines int, showSteps bool) {
	showLogLines = defaultShowLogLines
	showSteps = defaultShowSteps
	if len(raw) == 0 {
		return
	}
	var opts agentRunViewOpts
	if err := json.Unmarshal(raw, &opts); err != nil {
		return
	}
	if opts.ShowLogLines != nil {
		if _, ok := allowedLogLines[*opts.ShowLogLines]; ok {
			showLogLines = *opts.ShowLogLines
		} else if *opts.ShowLogLines == 0 {
			// Explicit zero disables the log block.
			showLogLines = 0
		} else {
			showLogLines = defaultShowLogLines
		}
	}
	if opts.ShowSteps != nil {
		showSteps = *opts.ShowSteps
	}
	return
}

func renderAgentRunBlocks(run *model.AgentRun, showLogLines int, showSteps bool) (json.RawMessage, error) {
	blocks := make([]map[string]any, 0, 7)

	blocks = append(blocks, headingBlock(3, fmt.Sprintf("Agent run %s", run.ID.String())))
	blocks = append(blocks, paragraphBlock(fmt.Sprintf(
		"Status: %s • Runtime: %s • Provider: %s • Model: %s",
		run.Status, run.Runtime, run.Provider, run.Model,
	)))
	blocks = append(blocks, paragraphBlock(fmt.Sprintf(
		"Started: %s • Duration: %s",
		run.StartedAt.UTC().Format(time.RFC3339),
		humanDuration(run.StartedAt, run.CompletedAt),
	)))
	blocks = append(blocks, paragraphBlock(fmt.Sprintf(
		"Cost: $%.4f • Input tokens: %d • Output tokens: %d • Turns: %d",
		run.CostUsd, run.InputTokens, run.OutputTokens, run.TurnCount,
	)))

	if showSteps {
		// Reserved for future step model; AgentRun does not carry
		// per-step data today.
		blocks = append(blocks, paragraphBlock(fmt.Sprintf("Steps: %d turns", run.TurnCount)))
	}
	if showLogLines > 0 {
		// TODO: wire real log fetching once the AgentRun log repository
		// is exposed. Placeholder keeps the block visibly incomplete.
		blocks = append(blocks, paragraphBlock("Logs: (live log feed not yet wired)"))
	}
	if run.ErrorMessage != "" {
		blocks = append(blocks, paragraphBlock("Error: "+run.ErrorMessage))
	}

	return json.Marshal(blocks)
}

func headingBlock(level int, text string) map[string]any {
	return map[string]any{
		"id":      uuid.NewString(),
		"type":    "heading",
		"props":   map[string]any{"level": level},
		"content": inlineText(text),
	}
}

func paragraphBlock(text string) map[string]any {
	return map[string]any{
		"id":      uuid.NewString(),
		"type":    "paragraph",
		"content": inlineText(text),
	}
}

func inlineText(text string) []map[string]any {
	return []map[string]any{
		{"type": "text", "text": text, "styles": map[string]any{}},
	}
}

func humanDuration(start time.Time, end *time.Time) string {
	if end == nil {
		return "—"
	}
	d := end.Sub(start)
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	if total < 60 {
		return fmt.Sprintf("%ds", total)
	}
	minutes := total / 60
	seconds := total % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}

func copyScope(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
