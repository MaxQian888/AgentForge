"use client";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { Shield, Lock, ArrowUpCircle } from "lucide-react";
import { useTranslations } from "next-intl";
import type { PluginSource } from "@/lib/stores/plugin-store";

interface PluginTrustBadgeProps {
  source: PluginSource;
}

const trustColors: Record<string, string> = {
  verified: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  unknown: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  untrusted: "bg-red-500/15 text-red-700 dark:text-red-400",
};

const approvalColors: Record<string, string> = {
  approved: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  pending: "bg-yellow-500/15 text-yellow-700 dark:text-yellow-400",
  rejected: "bg-red-500/15 text-red-700 dark:text-red-400",
  "not-required": "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

export function PluginTrustBadge({ source }: PluginTrustBadgeProps) {
  const t = useTranslations("plugins");
  const trustStatus = source.trust?.status ?? "unknown";
  const approvalState = source.trust?.approvalState ?? "not-required";

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Badge
        variant="secondary"
        className={cn("text-xs", trustColors[trustStatus])}
      >
        {trustStatus}
      </Badge>

      <Badge
        variant="secondary"
        className={cn("text-xs", approvalColors[approvalState])}
      >
        {approvalState}
      </Badge>

      {source.digest ? (
        <Shield className="size-3.5 text-muted-foreground" />
      ) : null}

      {source.signature ? (
        <Lock className="size-3.5 text-muted-foreground" />
      ) : null}

      {source.release?.availableVersion ? (
        <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
          <ArrowUpCircle className="size-3.5" />
          {t("trustBadge.updateAvailable", {
            version: source.release.availableVersion,
          })}
        </span>
      ) : null}
    </div>
  );
}
