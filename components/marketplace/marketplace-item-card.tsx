"use client";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { ArrowUpCircle, CheckCircle, Download, Star } from "lucide-react";
import {
  typeDisplayLabel,
  type MarketplaceConsumptionRecord,
  type MarketplaceItem,
  type MarketplaceUpdateInfo,
} from "@/lib/stores/marketplace-store";

interface Props {
  item: MarketplaceItem;
  consumption?: MarketplaceConsumptionRecord | null;
  updateInfo?: MarketplaceUpdateInfo | null;
  selected?: boolean;
  onSelect?: (item: MarketplaceItem) => void;
  onInstall?: (item: MarketplaceItem) => void;
  onTagClick?: (tag: string) => void;
}

const TYPE_BADGE_VARIANTS: Record<string, string> = {
  plugin:
    "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  skill:
    "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  role: "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  workflow_template:
    "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
};


export function MarketplaceItemCard({
  item,
  consumption,
  updateInfo,
  selected,
  onSelect,
  onInstall,
  onTagClick,
}: Props) {
  const isInstalled = consumption?.status === "installed" && consumption.installed;
  const isBlocked = consumption?.status === "blocked";
  const hasUpdate = updateInfo?.hasUpdate === true;
  const actionLabel = hasUpdate
    ? "Update"
    : isInstalled
      ? consumption?.used
        ? "Manage"
        : "Installed"
      : isBlocked
        ? "Blocked"
        : "Install";

  return (
    <Card
      className={cn(
        "cursor-pointer transition-colors hover:border-primary/50",
        selected && "border-primary ring-1 ring-primary",
      )}
      onClick={() => onSelect?.(item)}
    >
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            {item.icon_url ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={item.icon_url} alt={item.name} className="w-8 h-8 rounded" />
            ) : (
              <div className="w-8 h-8 rounded bg-muted flex items-center justify-center text-xs font-bold shrink-0">
                {item.name.slice(0, 2).toUpperCase()}
              </div>
            )}
            <div className="min-w-0">
              <div className="flex items-center gap-1 flex-wrap">
                <span className="font-medium text-sm truncate">{item.name}</span>
                {item.is_verified && (
                  <CheckCircle className="w-3.5 h-3.5 text-blue-500 shrink-0" />
                )}
              </div>
              <span className="text-xs text-muted-foreground truncate block">
                by {item.author_name}
              </span>
            </div>
          </div>
          <span
            className={cn(
              "text-xs px-2 py-0.5 rounded-full font-medium shrink-0",
              TYPE_BADGE_VARIANTS[item.type],
            )}
          >
            {typeDisplayLabel(item.type)}
          </span>
        </div>
        {item.sourceType === "builtin" ? (
          <div className="mt-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            Built-in
          </div>
        ) : null}
        {hasUpdate ? (
          <div className="mt-1 flex items-center gap-1 text-[11px] text-blue-600 dark:text-blue-400">
            <ArrowUpCircle className="size-3" />
            Update: v{updateInfo!.latestVersion}
          </div>
        ) : null}
      </CardHeader>
      <CardContent className="pb-2">
        <p className="text-xs text-muted-foreground line-clamp-2">
          {item.description || "No description provided."}
        </p>
        {item.tags.length > 0 ? (
          <div className="mt-1.5 flex flex-wrap gap-1">
            {item.tags.slice(0, 3).map((tag) => (
              <button
                key={tag}
                type="button"
                className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] hover:bg-muted/80"
                onClick={(e) => {
                  e.stopPropagation();
                  onTagClick?.(tag);
                }}
              >
                {tag}
              </button>
            ))}
          </div>
        ) : null}
      </CardContent>
      <CardFooter className="flex items-center justify-between pt-0">
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <span className="flex items-center gap-1">
            <Star className="w-3 h-3" />
            {item.avg_rating.toFixed(1)}
          </span>
          <span className="flex items-center gap-1">
            <Download className="w-3 h-3" />
            {item.download_count}
          </span>
        </div>
        <Button
          size="sm"
          variant={hasUpdate ? "default" : isInstalled || isBlocked ? "secondary" : "default"}
          disabled={isBlocked}
          onClick={(e) => {
            e.stopPropagation();
            if (hasUpdate) {
              onInstall?.(item);
              return;
            }
            if (isInstalled) {
              onSelect?.(item);
              return;
            }
            onInstall?.(item);
          }}
        >
          {actionLabel}
        </Button>
      </CardFooter>
      {consumption?.warning || consumption?.failureReason ? (
        <div className="px-6 pb-4 text-[11px] text-muted-foreground">
          {consumption.warning ?? consumption.failureReason}
        </div>
      ) : null}
    </Card>
  );
}

