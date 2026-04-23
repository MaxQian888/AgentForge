"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { EmployeeRunRow as Row } from "@/lib/stores/employee-runs-store";

const KIND_COLOR: Record<Row["kind"], string> = {
  workflow: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  agent: "bg-violet-500/15 text-violet-700 dark:text-violet-400",
};

const STATUS_COLOR: Record<string, string> = {
  pending: "bg-zinc-500/15 text-zinc-700 dark:text-zinc-400",
  running: "bg-blue-500/15 text-blue-700 dark:text-blue-400 animate-pulse",
  paused: "bg-amber-500/15 text-amber-700 dark:text-amber-400",
  completed: "bg-green-500/15 text-green-700 dark:text-green-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  cancelled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

function fmtDuration(ms: number, tEmployees: ReturnType<typeof useTranslations>): string {
  if (ms < 1000) return `${ms}${tEmployees("duration.ms")}`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}${tEmployees("duration.s")}`;
  const m = Math.floor(ms / 60_000);
  const s = Math.floor((ms % 60_000) / 1000);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function fmtTime(iso: string | undefined, emptyValue: string): string {
  if (!iso) return emptyValue;
  const t = new Date(iso);
  if (Number.isNaN(t.getTime())) return emptyValue;
  return t.toLocaleString();
}

export function EmployeeRunRow({ row }: { row: Row }) {
  const tEmployees = useTranslations("employees");
  const tAgents = useTranslations("agents");
  const emptyValue = tEmployees("emptyValue");

  return (
    <div className="grid grid-cols-12 items-center gap-3 px-4 py-3 border-b text-sm">
      <div className="col-span-2">
        <Badge className={cn("uppercase text-[10px]", KIND_COLOR[row.kind])}>
          {tEmployees(`kind.${row.kind}`)}
        </Badge>
      </div>
      <div className="col-span-4 truncate">
        <Link
          href={row.refUrl}
          className="font-medium hover:underline focus-visible:underline"
        >
          {row.name}
        </Link>
        <div className="text-xs text-muted-foreground truncate">{row.id}</div>
      </div>
      <div className="col-span-2">
        <Badge
          className={cn(
            "text-[11px]",
            STATUS_COLOR[row.status] ?? STATUS_COLOR.pending,
          )}
        >
          {tAgents(`status.${row.status}`)}
        </Badge>
      </div>
      <div className="col-span-2 text-muted-foreground text-xs">
        {fmtTime(row.startedAt, emptyValue)}
      </div>
      <div className="col-span-2 text-right tabular-nums">
        {row.durationMs === undefined || row.durationMs === null || !Number.isFinite(row.durationMs)
          ? emptyValue
          : fmtDuration(row.durationMs, tEmployees)}
      </div>
    </div>
  );
}
