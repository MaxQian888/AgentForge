"use client";

import { useEffect, useMemo, type ReactNode } from "react";
import { useTranslations } from "next-intl";
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

export function WorkflowTriggersSection({ workflowId, projectId }: WorkflowTriggersSectionProps) {
  const t = useTranslations("workflow");
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
          <CardTitle>{t("triggers.title")}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{t("triggers.selectWorkflow")}</p>
        </CardContent>
      </Card>
    );
  }

  const triggers = triggersByWorkflow[workflowId] ?? [];
  const isLoading = loading[workflowId] ?? false;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("triggers.title")}</CardTitle>
        <p className="text-sm text-muted-foreground mt-1">
          {t("triggers.description")}
        </p>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <p className="text-sm text-muted-foreground">{t("triggers.loading")}</p>
        ) : triggers.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-sm text-muted-foreground">
              {t("triggers.noTriggersDesc")}
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("triggers.table.source")}</TableHead>
                <TableHead>{t("triggers.table.engine")}</TableHead>
                <TableHead>{t("triggers.table.configSummary")}</TableHead>
                <TableHead>{t("triggers.table.actingEmployee")}</TableHead>
                <TableHead>{t("triggers.table.dedupeWindow")}</TableHead>
                <TableHead>{t("triggers.table.enabled")}</TableHead>
                <TableHead>{t("triggers.table.createdAt")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {triggers.map((tItem) => (
                <TableRow key={tItem.id}>
                  <TableCell>
                    <Badge variant={tItem.source === "im" ? "default" : "secondary"}>
                      {tItem.source}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant={tItem.targetKind === "plugin" ? "outline" : "default"}
                      title={tItem.targetKind === "plugin"
                        ? t("triggers.badge.pluginTitle")
                        : t("triggers.badge.dagTitle")}
                    >
                      {tItem.targetKind}
                    </Badge>
                  </TableCell>
                  <TableCell className="max-w-sm">
                    <code className="text-xs bg-muted px-1 py-0.5 rounded block truncate">
                      {configSummary(tItem, t)}
                    </code>
                    {tItem.disabledReason ? (
                      <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
                        {t("triggers.disabledPrefix")}{t(`triggers.disabledReason.${tItem.disabledReason}` as const) ?? tItem.disabledReason}
                      </p>
                    ) : null}
                  </TableCell>
                  <TableCell className="text-xs">
                    {renderActingEmployee(tItem, employeesByID, t)}
                  </TableCell>
                  <TableCell className="text-sm">
                    {tItem.dedupeWindowSeconds > 0 ? `${tItem.dedupeWindowSeconds}s` : "—"}
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={tItem.enabled}
                      onCheckedChange={(enabled) => setEnabled(workflowId, tItem.id, enabled)}
                    />
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {new Date(tItem.createdAt).toLocaleString()}
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
  tItem: WorkflowTrigger,
  employeesByID: Map<string, Employee>,
  t: ReturnType<typeof useTranslations>,
): ReactNode {
  if (!tItem.actingEmployeeId) {
    return <span className="text-muted-foreground">—</span>;
  }
  const emp = employeesByID.get(tItem.actingEmployeeId);
  if (emp) {
    return (
      <span title={tItem.actingEmployeeId}>
        {emp.displayName || emp.name}
        {emp.state !== "active" ? (
          <Badge variant="outline" className="ml-1 text-[10px]">
            {emp.state}
          </Badge>
        ) : null}
      </span>
    );
  }
  return <code className="text-[11px]">{tItem.actingEmployeeId.slice(0, 8)}…</code>;
}

function configSummary(tItem: WorkflowTrigger, t: ReturnType<typeof useTranslations>): string {
  const cfg = tItem.config as Record<string, unknown>;
  const target = tItem.targetKind === "plugin" && tItem.pluginId ? ` → ${tItem.pluginId}` : "";
  if (tItem.source === "im") {
    const platform = (cfg.platform as string) ?? t("triggers.config.unknown");
    const command = (cfg.command as string) ?? "";
    return `${platform} ${command}${target}`.trim();
  }
  if (tItem.source === "schedule") {
    const cron = (cfg.cron as string) ?? t("triggers.config.unknown");
    const tz = (cfg.timezone as string) ?? t("triggers.config.utc");
    return `${cron} (${tz})${target}`;
  }
  return JSON.stringify(cfg) + target;
}
