"use client";

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
import type { PluginRecord, PluginLifecycleState } from "@/lib/stores/plugin-store";
import { usePluginStore } from "@/lib/stores/plugin-store";
import {
  Play,
  Pause,
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
  onSelect?: (plugin: PluginRecord) => void;
  selected?: boolean;
}

export function PluginCard({
  plugin,
  onConfigure,
  onSelect,
  selected = false,
}: PluginCardProps) {
  const enablePlugin = usePluginStore((s) => s.enablePlugin);
  const disablePlugin = usePluginStore((s) => s.disablePlugin);
  const activatePlugin = usePluginStore((s) => s.activatePlugin);
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
  const canCheckHealth =
    isExecutable && (state === "active" || state === "degraded");

  let runtimeHint = "This plugin is ready to be managed from the installed registry.";
  if (!isExecutable) {
    runtimeHint =
      "Runtime actions unavailable: this plugin does not use an executable runtime host in the current platform phase.";
  } else if (state === "disabled") {
    runtimeHint = "Enable this plugin before activation or health checks are available.";
  } else if (state === "installed") {
    runtimeHint = "Installed but not enabled yet.";
  } else if (state === "activating") {
    runtimeHint = "Plugin is activating. Runtime checks will become available once activation finishes.";
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

        <div className="grid gap-1 text-xs text-muted-foreground">
          <p>Runtime: {plugin.spec.runtime}</p>
          <p>Host: {plugin.runtime_host ?? "Not executable"}</p>
          <p>Source: {plugin.source.type}</p>
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
              Disable
            </Button>
          ) : canEnable ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void enablePlugin(id)}
            >
              <Play className="mr-1 size-3.5" />
              Enable
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
              Activate
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
              Restart
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
              Health
            </Button>
          ) : null}

          {/* Configure */}
          <Button
            variant="outline"
            size="sm"
            onClick={() => onConfigure?.(plugin)}
          >
            <Settings className="mr-1 size-3.5" />
            Configure
          </Button>

          <Button
            variant={selected ? "default" : "ghost"}
            size="sm"
            onClick={() => onSelect?.(plugin)}
          >
            Details
          </Button>

          {/* Uninstall */}
          <Button
            variant="destructive"
            size="sm"
            onClick={() => void uninstallPlugin(id)}
          >
            <Trash2 className="mr-1 size-3.5" />
            Uninstall
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
