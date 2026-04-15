# Unified Event Bus Design (borrowing OpenAgents ONM)

Status: Draft · 2026-04-16 · Internal / pre-1.0 / experimental

## Goal

Replace AgentForge's four fragmented event carriers (`AgentEvent`, `PluginEventRecord`, `ws.Event`, `BridgeAgentEvent`) with a single event envelope and a pluggable `guard → transform → observe` pipeline, modelled on the OpenAgents Network Model (ONM). Introduce named channel subscriptions so the frontend no longer filters a firehose broadcast. This is the foundation for later event-middleware features (rate-limit, audit trail, agent-to-agent messaging) and federation.

## Context

### What openagents-org/openagents has that we do not

Verified against real sources on branch `develop`:

- **Single `Event` envelope** (`sdk/src/openagents/core/onm_events.py`): `id / type / source / target / payload / metadata / timestamp / network / visibility`. `target` is never null. Type names follow `{domain}.{entity}.{action}`; `network.*` is reserved.
- **Mod pipeline** (`sdk/src/openagents/core/onm_pipeline.py`): three ordered modes (`guard` 0, `transform` 1, `observe` 2), then by `priority`. Guard rejects, Transform rewrites, Observe has side-effects only. Implemented as subclasses `GuardMod` / `TransformMod` / `ObserveMod`.
- **URI addressing** (`sdk/src/openagents/core/onm_addressing.py`): `agent:name`, `channel/name`, `mod/name`, `human:name`, `core`. Cross-network `{network}::{entity}`. Implemented by an `Address` dataclass + `parse_address()` helper; there is no `Addressable` protocol.
- **Transport / Network separation**: `AgentNetwork` routes events regardless of whether the peer arrived over HTTP / WebSocket / Stdio / gRPC / MCP / A2A.
- **BaseAdapter** pattern (`sdk/src/openagents/adapters/base.py`) for bridging external CLI agents.
- Notably, `core/` is a redesign layer that coexists with the older `sdk/` implementation — openagents is itself migrating onto ONM.

### AgentForge's current state (verified)

- Four uncorrelated event structs live side-by-side:
  - `AgentEvent` (`src-go/internal/model/agent_event.go` L10-L19) — persisted rows
  - `PluginEventRecord` (`src-go/internal/model/plugin.go` L242-L249)
  - `Event` (`src-go/internal/ws/events.go` L114-L133) — WebSocket broadcast
  - `BridgeAgentEvent` (`src-go/internal/ws/events.go`) — TS Bridge ingress
- No shared base; each sink (repo, WS hub, bridge handler, IM forwarder) has its own envelope.
- **Event type strings already follow `{domain}.{entity}.{action}`** — 80+ constants in `src-go/internal/ws/events.go` (`task.created`, `workflow.execution.started`, `plugin.lifecycle`, …). This naming is preserved by this design.
- `Hub.BroadcastEvent` (`src-go/internal/ws/hub.go` L97-L116) filters only by `projectID`. The frontend `lib/ws-client.ts` already has a `subscribe(channel)` stub, but the server never consumes it. Frontend stores filter the firehose client-side.
- Go middleware is HTTP-only; review-trigger / audit / rate-limit logic is scattered across `service` and custom middleware (`src-go/internal/middleware/review_trigger.go`).
- TS Bridge has a unified `RuntimeAdapter` interface; the real hot-spot is `src-bridge/src/handlers/claude-runtime.ts` at 1052 LOC, which is **out of scope** for this spec.

### Why now

1. AgentForge is in internal testing; breaking changes are free. Full replacement is cheaper than dual-write maintenance.
2. The frontend-backend channel-subscription contract is already half-implemented and half-broken. Every new WS feature currently pays for that gap.
3. Upcoming work (agent-to-agent messaging, rate-limit, federation, plugin-owned mods) all require an event bus. Building it once now unblocks multiple future features.

## Non-goals

- IM Bridge integration (deferred; M4+ if evaluated).
- Marketplace microservice changes.
- WASM/MCP plugin runtime rework.
- Collapsing `claude-runtime.ts` or any other bridge handler refactor.
- Federation / cross-network routing. `network` field is reserved but unused in v1.
- DID or token-auth identity scheme. `source` authentication is delegated to the existing auth middleware.
- Backward-compatibility shims, feature flags for gradual rollout, or deprecation cycles. Project is pre-1.0 internal.

## Key decisions

1. **Single envelope, single bus.** `eventbus.Event` replaces the four legacy structs. `eventbus.Bus.Publish(Event)` replaces all direct calls to `hub.BroadcastEvent` and direct `*Repository.Create` from service code for event rows.
2. **Pipeline is the only runtime.** Persistence, WebSocket fan-out, and IM forward are themselves `ObserveMod`s registered on the bus. There are no privileged sinks.
3. **Full replacement over parallel systems.** Legacy event structs and `agent_events` table are deleted, not deprecated.
4. **Channel subscription is the default WebSocket model.** Public/system events still broadcast; everything else routes via `metadata.channels`.
5. **Type naming is preserved.** The 80+ existing event-type constants stay as-is. Only the envelope changes.

## Architecture

```
service.X ──┐
handler.Y ──┼──► Bus.Publish(Event) ──► Pipeline ──► (Mods)
bridge.Z ───┘

Pipeline order:
    Guard priority-asc     — may reject; may not mutate
        ↓
    Transform priority-asc — must return next Event; may not reject
        ↓
    Observe priority-asc   — side-effects only; no rejects, no mutations, errors swallowed
```

`Sink` is not a distinct concept; built-in sinks (`core.persist`, `core.ws-fanout`, …) are ordinary `ObserveMod`s. User-authored mods (rate-limit, audit) participate in the same pipeline.

## Event envelope

```go
// src-go/internal/eventbus/event.go
type Event struct {
    ID         string          `json:"id"`         // ULID, required
    Type       string          `json:"type"`       // {domain}.{entity}.{action}, required
    Source     string          `json:"source"`     // URI, required
    Target     string          `json:"target"`     // URI, required, never empty
    Payload    json.RawMessage `json:"payload"`
    Metadata   map[string]any  `json:"metadata"`
    Timestamp  int64           `json:"timestamp"`  // unix ms
    Visibility Visibility      `json:"visibility"` // defaults to "channel"
}

type Visibility string
const (
    VisibilityPublic  Visibility = "public"
    VisibilityChannel Visibility = "channel"
    VisibilityDirect  Visibility = "direct"
    VisibilityModOnly Visibility = "mod_only"
)
```

Reserved metadata keys (the bus exposes helper accessors `GetChannels(e)`, `SetChannels(e, []string)` etc. that assert the expected concrete type; direct map access is discouraged):

- `metadata.channels` — `[]string` of channel URIs this event fans out to; populated by `core.channel-router`. `core.validate` (guard) rejects events whose `channels` exists but is not a `[]string`.
- `metadata.span_id`, `metadata.trace_id` — tracing.
- `metadata.causation_id`, `metadata.correlation_id` — event chains.
- `metadata.user_id`, `metadata.project_id` — enriched from auth context.

## Addressing

```go
// src-go/internal/eventbus/address.go
type Address struct {
    Scheme string // agent | role | task | project | team | workflow | plugin | skill | channel | user | core
    Name   string // URI tail after the scheme; channel may contain one inner scheme
    Raw    string // original string
}

func ParseAddress(s string) (Address, error)
```

Canonical forms:

| URI                          | Meaning                                         |
| ---------------------------- | ----------------------------------------------- |
| `agent:run-abc123`           | A single `AgentRun`                             |
| `role:planner`               | A declared Role                                 |
| `task:7b2e9a`                | A Task                                          |
| `project:demo`               | A Project                                       |
| `team:alpha`                 | A Team                                          |
| `workflow:wf-42`             | A Workflow definition                           |
| `plugin:review-linter`       | A Plugin instance                               |
| `user:max`                   | A human                                         |
| `core`                       | The bus / system itself                         |
| `channel:project:demo`       | Project-scoped channel                          |
| `channel:task:7b2e9a`        | Task-scoped channel                             |
| `channel:agent:run-abc123`   | Run-scoped channel                              |

`network::` prefixes are reserved (not parsed in v1).

## Mod interface

```go
// src-go/internal/eventbus/mod.go
type Mode uint8
const (
    ModeGuard     Mode = 1
    ModeTransform Mode = 2
    ModeObserve   Mode = 3
)

type Mod interface {
    Name() string
    Intercepts() []string // glob patterns, e.g. "task.*", "workflow.execution.*", "*"
    Priority() int        // asc within mode; default 100
    Mode() Mode
}

type GuardMod interface {
    Mod
    Guard(ctx context.Context, e *Event, pc *PipelineCtx) error
}
type TransformMod interface {
    Mod
    Transform(ctx context.Context, e *Event, pc *PipelineCtx) (*Event, error)
}
type ObserveMod interface {
    Mod
    Observe(ctx context.Context, e *Event, pc *PipelineCtx) // errors are swallowed
}

type PipelineCtx struct {
    NetworkID string              // reserved; v1 = ""
    Emits     []Event             // new events scheduled for re-entry (max depth 3)
    SpanID    string
    Attrs     map[string]any
}
```

### Pipeline semantics

1. `Bus.Publish(e)` validates `id`, `type`, `source`, `target`, `timestamp`. On failure returns error without invoking any mod.
2. Group mods by mode. Sort each group by priority asc.
3. For each `GuardMod` matching `e.Type`: call `Guard`. Any non-nil error → abandon event, emit synthetic `event.error` (target = original `source`).
4. For each `TransformMod` matching `e.Type`: call `Transform`. Returned `*Event` becomes input for the next. Nil-with-error → same as guard rejection with a different `reason`.
5. `ObserveMod`s are run in parallel with `ctx` carrying a 5s deadline. Panics are recovered and logged. Mutual isolation: one slow observer must not block another. **Observer ordering is explicitly not guaranteed** — mods must not depend on another observer's side-effect having completed (e.g., an audit observer must not assume `core.persist` already wrote the row; if that dependency is needed, model it as a subsequent event via `PipelineCtx.Emits`).
6. If `PipelineCtx.Emits` is non-empty, recurse. `metadata.causation_depth` must not exceed 3; deeper chains error and drop the new event.

### Built-in mods (delivered with M1)

| Name                 | Mode      | Intercepts | Responsibility                                                                 |
| -------------------- | --------- | ---------- | ------------------------------------------------------------------------------ |
| `core.auth`          | guard     | `*`        | Verify `source` matches the authenticated principal of the publishing context  |
| `core.validate`      | guard     | `*`        | Enforce type regex, non-empty target, payload shape (nil vs raw allowed)       |
| `core.channel-router`| transform | `*`        | Compute `metadata.channels` from `target` and declared rules                   |
| `core.enrich`        | transform | `*`        | Attach `span_id`, `user_id`, `project_id` from context                         |
| `core.persist`       | observe   | `*`        | Write to the new `events` table                                                |
| `core.ws-fanout`     | observe   | `*`        | Push to WebSocket clients subscribed to any `metadata.channels` entry          |
| `core.metrics`       | observe   | `*`        | Prometheus counters + latency histograms                                       |

### Future mods (not in v1, listed for shape confirmation)

`core.rate-limit`, `core.audit-trail`, `core.im-forward` (replacing `im_event_routing`), `workflow.trigger-dispatcher` (moves workflow status→status triggers onto the bus), plugin-contributed mods via the plugin control plane.

## Channel subscription protocol

### Client → server

```json
{"op": "subscribe",   "channels": ["project:demo", "task:7b2e", "agent:run-abc"]}
{"op": "unsubscribe", "channels": ["task:7b2e"]}
```

### Server → client

```json
{"channel": "task:7b2e", "event": { /* full Event envelope */ }}
```

### Routing rules

- `Visibility=public` events ignore subscription set and broadcast to all connected clients of matching `project_id` (auth scope).
- `Visibility=channel`: delivered to a client iff the client has subscribed to any channel in `event.metadata.channels`.
- `Visibility=direct`: delivered iff `target` equals a client-bound identity (`agent:…` or `user:…`).
- `Visibility=mod_only`: never reaches WebSocket; observed by in-process mods only.

### Hub server-side

- `Client` struct tracks `map[string]bool` (subscription set).
- `core.ws-fanout` maintains an inverted index `channel → []*Client` updated on subscribe/unsubscribe.
- Unknown `op` or malformed message → one-shot error frame, connection remains open.

### Frontend

- `lib/ws-client.ts` gains real `subscribe(channels)` / `unsubscribe(channels)` implementations.
- A new `lib/stores/subscription-manager.ts` reference-counts desired channels across all stores and flushes batched subscribe/unsubscribe messages.
- Each store declares required channels at mount; unmount releases them. Old "filter the firehose" client code is removed.

## Phase plan

### M1 — Go EventBus core

**In scope (src-go):**

- New package `internal/eventbus` with `Event`, `Address`, `Bus`, `Pipeline`, `Mod`, `Mode`, 7 built-in mods.
- New `events` table; drop `agent_events`. Migration replaces the old table; historical rows are discarded.
- New `events_dead_letter` table used by `core.persist` when a persist attempt fails after retries (see Risk 2). Schema mirrors `events` plus `last_error`, `retry_count`, `first_seen_at`.
- All service/handler/bridge_handler call sites migrate from `hub.BroadcastEvent` or `*Repository.Create` (for event rows) to `bus.Publish`.
- Delete `AgentEvent`, `PluginEventRecord`, `ws.Event`, `BridgeAgentEvent`.
- `im_event_routing` keeps its current surface but its producer side goes through the bus; M1 wraps it as an `ObserveMod` named `im.forward-legacy` to minimize IM-side churn. (It is removed in M4 when proper `core.im-forward` lands.)
- Existing 80+ event-type constants remain and are reused.
- Frontend continues to receive the legacy-shaped payload because the Bus emits `metadata.channels = ["project:<id>"]` and `Visibility=public`; no subscription model yet.

**Verification:**

- `go test ./...` passes.
- Existing Go integration tests pass.
- `pnpm dev:backend:verify` green.
- Manual smoke: create task, start agent, observe WS frames.

### M2 — Frontend channel subscribe

**In scope:**

- `lib/ws-client.ts` subscribe/unsubscribe implementation.
- `lib/stores/subscription-manager.ts` new module.
- Every store under `lib/stores/` that previously filtered the firehose migrates to declared channels; list is enumerated during M2 execution planning.
- Server-side `core.ws-fanout` switches default from "broadcast to project" to "route by channels" once this milestone ships. Legacy broadcast path is removed.
- Payload shape migrated to new envelope.

**Verification:**

- `pnpm test` green.
- Every dashboard page smoke test passes.
- WebSocket frame rate drops on loaded projects (observable metric).

### M3 — Bridge integration

**In scope (src-bridge):**

- Bridge's outgoing envelope (the wire format for `/ws/bridge`) becomes the shared `Event`.
- `BridgeAgentEvent` removed client-side too; shared TS/Go schema lives in `src-bridge/src/schemas.ts` mirroring `eventbus.Event`.
- `/bridge/execute` internal event production converts to `Event`; bridge_handler delegates to `bus.Publish`.

**Verification:**

- Full runtime e2e: Claude / Codex / OpenCode runtimes each complete one agent run producing cost + review + agent event chain.

### Out of scope / deferred

- **M4+ (not in v1):** IM Bridge refactor, plugin-contributed mods, `core.rate-limit`, `core.audit-trail`, federation.

## Testing & observability

- **Unit:** every mod tested in isolation (guard rejection, transform mutation, observe side-effect). Pipeline tested for ordering, priority, error propagation, depth limit.
- **Integration:** minimal Bus instance with all 7 core mods registered; assert `task.created` flows guard → transform → persist + ws-fanout + metrics.
- **Chaos:** inject panics in each observer; assert other observers still fire and the bus does not deadlock.
- **Metrics:** per-mod `duration_seconds` histogram, `events_rejected_total{mod}`, `events_published_total{type}`.
- **Tracing:** each `Publish` opens a span; mods add attributes; observer execution is a child span.

## Migration checklist (for plan phase, informal)

- New migration `NNN_events_table.sql` creating `events` and `events_dead_letter`, dropping `agent_events`.
- Code-mod every `hub.BroadcastEvent(&ws.Event{...})` → `bus.Publish(Event{...})`.
- Code-mod every `*EventRepository.Create` call site.
- Remove `src-go/internal/ws/events.go` struct definitions (keep the string constants file, renamed).
- Update `docs/api/asyncapi.yaml` to describe the new channel protocol; the existing AsyncAPI spec (2026-04-08) becomes the migration target.

## Risks

1. **Broadcast regression during M1-M2 window.** M1 ships with legacy-shaped public events; if M2 lags, frontend sees large envelopes but no channel benefit. Mitigation: M2 must land within the same release train as M1.
2. **Panic in a core observer** could silently drop persistence. Mitigation: `core.persist` has its own dead-letter queue (separate table); alerting on non-empty queue.
3. **Test burden.** 30+ stores in M2 means broad regression surface. Mitigation: subscription-manager is the only new moving part; stores only change their `mount`/`unmount` calls.
4. **IM Bridge legacy wrap.** The `im.forward-legacy` observer reproduces the old delivery semantics exactly. Any behavior drift will only surface in M4 when it is replaced; we accept the risk and keep observer logic extremely thin.

## References

- OpenAgents source (branch `develop`): `sdk/src/openagents/core/onm_events.py`, `onm_pipeline.py`, `onm_addressing.py`, `onm_mods.py`, `adapters/base.py`.
- AgentForge source (branch `master`):
  - `src-go/internal/model/agent_event.go`
  - `src-go/internal/model/plugin.go`
  - `src-go/internal/ws/events.go`, `hub.go`
  - `src-go/internal/middleware/review_trigger.go`
  - `lib/ws-client.ts`
- Existing related spec: `docs/superpowers/specs/2026-04-08-asyncapi-event-streams-design.md` — describes the current event surface; this spec supersedes its modelling assumptions and provides the schema this AsyncAPI doc should mirror after M3.
- Project stage context: internal testing, pre-1.0, breaking changes permitted (per project memory 2026-04-16).
