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
import type { AgentTeam, UpdateTeamInput } from "@/lib/stores/team-store";

interface EditTeamDialogProps {
  open: boolean;
  team: AgentTeam;
  onSave: (input: UpdateTeamInput) => Promise<void>;
  onClose: () => void;
}

export function EditTeamDialog({
  open,
  team,
  onSave,
  onClose,
}: EditTeamDialogProps) {
  const [name, setName] = useState(team.name);
  const [budget, setBudget] = useState(String(team.totalBudget));
  const [saving, setSaving] = useState(false);

  const budgetNum = parseFloat(budget);
  const budgetTooLow =
    !isNaN(budgetNum) && budgetNum < team.totalSpent;

  const handleSave = async () => {
    setSaving(true);
    try {
      const input: UpdateTeamInput = {};
      if (name.trim() !== team.name) input.name = name.trim();
      const parsed = parseFloat(budget);
      if (!isNaN(parsed) && parsed !== team.totalBudget)
        input.totalBudgetUsd = parsed;
      await onSave(input);
      onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Edit Team</DialogTitle>
          <DialogDescription>
            Update the team name or budget allocation.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-team-name">Team Name</Label>
            <Input
              id="edit-team-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="edit-team-budget">Budget (USD)</Label>
            <Input
              id="edit-team-budget"
              type="number"
              min={0}
              step={0.01}
              value={budget}
              onChange={(e) => setBudget(e.target.value)}
            />
            {budgetTooLow && (
              <p className="text-xs text-destructive">
                Budget cannot be less than already spent ($
                {team.totalSpent.toFixed(2)}).
              </p>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={handleSave}
            disabled={saving || budgetTooLow || !name.trim()}
          >
            {saving ? "Saving..." : "Save Changes"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
