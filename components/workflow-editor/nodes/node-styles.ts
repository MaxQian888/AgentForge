export interface NodeStyle {
  bg: string;
  border: string;
  iconColor: string;
}

export const NODE_STYLES: Record<string, NodeStyle> = {
  trigger: {
    bg: "bg-green-50 dark:bg-green-950",
    border: "border-green-400 dark:border-green-600",
    iconColor: "text-green-600 dark:text-green-400",
  },
  condition: {
    bg: "bg-amber-50 dark:bg-amber-950",
    border: "border-amber-400 dark:border-amber-600",
    iconColor: "text-amber-600 dark:text-amber-400",
  },
  agent_dispatch: {
    bg: "bg-blue-50 dark:bg-blue-950",
    border: "border-blue-400 dark:border-blue-600",
    iconColor: "text-blue-600 dark:text-blue-400",
  },
  notification: {
    bg: "bg-yellow-50 dark:bg-yellow-950",
    border: "border-yellow-400 dark:border-yellow-600",
    iconColor: "text-yellow-600 dark:text-yellow-400",
  },
  status_transition: {
    bg: "bg-purple-50 dark:bg-purple-950",
    border: "border-purple-400 dark:border-purple-600",
    iconColor: "text-purple-600 dark:text-purple-400",
  },
  gate: {
    bg: "bg-red-50 dark:bg-red-950",
    border: "border-red-400 dark:border-red-600",
    iconColor: "text-red-600 dark:text-red-400",
  },
  parallel_split: {
    bg: "bg-orange-50 dark:bg-orange-950",
    border: "border-orange-400 dark:border-orange-600",
    iconColor: "text-orange-600 dark:text-orange-400",
  },
  parallel_join: {
    bg: "bg-orange-50 dark:bg-orange-950",
    border: "border-orange-400 dark:border-orange-600",
    iconColor: "text-orange-600 dark:text-orange-400",
  },
  llm_agent: {
    bg: "bg-indigo-50 dark:bg-indigo-950",
    border: "border-indigo-400 dark:border-indigo-600",
    iconColor: "text-indigo-600 dark:text-indigo-400",
  },
  function: {
    bg: "bg-cyan-50 dark:bg-cyan-950",
    border: "border-cyan-400 dark:border-cyan-600",
    iconColor: "text-cyan-600 dark:text-cyan-400",
  },
  loop: {
    bg: "bg-pink-50 dark:bg-pink-950",
    border: "border-pink-400 dark:border-pink-600",
    iconColor: "text-pink-600 dark:text-pink-400",
  },
  human_review: {
    bg: "bg-emerald-50 dark:bg-emerald-950",
    border: "border-emerald-400 dark:border-emerald-600",
    iconColor: "text-emerald-600 dark:text-emerald-400",
  },
  wait_event: {
    bg: "bg-slate-50 dark:bg-slate-950",
    border: "border-slate-400 dark:border-slate-600",
    iconColor: "text-slate-600 dark:text-slate-400",
  },
  sub_workflow: {
    bg: "bg-violet-50 dark:bg-violet-950",
    border: "border-violet-400 dark:border-violet-600",
    iconColor: "text-violet-600 dark:text-violet-400",
  },
};

export const MINIMAP_COLORS: Record<string, string> = {
  trigger: "#22c55e",        // green-500
  condition: "#f59e0b",      // amber-500
  agent_dispatch: "#3b82f6", // blue-500
  notification: "#eab308",   // yellow-500
  status_transition: "#a855f7", // purple-500
  gate: "#ef4444",           // red-500
  parallel_split: "#f97316", // orange-500
  parallel_join: "#f97316",  // orange-500
  llm_agent: "#6366f1",      // indigo-500
  function: "#06b6d4",       // cyan-500
  loop: "#ec4899",           // pink-500
  human_review: "#10b981",   // emerald-500
  wait_event: "#64748b",     // slate-500
  sub_workflow: "#8b5cf6",   // violet-500
};
