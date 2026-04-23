"use client";

/**
 * TriggerListTable — read/edit row table for the per-employee Triggers tab
 * (Spec 1C). Each row exposes inline enable/disable, plus an actions menu
 * with Edit / Test / Delete (Delete is intercepted by AlertDialog and is
 * disabled for dag_node-owned rows since the backend will refuse them).
 */
import { useState, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { MoreHorizontal, Pencil, Play, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  useEmployeeTriggerStore,
} from "@/lib/stores/employee-trigger-store";
import type { WorkflowTrigger } from "@/lib/stores/workflow-trigger-store";

interface Props {
  employeeId: string;
  triggers: WorkflowTrigger[];
  onEdit: (t: WorkflowTrigger) => void;
  onTest: (t: WorkflowTrigger) => void;
}

export function TriggerListTable({ employeeId, triggers, onEdit, onTest }: Props) {
  const t = useTranslations("triggers.listTable");
  const patchTrigger = useEmployeeTriggerStore((s) => s.patchTrigger);
  const deleteTrigger = useEmployeeTriggerStore((s) => s.deleteTrigger);
  const [confirmDel, setConfirmDel] = useState<WorkflowTrigger | null>(null);

  if (triggers.length === 0) {
    return (
      <div className="text-center py-8">
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      </div>
    );
  }

  return (
    <>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("colName")}</TableHead>
            <TableHead>{t("colSource")}</TableHead>
            <TableHead>{t("colConfig")}</TableHead>
            <TableHead>{t("colEnabled")}</TableHead>
            <TableHead className="w-[60px]" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {triggers.map((trigger) => (
            <TableRow key={trigger.id} id={`trigger-${trigger.id}`}>
              <TableCell className="font-medium">
                {trigger.displayName?.trim() || <span className="text-muted-foreground">{t("unnamed")}</span>}
                {trigger.createdVia === "dag_node" ? (
                  <Badge variant="outline" className="ml-2 text-[10px]">{t("dagBadge")}</Badge>
                ) : null}
              </TableCell>
              <TableCell>
                <Badge variant={trigger.source === "im" ? "default" : "secondary"}>
                  {trigger.source}
                </Badge>
              </TableCell>
              <TableCell className="max-w-sm">
                <code className="text-xs bg-muted px-1 py-0.5 rounded block truncate">
                  {configSummary(trigger)}
                </code>
              </TableCell>
              <TableCell>
                <Switch
                  checked={trigger.enabled}
                  onCheckedChange={(enabled) => void patchTrigger(trigger.id, { enabled })}
                />
              </TableCell>
              <TableCell>
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="icon">
                      <MoreHorizontal className="h-4 w-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onClick={() => onEdit(trigger)}>
                      <Pencil className="h-4 w-4 mr-2" /> {t("actionEdit")}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => onTest(trigger)}>
                      <Play className="h-4 w-4 mr-2" /> {t("actionTest")}
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => setConfirmDel(trigger)}
                      disabled={trigger.createdVia === "dag_node"}
                      className="text-destructive"
                    >
                      <Trash2 className="h-4 w-4 mr-2" /> {t("actionDelete")}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <AlertDialog open={confirmDel !== null} onOpenChange={(open) => !open && setConfirmDel(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDesc", { name: confirmDel?.displayName ?? t("unnamed") })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("deleteCancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={async () => {
                if (confirmDel) {
                  await deleteTrigger(confirmDel.id, employeeId);
                  setConfirmDel(null);
                }
              }}
            >
              {t("deleteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

function configSummary(t: WorkflowTrigger): ReactNode {
  const cfg = t.config as Record<string, unknown>;
  if (t.source === "im") {
    const platform = (cfg.platform as string) ?? "?";
    const command = (cfg.command as string) ?? "";
    return `${platform} ${command}`.trim();
  }
  if (t.source === "schedule") {
    const cron = (cfg.cron as string) ?? "?";
    const tz = (cfg.timezone as string) ?? "UTC";
    return `${cron} (${tz})`;
  }
  return JSON.stringify(cfg);
}
