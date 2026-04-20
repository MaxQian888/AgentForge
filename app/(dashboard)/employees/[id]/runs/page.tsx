"use client";

import { useEffect } from "react";
import { useParams } from "next/navigation";
import { Loader2, Inbox } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { EmptyState } from "@/components/shared/empty-state";
import { EmployeeRunRow } from "@/components/employees/employee-run-row";
import {
  useEmployeeRunsStore,
  type EmployeeRunKind,
} from "@/lib/stores/employee-runs-store";

const KIND_FILTERS: { value: EmployeeRunKind; label: string }[] = [
  { value: "all", label: "All" },
  { value: "workflow", label: "Workflows" },
  { value: "agent", label: "Agents" },
];

export default function EmployeeRunsPage() {
  const params = useParams<{ id: string }>();
  const employeeId = params.id;
  const rows = useEmployeeRunsStore(
    (s) => s.runsByEmployee[employeeId] ?? [],
  );
  const loading = useEmployeeRunsStore(
    (s) => s.loadingByEmployee[employeeId] ?? false,
  );
  const page = useEmployeeRunsStore(
    (s) => s.pageByEmployee[employeeId] ?? 1,
  );
  const hasMore = useEmployeeRunsStore(
    (s) => s.hasMoreByEmployee[employeeId] ?? false,
  );
  const kind = useEmployeeRunsStore(
    (s) => s.kindByEmployee[employeeId] ?? "all",
  );
  const fetchRuns = useEmployeeRunsStore((s) => s.fetchRuns);

  useEffect(() => {
    if (!employeeId) return;
    void fetchRuns(employeeId, 1, kind);
  }, [employeeId, kind, fetchRuns]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>执行历史 (Runs)</CardTitle>
        <div className="flex gap-1">
          {KIND_FILTERS.map((f) => (
            <Button
              key={f.value}
              size="sm"
              variant={kind === f.value ? "default" : "outline"}
              onClick={() => fetchRuns(employeeId, 1, f.value)}
            >
              {f.label}
            </Button>
          ))}
        </div>
      </CardHeader>
      <CardContent className="p-0">
        {loading && rows.length === 0 ? (
          <div className="p-8 flex items-center justify-center text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin mr-2" />
            加载中...
          </div>
        ) : rows.length === 0 ? (
          <EmptyState
            icon={Inbox}
            title="暂无执行记录"
            description="该员工还没有驱动过任何 workflow 或 agent run。绑定 trigger 后即可在此看到回放。"
          />
        ) : (
          <>
            <div className="grid grid-cols-12 items-center gap-3 px-4 py-2 border-b bg-muted/40 text-xs font-medium uppercase text-muted-foreground">
              <div className="col-span-2">类型</div>
              <div className="col-span-4">名称 / ID</div>
              <div className="col-span-2">状态</div>
              <div className="col-span-2">开始时间</div>
              <div className="col-span-2 text-right">耗时</div>
            </div>
            {rows.map((row) => (
              <EmployeeRunRow key={`${row.kind}-${row.id}`} row={row} />
            ))}
            {hasMore && (
              <div className="p-3 border-t flex justify-center">
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={loading}
                  onClick={() => fetchRuns(employeeId, page + 1, kind)}
                >
                  {loading ? (
                    <Loader2 className="h-3 w-3 animate-spin mr-1" />
                  ) : null}
                  加载更多
                </Button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
