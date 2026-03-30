"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { PluginTrustBadge } from "./plugin-trust-badge";
import type { PluginRecord, PluginLifecycleState } from "@/lib/stores/plugin-store";
import { usePluginStore } from "@/lib/stores/plugin-store";
import {
  ArrowUpCircle,
  Play,
  Pause,
  Square,
  Terminal,
  Zap,
  RotateCcw,
  HeartPulse,
  Settings,
  Trash2,
  MoreHorizontal,
} from "lucide-react";

const stateColors: Record<PluginLifecycleState, string> = {
  installed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  enabled: "bg-green-500/15 text-green-700 dark:text-green-400",
  activating: "bg-cyan-500/15 text-cyan-700 dark:text-cyan-400",
  active: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  degraded: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  disabled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
};

const stateDotColors: Record<PluginLifecycleState, string> = {
  installed: "bg-blue-500",
  enabled: "bg-green-500",
  activating: "bg-cyan-500 animate-pulse",
  active: "bg-emerald-500",
  degraded: "bg-orange-500",
  disabled: "bg-zinc-400",
};

interface PluginCardProps {
  plugin: PluginRecord;
  onConfigure?: (plugin: PluginRecord) => void;
  onInvoke?: (plugin: PluginRecord) => void;
  onSelect?: (plugin: PluginRecord) => void;
  selected?: boolean;
}

export function PluginCard({
  plugin,
  onConfigure,
  onInvoke,
  onSelect,
  selected = false,
}: PluginCardProps) {
  const t = useTranslations("plugins");
  const enablePlugin = usePluginStore((s) => s.enablePlugin);
  const disablePlugin = usePluginStore((s) => s.disablePlugin);
  const activatePlugin = usePluginStore((s) => s.activatePlugin);
  const deactivatePlugin = usePluginStore((s) => s.deactivatePlugin);
  const updatePlugin = usePluginStore((s) => s.updatePlugin);
  const uninstallPlugin = usePluginStore((s) => s.uninstallPlugin);
  const checkHealth = usePluginStore((s) => s.checkHealth);
  const restartPlugin = usePluginStore((s) => s.restartPlugin);

  const id = plugin.metadata.id;
  const state = plugin.lifecycle_state;
  const isExecutable =
    plugin.spec.runtime !== "declarative" && Boolean(plugin.runtime_host);

  const isEnabled =
    state === "enabled" ||
    state === "active" ||
    state === "activating" ||
    state === "degraded";
  const canEnable = state === "installed" || state === "disabled";
  const canDisable = isEnabled;
  const canActivate = state === "enabled" && isExecutable;
  const canRestart = isExecutable && (state === "active" || state === "degraded");
  const canDeactivate = state === "active" && isExecutable;
  const canInvoke = state === "active" && isExecutable;
  const hasUpdate =
    Boolean(plugin.source.release?.availableVersion) &&
    plugin.source.release?.availableVersion !== plugin.metadata.version &&
    Boolean(plugin.source.path ?? plugin.resolved_source_path);
  const canCheckHealth =
    isExecutable && (state === "active" || state === "degraded");

  return (
    <div
      role="button"
      tabIndex={0}
      className={cn(
        "group relative flex cursor-pointer flex-col gap-3 rounded-xl border bg-card p-4 text-left transition-all hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        selected
          ? "border-primary/60 shadow-sm ring-1 ring-primary/20"
          : "border-border/50 hover:border-border",
      )}
      onClick={() => onSelect?.(plugin)}
      onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onSelect?.(plugin); } }}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className={cn("size-2 shrink-0 rounded-full", stateDotColors[state])} />
            <h3 className="truncate text-sm font-semibold leading-tight">
              {plugin.metadata.name}
            </h3>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            v{plugin.metadata.version}
            {plugin.source.type === "builtin" ? " \u00b7 built-in" : ""}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <Badge variant="outline" className="text-[10px] px-1.5 py-0">
            {plugin.kind.replace("Plugin", "")}
          </Badge>
          <Badge
            variant="secondary"
            className={cn("text-[10px] px-1.5 py-0", stateColors[state])}
          >
            {state}
          </Badge>
        </div>
      </div>

      {/* Description */}
      {plugin.metadata.description ? (
        <p className="text-xs leading-relaxed text-muted-foreground line-clamp-2">
          {plugin.metadata.description}
        </p>
      ) : null}

      {/* Meta row */}
      <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
        <span>{plugin.spec.runtime}</span>
        <span className="text-border">/</span>
        <span>{plugin.runtime_host ?? t("pluginCard.hostNotExecutable")}</span>
        <PluginTrustBadge source={plugin.source} />
      </div>

      {plugin.last_error ? (
        <p className="rounded-md bg-destructive/10 px-2 py-1 text-[11px] text-destructive line-clamp-1">
          {plugin.last_error}
        </p>
      ) : null}

      {/* Actions */}
      <div
        className="flex items-center gap-1.5"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") e.stopPropagation(); }}
      >
        {/* Primary action */}
        {canEnable ? (
          <Button
            variant="default"
            size="sm"
            className="h-7 text-xs"
            onClick={() => void enablePlugin(id)}
          >
            <Play className="mr-1 size-3" />
            {t("pluginCard.enable")}
          </Button>
        ) : canActivate ? (
          <Button
            variant="default"
            size="sm"
            className="h-7 text-xs"
            onClick={() => void activatePlugin(id)}
          >
            <Zap className="mr-1 size-3" />
            {t("pluginCard.activate")}
          </Button>
        ) : canDisable ? (
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            onClick={() => void disablePlugin(id)}
          >
            <Pause className="mr-1 size-3" />
            {t("pluginCard.disable")}
          </Button>
        ) : null}

        {hasUpdate ? (
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs text-blue-600 border-blue-200 hover:bg-blue-50 dark:text-blue-400 dark:border-blue-800 dark:hover:bg-blue-950"
            onClick={() => void updatePlugin(plugin)}
          >
            <ArrowUpCircle className="mr-1 size-3" />
            {t("pluginCard.update")}
          </Button>
        ) : null}

        {/* Overflow menu for secondary actions */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="sm" className="ml-auto h-7 w-7 p-0" aria-label={t("moreActions")}>
              <MoreHorizontal className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-40">
            {canInvoke ? (
              <DropdownMenuItem onClick={() => onInvoke?.(plugin)}>
                <Terminal className="mr-2 size-3.5" />
                {t("pluginCard.invoke")}
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem onClick={() => onConfigure?.(plugin)}>
              <Settings className="mr-2 size-3.5" />
              {t("pluginCard.configure")}
            </DropdownMenuItem>
            {canCheckHealth ? (
              <DropdownMenuItem onClick={() => void checkHealth(id)}>
                <HeartPulse className="mr-2 size-3.5" />
                {t("pluginCard.health")}
              </DropdownMenuItem>
            ) : null}
            {canRestart ? (
              <DropdownMenuItem onClick={() => void restartPlugin(id)}>
                <RotateCcw className="mr-2 size-3.5" />
                {t("pluginCard.restart")}
              </DropdownMenuItem>
            ) : null}
            {canDeactivate ? (
              <DropdownMenuItem onClick={() => void deactivatePlugin(id)}>
                <Square className="mr-2 size-3.5" />
                {t("pluginCard.deactivate")}
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="text-destructive focus:text-destructive"
              onClick={() => void uninstallPlugin(id)}
            >
              <Trash2 className="mr-2 size-3.5" />
              {t("pluginCard.uninstall")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
