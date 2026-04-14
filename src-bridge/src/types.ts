import type { CostAccountingData } from "./cost/accounting.js";

export type AgentRuntimeKey =
  | "claude_code"
  | "codex"
  | "opencode"
  | "cursor"
  | "gemini"
  | "qoder"
  | "iflow";
export type DecomposeExecutionMode = "human" | "agent";
export type HookName =
  | "PreToolUse"
  | "PostToolUse"
  | "SubagentStart"
  | "SubagentStop"
  | "PermissionRequest"
  | "SessionStart"
  | "SessionEnd"
  | "Notification"
  | "UserPromptSubmit";
export type ResumeBlockingReason =
  | "missing_continuity_state"
  | "expired_continuity_state"
  | "runtime_mismatch"
  | "provider_rejected"
  | "continuity_not_supported";

export interface ThinkingConfig {
  enabled: boolean;
  budget_tokens?: number;
}

export interface StructuredOutputSchema {
  type: "json_schema";
  schema: Record<string, unknown>;
}

export interface HookDefinition {
  hook: HookName;
  matcher?: Record<string, unknown>;
}

export interface HooksConfig {
  hooks: HookDefinition[];
  callback_url?: string;
  timeout_ms?: number;
}

export interface Attachment {
  type: "image" | "file";
  path: string;
  mime_type?: string;
}

export interface AgentDefinition {
  description: string;
  prompt: string;
  tools?: string[];
  model?: string;
}

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
  warn_threshold?: number;
  allowed_tools: string[];
  permission_mode: string;
  role_config?: RoleConfig;
  team_id?: string;
  team_role?: "planner" | "coder" | "reviewer";
  team_context?: string;
  thinking_config?: ThinkingConfig;
  output_schema?: StructuredOutputSchema;
  hooks_config?: HooksConfig;
  hook_callback_url?: string;
  hook_timeout_ms?: number;
  attachments?: Attachment[];
  file_checkpointing?: boolean;
  agents?: Record<string, AgentDefinition>;
  disallowed_tools?: string[];
  fallback_model?: string;
  additional_directories?: string[];
  include_partial_messages?: boolean;
  tool_permission_callback?: boolean;
  web_search?: boolean;
  env?: Record<string, string>;
}

export interface ResumeRequest {
  task_id: string;
}

/** Request from Go Orchestrator to decompose an existing task. */
export interface DecomposeTaskRequest {
  task_id: string;
  title: string;
  description: string;
  priority: "critical" | "high" | "medium" | "low";
  provider?: string;
  model?: string;
  context?: unknown;
}

/** One child task returned from decomposition. */
export interface DecomposeSubtask {
  title: string;
  description: string;
  priority: "critical" | "high" | "medium" | "low";
  executionMode: DecomposeExecutionMode;
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
  /** MCP Server plugin IDs this role should use. */
  tools?: string[];
  /** Selected declared functions per plugin. */
  plugin_bindings?: Array<{
    plugin_id: string;
    functions?: string[];
  }>;
  /** Knowledge content to inject into the system prompt. */
  knowledge_context?: string;
  /** Auto-loaded skill bundles resolved by Go for prompt injection. */
  loaded_skills?: RoleExecutionSkill[];
  /** Non-auto-load skills kept as available runtime inventory. */
  available_skills?: RoleExecutionSkill[];
  /** Skill projection diagnostics computed by Go. */
  skill_diagnostics?: RoleExecutionSkillDiagnostic[];
  /** Output filters to apply (e.g., "no_credentials", "no_pii"). */
  output_filters?: string[];
  /** Tools explicitly blocked for this role. */
  blocked_tools?: string[];
  /** File path patterns the role is allowed to access. */
  file_permissions?: {
    allowed_patterns?: string[];
    blocked_patterns?: string[];
  };
  /** Network access restrictions. */
  network_permissions?: {
    allowed_domains?: string[];
    blocked?: boolean;
  };
}

export interface RoleExecutionSkill {
  path: string;
  label: string;
  description?: string;
  instructions?: string;
  display_name?: string;
  short_description?: string;
  default_prompt?: string;
  available_parts?: string[];
  reference_count?: number;
  script_count?: number;
  asset_count?: number;
  source?: string;
  source_root?: string;
  origin?: string;
  requires?: string[];
  tools?: string[];
}

export interface RoleExecutionSkillDiagnostic {
  code: string;
  path?: string;
  message: string;
  blocking: boolean;
  auto_load?: boolean;
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
  | "budget_alert"
  | "error"
  | "snapshot"
  | "heartbeat"
  | "reasoning"
  | "file_change"
  | "todo_update"
  | "progress"
  | "rate_limit"
  | "partial_message"
  | "permission_request"
  // PRD agent-prefixed aliases (Go can match either format)
  | "agent.output"
  | "agent.tool_call"
  | "agent.tool_result"
  | "agent.status"
  | "agent.cost"
  | "agent.error"
  | "agent.snapshot"
  // Plugin lifecycle & tool execution events
  | "tool.status_change"
  | "tool.call_log";

/** Current status of an agent runtime. */
export interface AgentStatus {
  task_id: string;
  state:
    | "idle"
    | "starting"
    | "running"
    | "paused"
    | "completed"
    | "failed"
    | "cancelled"
    | "budget_exceeded";
  turn_number: number;
  last_tool: string;
  last_activity_ms: number;
  spent_usd: number;
  runtime: AgentRuntimeKey;
  provider: string;
  model: string;
  role_id?: string;
  team_id?: string;
  team_role?: "planner" | "coder" | "reviewer";
  resume_ready?: boolean;
  resume_blocked_reason?: ResumeBlockingReason;
  structured_output?: Record<string, unknown>;
  thinking_enabled?: boolean;
  file_checkpointing?: boolean;
  active_hooks?: HookName[];
  subagent_count?: number;
  live_controls?: {
    interrupt?: boolean;
    set_model?: boolean;
    set_thinking_budget?: boolean;
    mcp_status?: boolean;
  };
  cost_accounting?: CostAccountingData;
  role_enforcement?: {
    max_turns: number;
    current_turn: number;
    turns_remaining: number;
    api_calls: number;
    execution_duration_ms: number;
  };
}

export interface RuntimeDiagnostic {
  code:
    | "missing_credentials"
    | "missing_executable"
    | "incompatible_provider"
    | "missing_server_url"
    | "server_unreachable"
    | "authentication_failed"
    | "provider_unavailable"
    | "model_unavailable"
    | "catalog_agents_unavailable"
    | "catalog_skills_unavailable"
    | "catalog_providers_unavailable"
    | "sunset_window"
    | "runtime_sunset"
    | "stale_default_selection";
  message: string;
  blocking: boolean;
}

export type RuntimeSupportState = "supported" | "degraded" | "unsupported";

export interface RuntimeCapabilityDescriptor {
  state: RuntimeSupportState;
  reasonCode?: string;
  message?: string;
  requiresRequestFields?: string[];
}

export interface RuntimeInteractionCapabilities {
  inputs: Record<string, RuntimeCapabilityDescriptor>;
  lifecycle: Record<string, RuntimeCapabilityDescriptor>;
  approval: Record<string, RuntimeCapabilityDescriptor>;
  mcp: Record<string, RuntimeCapabilityDescriptor>;
  diagnostics: Record<string, RuntimeCapabilityDescriptor>;
}

export interface RuntimeCatalogProvider {
  provider: string;
  connected: boolean;
  defaultModel?: string;
  modelOptions?: string[];
  authRequired?: boolean;
  authMethods?: string[];
}

export interface RuntimeLaunchContract {
  promptTransport: "stdin" | "positional" | "prompt_flag";
  outputMode: "text" | "json" | "stream-json";
  supportedOutputModes: Array<"text" | "json" | "stream-json">;
  supportedApprovalModes: string[];
  additionalDirectories: boolean;
  envOverrides: boolean;
}

export interface RuntimeLifecycleMetadata {
  stage: "active" | "sunsetting" | "sunset";
  sunsetAt?: string;
  replacementRuntime?: AgentRuntimeKey;
  message?: string;
}

export interface RuntimeCatalogEntry {
  key: AgentRuntimeKey;
  label: string;
  defaultProvider: string;
  compatibleProviders: string[];
  defaultModel?: string;
  modelOptions?: string[];
  available: boolean;
  diagnostics: RuntimeDiagnostic[];
  supportedFeatures: string[];
  interactionCapabilities?: RuntimeInteractionCapabilities;
  agents?: string[];
  skills?: string[];
  providers?: RuntimeCatalogProvider[];
  launchContract?: RuntimeLaunchContract;
  lifecycle?: RuntimeLifecycleMetadata;
}

export interface RuntimeCatalog {
  defaultRuntime: AgentRuntimeKey;
  runtimes: RuntimeCatalogEntry[];
}

/** Health check response. */
export interface HealthResponse {
  status: "SERVING" | "NOT_SERVING";
  active_agents: number;
  max_agents: number;
  uptime_ms: number;
}

export interface RuntimePoolSummary {
  active: number;
  max: number;
  warm_total: number;
  warm_available: number;
  warm_reuse_hits: number;
  cold_starts: number;
  last_reconcile_at: number;
  degraded: boolean;
}

/** Cost update event data. */
export interface CostUpdate {
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens?: number;
  cost_usd: number;
  budget_remaining_usd: number;
  turn_number: number;
  cost_accounting?: CostAccountingData;
}

export interface ReasoningEventData {
  content: string;
}

export interface FileChangeEventData {
  files: Array<{
    path: string;
    change_type?: string;
    [key: string]: unknown;
  }>;
}

export interface TodoUpdateEventData {
  todos: Array<{
    id?: string;
    content?: string;
    status?: string;
    [key: string]: unknown;
  }>;
}

export interface ProgressEventData {
  tool_name?: string;
  progress_text?: string;
  item_type?: string;
  partial_output?: unknown;
}

export interface RateLimitEventData {
  utilization?: number;
  reset_at?: string | number;
}

export interface PartialMessageEventData {
  content: string;
  is_complete: boolean;
}

export interface PermissionRequestEventData {
  request_id: string;
  tool_name?: string;
  context?: unknown;
  elicitation_type?: string;
  fields?: unknown[];
  mcp_server_id?: string;
}

/** Cancel request from Go Orchestrator. */
export interface CancelRequest {
  task_id: string;
  reason?: string;
}

export interface ForkRequest {
  task_id: string;
  message_id?: string;
}

export interface RollbackRequest {
  task_id: string;
  checkpoint_id?: string;
  turns?: number;
}

export interface RevertRequest {
  task_id: string;
  message_id: string;
}

export interface UnrevertRequest {
  task_id: string;
}

export interface CommandRequest {
  task_id: string;
  command: string;
  arguments?: string;
}

export interface InterruptRequest {
  task_id: string;
}

export interface ModelSwitchRequest {
  task_id: string;
  model: string;
}

export interface PermissionResponse {
  decision: "allow" | "deny";
  reason?: string;
}

export interface ClaudeContinuityState {
  runtime: "claude_code";
  resume_ready: boolean;
  captured_at: number;
  blocking_reason?: ResumeBlockingReason;
  session_handle?: string;
  checkpoint_id?: string;
  resume_token?: string;
  query_ref?: string;
  fork_available?: boolean;
}

export interface CodexContinuityState {
  runtime: "codex";
  resume_ready: boolean;
  captured_at: number;
  blocking_reason?: ResumeBlockingReason;
  thread_id?: string;
  fork_available?: boolean;
  rollback_turns?: number;
}

export interface OpenCodeContinuityState {
  runtime: "opencode";
  resume_ready: boolean;
  captured_at: number;
  blocking_reason?: ResumeBlockingReason;
  upstream_session_id?: string;
  latest_message_id?: string;
  server_url?: string;
  fork_available?: boolean;
  revert_message_ids?: string[];
}

export interface CliRuntimeContinuityState {
  runtime: "cursor" | "gemini" | "qoder" | "iflow";
  resume_ready: boolean;
  captured_at: number;
  blocking_reason?: ResumeBlockingReason;
}

export type RuntimeContinuityState =
  | ClaudeContinuityState
  | CodexContinuityState
  | OpenCodeContinuityState
  | CliRuntimeContinuityState;

/** Session snapshot for persistence. */
export interface SessionSnapshot {
  task_id: string;
  session_id: string;
  status: string;
  turn_number: number;
  spent_usd: number;
  created_at: number;
  updated_at: number;
  request?: ExecuteRequest;
  continuity?: RuntimeContinuityState;
  cost_accounting?: CostAccountingData;
}
