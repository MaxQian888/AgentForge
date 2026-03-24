import { Suspense } from "react";
import { DashboardPageClient } from "@/components/dashboard/dashboard-page-client";

function DashboardPageFallback() {
  return <p className="text-sm text-muted-foreground">Loading dashboard...</p>;
}

export default function DashboardPage() {
  return (
    <Suspense fallback={<DashboardPageFallback />}>
      <DashboardPageClient />
    </Suspense>
  );
}
