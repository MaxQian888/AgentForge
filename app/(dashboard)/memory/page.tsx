"use client";

import { useTranslations } from "next-intl";
import { FolderOpen } from "lucide-react";
import { MemoryPanel } from "@/components/memory/memory-panel";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function MemoryPage() {
  useBreadcrumbs([{ label: "Operations", href: "/" }, { label: "Memory" }]);
  const t = useTranslations("memory");
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t("title")} />
      {selectedProjectId ? (
        <MemoryPanel projectId={selectedProjectId} />
      ) : (
        <EmptyState
          icon={FolderOpen}
          title={t("selectProject")}
        />
      )}
    </div>
  );
}
