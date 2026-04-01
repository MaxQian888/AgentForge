"use client";

import { Component, type ReactNode, type ErrorInfo } from "react";
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Panel,
  Position,
  ReactFlow,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useTranslations } from "next-intl";
import { ActivityIcon, AlertTriangle, Bot, Cpu, GitBranch, TimerReset } from "lucide-react";
import type {
  AgentVisualizationModel,
  AgentVisualizationNodeData,
  AgentVisualizationTone,
} from "./agent-visualization-model";

type VisualizationNodeData = AgentVisualizationNodeData & {
  [key: string]: unknown;
  nodeId: string;
  onSelectAgent?: (id: string) => void;
  onSelectVisualizationNode?: (id: string) => void;
};

type VisualizationNode = Node<VisualizationNodeData, "agentVisualization">;

interface AgentVisualizationCanvasProps {
  model: AgentVisualizationModel;
  loading: boolean;
  requestedMemberId: string | null;
  selectedAgentId: string | null;
  selectedVisualizationNodeId: string | null;
  onSelectAgent: (id: string) => void;
  onSelectVisualizationNode: (id: string) => void;
}

interface VisualizationErrorBoundaryState {
  hasError: boolean;
}

class VisualizationErrorBoundary extends Component<
  { children: ReactNode },
  VisualizationErrorBoundaryState
> {
  state: VisualizationErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[VisualizationErrorBoundary]", error, info);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex items-center justify-center h-full p-8">
          <Alert variant="destructive" className="max-w-md">
            <AlertTriangle className="h-4 w-4" />
            <AlertTitle>Visualization Error</AlertTitle>
            <AlertDescription className="mt-2">
              <p className="mb-3">The flow graph encountered an error.</p>
              <Button
                size="sm"
                variant="outline"
                onClick={() => this.setState({ hasError: false })}
              >
                Retry
              </Button>
            </AlertDescription>
          </Alert>
        </div>
      );
    }
    return this.props.children;
  }
}

const MINIMAP_NODE_COLORS: Record<AgentVisualizationTone, string> = {
  default: "hsl(var(--muted-foreground))",
  success: "hsl(142 71% 45%)",
  warning: "hsl(38 92% 50%)",
  danger: "hsl(0 84% 60%)",
  muted: "hsl(var(--muted-foreground) / 0.5)",
};

const TONE_CLASSES: Record<AgentVisualizationTone, string> = {
  default: "border-border/70 bg-card text-card-foreground",
  success: "border-emerald-500/30 bg-emerald-500/5 text-foreground",
  warning: "border-amber-500/30 bg-amber-500/5 text-foreground",
  danger: "border-destructive/35 bg-destructive/8 text-foreground",
  muted: "border-border/50 bg-muted/40 text-foreground",
};

const PROGRESS_CLASSES: Record<AgentVisualizationTone, string | undefined> = {
  default: undefined,
  success: "bg-emerald-500",
  warning: "bg-amber-500",
  danger: "bg-destructive",
  muted: "bg-muted-foreground",
};

function VisualizationNode({
  id,
  data,
  selected,
}: NodeProps<VisualizationNode>) {
  const content = (
    <div
      className={cn(
        "min-w-[220px] max-w-[220px] rounded-xl border shadow-sm transition-shadow",
        TONE_CLASSES[data.tone],
        selected && "ring-2 ring-ring/60",
      )}
    >
      <div className="flex flex-col gap-3 p-3">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold">{data.title}</p>
            {data.subtitle ? (
              <p className="truncate text-xs text-muted-foreground">
                {data.subtitle}
              </p>
            ) : null}
          </div>
          <Badge variant="outline" className="shrink-0 capitalize">
            {data.kind}
          </Badge>
        </div>
        {data.badges?.length ? (
          <div className="flex flex-wrap gap-1.5">
            {data.badges.map((badge) => (
              <Badge
                key={`${id}-${badge}`}
                variant={data.tone === "danger" ? "destructive" : "secondary"}
                className="max-w-full truncate"
              >
                {badge}
              </Badge>
            ))}
          </div>
        ) : null}
        {data.metadata?.length ? (
          <div className="space-y-1">
            {data.metadata.map((item) => (
              <p
                key={`${id}-${item}`}
                className="truncate text-xs text-muted-foreground"
              >
                {item}
              </p>
            ))}
          </div>
        ) : null}
        {typeof data.budgetPct === "number" ? (
          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
              <span>Budget</span>
              <span>{Math.round(data.budgetPct)}%</span>
            </div>
            <Progress
              value={data.budgetPct}
              className="h-1.5"
              indicatorClassName={PROGRESS_CLASSES[data.tone]}
            />
          </div>
        ) : null}
      </div>
    </div>
  );

  const isAgentNode = data.kind === "agent";
  const hasFocusHandler = typeof data.onSelectVisualizationNode === "function";
  const hasAgentHandler = typeof data.onSelectAgent === "function";
  const agentId = id.startsWith("agent:") ? id.slice("agent:".length) : id;

  return (
    <>
      <Handle type="target" position={Position.Left} className="!size-2 !bg-border" />
      {isAgentNode && (hasFocusHandler || hasAgentHandler) ? (
        <button
          type="button"
          className="cursor-pointer text-left"
          onClick={() => data.onSelectVisualizationNode?.(id)}
          onDoubleClick={() => data.onSelectAgent?.(agentId)}
        >
          {content}
        </button>
      ) : hasFocusHandler ? (
        <button
          type="button"
          className="cursor-pointer text-left"
          onClick={() => data.onSelectVisualizationNode?.(id)}
        >
          {content}
        </button>
      ) : (
        content
      )}
      <Handle type="source" position={Position.Right} className="!size-2 !bg-border" />
    </>
  );
}

function LegendItem({
  icon,
  label,
}: {
  icon: ReactNode;
  label: string;
}) {
  return (
    <div className="flex items-center gap-2 text-xs text-muted-foreground">
      <span className="inline-flex size-5 items-center justify-center rounded-md border bg-background">
        {icon}
      </span>
      <span>{label}</span>
    </div>
  );
}

export function AgentVisualizationCanvas({
  model,
  loading,
  requestedMemberId,
  selectedAgentId,
  selectedVisualizationNodeId,
  onSelectAgent,
  onSelectVisualizationNode,
}: AgentVisualizationCanvasProps) {
  const t = useTranslations("agents");

  if (loading && !model.summary.hasGraphData) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">{t("visualization.loading")}</p>
          <Skeleton className="h-[420px] w-full rounded-xl" />
        </div>
      </div>
    );
  }

  if (!model.summary.hasGraphData) {
    return (
      <div className="flex flex-col gap-4 p-6">
        <Empty className="min-h-[420px] border bg-muted/15">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Bot className="size-5" />
            </EmptyMedia>
            <EmptyTitle>
              {requestedMemberId
                ? t("visualization.empty.noMatch")
                : t("visualization.empty.noAgents")}
            </EmptyTitle>
            <EmptyDescription>
              {requestedMemberId
                ? t("visualization.empty.noMatchDescription")
                : t("visualization.empty.noAgentsDescription")}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      </div>
    );
  }

  const nodes: VisualizationNode[] = model.nodes.map((node) => ({
    ...node,
    selected:
      (node.id.startsWith("agent:") &&
        selectedAgentId != null &&
        node.id === `agent:${selectedAgentId}`) ||
      (selectedVisualizationNodeId != null &&
        node.id === selectedVisualizationNodeId),
    data: {
      ...node.data,
      nodeId: node.id,
      onSelectAgent,
      onSelectVisualizationNode,
    },
  }));

  return (
    <div className="flex flex-col gap-4 p-6">
      {model.summary.isDegraded ? (
        <Alert className="border-amber-500/40 bg-amber-500/5">
          <ActivityIcon className="size-4" />
          <AlertTitle>{t("visualization.degraded.title")}</AlertTitle>
          <AlertDescription>
            {t("visualization.degraded.description")}
          </AlertDescription>
        </Alert>
      ) : null}

      <div className="h-[min(68vh,720px)] overflow-hidden rounded-xl border bg-background">
        <VisualizationErrorBoundary>
        <ReactFlow
          nodes={nodes}
          edges={model.edges}
          fitView
          minZoom={0.5}
          maxZoom={1.5}
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable
          panOnDrag
          nodeTypes={{ agentVisualization: VisualizationNode }}
        >
          <Background />
          <MiniMap
            className="border rounded-md"
            nodeColor={(node) => {
              const tone = (node.data as AgentVisualizationNodeData | undefined)?.tone ?? "default";
              return MINIMAP_NODE_COLORS[tone] ?? MINIMAP_NODE_COLORS.default;
            }}
          />
          <Controls className="border rounded-md bg-background" />
          <Panel position="top-right">
            <div className="rounded-xl border bg-background/95 p-3 shadow-sm backdrop-blur">
              <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("visualization.legend.title")}
              </p>
              <div className="grid gap-2">
                <LegendItem
                  icon={<GitBranch className="size-3.5" />}
                  label={t("visualization.legend.task")}
                />
                <LegendItem
                  icon={<TimerReset className="size-3.5" />}
                  label={t("visualization.legend.dispatch")}
                />
                <LegendItem
                  icon={<Bot className="size-3.5" />}
                  label={t("visualization.legend.agent")}
                />
                <LegendItem
                  icon={<Cpu className="size-3.5" />}
                  label={t("visualization.legend.runtime")}
                />
              </div>
            </div>
          </Panel>
        </ReactFlow>
        </VisualizationErrorBoundary>
      </div>
    </div>
  );
}
