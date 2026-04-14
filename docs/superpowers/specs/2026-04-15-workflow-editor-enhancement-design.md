# Workflow Editor Enhancement Design

**Date:** 2026-04-15
**Status:** Draft
**Scope:** Frontend workflow editor — independent module extraction, node config panels, edge condition editor, data flow preview, canvas interaction improvements

## Context

AgentForge's backend workflow engine is production-grade: 12+ node types, DAG execution with data flow, template system, human review, external events, parallel execution, loops, and an expression engine. The frontend editor covers ~70-75% of backend capabilities. Key gaps: no node configuration UI, incomplete node palette (5 of 13 types missing from toolbar), no edge condition editing, no data flow preview, and limited canvas interactions (no undo/redo, copy/paste, or alignment aids).

This design extracts the workflow editor into a self-contained module and fills all feature gaps.

## Decision Record

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Architecture | Independent module (`components/workflow-editor/`) | High cohesion, low coupling; reusable; clean public API |
| Config panel style | Right-side drawer with scrollable accordion | Better mobile adaptability; no tab-switching needed |
| Edge condition editor | Hybrid mode (visual builder + expression input) | Balances low barrier to entry with full expression flexibility |
| Canvas library | Continue with ReactFlow (`@xyflow/react`) | Mature ecosystem, strong custom node/edge support, already integrated |
| All 5 enhancement items | Must-have | Complete palette, node config, edge config, data flow preview, canvas interactions |

## 1. Module Structure

Extract the workflow editor from `components/workflow/` into `components/workflow-editor/` as a self-contained module with a single public API surface.

```
components/workflow-editor/
├── index.ts                      # Public API: <WorkflowEditor> + types
├── workflow-editor.tsx            # Shell: orchestrates canvas + toolbar + config panel
├── context.tsx                    # EditorContext: nodes/edges state, selection, undo/redo
├── types.ts                      # Internal types (aligned with workflow-store but decoupled)
├── hooks/
│   ├── use-editor-actions.ts     # save, execute, add-node, delete, copy/paste
│   ├── use-undo-redo.ts          # Undo/redo history stack
│   └── use-data-flow.ts          # Parse DAG upstream node output fields
├── canvas/
│   ├── editor-canvas.tsx         # ReactFlow canvas (migrated from workflow-canvas.tsx)
│   ├── custom-edge.tsx           # Custom edge component with click-to-edit condition
│   └── snap-grid.ts             # Alignment guide/snap logic
├── nodes/
│   ├── node-types.tsx            # 13 node type definitions (migrated from workflow-node-types.tsx)
│   ├── node-registry.ts          # Node metadata registry (icon, color, config schema, category)
│   └── node-styles.ts            # Node style constants extracted
├── toolbar/
│   ├── editor-toolbar.tsx        # Toolbar (migrated from workflow-toolbar.tsx)
│   └── node-palette.tsx          # Complete 13-node categorized drag palette
├── config-panel/
│   ├── node-config-panel.tsx     # Right drawer accordion panel
│   ├── edge-config-panel.tsx     # Edge condition editing panel
│   ├── data-flow-preview.tsx     # Upstream node data flow preview
│   ├── condition-builder.tsx     # Hybrid mode condition builder (shared)
│   └── node-configs/             # Per-node-type custom config forms (overrides)
│       ├── trigger-config.tsx
│       ├── llm-agent-config.tsx
│       ├── condition-config.tsx
│       ├── function-config.tsx
│       ├── loop-config.tsx
│       ├── human-review-config.tsx
│       ├── wait-event-config.tsx
│       ├── notification-config.tsx
│       ├── status-transition-config.tsx
│       ├── gate-config.tsx
│       ├── parallel-split-config.tsx
│       └── parallel-join-config.tsx
```

### Public API

```tsx
// components/workflow-editor/index.ts
export { WorkflowEditor } from "./workflow-editor";
export type { WorkflowEditorProps } from "./workflow-editor";
```

```tsx
interface WorkflowEditorProps {
  definition: WorkflowDefinition;
  onSave: (data: { name: string; description: string; nodes: WorkflowNodeData[]; edges: WorkflowEdgeData[] }) => Promise<boolean>;
  onExecute: (id: string) => void;
}
```

The page (`app/(dashboard)/workflow/page.tsx`) renders `<WorkflowEditor>` and wires `onSave` to `useWorkflowStore.updateDefinition()`, `onExecute` to `useWorkflowStore.startExecution()`. All internal state lives inside the module.

### Migration Path

Old files in `components/workflow/` (workflow-canvas.tsx, workflow-node-types.tsx, workflow-toolbar.tsx) are kept temporarily. The workflow page switches its import to the new module. Once verified, old files are deleted.

## 2. EditorContext and State Management

Editor-internal state is managed via React Context + `useReducer`, fully decoupled from the global Zustand store. Zustand handles only API communication.

### State Shape

```ts
interface EditorState {
  nodes: Node[];           // ReactFlow Node[]
  edges: Edge[];           // ReactFlow Edge[]
  selectedNodeId: string | null;
  selectedEdgeId: string | null;
  undoStack: Snapshot[];   // { nodes, edges }
  redoStack: Snapshot[];
  dirty: boolean;
  clipboard: Node[];
}
```

### Actions (dispatch)

| Action | Effect |
|--------|--------|
| `LOAD` | Initialize from WorkflowDefinition; clear undo/redo |
| `ADD_NODE` | Push snapshot → add node → mark dirty |
| `DELETE_NODES` | Push snapshot → remove nodes + connected edges → mark dirty |
| `UPDATE_NODE_CONFIG` | Push snapshot → merge config into node.data.config → mark dirty |
| `UPDATE_NODE_LABEL` | Push snapshot → update node.data.label → mark dirty |
| `ADD_EDGE` | Push snapshot → add edge → mark dirty |
| `DELETE_EDGE` | Push snapshot → remove edge → mark dirty |
| `UPDATE_EDGE_CONDITION` | Push snapshot → set edge condition + label → mark dirty |
| `SELECT_NODE` | Set selectedNodeId, clear selectedEdgeId |
| `SELECT_EDGE` | Set selectedEdgeId, clear selectedNodeId |
| `DESELECT` | Clear both selection IDs |
| `UNDO` | Pop undoStack → push current to redoStack → restore |
| `REDO` | Pop redoStack → push current to undoStack → restore |
| `COPY` | Copy selected nodes to clipboard |
| `PASTE` | Push snapshot → add clipboard nodes with new IDs + offset (50,50) → mark dirty |
| `MARK_CLEAN` | Set dirty = false (after successful save) |

### Undo/Redo Strategy

- Every mutation action (ADD_NODE, DELETE_NODES, UPDATE_NODE_CONFIG, ADD_EDGE, etc.) pushes a `{nodes, edges}` snapshot onto undoStack before applying the change.
- UNDO pops undoStack top, pushes current state to redoStack, restores popped snapshot.
- Any new mutation clears redoStack.
- Stack depth capped at 50.

### Interaction with Zustand Store

```
EditorContext (internal state)
      │
      │ onSave callback
      │ converts nodes/edges → WorkflowNodeData[]/WorkflowEdgeData[]
      ▼
Zustand workflow-store.updateDefinition() → PUT /api/v1/workflows/:id
```

The editor module never imports or calls Zustand directly. The parent page provides callbacks.

## 3. Node Palette

### Node Categories

| Category | Nodes | Description |
|----------|-------|-------------|
| **Entry** | trigger | Workflow entry point |
| **Logic** | condition, gate, function | Control flow and computation |
| **Agent** | agent_dispatch, llm_agent | AI task execution |
| **Flow Control** | parallel_split, parallel_join, loop | Parallelism and iteration |
| **Human** | human_review, wait_event | Human-in-the-loop and external signals |
| **Action** | notification, status_transition | Side-effect actions |

### Node Registry (`node-registry.ts`)

Single source of truth for all node metadata. Consumed by palette, canvas node rendering, and config panel.

```ts
interface NodeTypeMeta {
  type: string;
  label: string;
  category: "entry" | "logic" | "agent" | "flow" | "human" | "action";
  icon: LucideIcon;
  color: string;                    // Tailwind color token, e.g. "green"
  description: string;              // Tooltip text
  configSchema: ConfigField[];      // Drives config panel form generation
  defaultConfig: Record<string, unknown>;
}

interface ConfigField {
  key: string;
  label: string;
  type: "text" | "textarea" | "select" | "number" | "boolean" | "expression" | "json";
  options?: { value: string; label: string }[];
  placeholder?: string;
  required?: boolean;
  group: string;  // Accordion group name: "General", "Agent Config", "Data Flow", etc.
}
```

### Palette UI (`node-palette.tsx`)

- Nodes displayed in categorized collapsible groups with section headers.
- Both click-to-add and drag-to-canvas supported.
- Search filter input at top for quick lookup when many nodes are visible.
- Each node shows icon + label + tooltip with description.

## 4. Node Config Panel

### Layout

Right-side drawer panel, slides in when a node is selected on canvas. Closes on Escape or clicking canvas blank area.

Uses scrollable accordion pattern (not tabs). All config groups are collapsible sections in a single scrollable column. Multiple groups can be open simultaneously.

### Panel Structure

```
┌──────────────────────────────┐
│ [icon] Node Label (editable) ✕│  Header: node type icon, inline-editable label, close button
├──────────────────────────────┤
│ ▼ General                     │  Always present: label, description
├──────────────────────────────┤
│ ▼ [Type-specific config]      │  Driven by configSchema or custom override
├──────────────────────────────┤
│ ▶ Data Flow                   │  Shared: upstream node output preview
├──────────────────────────────┤
│ ▶ Advanced                    │  Raw JSON editor for config (escape hatch)
├──────────────────────────────┤
│   [Delete Node]               │  Danger zone at bottom
└──────────────────────────────┘
```

### Per-Node-Type Config Fields

| Node Type | Key Config Fields |
|-----------|-------------------|
| trigger | (label only) |
| llm_agent | runtime, provider, model, budgetUsd, prompt, systemPrompt |
| agent_dispatch | runtime, provider, model, budgetUsd |
| condition | expression (hybrid condition builder) |
| function | expression (code expression input) |
| loop | maxIterations, exitCondition (expression) |
| human_review | prompt (review prompt text), reviewerHint |
| wait_event | eventType, timeout |
| notification | message (supports `{{}}` template vars), channel |
| status_transition | targetStatus (dropdown from ALL_TASK_STATUSES) |
| gate | expression (pass condition) |
| parallel_split | (label only) |
| parallel_join | (label only) |

### Schema-Driven vs Custom Override

Most nodes render forms from `configSchema` automatically. Two nodes get custom override components:

- **`llm-agent-config.tsx`**: runtime → provider → model cascade dropdowns (selecting runtime changes provider list, selecting provider changes model list).
- **`condition-config.tsx`**: Embeds the shared hybrid condition builder component.

All other nodes use the generic schema-driven renderer.

## 5. Edge Condition Editor

### Custom Edge Component (`custom-edge.tsx`)

Replaces ReactFlow default edges:
- Clickable hot zone along the edge path.
- Displays label text on the edge (condition summary if present, e.g., `urgency > 0.7`).
- Selected state: blue highlight + thicker stroke + label background highlight.
- Edges without conditions render as plain arrows with no extra annotation.

### Edge Config Panel (`edge-config-panel.tsx`)

Appears in the right drawer when an edge is selected (replaces node config panel).

```
┌──────────────────────────────┐
│ Edge: Source → Target      ✕  │  Header: source label → target label
├──────────────────────────────┤
│ ▼ Label                       │
│   [Optional display label  ]  │
├──────────────────────────────┤
│ ▼ Condition                   │
│   ○ Visual   ● Expression     │  Hybrid mode toggle
│   [condition builder content] │
├──────────────────────────────┤
│   [Delete Edge]               │
└──────────────────────────────┘
```

### Hybrid Condition Builder (`condition-builder.tsx`)

Shared component used by edge conditions, condition nodes, gate nodes, and loop exit conditions.

**Visual Mode:**
- Node dropdown: lists all upstream predecessor nodes (derived from DAG traversal).
- Field dropdown: lists known output fields for the selected upstream node.
- Operator dropdown: `==`, `!=`, `>`, `<`, `>=`, `<=`, `contains`.
- Value input: free text, auto-infers type (number/boolean/string).
- Live expression preview below: e.g., `{{planner.output.subtasks}} > 0`.

**Expression Mode:**
- Text input (single or multi-line).
- Autocomplete triggered by `{{`: lists upstream nodes and their fields.
- Syntax highlighting for `{{...}}` template variable patterns.
- Supports all backend expression syntax: comparison operators, `len()` function, path lookups.

**Mode Switching:**
- Visual → Expression: serializes visual rule into expression string.
- Expression → Visual: parses if expression matches `{{node.field}} op value` pattern; blocks switch with explanation if expression is too complex for visual representation.

## 6. Data Flow Preview

### Component (`data-flow-preview.tsx`)

Embedded in the "Data Flow" accordion group of the node config panel. Available for all node types.

**Behavior:**
- Traverses edges backward from the current node to find all predecessor nodes.
- Sorted by topological distance (direct predecessors first).
- Each upstream node renders as an expandable card showing:
  - Node icon, label, and type badge.
  - Known output field paths with copy-to-clipboard buttons (`{{node_id.output.field}}`).
- Bottom hint: "Type `{{node.output.field}}` in any config to reference upstream data."

**Field Discovery Logic:**
1. If the upstream node has a known output structure from its `configSchema` metadata, show those fields.
2. If the workflow has been executed before, parse actual output keys from the DataStore (requires fetching execution data).
3. Fallback: show generic `output.*` placeholder.

## 7. Canvas Interaction Enhancements

### Undo/Redo
- Keyboard: `Ctrl+Z` (undo), `Ctrl+Shift+Z` (redo). Mac: `Cmd` equivalents.
- Toolbar buttons with disabled state when stack is empty.
- Managed by `use-undo-redo.ts` hook wrapping EditorContext dispatch.

### Copy/Paste
- `Ctrl+C`: copies selected nodes (single or multi-select) to clipboard within EditorContext.
- `Ctrl+V`: pastes with new IDs and position offset (+50, +50). Internal edges between pasted nodes are preserved; edges to external nodes are dropped.

### Multi-Select
- Enable ReactFlow's `selectionOnDrag` prop.
- Box-select multiple nodes, then bulk delete (Delete key) or bulk drag.

### Alignment Guides (`snap-grid.ts`)
- On node drag, calculate proximity to other nodes' X/Y coordinates.
- Display dashed guide lines when within 8px snap threshold.
- Implemented via ReactFlow's `onNodeDrag` callback computing nearest alignment candidates.

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+Z` | Undo |
| `Ctrl+Shift+Z` | Redo |
| `Ctrl+C` | Copy selected nodes |
| `Ctrl+V` | Paste nodes |
| `Delete` / `Backspace` | Delete selected nodes/edges |
| `Escape` | Deselect all, close config panel |
| `Ctrl+S` | Save workflow |
| `Ctrl+A` | Select all nodes |

## 8. Migration Strategy

1. Create `components/workflow-editor/` with all new files.
2. Migrate logic from `workflow-canvas.tsx`, `workflow-node-types.tsx`, `workflow-toolbar.tsx` into the new module structure.
3. Update `app/(dashboard)/workflow/page.tsx` to import `<WorkflowEditor>` from the new module.
4. Verify all existing functionality works (create, edit, save, execute workflows).
5. Delete old `components/workflow/workflow-canvas.tsx`, `workflow-node-types.tsx`, `workflow-toolbar.tsx`.
6. Keep `workflow-config-panel.tsx` and `workflow-execution-view.tsx` in `components/workflow/` (out of scope for this design).

## 9. Out of Scope

The following are explicitly deferred:
- Template gallery UI and template management
- Workflow versioning UI
- Import/export workflows
- Workflow search/filter
- Human review approval UI (execution-side, not editor)
- External event trigger UI (execution-side)
- Execution view enhancements
