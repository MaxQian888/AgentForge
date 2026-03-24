"use client";

import { Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { TeamDetailView } from "@/components/team/team-detail-view";

function TeamDetailContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const teamId = searchParams.get("id");

  if (!teamId) {
    router.replace("/teams");
    return null;
  }

  return <TeamDetailView teamId={teamId} />;
}

export default function TeamDetailPage() {
  return (
    <Suspense>
      <TeamDetailContent />
    </Suspense>
  );
}
