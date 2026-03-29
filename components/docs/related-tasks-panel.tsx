"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export interface RelatedTaskItem {
  linkId: string;
  taskId: string;
  title: string;
  status: string;
  assigneeName?: string | null;
  dueDate?: string | null;
}

export function RelatedTasksPanel({
  tasks,
  onAddTask,
  onRemoveTask,
}: {
  tasks: RelatedTaskItem[];
  onAddTask?: () => void;
  onRemoveTask?: (linkId: string) => void;
}) {
  const t = useTranslations("docs");

  return (
    <div className="rounded-xl border border-border/60 bg-card/70 p-4">
      <div className="flex items-center justify-between gap-2">
        <div>
          <h2 className="text-base font-semibold">{t("relatedTasks.title")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("relatedTasks.desc")}
          </p>
        </div>
        <Button type="button" size="sm" variant="outline" onClick={onAddTask}>
          {t("relatedTasks.linkTask")}
        </Button>
      </div>

      <div className="mt-3 space-y-3">
        {tasks.map((task) => (
          <div
            key={task.linkId}
            className="rounded-lg border border-border/60 bg-background px-3 py-2"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <Link href={`/project?taskId=${task.taskId}`} className="font-medium hover:underline">
                  {task.title}
                </Link>
                <div className="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
                  <Badge variant="outline">{task.status}</Badge>
                  {task.assigneeName ? <span>{task.assigneeName}</span> : null}
                  {task.dueDate ? <span>{task.dueDate}</span> : null}
                </div>
              </div>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                aria-label={`Remove ${task.title}`}
                onClick={() => onRemoveTask?.(task.linkId)}
              >
                {t("relatedTasks.remove")}
              </Button>
            </div>
          </div>
        ))}
        {tasks.length === 0 ? (
          <div className="rounded-lg border border-dashed border-border/60 px-3 py-4 text-sm text-muted-foreground">
            {t("relatedTasks.empty")}
          </div>
        ) : null}
      </div>
    </div>
  );
}
