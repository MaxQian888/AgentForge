"use client";

import type { ReactNode } from "react";
import {
  Bot,
  ClipboardCheck,
  DollarSign,
  ListTodo,
  MoreHorizontal,
  Radio,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import type { ProjectionStatus } from "./types";

function KindIcon({ kind }: { kind: string }) {
  const common = "size-4";
  switch (kind) {
    case "agent_run":
      return <Bot className={common} aria-hidden="true" />;
    case "cost_summary":
      return <DollarSign className={common} aria-hidden="true" />;
    case "review":
      return <ClipboardCheck className={common} aria-hidden="true" />;
    case "task_group":
      return <ListTodo className={common} aria-hidden="true" />;
    default:
      return <Radio className={common} aria-hidden="true" />;
  }
}

function StatusBanner({
  status,
  diagnostics,
}: {
  status: ProjectionStatus | undefined;
  diagnostics?: string;
}) {
  if (!status || status === "ok") return null;
  if (status === "not_found") {
    return (
      <div className="rounded-md border border-amber-400/30 bg-amber-400/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300">
        This live artifact is no longer available.
      </div>
    );
  }
  if (status === "forbidden") {
    return (
      <div
        className="rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground"
        role="status"
      >
        You do not have access to this live artifact.
      </div>
    );
  }
  // degraded
  return (
    <div
      className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive"
      role="status"
    >
      <div className="font-medium">Live update temporarily unavailable</div>
      {diagnostics ? (
        <div className="mt-1 text-[11px] opacity-80">{diagnostics}</div>
      ) : null}
    </div>
  );
}

export interface LiveArtifactChromeProps {
  kind: string;
  title: string;
  status?: ProjectionStatus;
  diagnostics?: string;
  onOpenSource: () => void;
  onFreeze: () => void;
  onRemove: () => void;
  children: ReactNode;
  className?: string;
}

export function LiveArtifactChrome({
  kind,
  title,
  status,
  diagnostics,
  onOpenSource,
  onFreeze,
  onRemove,
  children,
  className,
}: LiveArtifactChromeProps) {
  const canFreeze = status === "ok";
  return (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-xl border border-indigo-500/20 bg-indigo-500/5 p-4",
        className,
      )}
      data-live-artifact-kind={kind}
      data-live-artifact-status={status ?? "loading"}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="inline-flex items-center gap-2 rounded-full border border-indigo-500/30 bg-background px-2.5 py-1 text-xs font-medium text-indigo-700 dark:text-indigo-300">
          <KindIcon kind={kind} />
          <span>{title}</span>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-label="Live artifact actions"
              className="inline-flex size-7 items-center justify-center rounded-md border border-transparent text-muted-foreground hover:border-border hover:bg-accent"
            >
              <MoreHorizontal className="size-4" aria-hidden="true" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-[10rem]">
            <DropdownMenuItem onSelect={onOpenSource}>
              Open source
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={(event) => {
                if (!canFreeze) {
                  event.preventDefault();
                  return;
                }
                onFreeze();
              }}
              disabled={!canFreeze}
              aria-disabled={!canFreeze}
            >
              Freeze
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={onRemove} variant="destructive">
              Remove
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <StatusBanner status={status} diagnostics={diagnostics} />
      <div>{children}</div>
    </div>
  );
}
