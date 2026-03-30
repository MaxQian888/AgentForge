"use client";

import { Suspense, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

function AgentRedirect() {
  useBreadcrumbs([{ label: "Agents", href: "/agents" }, { label: "Agent" }]);
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
