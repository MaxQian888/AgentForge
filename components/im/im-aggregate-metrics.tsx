"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Gauge,
  Layers,
  Timer,
} from "lucide-react";
import { MetricCard } from "@/components/shared/metric-card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { useIMStore } from "@/lib/stores/im-store";

const QUEUE_WARNING_THRESHOLD = 100;
const WINDOW_MS = 24 * 60 * 60 * 1000;
const NOW_REFRESH_MS = 60_000;

export function IMAggregateMetrics() {
  const t = useTranslations("im");
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);
  const deliveries = useIMStore((s) => s.deliveries);
  const [nowMs, setNowMs] = useState<number>(() => Date.now());

  useEffect(() => {
    const interval = window.setInterval(() => setNowMs(Date.now()), NOW_REFRESH_MS);
    return () => window.clearInterval(interval);
  }, []);

  const aggregates = useMemo(() => {
    const recent = deliveries.filter((delivery) => {
      const created = Date.parse(delivery.createdAt);
      return Number.isFinite(created) && nowMs - created <= WINDOW_MS;
    });

    const settled = recent.filter((delivery) =>
      ["delivered", "suppressed", "failed", "timeout"].includes(delivery.status),
    );
    const successful = settled.filter((delivery) =>
      ["delivered", "suppressed"].includes(delivery.status),
    );
    const latencies = settled
      .map((delivery) => delivery.latencyMs)
      .filter((value): value is number => typeof value === "number" && value > 0);
    const avgLatency = latencies.length > 0
      ? Math.round(latencies.reduce((acc, value) => acc + value, 0) / latencies.length)
      : 0;
    const successRatePercent = settled.length > 0
      ? Math.round((successful.length / settled.length) * 100)
      : null;
    const failureCount = settled.filter((delivery) => delivery.status === "failed").length;

    // Throughput: settled per minute, over the observed span.
    const processedAtValues = settled
      .map((delivery) => delivery.processedAt ?? delivery.createdAt)
      .map((value) => Date.parse(value))
      .filter((value) => Number.isFinite(value));
    let perMinute = 0;
    if (processedAtValues.length >= 2) {
      const spanMs = Math.max(
        1,
        Math.max(...processedAtValues) - Math.min(...processedAtValues),
      );
      perMinute = Math.round((settled.length / spanMs) * 60_000);
    }

    return {
      total: recent.length,
      successRatePercent,
      avgLatencyMs: avgLatency,
      failureCount,
      perMinute,
    };
  }, [deliveries, nowMs]);

  const queueDepth = bridgeStatus.pendingDeliveries;
  const queueIsBackedUp = queueDepth >= QUEUE_WARNING_THRESHOLD;
  const drainSeconds = aggregates.perMinute > 0
    ? Math.round((queueDepth / aggregates.perMinute) * 60)
    : null;

  const successRateText =
    aggregates.successRatePercent === null ? "—" : `${aggregates.successRatePercent}%`;
  const avgLatencyText =
    aggregates.avgLatencyMs > 0 ? `${aggregates.avgLatencyMs} ms` : "—";
  const perMinuteText = aggregates.perMinute > 0 ? `${aggregates.perMinute}` : "—";

  return (
    <section className="flex flex-col gap-3" data-testid="im-aggregate-metrics">
      <div>
        <h2 className="text-lg font-semibold">{t("metrics.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("metrics.description")}</p>
      </div>

      {queueIsBackedUp ? (
        <Alert variant="destructive">
          <AlertTriangle className="size-4" />
          <AlertTitle>{t("metrics.queueWarning")}</AlertTitle>
          {drainSeconds !== null ? (
            <AlertDescription>
              {t("metrics.queueDrainEstimate", { seconds: drainSeconds })}
            </AlertDescription>
          ) : null}
        </Alert>
      ) : null}

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <MetricCard
          label={t("metrics.totalMessages")}
          value={aggregates.total}
          icon={Activity}
        />
        <MetricCard
          label={t("metrics.successRate")}
          value={successRateText}
          icon={CheckCircle2}
        />
        <MetricCard
          label={t("metrics.avgLatency")}
          value={avgLatencyText}
          icon={Timer}
        />
        <MetricCard
          label={t("metrics.pendingQueue")}
          value={queueDepth}
          icon={Layers}
        />
        <MetricCard
          label={t("metrics.processingRate")}
          value={perMinuteText}
          icon={Gauge}
        />
      </div>

      {aggregates.failureCount > 0 ? (
        <p className="text-xs text-muted-foreground">
          {t("metrics.failureCount")}: {aggregates.failureCount}
        </p>
      ) : null}
    </section>
  );
}
