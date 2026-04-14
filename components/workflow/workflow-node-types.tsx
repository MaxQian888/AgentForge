"use client";

import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import {
  Play,
  GitBranch,
  Bot,
  Bell,
  ArrowRightLeft,
  Lock,
  Split,
  Merge,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface WorkflowNodeBase {
  label: string;
  config?: Record<string, unknown>;
}

const nodeStyles: Record<
  string,
  { bg: string; border: string; icon: React.ElementType; iconColor: string }
> = {
  trigger: {
    bg: "bg-green-50 dark:bg-green-950",
    border: "border-green-400 dark:border-green-600",
    icon: Play,
    iconColor: "text-green-600 dark:text-green-400",
  },
  condition: {
    bg: "bg-amber-50 dark:bg-amber-950",
    border: "border-amber-400 dark:border-amber-600",
    icon: GitBranch,
    iconColor: "text-amber-600 dark:text-amber-400",
  },
  agent_dispatch: {
    bg: "bg-blue-50 dark:bg-blue-950",
    border: "border-blue-400 dark:border-blue-600",
    icon: Bot,
    iconColor: "text-blue-600 dark:text-blue-400",
  },
  notification: {
    bg: "bg-yellow-50 dark:bg-yellow-950",
    border: "border-yellow-400 dark:border-yellow-600",
    icon: Bell,
    iconColor: "text-yellow-600 dark:text-yellow-400",
  },
  status_transition: {
    bg: "bg-purple-50 dark:bg-purple-950",
    border: "border-purple-400 dark:border-purple-600",
    icon: ArrowRightLeft,
    iconColor: "text-purple-600 dark:text-purple-400",
  },
  gate: {
    bg: "bg-red-50 dark:bg-red-950",
    border: "border-red-400 dark:border-red-600",
    icon: Lock,
    iconColor: "text-red-600 dark:text-red-400",
  },
  parallel_split: {
    bg: "bg-orange-50 dark:bg-orange-950",
    border: "border-orange-400 dark:border-orange-600",
    icon: Split,
    iconColor: "text-orange-600 dark:text-orange-400",
  },
  parallel_join: {
    bg: "bg-orange-50 dark:bg-orange-950",
    border: "border-orange-400 dark:border-orange-600",
    icon: Merge,
    iconColor: "text-orange-600 dark:text-orange-400",
  },
};

function BaseWorkflowNode({
  data,
  nodeType,
  selected,
  isCondition,
}: {
  data: WorkflowNodeBase;
  nodeType: string;
  selected?: boolean;
  isCondition?: boolean;
}) {
  const style = nodeStyles[nodeType] ?? nodeStyles.trigger;
  const IconComponent = style.icon;

  return (
    <div
      className={cn(
        "relative px-4 py-3 min-w-[140px] border-2 shadow-sm transition-shadow",
        style.bg,
        style.border,
        selected && "ring-2 ring-blue-500 shadow-md",
        isCondition ? "rotate-45 w-[100px] h-[100px] flex items-center justify-center" : "rounded-lg"
      )}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-muted-foreground !w-2.5 !h-2.5 !border-2 !border-background"
        style={isCondition ? { top: -6, left: "50%", transform: "rotate(-45deg)" } : undefined}
      />
      <div className={cn("flex items-center gap-2", isCondition && "-rotate-45")}>
        <IconComponent className={cn("h-4 w-4 shrink-0", style.iconColor)} />
        <span className="text-sm font-medium truncate max-w-[120px]">
          {data.label}
        </span>
      </div>
      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-muted-foreground !w-2.5 !h-2.5 !border-2 !border-background"
        style={isCondition ? { bottom: -6, left: "50%", transform: "rotate(-45deg)" } : undefined}
      />
    </div>
  );
}

export const TriggerNode = memo(function TriggerNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="trigger"
      selected={props.selected}
    />
  );
});

export const ConditionNode = memo(function ConditionNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="condition"
      selected={props.selected}
      isCondition
    />
  );
});

export const AgentDispatchNode = memo(function AgentDispatchNode(
  props: NodeProps
) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="agent_dispatch"
      selected={props.selected}
    />
  );
});

export const NotificationNode = memo(function NotificationNode(
  props: NodeProps
) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="notification"
      selected={props.selected}
    />
  );
});

export const StatusTransitionNode = memo(function StatusTransitionNode(
  props: NodeProps
) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="status_transition"
      selected={props.selected}
    />
  );
});

export const GateNode = memo(function GateNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="gate"
      selected={props.selected}
    />
  );
});

export const ParallelSplitNode = memo(function ParallelSplitNode(
  props: NodeProps
) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="parallel_split"
      selected={props.selected}
    />
  );
});

export const ParallelJoinNode = memo(function ParallelJoinNode(
  props: NodeProps
) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="parallel_join"
      selected={props.selected}
    />
  );
});

export const workflowNodeTypes = {
  trigger: TriggerNode,
  condition: ConditionNode,
  agent_dispatch: AgentDispatchNode,
  notification: NotificationNode,
  status_transition: StatusTransitionNode,
  gate: GateNode,
  parallel_split: ParallelSplitNode,
  parallel_join: ParallelJoinNode,
};

export const NODE_TYPE_LABELS: Record<string, string> = {
  trigger: "Trigger",
  condition: "Condition",
  agent_dispatch: "Agent Dispatch",
  notification: "Notification",
  status_transition: "Status Transition",
  gate: "Gate",
  parallel_split: "Parallel Split",
  parallel_join: "Parallel Join",
};
