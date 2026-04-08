"use client";

import { cn } from "@/lib/utils";
import type { PluginKind } from "@/lib/stores/plugin-store";

const kindColors: Record<PluginKind, { bg: string; text: string }> = {
  ToolPlugin: { bg: "bg-blue-500/15", text: "text-blue-700 dark:text-blue-400" },
  RolePlugin: { bg: "bg-purple-500/15", text: "text-purple-700 dark:text-purple-400" },
  WorkflowPlugin: { bg: "bg-amber-500/15", text: "text-amber-700 dark:text-amber-400" },
  IntegrationPlugin: { bg: "bg-teal-500/15", text: "text-teal-700 dark:text-teal-400" },
  ReviewPlugin: { bg: "bg-rose-500/15", text: "text-rose-700 dark:text-rose-400" },
};

const sizeClasses = {
  sm: "size-8 text-xs rounded-md",
  md: "size-10 text-sm rounded-lg",
  lg: "size-12 text-base rounded-lg",
} as const;

interface PluginIconProps {
  name: string;
  kind: PluginKind;
  size?: keyof typeof sizeClasses;
  className?: string;
}

export function PluginIcon({ name, kind, size = "md", className }: PluginIconProps) {
  const colors = kindColors[kind] ?? kindColors.ToolPlugin;
  const initials = name
    .split(/[\s\-_]+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");

  return (
    <div
      className={cn(
        "flex shrink-0 items-center justify-center font-semibold select-none",
        colors.bg,
        colors.text,
        sizeClasses[size],
        className,
      )}
    >
      {initials || "P"}
    </div>
  );
}
