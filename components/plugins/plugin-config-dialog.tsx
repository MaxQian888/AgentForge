"use client";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { usePluginStore } from "@/lib/stores/plugin-store";
import type { PluginRecord } from "@/lib/stores/plugin-store";
import { PluginConfigForm } from "./plugin-config-form";

interface PluginConfigDialogProps {
  plugin: PluginRecord | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function PluginConfigDialog({
  plugin,
  open,
  onOpenChange,
}: PluginConfigDialogProps) {
  const updateConfig = usePluginStore((s) => s.updateConfig);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            Configure {plugin?.metadata.name ?? "Plugin"}
          </DialogTitle>
          <DialogDescription>
            Edit the plugin configuration. Changes take effect after saving.
          </DialogDescription>
        </DialogHeader>
        {plugin ? (
          <PluginConfigForm
            key={plugin.metadata.id}
            plugin={plugin}
            onSave={async (config) => {
              await updateConfig(plugin.metadata.id, config);
              onOpenChange(false);
            }}
            onCancel={() => onOpenChange(false)}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
