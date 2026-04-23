"use client";

import { useTranslations } from "next-intl";
import { FolderOpen, Lock } from "lucide-react";
import { MemoryPanel } from "@/components/memory/memory-panel";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { useFeatureFlag } from "@/lib/feature-flags";

export default function MemoryPage() {
  const tc = useTranslations("common");
  useBreadcrumbs([{ label: tc("nav.group.operations"), href: "/" }, { label: tc("nav.memory") }]);
  const t = useTranslations("memory");
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);
  const memoryExplorerEnabled = useFeatureFlag("MEMORY_EXPLORER");

  if (!memoryExplorerEnabled) {
    return (
      <div className="flex flex-col gap-[var(--space-section-gap)]">
        <PageHeader title={t("title")} />
        <EmptyState
          icon={Lock}
          title={t("title")}
          description={t("featureDisabled")}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
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
