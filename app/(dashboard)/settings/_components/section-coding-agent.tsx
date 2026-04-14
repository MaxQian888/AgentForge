"use client";

import { useTranslations } from "next-intl";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
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

export function SectionCodingAgent({ draft, patchDraft, validationErrors, clearValidationError, project }: ProjectSectionProps) {
  const t = useTranslations("settings");

  const runtimeOptions = project.codingAgentCatalog?.runtimes ?? [];
  const selectedRuntime =
    runtimeOptions.find((o) => o.runtime === draft.settings.codingAgent.runtime) ?? runtimeOptions[0];
  const compatibleProviders = selectedRuntime?.compatibleProviders ?? [];
  const modelOptions =
    selectedRuntime?.modelOptions && selectedRuntime.modelOptions.length > 0
      ? selectedRuntime.modelOptions
      : selectedRuntime?.defaultModel
        ? [selectedRuntime.defaultModel]
        : [];
  const selectedDiagnostics = selectedRuntime?.diagnostics ?? [];

  const handleRuntimeChange = (nextRuntime: string) => {
    patchDraft((d) => {
      const nextOption = runtimeOptions.find((o) => o.runtime === nextRuntime);
      if (!nextOption) return d;
      return {
        ...d,
        settings: {
          ...d.settings,
          codingAgent: {
            runtime: nextRuntime,
            provider: nextOption.defaultProvider,
            model: nextOption.modelOptions?.[0] ?? nextOption.defaultModel,
          },
        },
      };
    });
    clearValidationError("runtime");
    clearValidationError("provider");
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">{t("codingAgentDefaults")}</h2>
      </div>

      <Card>
        <CardContent className="space-y-4 pt-6">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="flex flex-col gap-2">
              <Label>{t("runtime")}</Label>
              <Select value={draft.settings.codingAgent.runtime} onValueChange={handleRuntimeChange}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {runtimeOptions.map((o) => (
                    <SelectItem key={o.runtime} value={o.runtime} disabled={!o.available}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldError message={validationErrors.runtime} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("provider")}</Label>
              <Select
                value={draft.settings.codingAgent.provider}
                onValueChange={(v) => {
                  patchDraft((d) => ({
                    ...d,
                    settings: { ...d.settings, codingAgent: { ...d.settings.codingAgent, provider: v } },
                  }));
                  clearValidationError("provider");
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {compatibleProviders.map((p) => (
                    <SelectItem key={p} value={p}>{p}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldError message={validationErrors.provider} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("model")}</Label>
              <Select
                value={draft.settings.codingAgent.model}
                onValueChange={(v) => {
                  patchDraft((d) => ({
                    ...d,
                    settings: { ...d.settings, codingAgent: { ...d.settings.codingAgent, model: v } },
                  }));
                  clearValidationError("model");
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {modelOptions.map((m) => (
                    <SelectItem key={m} value={m}>{m}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldError message={validationErrors.model} />
            </div>
          </div>

          {selectedDiagnostics.length > 0 && (
            <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
              {selectedDiagnostics.map((d) => (
                <p key={`${d.code}-${d.message}`}>{d.message}</p>
              ))}
            </div>
          )}

          <div className="grid gap-3 md:grid-cols-2">
            {runtimeOptions.map((o) => (
              <div key={o.runtime} className="rounded-md border p-4 text-sm">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="font-medium">{o.label}</p>
                    <p className="text-muted-foreground">
                      {o.defaultProvider} / {o.defaultModel}
                    </p>
                    {(o.modelOptions?.length ?? 0) > 1 && (
                      <p className="mt-1 text-xs text-muted-foreground">
                        {(o.modelOptions ?? []).join(", ")}
                      </p>
                    )}
                  </div>
                  <Badge variant={o.available ? "default" : "secondary"}>
                    {o.available ? t("runtimeReady") : t("runtimeUnavailable")}
                  </Badge>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
