import {
  buildAgentProfileDraft,
  buildAgentProfileSummary,
  getAgentProfileReadiness,
  serializeAgentProfileDraft,
} from "./agent-profile";

describe("agent profile helpers", () => {
  it("builds a typed draft from stored agent config", () => {
    const draft = buildAgentProfileDraft(
      JSON.stringify({
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: 5.5,
        notes: "focus on UI polish",
      }),
    );

    expect(draft).toEqual({
      roleId: "frontend-developer",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      maxBudgetUsd: "5.5",
      notes: "focus on UI polish",
    });
  });

  it("serializes structured drafts into normalized agent config", () => {
    expect(
      serializeAgentProfileDraft({
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: "7",
        notes: "ship with focused tests",
      }),
    ).toBe(
      JSON.stringify({
        roleId: "frontend-developer",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: 7,
        notes: "ship with focused tests",
      }),
    );
  });

  it("reports readiness and summary cues for incomplete agent profiles", () => {
    const readiness = getAgentProfileReadiness({
      roleId: "",
      runtime: "codex",
      provider: "openai",
      model: "gpt-5-codex",
      maxBudgetUsd: "",
      notes: "",
    });

    expect(readiness).toEqual({
      state: "incomplete",
      label: "Needs role binding",
      missing: ["roleId"],
    });

    expect(
      buildAgentProfileSummary({
        roleId: "",
        runtime: "codex",
        provider: "openai",
        model: "gpt-5-codex",
        maxBudgetUsd: "",
        notes: "",
      }),
    ).toEqual(["codex", "openai", "gpt-5-codex"]);
  });
});
