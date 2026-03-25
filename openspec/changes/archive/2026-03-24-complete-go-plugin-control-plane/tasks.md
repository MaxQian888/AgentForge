## 1. Persistence Foundations

- [x] 1.1 Add plugin control-plane migrations for persistent registry records, runtime instance snapshots, and plugin event audit storage.
- [x] 1.2 Implement Go repositories for plugin records, current runtime instance snapshots, and plugin event audit entries, including the documented degraded fallback when the main database is unavailable.
- [x] 1.3 Extend plugin models and server wiring so the Go control plane can load persistent plugin state before handling plugin operations.

## 2. Control-Plane Service Flow

- [x] 2.1 Refactor the plugin service so install, enable, disable, activate, health, restart, invoke, runtime-sync, and uninstall all update the persistent registry and current runtime instance snapshot through one authoritative path.
- [x] 2.2 Add a plugin event service that appends structured lifecycle/runtime audit records for each control-plane action.
- [x] 2.3 Enforce manifest-declared capabilities and permission-aware activation or invocation checks before the Go runtime or TS bridge is called.

## 3. Catalog And Operator APIs

- [x] 3.1 Replace the hardcoded marketplace response with a manifest-backed catalog service that merges built-in discovery data with installed registry state.
- [x] 3.2 Update plugin handlers and routes so operator-facing plugin APIs return control-plane-backed lifecycle, catalog, and runtime reconciliation results.
- [x] 3.3 Broadcast operator-relevant plugin lifecycle events through the Go-side plugin event hub so dashboards can observe state changes without relying on placeholder polling semantics.

## 4. Verification

- [x] 4.1 Add focused repository and service tests for persistence, restart survival, runtime-state reconciliation, and permission or capability rejection flows.
- [x] 4.2 Add handler or route tests covering catalog responses, audit-producing control-plane actions, and runtime-sync behavior.
- [x] 4.3 Run the relevant Go migration, repository, service, and handler verification commands and document any scoped follow-up constraints.
