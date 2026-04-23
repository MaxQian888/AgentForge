jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const messages: Record<string, string> = {
      noReviewsYet: "No reviews yet.",
      layerReview: "Layer {layer} Review",
      noSummary: "No summary available.",
      recommendationApprove: "Approve",
      recommendationRequestChanges: "Request Changes",
      recommendationReject: "Reject",
      rejectReview: "Reject",
      blockReview: "Block",
      approveCommentLabel: "Comment (optional)",
      requestChangesCommentLabel: "Comment (optional)",
      rejectCommentLabel: "Reject reason (required)",
      blockCommentLabel: "Blocking reason (required)",
      approveCommentPlaceholder: "Optional approval comment...",
      requestChangesCommentPlaceholder: "Describe what needs to change...",
      rejectCommentPlaceholder: "Describe why this review is rejected...",
      blockCommentPlaceholder: "Describe why this review is blocked...",
      confirmApprove: "Confirm Approve",
      confirmRequestChanges: "Confirm Request Changes",
      confirmReject: "Confirm Reject",
      confirmBlock: "Confirm Block",
      rejectReasonRequired: "A reject reason is required.",
      blockReasonRequired: "A blocking reason is required.",
      cancelTrigger: "Cancel",
      selectReview: "Select review",
      statusPendingHuman: "Pending Human",
      statusCompleted: "Completed",
      statusPending: "Pending",
      statusInProgress: "In Progress",
      statusFailed: "Failed",
      riskHigh: "High",
      riskMedium: "Medium",
      riskLow: "Low",
      riskCritical: "Critical",
      unassigned: "Unassigned",
      noBranch: "No branch",
      timeMinutesAgo: "{count}m ago",
      timeHoursAgo: "{count}h ago",
      timeDaysAgo: "{count}d ago",
    };
    const template = messages[key] ?? key;
    return template.replace(/\{(\w+)\}/g, (_, token) => String(values?.[token] ?? ""));
  },
}));

const fetchTaskById = jest.fn().mockResolvedValue(null);
const taskStoreState = {
  tasks: [] as Array<{
    id: string;
    assigneeName: string | null;
    agentBranch: string;
  }>,
  fetchTaskById,
};

jest.mock("@/lib/stores/task-store", () => ({
  useTaskStore: (selector?: (state: typeof taskStoreState) => unknown) =>
    typeof selector === "function" ? selector(taskStoreState) : taskStoreState,
}));

import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { ReviewList } from "./review-list";
import type { ReviewDTO } from "@/lib/stores/review-store";

function makeReview(overrides: Partial<ReviewDTO> = {}): ReviewDTO {
  return {
    id: "review-1",
    taskId: "task-1",
    prUrl: "https://github.com/org/repo/pull/1",
    prNumber: 1,
    layer: 2,
    status: "pending_human",
    riskLevel: "high",
    findings: [],
    summary: "Critical checks completed.",
    recommendation: "request_changes",
    costUsd: 1.25,
    createdAt: "2026-03-25T08:00:00.000Z",
    updatedAt: "2026-03-25T08:30:00.000Z",
    ...overrides,
  };
}

describe("ReviewList", () => {
  beforeEach(() => {
    fetchTaskById.mockClear();
    taskStoreState.tasks = [];
  });

  it("shows an empty-state message when no reviews exist", () => {
    render(<ReviewList reviews={[]} onSelect={jest.fn()} />);

    expect(screen.getByText("No reviews yet.")).toBeInTheDocument();
  });

  it("groups reviews into status columns and shows per-column counts", () => {
    render(
      <ReviewList
        reviews={[
          makeReview({ id: "review-1", status: "pending_human" }),
          makeReview({ id: "review-2", status: "completed", recommendation: "approve" }),
          makeReview({ id: "review-3", status: "failed", recommendation: "reject" }),
        ]}
        onSelect={jest.fn()}
      />
    );

    expect(screen.getByTestId("review-column-pending_human")).toHaveTextContent(
      "Pending Human"
    );
    expect(screen.getByTestId("review-column-pending_human")).toHaveTextContent("1");
    expect(screen.getByTestId("review-column-completed")).toHaveTextContent("1");
    expect(screen.getByTestId("review-column-failed")).toHaveTextContent("1");
  });

  it("shows assignee, branch, and age metadata when the related task is available", () => {
    jest.useFakeTimers().setSystemTime(new Date("2026-03-27T08:00:00.000Z"));
    taskStoreState.tasks = [
      {
        id: "task-1",
        assigneeName: "Alice",
        agentBranch: "agent/review-1",
      },
    ];

    render(<ReviewList reviews={[makeReview()]} onSelect={jest.fn()} />);

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("agent/review-1")).toBeInTheDocument();
    expect(screen.getByText("2d ago")).toBeInTheDocument();

    jest.useRealTimers();
  });

  it("lets users select, approve, and request changes on a pending_human review without using prompt dialogs", async () => {
    const user = userEvent.setup();
    const onSelect = jest.fn();
    const onApprove = jest.fn();
    const onRequestChanges = jest.fn();
    const promptSpy = jest.spyOn(window, "prompt");

    render(
      <ReviewList
        reviews={[makeReview()]}
        onSelect={onSelect}
        onApprove={onApprove}
        onRequestChanges={onRequestChanges}
      />,
    );

    await user.click(screen.getByText("Layer 2 Review"));
    expect(onSelect).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Approve" }));
    await user.type(
      screen.getByPlaceholderText("Optional approval comment..."),
      "Ship after quick pass",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Approve" }));
    expect(onApprove).toHaveBeenCalledWith("review-1", "Ship after quick pass");

    await user.click(screen.getByRole("button", { name: "Request Changes" }));
    expect(promptSpy).not.toHaveBeenCalled();
    await user.type(
      screen.getByPlaceholderText("Describe what needs to change..."),
      "Needs fixes",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Request Changes" }));
    expect(onRequestChanges).toHaveBeenCalledWith("review-1", "Needs fixes");

    expect(onSelect).toHaveBeenCalledTimes(1);
    promptSpy.mockRestore();
  });

  it("renders selection checkboxes and invokes onToggleSelect without opening the card", async () => {
    const user = userEvent.setup();
    const onSelect = jest.fn();
    const onToggleSelect = jest.fn();

    render(
      <ReviewList
        reviews={[makeReview()]}
        onSelect={onSelect}
        onToggleSelect={onToggleSelect}
        selectedIds={new Set()}
      />,
    );

    const checkbox = screen.getByTestId("review-select-review-1");
    await user.click(checkbox);
    expect(onToggleSelect).toHaveBeenCalledWith("review-1");
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("exposes reject and block actions for pending_human reviews", async () => {
    const user = userEvent.setup();
    const onReject = jest.fn();
    const onBlock = jest.fn();

    render(
      <ReviewList
        reviews={[makeReview()]}
        onSelect={jest.fn()}
        onReject={onReject}
        onBlock={onBlock}
      />,
    );

    expect(screen.getByRole("button", { name: "Reject" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Block" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Block" }));
    await user.type(
      screen.getByPlaceholderText("Describe why this review is blocked..."),
      "Waiting on infra",
    );
    await user.click(screen.getByRole("button", { name: "Confirm Block" }));
    expect(onBlock).toHaveBeenCalledWith(
      "review-1",
      "Waiting on infra",
      "Waiting on infra",
    );
  });
});
