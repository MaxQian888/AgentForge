"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useSavedViewStore, type SavedView } from "@/lib/stores/saved-view-store";

export function ViewShareDialog({
  open,
  onOpenChange,
  projectId,
  view,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  view: SavedView | null;
}) {
  const updateView = useSavedViewStore((state) => state.updateView);
  const [roleIds, setRoleIds] = useState("");
  const [memberIds, setMemberIds] = useState("");

  if (!view) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Share View</DialogTitle>
          <DialogDescription>Share this view with role IDs or member IDs.</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-2">
            <Label>Role IDs</Label>
            <Input value={roleIds} onChange={(event) => setRoleIds(event.target.value)} placeholder="reviewer, lead" />
          </div>
          <div className="space-y-2">
            <Label>Member IDs</Label>
            <Input value={memberIds} onChange={(event) => setMemberIds(event.target.value)} placeholder="member-1, member-2" />
          </div>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            onClick={async () => {
              await updateView(projectId, view.id, {
                sharedWith: {
                  roleIds: roleIds.split(",").map((item) => item.trim()).filter(Boolean),
                  memberIds: memberIds.split(",").map((item) => item.trim()).filter(Boolean),
                },
              });
              onOpenChange(false);
            }}
          >
            Save sharing
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
