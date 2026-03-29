"use client";

import { useTranslations } from "next-intl";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { WorkflowConfigPanel } from "@/components/workflow/workflow-config-panel";

export default function WorkflowPage() {
  const t = useTranslations("workflow");
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">{t("title")}</h1>
      {selectedProjectId ? (
        <WorkflowConfigPanel projectId={selectedProjectId} />
      ) : (
        <div className="text-sm text-muted-foreground">
          {t("selectProject")}
        </div>
      )}
    </div>
  );
}
