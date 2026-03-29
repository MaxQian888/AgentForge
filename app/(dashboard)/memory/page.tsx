"use client";

import { useTranslations } from "next-intl";
import { MemoryPanel } from "@/components/memory/memory-panel";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

export default function MemoryPage() {
  const t = useTranslations("memory");
  const selectedProjectId = useDashboardStore((s) => s.selectedProjectId);

  if (!selectedProjectId) {
    return (
      <div className="flex flex-col gap-6">
        <h1 className="text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">
          {t("selectProject")}
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-bold">{t("title")}</h1>
      <MemoryPanel projectId={selectedProjectId} />
    </div>
  );
}
