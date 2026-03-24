"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface InviteMemberDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onInvite: (data: {
    email: string;
    role: string;
    type: "human" | "agent";
  }) => Promise<void>;
}

export function InviteMemberDialog({
  open,
  onOpenChange,
  onInvite,
}: InviteMemberDialogProps) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("developer");
  const [type, setType] = useState<"human" | "agent">("human");
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!email) return;
    setSubmitting(true);
    try {
      await onInvite({ email, role, type });
      setEmail("");
      setRole("developer");
      setType("human");
      onOpenChange(false);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Team Member</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>Email or Name</Label>
            <Input
              placeholder="member@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label>Type</Label>
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={type}
              onChange={(e) => setType(e.target.value as "human" | "agent")}
            >
              <option value="human">Human</option>
              <option value="agent">Agent</option>
            </select>
          </div>
          <div className="flex flex-col gap-2">
            <Label>Role</Label>
            <select
              className="h-10 rounded-md border bg-background px-3 text-sm"
              value={role}
              onChange={(e) => setRole(e.target.value)}
            >
              <option value="admin">Admin</option>
              <option value="developer">Developer</option>
              <option value="reviewer">Reviewer</option>
              <option value="viewer">Viewer</option>
            </select>
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
          <Button
            type="button"
            disabled={!email || submitting}
            onClick={() => void handleSubmit()}
          >
            {submitting ? "Adding..." : "Add Member"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
