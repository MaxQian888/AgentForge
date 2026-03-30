"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import type { TaskStatus } from "@/lib/stores/task-store";
import type { TeamMember } from "@/lib/dashboard/summary";

const bulkStatuses: TaskStatus[] = [
  "inbox",
  "triaged",
  "assigned",
  "in_progress",
  "in_review",
  "done",
  "cancelled",
];

interface BulkActionToolbarProps {
  selectedCount: number;
  members: TeamMember[];
  onBulkStatusChange: (status: TaskStatus) => void;
  onBulkAssign: (assigneeId: string, assigneeType: "human" | "agent") => void;
  onBulkDelete: () => void;
  onClearSelection: () => void;
}

export function BulkActionToolbar({
  selectedCount,
  members,
  onBulkStatusChange,
  onBulkAssign,
  onBulkDelete,
  onClearSelection,
}: BulkActionToolbarProps) {
  const t = useTranslations("tasks");
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

  if (selectedCount === 0) return null;

  return (
    <div data-testid="bulk-action-toolbar" className="flex items-center gap-3 rounded-lg border bg-muted/50 px-4 py-2">
      <span className="text-sm font-medium">
        {t("bulk.selected", { count: selectedCount })}
      </span>

      <Select onValueChange={(value) => onBulkStatusChange(value as TaskStatus)}>
        <SelectTrigger className="h-8 w-[150px]">
          <SelectValue placeholder={t("bulk.changeStatus")} />
        </SelectTrigger>
        <SelectContent>
          {bulkStatuses.map((status) => (
            <SelectItem key={status} value={status}>
              {status.replace(/_/g, " ")}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        onValueChange={(value) => {
          const member = members.find((m) => m.id === value);
          if (member) {
            onBulkAssign(member.id, member.type);
          }
        }}
      >
        <SelectTrigger className="h-8 w-[150px]">
          <SelectValue placeholder={t("bulk.assignTo")} />
        </SelectTrigger>
        <SelectContent>
          {members
            .filter((m) => m.isActive)
            .map((member) => (
              <SelectItem key={member.id} value={member.id}>
                {member.name}
              </SelectItem>
            ))}
        </SelectContent>
      </Select>

      <Button
        type="button"
        size="sm"
        variant="destructive"
        onClick={() => setDeleteConfirmOpen(true)}
      >
        {t("bulk.deleteSelected")}
      </Button>

      <Button
        type="button"
        size="sm"
        variant="ghost"
        onClick={onClearSelection}
      >
        {t("bulk.clearSelection")}
      </Button>

      <ConfirmDialog
        open={deleteConfirmOpen}
        title={t("bulk.deleteConfirmTitle", { count: selectedCount })}
        description={t("bulk.deleteConfirmDescription")}
        confirmLabel={t("bulk.deleteConfirmLabel")}
        variant="destructive"
        onConfirm={() => {
          setDeleteConfirmOpen(false);
          onBulkDelete();
        }}
        onCancel={() => setDeleteConfirmOpen(false)}
      />
    </div>
  );
}
