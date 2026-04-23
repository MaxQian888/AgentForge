"use client";

import { useTranslations } from "next-intl";
import { Activity, Wifi, WifiOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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

function SummaryCard({
  label,
  value,
  supporting,
}: {
  label: string;
  value: string | number;
  supporting?: string;
}) {
  return (
    <Card>
      <CardContent className="py-4">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className="mt-1 text-lg font-bold">{value}</p>
        {supporting ? (
          <p className="mt-1 text-xs text-muted-foreground">{supporting}</p>
        ) : null}
      </CardContent>
    </Card>
  );
}

export function IMBridgeHealth({
  onConfigureProvider,
}: {
  onConfigureProvider?: (platform: string) => void;
}) {
  const t = useTranslations("im");
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);
  const providerSummaries: IMBridgeProviderDetail[] =
    bridgeStatus.providerDetails.length > 0
      ? bridgeStatus.providerDetails
      : bridgeStatus.providers.map((platform) => ({
          platform,
          pendingDeliveries: 0,
          recentFailures: 0,
          recentDowngrades: 0,
        }));

  const averageLatencyText =
    bridgeStatus.averageLatencyMs > 0
      ? `${bridgeStatus.averageLatencyMs} ms`
      : t("health.latencyUnavailable");

  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-lg font-semibold">{t("health.title")}</h2>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
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
                  healthDotColors[bridgeStatus.health] ?? "bg-zinc-400",
                )}
              />
              <Badge
                variant="secondary"
                className={cn("text-sm capitalize", healthColors[bridgeStatus.health])}
              >
                {t(`healthStatusLabels.${bridgeStatus.health}`, { defaultValue: bridgeStatus.health })}
              </Badge>
            </div>
          </CardContent>
        </Card>

        <SummaryCard
          label={t("health.lastHeartbeat")}
          value={
            bridgeStatus.lastHeartbeat
              ? new Date(bridgeStatus.lastHeartbeat).toLocaleString()
              : t("health.noHeartbeat")
          }
        />

        <SummaryCard
          label={t("health.providers")}
          value={providerSummaries.length}
        />
        <SummaryCard
          label={t("health.pendingDeliveries")}
          value={bridgeStatus.pendingDeliveries}
        />
        <SummaryCard
          label={t("health.recentFailures")}
          value={bridgeStatus.recentFailures}
        />
        <SummaryCard
          label={t("health.averageLatency")}
          value={averageLatencyText}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Activity className="size-4" />
            {t("health.registeredProviders")}
          </CardTitle>
          <CardDescription>{t("health.registeredProvidersDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          {providerSummaries.length === 0 ? (
            <div className="flex h-[80px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
              {t("health.noProviders")}
            </div>
          ) : (
            <div className="grid gap-3">
              {providerSummaries.map((provider) => {
                const capabilityMatrix = provider.capabilityMatrix ?? {};
                const asyncUpdateModes = Array.isArray(
                  capabilityMatrix.asyncUpdateModes,
                )
                  ? (capabilityMatrix.asyncUpdateModes as string[])
                  : [];
                const diagnosticsEntries = Object.entries(provider.diagnostics ?? {});

                return (
                  <div key={provider.platform} className="rounded-md border p-4">
                    <div className="flex flex-wrap items-center gap-2">
                      <PlatformBadge platform={provider.platform} />
                      {provider.status ? (
                        <Badge variant="secondary" className="text-xs">
                          {provider.status}
                        </Badge>
                      ) : null}
                      {provider.transport ? (
                        <Badge variant="secondary" className="text-xs">
                          {provider.transport}
                        </Badge>
                      ) : null}
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

                    <div className="mt-4 grid gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4">
                      <div>
                        <p className="text-muted-foreground">{t("health.providerPending")}</p>
                        <p className="font-medium">{provider.pendingDeliveries}</p>
                      </div>
                      <div>
                        <p className="text-muted-foreground">{t("health.providerFailures")}</p>
                        <p className="font-medium">{provider.recentFailures}</p>
                      </div>
                      <div>
                        <p className="text-muted-foreground">{t("health.providerDowngrades")}</p>
                        <p className="font-medium">{provider.recentDowngrades}</p>
                      </div>
                      <div>
                        <p className="text-muted-foreground">{t("health.providerLastDelivery")}</p>
                        <p className="font-medium">
                          {provider.lastDeliveryAt
                            ? new Date(provider.lastDeliveryAt).toLocaleString()
                            : t("health.noHeartbeat")}
                        </p>
                      </div>
                    </div>

                    {provider.tenants && provider.tenants.length > 0 ? (
                      <div className="mt-4 space-y-1">
                        <p className="text-sm font-medium">
                          {t("health.tenantMounts", {
                            defaultValue: "Tenant mounts",
                          })}
                        </p>
                        <div className="flex flex-wrap gap-2">
                          {provider.tenants.map((tenantId) => {
                            const binding = provider.tenantManifest?.find(
                              (b) => b.id === tenantId,
                            );
                            return (
                              <span
                                key={tenantId}
                                className="rounded-md border border-dashed border-muted-foreground/40 bg-muted/30 px-2 py-1 text-xs"
                                title={
                                  binding
                                    ? `projectId=${binding.projectId}`
                                    : undefined
                                }
                              >
                                {tenantId}
                                {binding ? (
                                  <span className="ml-1 text-muted-foreground">
                                    → {binding.projectId.slice(0, 8)}…
                                  </span>
                                ) : null}
                              </span>
                            );
                          })}
                        </div>
                      </div>
                    ) : null}

                    <div className="mt-4 space-y-2">
                      {onConfigureProvider ? (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => onConfigureProvider(provider.platform)}
                        >
                          {t("health.configure")}
                        </Button>
                      ) : null}
                      <p className="text-sm font-medium">{t("health.diagnosticsTitle")}</p>
                      {diagnosticsEntries.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                          {t("health.diagnosticsUnavailable")}
                        </p>
                      ) : (
                        <div className="grid gap-2 sm:grid-cols-2">
                          {diagnosticsEntries.map(([key, value]) => (
                            <div key={key} className="rounded-md bg-muted/40 px-3 py-2 text-sm">
                              <p className="text-xs uppercase tracking-wide text-muted-foreground">
                                {key}
                              </p>
                              <p className="font-medium">{value}</p>
                            </div>
                          ))}
                        </div>
                      )}
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
