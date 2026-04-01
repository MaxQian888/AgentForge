import type { Edge, Node } from "@xyflow/react";
import type {
  Agent,
  AgentPoolQueueEntry,
  AgentPoolSummary,
  BridgeHealthSummary,
} from "@/lib/stores/agent-store";
import type {
  CodingAgentDiagnostic,
  CodingAgentCatalog,
  CodingAgentRuntimeOption,
} from "@/lib/stores/project-store";
import { priorityLabel } from "./agent-status-colors";

const LANE_X = {
  task: 32,
  dispatch: 300,
  agent: 568,
  runtime: 836,
} as const;

const TASK_ROW_GAP = 196;
const LANE_STACK_GAP = 104;

export type AgentVisualizationNodeKind =
  | "task"
  | "dispatch"
  | "agent"
  | "runtime";

export type AgentVisualizationTone =
  | "default"
  | "success"
  | "warning"
  | "danger"
  | "muted";

export interface AgentVisualizationNodeData {
  [key: string]: unknown;
  kind: AgentVisualizationNodeKind;
  title: string;
  subtitle?: string;
  metadata?: string[];
  badges?: string[];
  tone: AgentVisualizationTone;
  budgetPct?: number;
}

export interface AgentVisualizationSummary {
  agentCount: number;
  queueCount: number;
  runtimeCount: number;
  taskCount: number;
  hasGraphData: boolean;
  isFiltered: boolean;
  isDegraded: boolean;
}

export interface AgentVisualizationTaskFocus {
  kind: "task";
  nodeId: string;
  taskId: string;
  taskTitle: string;
  agentCount: number;
  queueCount: number;
  agentIds: string[];
  queueEntryIds: string[];
}

export interface AgentVisualizationDispatchFocus {
  kind: "dispatch";
  nodeId: string;
  entryId: string;
  taskId: string;
  taskTitle: string;
  status: string;
  reason: string;
  priority?: number;
  priorityLabel: string;
  runtime: string;
  provider: string;
  model: string;
  budgetUsd?: number;
  agentRunId?: string;
}

export interface AgentVisualizationRuntimeFocus {
  kind: "runtime";
  nodeId: string;
  label: string;
  runtime: string;
  provider: string;
  model: string;
  available: boolean;
  diagnostics: CodingAgentDiagnostic[];
  supportedFeatures: string[];
  agentCount: number;
  dispatchCount: number;
  agentIds: string[];
  queueEntryIds: string[];
}

export interface AgentVisualizationAgentFocus {
  kind: "agent";
  nodeId: string;
  agentId: string;
  label: string;
  status: string;
  runtime: string;
  provider: string;
  model: string;
  costUsd: number;
  budgetUsd: number;
  budgetPct: number;
  turnCount: number;
  taskId: string;
  taskTitle: string;
  canResume: boolean;
  worktreePath?: string;
  branchName?: string;
}

export type AgentVisualizationFocus =
  | AgentVisualizationTaskFocus
  | AgentVisualizationDispatchFocus
  | AgentVisualizationRuntimeFocus
  | AgentVisualizationAgentFocus;

export interface AgentVisualizationModel {
  nodes: Node<AgentVisualizationNodeData, "agentVisualization">[];
  edges: Edge[];
  summary: AgentVisualizationSummary;
  focusByNodeId: Record<string, AgentVisualizationFocus>;
}

interface BuildAgentVisualizationModelInput {
  agents: Agent[];
  pool: AgentPoolSummary | null;
  runtimeCatalog: CodingAgentCatalog | null;
  bridgeHealth: BridgeHealthSummary | null;
  requestedMemberId: string | null;
}

interface TaskScope {
  taskId: string;
  taskTitle: string;
  agents: Agent[];
  queueEntries: AgentPoolQueueEntry[];
}

interface RuntimeDescriptor {
  id: string;
  runtime: string;
  provider: string;
  model: string;
  label: string;
  tone: AgentVisualizationTone;
  metadata: string[];
  available: boolean;
  diagnostics: CodingAgentDiagnostic[];
  supportedFeatures: string[];
  agentIds: Set<string>;
  queueEntryIds: Set<string>;
}

function compareByText(a: string, b: string) {
  return a.localeCompare(b, "en");
}

function sortAgents(agents: Agent[]) {
  return [...agents].sort((a, b) => {
    const started = (a.startedAt || a.createdAt || a.id || "").localeCompare(
      b.startedAt || b.createdAt || b.id || "",
    );
    if (started !== 0) {
      return started;
    }
    return compareByText(a.id || "", b.id || "");
  });
}

function sortQueueEntries(entries: AgentPoolQueueEntry[]) {
  return [...entries].sort((a, b) => {
    const created = a.createdAt.localeCompare(b.createdAt);
    if (created !== 0) {
      return created;
    }
    return compareByText(a.entryId, b.entryId);
  });
}

function budgetPct(cost: number, budget: number) {
  if (budget <= 0) {
    return 0;
  }

  return Math.min((cost / budget) * 100, 100);
}

function toneFromAgentStatus(agent: Agent): AgentVisualizationTone {
  switch (agent.status) {
    case "running":
      return budgetPct(agent.cost, agent.budget) >= 80 ? "warning" : "success";
    case "paused":
    case "budget_exceeded":
      return "warning";
    case "failed":
    case "cancelled":
      return "danger";
    case "completed":
      return "muted";
    default:
      return "default";
  }
}

function toneFromQueue(entry: AgentPoolQueueEntry): AgentVisualizationTone {
  if (entry.status === "blocked") {
    return "danger";
  }
  if (entry.status === "queued") {
    return "warning";
  }
  return "default";
}

function runtimeLookup(
  runtimeCatalog: CodingAgentCatalog | null,
  runtime: string,
): CodingAgentRuntimeOption | undefined {
  return runtimeCatalog?.runtimes.find((item) => item.runtime === runtime);
}

function runtimeId(
  runtime: string,
  provider: string,
  model: string,
) {
  return `runtime:${runtime || "unknown"}:${provider || "unknown"}:${model || "unknown"}`;
}

function resolveRuntimeDescriptor(
  runtimeCatalog: CodingAgentCatalog | null,
  runtime: string,
  provider: string,
  model: string,
): RuntimeDescriptor | null {
  if (!runtime && !provider && !model) {
    return null;
  }

  const runtimeOption = runtimeLookup(runtimeCatalog, runtime);
  const resolvedProvider = provider || runtimeOption?.defaultProvider || "unknown";
  const resolvedModel = model || runtimeOption?.defaultModel || "unknown";
  const label = runtimeOption?.label || runtime || "Unknown Runtime";
  const metadata = [
    resolvedProvider !== "unknown" ? resolvedProvider : null,
    resolvedModel !== "unknown" ? resolvedModel : null,
  ].filter(Boolean) as string[];

  return {
    id: runtimeId(runtime || "unknown", resolvedProvider, resolvedModel),
    runtime,
    provider: resolvedProvider,
    model: resolvedModel,
    label,
    tone:
      runtimeOption?.available === false
        ? "danger"
        : runtimeOption?.available
          ? "success"
          : "default",
    metadata:
      runtimeOption?.diagnostics.length
        ? [...metadata, runtimeOption.diagnostics[0].message]
        : metadata,
    available: runtimeOption?.available ?? false,
    diagnostics: runtimeOption?.diagnostics ?? [],
    supportedFeatures: runtimeOption?.supportedFeatures ?? [],
    agentIds: new Set<string>(),
    queueEntryIds: new Set<string>(),
  };
}

function collectTaskScopes(
  agents: Agent[],
  queueEntries: AgentPoolQueueEntry[],
): TaskScope[] {
  const scopes = new Map<string, TaskScope>();

  for (const agent of sortAgents(agents)) {
    const taskId = agent.taskId || agent.id || "unknown-task";
    const current = scopes.get(taskId) ?? {
      taskId,
      taskTitle: agent.taskTitle || taskId,
      agents: [],
      queueEntries: [],
    };
    current.agents.push(agent);
    scopes.set(taskId, current);
  }

  for (const entry of sortQueueEntries(queueEntries)) {
    const current = scopes.get(entry.taskId) ?? {
      taskId: entry.taskId,
      taskTitle: entry.taskId,
      agents: [],
      queueEntries: [],
    };
    current.queueEntries.push(entry);
    scopes.set(entry.taskId, current);
  }

  return [...scopes.values()].sort((a, b) => compareByText(a.taskId, b.taskId));
}

export function buildAgentVisualizationModel({
  agents,
  pool,
  runtimeCatalog,
  bridgeHealth,
  requestedMemberId,
}: BuildAgentVisualizationModelInput): AgentVisualizationModel {
  const edgeLabelStyle = { fontSize: 10, fill: "hsl(var(--muted-foreground))" };
  const edgeLabelBgStyle = { fill: "hsl(var(--background))", fillOpacity: 0.8 };

  const scopedAgents = requestedMemberId
    ? agents.filter((agent) => agent.memberId === requestedMemberId)
    : agents;
  const scopedQueueEntries = requestedMemberId
    ? (pool?.queue ?? []).filter((entry) => entry.memberId === requestedMemberId)
    : (pool?.queue ?? []);

  const taskScopes = collectTaskScopes(scopedAgents, scopedQueueEntries);
  const nodes: Node<AgentVisualizationNodeData, "agentVisualization">[] = [];
  const edges: Edge[] = [];
  const runtimeDescriptors = new Map<string, RuntimeDescriptor>();
  const focusByNodeId: Record<string, AgentVisualizationFocus> = {};

  taskScopes.forEach((scope, taskIndex) => {
    const taskY = taskIndex * TASK_ROW_GAP;
    const taskNodeId = `task:${scope.taskId}`;

    nodes.push({
      id: taskNodeId,
      type: "agentVisualization",
      position: { x: LANE_X.task, y: taskY },
      data: {
        kind: "task",
        title: scope.taskTitle,
        subtitle: scope.taskId,
        metadata: [
          `${scope.agents.length} agent${scope.agents.length === 1 ? "" : "s"}`,
          `${scope.queueEntries.length} queued`,
        ],
        tone: "default",
      },
      draggable: false,
      selectable: true,
    });
    focusByNodeId[taskNodeId] = {
      kind: "task",
      nodeId: taskNodeId,
      taskId: scope.taskId,
      taskTitle: scope.taskTitle,
      agentCount: scope.agents.length,
      queueCount: scope.queueEntries.length,
      agentIds: scope.agents.map((agent) => agent.id),
      queueEntryIds: scope.queueEntries.map((entry) => entry.entryId),
    };

    sortQueueEntries(scope.queueEntries).forEach((entry, index) => {
      const dispatchNodeId = `dispatch:${entry.entryId}`;
      nodes.push({
        id: dispatchNodeId,
        type: "agentVisualization",
        position: { x: LANE_X.dispatch, y: taskY + index * LANE_STACK_GAP },
        data: {
          kind: "dispatch",
          title: entry.status,
          subtitle: entry.reason || entry.taskId,
          metadata: [priorityLabel(entry.priority)],
          badges: [entry.runtime || "runtime pending"],
          tone: toneFromQueue(entry),
        },
        draggable: false,
        selectable: true,
      });
      focusByNodeId[dispatchNodeId] = {
        kind: "dispatch",
        nodeId: dispatchNodeId,
        entryId: entry.entryId,
        taskId: scope.taskId,
        taskTitle: scope.taskTitle,
        status: entry.status,
        reason: entry.reason,
        priority: entry.priority,
        priorityLabel: priorityLabel(entry.priority),
        runtime: entry.runtime || "",
        provider: entry.provider || "",
        model: entry.model || "",
        budgetUsd: entry.budgetUsd,
        agentRunId: entry.agentRunId,
      };

      edges.push({
        id: `${taskNodeId}->${dispatchNodeId}`,
        source: taskNodeId,
        target: dispatchNodeId,
        type: "smoothstep",
        animated: entry.status === "queued",
        label: "queued",
        labelStyle: edgeLabelStyle,
        labelBgStyle: edgeLabelBgStyle,
        style:
          entry.status === "queued"
            ? { stroke: "hsl(var(--chart-2))" }
            : entry.status === "blocked"
              ? { stroke: "hsl(var(--destructive))", opacity: 0.6 }
              : undefined,
      });

      const runtime = resolveRuntimeDescriptor(
        runtimeCatalog,
        entry.runtime || "",
        entry.provider || "",
        entry.model || "",
      );
      if (runtime) {
        const existingRuntime = runtimeDescriptors.get(runtime.id) ?? runtime;
        existingRuntime.queueEntryIds.add(entry.entryId);
        runtimeDescriptors.set(runtime.id, existingRuntime);
        edges.push({
          id: `${dispatchNodeId}->${runtime.id}`,
          source: dispatchNodeId,
          target: runtime.id,
          type: "smoothstep",
          animated: entry.status === "queued",
          label: "targets",
          labelStyle: edgeLabelStyle,
          labelBgStyle: edgeLabelBgStyle,
          style:
            entry.status === "queued"
              ? { stroke: "hsl(var(--chart-2))" }
              : entry.status === "blocked"
                ? { stroke: "hsl(var(--destructive))", opacity: 0.6 }
                : undefined,
        });
      }
    });

    sortAgents(scope.agents).forEach((agent, index) => {
      const agentNodeId = `agent:${agent.id}`;
      const usage = budgetPct(agent.cost, agent.budget);

      nodes.push({
        id: agentNodeId,
        type: "agentVisualization",
        position: {
          x: LANE_X.agent,
          y:
            taskY +
            Math.max(scope.queueEntries.length, 1) * 24 +
            index * LANE_STACK_GAP,
        },
        data: {
          kind: "agent",
          title: agent.roleName || agent.id,
          subtitle: agent.taskTitle || agent.taskId || agent.id,
          metadata: [
            agent.runtime || "runtime pending",
            `${agent.provider || "-"} / ${agent.model || "-"}`,
          ],
          badges: [agent.status],
          tone: toneFromAgentStatus(agent),
          budgetPct: usage,
        },
        draggable: false,
        selectable: true,
      });
      focusByNodeId[agentNodeId] = {
        kind: "agent",
        nodeId: agentNodeId,
        agentId: agent.id,
        label: agent.roleName || agent.id,
        status: agent.status,
        runtime: agent.runtime,
        provider: agent.provider,
        model: agent.model,
        costUsd: agent.cost,
        budgetUsd: agent.budget,
        budgetPct: usage,
        turnCount: agent.turns,
        taskId: scope.taskId,
        taskTitle: scope.taskTitle,
        canResume: agent.canResume,
        worktreePath: agent.worktreePath || undefined,
        branchName: agent.branchName || undefined,
      };

      edges.push({
        id: `${taskNodeId}->${agentNodeId}`,
        source: taskNodeId,
        target: agentNodeId,
        type: "smoothstep",
        animated: agent.status === "running" || agent.status === "starting",
        label: "assigned",
        labelStyle: edgeLabelStyle,
        labelBgStyle: edgeLabelBgStyle,
        style:
          agent.status === "running" || agent.status === "starting"
            ? { stroke: "hsl(var(--chart-2))" }
            : agent.status === "completed"
              ? { stroke: "hsl(var(--muted-foreground))", opacity: 0.5 }
              : agent.status === "failed" || agent.status === "cancelled"
                ? { stroke: "hsl(var(--destructive))", opacity: 0.6 }
                : undefined,
      });

      const runtime = resolveRuntimeDescriptor(
        runtimeCatalog,
        agent.runtime,
        agent.provider,
        agent.model,
      );
      if (runtime) {
        const existingRuntime = runtimeDescriptors.get(runtime.id) ?? runtime;
        existingRuntime.agentIds.add(agent.id);
        runtimeDescriptors.set(runtime.id, existingRuntime);
        edges.push({
          id: `${agentNodeId}->${runtime.id}`,
          source: agentNodeId,
          target: runtime.id,
          type: "smoothstep",
          animated: agent.status === "running",
          label: "uses",
          labelStyle: edgeLabelStyle,
          labelBgStyle: edgeLabelBgStyle,
          style:
            agent.status === "running"
              ? { stroke: "hsl(var(--chart-2))" }
              : agent.status === "completed"
                ? { stroke: "hsl(var(--muted-foreground))", opacity: 0.5 }
                : agent.status === "failed" || agent.status === "cancelled"
                  ? { stroke: "hsl(var(--destructive))", opacity: 0.6 }
                  : undefined,
        });
      }
    });
  });

  [...runtimeDescriptors.values()]
    .sort((a, b) => compareByText(a.id, b.id))
    .forEach((runtime, index) => {
      nodes.push({
        id: runtime.id,
        type: "agentVisualization",
        position: { x: LANE_X.runtime, y: index * TASK_ROW_GAP },
        data: {
          kind: "runtime",
          title: runtime.label,
          subtitle: runtime.runtime || "unknown",
          metadata: runtime.metadata,
          badges: [runtime.provider, runtime.model].filter(Boolean),
          tone: runtime.tone,
        },
        draggable: false,
        selectable: true,
      });
      focusByNodeId[runtime.id] = {
        kind: "runtime",
        nodeId: runtime.id,
        label: runtime.label,
        runtime: runtime.runtime,
        provider: runtime.provider,
        model: runtime.model,
        available: runtime.available,
        diagnostics: runtime.diagnostics,
        supportedFeatures: runtime.supportedFeatures,
        agentCount: runtime.agentIds.size,
        dispatchCount: runtime.queueEntryIds.size,
        agentIds: [...runtime.agentIds].sort(compareByText),
        queueEntryIds: [...runtime.queueEntryIds].sort(compareByText),
      };
    });

  return {
    nodes,
    edges,
    summary: {
      agentCount: scopedAgents.length,
      queueCount: scopedQueueEntries.length,
      runtimeCount: runtimeDescriptors.size,
      taskCount: taskScopes.length,
      hasGraphData: scopedAgents.length > 0 || scopedQueueEntries.length > 0,
      isFiltered: Boolean(requestedMemberId),
      isDegraded: bridgeHealth?.status === "degraded" || Boolean(pool?.degraded),
    },
    focusByNodeId,
  };
}
