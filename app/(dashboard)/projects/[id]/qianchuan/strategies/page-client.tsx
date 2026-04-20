"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { Plus, ScrollText } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { EmptyState } from "@/components/shared/empty-state";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import {
  useQianchuanStrategiesStore,
  type StrategyStatus,
  type QianchuanStrategy,
} from "@/lib/stores/qianchuan-strategies-store";

type StatusFilter = StrategyStatus | "all";

const STATUS_FILTER_IDS: StatusFilter[] = ["all", "draft", "published", "archived"];

export default function QianchuanStrategiesListPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const projectId = params?.id ?? "";
  const t = useTranslations("qianchuan");
  useBreadcrumbs([
    { label: "Projects", href: "/projects" },
    { label: projectId, href: `/projects/${projectId}` },
    { label: t("title") },
  ]);

  const { strategies, loading, fetchList } = useQianchuanStrategiesStore();
  const [filter, setFilter] = useState<StatusFilter>("all");

  useEffect(() => {
    if (projectId) fetchList(projectId);
  }, [projectId, fetchList]);

  const visible = useMemo(() => {
    if (filter === "all") return strategies;
    return strategies.filter((row) => row.status === filter);
  }, [strategies, filter]);

  function statusBadge(status: StrategyStatus) {
    const label = t(`status.${status}` as `status.${StrategyStatus}`);
    switch (status) {
      case "draft":
        return <Badge variant="secondary">{label}</Badge>;
      case "published":
        return <Badge>{label}</Badge>;
      case "archived":
        return <Badge variant="outline">{label}</Badge>;
    }
  }

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader
        title={t("title")}
        description={t("description")}
        actions={
          <Button
            size="sm"
            onClick={() =>
              router.push(`/projects/${projectId}/qianchuan/strategies/new/edit`)
            }
          >
            <Plus className="mr-1 size-4" />
            {t("newStrategy")}
          </Button>
        }
      />

      <SectionCard title={t("list.title")} description={t("list.description")}>
        <div className="flex flex-wrap gap-2 px-4 pt-4">
          {STATUS_FILTER_IDS.map((id) => (
            <Button
              key={id}
              variant={filter === id ? "default" : "outline"}
              size="sm"
              onClick={() => setFilter(id)}
            >
              {t(`filters.${id}` as `filters.${StatusFilter}`)}
            </Button>
          ))}
        </div>

        {loading ? (
          <div className="flex flex-col gap-2 p-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        ) : visible.length === 0 ? (
          <EmptyState icon={ScrollText} title={t("list.empty")} />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("list.columnName")}</TableHead>
                <TableHead>{t("list.columnVersion")}</TableHead>
                <TableHead>{t("list.columnStatus")}</TableHead>
                <TableHead>{t("list.columnSource")}</TableHead>
                <TableHead className="w-[120px]">{t("list.columnActions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visible.map((row: QianchuanStrategy) => (
                <TableRow key={row.id}>
                  <TableCell className="font-mono text-sm">{row.name}</TableCell>
                  <TableCell>v{row.version}</TableCell>
                  <TableCell>{statusBadge(row.status)}</TableCell>
                  <TableCell>
                    {row.isSystem ? (
                      <Badge variant="outline">{t("status.system")}</Badge>
                    ) : (
                      <Badge variant="secondary">{t("status.project")}</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        router.push(`/projects/${projectId}/qianchuan/strategies/${row.id}/edit`)
                      }
                    >
                      {row.isSystem ? t("list.actionView") : t("list.actionEdit")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </SectionCard>
    </div>
  );
}
