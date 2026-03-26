"use client";

import { MessageSquare } from "lucide-react";
import { Badge } from "@/components/ui/badge";
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
  delivered: "bg-green-500/15 text-green-700 dark:text-green-400",
  suppressed: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  failed: "bg-red-500/15 text-red-700 dark:text-red-400",
};

export function IMMessageHistory() {
  const deliveries = useIMStore((s) => s.deliveries);
  const loading = useIMStore((s) => s.loading);

  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-lg font-semibold">Message History</h2>

      {loading ? (
        <p className="text-muted-foreground">Loading deliveries...</p>
      ) : deliveries.length === 0 ? (
        <div className="flex h-[120px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
          <MessageSquare className="mr-2 size-4" />
          No delivery history available.
        </div>
      ) : (
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Channel</TableHead>
                <TableHead>Platform</TableHead>
                <TableHead>Event Type</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Time</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {deliveries.map((delivery) => (
                <TableRow key={delivery.id}>
                  <TableCell className="font-medium">
                    {delivery.channelId}
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="capitalize text-xs">
                      {delivery.platform}
                    </Badge>
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
                    {new Date(delivery.createdAt).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
