## 1. Composition Root And Route Wiring

- [x] 1.1 Instantiate the WebSocket hub, bridge client, worktree manager, and agent runtime service in `src-go/cmd/server/main.go`, and start the hub lifecycle with server startup.
- [x] 1.2 Update route registration so `/api/v1/agents/spawn` and `/ws` receive service-backed dependencies instead of the repository-only agent handler and echo-style WebSocket handler.

## 2. Spawn Orchestration And State Persistence

- [x] 2.1 Extend the agent service and supporting repository interfaces so a spawn request can create a run, allocate a worktree, call the bridge execute API, and persist `agent_branch`, `agent_worktree`, and `agent_session_id` on the task.
- [x] 2.2 Implement compensation logic for partial startup failures, including failed run status updates and worktree cleanup when bridge startup does not succeed.
- [x] 2.3 Update HTTP error mapping for spawn and cancel paths so duplicate-run, invalid-state, and bridge/worktree failures return consistent API responses.

## 3. Realtime Delivery And Verification

- [x] 3.1 Switch the WebSocket endpoint to the hub-backed handler and ensure agent lifecycle broadcasts reach authenticated clients without relying on echo behavior.
- [x] 3.2 Add or update focused Go tests for route wiring, WebSocket authentication/server-push behavior, and agent spawn success/failure flows.
- [x] 3.3 Run targeted Go test suites covering the touched server, handler, service, repository, and WebSocket packages.
