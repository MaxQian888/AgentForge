export interface AgentProfileDraft {
  roleId: string;
  runtime: string;
  provider: string;
  model: string;
  maxBudgetUsd: string;
  notes: string;
}

export interface AgentProfile {
  roleId: string;
  runtime: string;
  provider: string;
  model: string;
  maxBudgetUsd: number | null;
  notes: string;
}

export interface AgentProfileReadiness {
  state: "ready" | "incomplete";
  label: string;
  missing: string[];
}

const EMPTY_DRAFT: AgentProfileDraft = {
  roleId: "",
  runtime: "",
  provider: "",
  model: "",
  maxBudgetUsd: "",
  notes: "",
};

function parseRawAgentConfig(raw?: string | null): Record<string, unknown> {
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? (parsed as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

export function buildAgentProfileDraft(raw?: string | null): AgentProfileDraft {
  const parsed = parseRawAgentConfig(raw);

  return {
    roleId: typeof parsed.roleId === "string" ? parsed.roleId : "",
    runtime: typeof parsed.runtime === "string" ? parsed.runtime : "",
    provider: typeof parsed.provider === "string" ? parsed.provider : "",
    model: typeof parsed.model === "string" ? parsed.model : "",
    maxBudgetUsd:
      typeof parsed.maxBudgetUsd === "number"
        ? String(parsed.maxBudgetUsd)
        : typeof parsed.maxBudgetUsd === "string"
          ? parsed.maxBudgetUsd
          : "",
    notes: typeof parsed.notes === "string" ? parsed.notes : "",
  };
}

export function parseAgentProfile(raw?: string | null): AgentProfile {
  const draft = buildAgentProfileDraft(raw);
  return {
    roleId: draft.roleId,
    runtime: draft.runtime,
    provider: draft.provider,
    model: draft.model,
    maxBudgetUsd: draft.maxBudgetUsd ? Number(draft.maxBudgetUsd) : null,
    notes: draft.notes,
  };
}

export function serializeAgentProfileDraft(
  draft?: Partial<AgentProfileDraft> | null,
): string {
  const normalized = { ...EMPTY_DRAFT, ...(draft ?? {}) };
  const payload: Record<string, unknown> = {};

  if (normalized.roleId.trim()) payload.roleId = normalized.roleId.trim();
  if (normalized.runtime.trim()) payload.runtime = normalized.runtime.trim();
  if (normalized.provider.trim()) payload.provider = normalized.provider.trim();
  if (normalized.model.trim()) payload.model = normalized.model.trim();
  if (normalized.maxBudgetUsd.trim()) {
    const parsedBudget = Number(normalized.maxBudgetUsd);
    if (!Number.isNaN(parsedBudget)) {
      payload.maxBudgetUsd = parsedBudget;
    }
  }
  if (normalized.notes.trim()) payload.notes = normalized.notes.trim();

  return JSON.stringify(payload);
}

export function getAgentProfileReadiness(
  draft?: Partial<AgentProfileDraft | AgentProfile> | null,
): AgentProfileReadiness {
  const normalized = { ...EMPTY_DRAFT, ...(draft ?? {}) };
  const missing: string[] = [];
  const missingSetupFields: string[] = [];

  if (!normalized.roleId.trim()) {
    missing.push("roleId");
  }

  if (!normalized.runtime.trim()) {
    missing.push("runtime");
    missingSetupFields.push("runtime");
  }
  if (!normalized.provider.trim()) {
    missing.push("provider");
    missingSetupFields.push("provider");
  }
  if (!normalized.model.trim()) {
    missing.push("model");
    missingSetupFields.push("model");
  }

  if (missingSetupFields.length > 0) {
    return {
      state: "incomplete",
      label: "Setup Required",
      missing,
    };
  }

  if (missing.length > 0) {
    return {
      state: "incomplete",
      label: "Needs role binding",
      missing,
    };
  }

  return {
    state: "ready",
    label: "Ready",
    missing: [],
  };
}

export function buildAgentProfileSummary(
  draft?: Partial<AgentProfileDraft | AgentProfile> | null,
): string[] {
  const normalized = { ...EMPTY_DRAFT, ...(draft ?? {}) };
  const rawBudget = normalized.maxBudgetUsd;
  const budgetValue =
    typeof rawBudget === "number"
      ? rawBudget
      : typeof rawBudget === "string" && rawBudget.trim()
        ? Number(rawBudget)
        : null;
  return [
    normalized.runtime.trim(),
    normalized.provider.trim(),
    normalized.model.trim(),
    budgetValue != null && !Number.isNaN(budgetValue)
      ? `$${budgetValue.toFixed(2)} budget`
      : "",
  ].filter(Boolean);
}
