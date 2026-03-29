## 1. OpenCode transport and readiness foundation

- [x] 1.1 Add a Bridge-local OpenCode transport client around the official `opencode serve` APIs, including server URL parsing, optional basic auth, health probing, and provider/model discovery helpers
- [x] 1.2 Extend OpenCode runtime configuration parsing so `opencode` readiness validates transport reachability and upstream provider/model availability instead of only checking whether the `opencode` executable exists
- [x] 1.3 Add focused unit tests for OpenCode transport health, auth failure, unreachable server, and provider/model diagnostics paths

## 2. OpenCode runtime execution and continuity binding

- [x] 2.1 Replace the current `opencode` command-adapter execution path in the runtime registry with an OpenCode-specific adapter that starts work through the official transport and records the bound upstream session identity
- [x] 2.2 Extend Bridge runtime snapshot or continuity metadata to persist OpenCode session binding details needed for truthful pause/resume without breaking Claude/Codex snapshots
- [x] 2.3 Implement OpenCode event normalization and reconcile logic so upstream output, tool activity, usage, and terminal states map to canonical AgentForge runtime events even after stream interruption

## 3. Lifecycle control and bridge route behavior

- [x] 3.1 Update OpenCode cancel and pause flows so they stop the upstream generation through the official control plane and preserve or discard resumable state according to the route semantics
- [x] 3.2 Update `/bridge/resume` for `opencode` so it continues the saved upstream session instead of replaying the original execute payload
- [x] 3.3 Ensure status, error classification, and terminal cleanup paths remain truthful for OpenCode runs, including degraded transport failures and non-resumable cancel behavior

## 4. Verification and documentation

- [x] 4.1 Add focused bridge tests for OpenCode execute, pause, resume, cancel, and transport-reconcile behavior using official-transport-shaped fixtures or stubs
- [x] 4.2 Update Bridge documentation and env/setup guidance to describe OpenCode server prerequisites, auth configuration, readiness diagnostics, and local verification steps
- [x] 4.3 Run scoped validation for the changed bridge modules and record the exact verification commands needed to prove OpenCode connectivity is working end to end
