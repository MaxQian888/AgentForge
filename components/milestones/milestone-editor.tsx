"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("milestones");
  const tc = useTranslations("common");
  const createMilestone = useMilestoneStore((state) => state.createMilestone);
  const [name, setName] = useState("");
  const [targetDate, setTargetDate] = useState("");
  const [status, setStatus] = useState("planned");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("editor.title")}</DialogTitle>
          <DialogDescription>{t("editor.description")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="space-y-2">
            <Label>{t("editor.name")}</Label>
            <Input value={name} onChange={(event) => setName(event.target.value)} placeholder={t("editor.namePlaceholder")} />
          </div>
          <div className="space-y-2">
            <Label>{t("editor.targetDate")}</Label>
            <Input type="date" value={targetDate} onChange={(event) => setTargetDate(event.target.value)} />
          </div>
          <div className="space-y-2">
            <Label>{t("editor.status")}</Label>
            <select className="h-10 w-full rounded-md border bg-background px-3 text-sm" value={status} onChange={(event) => setStatus(event.target.value)}>
              {["planned", "in_progress", "completed", "missed"].map((item) => (
                <option key={item} value={item}>
                  {t(`status.${item}`)}
                </option>
              ))}
            </select>
          </div>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {tc("action.cancel")}
          </Button>
          <Button
            type="button"
            disabled={!name.trim()}
            onClick={async () => {
              await createMilestone(projectId, { name, targetDate: targetDate || null, status, description: "" });
              onOpenChange(false);
            }}
          >
            {tc("action.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
