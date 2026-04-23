"use client";

import { Suspense } from "react";
import { useTranslations } from "next-intl";
import { TeamPageClient } from "@/components/team/team-page-client";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function TeamPageFallback() {
  const t = useTranslations("teams");
  return <p className="text-sm text-muted-foreground">{t("teamPage.loading")}</p>;
}

export default function TeamPage() {
  const tc = useTranslations("common");
  useBreadcrumbs([{ label: tc("nav.group.project"), href: "/" }, { label: tc("nav.team") }]);
  return (
    <Suspense fallback={<TeamPageFallback />}>
      <TeamPageClient />
    </Suspense>
  );
}
