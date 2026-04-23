"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const statusStyles: Record<string, string> = {
  succeeded: "bg-green-500/15 text-green-700 dark:text-green-400 border-green-500/20",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400 border-red-500/20",
  running: "bg-blue-500/15 text-blue-700 dark:text-blue-400 border-blue-500/20",
  cancel_requested: "bg-amber-500/15 text-amber-700 dark:text-amber-400 border-amber-500/20",
  cancelled: "bg-orange-500/15 text-orange-700 dark:text-orange-400 border-orange-500/20",
  pending: "bg-amber-500/15 text-amber-700 dark:text-amber-400 border-amber-500/20",
  skipped: "bg-muted text-muted-foreground border-border",
  paused: "bg-muted text-muted-foreground border-border",
  disabled: "bg-muted text-muted-foreground border-border",
  "never-run": "bg-muted text-muted-foreground border-border",
};

export function SchedulerStatusBadge({
  status,
  className,
}: {
  status: string | undefined;
  className?: string;
}) {
  const t = useTranslations("scheduler");
  const label = status ?? "never-run";
  return (
    <Badge
      variant="outline"
      className={cn(
        "text-xs font-medium capitalize",
        statusStyles[label] ?? statusStyles["never-run"],
        className,
      )}
    >
      {t(`statusLabels.${label}`)}
    </Badge>
  );
}
