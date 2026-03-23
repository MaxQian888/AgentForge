import type { SessionSnapshot } from "../types.js";

export class SessionManager {
  private snapshots: Map<string, SessionSnapshot> = new Map();

  save(taskId: string, snapshot: SessionSnapshot): void {
    this.snapshots.set(taskId, { ...snapshot });
  }

  restore(taskId: string): SessionSnapshot | null {
    return this.snapshots.get(taskId) ?? null;
  }

  delete(taskId: string): void {
    this.snapshots.delete(taskId);
  }

  list(): SessionSnapshot[] {
    return Array.from(this.snapshots.values());
  }
}
