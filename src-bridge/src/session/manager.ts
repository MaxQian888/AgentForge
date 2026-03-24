import type { SessionSnapshot } from "../types.js";
import { existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";

interface SessionManagerOptions {
  baseDir?: string;
}

export class SessionManager {
  private snapshots: Map<string, SessionSnapshot> = new Map();
  private readonly baseDir?: string;

  constructor(options: SessionManagerOptions = {}) {
    this.baseDir = options.baseDir?.trim() || undefined;
    this.loadPersistedSnapshots();
  }

  save(taskId: string, snapshot: SessionSnapshot): void {
    this.snapshots.set(taskId, { ...snapshot });
    this.persist(taskId, snapshot);
  }

  restore(taskId: string): SessionSnapshot | null {
    const snapshot = this.snapshots.get(taskId);
    return snapshot ? { ...snapshot } : null;
  }

  delete(taskId: string): void {
    this.snapshots.delete(taskId);
    const filePath = this.filePath(taskId);
    if (filePath) {
      rmSync(filePath, { force: true });
    }
  }

  list(): SessionSnapshot[] {
    return Array.from(this.snapshots.values()).map((snapshot) => ({ ...snapshot }));
  }

  private loadPersistedSnapshots(): void {
    if (!this.baseDir || !existsSync(this.baseDir)) {
      return;
    }

    for (const name of readdirSync(this.baseDir)) {
      if (!name.endsWith(".json")) {
        continue;
      }

      const filePath = join(this.baseDir, name);

      try {
        const parsed = JSON.parse(readFileSync(filePath, "utf8")) as SessionSnapshot;
        if (parsed?.task_id) {
          this.snapshots.set(parsed.task_id, parsed);
        }
      } catch {
        continue;
      }
    }
  }

  private persist(taskId: string, snapshot: SessionSnapshot): void {
    const filePath = this.filePath(taskId);
    if (!filePath) {
      return;
    }

    mkdirSync(this.baseDir!, { recursive: true });
    writeFileSync(filePath, JSON.stringify(snapshot, null, 2), "utf8");
  }

  private filePath(taskId: string): string | undefined {
    if (!this.baseDir) {
      return undefined;
    }

    const safeTaskID = basename(`${taskId}.json`);
    return join(this.baseDir, safeTaskID);
  }
}
