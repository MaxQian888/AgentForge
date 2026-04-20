"use client";

import { useEffect, useMemo, type ReactNode } from "react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  useWorkflowTriggerStore,
  type WorkflowTrigger,
} from "@/lib/stores/workflow-trigger-store";
import { useEmployeeStore, type Employee } from "@/lib/stores/employee-store";

interface WorkflowTriggersSectionProps {
  workflowId: string | null;
  projectId?: string | null;
}

// Machine-readable disabled-reason codes emitted by the trigger registrar and
// surfaced on WorkflowTrigger.disabledReason. Keep in sync with
// src-go/internal/trigger/registrar.go DisabledReason* constants.
const DISABLED_REASON_LABELS: Record<string, string> = {
  dag_workflow_not_found: "DAG 工作流未找到",
  dag_workflow_inactive: "DAG 工作流非激活",
  plugin_not_found: "插件未找到",
  plugin_disabled: "插件已禁用",
  plugin_not_workflow_kind: "插件不是工作流类型",
  plugin_process_not_executable: "插件流程不可执行",
  plugin_target_missing_plugin_id: "插件目标缺少 plugin_id",
  acting_employee_not_found: "数字员工未找到",
  acting_employee_cross_project: "数字员工跨项目引用",
  acting_employee_archived: "数字员工已归档",
};

export function WorkflowTriggersSection({ workflowId, projectId }: WorkflowTriggersSectionProps) {
  const triggersByWorkflow = useWorkflowTriggerStore((s) => s.triggersByWorkflow);
  const loading = useWorkflowTriggerStore((s) => s.loading);
  const fetchTriggers = useWorkflowTriggerStore((s) => s.fetchTriggers);
  const setEnabled = useWorkflowTriggerStore((s) => s.setEnabled);

  const employeesByProject = useEmployeeStore((s) => s.employeesByProject);
  const fetchEmployees = useEmployeeStore((s) => s.fetchEmployees);

  useEffect(() => {
    if (workflowId) void fetchTriggers(workflowId);
  }, [workflowId, fetchTriggers]);

  useEffect(() => {
    if (projectId) void fetchEmployees(projectId);
  }, [projectId, fetchEmployees]);

  const employeesByID = useMemo(() => {
    if (!projectId) return new Map<string, Employee>();
    const rows = employeesByProject[projectId] ?? [];
    return new Map(rows.map((e) => [e.id, e]));
  }, [employeesByProject, projectId]);

  if (!workflowId) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>触发器 (Triggers)</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">请选择工作流以查看触发器。</p>
        </CardContent>
      </Card>
    );
  }

  const triggers = triggersByWorkflow[workflowId] ?? [];
  const isLoading = loading[workflowId] ?? false;

  return (
    <Card>
      <CardHeader>
        <CardTitle>触发器 (Triggers)</CardTitle>
        <p className="text-sm text-muted-foreground mt-1">
          这里展示工作流 DAG 中定义的触发器节点的运行时状态。保存工作流时会自动同步。
          可以临时停用单个触发器而无需编辑 DAG。
        </p>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <p className="text-sm text-muted-foreground">加载中...</p>
        ) : triggers.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-sm text-muted-foreground">
              此工作流没有注册任何触发器。在 DAG 编辑器里给 trigger 节点配置 source=im 或 schedule 后保存即可。
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Source</TableHead>
                <TableHead>Engine</TableHead>
                <TableHead>配置摘要</TableHead>
                <TableHead>扮演员工</TableHead>
                <TableHead>幂等窗口</TableHead>
                <TableHead>启用</TableHead>
                <TableHead>创建时间</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {triggers.map((t) => (
                <TableRow key={t.id}>
                  <TableCell>
                    <Badge variant={t.source === "im" ? "default" : "secondary"}>
                      {t.source}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={t.targetKind === "plugin" ? "outline" : "default"}
                      title={t.targetKind === "plugin"
                        ? "Fires a legacy workflow plugin run"
                        : "Fires a DAG workflow execution"}
                    >
                      {t.targetKind}
                    </Badge>
                  </TableCell>
                  <TableCell className="max-w-sm">
                    <code className="text-xs bg-muted px-1 py-0.5 rounded block truncate">
                      {configSummary(t)}
                    </code>
                    {t.disabledReason ? (
                      <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                        触发器被禁用：{DISABLED_REASON_LABELS[t.disabledReason] ?? t.disabledReason}
                      </p>
                    ) : null}
                  </TableCell>
                  <TableCell className="text-xs">
                    {renderActingEmployee(t, employeesByID)}
                  </TableCell>
                  <TableCell className="text-sm">
                    {t.dedupeWindowSeconds > 0 ? `${t.dedupeWindowSeconds}s` : "—"}
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={t.enabled}
                      onCheckedChange={(enabled) => setEnabled(workflowId, t.id, enabled)}
                    />
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {new Date(t.createdAt).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

function renderActingEmployee(
  t: WorkflowTrigger,
  employeesByID: Map<string, Employee>,
): ReactNode {
  if (!t.actingEmployeeId) {
    return <span className="text-muted-foreground">—</span>;
  }
  const emp = employeesByID.get(t.actingEmployeeId);
  if (emp) {
    return (
      <span title={t.actingEmployeeId}>
        {emp.displayName || emp.name}
        {emp.state !== "active" ? (
          <Badge variant="outline" className="ml-1 text-[10px]">
            {emp.state}
          </Badge>
        ) : null}
      </span>
    );
  }
  return <code className="text-[11px]">{t.actingEmployeeId.slice(0, 8)}…</code>;
}

function configSummary(t: WorkflowTrigger): string {
  const cfg = t.config as Record<string, unknown>;
  const target = t.targetKind === "plugin" && t.pluginId ? ` → ${t.pluginId}` : "";
  if (t.source === "im") {
    const platform = (cfg.platform as string) ?? "?";
    const command = (cfg.command as string) ?? "";
    return `${platform} ${command}${target}`.trim();
  }
  if (t.source === "schedule") {
    const cron = (cfg.cron as string) ?? "?";
    const tz = (cfg.timezone as string) ?? "UTC";
    return `${cron} (${tz})${target}`;
  }
  return JSON.stringify(cfg) + target;
}
