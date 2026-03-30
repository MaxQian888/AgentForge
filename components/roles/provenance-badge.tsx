"use client";

import { cn } from "@/lib/utils";
import type { FieldProvenance } from "@/lib/roles/role-management";

const provenanceStyles: Record<FieldProvenance, string> = {
  inherited: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  template: "bg-purple-500/15 text-purple-700 dark:text-purple-400",
  explicit: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
};

interface ProvenanceBadgeProps {
  provenance: FieldProvenance;
  className?: string;
}

export function ProvenanceBadge({ provenance, className }: ProvenanceBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium leading-none",
        provenanceStyles[provenance],
        className,
      )}
    >
      {provenance}
    </span>
  );
}
