"use client";

import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FieldError } from "@/components/shared/field-error";
import type { ProjectSectionProps } from "./types";

const WEBHOOK_EVENTS = ["push", "pr_opened", "pr_merged", "review_completed"];

export function SectionWebhook({ draft, patchDraft, validationErrors, clearValidationError }: ProjectSectionProps) {
  const t = useTranslations("settings");

  const patchWebhook = (field: string, value: unknown) => {
    patchDraft((d) => ({
      ...d,
      settings: {
        ...d.settings,
        webhook: { ...d.settings.webhook, [field]: value },
      },
    }));
  };

  const toggleEvent = (event: string) => {
    patchDraft((d) => ({
      ...d,
      settings: {
        ...d.settings,
        webhook: {
          ...d.settings.webhook,
          events: d.settings.webhook.events.includes(event)
            ? d.settings.webhook.events.filter((e) => e !== event)
            : [...d.settings.webhook.events, event],
        },
      },
    }));
    clearValidationError("webhookEvents");
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("webhookConfig")}</h2>
        <p className="text-sm text-muted-foreground">{t("webhookConfigDesc")}</p>
      </div>

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-webhook-url">{t("webhookUrl")}</Label>
              <Input
                id="settings-webhook-url"
                value={draft.settings.webhook.url}
                aria-invalid={Boolean(validationErrors.webhookUrl)}
                placeholder={t("webhookUrlPlaceholder")}
                onChange={(e) => {
                  patchWebhook("url", e.target.value);
                  clearValidationError("webhookUrl");
                }}
              />
              <FieldError message={validationErrors.webhookUrl} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-webhook-secret">{t("webhookSecret")}</Label>
              <Input
                id="settings-webhook-secret"
                type="password"
                value={draft.settings.webhook.secret}
                placeholder={t("webhookSecretPlaceholder")}
                onChange={(e) => patchWebhook("secret", e.target.value)}
              />
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>{t("webhookEvents")}</Label>
              <div className="flex flex-wrap gap-2">
                {WEBHOOK_EVENTS.map((event) => (
                  <Button
                    key={event}
                    type="button"
                    size="sm"
                    variant={draft.settings.webhook.events.includes(event) ? "default" : "outline"}
                    onClick={() => toggleEvent(event)}
                  >
                    {event}
                  </Button>
                ))}
              </div>
              <FieldError message={validationErrors.webhookEvents} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("webhookActive")}</Label>
              <Select
                value={draft.settings.webhook.active ? "yes" : "no"}
                onValueChange={(v) => patchWebhook("active", v === "yes")}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("active")}</SelectItem>
                  <SelectItem value="no">{t("inactive")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
