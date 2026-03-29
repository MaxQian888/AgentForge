"use client";

import { Suspense } from "react";
import { useTranslations } from "next-intl";
import { DashboardPageClient } from "@/components/dashboard/dashboard-page-client";

function DashboardPageFallback() {
  const t = useTranslations("dashboard");
  return <p className="text-sm text-muted-foreground">{t("loading")}</p>;
}

export default function DashboardPage() {
  return (
    <Suspense fallback={<DashboardPageFallback />}>
      <DashboardPageClient />
    </Suspense>
  );
}
