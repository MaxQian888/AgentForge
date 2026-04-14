"use client";

import React, {
  createContext,
  useContext,
  useReducer,
  type Dispatch,
} from "react";
import type { Node, Edge } from "@xyflow/react";
import type { EditorState, EditorAction, Snapshot } from "./types";

// ── Constants ─────────────────────────────────────────────────────────────────

const MAX_UNDO_STACK = 50;

// ── Initial state ─────────────────────────────────────────────────────────────

export const initialState: EditorState = {
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

// ── Helpers ───────────────────────────────────────────────────────────────────

/**
 * Creates a shallow snapshot of the current nodes/edges and prepends it to the
 * existing undo stack, capped at MAX_UNDO_STACK entries.
 */
export function pushUndo(state: EditorState): Snapshot[] {
  const snapshot: Snapshot = {
    nodes: [...state.nodes],
    edges: [...state.edges],
  };
  const next = [snapshot, ...state.undoStack];
  return next.length > MAX_UNDO_STACK ? next.slice(0, MAX_UNDO_STACK) : next;
}

// ── Reducer ───────────────────────────────────────────────────────────────────

export function editorReducer(
  state: EditorState,
  action: EditorAction
): EditorState {
  switch (action.type) {
    // ── Metadata mutations ──────────────────────────────────────────────────

    case "LOAD":
      return {
        ...state,
        name: action.name,
        description: action.description,
        nodes: action.nodes,
        edges: action.edges,
        undoStack: [],
        redoStack: [],
        dirty: false,
        selectedNodeId: null,
        selectedEdgeId: null,
        clipboard: [],
      };

    case "UPDATE_NAME":
      return { ...state, name: action.name, dirty: true };

    case "UPDATE_DESCRIPTION":
      return { ...state, description: action.description, dirty: true };

    // ── Node mutations ──────────────────────────────────────────────────────

    case "ADD_NODE":
      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        nodes: [...state.nodes, action.node],
        dirty: true,
      };

    case "DELETE_NODES": {
      const idSet = new Set(action.nodeIds);
      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        nodes: state.nodes.filter((n) => !idSet.has(n.id)),
        edges: state.edges.filter(
          (e) => !idSet.has(e.source) && !idSet.has(e.target)
        ),
        dirty: true,
      };
    }

    case "UPDATE_NODE_CONFIG": {
      const nodes = state.nodes.map((n) => {
        if (n.id !== action.nodeId) return n;
        return {
          ...n,
          data: {
            ...n.data,
            config: {
              ...(n.data.config as Record<string, unknown> | undefined),
              ...action.config,
            },
          },
        };
      });
      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        nodes,
        dirty: true,
      };
    }

    case "UPDATE_NODE_LABEL": {
      const nodes = state.nodes.map((n) => {
        if (n.id !== action.nodeId) return n;
        return { ...n, data: { ...n.data, label: action.label } };
      });
      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        nodes,
        dirty: true,
      };
    }

    // ── Edge mutations ──────────────────────────────────────────────────────

    case "ADD_EDGE":
      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        edges: [...state.edges, action.edge],
        dirty: true,
      };

    case "DELETE_EDGE":
      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        edges: state.edges.filter((e) => e.id !== action.edgeId),
        dirty: true,
      };

    case "UPDATE_EDGE_CONDITION": {
      const edges = state.edges.map((e) => {
        if (e.id !== action.edgeId) return e;
        return {
          ...e,
          label: action.label,
          data: {
            ...(e.data as Record<string, unknown> | undefined),
            condition: action.condition,
          },
        };
      });
      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        edges,
        dirty: true,
      };
    }

    // ── Selection ───────────────────────────────────────────────────────────

    case "SELECT_NODE":
      return {
        ...state,
        selectedNodeId: action.nodeId,
        selectedEdgeId: null,
      };

    case "SELECT_EDGE":
      return {
        ...state,
        selectedEdgeId: action.edgeId,
        selectedNodeId: null,
      };

    case "DESELECT":
      return { ...state, selectedNodeId: null, selectedEdgeId: null };

    // ── Undo / Redo ─────────────────────────────────────────────────────────

    case "UNDO": {
      if (state.undoStack.length === 0) return state;
      const [snapshot, ...remainingUndo] = state.undoStack;
      const currentSnapshot: Snapshot = {
        nodes: [...state.nodes],
        edges: [...state.edges],
      };
      return {
        ...state,
        nodes: snapshot.nodes,
        edges: snapshot.edges,
        undoStack: remainingUndo,
        redoStack: [currentSnapshot, ...state.redoStack],
      };
    }

    case "REDO": {
      if (state.redoStack.length === 0) return state;
      const [snapshot, ...remainingRedo] = state.redoStack;
      const currentSnapshot: Snapshot = {
        nodes: [...state.nodes],
        edges: [...state.edges],
      };
      return {
        ...state,
        nodes: snapshot.nodes,
        edges: snapshot.edges,
        undoStack: [currentSnapshot, ...state.undoStack],
        redoStack: remainingRedo,
      };
    }

    // ── Clipboard ───────────────────────────────────────────────────────────

    case "COPY":
      return { ...state, clipboard: action.nodes };

    case "PASTE": {
      if (state.clipboard.length === 0) return state;

      // Build old-ID → new-ID map for remapping internal edges
      const idMap = new Map<string, string>();
      let counter = 0;
      const pastedNodes: Node[] = state.clipboard.map((n) => {
        const newId = `paste_${Date.now()}_${counter++}`;
        idMap.set(n.id, newId);
        return {
          ...n,
          id: newId,
          position: {
            x: n.position.x + 50,
            y: n.position.y + 50,
          },
          data: { ...n.data },
        };
      });

      // Preserve edges whose both endpoints are in the clipboard
      const clipboardIds = new Set(state.clipboard.map((n) => n.id));
      const pastedEdges: Edge[] = state.edges
        .filter(
          (e) => clipboardIds.has(e.source) && clipboardIds.has(e.target)
        )
        .map((e) => ({
          ...e,
          id: `paste_edge_${Date.now()}_${e.id}`,
          source: idMap.get(e.source) ?? e.source,
          target: idMap.get(e.target) ?? e.target,
        }));

      return {
        ...state,
        undoStack: pushUndo(state),
        redoStack: [],
        nodes: [...state.nodes, ...pastedNodes],
        edges: [...state.edges, ...pastedEdges],
        dirty: true,
      };
    }

    // ── Misc ────────────────────────────────────────────────────────────────

    case "MARK_CLEAN":
      return { ...state, dirty: false };

    case "SYNC_REACTFLOW":
      // Update nodes/edges directly — no undo push, no dirty flag
      return { ...state, nodes: action.nodes, edges: action.edges };

    default:
      return state;
  }
}

// ── Context + Provider ────────────────────────────────────────────────────────

interface EditorContextValue {
  state: EditorState;
  dispatch: Dispatch<EditorAction>;
}

const EditorContext = createContext<EditorContextValue | null>(null);

export function EditorProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(editorReducer, initialState);
  return (
    <EditorContext.Provider value={{ state, dispatch }}>
      {children}
    </EditorContext.Provider>
  );
}

export function useEditor(): EditorContextValue {
  const ctx = useContext(EditorContext);
  if (!ctx) {
    throw new Error("useEditor must be used inside <EditorProvider>");
  }
  return ctx;
}
