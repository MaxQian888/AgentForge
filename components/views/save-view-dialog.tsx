"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("views");
  const tCommon = useTranslations("common");
  const createView = useSavedViewStore((state) => state.createView);
  const [name, setName] = useState("");
  const [shared, setShared] = useState(false);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("saveViewDialog.title")}</DialogTitle>
          <DialogDescription>{t("saveViewDialog.description")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-2">
            <Label>{t("saveViewDialog.nameLabel")}</Label>
            <Input value={name} onChange={(event) => setName(event.target.value)} placeholder={t("saveViewDialog.namePlaceholder")} />
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={shared} onChange={(event) => setShared(event.target.checked)} />
            {t("saveViewDialog.sharedLabel")}
          </label>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {tCommon("action.cancel")}
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
            {tCommon("action.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
