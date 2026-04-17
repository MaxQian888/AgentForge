"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { MessageSquare } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { PlatformBadge } from "@/components/shared/platform-badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { useIMStore, type IMDeliveryStatus } from "@/lib/stores/im-store";

const statusColors: Record<IMDeliveryStatus, string> = {
  pending: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  delivered: "bg-green-500/15 text-green-700 dark:text-green-400",
  suppressed: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
  timeout: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
};

export function IMMessageHistory() {
  const t = useTranslations("im");
  const deliveries = useIMStore((s) => s.deliveries);
  const loading = useIMStore((s) => s.loading);
  const retryDelivery = useIMStore((s) => s.retryDelivery);
  const retryDeliveries = useIMStore((s) => s.retryDeliveries);
  const clearRetryQueue = useIMStore((s) => s.clearRetryQueue);
  const fetchDeliveryHistory = useIMStore((s) => s.fetchDeliveryHistory);
  const setHistoryFilters = useIMStore((s) => s.setHistoryFilters);
  const historyFilters = useIMStore((s) => s.historyFilters);
  const [selectedDeliveryId, setSelectedDeliveryId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [draftFilters, setDraftFilters] = useState({
    status: historyFilters.status ?? "",
    platform: historyFilters.platform ?? "",
    eventType: historyFilters.eventType ?? "",
  });
  const [clearQueueOpen, setClearQueueOpen] = useState(false);

  const pendingRetryCount = useMemo(
    () =>
      deliveries.filter((delivery) =>
        ["failed", "timeout", "pending"].includes(delivery.status),
      ).length,
    [deliveries],
  );
  const matchCount = deliveries.length;

  const selectedDelivery = useMemo(
    () => deliveries.find((delivery) => delivery.id === selectedDeliveryId) ?? null,
    [deliveries, selectedDeliveryId],
  );

  const selectedPayload = useMemo(() => {
    if (!selectedDelivery) {
      return "";
    }
    return JSON.stringify(
      {
        content: selectedDelivery.content ?? "",
        metadata: selectedDelivery.metadata ?? {},
        failureReason: selectedDelivery.failureReason ?? "",
        downgradeReason: selectedDelivery.downgradeReason ?? "",
        queuedAt: selectedDelivery.createdAt,
        processedAt: selectedDelivery.processedAt ?? "",
        latencyMs: selectedDelivery.latencyMs ?? null,
      },
      null,
      2,
    );
  }, [selectedDelivery]);

  const retryableSelectedIds = selectedIds.filter((id) => {
    const delivery = deliveries.find((item) => item.id === id);
    return delivery?.status === "failed" || delivery?.status === "timeout";
  });

  const formatLatency = (latencyMs?: number) =>
    typeof latencyMs === "number" && latencyMs > 0
      ? `${latencyMs.toLocaleString()} ms`
      : t("history.detailUnavailable");

  const applyFilters = async () => {
    const nextFilters = {
      status: draftFilters.status,
      platform: draftFilters.platform,
      eventType: draftFilters.eventType,
    };
    setHistoryFilters(nextFilters);
    await fetchDeliveryHistory(nextFilters);
  };

  const clearFilters = async () => {
    setDraftFilters({ status: "", platform: "", eventType: "" });
    setHistoryFilters({});
    await fetchDeliveryHistory({});
  };

  const handleBatchRetry = async () => {
    if (retryableSelectedIds.length === 0) {
      return;
    }
    await retryDeliveries(retryableSelectedIds);
    setSelectedIds([]);
  };

  const handleConfirmClearQueue = async () => {
    setClearQueueOpen(false);
    await clearRetryQueue();
  };

  const toggleSelection = (deliveryId: string, checked: boolean) => {
    setSelectedIds((current) =>
      checked ? [...new Set([...current, deliveryId])] : current.filter((id) => id !== deliveryId),
    );
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-semibold">{t("history.title")}</h2>
          {matchCount > 0 ? (
            <Badge variant="secondary" data-testid="history-match-count">
              {t(matchCount === 1 ? "history.filterMatchCount" : "history.filterMatchCountPlural", {
                count: matchCount,
              })}
            </Badge>
          ) : null}
        </div>
        <div className="grid gap-3 rounded-md border p-4 md:min-w-[420px] md:grid-cols-3">
          <div className="flex flex-col gap-1">
            <label htmlFor="history-status" className="text-sm font-medium">
              {t("history.filterStatus")}
            </label>
            <Select
              value={draftFilters.status || "__none__"}
              onValueChange={(value) =>
                setDraftFilters((current) => ({ ...current, status: value === "__none__" ? "" : value }))
              }
            >
              <SelectTrigger aria-label={t("history.filterStatus")} className="text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__none__">All</SelectItem>
                <SelectItem value="pending">pending</SelectItem>
                <SelectItem value="delivered">delivered</SelectItem>
                <SelectItem value="failed">failed</SelectItem>
                <SelectItem value="timeout">timeout</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="history-platform" className="text-sm font-medium">
              {t("history.filterPlatform")}
            </label>
            <input
              id="history-platform"
              aria-label={t("history.filterPlatform")}
              className="rounded-md border bg-background px-3 py-2 text-sm"
              value={draftFilters.platform}
              onChange={(event) =>
                setDraftFilters((current) => ({ ...current, platform: event.target.value }))
              }
            />
          </div>
          <div className="flex flex-col gap-1">
            <label htmlFor="history-event" className="text-sm font-medium">
              {t("history.filterEventType")}
            </label>
            <input
              id="history-event"
              aria-label={t("history.filterEventType")}
              className="rounded-md border bg-background px-3 py-2 text-sm"
              value={draftFilters.eventType}
              onChange={(event) =>
                setDraftFilters((current) => ({ ...current, eventType: event.target.value }))
              }
            />
          </div>
          <div className="flex flex-wrap gap-2 md:col-span-3">
            <Button variant="outline" size="sm" onClick={() => void applyFilters()}>
              {t("history.applyFilters")}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => void clearFilters()}>
              {t("history.clearFilters")}
            </Button>
            <Button
              size="sm"
              onClick={() => void handleBatchRetry()}
              disabled={retryableSelectedIds.length === 0}
            >
              {t("history.retrySelected")}
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setClearQueueOpen(true)}
              disabled={pendingRetryCount === 0 || loading}
            >
              {t("history.clearRetryQueue")}
            </Button>
          </div>
        </div>
      </div>

      {loading ? (
        <p className="text-muted-foreground">{t("history.loading")}</p>
      ) : deliveries.length === 0 ? (
        <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
          <MessageSquare className="mr-2 size-4" />
          {t("history.noDeliveries")}
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead />
                <TableHead>{t("history.colChannel")}</TableHead>
                <TableHead>{t("history.colPlatform")}</TableHead>
                <TableHead>{t("history.colEventType")}</TableHead>
                <TableHead>{t("history.colStatus")}</TableHead>
                <TableHead>{t("history.colDowngrade")}</TableHead>
                <TableHead>{t("history.colTime")}</TableHead>
                <TableHead>{t("history.colProcessed")}</TableHead>
                <TableHead>{t("history.colLatency")}</TableHead>
                <TableHead>{t("history.colActions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deliveries.map((delivery) => (
                <TableRow key={delivery.id}>
                  <TableCell>
                    <input
                      type="checkbox"
                      aria-label={`Select ${delivery.id}`}
                      checked={selectedIds.includes(delivery.id)}
                      onChange={(event) => toggleSelection(delivery.id, event.target.checked)}
                    />
                  </TableCell>
                  <TableCell className="font-medium">{delivery.channelId}</TableCell>
                  <TableCell>
                    <PlatformBadge platform={delivery.platform} />
                  </TableCell>
                  <TableCell className="text-sm">{delivery.eventType}</TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1">
                      <Badge
                        variant="secondary"
                        className={cn("w-fit text-xs", statusColors[delivery.status])}
                      >
                        {delivery.status}
                      </Badge>
                      {delivery.status === "failed" && delivery.failureReason ? (
                        <span className="text-xs text-destructive">{delivery.failureReason}</span>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {delivery.downgradeReason ?? "—"}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {new Date(delivery.createdAt).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {delivery.processedAt
                      ? new Date(delivery.processedAt).toLocaleString()
                      : t("history.detailUnavailable")}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {formatLatency(delivery.latencyMs)}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setSelectedDeliveryId(delivery.id)}
                      >
                        {t("history.previewPayload")}
                      </Button>
                      {delivery.status === "failed" || delivery.status === "timeout" ? (
                        <Button size="sm" onClick={() => void retryDelivery(delivery.id)}>
                          {t("history.retry")}
                        </Button>
                      ) : null}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <ConfirmDialog
        open={clearQueueOpen}
        title={t("history.clearRetryQueueTitle")}
        description={t("history.clearRetryQueueDescription", { count: pendingRetryCount })}
        confirmLabel={t("history.clearRetryQueueAction")}
        variant="destructive"
        onConfirm={() => void handleConfirmClearQueue()}
        onCancel={() => setClearQueueOpen(false)}
      />

      <Sheet
        open={selectedDelivery !== null}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedDeliveryId(null);
          }
        }}
      >
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{t("history.deliveryPayload")}</SheetTitle>
            <SheetDescription>{selectedDelivery?.eventType ?? ""}</SheetDescription>
          </SheetHeader>
          {selectedDelivery ? (
            <div className="space-y-4 px-4 pb-4 text-sm">
              <div className="grid gap-3 sm:grid-cols-3">
                <div>
                  <p className="text-xs uppercase tracking-wide text-muted-foreground">
                    {t("history.detailQueuedAt")}
                  </p>
                  <p>{new Date(selectedDelivery.createdAt).toLocaleString()}</p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wide text-muted-foreground">
                    {t("history.detailProcessedAt")}
                  </p>
                  <p>
                    {selectedDelivery.processedAt
                      ? new Date(selectedDelivery.processedAt).toLocaleString()
                      : t("history.detailUnavailable")}
                  </p>
                </div>
                <div>
                  <p className="text-xs uppercase tracking-wide text-muted-foreground">
                    {t("history.detailLatency")}
                  </p>
                  <p>{formatLatency(selectedDelivery.latencyMs)}</p>
                </div>
              </div>
              <pre className="overflow-x-auto rounded-md bg-muted/50 p-3 text-xs leading-5 text-muted-foreground">
                {selectedPayload}
              </pre>
            </div>
          ) : null}
        </SheetContent>
      </Sheet>
    </div>
  );
}
