"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useMilestoneStore } from "@/lib/stores/milestone-store";

export function MilestoneEditor({
  open,
  onOpenChange,
  projectId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
}) {
  const createMilestone = useMilestoneStore((state) => state.createMilestone);
  const [name, setName] = useState("");
  const [targetDate, setTargetDate] = useState("");
  const [status, setStatus] = useState("planned");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Milestone</DialogTitle>
          <DialogDescription>Define a roadmap milestone and target date.</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="v2.0 Release" />
          </div>
          <div className="space-y-2">
            <Label>Target Date</Label>
            <Input type="date" value={targetDate} onChange={(event) => setTargetDate(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>Status</Label>
            <select className="h-10 w-full rounded-md border bg-background px-3 text-sm" value={status} onChange={(event) => setStatus(event.target.value)}>
              {["planned", "in_progress", "completed", "missed"].map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </div>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            type="button"
            disabled={!name.trim()}
            onClick={async () => {
              await createMilestone(projectId, { name, targetDate: targetDate || null, status, description: "" });
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
