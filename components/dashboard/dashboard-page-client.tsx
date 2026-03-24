"use client";

import { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { DashboardOverview } from "./dashboard-overview";
import { useDashboardStore } from "@/lib/stores/dashboard-store";

export function DashboardPageClient() {
  const searchParams = useSearchParams();
  const projectId = searchParams.get("project");
  const summary = useDashboardStore((state) => state.summary);
  const loading = useDashboardStore((state) => state.loading);
  const error = useDashboardStore((state) => state.error);
  const sectionErrors = useDashboardStore((state) => state.sectionErrors);
  const fetchSummary = useDashboardStore((state) => state.fetchSummary);

  useEffect(() => {
    void fetchSummary({ projectId });
  }, [fetchSummary, projectId]);

  return (
    <DashboardOverview
      summary={summary}
      loading={loading}
      error={error}
      sectionErrors={sectionErrors}
      onRetry={() => {
        void fetchSummary({ projectId });
      }}
    />
  );
}
