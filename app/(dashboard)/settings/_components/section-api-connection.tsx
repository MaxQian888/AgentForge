"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Check, Copy, RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useBackendUrl } from "@/hooks/use-backend-url";
import { useApiSettingsStore } from "@/lib/stores/api-settings-store";

type HealthStatus = "checking" | "healthy" | "unhealthy";

function CopyButton({ text }: { text: string }) {
  const t = useTranslations("settings.apiConnection");
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <Button type="button" variant="ghost" size="sm" className="h-8 gap-1.5" onClick={handleCopy}>
      {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      <span className="text-xs">{copied ? t("copied") : t("copyUrl")}</span>
    </Button>
  );
}

export function SectionApiConnection() {
  const t = useTranslations("settings.apiConnection");
  const backendUrl = useBackendUrl();
  const [health, setHealth] = useState<HealthStatus>("checking");

  const timeoutMs = useApiSettingsStore((s) => s.timeoutMs);
  const retryCount = useApiSettingsStore((s) => s.retryCount);
  const setTimeoutMs = useApiSettingsStore((s) => s.setTimeoutMs);
  const setRetryCount = useApiSettingsStore((s) => s.setRetryCount);

  const imBridgeUrl = backendUrl.replace(/:\d+$/, ":7779");
  const marketplaceUrl = backendUrl.replace(/:\d+$/, ":7781");

  const runHealthCheck = useCallback(() => {
    setHealth("checking");
    fetch(`${backendUrl}/api/v1/health`, {
      signal: AbortSignal.timeout(5000),
    })
      .then((res) => setHealth(res.ok ? "healthy" : "unhealthy"))
      .catch(() => setHealth("unhealthy"));
  }, [backendUrl]);

  useEffect(() => {
    const timer = setTimeout(() => runHealthCheck(), 0);
    return () => clearTimeout(timer);
  }, [runHealthCheck]);

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("title")}</h2>
        <p className="text-sm text-muted-foreground">{t("description")}</p>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">{t("healthStatus")}</CardTitle>
            <div className="flex items-center gap-2">
              <Badge
                variant={health === "healthy" ? "default" : health === "checking" ? "secondary" : "destructive"}
              >
                {health === "healthy" && t("healthy")}
                {health === "unhealthy" && t("unhealthy")}
                {health === "checking" && t("checking")}
              </Badge>
              <Button type="button" variant="ghost" size="icon" className="size-8" onClick={runHealthCheck}>
                <RefreshCw className="size-3.5" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-3">
            <div className="flex flex-col gap-1.5">
              <Label className="text-muted-foreground text-xs">{t("backendUrl")}</Label>
              <div className="flex items-center gap-2 rounded-md border bg-muted/50 px-3 py-2">
                <code className="flex-1 text-sm">{backendUrl}</code>
                <CopyButton text={backendUrl} />
              </div>
              <p className="text-xs text-muted-foreground">{t("backendUrlDesc")}</p>
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="text-muted-foreground text-xs">{t("imBridgeUrl")}</Label>
              <div className="flex items-center gap-2 rounded-md border bg-muted/50 px-3 py-2">
                <code className="flex-1 text-sm">{imBridgeUrl}</code>
                <CopyButton text={imBridgeUrl} />
              </div>
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="text-muted-foreground text-xs">{t("marketplaceUrl")}</Label>
              <div className="flex items-center gap-2 rounded-md border bg-muted/50 px-3 py-2">
                <code className="flex-1 text-sm">{marketplaceUrl}</code>
                <CopyButton text={marketplaceUrl} />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("connectionTimeout")}</CardTitle>
          <CardDescription>{t("connectionTimeoutDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-timeout">{t("connectionTimeout")}</Label>
              <Input
                id="settings-timeout"
                type="number"
                min={1000}
                max={120000}
                step={1000}
                value={timeoutMs}
                onChange={(e) => setTimeoutMs(Number(e.target.value))}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-retry">{t("retryCount")}</Label>
              <Input
                id="settings-retry"
                type="number"
                min={0}
                max={10}
                value={retryCount}
                onChange={(e) => setRetryCount(Number(e.target.value))}
              />
              <p className="text-xs text-muted-foreground">{t("retryCountDesc")}</p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
