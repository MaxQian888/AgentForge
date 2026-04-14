"use client";

import { useEffect, useCallback, useState } from "react";
import {
  ReactFlowProvider,
  useReactFlow,
  MarkerType,
  type Node,
  type Edge,
} from "@xyflow/react";
import type { WorkflowDefinition } from "@/lib/stores/workflow-store";
import type { WorkflowNodeData, WorkflowEdgeData } from "./types";
import { EditorProvider, useEditor } from "./context";
import { useEditorActions } from "./hooks/use-editor-actions";
import { EditorToolbar } from "./toolbar/editor-toolbar";
import { EditorCanvas } from "./canvas/editor-canvas";
import { NodeConfigPanel } from "./config-panel/node-config-panel";
import { EdgeConfigPanel } from "./config-panel/edge-config-panel";

// ── Conversion helpers ────────────────────────────────────────────────────────

function toReactFlowNodes(wfNodes: WorkflowNodeData[]): Node[] {
  return (wfNodes ?? []).map((n) => ({
    id: n.id,
    type: n.type,
    position: n.position,
    data: { label: n.label, config: n.config },
  }));
}

function toReactFlowEdges(wfEdges: WorkflowEdgeData[]): Edge[] {
  return (wfEdges ?? []).map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    label: e.label || e.condition || undefined,
    data: { condition: e.condition },
    animated: false,
    markerEnd: { type: MarkerType.ArrowClosed },
    style: { strokeWidth: 2 },
  }));
}

// ── Public API types ──────────────────────────────────────────────────────────

export interface WorkflowEditorProps {
  definition: WorkflowDefinition;
  onSave: (data: {
    name: string;
    description: string;
    nodes: WorkflowNodeData[];
    edges: WorkflowEdgeData[];
  }) => Promise<boolean>;
  onExecute: (id: string) => void;
  onDirtyChange?: (dirty: boolean) => void;
}

// ── Inner component (uses hooks that require the providers) ───────────────────

function WorkflowEditorInner({
  definition,
  onSave,
  onExecute,
  onDirtyChange,
}: WorkflowEditorProps) {
  const { state, dispatch } = useEditor();
  const reactFlow = useReactFlow();
  const [saving, setSaving] = useState(false);

  // Load definition on mount
  useEffect(() => {
    dispatch({
      type: "LOAD",
      name: definition.name,
      description: definition.description,
      nodes: toReactFlowNodes(definition.nodes),
      edges: toReactFlowEdges(definition.edges),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Notify parent when dirty state changes
  useEffect(() => {
    onDirtyChange?.(state.dirty);
  }, [state.dirty, onDirtyChange]);

  const wrappedSave = useCallback(
    async (data: {
      name: string;
      description: string;
      nodes: WorkflowNodeData[];
      edges: WorkflowEdgeData[];
    }) => {
      setSaving(true);
      try {
        return await onSave(data);
      } finally {
        setSaving(false);
      }
    },
    [onSave]
  );

  const { handleSave, handleAddNode } = useEditorActions({
    onSave: wrappedSave,
    onExecute,
    definitionId: definition.id,
    status: definition.status,
  });

  const handleExecute = useCallback(() => {
    onExecute(definition.id);
  }, [onExecute, definition.id]);

  // Drop handler — converts screen coords to flow coords and adds a node
  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const type = e.dataTransfer.getData("application/workflow-node-type");
      if (!type) return;

      const position = reactFlow.screenToFlowPosition({
        x: e.clientX,
        y: e.clientY,
      });

      const id = `node_${Date.now()}`;
      const node: Node = {
        id,
        type,
        position,
        data: { label: type, config: {} },
      };
      dispatch({ type: "ADD_NODE", node });
    },
    [reactFlow, dispatch]
  );

  const onDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  }, []);

  const hasRightPanel = !!(state.selectedNodeId || state.selectedEdgeId);

  return (
    <div className="flex flex-col h-full">
      {/* Toolbar spans full width */}
      <EditorToolbar
        status={definition.status}
        saving={saving}
        onExecute={handleExecute}
        onSave={handleSave}
        onAddNode={handleAddNode}
      />

      {/* Main area: canvas + optional right panel */}
      <div className="flex flex-1 overflow-hidden">
        {/* Canvas */}
        <div className="flex-1 overflow-hidden">
          <EditorCanvas onDrop={onDrop} onDragOver={onDragOver} />
        </div>

        {/* Right panel — slides in when a node or edge is selected */}
        <div
          className={[
            "w-80 border-l bg-background overflow-hidden transition-all duration-200",
            hasRightPanel
              ? "translate-x-0 opacity-100"
              : "translate-x-full opacity-0 w-0",
          ].join(" ")}
        >
          {state.selectedNodeId && <NodeConfigPanel />}
          {state.selectedEdgeId && <EdgeConfigPanel />}
        </div>
      </div>
    </div>
  );
}

// ── Shell (wraps providers) ───────────────────────────────────────────────────

export function WorkflowEditor(props: WorkflowEditorProps) {
  return (
    <ReactFlowProvider>
      <EditorProvider>
        <WorkflowEditorInner {...props} />
      </EditorProvider>
    </ReactFlowProvider>
  );
}
