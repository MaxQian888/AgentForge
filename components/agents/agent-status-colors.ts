import type { AgentStatus } from "@/lib/stores/agent-store";

export const statusColors: Record<AgentStatus, string> = {
  starting: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  running: "bg-green-500/15 text-green-700 dark:text-green-400",
  paused: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  completed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
  budget_exceeded: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
};

export const statusDotColors: Record<AgentStatus, string> = {
  starting: "bg-blue-500",
  running: "bg-green-500",
  paused: "bg-yellow-500",
  completed: "bg-blue-400",
  failed: "bg-red-500",
  cancelled: "bg-zinc-400",
  budget_exceeded: "bg-amber-500",
};

export function priorityLabel(
  priority?: number,
): "low" | "normal" | "high" | "critical" {
  switch (priority) {
    case 30:
      return "critical";
    case 20:
      return "high";
    case 10:
      return "normal";
    default:
      return "low";
  }
}
