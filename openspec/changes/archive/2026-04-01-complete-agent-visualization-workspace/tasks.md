## 1. Graph Foundation

- [x] 1.1 Add the React Flow dependency and wire its required base styles into the existing frontend style entrypoints without changing the current `/agents` route contract.
- [x] 1.2 Create a dedicated agent-graph mapping seam that converts the current `agents`, `pool.queue`, `runtimeCatalog`, `bridgeHealth`, and workspace scope into deterministic nodes, edges, and summary metadata.
- [x] 1.3 Add focused tests for the graph mapping seam so grouped runtime targets, queued dispatch entries, and scoped member filtering are stable before UI integration.

## 2. Visualization Workspace Integration

- [x] 2.1 Build the Agent visualization canvas and node renderers under `components/agents/*`, covering task, queued or blocked dispatch, agent, and runtime summaries with operator-readable status cues.
- [x] 2.2 Integrate the visualization as a first-class view inside `AgentWorkspace`, keeping the existing sidebar, monitor/dispatch framing, and URL-driven agent detail flow intact.
- [x] 2.3 Connect agent-node selection to the current `?agent=` deep-link behavior so graph-driven navigation lands in the existing `AgentWorkspaceDetail` surface.

## 3. States And Copy

- [x] 3.1 Implement explicit loading, empty, no-match, and degraded states for the visualization using the same workspace semantics already used on `/agents`.
- [x] 3.2 Add or update the required `agents` translation keys and visual legend/status copy in both `messages/en/*` and `messages/zh-CN/*`.
- [x] 3.3 Ensure graph nodes and surrounding summaries preserve consistent status, runtime, and budget emphasis with the rest of the agent workspace.

## 4. Verification

- [x] 4.1 Add or update component tests for the visualization canvas states and node summaries in `components/agents/*.test.tsx`.
- [x] 4.2 Update `/agents` workspace tests to cover view switching, visualization rendering, and graph-driven agent selection without breaking existing monitor/dispatch behavior.
- [x] 4.3 Run targeted frontend verification for the touched agents workspace surfaces and record any remaining repo-level blockers separately from this change.
