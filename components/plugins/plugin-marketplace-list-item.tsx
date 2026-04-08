"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { PluginIcon } from "./plugin-icon";
import type {
  MarketplacePluginEntry,
  PluginKind,
} from "@/lib/stores/plugin-store";
import { Download } from "lucide-react";

interface PluginMarketplaceListItemProps {
  entry: MarketplacePluginEntry;
  onInstall?: (entry: MarketplacePluginEntry) => void;
  onSelect?: (entry: MarketplacePluginEntry) => void;
  selected?: boolean;
  loading?: boolean;
}

export function PluginMarketplaceListItem({
  entry,
  onInstall,
  onSelect,
  selected = false,
  loading = false,
}: PluginMarketplaceListItemProps) {
  const t = useTranslations("plugins");

  const kind = (entry.kind ?? "ToolPlugin") as PluginKind;

  return (
    <div
      role="button"
      tabIndex={0}
      className={cn(
        "group flex items-center gap-3 border-l-2 px-3 py-2.5 transition-colors cursor-pointer",
        selected
          ? "border-l-primary bg-accent/50"
          : "border-l-transparent hover:bg-accent/30",
      )}
      onClick={() => onSelect?.(entry)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelect?.(entry);
        }
      }}
    >
      <PluginIcon name={entry.name} kind={kind} size="md" />

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-semibold leading-tight">
            {entry.name}
          </span>
          <span className="shrink-0 text-xs text-muted-foreground">
            v{entry.version}
          </span>
        </div>
        <p className="truncate text-xs text-muted-foreground leading-relaxed">
          {entry.description}
        </p>
        <div className="mt-0.5 flex items-center gap-2 text-[11px] text-muted-foreground">
          <Badge variant="outline" className="h-4 px-1 py-0 text-[10px]">
            {kind.replace("Plugin", "")}
          </Badge>
          {entry.author ? (
            <span>{entry.author}</span>
          ) : null}
          {entry.runtime ? (
            <>
              <span className="text-border">·</span>
              <span>{entry.runtime}</span>
            </>
          ) : null}
        </div>
      </div>

      <div
        className="flex shrink-0 items-center gap-1.5"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") e.stopPropagation();
        }}
      >
        {entry.installed ? (
          <Badge className="h-5 bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 text-[10px]">
            {t("installed")}
          </Badge>
        ) : entry.installable === false ? (
          <Badge variant="secondary" className="h-5 text-[10px]">
            {t("browseOnly")}
          </Badge>
        ) : (
          <Button
            variant="outline"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={() => onInstall?.(entry)}
            disabled={loading}
          >
            <Download className="mr-1 size-3" />
            {t("install")}
          </Button>
        )}
      </div>
    </div>
  );
}
