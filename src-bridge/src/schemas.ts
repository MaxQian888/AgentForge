import { z } from "zod";

const UnknownRecordSchema = z.record(z.string(), z.unknown());
const HookNameSchema = z.enum([
  "PreToolUse",
  "PostToolUse",
  "SubagentStart",
  "SubagentStop",
  "PermissionRequest",
]);

const RoleExecutionSkillSchema = z.object({
  path: z.string().min(1),
  label: z.string().min(1),
  description: z.string().optional(),
  instructions: z.string().optional(),
  source: z.string().optional(),
  source_root: z.string().optional(),
  origin: z.string().optional(),
  requires: z.array(z.string()).optional(),
  tools: z.array(z.string()).optional(),
}).strict();

const RoleExecutionSkillDiagnosticSchema = z.object({
  code: z.string().min(1),
  path: z.string().optional(),
  message: z.string().min(1),
  blocking: z.boolean(),
  auto_load: z.boolean().optional(),
}).strict();

export const RoleConfigSchema = z.object({
  role_id: z.string().min(1),
  name: z.string().min(1),
  role: z.string().min(1),
  goal: z.string().min(1),
  backstory: z.string(),
  system_prompt: z.string(),
  allowed_tools: z.array(z.string()),
  max_budget_usd: z.number().positive(),
  max_turns: z.number().int().positive(),
  permission_mode: z.string(),
  tools: z.array(z.string()).optional(),
  knowledge_context: z.string().optional(),
  loaded_skills: z.array(RoleExecutionSkillSchema).optional(),
  available_skills: z.array(RoleExecutionSkillSchema).optional(),
  skill_diagnostics: z.array(RoleExecutionSkillDiagnosticSchema).optional(),
  output_filters: z.array(z.string()).optional(),
}).strict();

export const ThinkingConfigSchema = z.object({
  enabled: z.boolean(),
  budget_tokens: z.number().int().positive().optional(),
});

export const StructuredOutputSchemaSchema = z.object({
  type: z.literal("json_schema"),
  schema: UnknownRecordSchema,
});

export const HookDefinitionSchema = z.object({
  hook: HookNameSchema,
  matcher: UnknownRecordSchema.optional(),
});

export const HooksConfigSchema = z.object({
  hooks: z.array(HookDefinitionSchema).min(1),
  callback_url: z.string().url().optional(),
  timeout_ms: z.number().int().positive().default(5000).optional(),
});

export const AttachmentSchema = z.object({
  type: z.enum(["image", "file"]),
  path: z.string().min(1),
  mime_type: z.string().min(1).optional(),
});

export const AgentDefinitionSchema = z.object({
  description: z.string().min(1),
  prompt: z.string().min(1),
  tools: z.array(z.string()).optional(),
  model: z.string().min(1).optional(),
});

export const ExecuteRequestSchema = z.object({
  task_id: z.string().min(1),
  session_id: z.string().min(1),
  runtime: z.enum(["claude_code", "codex", "opencode"]).optional(),
  provider: z.string().min(1).optional(),
  model: z.string().min(1).optional(),
  prompt: z.string().min(1),
  worktree_path: z.string(),
  branch_name: z.string(),
  system_prompt: z.string().default(""),
  max_turns: z.number().int().positive().default(50),
  budget_usd: z.number().positive().default(1.0),
  warn_threshold: z.number().min(0).max(1).optional(),
  allowed_tools: z.array(z.string()).default([]),
  permission_mode: z.string().default("default"),
  role_config: RoleConfigSchema.optional(),
  team_id: z.string().optional(),
  team_role: z.enum(["planner", "coder", "reviewer"]).optional(),
  thinking_config: ThinkingConfigSchema.optional(),
  output_schema: StructuredOutputSchemaSchema.optional(),
  hooks_config: HooksConfigSchema.optional(),
  hook_callback_url: z.string().url().optional(),
  hook_timeout_ms: z.number().int().positive().default(5000).optional(),
  attachments: z.array(AttachmentSchema).optional(),
  file_checkpointing: z.boolean().optional(),
  agents: z.record(z.string(), AgentDefinitionSchema).optional(),
  disallowed_tools: z.array(z.string()).optional(),
  fallback_model: z.string().min(1).optional(),
  additional_directories: z.array(z.string().min(1)).optional(),
  include_partial_messages: z.boolean().optional(),
  tool_permission_callback: z.boolean().optional(),
  web_search: z.boolean().optional(),
  env: z.record(z.string(), z.string()).optional(),
});

export const DecomposeTaskRequestSchema = z.object({
  task_id: z.string().min(1),
  title: z.string().min(1),
  description: z.string().min(1),
  priority: z.enum(["critical", "high", "medium", "low"]),
  provider: z.string().min(1).optional(),
  model: z.string().min(1).optional(),
});

export const DecomposeSubtaskSchema = z.object({
  title: z.string().min(1),
  description: z.string().min(1),
  priority: z.enum(["critical", "high", "medium", "low"]),
  executionMode: z.enum(["human", "agent"]),
});

export const DecomposeTaskResponseSchema = z.object({
  summary: z.string().min(1),
  subtasks: z.array(DecomposeSubtaskSchema).min(1),
});

export const CancelRequestSchema = z.object({
  task_id: z.string().min(1),
  reason: z.string().optional(),
});

export const ForkRequestSchema = z.object({
  task_id: z.string().min(1),
  message_id: z.string().min(1).optional(),
});

export const RollbackRequestSchema = z.object({
  task_id: z.string().min(1),
  checkpoint_id: z.string().min(1).optional(),
  turns: z.number().int().positive().optional(),
});

export const RevertRequestSchema = z.object({
  task_id: z.string().min(1),
  message_id: z.string().min(1),
});

export const UnrevertRequestSchema = z.object({
  task_id: z.string().min(1),
});

export const CommandRequestSchema = z.object({
  task_id: z.string().min(1),
  command: z.string().min(1),
  arguments: z.string().min(1).optional(),
});

export const InterruptRequestSchema = z.object({
  task_id: z.string().min(1),
});

export const ModelSwitchRequestSchema = z.object({
  task_id: z.string().min(1),
  model: z.string().min(1),
});

export const PermissionResponseSchema = z.object({
  decision: z.enum(["allow", "deny"]),
  reason: z.string().optional(),
});

export const ResumeRequestSchema = ExecuteRequestSchema.partial().extend({
  task_id: z.string().min(1),
});

export const ReasoningEventDataSchema = z.object({
  content: z.string().min(1),
});

export const FileChangeEventDataSchema = z.object({
  files: z.array(
    z.object({
      path: z.string().min(1),
      change_type: z.string().min(1).optional(),
    }).catchall(z.unknown()),
  ).min(1),
});

export const TodoUpdateEventDataSchema = z.object({
  todos: z.array(
    z.object({
      id: z.string().min(1).optional(),
      content: z.string().min(1).optional(),
      status: z.string().min(1).optional(),
    }).catchall(z.unknown()),
  ),
});

export const ProgressEventDataSchema = z.object({
  tool_name: z.string().min(1).optional(),
  progress_text: z.string().min(1).optional(),
  item_type: z.string().min(1).optional(),
  partial_output: z.unknown().optional(),
});

export const RateLimitEventDataSchema = z.object({
  utilization: z.number().min(0).max(1).optional(),
  reset_at: z.union([z.string().min(1), z.number()]).optional(),
});

export const PartialMessageEventDataSchema = z.object({
  content: z.string(),
  is_complete: z.boolean(),
});

export const PermissionRequestEventDataSchema = z.object({
  request_id: z.string().min(1),
  tool_name: z.string().min(1).optional(),
  context: z.unknown().optional(),
  elicitation_type: z.string().min(1).optional(),
  fields: z.array(z.unknown()).optional(),
  mcp_server_id: z.string().min(1).optional(),
});

export const ReviewDimensionSchema = z.enum([
  "logic",
  "security",
  "performance",
  "compliance",
]);

export const DeepReviewRequestSchema = z.object({
  review_id: z.string().min(1),
  task_id: z.string().min(1),
  pr_url: z.string().min(1),
  pr_number: z.number().int().nonnegative().optional(),
  title: z.string().optional(),
  description: z.string().optional(),
  diff: z.string().optional(),
  trigger_event: z.string().min(1).optional(),
  changed_files: z.array(z.string().min(1)).optional(),
  dimensions: z.array(ReviewDimensionSchema).optional(),
  review_plugins: z.array(
    z.object({
      plugin_id: z.string().min(1),
      name: z.string().min(1),
      entrypoint: z.string().min(1).optional(),
      source_type: z.string().min(1).optional(),
      transport: z.enum(["stdio", "http"]).optional(),
      command: z.string().min(1).optional(),
      args: z.array(z.string()).optional(),
      url: z.string().url().optional(),
      events: z.array(z.string().min(1)).optional(),
      file_patterns: z.array(z.string().min(1)).optional(),
      output_format: z.string().min(1).optional(),
    }),
  ).optional(),
});
