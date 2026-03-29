"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export interface TaskLinkPickerItem {
  id: string;
  title: string;
  status: string;
}

export function TaskLinkPicker({
  open,
  onOpenChange,
  tasks,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tasks: TaskLinkPickerItem[];
  onPick: (taskId: string) => void;
}) {
  const t = useTranslations("docs");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("taskLinkPicker.title")}</DialogTitle>
          <DialogDescription>
            {t("taskLinkPicker.desc")}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          {tasks.map((task) => (
            <button
              key={task.id}
              type="button"
              className="rounded-lg border border-border/60 px-4 py-3 text-left hover:bg-accent/40"
              onClick={() => onPick(task.id)}
            >
              <div className="font-medium">{task.title}</div>
              <div className="text-xs text-muted-foreground">{task.status}</div>
            </button>
          ))}
          {tasks.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border/70 p-4 text-sm text-muted-foreground">
              {t("taskLinkPicker.noTasks")}
            </div>
          ) : null}
        </div>
        <div className="flex justify-end">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("taskLinkPicker.close")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
