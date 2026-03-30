import { describe, expect, test } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { SessionManager } from "./manager.js";

describe("SessionManager", () => {
  test("saves, restores, lists, and deletes session snapshots", () => {
    const manager = new SessionManager();
    const snapshot = {
      task_id: "task-123",
      session_id: "session-123",
      status: "running",
      turn_number: 4,
      spent_usd: 0.12,
      created_at: 100,
      updated_at: 200,
    };

    manager.save("task-123", snapshot);
    const restored = manager.restore("task-123");

    expect(restored).toEqual(snapshot);
    expect(restored).not.toBe(snapshot);
    expect(manager.list()).toEqual([snapshot]);

    manager.delete("task-123");

    expect(manager.restore("task-123")).toBeNull();
    expect(manager.list()).toEqual([]);
  });

  test("persists snapshots to disk and reloads them for session resume", () => {
    const baseDir = mkdtempSync(join(tmpdir(), "agentforge-session-manager-"));

    try {
      const manager = new SessionManager({ baseDir });
      const snapshot = {
        task_id: "task-456",
        session_id: "session-456",
        status: "paused",
        turn_number: 7,
        spent_usd: 0.42,
        created_at: 100,
        updated_at: 200,
        request: {
          task_id: "task-456",
          session_id: "session-456",
          prompt: "Resume the task",
          worktree_path: "D:/Project/AgentForge",
          branch_name: "agent/task-456",
          system_prompt: "",
          max_turns: 12,
          budget_usd: 5,
          allowed_tools: ["Read"],
          permission_mode: "default",
          runtime: "codex" as const,
        },
      };

      manager.save("task-456", snapshot);

      const reloaded = new SessionManager({ baseDir });

      expect(reloaded.restore("task-456")).toMatchObject({
        ...snapshot,
        continuity: {
          runtime: "codex",
          resume_ready: false,
          blocking_reason: "missing_continuity_state",
        },
      });
      expect(reloaded.list()).toMatchObject([
        {
          ...snapshot,
          continuity: {
            runtime: "codex",
            resume_ready: false,
            blocking_reason: "missing_continuity_state",
          },
        },
      ]);
    } finally {
      rmSync(baseDir, { force: true, recursive: true });
    }
  });

  test("persists continuity metadata and resume readiness alongside legacy request snapshots", () => {
    const baseDir = mkdtempSync(join(tmpdir(), "agentforge-session-manager-"));

    try {
      const manager = new SessionManager({ baseDir });
      const snapshot = {
        task_id: "task-789",
        session_id: "session-789",
        status: "paused",
        turn_number: 5,
        spent_usd: 0.51,
        created_at: 100,
        updated_at: 400,
        request: {
          task_id: "task-789",
          session_id: "session-789",
          prompt: "Resume the Claude runtime",
          worktree_path: "D:/Project/AgentForge",
          branch_name: "agent/task-789",
          system_prompt: "",
          max_turns: 12,
          budget_usd: 5,
          allowed_tools: ["Read"],
          permission_mode: "default",
          runtime: "claude_code" as const,
          provider: "anthropic",
          model: "claude-sonnet-4-5",
        },
        continuity: {
          runtime: "claude_code" as const,
          resume_ready: true,
          captured_at: 350,
          session_handle: "claude-session-789",
          checkpoint_id: "checkpoint-1",
          resume_token: "resume-token-1",
          query_ref: "query-ref-1",
          fork_available: true,
        },
      };

      manager.save("task-789", snapshot);

      const reloaded = new SessionManager({ baseDir });

      expect(reloaded.restore("task-789")).toEqual(snapshot);
      expect(reloaded.list()).toEqual([snapshot]);
    } finally {
      rmSync(baseDir, { force: true, recursive: true });
    }
  });

  test("preserves opencode continuity metadata and upgrades legacy opencode snapshots to blocked continuity", () => {
    const baseDir = mkdtempSync(join(tmpdir(), "agentforge-session-manager-"));

    try {
      const manager = new SessionManager({ baseDir });
      manager.save("task-opencode-legacy", {
        task_id: "task-opencode-legacy",
        session_id: "session-opencode-legacy",
        status: "paused",
        turn_number: 1,
        spent_usd: 0.2,
        created_at: 100,
        updated_at: 200,
        request: {
          task_id: "task-opencode-legacy",
          session_id: "session-opencode-legacy",
          prompt: "Resume the OpenCode runtime",
          worktree_path: "D:/Project/AgentForge",
          branch_name: "agent/task-opencode-legacy",
          system_prompt: "",
          max_turns: 12,
          budget_usd: 5,
          allowed_tools: ["Read"],
          permission_mode: "default",
          runtime: "opencode" as const,
          provider: "opencode",
          model: "opencode-default",
        },
      });
      manager.save("task-opencode-bound", {
        task_id: "task-opencode-bound",
        session_id: "session-opencode-bound",
        status: "paused",
        turn_number: 3,
        spent_usd: 0.8,
        created_at: 100,
        updated_at: 400,
        request: {
          task_id: "task-opencode-bound",
          session_id: "session-opencode-bound",
          prompt: "Continue the OpenCode session",
          worktree_path: "D:/Project/AgentForge",
          branch_name: "agent/task-opencode-bound",
          system_prompt: "",
          max_turns: 12,
          budget_usd: 5,
          allowed_tools: ["Read"],
          permission_mode: "default",
          runtime: "opencode" as const,
          provider: "opencode",
          model: "opencode-default",
        },
        continuity: {
          runtime: "opencode" as const,
          resume_ready: true,
          captured_at: 350,
          upstream_session_id: "opencode-session-123",
          latest_message_id: "message-9",
          server_url: "http://127.0.0.1:4096",
          fork_available: true,
          revert_message_ids: ["message-9"],
        },
      });
      manager.save("task-codex-bound", {
        task_id: "task-codex-bound",
        session_id: "session-codex-bound",
        status: "paused",
        turn_number: 4,
        spent_usd: 0.6,
        created_at: 100,
        updated_at: 500,
        request: {
          task_id: "task-codex-bound",
          session_id: "session-codex-bound",
          prompt: "Resume the Codex runtime",
          worktree_path: "D:/Project/AgentForge",
          branch_name: "agent/task-codex-bound",
          system_prompt: "",
          max_turns: 12,
          budget_usd: 5,
          allowed_tools: ["Read"],
          permission_mode: "default",
          runtime: "codex" as const,
          provider: "openai",
          model: "gpt-5-codex",
        },
        continuity: {
          runtime: "codex" as const,
          resume_ready: true,
          captured_at: 450,
          thread_id: "thread-123",
          fork_available: true,
          rollback_turns: 3,
        },
      });

      const reloaded = new SessionManager({ baseDir });

      expect(reloaded.restore("task-opencode-legacy")).toMatchObject({
        continuity: {
          runtime: "opencode",
          resume_ready: false,
          blocking_reason: "missing_continuity_state",
        },
      });
      expect(reloaded.restore("task-opencode-bound")).toMatchObject({
        continuity: {
          runtime: "opencode",
          resume_ready: true,
          upstream_session_id: "opencode-session-123",
          latest_message_id: "message-9",
          server_url: "http://127.0.0.1:4096",
          fork_available: true,
          revert_message_ids: ["message-9"],
        },
      });
      expect(reloaded.restore("task-codex-bound")).toMatchObject({
        continuity: {
          runtime: "codex",
          resume_ready: true,
          thread_id: "thread-123",
          fork_available: true,
          rollback_turns: 3,
        },
      });
    } finally {
      rmSync(baseDir, { force: true, recursive: true });
    }
  });
});
