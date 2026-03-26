"use client";

import { useEffect } from "react";
import { RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { IMChannelConfig } from "@/components/im/im-channel-config";
import { IMBridgeHealth } from "@/components/im/im-bridge-health";
import { IMMessageHistory } from "@/components/im/im-message-history";
import { useIMStore } from "@/lib/stores/im-store";

export default function IMBridgePage() {
  const fetchChannels = useIMStore((s) => s.fetchChannels);
  const fetchBridgeStatus = useIMStore((s) => s.fetchBridgeStatus);
  const fetchDeliveryHistory = useIMStore((s) => s.fetchDeliveryHistory);
  const loading = useIMStore((s) => s.loading);
  const error = useIMStore((s) => s.error);
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);

  useEffect(() => {
    void fetchChannels();
    void fetchBridgeStatus();
    void fetchDeliveryHistory();
  }, [fetchChannels, fetchBridgeStatus, fetchDeliveryHistory]);

  const handleRefresh = () => {
    void fetchChannels();
    void fetchBridgeStatus();
    void fetchDeliveryHistory();
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold">IM Bridge</h1>
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
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          disabled={loading}
        >
          <RefreshCw className="mr-1 size-3.5" />
          Refresh
        </Button>
      </div>

      {error ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <Tabs defaultValue="channels">
        <TabsList>
          <TabsTrigger value="channels">Channels</TabsTrigger>
          <TabsTrigger value="health">Bridge Health</TabsTrigger>
          <TabsTrigger value="history">Message History</TabsTrigger>
        </TabsList>

        <TabsContent value="channels">
          <IMChannelConfig />
        </TabsContent>

        <TabsContent value="health">
          <IMBridgeHealth />
        </TabsContent>

        <TabsContent value="history">
          <IMMessageHistory />
        </TabsContent>
      </Tabs>
    </div>
  );
}
