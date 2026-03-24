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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { usePlatformCapability } from "@/hooks/use-platform-capability";
import { usePluginStore } from "@/lib/stores/plugin-store";

interface PluginInstallDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function PluginInstallDialog({
  open,
  onOpenChange,
}: PluginInstallDialogProps) {
  const [path, setPath] = useState("");
  const installLocal = usePluginStore((s) => s.installLocal);
  const loading = usePluginStore((s) => s.loading);
  const { isDesktop, selectFiles } = usePlatformCapability();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!path.trim()) return;
    await installLocal(path.trim());
    setPath("");
    onOpenChange(false);
  };

  const handleBrowse = async () => {
    const result = await selectFiles({
      directory: true,
      multiple: false,
      title: "Select a local plugin directory",
    });

    if (result.ok && result.mode === "desktop" && result.paths[0]) {
      setPath(result.paths[0]);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(e) => void handleSubmit(e)}>
          <DialogHeader>
            <DialogTitle>Install Local Plugin</DialogTitle>
            <DialogDescription>
              Provide the local filesystem path to the plugin directory or
              manifest file.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="plugin-path">Plugin Path</Label>
              <div className="flex gap-2">
                <Input
                  id="plugin-path"
                  placeholder="/path/to/plugin"
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => void handleBrowse()}
                >
                  Browse
                </Button>
              </div>
              {!isDesktop ? (
                <p className="text-xs text-muted-foreground">
                  Native path browsing is only available in the desktop shell.
                </p>
              ) : null}
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={!path.trim() || loading}>
              {loading ? "Installing..." : "Install"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
