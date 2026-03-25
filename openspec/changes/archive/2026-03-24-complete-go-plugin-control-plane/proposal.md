## Why

The current Go plugin server only provides a minimal runtime loop for registration, activation, health checks, restart, and WASM invocation. Relative to `docs/PRD.md` and `docs/part/PLUGIN_SYSTEM_DESIGN.md`, the control plane is still incomplete: plugin registry state is in-memory only, runtime instances are not persisted or reconciled, plugin lifecycle events are not audited or broadcast through a dedicated event hub, and the marketplace endpoint still returns placeholder data instead of a real catalog.

This gap now blocks the plugin dashboard, built-in plugin operations, and future IM / workflow integrations from relying on the Go server as the authoritative plugin host. We need to complete the Go-side control plane before more plugin kinds and operator workflows are added on top of an unstable foundation.

## What Changes

- Persist plugin registry records and runtime instance state in the Go server instead of keeping the authoritative plugin registry in process memory only.
- Add a Go control-plane event pipeline that records plugin lifecycle and runtime events, reconciles host updates, and exposes operator-visible audit data.
- Replace placeholder marketplace responses with a real catalog feed sourced from built-in and installable plugin manifests the Go server can actually serve.
- Strengthen Go-side activation and invocation flows so runtime state reconciliation, permission-aware execution checks, and lifecycle transitions happen through one authoritative path.
- Normalize Go plugin management APIs around the real control-plane model used by the dashboard and future bridge/runtime integrations.

## Capabilities

### New Capabilities
- `plugin-instance-state`: Persist and reconcile plugin runtime instances so operators can inspect which runtime artifact is active, degraded, or restarting.
- `plugin-event-audit`: Capture plugin lifecycle and runtime events in an append-only audit log and broadcast them through the Go event hub.
- `plugin-catalog-feed`: Serve built-in and installable plugin catalog data from real manifests instead of hardcoded placeholder marketplace entries.

### Modified Capabilities
- `plugin-registry`: Change the registry requirements so the Go server owns persistent plugin records, source metadata, instance linkage, and control-plane queries.
- `plugin-runtime`: Change runtime requirements so activation, health, restart, invoke, and runtime-state reconciliation flow through the authoritative Go control plane with permission-aware guards.

## Impact

- Affected Go code: `src-go/internal/service/plugin_service.go`, `src-go/internal/handler/plugin_handler.go`, `src-go/internal/model/plugin.go`, `src-go/internal/server/routes.go`, new repository/event components under `src-go/internal/repository` and `src-go/internal/service`.
- Affected data model: new migrations for plugin registry persistence, plugin instance tracking, and plugin event audit storage.
- Affected APIs: `/api/v1/plugins*` management endpoints, runtime-state sync flow, operator-facing plugin catalog responses, and plugin event delivery.
- Affected systems: Go orchestrator plugin host, dashboard plugin management surface, built-in plugin discovery/install path, and future integration/workflow plugin execution.
