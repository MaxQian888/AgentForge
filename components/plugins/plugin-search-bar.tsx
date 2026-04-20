"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import {
  usePluginStore,
  type PluginPanelFilters,
} from "@/lib/stores/plugin-store";
import {
  FolderOpen,
  RefreshCw,
  Search,
  ScanLine,
  SlidersHorizontal,
  X,
} from "lucide-react";

interface PluginSearchBarProps {
  installedCount: number;
  builtinCount: number;
  marketplaceCount: number;
  remoteCount: number;
  loading: boolean;
  onRefresh: () => void;
  onInstallLocal: () => void;
  onRescan?: () => void;
}

export function PluginSearchBar({
  installedCount,
  builtinCount,
  marketplaceCount,
  remoteCount,
  loading,
  onRefresh,
  onInstallLocal,
  onRescan,
}: PluginSearchBarProps) {
  const t = useTranslations("plugins");
  const filters = usePluginStore((s) => s.filters);
  const viewCategory = usePluginStore((s) => s.viewCategory);
  const setFilters = usePluginStore((s) => s.setFilters);
  const resetFilters = usePluginStore((s) => s.resetFilters);
  const setViewCategory = usePluginStore((s) => s.setViewCategory);

  const hasActiveFilters =
    filters.kind !== "all" ||
    filters.lifecycleState !== "all" ||
    filters.runtimeHost !== "all" ||
    filters.sourceType !== "all";

  const categories = [
    { key: "installed" as const, label: t("tabInstalled"), count: installedCount },
    { key: "builtin" as const, label: t("tabBuiltIn"), count: builtinCount },
    { key: "marketplace" as const, label: t("tabMarketplace"), count: marketplaceCount },
    { key: "remote" as const, label: t("tabRemote"), count: remoteCount },
  ];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            aria-label={t("searchPlugins")}
            className="h-8 pl-9 pr-8 text-sm"
            value={filters.query}
            onChange={(e) => setFilters({ query: e.target.value })}
            placeholder={t("searchPlaceholder")}
          />
          {filters.query ? (
            <button
              type="button"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground hover:text-foreground"
              onClick={() => setFilters({ query: "" })}
              aria-label={t("clearFilters")}
            >
              <X className="size-3.5" />
            </button>
          ) : null}
        </div>

        <Popover>
          <PopoverTrigger asChild>
            <Button
              variant={hasActiveFilters ? "secondary" : "outline"}
              size="sm"
              className="h-8 w-8 p-0"
            >
              <SlidersHorizontal className="size-3.5" />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" className="w-64 space-y-3">
            <div className="space-y-1.5">
              <label className="text-xs font-medium">{t("kind")}</label>
              <Select
                value={filters.kind}
                onValueChange={(v) =>
                  setFilters({ kind: v as PluginPanelFilters["kind"] })
                }
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("allKinds")}</SelectItem>
                  <SelectItem value="ToolPlugin">Tool</SelectItem>
                  <SelectItem value="RolePlugin">Role</SelectItem>
                  <SelectItem value="WorkflowPlugin">Workflow</SelectItem>
                  <SelectItem value="IntegrationPlugin">Integration</SelectItem>
                  <SelectItem value="ReviewPlugin">Review</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium">{t("lifecycle")}</label>
              <Select
                value={filters.lifecycleState}
                onValueChange={(v) =>
                  setFilters({
                    lifecycleState: v as PluginPanelFilters["lifecycleState"],
                  })
                }
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("allStates")}</SelectItem>
                  <SelectItem value="installed">installed</SelectItem>
                  <SelectItem value="enabled">enabled</SelectItem>
                  <SelectItem value="activating">activating</SelectItem>
                  <SelectItem value="active">active</SelectItem>
                  <SelectItem value="degraded">degraded</SelectItem>
                  <SelectItem value="disabled">disabled</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium">{t("hostFilter")}</label>
              <Select
                value={filters.runtimeHost}
                onValueChange={(v) =>
                  setFilters({
                    runtimeHost: v as PluginPanelFilters["runtimeHost"],
                  })
                }
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("allHosts")}</SelectItem>
                  <SelectItem value="go-orchestrator">go-orchestrator</SelectItem>
                  <SelectItem value="ts-bridge">ts-bridge</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs font-medium">{t("source")}</label>
              <Select
                value={filters.sourceType}
                onValueChange={(v) =>
                  setFilters({
                    sourceType: v as PluginPanelFilters["sourceType"],
                  })
                }
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("allSources")}</SelectItem>
                  <SelectItem value="builtin">builtin</SelectItem>
                  <SelectItem value="local">local</SelectItem>
                  <SelectItem value="marketplace">marketplace</SelectItem>
                  <SelectItem value="registry">registry</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {hasActiveFilters ? (
              <Button
                variant="ghost"
                size="sm"
                className="h-7 w-full text-xs"
                onClick={resetFilters}
              >
                <X className="mr-1 size-3" />
                {t("clearFilters")}
              </Button>
            ) : null}
          </PopoverContent>
        </Popover>

        <Button
          variant="outline"
          size="sm"
          className="h-8 w-8 p-0"
          onClick={onRefresh}
          disabled={loading}
          title="Refresh plugin list"
        >
          <RefreshCw className={cn("size-3.5", loading && "animate-spin")} />
        </Button>

        {onRescan ? (
          <Button
            variant="outline"
            size="sm"
            className="h-8 gap-1.5 text-xs"
            onClick={onRescan}
            disabled={loading}
            title="Rescan plugins/ on disk for newly-added manifests"
          >
            <ScanLine className="size-3.5" />
            Rescan
          </Button>
        ) : null}

        <Button
          size="sm"
          className="h-8 gap-1.5 text-xs"
          onClick={onInstallLocal}
        >
          <FolderOpen className="size-3.5" />
          {t("installLocal")}
        </Button>
      </div>

      <div className="flex items-center gap-1">
        {categories.map((cat) => (
          <button
            key={cat.key}
            type="button"
            className={cn(
              "flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
              viewCategory === cat.key
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
            )}
            onClick={() => setViewCategory(cat.key)}
          >
            {cat.label}
            <Badge
              variant={viewCategory === cat.key ? "outline" : "secondary"}
              className={cn(
                "h-4 min-w-4 px-1 text-[10px]",
                viewCategory === cat.key && "border-primary-foreground/30 text-primary-foreground",
              )}
            >
              {cat.count}
            </Badge>
          </button>
        ))}
      </div>
    </div>
  );
}
