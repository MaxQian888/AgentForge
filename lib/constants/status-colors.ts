export type StatusKey =
  | "active"
  | "running"
  | "idle"
  | "planning"
  | "pending"
  | "error"
  | "failed"
  | "warning"
  | "degraded"
  | "success"
  | "completed"
  | "healthy"
  | "executing"
  | "reviewing";

interface StatusColorEntry {
  dot: string;
  bg: string;
  text: string;
}

const STATUS_COLORS: Record<StatusKey, StatusColorEntry> = {
  active: {
    dot: "bg-emerald-500",
    bg: "bg-emerald-500/15",
    text: "text-emerald-700 dark:text-emerald-400",
  },
  running: {
    dot: "bg-emerald-500",
    bg: "bg-emerald-500/15",
    text: "text-emerald-700 dark:text-emerald-400",
  },
  executing: {
    dot: "bg-emerald-500",
    bg: "bg-emerald-500/15",
    text: "text-emerald-700 dark:text-emerald-400",
  },
  healthy: {
    dot: "bg-emerald-500",
    bg: "bg-emerald-500/15",
    text: "text-emerald-700 dark:text-emerald-400",
  },
  success: {
    dot: "bg-emerald-500",
    bg: "bg-emerald-500/15",
    text: "text-emerald-700 dark:text-emerald-400",
  },
  completed: {
    dot: "bg-zinc-400 dark:bg-zinc-500",
    bg: "bg-zinc-500/10",
    text: "text-zinc-600 dark:text-zinc-400",
  },
  idle: {
    dot: "bg-blue-500",
    bg: "bg-blue-500/15",
    text: "text-blue-700 dark:text-blue-400",
  },
  planning: {
    dot: "bg-blue-500",
    bg: "bg-blue-500/15",
    text: "text-blue-700 dark:text-blue-400",
  },
  reviewing: {
    dot: "bg-blue-500",
    bg: "bg-blue-500/15",
    text: "text-blue-700 dark:text-blue-400",
  },
  pending: {
    dot: "bg-zinc-300 dark:bg-zinc-600",
    bg: "bg-zinc-500/10",
    text: "text-zinc-600 dark:text-zinc-400",
  },
  error: {
    dot: "bg-red-500",
    bg: "bg-red-500/15",
    text: "text-red-700 dark:text-red-400",
  },
  failed: {
    dot: "bg-red-500",
    bg: "bg-red-500/15",
    text: "text-red-700 dark:text-red-400",
  },
  warning: {
    dot: "bg-amber-500",
    bg: "bg-amber-500/15",
    text: "text-amber-700 dark:text-amber-400",
  },
  degraded: {
    dot: "bg-amber-500",
    bg: "bg-amber-500/15",
    text: "text-amber-700 dark:text-amber-400",
  },
};

export function getStatusColor(status: string): StatusColorEntry {
  const key = status.toLowerCase() as StatusKey;
  return (
    STATUS_COLORS[key] ?? {
      dot: "bg-zinc-300 dark:bg-zinc-600",
      bg: "bg-zinc-500/10",
      text: "text-zinc-600 dark:text-zinc-400",
    }
  );
}

export { STATUS_COLORS };
