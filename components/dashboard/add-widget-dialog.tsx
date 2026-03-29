"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { DASHBOARD_WIDGETS } from "./widget-catalog";

export function AddWidgetDialog({
  open,
  onOpenChange,
  projectId,
  dashboardId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: string;
  dashboardId: string;
}) {
  const t = useTranslations("dashboard");
  const saveWidget = useDashboardStore((state) => state.saveWidget);
  const [widgetType, setWidgetType] = useState(DASHBOARD_WIDGETS[0].type);
  const selectedWidget =
    DASHBOARD_WIDGETS.find((widget) => widget.type === widgetType) ??
    DASHBOARD_WIDGETS[0];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("widget.addTitle")}</DialogTitle>
          <DialogDescription>{t("widget.addDescription")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          {DASHBOARD_WIDGETS.map((widget) => (
            <label
              key={widget.type}
              className="flex items-start gap-3 rounded-md border px-3 py-2 text-sm"
            >
              <input
                aria-label={t(widget.titleKey)}
                type="radio"
                checked={widgetType === widget.type}
                onChange={() => setWidgetType(widget.type)}
              />
              <div className="space-y-1">
                <div className="font-medium">{t(widget.titleKey)}</div>
                <div className="text-muted-foreground">
                  {t(widget.descriptionKey)}
                </div>
              </div>
            </label>
          ))}
        </div>
        <div className="rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground">
          {t(selectedWidget.descriptionKey)}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("widget.cancel")}
          </Button>
          <Button
            type="button"
            onClick={async () => {
              await saveWidget(projectId, dashboardId, {
                widgetType,
                config: selectedWidget.defaultConfig,
                position: { x: 0, y: 0, w: 1, h: 1 },
              });
              onOpenChange(false);
            }}
          >
            {t("widget.add")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
