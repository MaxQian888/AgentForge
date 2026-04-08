"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useIMStore } from "@/lib/stores/im-store";

const HEALTH_VARIANT: Record<string, "default" | "secondary" | "destructive"> = {
  healthy: "default",
  degraded: "secondary",
  disconnected: "destructive",
};

export function SectionIMBridge() {
  const t = useTranslations("settings.imBridgeSettings");
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);
  const channels = useIMStore((s) => s.channels);
  const fetchBridgeStatus = useIMStore((s) => s.fetchBridgeStatus);
  const fetchChannels = useIMStore((s) => s.fetchChannels);

  useEffect(() => {
    void fetchBridgeStatus();
    void fetchChannels();
  }, [fetchBridgeStatus, fetchChannels]);

  const healthLabel =
    bridgeStatus.health === "healthy"
      ? t("healthy")
      : bridgeStatus.health === "degraded"
        ? t("degraded")
        : t("disconnected");

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("title")}</h2>
        <p className="text-sm text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{t("bridgeHealth")}</CardTitle>
            <Badge variant={HEALTH_VARIANT[bridgeStatus.health] ?? "secondary"}>
              {healthLabel}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Registered platforms */}
          <div className="flex flex-col gap-2">
            <p className="text-sm font-medium">{t("registeredPlatforms")}</p>
            {bridgeStatus.providers.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {bridgeStatus.providers.map((platform) => (
                  <Badge key={platform} variant="outline" className="capitalize">
                    {platform}
                  </Badge>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">{t("noPlatforms")}</p>
            )}
          </div>

          {/* Stats grid */}
          <div className="grid gap-3 md:grid-cols-3">
            <div className="rounded-md border p-3 text-center">
              <p className="text-2xl font-semibold">{bridgeStatus.pendingDeliveries}</p>
              <p className="text-xs text-muted-foreground">{t("pendingDeliveries")}</p>
            </div>
            <div className="rounded-md border p-3 text-center">
              <p className="text-2xl font-semibold">{bridgeStatus.recentFailures}</p>
              <p className="text-xs text-muted-foreground">{t("recentFailures")}</p>
            </div>
            <div className="rounded-md border p-3 text-center">
              <p className="text-2xl font-semibold">
                {bridgeStatus.averageLatencyMs}
                <span className="text-sm font-normal text-muted-foreground"> {t("latencyUnit")}</span>
              </p>
              <p className="text-xs text-muted-foreground">{t("averageLatency")}</p>
            </div>
          </div>

          {/* Provider details */}
          {bridgeStatus.providerDetails.length > 0 && (
            <div className="space-y-2">
              {bridgeStatus.providerDetails.map((provider) => (
                <div
                  key={provider.platform}
                  className="flex items-center justify-between rounded-md border p-3 text-sm"
                >
                  <div>
                    <span className="font-medium capitalize">{provider.platform}</span>
                    {provider.transport && (
                      <span className="ml-2 text-muted-foreground">({provider.transport})</span>
                    )}
                  </div>
                  <div className="flex items-center gap-3 text-muted-foreground">
                    <span>{provider.pendingDeliveries} pending</span>
                    <Badge variant={provider.status === "active" ? "default" : "secondary"} className="text-xs">
                      {provider.status ?? "unknown"}
                    </Badge>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Channel count + manage */}
          <div className="flex items-center justify-between rounded-md border bg-muted/50 p-3">
            <p className="text-sm text-muted-foreground">
              {t("channelCount", { count: channels.length })}
            </p>
            <Button asChild size="sm" variant="outline">
              <Link href="/im">{t("manageChannels")}</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
