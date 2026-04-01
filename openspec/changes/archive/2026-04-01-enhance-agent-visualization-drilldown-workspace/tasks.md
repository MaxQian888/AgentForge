## 1. Workspace Query State

- [x] 1.1 Extend the `/agents` workspace query-state handling so `view` and visualization focus survive reload, browser navigation, and deep links while keeping `agent` selection as the higher-priority detail state.
- [x] 1.2 Update workspace tests to cover visualization deep-link hydration, invalid `vizNode` fallback, and return-to-visualization behavior after closing agent detail.

## 2. Visualization Focus Model

- [x] 2.1 Expand `buildAgentVisualizationModel()` so it emits stable focus metadata for task, dispatch, and runtime nodes in addition to the existing nodes, edges, and summary.
- [x] 2.2 Update visualization node selection handling so non-agent nodes set or clear visualization focus while agent nodes continue to route into the existing `?agent=` detail flow.

## 3. Drilldown Surface

- [x] 3.1 Build the visualization drilldown surface for focused task, dispatch, and runtime nodes under `components/agents/*`, keeping the graph visible while the contextual panel is open.
- [x] 3.2 Add explicit loading, empty, and clear-focus states for visualization drilldown, including a mobile-friendly presentation that does not make the graph unreadable.

## 4. Shared Data Reuse

- [x] 4.1 Reuse `agent-store.fetchDispatchHistory(taskId)` and existing dispatch history presentation to populate task and dispatch drilldown context without introducing new backend APIs.
- [x] 4.2 Reuse runtime catalog availability, diagnostics, and connected graph relationships to populate runtime drilldown context and operator-readable summaries.
- [x] 4.3 Add or update the required `agents` translation keys in `messages/en/*` and `messages/zh-CN/*` for visualization focus, drilldown labels, and no-data copy.

## 5. Verification

- [x] 5.1 Add or update focused tests for visualization model focus metadata, node-driven drilldown rendering, and query-state behavior in the touched `components/agents/*.test.tsx` surfaces.
- [x] 5.2 Run targeted frontend verification for the updated agents workspace surfaces and record any unrelated repo-level blockers separately from this change.
