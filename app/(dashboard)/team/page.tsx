import { Suspense } from "react";
import { TeamPageClient } from "@/components/team/team-page-client";

function TeamPageFallback() {
  return <p className="text-sm text-muted-foreground">Loading team workspace...</p>;
}

export default function TeamPage() {
  return (
    <Suspense fallback={<TeamPageFallback />}>
      <TeamPageClient />
    </Suspense>
  );
}
