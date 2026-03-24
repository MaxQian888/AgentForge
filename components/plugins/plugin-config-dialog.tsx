"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { usePluginStore } from "@/lib/stores/plugin-store";
import type { PluginRecord } from "@/lib/stores/plugin-store";

interface ConfigFormProps {
  plugin: PluginRecord;
  onClose: () => void;
}

function ConfigForm({ plugin, onClose }: ConfigFormProps) {
  const [configText, setConfigText] = useState(
    JSON.stringify(plugin.spec.config ?? {}, null, 2)
  );
  const [parseError, setParseError] = useState<string | null>(null);
  const updateConfig = usePluginStore((s) => s.updateConfig);

  const handleSave = async () => {
    try {
      const parsed = JSON.parse(configText) as Record<string, unknown>;
      setParseError(null);
      await updateConfig(plugin.metadata.id, parsed);
      onClose();
    } catch {
      setParseError("Invalid JSON");
    }
  };

  return (
    <>
      <div className="grid gap-3 py-4">
        <Label htmlFor="config-json">Configuration</Label>
        <textarea
          id="config-json"
          className="flex min-h-[200px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 font-mono"
          value={configText}
          onChange={(e) => {
            setConfigText(e.target.value);
            setParseError(null);
          }}
        />
        {parseError ? (
          <p className="text-sm text-destructive">{parseError}</p>
        ) : null}
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <Button onClick={() => void handleSave()}>Save</Button>
      </DialogFooter>
    </>
  );
}

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
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            Configure {plugin?.metadata.name ?? "Plugin"}
          </DialogTitle>
          <DialogDescription>
            Edit the plugin configuration as JSON. Changes take effect after
            saving.
          </DialogDescription>
        </DialogHeader>
        {plugin ? (
          <ConfigForm
            key={plugin.metadata.id}
            plugin={plugin}
            onClose={() => onOpenChange(false)}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
