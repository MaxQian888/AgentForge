"use client";

import { Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { TeamDetailView } from "@/components/team/team-detail-view";
import { PageHeader } from "@/components/shared/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function TeamDetailContent() {
  useBreadcrumbs([{ label: "Teams", href: "/teams" }, { label: "Detail" }]);
  const searchParams = useSearchParams();
  const router = useRouter();
  const teamId = searchParams.get("id");

  if (!teamId) {
    router.replace("/teams");
    return null;
  }

  return <TeamDetailView teamId={teamId} />;
}

function TeamDetailFallback() {
  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader title="Loading..." />
      <div className="flex flex-col gap-[var(--space-stack-md)]">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    </div>
  );
}

export default function TeamDetailPage() {
  return (
    <Suspense fallback={<TeamDetailFallback />}>
      <TeamDetailContent />
    </Suspense>
  );
}
