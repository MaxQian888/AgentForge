"use client";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export interface DocLinkPickerItem {
  id: string;
  title: string;
  path?: string | null;
}

export function DocLinkPicker({
  open,
  onOpenChange,
  docs,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  docs: DocLinkPickerItem[];
  onPick: (pageId: string) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Link a document</DialogTitle>
          <DialogDescription>
            Select a requirement or design page to attach to this task.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3">
          {docs.map((doc) => (
            <button
              key={doc.id}
              type="button"
              className="rounded-lg border border-border/60 px-4 py-3 text-left hover:bg-accent/40"
              onClick={() => onPick(doc.id)}
            >
              <div className="font-medium">{doc.title}</div>
              {doc.path ? (
                <div className="text-xs text-muted-foreground">{doc.path}</div>
              ) : null}
            </button>
          ))}
          {docs.length === 0 ? (
            <div className="rounded-lg border border-dashed border-border/70 p-4 text-sm text-muted-foreground">
              No docs available to link yet.
            </div>
          ) : null}
        </div>
        <div className="flex justify-end">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
