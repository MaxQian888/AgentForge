## 1. TS Bridge MCP Interaction Surface

- [ ] 1.1 Extend `src-bridge/src/mcp/*` to support prompt discovery/retrieval alongside the existing tool and resource primitives, and define typed DTOs for refresh, tool invocation, resource read, and prompt access flows.
- [ ] 1.2 Update `src-bridge/src/plugins/tool-plugin-manager.ts` and related plugin types so each active `ToolPlugin` maintains a refreshable MCP interaction snapshot with transport details, discovery timestamps, capability counts, and latest interaction summaries.
- [ ] 1.3 Add typed Bridge routes in `src-bridge/src/server.ts` for MCP capability refresh, tool invocation, resource reading, and prompt retrieval, with structured error mapping and audit-friendly result summaries.

## 2. Go Control-Plane MCP Proxy

- [ ] 2.1 Extend `src-go/internal/bridge/client.go` with typed methods for MCP capability refresh, tool invocation, resource reading, and prompt retrieval against TS-hosted `ToolPlugin` instances.
- [ ] 2.2 Refactor `src-go/internal/service/plugin_service.go` so operator-triggered MCP interactions validate plugin kind/state, call the new bridge client methods, and append structured plugin events for success and failure outcomes.
- [ ] 2.3 Add or update plugin handler and route surfaces in `src-go/internal/handler/plugin_handler.go` and `src-go/internal/server/routes.go` so authenticated callers can use the new MCP interaction APIs through Go instead of reaching the bridge directly.

## 3. Registry And Runtime-State Synchronization

- [ ] 3.1 Extend Go and TS plugin runtime models to carry MCP interaction snapshot summaries in runtime metadata without persisting full MCP payloads.
- [ ] 3.2 Update the TS runtime reporter payload and Go runtime-state sync path (`/internal/plugins/runtime-state`) so refreshed capability data and latest interaction diagnostics reconcile into the authoritative plugin record.
- [ ] 3.3 Ensure plugin event audit entries capture MCP discovery and interaction operations with bounded summaries, typed error categories, and timestamps suitable for operator diagnostics.

## 4. Verification And Documentation

- [ ] 4.1 Add focused TS Bridge tests for MCP prompt/resource discovery, capability refresh, typed interaction routes, and runtime snapshot updates.
- [ ] 4.2 Add focused Go tests for bridge client methods, plugin service validation/audit behavior, handler routes, and registry synchronization of MCP interaction metadata.
- [ ] 4.3 Update plugin-facing documentation to describe the real MCP interaction surface, supported operator actions, runtime metadata fields, and scoped verification commands for this repo.
