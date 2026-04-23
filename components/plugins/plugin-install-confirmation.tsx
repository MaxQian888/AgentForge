"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useTranslations } from "next-intl";
import { Globe, FolderOpen, AlertTriangle } from "lucide-react";
import type { PluginSourceType, PluginPermissions } from "@/lib/stores/plugin-store";

interface PluginInstallConfirmationProps {
  sourceType: PluginSourceType;
  sourceLabel: string;
  permissions?: PluginPermissions;
  unsigned?: boolean;
  onConfirm: () => void;
  onBack: () => void;
}

export function PluginInstallConfirmation({
  sourceType,
  sourceLabel,
  permissions,
  unsigned,
  onConfirm,
  onBack,
}: PluginInstallConfirmationProps) {
  const t = useTranslations("plugins");
  const hasNetwork =
    permissions?.network?.required && permissions.network.domains?.length;
  const hasFilesystem =
    permissions?.filesystem?.required &&
    permissions.filesystem.allowed_paths?.length;

  return (
    <div className="grid gap-4">
      {/* Source summary */}
      <div className="flex items-start gap-2 rounded-md border p-3">
        <Badge variant="outline" className="shrink-0 capitalize">
          {sourceType}
        </Badge>
        <span className="text-sm break-all">{sourceLabel}</span>
      </div>

      {/* Permissions */}
      {(hasNetwork || hasFilesystem) && (
        <div className="grid gap-2">
          <p className="text-sm font-medium">{t("installConfirmation.requestedPermissions")}</p>
          {hasNetwork && (
            <div className="flex items-start gap-2 rounded-md border p-3 text-sm">
              <Globe className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <div className="grid gap-1">
                <span className="font-medium">{t("installConfirmation.networkAccess")}</span>
                <ul className="list-disc pl-4 text-xs text-muted-foreground">
                  {permissions!.network!.domains!.map((d) => (
                    <li key={d}>{d}</li>
                  ))}
                </ul>
              </div>
            </div>
          )}
          {hasFilesystem && (
            <div className="flex items-start gap-2 rounded-md border p-3 text-sm">
              <FolderOpen className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <div className="grid gap-1">
                <span className="font-medium">{t("installConfirmation.filesystemAccess")}</span>
                <ul className="list-disc pl-4 text-xs text-muted-foreground">
                  {permissions!.filesystem!.allowed_paths!.map((p) => (
                    <li key={p}>{p}</li>
                  ))}
                </ul>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Trust warning */}
      {unsigned && (
        <div
          className={cn(
            "flex items-start gap-2 rounded-md border p-3",
            "border-amber-500/50 bg-amber-500/10 text-amber-700 dark:text-amber-400"
          )}
        >
          <AlertTriangle className="mt-0.5 size-4 shrink-0" />
          <p className="text-sm">{t("installConfirmation.unsignedWarning")}</p>
        </div>
      )}

      {/* Actions */}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onBack}>
          {t("installConfirmation.back")}
        </Button>
        <Button type="button" onClick={onConfirm}>
          {unsigned ? t("installConfirmation.installAnyway") : t("installConfirmation.confirmInstall")}
        </Button>
      </div>
    </div>
  );
}
