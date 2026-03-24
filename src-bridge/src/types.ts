export type AgentRuntimeKey = "claude_code" | "codex" | "opencode";

/** Request from Go Orchestrator to execute an agent task. */
export interface ExecuteRequest {
  task_id: string;
  session_id: string;
  runtime?: AgentRuntimeKey;
  provider?: string;
  model?: string;
  prompt: string;
  worktree_path: string;
  branch_name: string;
  system_prompt: string;
  max_turns: number;
  budget_usd: number;
  allowed_tools: string[];
  permission_mode: string;
  role_config?: RoleConfig;
}

/** Request from Go Orchestrator to decompose an existing task. */
export interface DecomposeTaskRequest {
  task_id: string;
  title: string;
  description: string;
  priority: "critical" | "high" | "medium" | "low";
  provider?: string;
  model?: string;
}

/** One child task returned from decomposition. */
export interface DecomposeSubtask {
  title: string;
  description: string;
  priority: "critical" | "high" | "medium" | "low";
}

/** Structured decomposition result returned to Go. */
export interface DecomposeTaskResponse {
  summary: string;
  subtasks: DecomposeSubtask[];
}

/** Role-based configuration for agent persona injection. */
export interface RoleConfig {
  role_id: string;
  name: string;
  role: string;
  goal: string;
  backstory: string;
  system_prompt: string;
  allowed_tools: string[];
  max_budget_usd: number;
  max_turns: number;
  permission_mode: string;
}

/** Event envelope sent to Go Orchestrator via WebSocket. */
export interface AgentEvent {
  task_id: string;
  session_id: string;
  timestamp_ms: number;
  type: AgentEventType;
  data: unknown;
}

export type AgentEventType =
  | "output"
  | "tool_call"
  | "tool_result"
  | "status_change"
  | "cost_update"
  | "error"
  | "snapshot";

/** Current status of an agent runtime. */
export interface AgentStatus {
  task_id: string;
  state: "idle" | "starting" | "running" | "paused" | "completed" | "failed";
  turn_number: number;
  last_tool: string;
  last_activity_ms: number;
  spent_usd: number;
}

/** Health check response. */
export interface HealthResponse {
  status: "SERVING" | "NOT_SERVING";
  active_agents: number;
  max_agents: number;
  uptime_ms: number;
}

/** Cost update event data. */
export interface CostUpdate {
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cost_usd: number;
  budget_remaining_usd: number;
}

/** Cancel request from Go Orchestrator. */
export interface CancelRequest {
  task_id: string;
  reason?: string;
}

/** Session snapshot for persistence. */
export interface SessionSnapshot {
  task_id: string;
  session_id: string;
  status: string;
  turn_number: number;
  spent_usd: number;
  created_at: number;
  updated_at: number;
}
