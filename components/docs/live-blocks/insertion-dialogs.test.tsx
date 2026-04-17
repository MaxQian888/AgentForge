import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Agent } from "@/lib/stores/agent-store";
import type { ReviewDTO } from "@/lib/stores/review-store";
import type { SavedView } from "@/lib/stores/saved-view-store";
import {
  AgentRunPickerDialog,
  CostSummaryFilterDialog,
  ReviewPickerDialog,
  TaskGroupFilterDialog,
} from "./insertion-dialogs";

// ---------------------------------------------------------------------------
// Store mocks
// ---------------------------------------------------------------------------

jest.mock("@/lib/stores/agent-store", () => {
  const actual = jest.requireActual("@/lib/stores/agent-store");
  const state = { agents: [], fetchAgents: jest.fn(async () => {}) };
  const useAgentStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { ...actual, useAgentStore };
});

jest.mock("@/lib/stores/review-store", () => {
  const actual = jest.requireActual("@/lib/stores/review-store");
  const state = { allReviews: [], fetchAllReviews: jest.fn(async () => {}) };
  const useReviewStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { ...actual, useReviewStore };
});

jest.mock("@/lib/stores/member-store", () => {
  const state = { membersByProject: {}, fetchMembers: jest.fn(async () => {}) };
  const useMemberStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { useMemberStore };
});

jest.mock("@/lib/stores/saved-view-store", () => {
  const actual = jest.requireActual("@/lib/stores/saved-view-store");
  const state = { viewsByProject: {}, fetchViews: jest.fn(async () => {}) };
  const useSavedViewStore = (
    selector: (s: typeof state) => unknown = (s) => s,
  ) => selector(state);
  return { ...actual, useSavedViewStore };
});

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

function makeAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: "run-abcdef1234567890",
    taskId: "task-1",
    taskTitle: "Implement parser",
    memberId: "member-1",
    roleId: "role-1",
    roleName: "Agent",
    status: "running",
    runtime: "claude_code",
    provider: "anthropic",
    model: "sonnet",
    turns: 3,
    cost: 1.23,
    budget: 5,
    worktreePath: "",
    branchName: "",
    sessionId: "",
    lastActivity: "2026-04-17T00:00:00Z",
    startedAt: "2026-04-17T00:00:00Z",
    createdAt: "2026-04-17T00:00:00Z",
    canResume: false,
    memoryStatus: "none",
    ...overrides,
  };
}

function makeReview(overrides: Partial<ReviewDTO> = {}): ReviewDTO {
  return {
    id: "review-001122334455",
    taskId: "task-1",
    prUrl: "https://example.com/pr/1",
    prNumber: 1,
    layer: 1,
    status: "pending",
    riskLevel: "low",
    findings: [],
    summary: "Looks good",
    recommendation: "approve",
    costUsd: 0,
    createdAt: "2026-04-17T00:00:00Z",
    updatedAt: "2026-04-17T00:00:00Z",
    ...overrides,
  };
}

function makeSavedView(overrides: Partial<SavedView> = {}): SavedView {
  return {
    id: "view-1",
    projectId: "project-1",
    name: "My View",
    isDefault: false,
    sharedWith: [],
    config: {},
    createdAt: "2026-04-17T00:00:00Z",
    updatedAt: "2026-04-17T00:00:00Z",
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// AgentRunPickerDialog
// ---------------------------------------------------------------------------

describe("AgentRunPickerDialog", () => {
  it("lists runs, filters by search, and emits the right spec on confirm", async () => {
    const user = userEvent.setup();
    const onInsert = jest.fn();
    const onOpenChange = jest.fn();
    const agents = [
      makeAgent({ id: "run-alpha-0000000000", taskTitle: "Parse AST" }),
      makeAgent({ id: "run-beta-00000000000", taskTitle: "Refactor router" }),
    ];

    render(
      <AgentRunPickerDialog
        open
        onOpenChange={onOpenChange}
        projectId="project-1"
        onInsert={onInsert}
        agentsOverride={agents}
      />,
    );

    // Search box is present
    const search = screen.getByLabelText("Search agent runs");
    expect(search).toBeInTheDocument();

    // Both runs visible initially
    const list = screen.getByRole("listbox", { name: "Agent runs" });
    expect(within(list).getAllByRole("option")).toHaveLength(2);

    // Filter down
    await user.type(search, "router");
    const filteredOptions = within(list).getAllByRole("option");
    expect(filteredOptions).toHaveLength(1);
    expect(filteredOptions[0]).toHaveTextContent(/Refactor router/i);

    // Confirm is disabled until a run is selected
    const insertButton = screen.getByRole("button", { name: "Insert" });
    expect(insertButton).toBeDisabled();

    await user.click(filteredOptions[0]);
    expect(insertButton).toBeEnabled();

    await user.click(insertButton);
    expect(onInsert).toHaveBeenCalledTimes(1);
    expect(onInsert).toHaveBeenCalledWith({
      live_kind: "agent_run",
      target_ref: { kind: "agent_run", id: "run-beta-00000000000" },
      view_opts: { show_log_lines: 10, show_steps: true },
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows empty state when no runs match", () => {
    render(
      <AgentRunPickerDialog
        open
        onOpenChange={jest.fn()}
        onInsert={jest.fn()}
        agentsOverride={[]}
      />,
    );
    expect(screen.getByText(/No agent runs match/i)).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// CostSummaryFilterDialog
// ---------------------------------------------------------------------------

describe("CostSummaryFilterDialog", () => {
  it("disables confirm until both dates are filled", async () => {
    const onInsert = jest.fn();

    render(
      <CostSummaryFilterDialog
        open
        onOpenChange={jest.fn()}
        projectId="project-1"
        onInsert={onInsert}
      />,
    );

    const insertButton = screen.getByRole("button", { name: "Insert" });
    expect(insertButton).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Start date"), {
      target: { value: "2026-04-01" },
    });
    expect(insertButton).toBeDisabled();

    fireEvent.change(screen.getByLabelText("End date"), {
      target: { value: "2026-04-15" },
    });
    expect(insertButton).toBeEnabled();
  });

  it("emits the correct cost_summary spec on confirm", async () => {
    const user = userEvent.setup();
    const onInsert = jest.fn();
    const onOpenChange = jest.fn();

    render(
      <CostSummaryFilterDialog
        open
        onOpenChange={onOpenChange}
        projectId="project-1"
        onInsert={onInsert}
      />,
    );

    fireEvent.change(screen.getByLabelText("Start date"), {
      target: { value: "2026-04-01" },
    });
    fireEvent.change(screen.getByLabelText("End date"), {
      target: { value: "2026-04-15" },
    });
    await user.selectOptions(screen.getByLabelText("Runtime"), "codex");
    await user.selectOptions(screen.getByLabelText("Provider"), "openai");
    await user.selectOptions(screen.getByLabelText("Group by"), "runtime");

    await user.click(screen.getByRole("button", { name: "Insert" }));

    expect(onInsert).toHaveBeenCalledWith({
      live_kind: "cost_summary",
      target_ref: {
        kind: "cost_summary",
        filter: {
          range_start: "2026-04-01",
          range_end: "2026-04-15",
          runtime: "codex",
          provider: "openai",
        },
      },
      view_opts: { top_n: 5, group_by: "runtime" },
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

// ---------------------------------------------------------------------------
// ReviewPickerDialog
// ---------------------------------------------------------------------------

describe("ReviewPickerDialog", () => {
  it("builds a review spec on confirm", async () => {
    const user = userEvent.setup();
    const onInsert = jest.fn();

    render(
      <ReviewPickerDialog
        open
        onOpenChange={jest.fn()}
        projectId="project-1"
        onInsert={onInsert}
        reviewsOverride={[makeReview({ id: "review-target-0001" })]}
      />,
    );

    const option = screen.getByRole("option");
    await user.click(option);
    await user.click(screen.getByRole("button", { name: "Insert" }));

    expect(onInsert).toHaveBeenCalledWith({
      live_kind: "review",
      target_ref: { kind: "review", id: "review-target-0001" },
      view_opts: { show_findings_preview: true },
    });
  });
});

// ---------------------------------------------------------------------------
// TaskGroupFilterDialog
// ---------------------------------------------------------------------------

describe("TaskGroupFilterDialog", () => {
  it("builds a saved-view spec when in saved view mode", async () => {
    const user = userEvent.setup();
    const onInsert = jest.fn();
    const view = makeSavedView({ id: "view-123", name: "In Progress" });

    render(
      <TaskGroupFilterDialog
        open
        onOpenChange={jest.fn()}
        projectId="project-1"
        onInsert={onInsert}
        savedViewsOverride={[view]}
      />,
    );

    await user.selectOptions(screen.getByLabelText("Saved view"), "view-123");
    await user.click(screen.getByRole("button", { name: "Insert" }));

    expect(onInsert).toHaveBeenCalledWith({
      live_kind: "task_group",
      target_ref: {
        kind: "task_group",
        filter: { saved_view_id: "view-123" },
      },
      view_opts: { page_size: 50, sort: "updated_at_desc" },
    });
  });

  it("builds an inline-filter spec when switching to inline mode", async () => {
    const user = userEvent.setup();
    const onInsert = jest.fn();

    render(
      <TaskGroupFilterDialog
        open
        onOpenChange={jest.fn()}
        projectId="project-1"
        onInsert={onInsert}
        savedViewsOverride={[]}
      />,
    );

    await user.click(screen.getByRole("tab", { name: "Inline filter" }));
    await user.type(screen.getByLabelText("Status"), "in_progress");
    await user.type(screen.getByLabelText("Assignee"), "member-42");
    await user.click(screen.getByRole("button", { name: "Insert" }));

    expect(onInsert).toHaveBeenCalledWith({
      live_kind: "task_group",
      target_ref: {
        kind: "task_group",
        filter: { status: "in_progress", assignee: "member-42" },
      },
      view_opts: { page_size: 50, sort: "updated_at_desc" },
    });
  });

  it("disables confirm in saved-view mode when no view is chosen", () => {
    render(
      <TaskGroupFilterDialog
        open
        onOpenChange={jest.fn()}
        projectId="project-1"
        onInsert={jest.fn()}
        savedViewsOverride={[makeSavedView()]}
      />,
    );
    expect(screen.getByRole("button", { name: "Insert" })).toBeDisabled();
  });
});
