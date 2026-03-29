"use client";

import { useTranslations } from "next-intl";
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
  const t = useTranslations("tasks");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("docPicker.title")}</DialogTitle>
          <DialogDescription>
            {t("docPicker.description")}
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
              {t("docPicker.empty")}
            </div>
          ) : null}
        </div>
        <div className="flex justify-end">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("docPicker.close")}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
