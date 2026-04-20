package nodetypes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// MaxSubWorkflowDepth caps the ancestor chain the recursion guard will walk
// before rejecting the invocation as too deep. A value of 8 mirrors the design
// decision: bounded cycle detection in O(depth) with a small constant, matching
// human comprehensibility of nested call stacks.
const MaxSubWorkflowDepth = 8

// SubWorkflowInvocationError captures a structured rejection so the applier
// can distinguish "cycle" / "unknown target" / "cross-project" from opaque
// engine failures. Consumed by tests and surfaced in node error messages.
type SubWorkflowInvocationError struct {
	Reason  string // machine-readable code: "cycle", "cross_project", "unknown_target", "unresolved_mapping", "depth_exceeded"
	Message string
}

func (e *SubWorkflowInvocationError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("sub_workflow: %s: %s", e.Reason, e.Message)
	}
	return "sub_workflow: " + e.Reason
}

// Structured-reason constants used by SubWorkflowInvocationError.Reason.
const (
	SubWorkflowRejectCycle           = "cycle"
	SubWorkflowRejectDepthExceeded   = "depth_exceeded"
	SubWorkflowRejectCrossProject    = "cross_project"
	SubWorkflowRejectUnknownTarget   = "unknown_target"
	SubWorkflowRejectUnresolvedMap   = "unresolved_mapping"
	SubWorkflowRejectTrivialSelfLoop = "trivial_self_loop"
)

// SubWorkflowInvocation is the context passed to a SubWorkflowEngine when the
// applier dispatches a child run. The engine uses it to stamp provenance and
// attribution onto the child run so the invocation tree can be walked later.
type SubWorkflowInvocation struct {
	ParentExecutionID uuid.UUID
	ParentNodeID      string
	ProjectID         uuid.UUID
	ActingEmployeeID  *uuid.UUID
}

// SubWorkflowEngine is the narrow contract the applier uses to start a child
// run across engines. Each engine (DAG, plugin) registers a single
// implementation; the applier looks it up by kind. Deliberately decoupled
// from the trigger package's TargetEngine contract so nodetypes stays free of
// the service/trigger import cycle — the wiring layer (routes.go) is
// responsible for providing an adapter that ultimately calls the same
// service-layer start seam.
type SubWorkflowEngine interface {
	Kind() SubWorkflowTargetKind
	// Validate rejects unknown targets and cross-project references. The
	// applier calls this before invoking Start so malformed or forbidden
	// invocations never reach the engine's dispatch code. Implementations
	// MUST return a *SubWorkflowInvocationError whose Reason is one of
	// SubWorkflowRejectUnknownTarget or SubWorkflowRejectCrossProject when
	// the corresponding condition applies.
	Validate(ctx context.Context, target string, inv SubWorkflowInvocation) error
	// Start launches a child run of the named target with the rendered seed.
	// Returns the child run id so the applier can insert the parent-link row.
	Start(ctx context.Context, target string, seed map[string]any, inv SubWorkflowInvocation) (uuid.UUID, error)
}

// SubWorkflowEngineRegistry is a small kind→engine map. Separate struct for
// testability (callers can hand the applier a fake registry without the real
// service wiring).
type SubWorkflowEngineRegistry struct {
	engines map[SubWorkflowTargetKind]SubWorkflowEngine
}

// NewSubWorkflowEngineRegistry returns an empty registry. Use Register to add
// engines. Nil engines are skipped silently so partial wiring does not fail
// construction.
func NewSubWorkflowEngineRegistry(engines ...SubWorkflowEngine) *SubWorkflowEngineRegistry {
	r := &SubWorkflowEngineRegistry{engines: make(map[SubWorkflowTargetKind]SubWorkflowEngine)}
	for _, e := range engines {
		r.Register(e)
	}
	return r
}

// Register adds or replaces an engine keyed by its Kind().
func (r *SubWorkflowEngineRegistry) Register(e SubWorkflowEngine) {
	if r == nil || e == nil {
		return
	}
	if r.engines == nil {
		r.engines = make(map[SubWorkflowTargetKind]SubWorkflowEngine)
	}
	r.engines[e.Kind()] = e
}

// Get returns the engine for a given kind, or (nil, false) if unregistered.
func (r *SubWorkflowEngineRegistry) Get(kind SubWorkflowTargetKind) (SubWorkflowEngine, bool) {
	if r == nil {
		return nil, false
	}
	e, ok := r.engines[kind]
	return e, ok
}

// SubWorkflowLinkRepo is the applier's view of the parent-link persistence.
// Matches the superset of repository.WorkflowRunParentLinkRepository used by
// the applier and the recursion guard.
type SubWorkflowLinkRepo interface {
	Create(ctx context.Context, link *SubWorkflowLinkRecord) error
	GetByParent(ctx context.Context, parentExecutionID uuid.UUID, parentNodeID string) (*SubWorkflowLinkRecord, error)
	GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*SubWorkflowLinkRecord, error)
	ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*SubWorkflowLinkRecord, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// SubWorkflowLinkRecord is the applier's transport shape for parent-link rows.
// Identical to model.WorkflowRunParentLink but defined here so the nodetypes
// package doesn't pull model's full surface; the wiring layer adapts between
// the two.
type SubWorkflowLinkRecord struct {
	ID                uuid.UUID
	ParentExecutionID uuid.UUID
	// ParentKind identifies which engine owns the parent run side of the
	// linkage. Empty string maps to "dag_execution" in the persistence layer
	// for back-compat with DAG-only callers.
	ParentKind        string
	ParentNodeID      string
	ChildEngineKind   string
	ChildRunID        uuid.UUID
	Status            string
}

// WorkflowExecutionLookup resolves a parent execution id to the workflow id
// that produced it. Used by the recursion guard to detect whether any
// ancestor's workflow id equals the proposed child target.
type WorkflowExecutionLookup interface {
	GetExecutionWorkflowID(ctx context.Context, executionID uuid.UUID) (uuid.UUID, error)
}

// RecursionGuard walks the parent-link chain upward from the proposed parent
// execution and rejects any invocation that would form a cycle on the target
// DAG workflow id, or exceed the maximum depth. Only runs for DAG targets;
// plugin targets short-circuit to "no cycle" because plugin runs cannot host
// sub_workflow nodes.
type RecursionGuard struct {
	Links    SubWorkflowLinkRepo
	ExecLook WorkflowExecutionLookup
	MaxDepth int
}

// NewRecursionGuard returns a guard wired to the repository/exec-lookup pair.
// If maxDepth is zero, falls back to MaxSubWorkflowDepth.
func NewRecursionGuard(links SubWorkflowLinkRepo, lookup WorkflowExecutionLookup, maxDepth int) *RecursionGuard {
	if maxDepth <= 0 {
		maxDepth = MaxSubWorkflowDepth
	}
	return &RecursionGuard{Links: links, ExecLook: lookup, MaxDepth: maxDepth}
}

// Check walks up to MaxDepth ancestors starting at parentExecutionID. For each
// ancestor, it resolves the execution's workflow id and compares against
// targetWorkflowID. Returns a structured error when a cycle is found, when the
// depth limit is exceeded, or when ExecLook returns an error. Returns nil on
// success (no cycle, within depth).
//
// Depth semantics: depth 1 = direct self-recursion (parent's workflow_id ==
// target); depth N = Nth-ancestor collision. If the chain stops (no parent
// link row for an ancestor's exec_id) before MaxDepth is reached, the check
// succeeds — the chain was shorter than the limit.
//
// Cross-engine walks: on encountering a link whose parent is a plugin run
// (parent_kind='plugin_run'), the guard hops to the plugin run side and
// continues walking if a DAG ancestor invoked that plugin run. Comparisons
// are scoped to `(engine_kind, workflow_id)`: a plugin id does not collide
// with a DAG workflow of the same UUID text (bridge-legacy-to-dag-invocation).
func (g *RecursionGuard) Check(ctx context.Context, parentExecutionID uuid.UUID, targetWorkflowID string) error {
	return g.CheckFromEngine(ctx, string(SubWorkflowTargetDAG), parentExecutionID, targetWorkflowID)
}

// CheckFromEngine walks the ancestor chain starting at the given engine-kind
// and run id. For DAG starts, the walk compares each DAG ancestor's workflow
// id against targetWorkflowID. For plugin-run starts, the walk looks for a
// DAG ancestor that invoked the plugin run (via a link row where
// child_engine_kind='plugin' and child_run_id=startRunID) and continues from
// there. See Check for depth semantics.
func (g *RecursionGuard) CheckFromEngine(ctx context.Context, startKind string, startRunID uuid.UUID, targetWorkflowID string) error {
	if g == nil || g.Links == nil {
		return nil
	}
	parsedTarget, err := uuid.Parse(targetWorkflowID)
	if err != nil {
		// Not a UUID → can't be a DAG workflow id. Pass through; non-DAG
		// targets (like plugin ids) are checked elsewhere.
		return nil
	}

	currentKind := startKind
	currentID := startRunID
	for depth := 1; depth <= g.MaxDepth; depth++ {
		if currentKind == string(SubWorkflowTargetDAG) {
			if g.ExecLook == nil {
				return nil
			}
			wfID, err := g.ExecLook.GetExecutionWorkflowID(ctx, currentID)
			if err != nil {
				return fmt.Errorf("recursion guard: resolve exec %s workflow id: %w", currentID, err)
			}
			if wfID == parsedTarget {
				return &SubWorkflowInvocationError{
					Reason:  SubWorkflowRejectCycle,
					Message: fmt.Sprintf("target workflow %s forms a cycle at depth %d", targetWorkflowID, depth),
				}
			}
			// Hop one level up: find the parent link whose child side is this
			// DAG execution. The parent may be another DAG execution or a
			// plugin run (parent_kind discriminator).
			link, err := g.Links.GetByChild(ctx, string(SubWorkflowTargetDAG), currentID)
			if err != nil || link == nil {
				return nil // end of chain
			}
			parentKind := link.ParentKind
			if parentKind == "" {
				parentKind = "dag_execution"
			}
			switch parentKind {
			case "plugin_run":
				currentKind = string(SubWorkflowTargetPlugin)
				currentID = link.ParentExecutionID
			default:
				currentKind = string(SubWorkflowTargetDAG)
				currentID = link.ParentExecutionID
			}
			continue
		}

		// Plugin ancestor: walk up to the DAG run (if any) that invoked it via
		// a DAG sub_workflow → plugin link. No DAG-workflow-id comparison is
		// possible at this hop (plugin ids are not DAG UUIDs).
		link, err := g.Links.GetByChild(ctx, string(SubWorkflowTargetPlugin), currentID)
		if err != nil || link == nil {
			return nil
		}
		parentKind := link.ParentKind
		if parentKind == "" {
			parentKind = "dag_execution"
		}
		switch parentKind {
		case "plugin_run":
			currentKind = string(SubWorkflowTargetPlugin)
			currentID = link.ParentExecutionID
		default:
			currentKind = string(SubWorkflowTargetDAG)
			currentID = link.ParentExecutionID
		}
	}

	return &SubWorkflowInvocationError{
		Reason:  SubWorkflowRejectDepthExceeded,
		Message: fmt.Sprintf("sub-workflow depth exceeds limit %d", g.MaxDepth),
	}
}

// renderSubWorkflowMapping resolves template references in an InputMapping
// JSON object against parent execution context. Supported path prefixes:
//   - $parent.dataStore.<path> → parent.DataStore[...]
//   - $parent.context.<path>   → parent.Context[...]
//   - $event.<path>            → parent.DataStore["$event"][...]  (symmetry with triggers)
//
// Unresolvable paths produce a structured SubWorkflowInvocationError with
// reason "unresolved_mapping" so the caller can classify it cleanly.
func renderSubWorkflowMapping(mapping json.RawMessage, dataStore, context map[string]any) (map[string]any, error) {
	if len(mapping) == 0 {
		return map[string]any{}, nil
	}
	var typed map[string]any
	if err := json.Unmarshal(mapping, &typed); err != nil {
		return nil, &SubWorkflowInvocationError{
			Reason:  SubWorkflowRejectUnresolvedMap,
			Message: "inputMapping is not a JSON object: " + err.Error(),
		}
	}
	out := make(map[string]any, len(typed))
	for k, v := range typed {
		s, ok := v.(string)
		if !ok {
			out[k] = v
			continue
		}
		resolved, err := resolveSubWorkflowTemplate(s, dataStore, context)
		if err != nil {
			return nil, err
		}
		out[k] = resolved
	}
	return out, nil
}

// resolveSubWorkflowTemplate handles whole-template references that must keep
// the native resolved type. Embedded templates within a larger string fall
// back to string substitution; unresolvable references become the empty
// string (matching the trigger router's behavior).
func resolveSubWorkflowTemplate(s string, dataStore, context map[string]any) (any, error) {
	trimmed := trimBraces(s)
	if trimmed == "" {
		return s, nil
	}
	if segments, ok := splitParentPath(trimmed); ok {
		root := resolveParentRoot(segments[0], dataStore, context)
		if root == nil {
			return nil, &SubWorkflowInvocationError{
				Reason:  SubWorkflowRejectUnresolvedMap,
				Message: fmt.Sprintf("unresolved path %q", s),
			}
		}
		var cur any = root
		for _, seg := range segments[1:] {
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, &SubWorkflowInvocationError{
					Reason:  SubWorkflowRejectUnresolvedMap,
					Message: fmt.Sprintf("path %q traverses non-object at %q", s, seg),
				}
			}
			nxt, present := m[seg]
			if !present || nxt == nil {
				return nil, &SubWorkflowInvocationError{
					Reason:  SubWorkflowRejectUnresolvedMap,
					Message: fmt.Sprintf("path %q unresolved at %q", s, seg),
				}
			}
			cur = nxt
		}
		return cur, nil
	}
	// Not a whole-template reference (or malformed) — return as-is.
	return s, nil
}

// trimBraces returns the inner path text if s is exactly `{{...}}`; otherwise
// returns "" to signal "not a whole-template reference".
func trimBraces(s string) string {
	if len(s) < 5 {
		return ""
	}
	if s[0] != '{' || s[1] != '{' || s[len(s)-2] != '}' || s[len(s)-1] != '}' {
		return ""
	}
	inner := s[2 : len(s)-2]
	// Trim whitespace within the braces.
	for len(inner) > 0 && (inner[0] == ' ' || inner[0] == '\t') {
		inner = inner[1:]
	}
	for len(inner) > 0 && (inner[len(inner)-1] == ' ' || inner[len(inner)-1] == '\t') {
		inner = inner[:len(inner)-1]
	}
	return inner
}

// splitParentPath accepts inner text like "$parent.dataStore.foo" and returns
// its split path components if the prefix matches one of the supported roots.
// Returns (nil, false) for anything else.
func splitParentPath(inner string) ([]string, bool) {
	// Supported roots.
	prefixes := []string{"$parent.dataStore.", "$parent.context.", "$event."}
	for _, p := range prefixes {
		if len(inner) > len(p) && inner[:len(p)] == p {
			segs := []string{p[:len(p)-1]}
			rest := inner[len(p):]
			for len(rest) > 0 {
				i := indexOf(rest, '.')
				if i < 0 {
					segs = append(segs, rest)
					break
				}
				segs = append(segs, rest[:i])
				rest = rest[i+1:]
			}
			return segs, true
		}
	}
	return nil, false
}

func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// resolveParentRoot maps the first path segment to its concrete map.
func resolveParentRoot(root string, dataStore, context map[string]any) map[string]any {
	switch root {
	case "$parent.dataStore":
		return dataStore
	case "$parent.context":
		return context
	case "$event":
		// Mirrors the trigger router's `$event` shorthand: DataStore["$event"]
		// carries the seed for trigger-started executions.
		if dataStore != nil {
			if m, ok := dataStore["$event"].(map[string]any); ok {
				return m
			}
		}
		return nil
	}
	return nil
}

// ErrSubWorkflowNoEngine is returned when the applier cannot find an engine
// for the requested target kind.
var ErrSubWorkflowNoEngine = errors.New("sub_workflow: no engine registered for target kind")
