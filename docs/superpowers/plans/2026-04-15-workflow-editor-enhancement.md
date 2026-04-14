# Workflow Editor Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the workflow editor into an independent module and add node config panels, edge condition editor, data flow preview, and canvas interaction enhancements.

**Architecture:** New `components/workflow-editor/` module with React Context + useReducer for internal state (decoupled from Zustand), a node registry as single source of truth for all 14 node types, schema-driven config form generation with custom overrides for complex nodes, and a hybrid visual/expression condition builder shared across edge and node configuration.

**Tech Stack:** React 19, Next.js 16, ReactFlow (`@xyflow/react`), Tailwind CSS v4, shadcn/ui (Radix), Zustand (API layer only), Jest + Testing Library

**Prerequisites:** Verify `@xyflow/react` is installed: `pnpm list @xyflow/react`

**Spec clarifications applied in this plan:**
- The spec's module structure tree lists 13 files under `node-configs/`, but spec Section 4 states only 3 nodes need custom overrides (llm_agent, condition, sub_workflow). This plan follows Section 4: only 3 custom config files are created. All other node types use the schema-driven renderer.
- Importing *types* from `workflow-store.ts` (e.g., `WorkflowDefinition`) is acceptable for the public API contract — only store *functions* are avoided to maintain decoupling.

**Spec:** `docs/superpowers/specs/2026-04-15-workflow-editor-enhancement-design.md`

---

## File Structure

### New files to create

| File | Responsibility |
|------|----------------|
| `components/workflow-editor/types.ts` | Internal type definitions (Snapshot, EditorState, actions, ConfigField, NodeTypeMeta) |
| `components/workflow-editor/nodes/node-registry.ts` | All 14 node type metadata: icon, color, category, configSchema, defaultConfig |
| `components/workflow-editor/nodes/node-styles.ts` | Node style constants extracted from workflow-node-types.tsx |
| `components/workflow-editor/nodes/node-types.tsx` | 14 node type React components for ReactFlow (migrated + sub_workflow) |
| `components/workflow-editor/context.tsx` | EditorProvider, useEditor hook, reducer with all actions |
| `components/workflow-editor/hooks/use-undo-redo.ts` | Keyboard listener for Ctrl+Z/Ctrl+Shift+Z, toolbar state |
| `components/workflow-editor/hooks/use-editor-actions.ts` | Orchestrates save/execute/copy/paste/delete/selectAll/keyboard shortcuts |
| `components/workflow-editor/hooks/use-data-flow.ts` | DAG traversal to find upstream nodes and their output fields |
| `components/workflow-editor/toolbar/node-palette.tsx` | 14-node categorized drag+click palette with search |
| `components/workflow-editor/toolbar/editor-toolbar.tsx` | Toolbar with name/desc inputs, save/execute/undo/redo buttons, palette toggle |
| `components/workflow-editor/canvas/custom-edge.tsx` | Custom edge with click selection, condition label display |
| `components/workflow-editor/canvas/snap-grid.ts` | Alignment guide calculation (nearest neighbors, 8px threshold) |
| `components/workflow-editor/canvas/editor-canvas.tsx` | ReactFlow canvas with custom nodes/edges, drag-drop, multi-select |
| `components/workflow-editor/config-panel/condition-builder.tsx` | Hybrid visual/expression condition builder |
| `components/workflow-editor/config-panel/data-flow-preview.tsx` | Upstream node output field preview with copy buttons |
| `components/workflow-editor/config-panel/node-config-panel.tsx` | Right drawer accordion panel, schema-driven form renderer |
| `components/workflow-editor/config-panel/edge-config-panel.tsx` | Edge label + condition editing panel |
| `components/workflow-editor/config-panel/node-configs/llm-agent-config.tsx` | Custom: runtime→provider→model cascade |
| `components/workflow-editor/config-panel/node-configs/condition-config.tsx` | Custom: embeds condition-builder |
| `components/workflow-editor/config-panel/node-configs/sub-workflow-config.tsx` | Custom: workflow selector + input mapping JSON |
| `components/workflow-editor/workflow-editor.tsx` | Shell component: layout canvas + toolbar + right panel |
| `components/workflow-editor/index.ts` | Public API exports |

### Files to modify

| File | Change |
|------|--------|
| `app/(dashboard)/workflow/page.tsx` | Replace `WorkflowCanvas` import with `WorkflowEditor`, wire onSave/onExecute callbacks |

### Files to delete (after verification)

| File | Reason |
|------|--------|
| `components/workflow/workflow-canvas.tsx` | Migrated to workflow-editor/canvas/editor-canvas.tsx |
| `components/workflow/workflow-node-types.tsx` | Migrated to workflow-editor/nodes/ |
| `components/workflow/workflow-toolbar.tsx` | Migrated to workflow-editor/toolbar/ |

### Test files to create

| File | Tests |
|------|-------|
| `components/workflow-editor/nodes/node-registry.test.ts` | Registry completeness (14 types), category mapping, configSchema validity |
| `components/workflow-editor/context.test.ts` | All reducer actions, undo/redo stack, snapshot management |
| `components/workflow-editor/hooks/use-data-flow.test.ts` | DAG traversal, predecessor discovery |
| `components/workflow-editor/config-panel/condition-builder.test.tsx` | Visual→expression serialization, expression→visual parsing, mode switching |
| `components/workflow-editor/workflow-editor.test.tsx` | Integration: renders canvas + toolbar, node selection opens panel |

---

## Task 1: Types and Node Registry

**Files:**
- Create: `components/workflow-editor/types.ts`
- Create: `components/workflow-editor/nodes/node-registry.ts`
- Create: `components/workflow-editor/nodes/node-styles.ts`
- Test: `components/workflow-editor/nodes/node-registry.test.ts`

- [ ] **Step 1: Write the types file**

```ts
// components/workflow-editor/types.ts
import type { Node, Edge } from "@xyflow/react";
import type { LucideIcon } from "lucide-react";

// --- Workflow data types (aligned with workflow-store but decoupled) ---

export interface WorkflowNodeData {
  id: string;
  type: string;
  label: string;
  position: { x: number; y: number };
  config?: Record<string, unknown>;
}

export interface WorkflowEdgeData {
  id: string;
  source: string;
  target: string;
  condition?: string;
  label?: string;
}

// --- Editor state types ---

export type Snapshot = { nodes: Node[]; edges: Edge[] };

export type EditorAction =
  | { type: "LOAD"; name: string; description: string; nodes: Node[]; edges: Edge[] }
  | { type: "UPDATE_NAME"; name: string }
  | { type: "UPDATE_DESCRIPTION"; description: string }
  | { type: "ADD_NODE"; node: Node }
  | { type: "DELETE_NODES"; nodeIds: string[] }
  | { type: "UPDATE_NODE_CONFIG"; nodeId: string; config: Record<string, unknown> }
  | { type: "UPDATE_NODE_LABEL"; nodeId: string; label: string }
  | { type: "ADD_EDGE"; edge: Edge }
  | { type: "DELETE_EDGE"; edgeId: string }
  | { type: "UPDATE_EDGE_CONDITION"; edgeId: string; condition?: string; label?: string }
  | { type: "SELECT_NODE"; nodeId: string | null }
  | { type: "SELECT_EDGE"; edgeId: string | null }
  | { type: "DESELECT" }
  | { type: "UNDO" }
  | { type: "REDO" }
  | { type: "COPY"; nodes: Node[] }
  | { type: "PASTE" }
  | { type: "MARK_CLEAN" }
  | { type: "SYNC_REACTFLOW"; nodes: Node[]; edges: Edge[] };

export interface EditorState {
  name: string;
  description: string;
  nodes: Node[];
  edges: Edge[];
  selectedNodeId: string | null;
  selectedEdgeId: string | null;
  undoStack: Snapshot[];
  redoStack: Snapshot[];
  dirty: boolean;
  clipboard: Node[];
}

// --- Node registry types ---

export type NodeCategory = "entry" | "logic" | "agent" | "flow" | "human" | "action";

export interface ConfigField {
  key: string;
  label: string;
  type: "text" | "textarea" | "select" | "number" | "boolean" | "expression" | "json";
  options?: { value: string; label: string }[];
  placeholder?: string;
  required?: boolean;
  group: string;
}

export interface NodeTypeMeta {
  type: string;
  label: string;
  category: NodeCategory;
  icon: LucideIcon;
  color: string;
  description: string;
  configSchema: ConfigField[];
  defaultConfig: Record<string, unknown>;
}
```

- [ ] **Step 2: Write the node-styles file**

Extract the style constants from the existing `workflow-node-types.tsx` into `components/workflow-editor/nodes/node-styles.ts`:

```ts
// components/workflow-editor/nodes/node-styles.ts
export const NODE_STYLES: Record<string, { bg: string; border: string; iconColor: string }> = {
  trigger: {
    bg: "bg-green-50 dark:bg-green-950",
    border: "border-green-400 dark:border-green-600",
    iconColor: "text-green-600 dark:text-green-400",
  },
  condition: {
    bg: "bg-amber-50 dark:bg-amber-950",
    border: "border-amber-400 dark:border-amber-600",
    iconColor: "text-amber-600 dark:text-amber-400",
  },
  agent_dispatch: {
    bg: "bg-blue-50 dark:bg-blue-950",
    border: "border-blue-400 dark:border-blue-600",
    iconColor: "text-blue-600 dark:text-blue-400",
  },
  notification: {
    bg: "bg-yellow-50 dark:bg-yellow-950",
    border: "border-yellow-400 dark:border-yellow-600",
    iconColor: "text-yellow-600 dark:text-yellow-400",
  },
  status_transition: {
    bg: "bg-purple-50 dark:bg-purple-950",
    border: "border-purple-400 dark:border-purple-600",
    iconColor: "text-purple-600 dark:text-purple-400",
  },
  gate: {
    bg: "bg-red-50 dark:bg-red-950",
    border: "border-red-400 dark:border-red-600",
    iconColor: "text-red-600 dark:text-red-400",
  },
  parallel_split: {
    bg: "bg-orange-50 dark:bg-orange-950",
    border: "border-orange-400 dark:border-orange-600",
    iconColor: "text-orange-600 dark:text-orange-400",
  },
  parallel_join: {
    bg: "bg-orange-50 dark:bg-orange-950",
    border: "border-orange-400 dark:border-orange-600",
    iconColor: "text-orange-600 dark:text-orange-400",
  },
  llm_agent: {
    bg: "bg-indigo-50 dark:bg-indigo-950",
    border: "border-indigo-400 dark:border-indigo-600",
    iconColor: "text-indigo-600 dark:text-indigo-400",
  },
  function: {
    bg: "bg-cyan-50 dark:bg-cyan-950",
    border: "border-cyan-400 dark:border-cyan-600",
    iconColor: "text-cyan-600 dark:text-cyan-400",
  },
  loop: {
    bg: "bg-pink-50 dark:bg-pink-950",
    border: "border-pink-400 dark:border-pink-600",
    iconColor: "text-pink-600 dark:text-pink-400",
  },
  human_review: {
    bg: "bg-emerald-50 dark:bg-emerald-950",
    border: "border-emerald-400 dark:border-emerald-600",
    iconColor: "text-emerald-600 dark:text-emerald-400",
  },
  wait_event: {
    bg: "bg-slate-50 dark:bg-slate-950",
    border: "border-slate-400 dark:border-slate-600",
    iconColor: "text-slate-600 dark:text-slate-400",
  },
  sub_workflow: {
    bg: "bg-violet-50 dark:bg-violet-950",
    border: "border-violet-400 dark:border-violet-600",
    iconColor: "text-violet-600 dark:text-violet-400",
  },
};

/** MiniMap node color lookup */
export const MINIMAP_COLORS: Record<string, string> = {
  trigger: "#22c55e",
  condition: "#f59e0b",
  agent_dispatch: "#3b82f6",
  notification: "#eab308",
  status_transition: "#a855f7",
  gate: "#ef4444",
  parallel_split: "#f97316",
  parallel_join: "#f97316",
  llm_agent: "#6366f1",
  function: "#06b6d4",
  loop: "#ec4899",
  human_review: "#10b981",
  wait_event: "#64748b",
  sub_workflow: "#8b5cf6",
};
```

- [ ] **Step 3: Write the failing test for node registry**

```ts
// components/workflow-editor/nodes/node-registry.test.ts
import { NODE_REGISTRY, getNodeMeta, getNodesByCategory } from "./node-registry";

const ALL_NODE_TYPES = [
  "trigger", "condition", "agent_dispatch", "notification", "status_transition",
  "gate", "parallel_split", "parallel_join", "llm_agent", "function",
  "loop", "human_review", "wait_event", "sub_workflow",
];

describe("node-registry", () => {
  it("registers all 14 node types", () => {
    expect(NODE_REGISTRY).toHaveLength(14);
    const types = NODE_REGISTRY.map((n) => n.type);
    for (const t of ALL_NODE_TYPES) {
      expect(types).toContain(t);
    }
  });

  it("getNodeMeta returns correct entry for each type", () => {
    for (const t of ALL_NODE_TYPES) {
      const meta = getNodeMeta(t);
      expect(meta).toBeDefined();
      expect(meta!.type).toBe(t);
      expect(meta!.label).toBeTruthy();
      expect(meta!.icon).toBeDefined();
      expect(meta!.category).toBeTruthy();
    }
  });

  it("getNodeMeta returns undefined for unknown type", () => {
    expect(getNodeMeta("nonexistent")).toBeUndefined();
  });

  it("getNodesByCategory returns correct groupings", () => {
    const entry = getNodesByCategory("entry");
    expect(entry.map((n) => n.type)).toEqual(["trigger"]);

    const flow = getNodesByCategory("flow");
    expect(flow.map((n) => n.type)).toEqual(
      expect.arrayContaining(["parallel_split", "parallel_join", "loop", "sub_workflow"])
    );
  });

  it("every node has a valid configSchema with group fields", () => {
    for (const meta of NODE_REGISTRY) {
      for (const field of meta.configSchema) {
        expect(field.key).toBeTruthy();
        expect(field.group).toBeTruthy();
        expect(["text", "textarea", "select", "number", "boolean", "expression", "json"]).toContain(field.type);
      }
    }
  });
});
```

- [ ] **Step 4: Run test to verify it fails**

Run: `pnpm test -- --testPathPattern="node-registry" --no-coverage`
Expected: FAIL — module not found

- [ ] **Step 5: Write the node registry implementation**

Create `components/workflow-editor/nodes/node-registry.ts` with all 14 node entries. Each entry has: type, label, category, icon (from lucide-react), color, description, configSchema (array of ConfigField per the spec's per-node-type config fields table), and defaultConfig. Use the icons from the existing `workflow-node-types.tsx` (Play, GitBranch, Bot, Bell, ArrowRightLeft, Lock, Split, Merge, BrainCircuit, Code2, RefreshCw, UserCheck, Webhook) plus `Workflow` for sub_workflow.

Key configSchema examples:
- `llm_agent`: group "Agent Config" fields for runtime (select), provider (select), model (select), budgetUsd (number), prompt (textarea), systemPrompt (textarea)
- `condition`: group "Condition" with expression field (type: "expression")
- `loop`: group "Loop Config" with maxIterations (number) and exitCondition (expression)
- `sub_workflow`: group "Sub-Workflow" with workflowId (select) and inputMapping (json)
- `agent_dispatch`: group "Agent Config" with runtime (select), provider (select), model (select), budgetUsd (number) — same as llm_agent minus prompt/systemPrompt
- `trigger`, `parallel_split`, `parallel_join`: empty configSchema (label-only)

Export: `NODE_REGISTRY: NodeTypeMeta[]`, `getNodeMeta(type: string): NodeTypeMeta | undefined`, `getNodesByCategory(category: NodeCategory): NodeTypeMeta[]`.

- [ ] **Step 6: Run test to verify it passes**

Run: `pnpm test -- --testPathPattern="node-registry" --no-coverage`
Expected: PASS — all 5 tests green

- [ ] **Step 7: Commit**

```bash
git add components/workflow-editor/types.ts components/workflow-editor/nodes/
git commit -m "feat(workflow-editor): add types, node registry, and node styles"
```

---

## Task 2: EditorContext (Reducer + Provider)

**Files:**
- Create: `components/workflow-editor/context.tsx`
- Test: `components/workflow-editor/context.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// components/workflow-editor/context.test.ts
import { editorReducer } from "./context";
import type { EditorState, EditorAction } from "./types";

const emptyState: EditorState = {
  name: "",
  description: "",
  nodes: [],
  edges: [],
  selectedNodeId: null,
  selectedEdgeId: null,
  undoStack: [],
  redoStack: [],
  dirty: false,
  clipboard: [],
};

function makeNode(id: string, type = "trigger") {
  return { id, type, position: { x: 0, y: 0 }, data: { label: id } };
}

function makeEdge(id: string, source: string, target: string) {
  return { id, source, target };
}

describe("editorReducer", () => {
  it("LOAD initializes state from definition", () => {
    const nodes = [makeNode("n1")];
    const edges = [makeEdge("e1", "n1", "n2")];
    const next = editorReducer(emptyState, {
      type: "LOAD", name: "Test", description: "Desc", nodes, edges,
    });
    expect(next.name).toBe("Test");
    expect(next.description).toBe("Desc");
    expect(next.nodes).toEqual(nodes);
    expect(next.undoStack).toEqual([]);
    expect(next.redoStack).toEqual([]);
    expect(next.dirty).toBe(false);
  });

  it("ADD_NODE pushes undo snapshot and adds node", () => {
    const state = { ...emptyState, nodes: [makeNode("n1")] };
    const newNode = makeNode("n2", "llm_agent");
    const next = editorReducer(state, { type: "ADD_NODE", node: newNode });
    expect(next.nodes).toHaveLength(2);
    expect(next.undoStack).toHaveLength(1);
    expect(next.dirty).toBe(true);
  });

  it("DELETE_NODES removes nodes and connected edges", () => {
    const state = {
      ...emptyState,
      nodes: [makeNode("n1"), makeNode("n2"), makeNode("n3")],
      edges: [makeEdge("e1", "n1", "n2"), makeEdge("e2", "n2", "n3")],
    };
    const next = editorReducer(state, { type: "DELETE_NODES", nodeIds: ["n2"] });
    expect(next.nodes).toHaveLength(2);
    expect(next.edges).toHaveLength(0); // both edges connect to n2
  });

  it("UPDATE_NODE_CONFIG merges config", () => {
    const node = { ...makeNode("n1"), data: { label: "n1", config: { a: 1 } } };
    const state = { ...emptyState, nodes: [node] };
    const next = editorReducer(state, {
      type: "UPDATE_NODE_CONFIG", nodeId: "n1", config: { b: 2 },
    });
    expect(next.nodes[0].data.config).toEqual({ a: 1, b: 2 });
    expect(next.dirty).toBe(true);
  });

  it("UNDO restores previous state and pushes to redo", () => {
    let state = emptyState;
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n1") });
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n2") });
    expect(state.nodes).toHaveLength(2);
    expect(state.undoStack).toHaveLength(2);

    state = editorReducer(state, { type: "UNDO" });
    expect(state.nodes).toHaveLength(1);
    expect(state.redoStack).toHaveLength(1);
  });

  it("REDO restores and pushes back to undo", () => {
    let state = emptyState;
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n1") });
    state = editorReducer(state, { type: "UNDO" });
    expect(state.nodes).toHaveLength(0);

    state = editorReducer(state, { type: "REDO" });
    expect(state.nodes).toHaveLength(1);
  });

  it("new mutation clears redo stack", () => {
    let state = emptyState;
    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n1") });
    state = editorReducer(state, { type: "UNDO" });
    expect(state.redoStack).toHaveLength(1);

    state = editorReducer(state, { type: "ADD_NODE", node: makeNode("n2") });
    expect(state.redoStack).toHaveLength(0);
  });

  it("undo stack is capped at 50", () => {
    let state = emptyState;
    for (let i = 0; i < 55; i++) {
      state = editorReducer(state, { type: "ADD_NODE", node: makeNode(`n${i}`) });
    }
    expect(state.undoStack.length).toBeLessThanOrEqual(50);
  });

  it("COPY and PASTE duplicate nodes with new IDs", () => {
    const n1 = makeNode("n1");
    let state = { ...emptyState, nodes: [n1] };
    state = editorReducer(state, { type: "COPY", nodes: [n1] });
    expect(state.clipboard).toHaveLength(1);

    state = editorReducer(state, { type: "PASTE" });
    expect(state.nodes).toHaveLength(2);
    expect(state.nodes[1].id).not.toBe("n1"); // new ID
    expect(state.nodes[1].position.x).toBe(n1.position.x + 50); // offset from original
  });

  it("SELECT_NODE clears selectedEdgeId", () => {
    const state = { ...emptyState, selectedEdgeId: "e1" };
    const next = editorReducer(state, { type: "SELECT_NODE", nodeId: "n1" });
    expect(next.selectedNodeId).toBe("n1");
    expect(next.selectedEdgeId).toBeNull();
  });

  it("MARK_CLEAN sets dirty to false", () => {
    const state = { ...emptyState, dirty: true };
    const next = editorReducer(state, { type: "MARK_CLEAN" });
    expect(next.dirty).toBe(false);
  });

  it("SYNC_REACTFLOW updates nodes/edges without pushing undo", () => {
    const state = { ...emptyState, nodes: [makeNode("n1")] };
    const newNodes = [makeNode("n1"), makeNode("n2")];
    const next = editorReducer(state, { type: "SYNC_REACTFLOW", nodes: newNodes, edges: [] });
    expect(next.nodes).toHaveLength(2);
    expect(next.undoStack).toHaveLength(0);
    expect(next.dirty).toBe(false);
  });

  it("UPDATE_EDGE_CONDITION updates edge condition and label", () => {
    const edge = makeEdge("e1", "n1", "n2");
    const state = { ...emptyState, edges: [edge] };
    const next = editorReducer(state, {
      type: "UPDATE_EDGE_CONDITION", edgeId: "e1",
      condition: "{{n1.output.ok}} == true", label: "if ok",
    });
    expect((next.edges[0] as any).data?.condition).toBe("{{n1.output.ok}} == true");
    expect(next.edges[0].label).toBe("if ok");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test -- --testPathPattern="context\\.test" --no-coverage`
Expected: FAIL — module not found

- [ ] **Step 3: Write the EditorContext implementation**

Create `components/workflow-editor/context.tsx`:
- Export `editorReducer(state: EditorState, action: EditorAction): EditorState` — pure function implementing all actions per the spec.
- Helper `pushUndo(state)` — creates snapshot `{nodes, edges}`, pushes to undoStack (capped at 50), clears redoStack.
- `PASTE` generates new IDs using `Date.now()` + counter, offsets position by (50, 50), preserves internal edges.
- `SYNC_REACTFLOW` — special action for ReactFlow `onNodesChange`/`onEdgesChange` callbacks; updates nodes/edges without pushing undo (handles drag, resize etc).
- Export `EditorProvider` component wrapping `React.createContext` with `useReducer`.
- Export `useEditor()` hook returning `{ state, dispatch }`.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm test -- --testPathPattern="context\\.test" --no-coverage`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/workflow-editor/context.tsx components/workflow-editor/context.test.ts
git commit -m "feat(workflow-editor): add EditorContext with reducer and undo/redo"
```

---

## Task 3: Node Types (ReactFlow Components)

**Files:**
- Create: `components/workflow-editor/nodes/node-types.tsx`

- [ ] **Step 1: Create node-types.tsx**

Migrate from `components/workflow/workflow-node-types.tsx`. Changes:
- Import styles from `./node-styles` instead of inline constants.
- Import metadata from `./node-registry` for the `workflowNodeTypes` map and `NODE_TYPE_LABELS`.
- Add `SubWorkflowNode` component (same pattern as others, using `Workflow` icon from lucide-react).
- Each node component renders `BaseWorkflowNode` (same pattern as existing code).
- Add `onClick` handler prop to `BaseWorkflowNode` that calls `dispatch({ type: "SELECT_NODE", nodeId })` — but since node components can't access context directly (ReactFlow renders them), use ReactFlow's `onNodeClick` on the canvas instead. Keep node components pure presentational.
- Export `workflowNodeTypes` map with all 14 entries and `NODE_TYPE_LABELS` record.

- [ ] **Step 2: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors in new files

- [ ] **Step 3: Commit**

```bash
git add components/workflow-editor/nodes/node-types.tsx
git commit -m "feat(workflow-editor): add 14 node type components"
```

---

## Task 4: Hooks (use-data-flow, use-undo-redo, use-editor-actions)

**Files:**
- Create: `components/workflow-editor/hooks/use-data-flow.ts`
- Create: `components/workflow-editor/hooks/use-undo-redo.ts`
- Create: `components/workflow-editor/hooks/use-editor-actions.ts`
- Test: `components/workflow-editor/hooks/use-data-flow.test.ts`

- [ ] **Step 1: Write the failing test for use-data-flow**

```ts
// components/workflow-editor/hooks/use-data-flow.test.ts
import { findPredecessors, getUpstreamOutputFields } from "./use-data-flow";
import { NODE_REGISTRY } from "../nodes/node-registry";

describe("use-data-flow", () => {
  const nodes = [
    { id: "trigger", type: "trigger", position: { x: 0, y: 0 }, data: { label: "Start" } },
    { id: "planner", type: "llm_agent", position: { x: 0, y: 100 }, data: { label: "Planner" } },
    { id: "coder", type: "llm_agent", position: { x: 0, y: 200 }, data: { label: "Coder" } },
  ];
  const edges = [
    { id: "e1", source: "trigger", target: "planner" },
    { id: "e2", source: "planner", target: "coder" },
  ];

  it("finds direct predecessors of a node", () => {
    const preds = findPredecessors("coder", nodes, edges);
    expect(preds.map((p) => p.id)).toEqual(["planner", "trigger"]);
  });

  it("returns empty for trigger (no predecessors)", () => {
    const preds = findPredecessors("trigger", nodes, edges);
    expect(preds).toEqual([]);
  });

  it("returns known output fields for llm_agent", () => {
    const fields = getUpstreamOutputFields("planner", "llm_agent");
    expect(fields.length).toBeGreaterThan(0);
    expect(fields[0].copyTemplate).toContain("{{planner.");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test -- --testPathPattern="use-data-flow" --no-coverage`
Expected: FAIL

- [ ] **Step 3: Implement use-data-flow**

Export `findPredecessors(nodeId, nodes, edges)` — BFS/DFS backward traversal returning nodes sorted by topological distance. Export `getUpstreamOutputFields(nodeId, nodeType)` — returns `{ path: string; copyTemplate: string }[]` based on node registry configSchema output hints. Fallback: `[{ path: "output.*", copyTemplate: "{{nodeId.output}}" }]`.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm test -- --testPathPattern="use-data-flow" --no-coverage`
Expected: PASS

- [ ] **Step 5: Implement use-undo-redo**

Create `components/workflow-editor/hooks/use-undo-redo.ts`:
- Custom hook that registers `keydown` listeners for Ctrl+Z (undo) and Ctrl+Shift+Z (redo).
- Calls `dispatch({ type: "UNDO" })` / `dispatch({ type: "REDO" })`.
- Returns `{ canUndo: boolean; canRedo: boolean; undo: () => void; redo: () => void }` for toolbar buttons.
- Uses `useEditor()` context hook.

- [ ] **Step 6: Implement use-editor-actions**

Create `components/workflow-editor/hooks/use-editor-actions.ts`:
- Registers keyboard shortcuts: Ctrl+S (save), Ctrl+C (copy), Ctrl+V (paste), Ctrl+A (select all), Delete/Backspace (delete selected), Escape (deselect).
- `handleSave`: converts context state nodes/edges → `WorkflowNodeData[]`/`WorkflowEdgeData[]`, calls `onSave` prop, dispatches `MARK_CLEAN` on success.
- `handleCopy`: reads ReactFlow selection via `useReactFlow().getNodes().filter(n => n.selected)`, dispatches `COPY`.
- `handlePaste`: dispatches `PASTE`.
- `handleDelete`: dispatches `DELETE_NODES` with selected node IDs.
- `handleAddNode(type)`: creates new Node with ID, type, position, default label from registry, dispatches `ADD_NODE`.
- Unsaved changes guard: `useEffect` registering `beforeunload` when `dirty === true`.

- [ ] **Step 7: Commit**

```bash
git add components/workflow-editor/hooks/
git commit -m "feat(workflow-editor): add data-flow, undo-redo, and editor-actions hooks"
```

---

## Task 5: Toolbar and Node Palette

**Files:**
- Create: `components/workflow-editor/toolbar/node-palette.tsx`
- Create: `components/workflow-editor/toolbar/editor-toolbar.tsx`

- [ ] **Step 1: Implement node-palette**

Create `components/workflow-editor/toolbar/node-palette.tsx`:
- Import `NODE_REGISTRY`, `getNodesByCategory` from node-registry.
- Render 6 category groups (entry, logic, agent, flow, human, action) as collapsible sections.
- Each section header shows category name, collapsible via state toggle.
- Each node item: icon + label, draggable (`onDragStart` sets `application/workflow-node-type`), clickable (calls `onAddNode(type)`).
- Search input at top filters nodes by label match.
- Use shadcn `Collapsible`, `Button`, `Input`, `Tooltip` components.

- [ ] **Step 2: Implement editor-toolbar**

Create `components/workflow-editor/toolbar/editor-toolbar.tsx`:
- Name input, description input, status badge (from existing toolbar pattern).
- Undo/Redo buttons (disabled state from `use-undo-redo` hook).
- Save button (calls `use-editor-actions` handleSave).
- Execute button (disabled if status !== "active").
- Palette toggle button showing/hiding `NodePalette`.
- Props: `status: string`, `saving: boolean`, `onExecute: () => void`.
- Internal: reads `useEditor()` for name/description dispatch, `useUndoRedo()` for undo/redo state.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add components/workflow-editor/toolbar/
git commit -m "feat(workflow-editor): add categorized node palette and editor toolbar"
```

---

## Task 6: Custom Edge and Canvas

**Files:**
- Create: `components/workflow-editor/canvas/custom-edge.tsx`
- Create: `components/workflow-editor/canvas/snap-grid.ts`
- Create: `components/workflow-editor/canvas/editor-canvas.tsx`

- [ ] **Step 1: Implement custom-edge**

Create `components/workflow-editor/canvas/custom-edge.tsx`:
- Custom edge component using ReactFlow's `getBezierPath` and `EdgeLabelRenderer`.
- Renders invisible wider path for click target (strokeWidth 20, opacity 0).
- Visible path: strokeWidth 2, default gray; selected: blue, strokeWidth 3.
- Label: if edge has `data.condition` or `label`, render centered on path with background pill.
- `onClick` calls `dispatch({ type: "SELECT_EDGE", edgeId })`.
- Export `customEdgeTypes = { default: CustomEdge }`.

- [ ] **Step 2: Implement snap-grid**

Create `components/workflow-editor/canvas/snap-grid.ts`:
- Export `calculateSnapLines(draggingNode, allNodes, threshold = 8, maxNeighbors = 20): { x: number[]; y: number[] }`.
- Filter to visible viewport nodes (accept viewport bounds param).
- Compare draggingNode center X/Y against other nodes' center X/Y.
- Return arrays of X and Y coordinates that are within threshold.
- Export `SnapLines` React component that renders SVG dashed lines at given coordinates.

- [ ] **Step 3: Implement editor-canvas**

Create `components/workflow-editor/canvas/editor-canvas.tsx`:
- Migrate from `components/workflow/workflow-canvas.tsx`.
- Use `useEditor()` context for nodes/edges (not local `useNodesState`/`useEdgesState`).
- `onNodesChange` / `onEdgesChange` → dispatch `SYNC_REACTFLOW`.
- `onNodeClick` → dispatch `SELECT_NODE`.
- `onEdgeClick` → dispatch `SELECT_EDGE`.
- `onPaneClick` → dispatch `DESELECT`.
- `onConnect` → dispatch `ADD_EDGE`.
- `onDrop` → dispatch `ADD_NODE` (read type from dataTransfer, create node via registry defaults).
- Props: `nodeTypes` from node-types.tsx, `edgeTypes` from custom-edge.tsx.
- Enable `selectionOnDrag`, `deleteKeyCode={["Backspace", "Delete"]}`.
- Render `Background`, `Controls`, `MiniMap` (using `MINIMAP_COLORS`).
- Integrate `onNodeDrag` for snap-grid (calculate + render SnapLines).

- [ ] **Step 4: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add components/workflow-editor/canvas/
git commit -m "feat(workflow-editor): add custom edge, snap grid, and editor canvas"
```

---

## Task 7: Condition Builder

**Files:**
- Create: `components/workflow-editor/config-panel/condition-builder.tsx`
- Test: `components/workflow-editor/config-panel/condition-builder.test.tsx`

- [ ] **Step 1: Write the failing test**

```ts
// components/workflow-editor/config-panel/condition-builder.test.tsx
import { parseExpression, serializeVisualRule } from "./condition-builder";

describe("condition-builder", () => {
  describe("serializeVisualRule", () => {
    it("serializes a basic comparison", () => {
      const expr = serializeVisualRule("planner", "output.ok", "==", "true");
      expect(expr).toBe("{{planner.output.ok}} == true");
    });

    it("serializes numeric comparison", () => {
      const expr = serializeVisualRule("classify", "output.urgency", ">", "0.7");
      expect(expr).toBe("{{classify.output.urgency}} > 0.7");
    });
  });

  describe("parseExpression", () => {
    it("parses simple template expression", () => {
      const result = parseExpression("{{planner.output.ok}} == true");
      expect(result).toEqual({
        nodeId: "planner",
        field: "output.ok",
        operator: "==",
        value: "true",
      });
    });

    it("returns null for complex expression", () => {
      const result = parseExpression("len({{planner.output.items}}) > 0");
      expect(result).toBeNull();
    });

    it("parses all supported operators", () => {
      for (const op of ["==", "!=", ">", "<", ">=", "<="]) {
        const result = parseExpression(`{{n.output.x}} ${op} 5`);
        expect(result?.operator).toBe(op);
      }
    });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test -- --testPathPattern="condition-builder" --no-coverage`
Expected: FAIL

- [ ] **Step 3: Implement condition-builder**

Create `components/workflow-editor/config-panel/condition-builder.tsx`:
- Export pure functions `serializeVisualRule(nodeId, field, operator, value): string` and `parseExpression(expr): { nodeId, field, operator, value } | null`.
- Export `ConditionBuilder` React component:
  - Props: `value: string`, `onChange: (expr: string) => void`, `upstreamNodes: { id: string; label: string; type: string }[]`.
  - State: `mode: "visual" | "expression"`.
  - Visual mode: 4 fields (node dropdown, field dropdown from `getUpstreamOutputFields`, operator dropdown, value input). Live preview below.
  - Expression mode: textarea with `{{` autocomplete (simple: on `{{` keypress show dropdown of upstream node IDs, on select insert `{{nodeId.`).
  - Mode toggle: radio group. Visual→Expression calls `serializeVisualRule`. Expression→Visual calls `parseExpression`, blocks if null with toast.
  - Use shadcn `Select`, `Input`, `Textarea`, `RadioGroup`, `Label`.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm test -- --testPathPattern="condition-builder" --no-coverage`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/workflow-editor/config-panel/condition-builder.tsx components/workflow-editor/config-panel/condition-builder.test.tsx
git commit -m "feat(workflow-editor): add hybrid condition builder with visual/expression modes"
```

---

## Task 8: Config Panels (Node + Edge + Data Flow)

**Files:**
- Create: `components/workflow-editor/config-panel/data-flow-preview.tsx`
- Create: `components/workflow-editor/config-panel/node-config-panel.tsx`
- Create: `components/workflow-editor/config-panel/edge-config-panel.tsx`
- Create: `components/workflow-editor/config-panel/node-configs/llm-agent-config.tsx`
- Create: `components/workflow-editor/config-panel/node-configs/condition-config.tsx`
- Create: `components/workflow-editor/config-panel/node-configs/sub-workflow-config.tsx`

- [ ] **Step 1: Implement data-flow-preview**

Create `components/workflow-editor/config-panel/data-flow-preview.tsx`:
- Uses `useEditor()` to get nodes/edges, `findPredecessors` from use-data-flow hook.
- Renders list of upstream nodes grouped as "Direct Inputs" (distance 1) and collapsed "Indirect" (distance 2+).
- Each node card: icon (from registry), label, type badge, expandable output fields.
- Output fields have copy button that copies `{{nodeId.output.field}}` to clipboard (via `navigator.clipboard.writeText`).
- Bottom hint text.
- Use shadcn `Collapsible`, `Button`, `Badge`, `Tooltip`.

- [ ] **Step 2: Implement node-config-panel**

Create `components/workflow-editor/config-panel/node-config-panel.tsx`:
- Props: none (reads `selectedNodeId` from `useEditor()`).
- Resolves node from context state, looks up `NodeTypeMeta` from registry.
- Header: node icon, inline-editable label (dispatches `UPDATE_NODE_LABEL`), close button (dispatches `DESELECT`).
- Accordion groups rendered from `configSchema`:
  - "General" group: label input (always present).
  - Type-specific groups: render form fields based on `ConfigField.type`:
    - `text` → `<Input>`
    - `textarea` → `<Textarea>`
    - `select` → `<Select>` with `options`
    - `number` → `<Input type="number">`
    - `boolean` → `<Switch>`
    - `expression` → `<ConditionBuilder>`
    - `json` → `<Textarea>` with JSON validation
  - Each field change dispatches `UPDATE_NODE_CONFIG` with merged config.
- "Data Flow" group: renders `<DataFlowPreview nodeId={...}>`.
- "Advanced" group: raw JSON textarea showing `JSON.stringify(config, null, 2)`.
- Bottom: Delete button (dispatches `DELETE_NODES`).
- Custom override: if node type is `llm_agent`, render `LLMAgentConfig` instead of schema-driven fields for the "Agent Config" group. Similarly for `condition` and `sub_workflow`.
- Use shadcn `Accordion`, `AccordionItem`, `AccordionTrigger`, `AccordionContent`, `Input`, `Textarea`, `Select`, `Switch`, `Button`, `ScrollArea`.

- [ ] **Step 3: Implement edge-config-panel**

Create `components/workflow-editor/config-panel/edge-config-panel.tsx`:
- Props: none (reads `selectedEdgeId` from `useEditor()`).
- Header: "Edge: {sourceName} → {targetName}", close button.
- Label section: input for optional display label.
- Condition section: renders `<ConditionBuilder>` with upstream nodes derived from source node's predecessors.
- Delete button (dispatches `DELETE_EDGE`).
- All changes dispatch `UPDATE_EDGE_CONDITION`.

- [ ] **Step 4: Implement custom config overrides**

Create `components/workflow-editor/config-panel/node-configs/llm-agent-config.tsx`:
- Three cascading selects: runtime → provider → model.
- Runtime options: `claude_code`, `codex`, `cursor`, `gemini`, `opencode`, `qoder`.
- Provider options depend on runtime (for now, hardcode `anthropic`, `openai`, `google` as all available for each).
- Model options depend on provider (hardcode common models per provider).
- Budget USD number input.
- Prompt and systemPrompt textareas.

Create `components/workflow-editor/config-panel/node-configs/condition-config.tsx`:
- Wraps `<ConditionBuilder>` with upstream nodes from `use-data-flow`.

Create `components/workflow-editor/config-panel/node-configs/sub-workflow-config.tsx`:
- Workflow selector: accepts `definitions: WorkflowDefinition[]` prop from parent (passed through props, not Zustand). Renders as `<Select>` with workflow names.
- Input mapping: `<Textarea>` for JSON key-value pairs.

- [ ] **Step 5: Verify TypeScript compiles**

Run: `pnpm exec tsc --noEmit --pretty 2>&1 | head -20`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add components/workflow-editor/config-panel/
git commit -m "feat(workflow-editor): add node/edge config panels with data flow preview"
```

---

## Task 9: Shell Component and Public API

**Files:**
- Create: `components/workflow-editor/workflow-editor.tsx`
- Create: `components/workflow-editor/index.ts`
- Test: `components/workflow-editor/workflow-editor.test.tsx`

- [ ] **Step 1: Implement workflow-editor shell**

Create `components/workflow-editor/workflow-editor.tsx`:
- `"use client"` directive.
- Props: `WorkflowEditorProps` (definition, onSave, onExecute).
- Wraps children in `<EditorProvider>`.
- On mount, dispatch `LOAD` from definition.
- Layout: flex row
  - Left/center: `<EditorToolbar>` on top, `<EditorCanvas>` filling remaining space.
  - Right (conditional): if `selectedNodeId`, render `<NodeConfigPanel>` in a 320px-wide right panel with slide-in animation. If `selectedEdgeId`, render `<EdgeConfigPanel>` instead.
- Pass `onSave` and `onExecute` through to `useEditorActions` hook.
- `useEditorActions` handles keyboard shortcuts and beforeunload.

- [ ] **Step 2: Implement index.ts**

```ts
// components/workflow-editor/index.ts
export { WorkflowEditor } from "./workflow-editor";
export type { WorkflowEditorProps } from "./workflow-editor";
```

- [ ] **Step 3: Write integration test**

```ts
// components/workflow-editor/workflow-editor.test.tsx
import { render, screen } from "@testing-library/react";
import { WorkflowEditor } from "./workflow-editor";

// Mock ReactFlow (it requires browser APIs)
jest.mock("@xyflow/react", () => ({
  ReactFlow: ({ children }: any) => <div data-testid="reactflow">{children}</div>,
  Background: () => null,
  Controls: () => null,
  MiniMap: () => null,
  useNodesState: () => [[], jest.fn(), jest.fn()],
  useEdgesState: () => [[], jest.fn(), jest.fn()],
  useReactFlow: () => ({ getNodes: () => [], fitView: jest.fn(), screenToFlowPosition: jest.fn() }),
  ReactFlowProvider: ({ children }: any) => <div>{children}</div>,
  Handle: () => null,
  Position: { Top: "top", Bottom: "bottom" },
  MarkerType: { ArrowClosed: "arrowclosed" },
  BackgroundVariant: { Dots: "dots" },
  getBezierPath: () => ["", 0, 0],
  EdgeLabelRenderer: ({ children }: any) => <div>{children}</div>,
}));

const mockDefinition = {
  id: "wf-1",
  projectId: "p-1",
  name: "Test Workflow",
  description: "A test",
  status: "active",
  category: "user",
  nodes: [],
  edges: [],
  version: 1,
  createdAt: "2026-01-01",
  updatedAt: "2026-01-01",
};

describe("WorkflowEditor", () => {
  it("renders toolbar with workflow name", () => {
    render(
      <WorkflowEditor
        definition={mockDefinition}
        onSave={jest.fn()}
        onExecute={jest.fn()}
      />
    );
    expect(screen.getByDisplayValue("Test Workflow")).toBeInTheDocument();
  });

  it("renders the canvas area", () => {
    render(
      <WorkflowEditor
        definition={mockDefinition}
        onSave={jest.fn()}
        onExecute={jest.fn()}
      />
    );
    expect(screen.getByTestId("reactflow")).toBeInTheDocument();
  });
});
```

- [ ] **Step 4: Run tests**

Run: `pnpm test -- --testPathPattern="workflow-editor\\.test" --no-coverage`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/workflow-editor/workflow-editor.tsx components/workflow-editor/index.ts components/workflow-editor/workflow-editor.test.tsx
git commit -m "feat(workflow-editor): add shell component and public API"
```

---

## Task 10: Page Integration and Migration

**Files:**
- Modify: `app/(dashboard)/workflow/page.tsx:173-193`
- Delete (after verification): `components/workflow/workflow-canvas.tsx`, `components/workflow/workflow-node-types.tsx`, `components/workflow/workflow-toolbar.tsx`

- [ ] **Step 1: Update workflow page to use new editor**

In `app/(dashboard)/workflow/page.tsx`:
- Replace import `WorkflowCanvas` from `@/components/workflow/workflow-canvas` with `WorkflowEditor` from `@/components/workflow-editor`.
- In `WorkflowListTab`, replace the `<WorkflowCanvas definition={selectedDefinition} onExecute={handleExecute} />` block (around line 188) with:

```tsx
<WorkflowEditor
  definition={selectedDefinition}
  onSave={async (data) => {
    const ok = await updateDefinition(selectedDefinition.id, data);
    if (ok) toast.success("Workflow saved");
    return ok;
  }}
  onExecute={handleExecute}
/>
```

- Remove unused imports: `WorkflowCanvas`.
- Add unsaved-changes guard for "Back to list" button: add an `onDirtyChange?: (dirty: boolean) => void` callback prop to `WorkflowEditorProps`. The editor calls it whenever `dirty` changes. The page uses this to show a confirmation dialog when "Back to list" is clicked while dirty: `if (isDirty && !confirm("You have unsaved changes. Discard and leave?")) return;`

- [ ] **Step 2: Run existing workflow test**

Run: `pnpm test -- --testPathPattern="workflow" --no-coverage`
Expected: All workflow tests pass (workflow-config-panel.test.tsx, workflow-store.test.ts, new tests)

- [ ] **Step 3: Run full type check**

Run: `pnpm exec tsc --noEmit --pretty`
Expected: No errors

- [ ] **Step 4: Start dev server and manually verify**

Run: `pnpm dev`
- Navigate to Workflow tab
- Create a new workflow → editor should open
- Drag nodes from palette (all 14 types available)
- Connect nodes with edges
- Click a node → right panel should slide in with config accordion
- Click an edge → right panel should show edge condition editor
- Ctrl+Z / Ctrl+Shift+Z should undo/redo
- Save should persist
- Execute should trigger

- [ ] **Step 5: Delete old files**

```bash
git rm components/workflow/workflow-canvas.tsx
git rm components/workflow/workflow-node-types.tsx
git rm components/workflow/workflow-toolbar.tsx
```

**Do NOT delete** `components/workflow/workflow-config-panel.tsx` or `components/workflow/workflow-execution-view.tsx` — they remain in `components/workflow/` per the spec (Section 8, item 6).

Verify no other files import from these paths:
```bash
grep -r "workflow-canvas\|workflow-node-types\|workflow-toolbar" --include="*.tsx" --include="*.ts" app/ components/ lib/ | grep -v node_modules | grep -v ".test."
```

- [ ] **Step 6: Run all tests one final time**

Run: `pnpm test --no-coverage`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(workflow-editor): integrate new editor module, remove old canvas files"
```
