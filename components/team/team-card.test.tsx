jest.mock("next/link", () => ({
  __esModule: true,
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string, values?: Record<string, string | number>) => {
    if (namespace === "teams" && key === "card.untitled") {
      return "Untitled Team";
    }
    if (namespace === "teams" && key === "card.coders") {
      return `${values?.count ?? 0} coders`;
    }
    if (namespace === "teams" && key === "card.pipeline") {
      return "Pipeline";
    }
    return key;
  },
}));

import { render, screen } from "@testing-library/react";
import { TeamCard } from "./team-card";
import type { AgentTeam } from "@/lib/stores/team-store";

function makeTeam(overrides: Partial<AgentTeam> = {}): AgentTeam {
  return {
    id: "team-1",
    projectId: "project-1",
    taskId: "task-1",
    taskTitle: "Ship review workflow",
    name: "Review Squad",
    status: "reviewing",
    strategy: "planner-coder-reviewer",
    runtime: "codex",
    provider: "openai",
    model: "gpt-5.4",
    plannerRunId: "run-1",
    reviewerRunId: "run-2",
    coderRunIds: ["run-3", "run-4"],
    totalBudget: 10,
    totalSpent: 9,
    errorMessage: "",
    createdAt: "2026-03-25T07:30:00.000Z",
    updatedAt: "2026-03-25T08:30:00.000Z",
    ...overrides,
  };
}

describe("TeamCard", () => {
  it("renders the team pipeline summary and route link", () => {
    const { container } = render(<TeamCard team={makeTeam()} />);

    expect(
      screen.getByRole("link", { name: /Review Squad/i }),
    ).toHaveAttribute("href", "/teams/detail?id=team-1");
    expect(screen.getByText("reviewing")).toBeInTheDocument();
    expect(
      screen.getByText("Planner → Coder → Reviewer"),
    ).toBeInTheDocument();
    expect(screen.getByTitle("plan: completed")).toBeInTheDocument();
    expect(screen.getByTitle("execute: completed")).toBeInTheDocument();
    expect(screen.getByTitle("review: active")).toBeInTheDocument();
    expect(screen.getByText("$9.00 / $10.00")).toBeInTheDocument();
    expect(screen.getByText("2 coders")).toBeInTheDocument();

    const progress = container.querySelector('[style="width: 90%;"]');
    expect(progress).toHaveClass("bg-destructive");
  });

  it("falls back to an untitled team label when no title is available", () => {
    render(
      <TeamCard team={makeTeam({ name: "", taskTitle: "", totalBudget: 0 })} />,
    );

    expect(screen.getByText("Untitled Team")).toBeInTheDocument();
  });

  it("marks every pipeline dot as failed for terminal failure states", () => {
    render(<TeamCard team={makeTeam({ status: "failed" })} />);

    expect(screen.getByTitle("plan: failed")).toBeInTheDocument();
    expect(screen.getByTitle("execute: failed")).toBeInTheDocument();
    expect(screen.getByTitle("review: failed")).toBeInTheDocument();
  });
});
