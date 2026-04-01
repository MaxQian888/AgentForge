import { describe, expect, test } from "bun:test";
import type {
  AgentStatus,
  ExecuteRequest,
  RoleConfig,
  RuntimeCatalog,
  RuntimeContinuityState,
  SessionSnapshot,
} from "./types.js";

describe("bridge contract types", () => {
  test("accepts a codex execution snapshot with runtime catalog metadata", () => {
    const roleConfig: RoleConfig = {
      role_id: "code-reviewer",
      name: "Code Reviewer",
      role: "Senior Reviewer",
      goal: "Find risky changes before merge",
      backstory: "A skeptical reviewer focused on release safety.",
      system_prompt: "Review carefully.",
      allowed_tools: ["Read"],
      max_budget_usd: 5,
      max_turns: 8,
      permission_mode: "default",
      knowledge_context: "docs/review-playbook.md",
      loaded_skills: [
        {
          path: "roles/reviewer/skills/security.md",
          label: "Security",
        },
      ],
      skill_diagnostics: [
        {
          code: "inventory_loaded",
          message: "Loaded repo-local review skills.",
          blocking: false,
        },
      ],
      output_filters: ["no_credentials"],
    };
    const continuity: RuntimeContinuityState = {
      runtime: "codex",
      resume_ready: true,
      captured_at: 1_717_000_000_000,
      thread_id: "thread-codex-1",
      fork_available: true,
      rollback_turns: 2,
    };
    const request: ExecuteRequest = {
      task_id: "task-123",
      session_id: "session-123",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      prompt: "Review the bridge runtime contract.",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      system_prompt: "Base prompt",
      max_turns: 8,
      budget_usd: 5,
      allowed_tools: ["Read"],
      permission_mode: "default",
      role_config: roleConfig,
      team_id: "team-42",
      team_role: "reviewer",
      thinking_config: {
        enabled: true,
        budget_tokens: 8_000,
      },
      hooks_config: {
        hooks: [{ hook: "PreToolUse" }],
      },
      include_partial_messages: true,
      output_schema: {
        type: "json_schema",
        schema: {
          type: "object",
          properties: {
            summary: { type: "string" },
          },
        },
      },
    };
    const status: AgentStatus = {
      task_id: request.task_id,
      state: "paused",
      turn_number: 3,
      last_tool: "Read",
      last_activity_ms: 1_717_000_000_123,
      spent_usd: 1.25,
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      role_id: roleConfig.role_id,
      team_id: request.team_id,
      team_role: request.team_role,
      resume_ready: true,
      active_hooks: ["PreToolUse"],
      subagent_count: 1,
      thinking_enabled: true,
      cost_accounting: {
        total_cost_usd: 1.25,
        input_tokens: 2_000,
        output_tokens: 900,
        cache_read_tokens: 120,
        cache_creation_tokens: 80,
        mode: "estimated_api_pricing",
        coverage: "full",
        source: "openai_api_pricing",
        components: [],
      },
    };
    const catalog: RuntimeCatalog = {
      defaultRuntime: "codex",
      runtimes: [
        {
          key: "codex",
          label: "Codex",
          defaultProvider: "openai",
          compatibleProviders: ["openai", "codex"],
          defaultModel: "gpt-5-codex",
          available: true,
          diagnostics: [],
          supportedFeatures: ["fork", "reasoning", "output_schema"],
          skills: ["review"],
        },
      ],
    };
    const snapshot: SessionSnapshot = {
      task_id: request.task_id,
      session_id: request.session_id,
      status: status.state,
      turn_number: status.turn_number,
      spent_usd: status.spent_usd,
      created_at: 1_717_000_000_000,
      updated_at: 1_717_000_000_123,
      request,
      continuity,
      cost_accounting: status.cost_accounting,
    };

    expect(snapshot.request?.role_config?.knowledge_context).toBe("docs/review-playbook.md");
    expect(snapshot.continuity?.runtime).toBe("codex");
    expect(status.resume_ready).toBe(true);
    expect(status.cost_accounting?.mode).toBe("estimated_api_pricing");
    expect(catalog.runtimes[0]?.supportedFeatures).toContain("fork");
  });

  test("accepts opencode continuity payloads that preserve revertable message history", () => {
    const continuity: RuntimeContinuityState = {
      runtime: "opencode",
      resume_ready: false,
      captured_at: 1_717_000_100_000,
      blocking_reason: "missing_continuity_state",
      upstream_session_id: "opencode-session-1",
      latest_message_id: "assistant-22",
      server_url: "http://127.0.0.1:4096",
      fork_available: true,
      revert_message_ids: ["assistant-21", "assistant-22"],
    };

    expect(continuity.runtime).toBe("opencode");
    if (continuity.runtime !== "opencode") {
      throw new Error("Expected opencode continuity payload");
    }
    expect(continuity.revert_message_ids).toEqual(["assistant-21", "assistant-22"]);
  });
});
