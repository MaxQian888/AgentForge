"use client";

import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

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
          <DialogTitle>Create Tasks from Selection</DialogTitle>
          <DialogDescription>
            Choose one or more blocks and optionally attach them under an existing parent task.
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
            Select all blocks
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
                No blocks available to decompose.
              </div>
            ) : null}
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-sm font-medium">Parent task</label>
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={parentTaskId}
              onChange={(event) => setParentTaskId(event.target.value)}
            >
              <option value="">Create root backlog tasks</option>
              {tasks.map((task) => (
                <option key={task.id} value={task.id}>
                  {task.title}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={() => onConfirm({ blockIds: selectedBlockIds, parentTaskId: parentTaskId || null })}
            disabled={selectedBlockIds.length === 0}
          >
            Create Tasks
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
