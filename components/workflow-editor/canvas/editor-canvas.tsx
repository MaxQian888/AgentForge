"use client";

import "@xyflow/react/dist/style.css";

import { useCallback, useRef } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  MarkerType,
  BackgroundVariant,
  applyNodeChanges,
  applyEdgeChanges,
  type NodeMouseHandler,
  type EdgeMouseHandler,
  type OnConnect,
  type OnNodesChange,
  type OnEdgesChange,
} from "@xyflow/react";
import { useEditor } from "../context";
import { workflowNodeTypes } from "../nodes/node-types";
import { MINIMAP_COLORS } from "../nodes/node-styles";
import { customEdgeTypes } from "./custom-edge";

interface EditorCanvasProps {
  onDrop: (event: React.DragEvent) => void;
  onDragOver: (event: React.DragEvent) => void;
}

export function EditorCanvas({ onDrop, onDragOver }: EditorCanvasProps) {
  const { state, dispatch } = useEditor();
  const wrapperRef = useRef<HTMLDivElement>(null);

  const onNodesChange = useCallback<OnNodesChange>(
    (changes) => {
      const updatedNodes = applyNodeChanges(changes, state.nodes);
      dispatch({
        type: "SYNC_REACTFLOW",
        nodes: updatedNodes,
        edges: state.edges,
      });
    },
    [state.nodes, state.edges, dispatch]
  );

  const onEdgesChange = useCallback<OnEdgesChange>(
    (changes) => {
      const updatedEdges = applyEdgeChanges(changes, state.edges);
      dispatch({
        type: "SYNC_REACTFLOW",
        nodes: state.nodes,
        edges: updatedEdges,
      });
    },
    [state.nodes, state.edges, dispatch]
  );

  const onConnect = useCallback<OnConnect>(
    (connection) => {
      dispatch({
        type: "ADD_EDGE",
        edge: {
          ...connection,
          id: `edge_${Date.now()}`,
          source: connection.source,
          target: connection.target,
          markerEnd: { type: MarkerType.ArrowClosed },
          style: { strokeWidth: 2 },
        },
      });
    },
    [dispatch]
  );

  const onNodeClick = useCallback<NodeMouseHandler>(
    (_event, node) => {
      dispatch({ type: "SELECT_NODE", nodeId: node.id });
    },
    [dispatch]
  );

  const onEdgeClick = useCallback<EdgeMouseHandler>(
    (_event, edge) => {
      dispatch({ type: "SELECT_EDGE", edgeId: edge.id });
    },
    [dispatch]
  );

  const onPaneClick = useCallback(() => {
    dispatch({ type: "DESELECT" });
  }, [dispatch]);

  return (
    <div
      ref={wrapperRef}
      className="w-full h-full"
      onDrop={onDrop}
      onDragOver={onDragOver}
    >
      <ReactFlow
        nodes={state.nodes}
        edges={state.edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onNodeClick={onNodeClick}
        onEdgeClick={onEdgeClick}
        onPaneClick={onPaneClick}
        nodeTypes={workflowNodeTypes}
        edgeTypes={customEdgeTypes}
        selectionOnDrag
        deleteKeyCode={["Backspace", "Delete"]}
        fitView
        className="bg-muted/30"
      >
        <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
        <Controls />
        <MiniMap
          nodeColor={(node) =>
            MINIMAP_COLORS[node.type ?? ""] ?? "#6b7280"
          }
        />
      </ReactFlow>
    </div>
  );
}
