"use client";

import { useCallback, useRef, useState } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Edge,
  type Node,
  type ReactFlowInstance,
  BackgroundVariant,
  MarkerType,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { toast } from "sonner";
import { workflowNodeTypes, NODE_TYPE_LABELS } from "./workflow-node-types";
import { WorkflowToolbar } from "./workflow-toolbar";
import {
  useWorkflowStore,
  type WorkflowNodeData,
  type WorkflowEdgeData,
  type WorkflowDefinition,
} from "@/lib/stores/workflow-store";

interface WorkflowCanvasProps {
  definition: WorkflowDefinition;
  onExecute: (id: string) => void;
}

let nodeIdCounter = 0;
function getNextNodeId() {
  return `node_${Date.now()}_${++nodeIdCounter}`;
}

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
    animated: false,
    markerEnd: { type: MarkerType.ArrowClosed },
    style: { strokeWidth: 2 },
  }));
}

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
    condition: undefined,
  }));
}

export function WorkflowCanvas({
  definition,
  onExecute,
}: WorkflowCanvasProps) {
  const reactFlowWrapper = useRef<HTMLDivElement>(null);
  const [rfInstance, setRfInstance] = useState<ReactFlowInstance | null>(null);
  const [name, setName] = useState(definition.name);
  const [description, setDescription] = useState(definition.description);

  const [nodes, setNodes, onNodesChange] = useNodesState(
    toReactFlowNodes(definition.nodes)
  );
  const [edges, setEdges, onEdgesChange] = useEdgesState(
    toReactFlowEdges(definition.edges)
  );

  const { updateDefinition, saving } = useWorkflowStore();

  const onConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds) =>
        addEdge(
          {
            ...connection,
            id: `edge_${Date.now()}`,
            markerEnd: { type: MarkerType.ArrowClosed },
            style: { strokeWidth: 2 },
          },
          eds
        )
      );
    },
    [setEdges]
  );

  const handleAddNode = useCallback(
    (type: string) => {
      const nodeLabel =
        NODE_TYPE_LABELS[type] ?? type.replace(/_/g, " ");
      const newNode: Node = {
        id: getNextNodeId(),
        type,
        position: {
          x: 250 + Math.random() * 100,
          y: 150 + nodes.length * 80,
        },
        data: { label: nodeLabel },
      };
      setNodes((nds) => [...nds, newNode]);
    },
    [nodes.length, setNodes]
  );

  const handleDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      const type = event.dataTransfer.getData(
        "application/workflow-node-type"
      );
      if (!type || !rfInstance || !reactFlowWrapper.current) return;

      const bounds = reactFlowWrapper.current.getBoundingClientRect();
      const position = rfInstance.screenToFlowPosition({
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });

      const nodeLabel =
        NODE_TYPE_LABELS[type] ?? type.replace(/_/g, " ");
      const newNode: Node = {
        id: getNextNodeId(),
        type,
        position,
        data: { label: nodeLabel },
      };
      setNodes((nds) => [...nds, newNode]);
    },
    [rfInstance, setNodes]
  );

  const handleDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }, []);

  const handleSave = useCallback(async () => {
    const success = await updateDefinition(definition.id, {
      name,
      description,
      nodes: fromReactFlowNodes(nodes),
      edges: fromReactFlowEdges(edges),
    });
    if (success) {
      toast.success("Workflow saved");
    } else {
      toast.error("Failed to save workflow");
    }
  }, [definition.id, name, description, nodes, edges, updateDefinition]);

  const handleExecute = useCallback(() => {
    onExecute(definition.id);
  }, [definition.id, onExecute]);

  return (
    <div className="flex flex-col h-full">
      <WorkflowToolbar
        name={name}
        description={description}
        status={definition.status}
        saving={saving}
        onNameChange={setName}
        onDescriptionChange={setDescription}
        onSave={handleSave}
        onExecute={handleExecute}
        onAddNode={handleAddNode}
      />
      <div
        ref={reactFlowWrapper}
        className="flex-1 min-h-[500px]"
        onDrop={handleDrop}
        onDragOver={handleDragOver}
      >
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          onInit={setRfInstance}
          nodeTypes={workflowNodeTypes}
          fitView
          deleteKeyCode={["Backspace", "Delete"]}
          className="bg-muted/30"
        >
          <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
          <Controls />
          <MiniMap
            nodeColor={(node) => {
              const colors: Record<string, string> = {
                trigger: "#22c55e",
                condition: "#f59e0b",
                agent_dispatch: "#3b82f6",
                notification: "#eab308",
                status_transition: "#a855f7",
                gate: "#ef4444",
                parallel_split: "#f97316",
                parallel_join: "#f97316",
              };
              return colors[node.type ?? ""] ?? "#6b7280";
            }}
          />
        </ReactFlow>
      </div>
    </div>
  );
}
