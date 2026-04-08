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
import { PluginIcon } from "./plugin-icon";
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
  MoreVertical,
} from "lucide-react";

const stateDotColors: Record<PluginLifecycleState, string> = {
  installed: "bg-blue-500",
  enabled: "bg-green-500",
  activating: "bg-cyan-500 animate-pulse",
  active: "bg-emerald-500",
  degraded: "bg-orange-500",
  disabled: "bg-zinc-400",
};

const stateLabels: Record<PluginLifecycleState, string> = {
  installed: "Installed",
  enabled: "Enabled",
  activating: "Activating",
  active: "Active",
  degraded: "Degraded",
  disabled: "Disabled",
};

interface PluginListItemProps {
  plugin: PluginRecord;
  onConfigure?: (plugin: PluginRecord) => void;
  onInvoke?: (plugin: PluginRecord) => void;
  onSelect?: (plugin: PluginRecord) => void;
  selected?: boolean;
}

export function PluginListItem({
  plugin,
  onConfigure,
  onInvoke,
  onSelect,
  selected = false,
}: PluginListItemProps) {
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

  const canEnable = state === "installed" || state === "disabled";
  const canDisable =
    state === "enabled" ||
    state === "active" ||
    state === "activating" ||
    state === "degraded";
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
        "group flex items-center gap-3 border-l-2 px-3 py-2.5 transition-colors cursor-pointer",
        selected
          ? "border-l-primary bg-accent/50"
          : "border-l-transparent hover:bg-accent/30",
      )}
      onClick={() => onSelect?.(plugin)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelect?.(plugin);
        }
      }}
    >
      <PluginIcon name={plugin.metadata.name} kind={plugin.kind} size="md" />

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-semibold leading-tight">
            {plugin.metadata.name}
          </span>
          <span className="shrink-0 text-xs text-muted-foreground">
            v{plugin.metadata.version}
          </span>
          {hasUpdate ? (
            <ArrowUpCircle className="size-3.5 shrink-0 text-blue-500" />
          ) : null}
        </div>
        <p className="truncate text-xs text-muted-foreground leading-relaxed">
          {plugin.metadata.description || t("detailSidebar.selectPrompt")}
        </p>
        <div className="mt-0.5 flex items-center gap-2 text-[11px] text-muted-foreground">
          <Badge variant="outline" className="h-4 px-1 py-0 text-[10px]">
            {plugin.kind.replace("Plugin", "")}
          </Badge>
          <span className="flex items-center gap-1">
            <span className={cn("size-1.5 rounded-full", stateDotColors[state])} />
            {stateLabels[state]}
          </span>
          <span className="text-border">·</span>
          <span>{plugin.spec.runtime}</span>
        </div>
      </div>

      <div
        className="flex shrink-0 items-center gap-1"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") e.stopPropagation();
        }}
      >
        {canEnable ? (
          <Button
            variant="default"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={() => void enablePlugin(id)}
          >
            <Play className="mr-1 size-3" />
            {t("pluginCard.enable")}
          </Button>
        ) : canActivate ? (
          <Button
            variant="default"
            size="sm"
            className="h-6 px-2 text-[11px]"
            onClick={() => void activatePlugin(id)}
          >
            <Zap className="mr-1 size-3" />
            {t("pluginCard.activate")}
          </Button>
        ) : canDisable ? (
          <Button
            variant="outline"
            size="sm"
            className="h-6 px-2 text-[11px]"
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
            className="h-6 px-2 text-[11px] text-blue-600 border-blue-200 hover:bg-blue-50 dark:text-blue-400 dark:border-blue-800 dark:hover:bg-blue-950"
            onClick={() => void updatePlugin(plugin)}
          >
            {t("pluginCard.update")}
          </Button>
        ) : null}

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              className="h-6 w-6 p-0 opacity-0 group-hover:opacity-100 data-[state=open]:opacity-100"
              aria-label={t("moreActions")}
            >
              <MoreVertical className="size-3.5" />
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
