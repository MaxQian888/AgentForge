"use client";

/**
 * Per-employee Triggers tab (Spec 1C). Lists all workflow_triggers whose
 * acting_employee_id matches the current employee. CRUD lives here; the
 * workflow editor's triggers section is now read-only with a link back
 * to this page.
 */
import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { TriggerListTable } from "@/components/triggers/trigger-list-table";
import { TriggerEditDrawer } from "@/components/triggers/trigger-edit-drawer";
import { TriggerTestModal } from "@/components/triggers/trigger-test-modal";
import { useEmployeeTriggerStore } from "@/lib/stores/employee-trigger-store";
import type { WorkflowTrigger } from "@/lib/stores/workflow-trigger-store";

export default function EmployeeTriggersPage() {
  const params = useParams<{ id: string }>();
  const employeeId = params?.id ?? "";
  const triggers = useEmployeeTriggerStore(
    (s) => s.triggersByEmployee[employeeId] ?? [],
  );
  const fetchByEmployee = useEmployeeTriggerStore((s) => s.fetchByEmployee);
  const loading = useEmployeeTriggerStore((s) => s.loading[employeeId] ?? false);

  const [editing, setEditing] = useState<{ open: boolean; trigger?: WorkflowTrigger | null }>(
    { open: false },
  );
  const [testing, setTesting] = useState<{ open: boolean; trigger?: WorkflowTrigger }>(
    { open: false },
  );

  useEffect(() => {
    if (employeeId) void fetchByEmployee(employeeId);
  }, [employeeId, fetchByEmployee]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>触发器 (Triggers)</CardTitle>
          <p className="text-sm text-muted-foreground mt-1">
            管理派发到该员工的 IM 命令 / 定时任务触发器。FE-authored triggers
            (createdVia=manual) 可任意编辑或删除；DAG 节点产出的触发器为只读。
          </p>
        </div>
        <Button
          onClick={() => setEditing({ open: true, trigger: null })}
          disabled={!employeeId}
        >
          <Plus className="h-4 w-4 mr-1" /> 新建触发器
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground">加载中…</p>
        ) : (
          <TriggerListTable
            employeeId={employeeId}
            triggers={triggers}
            onEdit={(t) => setEditing({ open: true, trigger: t })}
            onTest={(t) => setTesting({ open: true, trigger: t })}
          />
        )}
      </CardContent>
      <TriggerEditDrawer
        open={editing.open}
        employeeId={employeeId}
        trigger={editing.trigger}
        onClose={() => setEditing({ open: false })}
      />
      <TriggerTestModal
        open={testing.open}
        triggerId={testing.trigger?.id}
        initialSample={defaultSample(testing.trigger)}
        onClose={() => setTesting({ open: false })}
      />
    </Card>
  );
}

function defaultSample(t: WorkflowTrigger | undefined): string | undefined {
  if (!t) return undefined;
  if (t.source === "im") {
    const cfg = t.config as Record<string, unknown>;
    return JSON.stringify(
      {
        platform: (cfg.platform as string) ?? "feishu",
        command: (cfg.command as string) ?? "/echo",
        content: `${(cfg.command as string) ?? "/echo"} hello`,
        chat_id: "c-1",
        args: ["hello"],
      },
      null,
      2,
    );
  }
  if (t.source === "schedule") {
    return JSON.stringify(
      { now: new Date().toISOString(), cron_id: t.id },
      null,
      2,
    );
  }
  return undefined;
}
