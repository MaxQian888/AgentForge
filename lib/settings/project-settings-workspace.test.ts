import type {
  CodingAgentCatalog,
  Project,
} from "@/lib/stores/project-store";
import {
  areSettingsDraftsEqual,
  createSettingsWorkspaceDraft,
  DEFAULT_BUDGET_GOVERNANCE,
  DEFAULT_REVIEW_POLICY,
  DEFAULT_WEBHOOK,
  getMinRiskLevelForBlockValue,
  getPrimaryReviewLayerLabel,
  getSettingsFallbackState,
  validateSettingsWorkspaceDraft,
} from "./project-settings-workspace";

const codingAgentCatalog: CodingAgentCatalog = {
  defaultRuntime: "codex",
  defaultSelection: {
    runtime: "codex",
    provider: "openai",
    model: "gpt-5-codex",
  },
  runtimes: [
    {
      runtime: "codex",
      label: "Codex",
      defaultProvider: "openai",
      compatibleProviders: ["openai", "codex"],
      defaultModel: "gpt-5-codex",
      modelOptions: ["gpt-5-codex", "o3"],
      available: true,
      diagnostics: [],
      supportedFeatures: ["reasoning", "fork"],
    },
    {
      runtime: "claude_code",
      label: "Claude Code",
      defaultProvider: "anthropic",
      compatibleProviders: ["anthropic"],
      defaultModel: "claude-sonnet-4-5",
      modelOptions: ["claude-sonnet-4-5", "claude-opus-4-1"],
      available: true,
      diagnostics: [],
      supportedFeatures: ["structured_output", "interrupt"],
    },
  ],
};

const baseProject: Project = {
  id: "project-1",
  name: "AgentForge",
  description: "Main delivery stream",
  status: "active",
  taskCount: 0,
  agentCount: 0,
  createdAt: "2026-03-27T10:00:00.000Z",
  repoUrl: "https://github.com/acme/agentforge",
  defaultBranch: "main",
  settings: {
    codingAgent: {
      runtime: "",
      provider: "",
      model: "",
    },
  },
  codingAgentCatalog,
};

describe("project-settings-workspace", () => {
  it("creates a draft using catalog defaults and fallback settings", () => {
    const draft = createSettingsWorkspaceDraft(baseProject);

    expect(draft).toEqual({
      name: "AgentForge",
      description: "Main delivery stream",
      repoUrl: "https://github.com/acme/agentforge",
      defaultBranch: "main",
      settings: {
        codingAgent: codingAgentCatalog.defaultSelection,
        budgetGovernance: DEFAULT_BUDGET_GOVERNANCE,
        reviewPolicy: DEFAULT_REVIEW_POLICY,
        webhook: DEFAULT_WEBHOOK,
      },
    });
  });

  it("falls back to the catalog default selection when stored runtime is unavailable", () => {
    const project: Project = {
      ...baseProject,
      settings: {
        ...baseProject.settings,
        codingAgent: {
          runtime: "iflow",
          provider: "iflow",
          model: "Qwen3-Coder",
        },
      },
      codingAgentCatalog: {
        defaultRuntime: "codex",
        defaultSelection: {
          runtime: "codex",
          provider: "openai",
          model: "gpt-5-codex",
        },
        runtimes: [
          ...codingAgentCatalog.runtimes,
          {
            runtime: "iflow",
            label: "iFlow CLI",
            defaultProvider: "iflow",
            compatibleProviders: ["iflow"],
            defaultModel: "Qwen3-Coder",
            modelOptions: ["Qwen3-Coder"],
            available: false,
            diagnostics: [
              {
                code: "runtime_sunset",
                message: "iFlow sunset",
                blocking: true,
              },
            ],
            supportedFeatures: ["progress"],
          },
        ],
      },
    };

    const draft = createSettingsWorkspaceDraft(project);

    expect(draft.settings.codingAgent).toEqual({
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
    });
  });

  it("tracks whether settings sections are falling back to defaults", () => {
    expect(getSettingsFallbackState(baseProject)).toEqual({
      budgetGovernance: true,
      reviewPolicy: true,
      webhook: true,
    });
  });

  it("compares drafts by value equality", () => {
    const draft = createSettingsWorkspaceDraft(baseProject);
    const sameDraft = createSettingsWorkspaceDraft(baseProject);
    const changedDraft = {
      ...sameDraft,
      defaultBranch: "develop",
    };

    expect(areSettingsDraftsEqual(draft, sameDraft)).toBe(true);
    expect(areSettingsDraftsEqual(draft, changedDraft)).toBe(false);
  });

  it("validates required fields, budget limits, alert thresholds, runtime, and webhook requirements", () => {
    const draft = createSettingsWorkspaceDraft(baseProject);
    const errors = validateSettingsWorkspaceDraft(
      {
        ...draft,
        name: "   ",
        defaultBranch: "",
        settings: {
          ...draft.settings,
          codingAgent: {
            ...draft.settings.codingAgent,
            runtime: "",
          },
          budgetGovernance: {
            ...draft.settings.budgetGovernance,
            maxTaskBudgetUsd: -1,
            maxDailySpendUsd: -2,
            alertThresholdPercent: 120,
          },
          webhook: {
            ...draft.settings.webhook,
            active: true,
            url: " ",
            events: [],
          },
        },
      },
      codingAgentCatalog,
    );

    expect(errors).toEqual(
      expect.objectContaining({
        name: "Project name is required.",
        defaultBranch: "Default branch is required.",
        maxTaskBudgetUsd: "Task budget cannot be negative.",
        maxDailySpendUsd: "Daily spend limit cannot be negative.",
        alertThresholdPercent: "Alert threshold must be between 0 and 100.",
        runtime: "Select a coding-agent runtime.",
        webhookUrl: "Webhook URL is required when webhook delivery is active.",
        webhookEvents:
          "Select at least one webhook event before enabling delivery.",
      }),
    );
  });

  it("validates provider compatibility for the selected runtime", () => {
    const draft = createSettingsWorkspaceDraft(baseProject);

    expect(
      validateSettingsWorkspaceDraft(
        {
          ...draft,
          settings: {
            ...draft.settings,
            codingAgent: {
              runtime: "codex",
              provider: "anthropic",
              model: "gpt-5-codex",
            },
          },
        },
        codingAgentCatalog,
      ),
    ).toEqual(
      expect.objectContaining({
        provider: "Selected provider is not supported by the current runtime.",
      }),
    );
  });

  it("rejects unavailable runtime selections", () => {
    const draft = createSettingsWorkspaceDraft(baseProject);

    expect(
      validateSettingsWorkspaceDraft(
        {
          ...draft,
          settings: {
            ...draft.settings,
            codingAgent: {
              runtime: "iflow",
              provider: "iflow",
              model: "Qwen3-Coder",
            },
          },
        },
        {
          ...codingAgentCatalog,
          runtimes: [
            ...codingAgentCatalog.runtimes,
            {
              runtime: "iflow",
              label: "iFlow CLI",
              defaultProvider: "iflow",
              compatibleProviders: ["iflow"],
              defaultModel: "Qwen3-Coder",
              modelOptions: ["Qwen3-Coder"],
              available: false,
              diagnostics: [
                {
                  code: "runtime_sunset",
                  message: "iFlow sunset",
                  blocking: true,
                },
              ],
              supportedFeatures: ["progress"],
            },
          ],
        },
      ),
    ).toEqual(
      expect.objectContaining({
        runtime: "Selected runtime is currently unavailable.",
      }),
    );
  });

  it("exposes review helper labels with sane fallbacks", () => {
    expect(getPrimaryReviewLayerLabel(["layer-2", "layer-3"])).toBe("layer-2");
    expect(getPrimaryReviewLayerLabel([])).toBe("none");
    expect(getMinRiskLevelForBlockValue("high")).toBe("high");
    expect(getMinRiskLevelForBlockValue("")).toBe("none");
  });
});
