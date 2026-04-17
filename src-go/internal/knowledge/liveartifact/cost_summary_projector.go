package liveartifact

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// ScopeAssetProject is a sentinel the WS subscription router substitutes
// with the asset's actual project id at fan-out time. The projector
// declares "scope by the hosting asset's project" without knowing the
// project id at Subscribe time (Subscribe only sees the target_ref).
// See §9.4 of the agent-artifact-doc-sync change for the router
// substitution contract.
const ScopeAssetProject = "$asset_project"

// costRunReader is the minimal dependency the cost-summary projector
// needs. The concrete implementation is *repository.AgentRunRepository;
// we aggregate ListByProject results in memory rather than calling into
// CostQueryService, whose surface is richer than we need (it also does
// project-budget lookups the projector shouldn't care about).
type costRunReader interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentRun, error)
}

// CostSummaryProjector renders a windowed cost aggregate over AgentRun
// rows as a BlockNote JSON fragment. The projector optionally groups by
// runtime, provider, or member and reports a delta vs. the prior
// equal-length window.
type CostSummaryProjector struct {
	runs costRunReader
}

// NewCostSummaryProjector constructs the projector with its dependency.
func NewCostSummaryProjector(runs costRunReader) *CostSummaryProjector {
	return &CostSummaryProjector{runs: runs}
}

// Kind reports the discriminator this projector handles.
func (p *CostSummaryProjector) Kind() LiveArtifactKind { return KindCostSummary }

// RequiredRole reports the minimum role tier for a successful projection.
// Approximation of cost-read — revisit when a finer permission lands.
func (p *CostSummaryProjector) RequiredRole() Role { return RoleEditor }

// --- target_ref / view_opts shapes ---

type costSummaryTargetRef struct {
	Kind   string                 `json:"kind"`
	Filter costSummaryTargetFilter `json:"filter"`
}

type costSummaryTargetFilter struct {
	RangeStart string `json:"range_start"`
	RangeEnd   string `json:"range_end"`
	Runtime    string `json:"runtime,omitempty"`
	Provider   string `json:"provider,omitempty"`
	MemberID   string `json:"member_id,omitempty"`
}

type costSummaryViewOpts struct {
	GroupBy string `json:"group_by,omitempty"`
	TopN    int    `json:"top_n,omitempty"`
}

const (
	groupByRuntime  = "runtime"
	groupByProvider = "provider"
	groupByMember   = "member"

	defaultTopN = 5
	minTopN     = 1
	maxTopN     = 20
)

// parsedFilter holds the resolved window + filters used for aggregation.
type parsedFilter struct {
	rangeStart time.Time
	rangeEnd   time.Time
	runtime    string
	provider   string
	memberID   *uuid.UUID
}

// Project runs the projection. See LiveArtifactProjector for contract.
func (p *CostSummaryProjector) Project(
	ctx context.Context,
	principal model.PrincipalContext,
	projectID uuid.UUID,
	targetRef json.RawMessage,
	viewOpts json.RawMessage,
) (ProjectionResult, error) {
	now := time.Now().UTC()

	if !PrincipalHasRole(principal, p.RequiredRole()) {
		return ProjectionResult{Status: StatusForbidden, ProjectedAt: now}, nil
	}

	filter, diag := parseCostTargetRef(targetRef, now)
	if diag != "" {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: diag,
		}, nil
	}

	groupBy, topN := parseCostViewOpts(viewOpts)

	allRuns, err := p.runs.ListByProject(ctx, projectID)
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	currentRuns := filterRunsByWindow(allRuns, filter.rangeStart, filter.rangeEnd, filter)
	// Prior window of equal length ending at range_start.
	windowLen := filter.rangeEnd.Sub(filter.rangeStart)
	priorStart := filter.rangeStart.Add(-windowLen)
	priorRuns := filterRunsByWindow(allRuns, priorStart, filter.rangeStart, filter)

	current := aggregateRuns(currentRuns)
	prior := aggregateRuns(priorRuns)

	fragment, err := renderCostSummaryBlocks(filter, groupBy, topN, current, prior, currentRuns)
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	ttl := 60 * time.Second
	return ProjectionResult{
		Status:      StatusOK,
		Projection:  fragment,
		ProjectedAt: now,
		TTLHint:     &ttl,
	}, nil
}

// Subscribe lists hub event topics that trigger a re-projection.
//
// Scope note: the projector does not know the hosting asset's project id
// at Subscribe time (target_ref carries filters, not a project id). It
// emits topics with the ScopeAssetProject sentinel; the WS subscription
// router substitutes the asset's actual project id before fan-out
// matching. See §9.4 of the agent-artifact-doc-sync change.
func (p *CostSummaryProjector) Subscribe(targetRef json.RawMessage) []EventTopic {
	// Narrow by member when the target_ref filters by member.
	var memberID string
	if len(targetRef) > 0 {
		var ref costSummaryTargetRef
		if err := json.Unmarshal(targetRef, &ref); err == nil {
			memberID = ref.Filter.MemberID
		}
	}
	build := func() map[string]string {
		scope := map[string]string{"project_id": ScopeAssetProject}
		if memberID != "" {
			scope["member_id"] = memberID
		}
		return scope
	}
	return []EventTopic{
		{Event: "agent.cost_update", Scope: build()},
		{Event: "team.cost_update", Scope: build()},
	}
}

// --- parsing helpers ---

func parseCostTargetRef(raw json.RawMessage, now time.Time) (parsedFilter, string) {
	if len(raw) == 0 {
		return parsedFilter{}, "target_ref missing"
	}
	var ref costSummaryTargetRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		return parsedFilter{}, fmt.Sprintf("target_ref invalid: %s", err.Error())
	}
	if ref.Kind != "" && ref.Kind != string(KindCostSummary) {
		return parsedFilter{}, fmt.Sprintf("target_ref kind %q not cost_summary", ref.Kind)
	}

	start, err := time.Parse(time.RFC3339, ref.Filter.RangeStart)
	if err != nil {
		return parsedFilter{}, fmt.Sprintf("range_start invalid: %s", err.Error())
	}
	end, err := time.Parse(time.RFC3339, ref.Filter.RangeEnd)
	if err != nil {
		return parsedFilter{}, fmt.Sprintf("range_end invalid: %s", err.Error())
	}
	start = start.UTC()
	end = end.UTC()
	// Neither endpoint should be in the future. We clamp range_end to
	// now if it is after now (partial window); range_start in the future
	// is rejected outright because it implies an empty / invalid window.
	if start.After(now) {
		return parsedFilter{}, "range_start is in the future"
	}
	if end.After(now) {
		// Clamp to now: callers pass "end of day" ranges and expect the
		// projector to aggregate up to the present.
		end = now
	}
	if !start.Before(end) {
		return parsedFilter{}, "range_start must be strictly before range_end"
	}

	out := parsedFilter{
		rangeStart: start,
		rangeEnd:   end,
		runtime:    ref.Filter.Runtime,
		provider:   ref.Filter.Provider,
	}
	if ref.Filter.MemberID != "" {
		id, err := uuid.Parse(ref.Filter.MemberID)
		if err != nil {
			return parsedFilter{}, fmt.Sprintf("member_id invalid: %s", err.Error())
		}
		out.memberID = &id
	}
	return out, ""
}

func parseCostViewOpts(raw json.RawMessage) (groupBy string, topN int) {
	topN = defaultTopN
	if len(raw) == 0 {
		return
	}
	var opts costSummaryViewOpts
	if err := json.Unmarshal(raw, &opts); err != nil {
		return
	}
	switch opts.GroupBy {
	case groupByRuntime, groupByProvider, groupByMember:
		groupBy = opts.GroupBy
	default:
		groupBy = ""
	}
	if opts.TopN != 0 {
		if opts.TopN < minTopN {
			topN = minTopN
		} else if opts.TopN > maxTopN {
			topN = maxTopN
		} else {
			topN = opts.TopN
		}
	}
	return
}

// --- aggregation ---

type costAggregate struct {
	totalCost    float64
	inputTokens  int64
	outputTokens int64
	runCount     int
}

func aggregateRuns(runs []*model.AgentRun) costAggregate {
	var agg costAggregate
	for _, r := range runs {
		if r == nil {
			continue
		}
		agg.totalCost += r.CostUsd
		agg.inputTokens += r.InputTokens
		agg.outputTokens += r.OutputTokens
		agg.runCount++
	}
	return agg
}

// filterRunsByWindow keeps runs whose StartedAt is in [start, end) and
// that also match the runtime / provider / member filters.
func filterRunsByWindow(runs []*model.AgentRun, start, end time.Time, f parsedFilter) []*model.AgentRun {
	out := make([]*model.AgentRun, 0, len(runs))
	for _, r := range runs {
		if r == nil {
			continue
		}
		started := r.StartedAt.UTC()
		if started.Before(start) || !started.Before(end) {
			continue
		}
		if f.runtime != "" && r.Runtime != f.runtime {
			continue
		}
		if f.provider != "" && r.Provider != f.provider {
			continue
		}
		if f.memberID != nil && r.MemberID != *f.memberID {
			continue
		}
		out = append(out, r)
	}
	return out
}

type groupRow struct {
	key  string
	cost float64
}

// groupRuns buckets runs by the chosen dimension, sorted by cost desc.
func groupRuns(runs []*model.AgentRun, groupBy string) []groupRow {
	if groupBy == "" {
		return nil
	}
	buckets := make(map[string]float64)
	for _, r := range runs {
		if r == nil {
			continue
		}
		var key string
		switch groupBy {
		case groupByRuntime:
			key = r.Runtime
		case groupByProvider:
			key = r.Provider
		case groupByMember:
			key = r.MemberID.String()
		}
		if key == "" {
			key = "(unknown)"
		}
		buckets[key] += r.CostUsd
	}
	out := make([]groupRow, 0, len(buckets))
	for k, v := range buckets {
		out = append(out, groupRow{key: k, cost: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].cost != out[j].cost {
			return out[i].cost > out[j].cost
		}
		return out[i].key < out[j].key
	})
	return out
}

// --- rendering ---

func renderCostSummaryBlocks(
	f parsedFilter,
	groupBy string,
	topN int,
	current costAggregate,
	prior costAggregate,
	currentRuns []*model.AgentRun,
) (json.RawMessage, error) {
	blocks := make([]map[string]any, 0, 4)

	heading := fmt.Sprintf(
		"Cost summary · %s → %s",
		f.rangeStart.Format("2006-01-02"),
		f.rangeEnd.Format("2006-01-02"),
	)
	blocks = append(blocks, headingBlock(3, heading))

	if current.runCount == 0 {
		// Zero-spend window: skip delta and table; show a single
		// paragraph. If prior is also zero, the delta line would be
		// meaningless; if prior has spend, the header already tells the
		// reader what the window is so skipping delta is fine.
		blocks = append(blocks, paragraphBlock("No cost recorded in this window."))
		return json.Marshal(blocks)
	}

	deltaCost := current.totalCost - prior.totalCost
	deltaPct := formatDeltaPct(current.totalCost, prior.totalCost)
	blocks = append(blocks, paragraphBlock(fmt.Sprintf(
		"Total: $%.4f · %d runs · Δ vs prior window: $%+.4f (%s)",
		current.totalCost, current.runCount, deltaCost, deltaPct,
	)))

	if groupBy != "" {
		groups := groupRuns(currentRuns, groupBy)
		if len(groups) > 0 {
			shown := groups
			if len(groups) > topN {
				shown = groups[:topN]
			}
			// TODO: align with BlockNote's `table` block schema once the
			// frontend (§10.3) lands. For now we emit one paragraph per
			// row so the content is always legible even if the table
			// shape drifts.
			for _, row := range shown {
				blocks = append(blocks, paragraphBlock(fmt.Sprintf(
					"%s=%s: $%.4f", groupBy, row.key, row.cost,
				)))
			}
			if len(groups) > len(shown) {
				blocks = append(blocks, paragraphBlock(fmt.Sprintf(
					"%d more groups omitted.", len(groups)-len(shown),
				)))
			}
		}
	}

	return json.Marshal(blocks)
}

// formatDeltaPct renders the delta % between current and prior totals.
// When prior is zero and current is non-zero we emit "n/a" — unambiguous
// and avoids an infinity symbol the frontend might render poorly.
func formatDeltaPct(current, prior float64) string {
	if prior == 0 {
		if current == 0 {
			return "+0.0%"
		}
		return "n/a"
	}
	pct := (current - prior) / prior * 100.0
	return fmt.Sprintf("%+.1f%%", pct)
}
