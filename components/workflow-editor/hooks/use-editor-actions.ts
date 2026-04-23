"use client";

import { useEffect, useCallback, useRef } from "react";
import { useTranslations } from "next-intl";
import type { Node, Edge } from "@xyflow/react";
import { useReactFlow, MarkerType } from "@xyflow/react";
import { useEditor } from "../context";
import { getNodeMeta } from "../nodes/node-registry";
import type { WorkflowNodeData, WorkflowEdgeData } from "../types";

// ── Export payload shape ──────────────────────────────────────────────────────

/**
 * Serialized workflow — stable, minimal JSON shape suitable for export/import.
 * Mirrors the persisted WorkflowDefinition sub-shape that the backend accepts.
 */
export interface WorkflowExportPayload {
  version: 1;
  name: string;
  description: string;
  nodes: WorkflowNodeData[];
  edges: WorkflowEdgeData[];
  exportedAt: string;
}

export function isWorkflowExportPayload(
  value: unknown
): value is WorkflowExportPayload {
  if (!value || typeof value !== "object") return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.name === "string" &&
    Array.isArray(v.nodes) &&
    Array.isArray(v.edges)
  );
}

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
  const t = useTranslations("workflow");

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

  // ── Export / Import (JSON) ────────────────────────────────────────────────

  /**
   * Builds the portable JSON payload from the current editor state.
   * Exposed as a pure function so it can be unit-tested without a DOM.
   */
  const buildExportPayload = useCallback((): WorkflowExportPayload => {
    const rfNodes = reactFlow.getNodes();
    const rfEdges = reactFlow.getEdges();
    return {
      version: 1,
      name: state.name,
      description: state.description,
      nodes: fromReactFlowNodes(rfNodes),
      edges: fromReactFlowEdges(rfEdges),
      exportedAt: new Date().toISOString(),
    };
  }, [reactFlow, state.name, state.description]);

  const handleExport = useCallback(() => {
    if (typeof window === "undefined") return;
    const payload = buildExportPayload();
    const json = JSON.stringify(payload, null, 2);
    const blob = new Blob([json], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const safeName = (state.name || t("editor.exportFallbackName")).replace(/[^a-z0-9-_]+/gi, "_");
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `${safeName}.workflow.json`;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
    URL.revokeObjectURL(url);
  }, [buildExportPayload, state.name]);

  /**
   * Replaces the current canvas with the contents of `payload`. Dispatches a
   * LOAD action — the caller is responsible for prompting/confirming when the
   * editor is dirty.
   */
  const applyImportPayload = useCallback(
    (payload: WorkflowExportPayload) => {
      const rfNodes: Node[] = (payload.nodes ?? []).map((n) => ({
        id: n.id,
        type: n.type,
        position: n.position,
        data: { label: n.label, config: n.config },
      }));
      const rfEdges: Edge[] = (payload.edges ?? []).map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        label: e.label || e.condition || undefined,
        data: { condition: e.condition },
        markerEnd: { type: MarkerType.ArrowClosed },
        style: { strokeWidth: 2 },
      }));
      dispatch({
        type: "LOAD",
        name: payload.name ?? state.name,
        description: payload.description ?? state.description,
        nodes: rfNodes,
        edges: rfEdges,
      });
    },
    [dispatch, state.name, state.description]
  );

  const handleImport = useCallback(
    async (
      file: File
    ): Promise<{ ok: true } | { ok: false; error: string }> => {
      try {
        const text = await file.text();
        const parsed: unknown = JSON.parse(text);
        if (!isWorkflowExportPayload(parsed)) {
          return { ok: false, error: t("editor.importInvalid") };
        }
        applyImportPayload(parsed);
        return { ok: true };
      } catch {
        return { ok: false, error: t("editor.importParseError") };
      }
    },
    [applyImportPayload]
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
    handleExport,
    handleImport,
    buildExportPayload,
    applyImportPayload,
    definitionId,
  };
}
