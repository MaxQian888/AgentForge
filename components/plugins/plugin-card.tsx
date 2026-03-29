"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
} from "lucide-react";

const stateColors: Record<PluginLifecycleState, string> = {
  installed: "bg-blue-500/15 text-blue-700 dark:text-blue-400",
  enabled: "bg-green-500/15 text-green-700 dark:text-green-400",
  activating: "bg-cyan-500/15 text-cyan-700 dark:text-cyan-400",
  active: "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400",
  degraded: "bg-orange-500/15 text-orange-700 dark:text-orange-400",
  disabled: "bg-zinc-500/15 text-zinc-600 dark:text-zinc-400",
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

  let runtimeHint = t("pluginCard.hintReady");
  if (!isExecutable) {
    runtimeHint = t("pluginCard.hintNoRuntime");
  } else if (state === "disabled") {
    runtimeHint = t("pluginCard.hintDisabled");
  } else if (state === "installed") {
    runtimeHint = t("pluginCard.hintInstalled");
  } else if (state === "activating") {
    runtimeHint = t("pluginCard.hintActivating");
  }

  return (
    <Card className={cn(selected && "border-primary/60 shadow-sm")}>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-2">
          <CardTitle className="text-base">{plugin.metadata.name}</CardTitle>
          <div className="flex items-center gap-1.5">
            <Badge variant="outline" className="text-xs">
              {plugin.kind}
            </Badge>
            <Badge
              variant="secondary"
              className={cn("text-xs", stateColors[state])}
            >
              {state}
            </Badge>
          </div>
        </div>
        <CardDescription className="text-xs">
          v{plugin.metadata.version}
          {plugin.source.type === "builtin" ? " \u00b7 built-in" : ""}
          {plugin.last_error ? ` \u00b7 Error: ${plugin.last_error}` : ""}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {plugin.metadata.description ? (
          <p className="text-sm text-muted-foreground line-clamp-2">
            {plugin.metadata.description}
          </p>
        ) : null}

        <PluginTrustBadge source={plugin.source} />

        <div className="grid gap-1 text-xs text-muted-foreground">
          <p>{t("pluginCard.runtime")}: {plugin.spec.runtime}</p>
          <p>{t("pluginCard.host")}: {plugin.runtime_host ?? t("pluginCard.hostNotExecutable")}</p>
          <p>{t("pluginCard.source")}: {plugin.source.type}</p>
        </div>

        <p className="text-xs text-muted-foreground">{runtimeHint}</p>

        <div className="flex flex-wrap gap-1.5">
          {/* Enable / Disable toggle */}
          {canDisable ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void disablePlugin(id)}
            >
              <Pause className="mr-1 size-3.5" />
              {t("pluginCard.disable")}
            </Button>
          ) : canEnable ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void enablePlugin(id)}
            >
              <Play className="mr-1 size-3.5" />
              {t("pluginCard.enable")}
            </Button>
          ) : null}

          {/* Activate */}
          {canActivate ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void activatePlugin(id)}
            >
              <Zap className="mr-1 size-3.5" />
              {t("pluginCard.activate")}
            </Button>
          ) : null}

          {/* Restart */}
          {canRestart ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void restartPlugin(id)}
            >
              <RotateCcw className="mr-1 size-3.5" />
              {t("pluginCard.restart")}
            </Button>
          ) : null}

          {/* Health Check */}
          {canCheckHealth ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void checkHealth(id)}
            >
              <HeartPulse className="mr-1 size-3.5" />
              {t("pluginCard.health")}
            </Button>
          ) : null}

          {/* Deactivate */}
          {canDeactivate ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void deactivatePlugin(id)}
            >
              <Square className="mr-1 size-3.5" />
              {t("pluginCard.deactivate")}
            </Button>
          ) : null}

          {/* Update */}
          {hasUpdate ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void updatePlugin(plugin)}
            >
              <ArrowUpCircle className="mr-1 size-3.5" />
              {t("pluginCard.update")}
            </Button>
          ) : null}

          {/* Invoke */}
          {canInvoke ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onInvoke?.(plugin)}
            >
              <Terminal className="mr-1 size-3.5" />
              {t("pluginCard.invoke")}
            </Button>
          ) : null}

          {/* Configure */}
          <Button
            variant="outline"
            size="sm"
            onClick={() => onConfigure?.(plugin)}
          >
            <Settings className="mr-1 size-3.5" />
            {t("pluginCard.configure")}
          </Button>

          <Button
            variant={selected ? "default" : "ghost"}
            size="sm"
            onClick={() => onSelect?.(plugin)}
          >
            {t("pluginCard.details")}
          </Button>

          {/* Uninstall */}
          <Button
            variant="destructive"
            size="sm"
            onClick={() => void uninstallPlugin(id)}
          >
            <Trash2 className="mr-1 size-3.5" />
            {t("pluginCard.uninstall")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
