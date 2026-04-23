"use client";

import { useTranslations } from "next-intl";
import { FolderOpen } from "lucide-react";
import { DocumentPanel } from "@/components/documents/document-panel";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function DocumentsPage() {
  const tc = useTranslations("common");
  const t = useTranslations("documents");
  useBreadcrumbs([{ label: tc("nav.group.operations"), href: "/" }, { label: tc("nav.documents") }]);
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title={t("title")} />
      {selectedProjectId ? (
        <DocumentPanel projectId={selectedProjectId} />
      ) : (
        <EmptyState
          icon={FolderOpen}
          title={t("selectProject")}
        />
      )}
    </div>
  );
}
