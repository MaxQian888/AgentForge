"use client";

import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { MessageSquare } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
  const [selectedDeliveryId, setSelectedDeliveryId] = useState<string | null>(
    null
  );

  const selectedDelivery = useMemo(
    () => deliveries.find((delivery) => delivery.id === selectedDeliveryId) ?? null,
    [deliveries, selectedDeliveryId]
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
      },
      null,
      2
    );
  }, [selectedDelivery]);

  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-lg font-semibold">{t("history.title")}</h2>

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
                <TableHead>{t("history.colChannel")}</TableHead>
                <TableHead>{t("history.colPlatform")}</TableHead>
                <TableHead>{t("history.colEventType")}</TableHead>
                <TableHead>{t("history.colStatus")}</TableHead>
                <TableHead>{t("history.colDowngrade")}</TableHead>
                <TableHead>{t("history.colTime")}</TableHead>
                <TableHead>{t("history.colActions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deliveries.map((delivery) => (
                <TableRow key={delivery.id}>
                  <TableCell className="font-medium">
                    {delivery.channelId}
                  </TableCell>
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
                        <span className="text-xs text-destructive">
                          {delivery.failureReason}
                        </span>
                      ) : null}
                    </div>
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {delivery.downgradeReason ?? "—"}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {new Date(delivery.createdAt).toLocaleString()}
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
                        <Button
                          size="sm"
                          onClick={() => void retryDelivery(delivery.id)}
                        >
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
            <SheetDescription>
              {selectedDelivery?.eventType ?? ""}
            </SheetDescription>
          </SheetHeader>
          <pre className="overflow-x-auto px-4 pb-4 text-xs leading-5 text-muted-foreground">
            {selectedPayload}
          </pre>
        </SheetContent>
      </Sheet>
    </div>
  );
}
