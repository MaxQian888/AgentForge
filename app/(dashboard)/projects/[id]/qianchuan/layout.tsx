"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { usePluginEnabled } from "@/hooks/use-plugin-enabled";

// The qianchuan-ads feature ships as a first-party in-proc integration
// plugin (see plugins/integrations/qianchuan-ads/manifest.yaml). This
// layout gates every page under /projects/[id]/qianchuan/** on the
// plugin's runtime state so operators can disable the feature via the
// backend env flag AGENTFORGE_PLUGIN_QIANCHUAN without redeploying the FE.
// NEXT_PUBLIC_PLUGIN_QIANCHUAN is still honored as a build-time override
// for environments where the backend isn't reachable (pure FE dev).
export default function QianchuanPluginLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const params = useParams<{ id: string }>();
  const projectId = params?.id ?? "";
  const { loading, enabled, lifecycleState } = usePluginEnabled(
    "qianchuan-ads",
    process.env.NEXT_PUBLIC_PLUGIN_QIANCHUAN,
  );

  if (loading) {
    return (
      <div className="space-y-4 p-8">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </div>
    );
  }

  if (!enabled) {
    return (
      <div className="flex min-h-[50vh] flex-col items-center justify-center gap-4 p-8 text-center">
        <h1 className="text-2xl font-semibold">千川插件未启用</h1>
        <p className="max-w-md text-sm text-muted-foreground">
          The <code className="rounded bg-muted px-1 py-0.5">qianchuan-ads</code>{" "}
          first-party integration plugin is not active on this backend
          {lifecycleState ? ` (lifecycle: ${lifecycleState})` : ""}. Ask an
          administrator to set <code>AGENTFORGE_PLUGIN_QIANCHUAN=enabled</code>{" "}
          and restart the orchestrator, or enable it from the plugins page.
        </p>
        <div className="flex gap-2">
          <Button asChild variant="default">
            <Link href="/plugins">Open plugins page</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href={`/projects/${projectId}`}>Back to project</Link>
          </Button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
