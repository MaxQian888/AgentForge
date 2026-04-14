"use client";

import { useEffect, useCallback, useRef } from "react";
import type { Node, Edge } from "@xyflow/react";
import { useReactFlow } from "@xyflow/react";
import { useEditor } from "../context";
import { getNodeMeta } from "../nodes/node-registry";
import type { WorkflowNodeData, WorkflowEdgeData } from "../types";

// ── Conversion helpers ────────────────────────────────────────────────────────

function fromReactFlowNodes(rfNodes: Node[]): WorkflowNodeData[] {
  return rfNodes.map((n) => ({
    id: n.id,
    type: n.type ?? "trigger",
    label: (n.data as { label?: string })?.label ?? "",
    position: n.position,
    config: (n.data as { config?: Record<string, unknown> })?.config,
  }));
}

function fromReactFlowEdges(rfEdges: Edge[]): WorkflowEdgeData[] {
  return rfEdges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    label: typeof e.label === "string" ? e.label : undefined,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    condition: (e.data as any)?.condition,
  }));
}

// ── Hook props ────────────────────────────────────────────────────────────────

export interface UseEditorActionsProps {
  onSave: (data: {
    name: string;
    description: string;
    nodes: WorkflowNodeData[];
    edges: WorkflowEdgeData[];
  }) => Promise<boolean>;
  onExecute: (id: string) => void;
  definitionId: string;
  status: string;
}

// ── Counter for unique node IDs ───────────────────────────────────────────────

let _nodeCounter = 0;

// ── Hook ──────────────────────────────────────────────────────────────────────

export function useEditorActions(props: UseEditorActionsProps) {
  const { onSave, definitionId } = props;
  const { state, dispatch } = useEditor();
  const reactFlow = useReactFlow();

  // Keep a stable ref to onSave so the keyboard handler never goes stale
  const onSaveRef = useRef(onSave);
  useEffect(() => {
    onSaveRef.current = onSave;
  }, [onSave]);

  // ── Core action callbacks ─────────────────────────────────────────────────

  const handleSave = useCallback(async () => {
    const rfNodes = reactFlow.getNodes();
    const rfEdges = reactFlow.getEdges();
    const payload = {
      name: state.name,
      description: state.description,
      nodes: fromReactFlowNodes(rfNodes),
      edges: fromReactFlowEdges(rfEdges),
    };
    const ok = await onSaveRef.current(payload);
    if (ok) {
      dispatch({ type: "MARK_CLEAN" });
    }
  }, [reactFlow, state.name, state.description, dispatch]);

  const handleCopy = useCallback(() => {
    const selected = reactFlow.getNodes().filter((n) => n.selected);
    dispatch({ type: "COPY", nodes: selected });
  }, [reactFlow, dispatch]);

  const handlePaste = useCallback(() => {
    dispatch({ type: "PASTE" });
  }, [dispatch]);

  const handleDelete = useCallback(() => {
    const selectedIds = reactFlow
      .getNodes()
      .filter((n) => n.selected)
      .map((n) => n.id);
    if (selectedIds.length > 0) {
      dispatch({ type: "DELETE_NODES", nodeIds: selectedIds });
    }
  }, [reactFlow, dispatch]);

  const handleAddNode = useCallback(
    (type: string) => {
      const meta = getNodeMeta(type);
      const id = `node_${Date.now()}_${_nodeCounter++}`;
      const node: Node = {
        id,
        type,
        position: {
          x: 250 + Math.random() * 100,
          y: 150 + state.nodes.length * 80,
        },
        data: {
          label: meta?.label ?? type,
          config: meta?.defaultConfig ?? {},
        },
      };
      dispatch({ type: "ADD_NODE", node });
    },
    [state.nodes.length, dispatch]
  );

  // ── Keyboard shortcuts ────────────────────────────────────────────────────

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      const isMac = navigator.platform.toUpperCase().includes("MAC");
      const ctrl = isMac ? e.metaKey : e.ctrlKey;
      const target = e.target as HTMLElement;
      const inInput =
        target.tagName === "INPUT" || target.tagName === "TEXTAREA";

      if (ctrl && e.key === "s") {
        e.preventDefault();
        void handleSave();
        return;
      }

      if (ctrl && e.key === "c" && !inInput) {
        handleCopy();
        return;
      }

      if (ctrl && e.key === "v" && !inInput) {
        handlePaste();
        return;
      }

      if (ctrl && e.key === "a" && !inInput) {
        e.preventDefault();
        // Select all nodes in ReactFlow
        reactFlow.setNodes((nodes) =>
          nodes.map((n) => ({ ...n, selected: true }))
        );
        return;
      }

      if ((e.key === "Delete" || e.key === "Backspace") && !inInput) {
        handleDelete();
        return;
      }

      if (e.key === "Escape") {
        dispatch({ type: "DESELECT" });
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [handleSave, handleCopy, handlePaste, handleDelete, dispatch, reactFlow]);

  // ── Unsaved changes guard ─────────────────────────────────────────────────

  useEffect(() => {
    if (!state.dirty) return;

    function handleBeforeUnload(e: BeforeUnloadEvent) {
      e.preventDefault();
      // Modern browsers show a generic message; returnValue is legacy
      e.returnValue = "";
    }

    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => window.removeEventListener("beforeunload", handleBeforeUnload);
  }, [state.dirty]);

  // ── Public API ────────────────────────────────────────────────────────────

  return {
    handleSave,
    handleCopy,
    handlePaste,
    handleDelete,
    handleAddNode,
    definitionId,
  };
}
