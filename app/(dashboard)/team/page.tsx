"use client";

import { Suspense } from "react";
import { useTranslations } from "next-intl";
import { TeamPageClient } from "@/components/team/team-page-client";

function TeamPageFallback() {
  const t = useTranslations("teams");
  return <p className="text-sm text-muted-foreground">{t("teamPage.loading")}</p>;
}

export default function TeamPage() {
  return (
    <Suspense fallback={<TeamPageFallback />}>
      <TeamPageClient />
    </Suspense>
  );
}
