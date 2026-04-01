"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export interface DecomposeDialogBlock {
  id: string;
  text: string;
}

export interface DecomposeDialogTask {
  id: string;
  title: string;
}

export function DecomposeTasksDialog({
  open,
  onOpenChange,
  blocks,
  tasks,
  initialBlockIds = [],
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  blocks: DecomposeDialogBlock[];
  tasks: DecomposeDialogTask[];
  initialBlockIds?: string[];
  onConfirm: (input: { blockIds: string[]; parentTaskId?: string | null }) => void;
}) {
  const t = useTranslations("docs");
  const [selectedBlockIds, setSelectedBlockIds] = useState<string[]>(initialBlockIds);
  const [parentTaskId, setParentTaskId] = useState("");

  const allSelected = useMemo(
    () => blocks.length > 0 && selectedBlockIds.length === blocks.length,
    [blocks.length, selectedBlockIds.length],
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent key={initialBlockIds.join("|")}>
        <DialogHeader>
          <DialogTitle>{t("decompose.title")}</DialogTitle>
          <DialogDescription>
            {t("decompose.desc")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <label className="flex items-center gap-2 text-sm font-medium">
            <input
              type="checkbox"
              checked={allSelected}
              onChange={(event) =>
                setSelectedBlockIds(event.target.checked ? blocks.map((block) => block.id) : [])
              }
            />
            {t("decompose.selectAll")}
          </label>

          <div className="max-h-64 space-y-2 overflow-auto">
            {blocks.map((block) => (
              <label
                key={block.id}
                className="flex cursor-pointer items-start gap-3 rounded-lg border border-border/60 px-3 py-2"
              >
                <input
                  type="checkbox"
                  checked={selectedBlockIds.includes(block.id)}
                  onChange={(event) =>
                    setSelectedBlockIds((current) =>
                      event.target.checked
                        ? [...current, block.id]
                        : current.filter((id) => id !== block.id),
                    )
                  }
                />
                <div className="min-w-0">
                  <div className="font-medium">{block.id}</div>
                  <div className="text-xs text-muted-foreground">{block.text}</div>
                </div>
              </label>
            ))}
            {blocks.length === 0 ? (
              <div className="rounded-lg border border-dashed border-border/60 px-3 py-4 text-sm text-muted-foreground">
                {t("decompose.noBlocks")}
              </div>
            ) : null}
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-sm font-medium">{t("decompose.parentTask")}</label>
            <Select
              value={parentTaskId || "__none__"}
              onValueChange={(value) => setParentTaskId(value === "__none__" ? "" : value)}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">{t("decompose.createRootTasks")}</SelectItem>
                {tasks.map((task) => (
                  <SelectItem key={task.id} value={task.id}>
                    {task.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("decompose.cancel")}
          </Button>
          <Button
            onClick={() => onConfirm({ blockIds: selectedBlockIds, parentTaskId: parentTaskId || null })}
            disabled={selectedBlockIds.length === 0}
          >
            {t("decompose.createTasks")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
