"use client";

import { useMemo } from "react";
import { useTranslations } from "next-intl";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/shared/status-dot";
import { PlatformBadge } from "@/components/shared/platform-badge";
import { EmptyState } from "@/components/shared/empty-state";
import { Wifi } from "lucide-react";
import { useIMStore, type IMBridgeProviderDetail } from "@/lib/stores/im-store";

export interface IMBridgeStatusCardsProps {
  onConfigureProvider?: (platform: string) => void;
  onSendTest?: (platform: string) => void;
}

type BridgeState = "connected" | "degraded" | "disconnected";

function deriveState(
  detail: IMBridgeProviderDetail | undefined,
  fallbackHealth: string,
): BridgeState {
  if (!detail) {
    if (fallbackHealth === "healthy") return "connected";
    if (fallbackHealth === "degraded") return "degraded";
    return "disconnected";
  }
  if (detail.status && detail.status.toLowerCase() !== "online") return "disconnected";
  if (detail.recentFailures > 0) return "degraded";
  if (detail.pendingDeliveries >= 100) return "degraded";
  return "connected";
}

const stateLabelKeys: Record<BridgeState, string> = {
  connected: "health.connected",
  degraded: "health.healthStatus",
  disconnected: "health.disconnected",
};

export function IMBridgeStatusCards({
  onConfigureProvider,
  onSendTest,
}: IMBridgeStatusCardsProps) {
  const t = useTranslations("im");
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);

  const summaries = useMemo(() => {
    const byPlatform = new Map<
      string,
      {
        platform: string;
        detail?: IMBridgeProviderDetail;
      }
    >();
    for (const detail of bridgeStatus.providerDetails) {
      byPlatform.set(detail.platform, { platform: detail.platform, detail });
    }
    for (const platform of bridgeStatus.providers) {
      if (!byPlatform.has(platform)) {
        byPlatform.set(platform, { platform });
      }
    }
    return Array.from(byPlatform.values());
  }, [bridgeStatus.providers, bridgeStatus.providerDetails]);

  if (summaries.length === 0) {
    return (
      <EmptyState
        icon={Wifi}
        title={t("bridges.title")}
        description={t("bridges.empty")}
      />
    );
  }

  return (
    <section className="flex flex-col gap-3" data-testid="im-bridge-status-cards">
      <div>
        <h2 className="text-lg font-semibold">{t("bridges.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("bridges.description")}</p>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {summaries.map((summary) => {
          const state = deriveState(summary.detail, bridgeStatus.health);
          const label = t(stateLabelKeys[state]);
          const pending = summary.detail?.pendingDeliveries ?? 0;
          const failures = summary.detail?.recentFailures ?? 0;
          const lastDeliveryAt = summary.detail?.lastDeliveryAt;

          return (
            <Card
              key={summary.platform}
              data-testid={`im-bridge-card-${summary.platform}`}
              data-state={state}
            >
              <CardContent className="flex flex-col gap-3 py-4">
                <div className="flex items-center justify-between gap-2">
                  <PlatformBadge platform={summary.platform} />
                  <div className="flex items-center gap-1.5">
                    <StatusDot status={state} />
                    <span className="text-xs text-muted-foreground">{label}</span>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-2 text-sm">
                  <div>
                    <p className="text-xs text-muted-foreground">{t("bridges.pending")}</p>
                    <p
                      className="font-medium"
                      data-testid={`bridge-pending-${summary.platform}`}
                    >
                      {pending}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">{t("bridges.failures")}</p>
                    <p
                      className="font-medium"
                      data-testid={`bridge-failures-${summary.platform}`}
                    >
                      {failures}
                    </p>
                  </div>
                  <div className="col-span-2">
                    <p className="text-xs text-muted-foreground">
                      {t("bridges.lastDelivery")}
                    </p>
                    <p className="text-sm font-medium">
                      {lastDeliveryAt
                        ? new Date(lastDeliveryAt).toLocaleString()
                        : t("health.noHeartbeat")}
                    </p>
                  </div>
                </div>

                <div className="flex flex-wrap gap-2">
                  {onConfigureProvider ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => onConfigureProvider(summary.platform)}
                    >
                      {t("bridges.configure")}
                    </Button>
                  ) : null}
                  {onSendTest ? (
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => onSendTest(summary.platform)}
                    >
                      {t("bridges.sendTest")}
                    </Button>
                  ) : null}
                </div>
              </CardContent>
            </Card>
          );
        })}
      </div>
    </section>
  );
}
