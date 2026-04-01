"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ApiError } from "@/lib/api-client";
import { FieldDefinitionEditor } from "@/components/fields/field-definition-editor";
import { FormBuilder } from "@/components/forms/form-builder";
import { RuleEditor } from "@/components/automations/rule-editor";
import { RuleList } from "@/components/automations/rule-list";
import { AutomationLogViewer } from "@/components/automations/automation-log-viewer";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { useDashboardStore } from "@/lib/stores/dashboard-store";
import { useProjectStore, type Project, type ProjectSettings, type ProjectUpdateInput } from "@/lib/stores/project-store";
import { useLocaleStore, SUPPORTED_LOCALES, type Locale } from "@/lib/stores/locale-store";
import {
  DEFAULT_WEBHOOK,
  areSettingsDraftsEqual,
  createSettingsWorkspaceDraft,
  getPrimaryReviewLayerLabel,
  getMinRiskLevelForBlockValue,
  getSettingsFallbackState,
  validateSettingsWorkspaceDraft,
  type SettingsValidationErrors,
  type SettingsWorkspaceDraft,
} from "@/lib/settings/project-settings-workspace";
import { useBreadcrumbs } from "@/hooks/use-breadcrumbs";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { ErrorBanner } from "@/components/shared/error-banner";
import { FolderOpen } from "lucide-react";

type UpdateProject = (id: string, data: ProjectUpdateInput) => Promise<Project | undefined>;
type SaveState = "idle" | "saving" | "saved" | "error";

function FieldError({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <p className="text-sm text-destructive" role="alert">
      {message}
    </p>
  );
}

function toProjectUpdateInput(draft: SettingsWorkspaceDraft): ProjectUpdateInput {
  return {
    name: draft.name.trim(),
    description: draft.description,
    repoUrl: draft.repoUrl,
    defaultBranch: draft.defaultBranch.trim(),
    settings: {
      codingAgent: draft.settings.codingAgent,
      budgetGovernance: draft.settings.budgetGovernance,
      reviewPolicy: {
        ...draft.settings.reviewPolicy,
        requiredLayers: [...draft.settings.reviewPolicy.requiredLayers],
      },
      webhook: draft.settings.webhook,
    } satisfies ProjectSettings,
  };
}

function extractServerError(error: unknown) {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string; errors?: Record<string, unknown> } | null;
    const rawErrors = body?.errors ?? {};
    const fieldErrors: SettingsValidationErrors = {};
    const mapping: Record<string, keyof SettingsValidationErrors> = {
      name: "name",
      defaultBranch: "defaultBranch",
      maxTaskBudgetUsd: "maxTaskBudgetUsd",
      maxDailySpendUsd: "maxDailySpendUsd",
      alertThresholdPercent: "alertThresholdPercent",
      runtime: "runtime",
      provider: "provider",
      webhookUrl: "webhookUrl",
      webhookEvents: "webhookEvents",
    };
    Object.entries(mapping).forEach(([rawKey, fieldKey]) => {
      const nextValue = rawErrors[rawKey];
      if (typeof nextValue === "string") fieldErrors[fieldKey] = nextValue;
    });
    return { fieldErrors, message: body?.message ?? error.message };
  }
  if (error instanceof Error) return { fieldErrors: {}, message: error.message };
  return { fieldErrors: {}, message: "Failed to save settings." };
}

function preserveRedactedWebhookSecret(project: Project, submittedSecret: string): Project {
  if (!submittedSecret) {
    return project;
  }

  if (project.settings.webhook?.secret) {
    return project;
  }

  return {
    ...project,
    settings: {
      ...project.settings,
      webhook: {
        ...DEFAULT_WEBHOOK,
        ...project.settings.webhook,
        secret: submittedSecret,
      },
    },
  };
}

function SettingsContent({ project, updateProject }: { project: Project; updateProject: UpdateProject }) {
  const t = useTranslations("settings");
  const [persistedSnapshot, setPersistedSnapshot] = useState(() => createSettingsWorkspaceDraft(project));
  const [draft, setDraft] = useState(() => createSettingsWorkspaceDraft(project));
  const [fallbackState, setFallbackState] = useState(() => getSettingsFallbackState(project));
  const [validationErrors, setValidationErrors] = useState<SettingsValidationErrors>({});
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const savedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (savedTimeoutRef.current) clearTimeout(savedTimeoutRef.current);
    };
  }, []);

  const runtimeOptions = project.codingAgentCatalog?.runtimes ?? [];
  const selectedRuntime =
    runtimeOptions.find((option) => option.runtime === draft.settings.codingAgent.runtime) ??
    runtimeOptions[0];
  const compatibleProviders = selectedRuntime?.compatibleProviders ?? [];
  const modelOptions =
    selectedRuntime?.modelOptions && selectedRuntime.modelOptions.length > 0
      ? selectedRuntime.modelOptions
      : selectedRuntime?.defaultModel
        ? [selectedRuntime.defaultModel]
        : [];
  const selectedDiagnostics = selectedRuntime?.diagnostics ?? [];
  const dirty = useMemo(() => !areSettingsDraftsEqual(draft, persistedSnapshot), [draft, persistedSnapshot]);
  const hasFallbackDefaults =
    fallbackState.budgetGovernance || fallbackState.reviewPolicy || fallbackState.webhook;

  const webhookSummary = useMemo(() => {
    if (!draft.settings.webhook.active) return t("webhookInactiveSummary");
    if (!draft.settings.webhook.url.trim()) return t("webhookMissingUrlSummary");
    if (draft.settings.webhook.events.length === 0) return t("webhookMissingEventsSummary");
    return t("webhookReadySummary");
  }, [draft.settings.webhook.active, draft.settings.webhook.events.length, draft.settings.webhook.url, t]);

  const clearValidationError = (field: keyof SettingsValidationErrors) => {
    setValidationErrors((current) => (current[field] ? { ...current, [field]: undefined } : current));
  };

  const patchDraft = (updater: (current: SettingsWorkspaceDraft) => SettingsWorkspaceDraft) => {
    setDraft((current) => updater(current));
    setSaveError(null);
    if (saveState === "error") setSaveState("idle");
  };

  const handleRuntimeChange = (nextRuntime: string) => {
    patchDraft((current) => {
      const nextOption = runtimeOptions.find((option) => option.runtime === nextRuntime);
      if (!nextOption) return current;
      return {
        ...current,
        settings: {
          ...current.settings,
          codingAgent: {
            runtime: nextRuntime,
            provider: nextOption.defaultProvider,
            model:
              nextOption.modelOptions?.[0] ?? nextOption.defaultModel,
          },
        },
      };
    });
    clearValidationError("runtime");
    clearValidationError("provider");
  };

  const handleDiscard = () => {
    setDraft(persistedSnapshot);
    setValidationErrors({});
    setSaveError(null);
    setSaveState("idle");
  };

  const handleSave = async () => {
    const nextValidationErrors = validateSettingsWorkspaceDraft(draft, project.codingAgentCatalog);
    if (Object.values(nextValidationErrors).some(Boolean)) {
      setValidationErrors(nextValidationErrors);
      setSaveState("error");
      setSaveError(t("validationSummary"));
      return;
    }

    setValidationErrors({});
    setSaveError(null);
    setSaveState("saving");
    try {
      const input = toProjectUpdateInput(draft);
      const updatedProjectResponse =
        (await updateProject(project.id, input)) ??
        ({ ...project, ...input, settings: input.settings ?? project.settings } as Project);
      const updatedProject = preserveRedactedWebhookSecret(
        updatedProjectResponse,
        draft.settings.webhook.secret
      );
      const nextPersistedSnapshot = createSettingsWorkspaceDraft(updatedProject);
      setPersistedSnapshot(nextPersistedSnapshot);
      setDraft(nextPersistedSnapshot);
      setFallbackState(getSettingsFallbackState(updatedProject));
      setSaveState("saved");
      if (savedTimeoutRef.current) clearTimeout(savedTimeoutRef.current);
      savedTimeoutRef.current = setTimeout(() => {
        setSaveState("idle");
        savedTimeoutRef.current = null;
      }, 2000);
    } catch (error) {
      const serverError = extractServerError(error);
      setValidationErrors((current) => ({ ...current, ...serverError.fieldErrors }));
      setSaveError(serverError.message);
      setSaveState("error");
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t("title")}
        actions={
          <div className="flex flex-wrap items-center gap-2 text-sm">
            {dirty ? <Badge variant="secondary">{t("unsavedChanges")}</Badge> : <Badge variant="outline">{t("allChangesSaved")}</Badge>}
            {saveState === "saved" && <span className="text-emerald-600 dark:text-emerald-400">{t("settingsSaved")}</span>}
          </div>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle>{t("general")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-project-name">{t("projectName")}</Label>
            <Input
              id="settings-project-name"
              value={draft.name}
              aria-invalid={Boolean(validationErrors.name)}
              onChange={(event) => {
                patchDraft((current) => ({ ...current, name: event.target.value }));
                clearValidationError("name");
              }}
            />
            <FieldError message={validationErrors.name} />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-description">{t("description")}</Label>
            <Input
              id="settings-description"
              value={draft.description}
              onChange={(event) => patchDraft((current) => ({ ...current, description: event.target.value }))}
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("repository")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-repo-url">{t("repoUrl")}</Label>
            <Input
              id="settings-repo-url"
              value={draft.repoUrl}
              placeholder={t("repoUrlPlaceholder")}
              onChange={(event) => patchDraft((current) => ({ ...current, repoUrl: event.target.value }))}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="settings-default-branch">{t("defaultBranch")}</Label>
            <Input
              id="settings-default-branch"
              value={draft.defaultBranch}
              aria-invalid={Boolean(validationErrors.defaultBranch)}
              onChange={(event) => {
                patchDraft((current) => ({ ...current, defaultBranch: event.target.value }));
                clearValidationError("defaultBranch");
              }}
            />
            <FieldError message={validationErrors.defaultBranch} />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("codingAgentDefaults")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="flex flex-col gap-2">
              <Label>{t("runtime")}</Label>
              <Select value={draft.settings.codingAgent.runtime} onValueChange={handleRuntimeChange}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {runtimeOptions.map((option) => (
                    <SelectItem key={option.runtime} value={option.runtime}>
                      {option.label}
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
                onValueChange={(value) => {
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      codingAgent: { ...current.settings.codingAgent, provider: value },
                    },
                  }));
                  clearValidationError("provider");
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {compatibleProviders.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldError message={validationErrors.provider} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("model")}</Label>
              <Select
                value={draft.settings.codingAgent.model}
                onValueChange={(value) => {
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      codingAgent: { ...current.settings.codingAgent, model: value },
                    },
                  }));
                  clearValidationError("model");
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {modelOptions.map((option) => (
                    <SelectItem key={option} value={option}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldError message={validationErrors.model} />
            </div>
          </div>
          {selectedDiagnostics.length > 0 && (
            <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
              {selectedDiagnostics.map((diagnostic) => (
                <p key={`${diagnostic.code}-${diagnostic.message}`}>{diagnostic.message}</p>
              ))}
            </div>
          )}
          <div className="grid gap-3 md:grid-cols-2">
            {runtimeOptions.map((option) => (
              <div key={option.runtime} className="rounded-md border p-4 text-sm">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="font-medium">{option.label}</p>
                    <p className="text-muted-foreground">
                      {option.defaultProvider} / {option.defaultModel}
                    </p>
                    {(option.modelOptions?.length ?? 0) > 1 ? (
                      <p className="mt-1 text-xs text-muted-foreground">
                        {(option.modelOptions ?? []).join(", ")}
                      </p>
                    ) : null}
                  </div>
                  <Badge variant={option.available ? "default" : "secondary"}>
                    {option.available ? t("runtimeReady") : t("runtimeUnavailable")}
                  </Badge>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("budgetGovernance")}</CardTitle>
          <CardDescription>{t("budgetGovernanceDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-max-task-budget">{t("maxTaskBudget")}</Label>
              <Input
                id="settings-max-task-budget"
                type="number"
                min={0}
                step={0.01}
                aria-invalid={Boolean(validationErrors.maxTaskBudgetUsd)}
                value={draft.settings.budgetGovernance.maxTaskBudgetUsd}
                onChange={(event) => {
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      budgetGovernance: {
                        ...current.settings.budgetGovernance,
                        maxTaskBudgetUsd: Number(event.target.value),
                      },
                    },
                  }));
                  clearValidationError("maxTaskBudgetUsd");
                }}
              />
              <FieldError message={validationErrors.maxTaskBudgetUsd} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-max-daily-spend">{t("maxDailySpend")}</Label>
              <Input
                id="settings-max-daily-spend"
                type="number"
                min={0}
                step={0.01}
                aria-invalid={Boolean(validationErrors.maxDailySpendUsd)}
                value={draft.settings.budgetGovernance.maxDailySpendUsd}
                onChange={(event) => {
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      budgetGovernance: {
                        ...current.settings.budgetGovernance,
                        maxDailySpendUsd: Number(event.target.value),
                      },
                    },
                  }));
                  clearValidationError("maxDailySpendUsd");
                }}
              />
              <FieldError message={validationErrors.maxDailySpendUsd} />
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-alert-threshold">{t("alertThreshold")}</Label>
              <Input
                id="settings-alert-threshold"
                type="number"
                min={0}
                max={100}
                aria-invalid={Boolean(validationErrors.alertThresholdPercent)}
                value={draft.settings.budgetGovernance.alertThresholdPercent}
                onChange={(event) => {
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      budgetGovernance: {
                        ...current.settings.budgetGovernance,
                        alertThresholdPercent: Number(event.target.value),
                      },
                    },
                  }));
                  clearValidationError("alertThresholdPercent");
                }}
              />
              <FieldError message={validationErrors.alertThresholdPercent} />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("autoStopOnExceed")}</Label>
              <Select
                value={draft.settings.budgetGovernance.autoStopOnExceed ? "yes" : "no"}
                onValueChange={(value) =>
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      budgetGovernance: {
                        ...current.settings.budgetGovernance,
                        autoStopOnExceed: value === "yes",
                      },
                    },
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("enabled")}</SelectItem>
                  <SelectItem value="no">{t("disabled")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("reviewPolicy")}</CardTitle>
          <CardDescription>{t("reviewPolicyDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>{t("autoTriggerOnPR")}</Label>
              <Select
                value={draft.settings.reviewPolicy.autoTriggerOnPR ? "yes" : "no"}
                onValueChange={(value) =>
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      reviewPolicy: {
                        ...current.settings.reviewPolicy,
                        autoTriggerOnPR: value === "yes",
                      },
                    },
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("enabled")}</SelectItem>
                  <SelectItem value="no">{t("disabled")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("requiredReviewLayers")}</Label>
              <Select
                value={getPrimaryReviewLayerLabel(draft.settings.reviewPolicy.requiredLayers)}
                onValueChange={(value) =>
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      reviewPolicy: {
                        ...current.settings.reviewPolicy,
                        requiredLayers: value === "none" ? [] : [value],
                      },
                    },
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t("disabled")}</SelectItem>
                  <SelectItem value="layer1">{t("layerQuick")}</SelectItem>
                  <SelectItem value="layer2">{t("layerDeep")}</SelectItem>
                  <SelectItem value="layer3">{t("layerHuman")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>{t("minRiskLevelForBlock")}</Label>
              <Select
                value={getMinRiskLevelForBlockValue(
                  draft.settings.reviewPolicy.minRiskLevelForBlock
                )}
                onValueChange={(value) =>
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      reviewPolicy: {
                        ...current.settings.reviewPolicy,
                        minRiskLevelForBlock: value === "none" ? "" : value,
                      },
                    },
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t("disabled")}</SelectItem>
                  <SelectItem value="critical">{t("riskCritical")}</SelectItem>
                  <SelectItem value="high">{t("riskHigh")}</SelectItem>
                  <SelectItem value="medium">{t("riskMedium")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("requireManualApproval")}</Label>
              <Select
                value={draft.settings.reviewPolicy.requireManualApproval ? "yes" : "no"}
                onValueChange={(value) =>
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      reviewPolicy: {
                        ...current.settings.reviewPolicy,
                        requireManualApproval: value === "yes",
                      },
                    },
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("required")}</SelectItem>
                  <SelectItem value="no">{t("notRequired")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("webhookConfig")}</CardTitle>
          <CardDescription>{t("webhookConfigDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label htmlFor="settings-webhook-url">{t("webhookUrl")}</Label>
              <Input
                id="settings-webhook-url"
                value={draft.settings.webhook.url}
                aria-invalid={Boolean(validationErrors.webhookUrl)}
                placeholder={t("webhookUrlPlaceholder")}
                onChange={(event) => {
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      webhook: { ...current.settings.webhook, url: event.target.value },
                    },
                  }));
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
                onChange={(event) =>
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      webhook: { ...current.settings.webhook, secret: event.target.value },
                    },
                  }))
                }
              />
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>{t("webhookEvents")}</Label>
              <div className="flex flex-wrap gap-2">
                {["push", "pr_opened", "pr_merged", "review_completed"].map((event) => (
                  <Button
                    key={event}
                    type="button"
                    size="sm"
                    variant={draft.settings.webhook.events.includes(event) ? "default" : "outline"}
                    onClick={() => {
                      patchDraft((current) => ({
                        ...current,
                        settings: {
                          ...current.settings,
                          webhook: {
                            ...current.settings.webhook,
                            events: current.settings.webhook.events.includes(event)
                              ? current.settings.webhook.events.filter((item) => item !== event)
                              : [...current.settings.webhook.events, event],
                          },
                        },
                      }));
                      clearValidationError("webhookEvents");
                    }}
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
                onValueChange={(value) =>
                  patchDraft((current) => ({
                    ...current,
                    settings: {
                      ...current.settings,
                      webhook: { ...current.settings.webhook, active: value === "yes" },
                    },
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="yes">{t("active")}</SelectItem>
                  <SelectItem value="no">{t("inactive")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("operatorDiagnostics")}</CardTitle>
          <CardDescription>{t("operatorDiagnosticsDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {dirty && (
            <div className="rounded-md border border-primary/30 bg-primary/5 p-3 text-sm">
              {t("draftRuntimeChanged")}
            </div>
          )}
          {hasFallbackDefaults && (
            <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
              {t("fallbackGovernanceDefaults")}
            </div>
          )}
          <div className="grid gap-3 md:grid-cols-3">
            <div className="rounded-md border p-3 text-sm">
              <p className="font-medium">{t("runtimeSummaryTitle")}</p>
              <p className="mt-1 text-muted-foreground">
                {(selectedRuntime?.label ?? draft.settings.codingAgent.runtime) || t("noRuntimeInfo")}
              </p>
              <p className="mt-2">
                {selectedRuntime?.available ? t("runtimeReadySummary") : t("runtimeBlockedSummary")}
              </p>
            </div>
            <div className="rounded-md border p-3 text-sm">
              <p className="font-medium">{t("reviewSummaryTitle")}</p>
              <p className="mt-1 text-muted-foreground">
                {draft.settings.reviewPolicy.requireManualApproval
                  ? t("reviewManualApprovalEnabled")
                  : t("reviewManualApprovalDisabled")}
              </p>
              <p className="mt-2">
                {t("reviewRiskSummary", {
                  risk: draft.settings.reviewPolicy.minRiskLevelForBlock || t("disabled"),
                })}
              </p>
            </div>
            <div className="rounded-md border p-3 text-sm">
              <p className="font-medium">{t("webhookSummaryTitle")}</p>
              <p className="mt-1 text-muted-foreground">{webhookSummary}</p>
              <p className="mt-2">
                {t("webhookEventCountSummary", { count: draft.settings.webhook.events.length })}
              </p>
            </div>
          </div>
          <div className="grid gap-3 md:grid-cols-2">
            {runtimeOptions.map((option) => (
              <div key={option.runtime} className="flex items-center justify-between rounded-md border p-3">
                <span className="text-sm font-medium">{option.label}</span>
                <Badge variant={option.available ? "default" : "secondary"}>
                  {option.available ? t("runtimeReady") : t("runtimeUnavailable")}
                </Badge>
              </div>
            ))}
            {runtimeOptions.length === 0 && (
              <p className="text-sm text-muted-foreground">{t("noRuntimeInfo")}</p>
            )}
          </div>
          <div className="flex gap-3">
            <Button asChild size="sm" variant="outline">
              <Link href="/agents">{t("viewAgentPool")}</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link href="/reviews">{t("viewReviewBacklog")}</Link>
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("customFields")}</CardTitle>
          <CardDescription>{t("customFieldsDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <FieldDefinitionEditor projectId={project.id} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("forms")}</CardTitle>
          <CardDescription>{t("formsDesc")}</CardDescription>
        </CardHeader>
        <CardContent>
          <FormBuilder projectId={project.id} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("automations")}</CardTitle>
          <CardDescription>{t("automationsDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <RuleEditor projectId={project.id} />
          <RuleList projectId={project.id} />
          <AutomationLogViewer projectId={project.id} />
        </CardContent>
      </Card>

      <Separator />

      <div className="flex flex-col gap-3">
        {saveError && (
          <ErrorBanner message={saveError} />
        )}
        <div className="flex items-center gap-3">
          <Button type="button" disabled={saveState === "saving"} onClick={() => void handleSave()}>
            {saveState === "saving" ? t("savingSettings") : t("saveSettings")}
          </Button>
          {dirty && (
            <Button type="button" variant="outline" onClick={handleDiscard}>
              {t("discardChanges")}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

const LOCALE_LABELS: Record<Locale, string> = {
  en: "English",
  "zh-CN": "中文（简体）",
};

function AppearanceCard() {
  const t = useTranslations("settings");
  const locale = useLocaleStore((s) => s.locale);
  const setLocale = useLocaleStore((s) => s.setLocale);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("appearance")}</CardTitle>
        <CardDescription>{t("appearanceDesc")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-2">
          <Label>{t("themeMode")}</Label>
          <ThemeToggle />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="appearance-language">{t("language")}</Label>
          <Select value={locale} onValueChange={(v) => setLocale(v as Locale)}>
            <SelectTrigger className="w-full max-w-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SUPPORTED_LOCALES.map((loc) => (
                <SelectItem key={loc} value={loc}>
                  {LOCALE_LABELS[loc]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </CardContent>
    </Card>
  );
}

export default function SettingsPage() {
  useBreadcrumbs([{ label: "Configuration", href: "/" }, { label: "Settings" }]);
  const t = useTranslations("settings");
  const { selectedProjectId } = useDashboardStore();
  const { projects, fetchProjects, updateProject } = useProjectStore();
  const project = projects.find((item) => item.id === selectedProjectId);

  useEffect(() => {
    void fetchProjects();
  }, [fetchProjects]);

  return (
    <div className="mx-auto w-full max-w-4xl flex flex-col gap-6">
      <AppearanceCard />
      {!selectedProjectId && (
        <EmptyState
          icon={FolderOpen}
          title={t("titleNoProject")}
          description={t("selectProject")}
        />
      )}
      {selectedProjectId && !project && (
        <EmptyState
          icon={FolderOpen}
          title={t("titleNoProject")}
          description={t("loadingProject")}
        />
      )}
      {project && (
        <SettingsContent key={project.id} project={project} updateProject={updateProject} />
      )}
    </div>
  );
}
