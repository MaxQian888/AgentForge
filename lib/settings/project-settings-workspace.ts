"use client";

import type {
  BudgetGovernance,
  CodingAgentCatalog,
  Project,
  ProjectSettings,
  ReviewPolicy,
  WebhookConfig,
} from "@/lib/stores/project-store";

export type SettingsWorkspaceDraft = {
  name: string;
  description: string;
  repoUrl: string;
  defaultBranch: string;
  settings: {
    codingAgent: ProjectSettings["codingAgent"];
    budgetGovernance: BudgetGovernance;
    reviewPolicy: ReviewPolicy;
    webhook: WebhookConfig;
  };
};

export type SettingsFallbackState = {
  budgetGovernance: boolean;
  reviewPolicy: boolean;
  webhook: boolean;
};

export type SettingsValidationErrors = Partial<
  Record<
    | "name"
    | "defaultBranch"
    | "maxTaskBudgetUsd"
    | "maxDailySpendUsd"
    | "alertThresholdPercent"
    | "runtime"
    | "provider"
    | "webhookUrl"
    | "webhookEvents",
    string
  >
>;

export const DEFAULT_BUDGET_GOVERNANCE: BudgetGovernance = {
  maxTaskBudgetUsd: 0,
  maxDailySpendUsd: 0,
  alertThresholdPercent: 80,
  autoStopOnExceed: false,
};

export const DEFAULT_REVIEW_POLICY: ReviewPolicy = {
  autoTriggerOnPR: false,
  requiredLayers: [],
  minRiskLevelForBlock: "",
  requireManualApproval: false,
  enabledPluginDimensions: [],
};

export const DEFAULT_WEBHOOK: WebhookConfig = {
  url: "",
  secret: "",
  events: [],
  active: false,
};

export function createSettingsWorkspaceDraft(project: Project): SettingsWorkspaceDraft {
  return {
    name: project.name,
    description: project.description ?? "",
    repoUrl: project.repoUrl ?? "",
    defaultBranch: project.defaultBranch ?? "main",
    settings: {
      codingAgent: {
        runtime:
          project.settings?.codingAgent.runtime ||
          project.codingAgentCatalog?.defaultSelection.runtime ||
          "",
        provider:
          project.settings?.codingAgent.provider ||
          project.codingAgentCatalog?.defaultSelection.provider ||
          "",
        model:
          project.settings?.codingAgent.model ||
          project.codingAgentCatalog?.defaultSelection.model ||
          "",
      },
      budgetGovernance: {
        ...DEFAULT_BUDGET_GOVERNANCE,
        ...project.settings?.budgetGovernance,
      },
      reviewPolicy: {
        ...DEFAULT_REVIEW_POLICY,
        ...project.settings?.reviewPolicy,
        requiredLayers: project.settings?.reviewPolicy?.requiredLayers ?? [],
        enabledPluginDimensions:
          project.settings?.reviewPolicy?.enabledPluginDimensions ?? [],
      },
      webhook: {
        ...DEFAULT_WEBHOOK,
        ...project.settings?.webhook,
        events: project.settings?.webhook?.events ?? [],
      },
    },
  };
}

export function getSettingsFallbackState(project: Project): SettingsFallbackState {
  return {
    budgetGovernance: project.settings?.budgetGovernance == null,
    reviewPolicy: project.settings?.reviewPolicy == null,
    webhook: project.settings?.webhook == null,
  };
}

export function areSettingsDraftsEqual(
  left: SettingsWorkspaceDraft,
  right: SettingsWorkspaceDraft
): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

export function validateSettingsWorkspaceDraft(
  draft: SettingsWorkspaceDraft,
  catalog?: CodingAgentCatalog
): SettingsValidationErrors {
  const errors: SettingsValidationErrors = {};

  if (!draft.name.trim()) {
    errors.name = "Project name is required.";
  }

  if (!draft.defaultBranch.trim()) {
    errors.defaultBranch = "Default branch is required.";
  }

  if (draft.settings.budgetGovernance.maxTaskBudgetUsd < 0) {
    errors.maxTaskBudgetUsd = "Task budget cannot be negative.";
  }

  if (draft.settings.budgetGovernance.maxDailySpendUsd < 0) {
    errors.maxDailySpendUsd = "Daily spend limit cannot be negative.";
  }

  if (
    draft.settings.budgetGovernance.alertThresholdPercent < 0 ||
    draft.settings.budgetGovernance.alertThresholdPercent > 100
  ) {
    errors.alertThresholdPercent = "Alert threshold must be between 0 and 100.";
  }

  const selectedRuntime = catalog?.runtimes.find(
    (option) => option.runtime === draft.settings.codingAgent.runtime
  );

  if (!draft.settings.codingAgent.runtime) {
    errors.runtime = "Select a coding-agent runtime.";
  }

  if (
    selectedRuntime &&
    !selectedRuntime.compatibleProviders.includes(draft.settings.codingAgent.provider)
  ) {
    errors.provider = "Selected provider is not supported by the current runtime.";
  }

  if (draft.settings.webhook.active && !draft.settings.webhook.url.trim()) {
    errors.webhookUrl = "Webhook URL is required when webhook delivery is active.";
  }

  if (
    draft.settings.webhook.active &&
    draft.settings.webhook.events.length === 0
  ) {
    errors.webhookEvents = "Select at least one webhook event before enabling delivery.";
  }

  return errors;
}

export function getPrimaryReviewLayerLabel(requiredLayers: string[]): string {
  return requiredLayers[0] ?? "none";
}

export function getMinRiskLevelForBlockValue(minRiskLevelForBlock: string): string {
  return minRiskLevelForBlock || "none";
}
