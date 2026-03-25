## Context

AgentForge already has the first slice of plugin execution in place: Go can parse manifests, keep an in-memory registry, delegate tool plugins to the TS bridge, and run Go-hosted WASM integration plugins. However, the Go plugin server still falls short of the control-plane architecture described in `docs/PRD.md` and `docs/part/PLUGIN_SYSTEM_DESIGN.md`.

The current gaps are structural rather than cosmetic. Plugin registry state only lives in memory, so installed plugins disappear on restart and runtime ownership is not durably recorded. There is no persistent plugin-instance model even though the design requires runtime-state reconciliation. Plugin lifecycle events are not written to an audit log or routed through a dedicated event hub. The marketplace endpoint returns hardcoded role placeholders instead of data sourced from real manifests. Permission declarations are parsed from manifests but not enforced during activation or invocation. These omissions make the Go server an unreliable source of truth for the dashboard and for future integration, workflow, and review plugins.

## Goals / Non-Goals

**Goals:**
- Make the Go server the authoritative, persistent control plane for plugin registry records, runtime instances, and lifecycle state.
- Add durable plugin instance snapshots and event audit records so runtime reconciliation survives restarts and supports operator troubleshooting.
- Replace placeholder catalog responses with manifest-backed built-in and installable plugin feed data.
- Enforce permission-aware activation and invocation checks on the Go side before runtime execution proceeds.
- Preserve the existing host split: TS bridge owns tool plugins, Go owns Go-hosted executable plugins.

**Non-Goals:**
- Building a remote plugin marketplace or OCI registry.
- Implementing full workflow-plugin or review-plugin runtimes.
- Replacing the existing WASM runtime contract or reworking TS bridge MCP internals.
- Shipping a new dashboard UX beyond the API and data contract adjustments required by the control plane.

## Decisions

### 1. Persist plugin control-plane state in dedicated repositories with graceful fallback
Use Go repositories for plugin records, plugin instance snapshots, and plugin event audit entries, backed by PostgreSQL when available. In environments where the main database is intentionally unavailable, keep a documented degraded fallback so local development does not crash, but treat the persistent repository path as the authoritative production behavior.

Alternatives considered:
- Keep the in-memory registry and rebuild on startup. Rejected because it breaks installed-plugin continuity and makes runtime reconciliation impossible.
- Store registry state only on disk as ad hoc JSON files. Rejected because the product design already expects relational queries, filtering, and audit history.

### 2. Separate top-level plugin records from runtime instance snapshots
Keep `PluginRecord` as the operator-facing registry row and introduce a distinct runtime instance snapshot model for active/degraded/restarting ownership details. The registry stores the latest summarized state, while instance snapshots retain the current runtime artifact identity, host ownership, health timestamps, and restart counters used for reconciliation.

Alternatives considered:
- Expand the existing registry record until it doubles as instance state. Rejected because install metadata and runtime-instance state evolve at different rates and need different lifecycle semantics.
- Create one instance row per event. Rejected because reconciliation needs a mutable current snapshot plus an append-only event history.

### 3. Treat lifecycle events as first-class control-plane outputs
Every install, enable, disable, activate, runtime-sync, health, restart, invoke, and uninstall transition should emit a structured plugin event. The Go server should append these events to storage and fan out operator-relevant events through a small event service that can later map to the broader EventEmitter/WS Hub model described in the PRD.

Alternatives considered:
- Only update the registry row and skip event history. Rejected because it prevents auditability and makes plugin troubleshooting opaque.
- Emit WebSocket events without storing them. Rejected because restarts and offline periods would erase the only operational trail.

### 4. Build the catalog feed from real manifests instead of static DTOs
The marketplace/catalog endpoint should aggregate manifest-backed built-ins plus installable sources known to the Go server, and merge those entries with installed registry records. This keeps the plugin dashboard aligned with what the server can actually install and operate.

Alternatives considered:
- Leave `ListMarketplace` hardcoded until a remote marketplace exists. Rejected because it produces misleading UI data today.
- Remove the endpoint entirely. Rejected because built-in discovery and install flows still need a catalog surface.

### 5. Enforce permissions at the Go control-plane boundary
Manifest permissions and declared capabilities should be validated before activation or invocation reaches the runtime. The Go server should own policy checks for filesystem/network declarations and deny unsupported operations early, while the runtime remains responsible for host-specific execution details.

Alternatives considered:
- Defer permission checks entirely to runtime implementations. Rejected because policy becomes inconsistent across hosts and harder to reason about.
- Treat permission declarations as informational only. Rejected because the design documents explicitly position them as security controls.

## Risks / Trade-offs

- [Database-backed control plane increases migration scope] -> Mitigation: isolate plugin tables and repositories, keep clear fallback behavior when DB is absent, and add focused repository tests.
- [Event logging can create noisy records] -> Mitigation: define a constrained event taxonomy and store structured summaries rather than unbounded payload dumps.
- [Permission enforcement may reject manifests that previously worked] -> Mitigation: make validation errors explicit, document accepted policy shape, and preserve backward-compatible defaults where declarations are absent.
- [Catalog normalization can drift from future remote marketplace work] -> Mitigation: model the local catalog as a provider behind one service interface so remote sources can replace or extend it later.

## Migration Plan

1. Add plugin control-plane migrations and repositories for registry rows, instance snapshots, and event audit entries.
2. Introduce a plugin control-plane service layer that wraps install/enable/activate/health/restart/invoke/runtime-sync/uninstall flows and emits audit events.
3. Switch the existing plugin HTTP handlers to the new service contract while preserving current endpoint shapes where practical.
4. Backfill built-in plugin discovery into the persistent registry on demand and ensure existing built-in manifests still install cleanly.
5. Update dashboard-facing catalog responses and runtime-state tests, then verify restart persistence and degraded-state reconciliation.

## Open Questions

- Should project-scoped plugin instances be exposed in the first API revision, or should the initial control plane ship with a global/default scope while storing project linkage only when callers provide it?
- Should runtime event broadcasting piggyback on the existing `ws.Hub` first, or should this change introduce a narrow plugin-event broadcaster abstraction immediately?
