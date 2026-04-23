"use client";

import { Suspense, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function AgentRedirect() {
  const t = useTranslations();
  useBreadcrumbs([{ label: t("common.nav.agents"), href: "/agents" }, { label: t("common.nav.agents") }]);
  const searchParams = useSearchParams();
  const router = useRouter();
  const agentId = searchParams.get("id");

  useEffect(() => {
    if (agentId) {
      router.replace(`/agents?agent=${agentId}`);
    } else {
      router.replace("/agents");
    }
  }, [agentId, router]);

  return null;
}

export default function AgentPage() {
  return (
    <Suspense>
      <AgentRedirect />
    </Suspense>
  );
}
