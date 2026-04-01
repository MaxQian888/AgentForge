"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { AgentPoolQueueEntry } from "@/lib/stores/agent-store";
import { priorityLabel } from "./agent-status-colors";

interface AgentPoolQueueTableProps {
  queue: AgentPoolQueueEntry[];
}

export function AgentPoolQueueTable({ queue }: AgentPoolQueueTableProps) {
  const t = useTranslations("agents");

  if (queue.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">{t("queue.empty")}</p>
    );
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("queue.task")}</TableHead>
            <TableHead>{t("queue.runtime")}</TableHead>
            <TableHead>{t("queue.priority")}</TableHead>
            <TableHead>{t("queue.status")}</TableHead>
            <TableHead>{t("queue.reason")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {queue.map((entry) => (
            <TableRow key={entry.entryId}>
              <TableCell className="font-medium">
                {entry.taskId}
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {entry.runtime || "-"}
                <div>{entry.provider || "-"}</div>
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {t(`priority.${priorityLabel(entry.priority)}`)}
              </TableCell>
              <TableCell>
                <Badge variant="secondary">{entry.status}</Badge>
              </TableCell>
              <TableCell className="text-xs text-muted-foreground">
                {entry.reason || "-"}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
