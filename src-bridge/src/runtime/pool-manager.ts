import { AgentRuntime } from "./agent-runtime.js";
import type { AgentRuntimeKey, AgentStatus, RuntimePoolSummary } from "../types.js";

export class RuntimePoolManager {
  private runtimes: Map<string, AgentRuntime> = new Map();
  private maxConcurrent: number;
  private warmAvailable = 0;
  private warmTotal = 0;
  private warmReuseHits = 0;
  private coldStarts = 0;
  private lastReconcileAt = Date.now();

  constructor(maxConcurrent = 10) {
    this.maxConcurrent = maxConcurrent;
  }

  acquire(taskId: string, sessionId: string, runtimeKey: AgentRuntimeKey = "claude_code"): AgentRuntime {
    if (this.runtimes.has(taskId)) {
      throw new Error(`Runtime already exists for task ${taskId}`);
    }
    if (this.runtimes.size >= this.maxConcurrent) {
      throw new Error(
        `Pool at capacity (${this.maxConcurrent}). Cannot acquire runtime for task ${taskId}`,
      );
    }
    const runtime = new AgentRuntime(taskId, sessionId);
    runtime.bindRequest({
      task_id: taskId,
      session_id: sessionId,
      runtime: runtimeKey,
      prompt: "",
      worktree_path: "",
      branch_name: "",
      system_prompt: "",
      max_turns: 0,
      budget_usd: 0,
      allowed_tools: [],
      permission_mode: "default",
    });
    if (this.warmAvailable > 0) {
      this.warmAvailable -= 1;
      this.warmReuseHits += 1;
    } else {
      this.coldStarts += 1;
    }
    this.lastReconcileAt = Date.now();
    this.runtimes.set(taskId, runtime);
    return runtime;
  }

  release(taskId: string): void {
    const runtime = this.runtimes.get(taskId);
    this.runtimes.delete(taskId);
    if (runtime) {
      this.warmAvailable = Math.min(this.warmAvailable + 1, this.maxConcurrent);
      this.warmTotal = Math.max(this.warmTotal, this.warmAvailable);
      this.lastReconcileAt = Date.now();
    }
  }

  get(taskId: string): AgentRuntime | undefined {
    return this.runtimes.get(taskId);
  }

  listActive(): AgentStatus[] {
    return Array.from(this.runtimes.values()).map((r) => r.toStatus());
  }

  /** Return all runtime objects (for graceful shutdown snapshot saving). */
  listRuntimes(): AgentRuntime[] {
    return Array.from(this.runtimes.values());
  }

  stats(): RuntimePoolSummary {
    return {
      active: this.runtimes.size,
      max: this.maxConcurrent,
      warm_total: this.warmTotal,
      warm_available: this.warmAvailable,
      warm_reuse_hits: this.warmReuseHits,
      cold_starts: this.coldStarts,
      last_reconcile_at: this.lastReconcileAt,
      degraded: false,
    };
  }
}
