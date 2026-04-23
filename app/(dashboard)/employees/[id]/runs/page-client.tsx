"use client";

import { useEffect } from "react";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
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

export default function EmployeeRunsPage() {
  const params = useParams<{ id: string }>();
  const employeeId = params.id;
  const t = useTranslations("employees");
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

  const kindFilters: { value: EmployeeRunKind; labelKey: string }[] = [
    { value: "all", labelKey: "all" },
    { value: "workflow", labelKey: "workflows" },
    { value: "agent", labelKey: "agents" },
  ];

  useEffect(() => {
    if (!employeeId) return;
    void fetchRuns(employeeId, 1, kind);
  }, [employeeId, kind, fetchRuns]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>{t("runHistory")}</CardTitle>
        <div className="flex gap-1">
          {kindFilters.map((f) => (
            <Button
              key={f.value}
              size="sm"
              variant={kind === f.value ? "default" : "outline"}
              onClick={() => fetchRuns(employeeId, 1, f.value)}
            >
              {t(f.labelKey)}
            </Button>
          ))}
        </div>
      </CardHeader>
      <CardContent className="p-0">
        {loading && rows.length === 0 ? (
          <div className="p-8 flex items-center justify-center text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin mr-2" />
            {t("loading")}
          </div>
        ) : rows.length === 0 ? (
          <EmptyState
            icon={Inbox}
            title={t("noRuns")}
            description={t("noRunsDescription")}
          />
        ) : (
          <>
            <div className="grid grid-cols-12 items-center gap-3 px-4 py-2 border-b bg-muted/40 text-xs font-medium uppercase text-muted-foreground">
              <div className="col-span-2">{t("colType")}</div>
              <div className="col-span-4">{t("colNameId")}</div>
              <div className="col-span-2">{t("colStatus")}</div>
              <div className="col-span-2">{t("colStartedAt")}</div>
              <div className="col-span-2 text-right">{t("colDuration")}</div>
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
                  {t("loadingMore")}
                </Button>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
