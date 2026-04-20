"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
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

const STATUS_FILTERS: { id: StatusFilter; label: string }[] = [
  { id: "all", label: "全部" },
  { id: "draft", label: "草稿" },
  { id: "published", label: "已发布" },
  { id: "archived", label: "已归档" },
];

function statusBadge(status: StrategyStatus) {
  switch (status) {
    case "draft":
      return <Badge variant="secondary">草稿</Badge>;
    case "published":
      return <Badge>已发布</Badge>;
    case "archived":
      return <Badge variant="outline">已归档</Badge>;
  }
}

export default function QianchuanStrategiesListPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const projectId = params?.id ?? "";
  useBreadcrumbs([
    { label: "Projects", href: "/projects" },
    { label: projectId, href: `/projects/${projectId}` },
    { label: "Qianchuan Strategies" },
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

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader
        title="千川策略库"
        description="声明式策略 — YAML 定义触发节奏、规则和动作。系统策略只读。"
        actions={
          <Button
            size="sm"
            onClick={() =>
              router.push(`/projects/${projectId}/qianchuan/strategies/new/edit`)
            }
          >
            <Plus className="mr-1 size-4" />
            新建策略
          </Button>
        }
      />

      <SectionCard title="策略列表" description="按状态筛选；编辑、发布、归档操作只对项目策略生效。">
        <div className="flex flex-wrap gap-2 px-4 pt-4">
          {STATUS_FILTERS.map((f) => (
            <Button
              key={f.id}
              variant={filter === f.id ? "default" : "outline"}
              size="sm"
              onClick={() => setFilter(f.id)}
            >
              {f.label}
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
          <EmptyState icon={ScrollText} title="尚未创建任何策略" />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>名称</TableHead>
                <TableHead>版本</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>来源</TableHead>
                <TableHead className="w-[120px]">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {visible.map((row: QianchuanStrategy) => (
                <TableRow key={row.id}>
                  <TableCell className="font-mono text-sm">{row.name}</TableCell>
                  <TableCell>v{row.version}</TableCell>
                  <TableCell>{statusBadge(row.status)}</TableCell>
                  <TableCell>
                    {row.isSystem ? <Badge variant="outline">system</Badge> : <Badge variant="secondary">project</Badge>}
                  </TableCell>
                  <TableCell>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        router.push(`/projects/${projectId}/qianchuan/strategies/${row.id}/edit`)
                      }
                    >
                      {row.isSystem ? "查看" : "编辑"}
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
