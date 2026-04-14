import { describe, expect, test } from "bun:test";
import {
  CancelRequestSchema,
  CommandRequestSchema,
  DecomposeTaskRequestSchema,
  DeepReviewRequestSchema,
  ExecuteRequestSchema,
  FileChangeEventDataSchema,
  ForkRequestSchema,
  InterruptRequestSchema,
  ModelSwitchRequestSchema,
  PartialMessageEventDataSchema,
  PermissionResponseSchema,
  ProgressEventDataSchema,
  RateLimitEventDataSchema,
  ReasoningEventDataSchema,
  RevertRequestSchema,
  RollbackRequestSchema,
  ResumeRequestSchema,
  ShellRequestSchema,
  ThinkingBudgetRequestSchema,
  TodoUpdateEventDataSchema,
  UnrevertRequestSchema,
} from "./schemas.js";

describe("bridge request schemas", () => {
  test("applies defaults for execute requests", () => {
    const parsed = ExecuteRequestSchema.parse({
      task_id: "task-123",
      session_id: "session-123",
      prompt: "Inspect the repository",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      budget_usd: 1,
    });

    expect(parsed.system_prompt).toBe("");
    expect(parsed.max_turns).toBe(50);
    expect(parsed.allowed_tools).toEqual([]);
    expect(parsed.permission_mode).toBe("default");
  });

  test("preserves optional provider and model fields for execute and decompose requests", () => {
    const execute = ExecuteRequestSchema.parse({
      task_id: "task-123",
      session_id: "session-123",
      prompt: "Inspect the repository",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      budget_usd: 1,
      runtime: "opencode",
      provider: "anthropic",
      model: "claude-sonnet-4-5",
    });

    const decompose = DecomposeTaskRequestSchema.parse({
      task_id: "task-123",
      title: "Split feature work",
      description: "Break this task down into focused subtasks.",
      priority: "high",
      provider: "openai",
      model: "gpt-5",
    });

    expect(execute.runtime).toBe("opencode");
    expect(execute.provider).toBe("anthropic");
    expect(execute.model).toBe("claude-sonnet-4-5");
    expect(decompose.provider).toBe("openai");
    expect(decompose.model).toBe("gpt-5");
  });

  test("preserves optional decomposition context payloads", () => {
    const decompose = DecomposeTaskRequestSchema.parse({
      task_id: "task-123",
      title: "Split feature work",
      description: "Break this task down into focused subtasks.",
      priority: "high",
      context: {
        relevantFiles: ["src-go/internal/server/routes.go"],
        waveMode: true,
      },
    });

    expect(decompose.context).toEqual({
      relevantFiles: ["src-go/internal/server/routes.go"],
      waveMode: true,
    });
  });

  test("rejects execute payloads with unknown runtime keys", () => {
    expect(
      ExecuteRequestSchema.safeParse({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        budget_usd: 1,
        runtime: "made_up_runtime",
      }).success,
    ).toBe(false);
  });

  test("accepts additional CLI-backed runtime keys", () => {
    for (const runtime of ["cursor", "gemini", "qoder", "iflow"] as const) {
      const parsed = ExecuteRequestSchema.parse({
        task_id: `task-${runtime}`,
        session_id: `session-${runtime}`,
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: `agent/task-${runtime}`,
        budget_usd: 1,
        runtime,
      });

      expect(parsed.runtime).toBe(runtime);
    }
  });

  test("accepts normalized role execution profiles and rejects raw YAML-shaped role payloads", () => {
    const parsed = ExecuteRequestSchema.parse({
      task_id: "task-123",
      session_id: "session-123",
      prompt: "Inspect the repository",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      budget_usd: 1,
      role_config: {
        role_id: "frontend-developer",
        name: "Frontend Developer",
        role: "Senior Frontend Developer",
        goal: "Build reliable UI",
        backstory: "A frontend specialist",
        system_prompt: "You build safe UI.",
        allowed_tools: ["Read", "Edit"],
        max_budget_usd: 5,
        max_turns: 20,
        permission_mode: "default",
        tools: ["github-tool"],
        plugin_bindings: [{ plugin_id: "github-tool", functions: ["search", "open_file"] }],
        knowledge_context: "docs/PRD.md",
        loaded_skills: [
          {
            path: "skills/react",
            label: "React",
            description: "React UI implementation guidance",
            instructions: "Prefer server-safe React composition.",
            display_name: "React Workspace",
            short_description: "Guide React work in this repository.",
            default_prompt: "Use $react to implement the current React seam.",
            available_parts: ["agents", "references"],
            reference_count: 1,
            source: "repo-local",
            source_root: "skills",
            origin: "direct",
            requires: ["skills/typescript"],
          },
        ],
        available_skills: [
          {
            path: "skills/testing",
            label: "Testing",
            description: "Regression-oriented test guidance",
            available_parts: ["agents"],
            source: "repo-local",
            source_root: "skills",
            origin: "direct",
          },
        ],
        skill_diagnostics: [],
        output_filters: ["no_pii"],
      },
      team_id: "team-123",
      team_role: "planner",
    });

    expect(parsed.role_config?.role_id).toBe("frontend-developer");
    expect(parsed.role_config?.tools).toEqual(["github-tool"]);
    expect(parsed.role_config?.plugin_bindings).toEqual([
      { plugin_id: "github-tool", functions: ["search", "open_file"] },
    ]);
    expect(parsed.role_config?.knowledge_context).toBe("docs/PRD.md");
    expect(parsed.role_config?.loaded_skills?.[0]?.path).toBe("skills/react");
    expect(parsed.role_config?.loaded_skills?.[0]?.display_name).toBe("React Workspace");
    expect(parsed.role_config?.loaded_skills?.[0]?.available_parts).toEqual(["agents", "references"]);
    expect(parsed.role_config?.available_skills?.[0]?.path).toBe("skills/testing");
    expect(parsed.role_config?.output_filters).toEqual(["no_pii"]);
    expect(parsed.team_id).toBe("team-123");
    expect(parsed.team_role).toBe("planner");
    expect(
      ExecuteRequestSchema.safeParse({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        budget_usd: 1,
        role_config: {
          metadata: { id: "frontend-developer", name: "Frontend Developer" },
          security: { permission_mode: "default" },
        },
      }).success,
    ).toBe(false);
  });

  test("rejects unsupported team roles", () => {
    expect(
      ExecuteRequestSchema.safeParse({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        budget_usd: 1,
        team_id: "team-123",
        team_role: "lead",
      }).success,
    ).toBe(false);
  });

  test("accepts bounded team execution context on resume payloads", () => {
    const parsed = ResumeRequestSchema.parse({
      task_id: "task-123",
      session_id: "session-123",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      prompt: "Resume the runtime",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-123",
      budget_usd: 1,
      team_id: "team-123",
      team_role: "reviewer",
      role_config: {
        role_id: "code-reviewer",
        name: "Code Reviewer",
        role: "Senior Reviewer",
        goal: "Find risky changes",
        backstory: "A skeptical reviewer",
        system_prompt: "Review carefully.",
        allowed_tools: ["Read"],
        max_budget_usd: 3,
        max_turns: 12,
        permission_mode: "default",
        tools: ["github-tool"],
        knowledge_context: "docs/PRD.md",
        loaded_skills: [],
        available_skills: [],
        skill_diagnostics: [],
        output_filters: ["no_pii"],
      },
    });

    expect(parsed.team_id).toBe("team-123");
    expect(parsed.team_role).toBe("reviewer");
    expect(parsed.role_config?.role_id).toBe("code-reviewer");
  });

  test("accepts advanced bridge execution options and route payloads", () => {
    const parsed = ExecuteRequestSchema.parse({
      task_id: "task-advanced",
      session_id: "session-advanced",
      runtime: "claude_code",
      provider: "anthropic",
      model: "claude-opus-4-1",
      prompt: "Run an advanced bridge task",
      worktree_path: "D:/Project/AgentForge",
      branch_name: "agent/task-advanced",
      budget_usd: 2,
      thinking_config: {
        enabled: true,
        budget_tokens: 10_000,
      },
      output_schema: {
        type: "json_schema",
        schema: {
          type: "object",
          properties: {
            summary: { type: "string" },
          },
        },
      },
      hooks_config: {
        hooks: [
          {
            hook: "PreToolUse",
            matcher: {
              tools: ["Read"],
            },
          },
          {
            hook: "SessionStart",
          },
          {
            hook: "Notification",
          },
        ],
        callback_url: "http://127.0.0.1:7777/hooks",
        timeout_ms: 4000,
      },
      hook_callback_url: "http://127.0.0.1:7777/hooks",
      hook_timeout_ms: 4000,
      attachments: [
        {
          type: "image",
          path: "D:/Project/AgentForge/tmp/screenshot.png",
          mime_type: "image/png",
        },
      ],
      file_checkpointing: true,
      agents: {
        reviewer: {
          description: "Review risky changes",
          prompt: "Act as a skeptical reviewer.",
          tools: ["Read", "Grep"],
          model: "claude-sonnet-4-5",
        },
      },
      disallowed_tools: ["Bash"],
      fallback_model: "claude-sonnet-4-5",
      additional_directories: ["D:/Shared"],
      include_partial_messages: true,
      tool_permission_callback: true,
      web_search: true,
      env: {
        FEATURE_FLAG: "enabled",
      },
    });

    expect(parsed.thinking_config).toEqual({
      enabled: true,
      budget_tokens: 10_000,
    });
    expect(parsed.output_schema?.type).toBe("json_schema");
    expect(parsed.hooks_config?.hooks[0]?.hook).toBe("PreToolUse");
    expect(parsed.hooks_config?.hooks[1]?.hook).toBe("SessionStart");
    expect(parsed.hooks_config?.hooks[2]?.hook).toBe("Notification");
    expect(parsed.attachments?.[0]?.type).toBe("image");
    expect(parsed.agents?.reviewer?.tools).toEqual(["Read", "Grep"]);
    expect(parsed.additional_directories).toEqual(["D:/Shared"]);
    expect(parsed.env).toEqual({ FEATURE_FLAG: "enabled" });

    expect(
      ForkRequestSchema.parse({
        task_id: "task-advanced",
        message_id: "message-1",
      }),
    ).toEqual({
      task_id: "task-advanced",
      message_id: "message-1",
    });
    expect(
      RollbackRequestSchema.parse({
        task_id: "task-advanced",
        checkpoint_id: "checkpoint-1",
        turns: 2,
      }),
    ).toEqual({
      task_id: "task-advanced",
      checkpoint_id: "checkpoint-1",
      turns: 2,
    });
    expect(
      RevertRequestSchema.parse({
        task_id: "task-advanced",
        message_id: "message-1",
      }),
    ).toEqual({
      task_id: "task-advanced",
      message_id: "message-1",
    });
    expect(
      UnrevertRequestSchema.parse({
        task_id: "task-advanced",
      }),
    ).toEqual({
      task_id: "task-advanced",
    });
    expect(
      CommandRequestSchema.parse({
        task_id: "task-advanced",
        command: "/compact",
        arguments: "--full",
      }),
    ).toEqual({
      task_id: "task-advanced",
      command: "/compact",
      arguments: "--full",
    });
    expect(
      ShellRequestSchema.parse({
        task_id: "task-advanced",
        command: "pnpm lint",
        agent: "reviewer",
        model: "opencode-fast",
      }),
    ).toEqual({
      task_id: "task-advanced",
      command: "pnpm lint",
      agent: "reviewer",
      model: "opencode-fast",
    });
    expect(
      InterruptRequestSchema.parse({
        task_id: "task-advanced",
      }),
    ).toEqual({
      task_id: "task-advanced",
    });
    expect(
      ThinkingBudgetRequestSchema.parse({
        task_id: "task-advanced",
        max_thinking_tokens: 2_048,
      }),
    ).toEqual({
      task_id: "task-advanced",
      max_thinking_tokens: 2_048,
    });
    expect(
      ModelSwitchRequestSchema.parse({
        task_id: "task-advanced",
        model: "claude-haiku-4-5",
      }),
    ).toEqual({
      task_id: "task-advanced",
      model: "claude-haiku-4-5",
    });
    expect(
      PermissionResponseSchema.parse({
        decision: "allow",
        reason: "approved",
      }),
    ).toEqual({
      decision: "allow",
      reason: "approved",
    });
  });

  test("validates new advanced event payload schemas", () => {
    expect(
      ReasoningEventDataSchema.parse({
        content: "Considering the next step.",
      }),
    ).toEqual({
      content: "Considering the next step.",
    });
    expect(
      FileChangeEventDataSchema.parse({
        files: [
          {
            path: "src/index.ts",
            change_type: "modified",
          },
        ],
      }),
    ).toEqual({
      files: [
        {
          path: "src/index.ts",
          change_type: "modified",
        },
      ],
    });
    expect(
      TodoUpdateEventDataSchema.parse({
        todos: [
          {
            id: "todo-1",
            content: "Finish the shared runtime routes",
            status: "in_progress",
          },
        ],
      }),
    ).toEqual({
      todos: [
        {
          id: "todo-1",
          content: "Finish the shared runtime routes",
          status: "in_progress",
        },
      ],
    });
    expect(
      ProgressEventDataSchema.parse({
        tool_name: "Read",
        progress_text: "Scanning the bridge sources",
      }),
    ).toEqual({
      tool_name: "Read",
      progress_text: "Scanning the bridge sources",
    });
    expect(
      RateLimitEventDataSchema.parse({
        utilization: 0.82,
        reset_at: "2026-03-30T10:00:00.000Z",
      }),
    ).toEqual({
      utilization: 0.82,
      reset_at: "2026-03-30T10:00:00.000Z",
    });
    expect(
      PartialMessageEventDataSchema.parse({
        content: "Partial response",
        is_complete: false,
      }),
    ).toEqual({
      content: "Partial response",
      is_complete: false,
    });
  });

  test("rejects invalid cancel and review payloads", () => {
    expect(CancelRequestSchema.safeParse({ task_id: "" }).success).toBe(false);
    expect(
      DeepReviewRequestSchema.safeParse({
        review_id: "review-123",
        task_id: "task-123",
        pr_url: "https://example.com/pr/123",
        pr_number: -1,
      }).success,
    ).toBe(false);
    expect(
      ExecuteRequestSchema.safeParse({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        budget_usd: 1,
        thinking_config: {
          enabled: true,
          budget_tokens: 0,
        },
      }).success,
    ).toBe(false);
    expect(
      ExecuteRequestSchema.safeParse({
        task_id: "task-123",
        session_id: "session-123",
        prompt: "Inspect the repository",
        worktree_path: "D:/Project/AgentForge",
        branch_name: "agent/task-123",
        budget_usd: 1,
        attachments: [
          {
            type: "video",
            path: "D:/Project/AgentForge/tmp/demo.mp4",
          },
        ],
      }).success,
    ).toBe(false);
    expect(PermissionResponseSchema.safeParse({ decision: "maybe" }).success).toBe(false);
    expect(RevertRequestSchema.safeParse({ task_id: "task-123" }).success).toBe(false);
  });
});
