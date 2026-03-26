"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useSavedViewStore } from "@/lib/stores/saved-view-store";

export function SaveViewDialog({
  open,
  onOpenChange,
  projectId,
  config,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  config: unknown;
}) {
  const createView = useSavedViewStore((state) => state.createView);
  const [name, setName] = useState("");
  const [shared, setShared] = useState(false);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Save View</DialogTitle>
          <DialogDescription>Persist the current workspace layout and filters.</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="My triage view" />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={shared} onChange={(event) => setShared(event.target.checked)} />
            Shared with project members
          </label>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            disabled={!name.trim()}
            onClick={async () => {
              await createView(projectId, {
                name,
                config,
                isDefault: false,
                sharedWith: shared ? { roleIds: [], memberIds: [] } : {},
              });
              setName("");
              setShared(false);
              onOpenChange(false);
            }}
          >
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
