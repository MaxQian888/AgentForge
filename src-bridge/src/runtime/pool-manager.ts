import { AgentRuntime } from "./agent-runtime.js";
import type { AgentStatus } from "../types.js";

export class RuntimePoolManager {
  private runtimes: Map<string, AgentRuntime> = new Map();
  private maxConcurrent: number;

  constructor(maxConcurrent = 10) {
    this.maxConcurrent = maxConcurrent;
  }

  acquire(taskId: string, sessionId: string): AgentRuntime {
    if (this.runtimes.has(taskId)) {
      throw new Error(`Runtime already exists for task ${taskId}`);
    }
    if (this.runtimes.size >= this.maxConcurrent) {
      throw new Error(
        `Pool at capacity (${this.maxConcurrent}). Cannot acquire runtime for task ${taskId}`,
      );
    }
    const runtime = new AgentRuntime(taskId, sessionId);
    this.runtimes.set(taskId, runtime);
    return runtime;
  }

  release(taskId: string): void {
    this.runtimes.delete(taskId);
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

  stats(): { active: number; max: number } {
    return { active: this.runtimes.size, max: this.maxConcurrent };
  }
}
