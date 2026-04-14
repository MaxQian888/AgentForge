import type { Node, Edge } from "@xyflow/react";
import type { LucideIcon } from "lucide-react";

// ── Core data shapes ──────────────────────────────────────────────────────────

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

// ── Snapshot (undo/redo) ──────────────────────────────────────────────────────

export type Snapshot = {
  nodes: Node[];
  edges: Edge[];
};

// ── Editor actions (flat shape — no payload wrapper) ──────────────────────────

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
  | { type: "UPDATE_EDGE_CONDITION"; edgeId: string; condition: string; label?: string }
  | { type: "SELECT_NODE"; nodeId: string }
  | { type: "SELECT_EDGE"; edgeId: string }
  | { type: "DESELECT" }
  | { type: "UNDO" }
  | { type: "REDO" }
  | { type: "COPY"; nodes: Node[] }
  | { type: "PASTE" }
  | { type: "MARK_CLEAN" }
  | { type: "SYNC_REACTFLOW"; nodes: Node[]; edges: Edge[] };

// ── Editor state ──────────────────────────────────────────────────────────────

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

// ── Node categories ───────────────────────────────────────────────────────────

export type NodeCategory = "entry" | "logic" | "agent" | "flow" | "human" | "action";

// ── Config field schema ───────────────────────────────────────────────────────

export interface ConfigField {
  key: string;
  label: string;
  type: "text" | "textarea" | "select" | "number" | "boolean" | "expression" | "json";
  options?: string[];
  placeholder?: string;
  required?: boolean;
  group: string;
}

// ── Node type metadata ────────────────────────────────────────────────────────

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
