// Package liveartifact defines the projector contract for live-artifact
// blocks embedded in wiki-page knowledge assets.
//
// Each supported kind (agent_run, cost_summary, review, task_group) is
// implemented as a separate LiveArtifactProjector and registered with the
// package-level registry. The projection endpoint and the WebSocket
// subscription router both look projectors up via Lookup.
package liveartifact

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// LiveArtifactKind discriminates a live-artifact block by the kind of
// entity it projects. The value is persisted inside BlockNote props as
// `live_kind` and MUST NOT be reused across unrelated entity shapes.
type LiveArtifactKind string

const (
	KindAgentRun     LiveArtifactKind = "agent_run"
	KindCostSummary  LiveArtifactKind = "cost_summary"
	KindReview       LiveArtifactKind = "review"
	KindTaskGroup    LiveArtifactKind = "task_group"
)

// ProjectionStatus expresses whether a projection succeeded and, if not,
// why. The frontend renders different block states per status.
type ProjectionStatus string

const (
	// StatusOK means the projection was computed and the Projection
	// field carries a BlockNote JSON fragment.
	StatusOK ProjectionStatus = "ok"
	// StatusNotFound means the target entity could not be resolved
	// (deleted, never existed, or a filter that yields no matches for a
	// singleton kind). The block renders a "no longer available" state.
	StatusNotFound ProjectionStatus = "not_found"
	// StatusForbidden means the principal lacks the per-entity read
	// permission required by this projector. The block renders a
	// "no access" state without leaking the target title or id.
	StatusForbidden ProjectionStatus = "forbidden"
	// StatusDegraded means the projector encountered a transient
	// failure (timeout, dependency error). The block renders the last
	// successful projection if one is still within TTL, otherwise a
	// "temporarily unavailable" state. Diagnostics MUST be set.
	StatusDegraded ProjectionStatus = "degraded"
)

// ProjectionResult is the outcome of a single projector invocation.
// The wire form is documented in the live-artifact-projection spec.
type ProjectionResult struct {
	Status      ProjectionStatus `json:"status"`
	Projection  json.RawMessage  `json:"projection,omitempty"`
	ProjectedAt time.Time        `json:"projected_at"`
	TTLHint     *time.Duration   `json:"ttl_hint_ms,omitempty"`
	Diagnostics string           `json:"diagnostics,omitempty"`
}

// EventTopic is a scope-filter on a WebSocket event name. The
// subscription router compares incoming hub events against the union of
// topics each open asset has registered; on any match the router emits a
// `knowledge.asset.live_artifacts_changed` payload that tells the client
// which block ids to re-project.
//
// The Event field matches the hub event name emitted by
// internal/ws/events.go (for example "agent.cost_update"). Scope is a
// logical-AND predicate: every key-value pair must match the event's
// payload for the topic to fire. Keys use the same snake_case shape the
// hub uses in its event payloads (for example "project_id",
// "agent_run_id", "review_id").
type EventTopic struct {
	Event string            `json:"event"`
	Scope map[string]string `json:"scope,omitempty"`
}

// Role is a logical read-permission tier a projector requires. Projectors
// map logical tiers to AgentForge's existing ProjectRole strings via
// PrincipalHasRole. We keep this decoupled so a future finer-grained
// permission system (e.g. a dedicated cost-read permission) can land
// without changing each projector.
type Role string

const (
	// RoleViewer is the default tier: any project member can read.
	RoleViewer Role = "viewer"
	// RoleEditor requires edit-or-higher; used for sensitive aggregates
	// like cost summaries until a finer cost-read permission exists.
	RoleEditor Role = "editor"
	// RoleAdmin requires admin-or-higher; reserved for future projectors
	// that expose privileged state.
	RoleAdmin Role = "admin"
)

// PrincipalHasRole reports whether the principal satisfies the required
// role tier against their project role.
func PrincipalHasRole(pc model.PrincipalContext, required Role) bool {
	switch required {
	case RoleViewer:
		return pc.CanRead()
	case RoleEditor:
		return pc.CanWrite()
	case RoleAdmin:
		return pc.CanAdmin()
	default:
		return false
	}
}

// LiveArtifactProjector turns a block reference into a rendered
// BlockNote JSON fragment.
//
// Implementations MUST:
//   - Return StatusForbidden (never a real error) when the principal
//     lacks read access. Leaking "entity exists" via a different error
//     shape is a vulnerability.
//   - Return StatusNotFound when the target reference cannot be
//     resolved. Orphan handling is on the client; projectors do not
//     delete blocks.
//   - Scope all queries to the supplied projectID. Cross-project
//     projections are explicitly out of scope.
type LiveArtifactProjector interface {
	// Kind returns the LiveArtifactKind this projector handles.
	Kind() LiveArtifactKind

	// RequiredRole returns the minimum role tier a principal must hold
	// for Project to return StatusOK. Even if the asset is readable,
	// projectors with higher required roles degrade to StatusForbidden
	// for less-privileged principals.
	RequiredRole() Role

	// Project runs the projection. targetRef and viewOpts are the raw
	// BlockNote `props.target_ref` and `props.view_opts` values from the
	// live block; each projector validates its own shape.
	Project(
		ctx context.Context,
		principal model.PrincipalContext,
		projectID uuid.UUID,
		targetRef json.RawMessage,
		viewOpts json.RawMessage,
	) (ProjectionResult, error)

	// Subscribe declares which hub event topics should trigger a
	// re-projection of blocks referencing targetRef. Projectors SHOULD
	// return narrowly-scoped topics (by target id/filter) to minimize
	// fan-out; broad wildcards hurt open-doc performance under load.
	Subscribe(targetRef json.RawMessage) []EventTopic
}
