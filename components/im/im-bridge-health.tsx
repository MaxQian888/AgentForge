"use client";

import { useTranslations } from "next-intl";
import { Activity, Wifi, WifiOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { PlatformBadge } from "@/components/shared/platform-badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { useIMStore, type IMBridgeProviderDetail } from "@/lib/stores/im-store";

const healthColors: Record<string, string> = {
  healthy: "bg-green-500/15 text-green-700 dark:text-green-400",
  degraded: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  disconnected: "bg-red-500/15 text-red-700 dark:text-red-400",
};

const healthDotColors: Record<string, string> = {
  healthy: "bg-green-500",
  degraded: "bg-yellow-500",
  disconnected: "bg-red-500",
};

export function IMBridgeHealth() {
  const t = useTranslations("im");
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);
  const providerSummaries: IMBridgeProviderDetail[] =
    bridgeStatus.providerDetails.length > 0
      ? bridgeStatus.providerDetails
      : bridgeStatus.providers.map((platform) => ({ platform }));

  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-lg font-semibold">{t("health.title")}</h2>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">{t("health.registration")}</p>
            <div className="mt-1 flex items-center gap-2">
              {bridgeStatus.registered ? (
                <Wifi className="size-4 text-green-600 dark:text-green-400" />
              ) : (
                <WifiOff className="size-4 text-red-600 dark:text-red-400" />
              )}
              <p className="text-lg font-bold">
                {bridgeStatus.registered ? t("health.connected") : t("health.disconnected")}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">{t("health.healthStatus")}</p>
            <div className="mt-1 flex items-center gap-2">
              <span
                className={cn(
                  "inline-block size-2.5 rounded-full",
                  healthDotColors[bridgeStatus.health] ?? "bg-zinc-400"
                )}
              />
              <Badge
                variant="secondary"
                className={cn(
                  "text-sm capitalize",
                  healthColors[bridgeStatus.health]
                )}
              >
                {bridgeStatus.health}
              </Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">{t("health.lastHeartbeat")}</p>
            <p className="mt-1 text-lg font-bold">
              {bridgeStatus.lastHeartbeat
                ? new Date(bridgeStatus.lastHeartbeat).toLocaleString()
                : t("health.noHeartbeat")}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">{t("health.providers")}</p>
            <p className="mt-1 text-lg font-bold">
              {bridgeStatus.providers.length}
            </p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="size-4" />
            {t("health.registeredProviders")}
          </CardTitle>
          <CardDescription>
            {t("health.registeredProvidersDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {bridgeStatus.providers.length === 0 ? (
            <div className="flex h-[80px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
              {t("health.noProviders")}
            </div>
          ) : (
            <div className="grid gap-3">
              {providerSummaries.map((provider) => {
                const capabilityMatrix = provider.capabilityMatrix ?? {};
                const asyncUpdateModes = Array.isArray(
                  capabilityMatrix.asyncUpdateModes
                )
                  ? (capabilityMatrix.asyncUpdateModes as string[])
                  : [];

                return (
                  <div
                    key={provider.platform}
                    className="rounded-md border p-3"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <PlatformBadge platform={provider.platform} />
                      {typeof capabilityMatrix.structuredSurface === "string" ? (
                        <Badge variant="secondary" className="text-xs">
                          {capabilityMatrix.structuredSurface}
                        </Badge>
                      ) : null}
                      {typeof capabilityMatrix.actionCallbackMode === "string" ? (
                        <Badge variant="secondary" className="text-xs">
                          {capabilityMatrix.actionCallbackMode}
                        </Badge>
                      ) : null}
                      {asyncUpdateModes.map((mode) => (
                        <Badge key={mode} variant="secondary" className="text-xs">
                          {mode}
                        </Badge>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
