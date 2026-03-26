"use client";

import { Activity, Wifi, WifiOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { useIMStore } from "@/lib/stores/im-store";

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
  const bridgeStatus = useIMStore((s) => s.bridgeStatus);

  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-lg font-semibold">Bridge Health</h2>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">Registration</p>
            <div className="mt-1 flex items-center gap-2">
              {bridgeStatus.registered ? (
                <Wifi className="size-4 text-green-600 dark:text-green-400" />
              ) : (
                <WifiOff className="size-4 text-red-600 dark:text-red-400" />
              )}
              <p className="text-lg font-bold">
                {bridgeStatus.registered ? "Connected" : "Disconnected"}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">Health Status</p>
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
            <p className="text-sm text-muted-foreground">Last Heartbeat</p>
            <p className="mt-1 text-lg font-bold">
              {bridgeStatus.lastHeartbeat
                ? new Date(bridgeStatus.lastHeartbeat).toLocaleString()
                : "No heartbeat"}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardContent className="py-4">
            <p className="text-sm text-muted-foreground">Providers</p>
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
            Registered Providers
          </CardTitle>
          <CardDescription>
            IM platform providers currently registered with the bridge.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {bridgeStatus.providers.length === 0 ? (
            <div className="flex h-[80px] items-center justify-center rounded-md border border-dashed text-sm text-muted-foreground">
              No providers registered. Ensure the IM bridge is running and
              configured.
            </div>
          ) : (
            <div className="flex flex-wrap gap-2">
              {bridgeStatus.providers.map((provider) => (
                <Badge
                  key={provider}
                  variant="outline"
                  className="capitalize"
                >
                  {provider}
                </Badge>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
