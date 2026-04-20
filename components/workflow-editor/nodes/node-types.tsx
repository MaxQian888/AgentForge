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
  BrainCircuit,
  Code2,
  RefreshCw,
  UserCheck,
  Webhook,
  Workflow,
  Globe,
  MessageSquare,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { NODE_STYLES } from "./node-styles";

interface WorkflowNodeBase {
  label: string;
  config?: Record<string, unknown>;
}

// Icon lookup — kept local so this file is self-contained for ReactFlow rendering.
const NODE_ICONS: Record<string, React.ElementType> = {
  trigger: Play,
  condition: GitBranch,
  agent_dispatch: Bot,
  notification: Bell,
  status_transition: ArrowRightLeft,
  gate: Lock,
  parallel_split: Split,
  parallel_join: Merge,
  llm_agent: BrainCircuit,
  function: Code2,
  loop: RefreshCw,
  human_review: UserCheck,
  wait_event: Webhook,
  sub_workflow: Workflow,
  http_call: Globe,
  im_send: MessageSquare,
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
  const style = NODE_STYLES[nodeType] ?? NODE_STYLES.trigger;
  const IconComponent = NODE_ICONS[nodeType] ?? Play;

  return (
    <div
      className={cn(
        "relative px-4 py-3 min-w-[140px] border-2 shadow-sm transition-shadow",
        style.bg,
        style.border,
        selected && "ring-2 ring-blue-500 shadow-md",
        isCondition
          ? "rotate-45 w-[100px] h-[100px] flex items-center justify-center"
          : "rounded-lg"
      )}
    >
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-muted-foreground !w-2.5 !h-2.5 !border-2 !border-background"
        style={
          isCondition
            ? { top: -6, left: "50%", transform: "rotate(-45deg)" }
            : undefined
        }
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
        style={
          isCondition
            ? { bottom: -6, left: "50%", transform: "rotate(-45deg)" }
            : undefined
        }
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

export const LLMAgentNode = memo(function LLMAgentNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="llm_agent"
      selected={props.selected}
    />
  );
});

export const FunctionNode = memo(function FunctionNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="function"
      selected={props.selected}
    />
  );
});

export const LoopNode = memo(function LoopNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="loop"
      selected={props.selected}
    />
  );
});

export const HumanReviewNode = memo(function HumanReviewNode(
  props: NodeProps
) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="human_review"
      selected={props.selected}
    />
  );
});

export const WaitEventNode = memo(function WaitEventNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="wait_event"
      selected={props.selected}
    />
  );
});

export const SubWorkflowNode = memo(function SubWorkflowNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="sub_workflow"
      selected={props.selected}
    />
  );
});

export const HTTPCallNode = memo(function HTTPCallNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="http_call"
      selected={props.selected}
    />
  );
});

export const IMSendNode = memo(function IMSendNode(props: NodeProps) {
  return (
    <BaseWorkflowNode
      data={props.data as unknown as WorkflowNodeBase}
      nodeType="im_send"
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
  llm_agent: LLMAgentNode,
  function: FunctionNode,
  loop: LoopNode,
  human_review: HumanReviewNode,
  wait_event: WaitEventNode,
  sub_workflow: SubWorkflowNode,
  http_call: HTTPCallNode,
  im_send: IMSendNode,
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
  llm_agent: "LLM Agent",
  function: "Function",
  loop: "Loop",
  human_review: "Human Review",
  wait_event: "Wait Event",
  sub_workflow: "Sub-Workflow",
  http_call: "HTTP Call",
  im_send: "IM Send",
};
