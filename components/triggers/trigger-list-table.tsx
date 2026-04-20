"use client";

/**
 * TriggerListTable — read/edit row table for the per-employee Triggers tab
 * (Spec 1C). Each row exposes inline enable/disable, plus an actions menu
 * with Edit / Test / Delete (Delete is intercepted by AlertDialog and is
 * disabled for dag_node-owned rows since the backend will refuse them).
 */
import { useState, type ReactNode } from "react";
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
  const patchTrigger = useEmployeeTriggerStore((s) => s.patchTrigger);
  const deleteTrigger = useEmployeeTriggerStore((s) => s.deleteTrigger);
  const [confirmDel, setConfirmDel] = useState<WorkflowTrigger | null>(null);

  if (triggers.length === 0) {
    return (
      <div className="text-center py-8">
        <p className="text-sm text-muted-foreground">
          这个员工还没有任何触发器。点击右上角“新建触发器”以从 IM 命令或定时任务派发工作流。
        </p>
      </div>
    );
  }

  return (
    <>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>名称</TableHead>
            <TableHead>来源</TableHead>
            <TableHead>配置摘要</TableHead>
            <TableHead>启用</TableHead>
            <TableHead className="w-[60px]" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {triggers.map((t) => (
            <TableRow key={t.id} id={`trigger-${t.id}`}>
              <TableCell className="font-medium">
                {t.displayName?.trim() || <span className="text-muted-foreground">(unnamed)</span>}
                {t.createdVia === "dag_node" ? (
                  <Badge variant="outline" className="ml-2 text-[10px]">DAG</Badge>
                ) : null}
              </TableCell>
              <TableCell>
                <Badge variant={t.source === "im" ? "default" : "secondary"}>
                  {t.source}
                </Badge>
              </TableCell>
              <TableCell className="max-w-sm">
                <code className="text-xs bg-muted px-1 py-0.5 rounded block truncate">
                  {configSummary(t)}
                </code>
              </TableCell>
              <TableCell>
                <Switch
                  checked={t.enabled}
                  onCheckedChange={(enabled) => void patchTrigger(t.id, { enabled })}
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
                    <DropdownMenuItem onClick={() => onEdit(t)}>
                      <Pencil className="h-4 w-4 mr-2" /> 编辑
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => onTest(t)}>
                      <Play className="h-4 w-4 mr-2" /> 试运行
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      onClick={() => setConfirmDel(t)}
                      disabled={t.createdVia === "dag_node"}
                      className="text-destructive"
                    >
                      <Trash2 className="h-4 w-4 mr-2" /> 删除
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
            <AlertDialogTitle>确认删除触发器？</AlertDialogTitle>
            <AlertDialogDescription>
              {confirmDel?.displayName ?? "(unnamed)"} 将被永久删除。该操作不会影响已经派发的工作流执行记录。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={async () => {
                if (confirmDel) {
                  await deleteTrigger(confirmDel.id, employeeId);
                  setConfirmDel(null);
                }
              }}
            >
              删除
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
