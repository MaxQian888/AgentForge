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

// ── Editor actions ────────────────────────────────────────────────────────────

export type EditorAction =
  | { type: "LOAD"; payload: { nodes: Node[]; edges: Edge[]; name?: string; description?: string } }
  | { type: "UPDATE_NAME"; payload: { name: string } }
  | { type: "UPDATE_DESCRIPTION"; payload: { description: string } }
  | { type: "ADD_NODE"; payload: { node: Node } }
  | { type: "DELETE_NODES"; payload: { ids: string[] } }
  | { type: "UPDATE_NODE_CONFIG"; payload: { id: string; config: Record<string, unknown> } }
  | { type: "UPDATE_NODE_LABEL"; payload: { id: string; label: string } }
  | { type: "ADD_EDGE"; payload: { edge: Edge } }
  | { type: "DELETE_EDGE"; payload: { id: string } }
  | { type: "UPDATE_EDGE_CONDITION"; payload: { id: string; condition: string } }
  | { type: "SELECT_NODE"; payload: { id: string } }
  | { type: "SELECT_EDGE"; payload: { id: string } }
  | { type: "DESELECT" }
  | { type: "UNDO" }
  | { type: "REDO" }
  | { type: "COPY"; payload: { nodes: Node[] } }
  | { type: "PASTE"; payload: { offset?: { x: number; y: number } } }
  | { type: "MARK_CLEAN" }
  | { type: "SYNC_REACTFLOW"; payload: { nodes: Node[]; edges: Edge[] } };

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
