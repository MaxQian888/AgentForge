"use client";

import { useTranslations } from "next-intl";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { PluginLifecycleState, PluginRecord } from "@/lib/stores/plugin-store";
import { usePluginStore } from "@/lib/stores/plugin-store";

interface PluginEnableToggleProps {
  plugin: PluginRecord;
  className?: string;
  showLabel?: boolean;
}

const ENABLED_STATES: PluginLifecycleState[] = [
  "enabled",
  "active",
  "activating",
  "degraded",
];

const TOGGLE_LOCKED_STATES: PluginLifecycleState[] = ["activating"];

export function PluginEnableToggle({
  plugin,
  className,
  showLabel = true,
}: PluginEnableToggleProps) {
  const t = useTranslations("plugins");
  const enablePlugin = usePluginStore((s) => s.enablePlugin);
  const disablePlugin = usePluginStore((s) => s.disablePlugin);

  const enabled = ENABLED_STATES.includes(plugin.lifecycle_state);
  const locked = TOGGLE_LOCKED_STATES.includes(plugin.lifecycle_state);
  const labelKey = enabled ? "pluginCard.disable" : "pluginCard.enable";

  const lockReason = locked ? t("pluginCard.hintActivating") : undefined;

  const handleChange = (checked: boolean) => {
    if (locked) return;
    if (checked) {
      void enablePlugin(plugin.metadata.id);
    } else {
      void disablePlugin(plugin.metadata.id);
    }
  };

  const control = (
    <Switch
      size="sm"
      checked={enabled}
      disabled={locked}
      onCheckedChange={handleChange}
      aria-label={t(labelKey)}
      data-testid={`plugin-enable-toggle-${plugin.metadata.id}`}
    />
  );

  return (
    <div className={cn("inline-flex items-center gap-2", className)}>
      {lockReason ? (
        <TooltipProvider>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex">{control}</span>
            </TooltipTrigger>
            <TooltipContent>{lockReason}</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      ) : (
        control
      )}
      {showLabel && (
        <span className="text-[11px] font-medium text-muted-foreground">
          {t(labelKey)}
        </span>
      )}
    </div>
  );
}
