"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionCard } from "@/components/shared/section-card";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { IMChannelConfig } from "@/components/im/im-channel-config";
import { IMBridgeHealth } from "@/components/im/im-bridge-health";
import { IMMessageHistory } from "@/components/im/im-message-history";
import { IMAggregateMetrics } from "@/components/im/im-aggregate-metrics";
import { IMBridgeStatusCards } from "@/components/im/im-bridge-status-cards";
import { useIMStore, type IMPlatform } from "@/lib/stores/im-store";
import { PageHeader } from "@/components/shared/page-header";
import { ErrorBanner } from "@/components/shared/error-banner";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";

export default function IMBridgePage() {
  useBreadcrumbs([{ label: "Configuration", href: "/" }, { label: "IM Bridge" }]);
  const t = useTranslations("im");
  const fetchChannels = useIMStore((s) => s.fetchChannels);
  const fetchBridgeStatus = useIMStore((s) => s.fetchBridgeStatus);
  const fetchDeliveryHistory = useIMStore((s) => s.fetchDeliveryHistory);
  const fetchEventTypes = useIMStore((s) => s.fetchEventTypes);
  const testSend = useIMStore((s) => s.testSend);
  const channels = useIMStore((s) => s.channels);
  const loading = useIMStore((s) => s.loading);
  const error = useIMStore((s) => s.error);
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);
  const deliveries = useIMStore((s) => s.deliveries);
  const lastTestSendResult = useIMStore((s) => s.lastTestSendResult);
  const [activeTab, setActiveTab] = useState("channels");
  const [preferredPlatform, setPreferredPlatform] = useState<string | null>(null);
  const [channelConfigSeed, setChannelConfigSeed] = useState(0);
  const [testPlatform, setTestPlatform] = useState<IMPlatform | "">("");
  const [testChannelId, setTestChannelId] = useState("");
  const [testMessage, setTestMessage] = useState("ping");
  const [testConfirmOpen, setTestConfirmOpen] = useState(false);

  useEffect(() => {
    void fetchChannels();
    void fetchBridgeStatus();
    void fetchDeliveryHistory();
    void fetchEventTypes();
  }, [fetchChannels, fetchBridgeStatus, fetchDeliveryHistory, fetchEventTypes]);

  // Poll bridge status + deliveries every 5s per spec (queue metrics refresh).
  useEffect(() => {
    const interval = window.setInterval(() => {
      void fetchBridgeStatus();
      void fetchDeliveryHistory();
    }, 5000);
    return () => window.clearInterval(interval);
  }, [fetchBridgeStatus, fetchDeliveryHistory]);

  const handleRefresh = () => {
    void fetchChannels();
    void fetchBridgeStatus();
    void fetchDeliveryHistory();
    void fetchEventTypes();
  };

  const handleConfigureProvider = (platform: string) => {
    setPreferredPlatform(platform);
    setActiveTab("channels");
    setChannelConfigSeed((current) => current + 1);
  };

  const activeChannels = useMemo(
    () => channels.filter((channel) => channel.active !== false),
    [channels],
  );
  const availablePlatforms = useMemo(
    () =>
      Array.from(new Set(activeChannels.map((channel) => channel.platform).filter(Boolean))).sort(),
    [activeChannels],
  );
  const successRateText = useMemo(() => {
    const settled = deliveries.filter((delivery) =>
      ["delivered", "suppressed", "failed", "timeout"].includes(delivery.status),
    );
    if (settled.length === 0) {
      return "-";
    }
    const successful = settled.filter((delivery) =>
      ["delivered", "suppressed"].includes(delivery.status),
    );
    return `${Math.round((successful.length / settled.length) * 100)}%`;
  }, [deliveries]);
  const effectiveTestPlatform =
    (testPlatform && availablePlatforms.includes(testPlatform) ? testPlatform : "") ||
    availablePlatforms[0] ||
    "";
  const platformChannels = useMemo(
    () => activeChannels.filter((channel) => channel.platform === effectiveTestPlatform),
    [activeChannels, effectiveTestPlatform],
  );
  const effectiveTestChannelId = testChannelId || platformChannels[0]?.channelId || "";

  const handleRequestTestSend = () => {
    if (!effectiveTestChannelId || !effectiveTestPlatform) return;
    setTestConfirmOpen(true);
  };

  const handleConfirmTestSend = async () => {
    setTestConfirmOpen(false);
    await testSend({
      platform: effectiveTestPlatform,
      channelId: effectiveTestChannelId,
      text: testMessage,
    });
  };

  const handleBridgeSendTest = (platform: string) => {
    const candidatePlatform = availablePlatforms.includes(platform as IMPlatform)
      ? (platform as IMPlatform)
      : effectiveTestPlatform;
    setTestPlatform(candidatePlatform || "");
    setTestChannelId("");
    // Queue the confirmation open on the next tick so state has settled.
    setTimeout(() => setTestConfirmOpen(true), 0);
  };

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <PageHeader
        title={t("title")}
        actions={
          <>
            <Badge
              variant="secondary"
              className={
                bridgeStatus.health === "healthy"
                  ? "bg-green-500/15 text-green-700 dark:text-green-400"
                  : bridgeStatus.health === "degraded"
                    ? "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400"
                    : "bg-red-500/15 text-red-700 dark:text-red-400"
              }
            >
              {bridgeStatus.health}
            </Badge>
            <Badge variant="secondary">{`${t("summaryPending")}: ${bridgeStatus.pendingDeliveries}`}</Badge>
            <Badge variant="secondary">{`${t("summaryFailures")}: ${bridgeStatus.recentFailures}`}</Badge>
            <Badge variant="secondary">{`${t("summarySuccessRate")}: ${successRateText}`}</Badge>
            <Button variant="outline" size="sm" onClick={handleRefresh} disabled={loading}>
              <RefreshCw className="mr-1 size-3.5" />
              {t("refresh")}
            </Button>
          </>
        }
      />

      {error && <ErrorBanner message={error} onRetry={handleRefresh} />}

      <IMAggregateMetrics />

      <IMBridgeStatusCards
        onConfigureProvider={handleConfigureProvider}
        onSendTest={handleBridgeSendTest}
      />

      <SectionCard
        title={t("testSendTitle")}
        bodyClassName="grid gap-4 md:grid-cols-[160px,1fr,1fr,auto]"
      >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="im-test-platform">{t("testSendPlatform")}</Label>
            <select
              id="im-test-platform"
              className="rounded-md border bg-background px-3 py-2 text-sm"
              value={effectiveTestPlatform}
              onChange={(event) => setTestPlatform(event.target.value as IMPlatform | "")}
              disabled={availablePlatforms.length === 0}
            >
              {availablePlatforms.length === 0 ? (
                <option value="">{t("channels.noChannels")}</option>
              ) : null}
              {availablePlatforms.map((platform) => (
                <option key={platform} value={platform}>
                  {platform}
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="im-test-channel">{t("testSendChannel")}</Label>
            <Input
              id="im-test-channel"
              value={effectiveTestChannelId}
              onChange={(event) => setTestChannelId(event.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="im-test-message">{t("testSendMessage")}</Label>
            <Input
              id="im-test-message"
              value={testMessage}
              onChange={(event) => setTestMessage(event.target.value)}
            />
          </div>
          <div className="flex items-end">
            <Button
              onClick={handleRequestTestSend}
              disabled={loading || !effectiveTestChannelId}
            >
              {t("testSendButton")}
            </Button>
          </div>
          {lastTestSendResult ? (
            <div className="md:col-span-4 flex flex-col gap-2">
              <Badge variant="secondary">
                {`${t("testSendResult")}: ${lastTestSendResult.status}`}
              </Badge>
              {typeof lastTestSendResult.latencyMs === "number" && lastTestSendResult.latencyMs > 0 ? (
                <p className="text-sm text-muted-foreground">
                  {t("testSendSuccessLatency", { latencyMs: lastTestSendResult.latencyMs })}
                </p>
              ) : null}
              {lastTestSendResult.failureReason ? (
                <>
                  <p className="text-sm text-destructive">{lastTestSendResult.failureReason}</p>
                  <p className="text-xs text-muted-foreground">{t("testSendRemediation")}</p>
                </>
              ) : null}
              {lastTestSendResult.downgradeReason ? (
                <p className="text-sm text-muted-foreground">
                  {`downgrade: ${lastTestSendResult.downgradeReason}`}
                </p>
              ) : null}
            </div>
          ) : null}
      </SectionCard>

      <ConfirmDialog
        open={testConfirmOpen}
        title={t("testSendConfirmTitle")}
        description={t("testSendConfirmDescription", {
          platform: effectiveTestPlatform || "?",
          channelId: effectiveTestChannelId || "?",
        })}
        confirmLabel={t("testSendConfirmAction")}
        onConfirm={() => void handleConfirmTestSend()}
        onCancel={() => setTestConfirmOpen(false)}
      />

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="channels">{t("tabChannels")}</TabsTrigger>
          <TabsTrigger value="health">{t("tabHealth")}</TabsTrigger>
          <TabsTrigger value="history">{t("tabHistory")}</TabsTrigger>
        </TabsList>

        <TabsContent value="channels">
          <IMChannelConfig
            key={`${preferredPlatform ?? "default"}:${channelConfigSeed}`}
            preferredPlatform={preferredPlatform}
          />
        </TabsContent>

        <TabsContent value="health">
          <IMBridgeHealth onConfigureProvider={handleConfigureProvider} />
        </TabsContent>

        <TabsContent value="history">
          <IMMessageHistory />
        </TabsContent>
      </Tabs>
    </div>
  );
}
