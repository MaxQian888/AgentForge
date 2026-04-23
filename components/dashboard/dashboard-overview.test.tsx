import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DashboardOverview } from "./dashboard-overview";
import type { DashboardSummary } from "@/lib/dashboard/summary";

describe("DashboardOverview", () => {
  const summary: DashboardSummary = {
    scope: {
      projectId: "project-1",
      projectName: "AgentForge",
      projectsCount: 2,
    },
    headline: {
      activeAgents: 2,
      tasksInProgress: 4,
      pendingReviews: 1,
      weeklyCost: 42.75,
    },
    progress: {
      total: 7,
      inbox: 1,
      triaged: 1,
      assigned: 1,
      inProgress: 4,
      inReview: 1,
      done: 0,
    },
    team: {
      totalMembers: 5,
      activeHumans: 3,
      activeAgents: 2,
      activeAgentRuns: 2,
      overloadedMembers: 1,
    },
    activity: [
      {
        id: "activity-1",
        type: "review_completed",
        title: "Deep review completed",
        message: "Review feedback is ready.",
        href: "/team?project=project-1&focus=review",
        createdAt: "2026-03-24T09:30:00.000Z",
      },
    ],
    risks: [
      {
        id: "risk-1",
        kind: "budget-pressure",
        title: "Budget pressure detected",
        description: "Spend is approaching the weekly threshold.",
        href: "/cost",
      },
    ],
    links: {
      projects: "/projects",
      team: "/team?project=project-1",
      agents: "/team?project=project-1&focus=agents",
      reviews: "/team?project=project-1&focus=review",
    },
  };

  it("renders insight sections, activity, and partial section errors", async () => {
    const user = userEvent.setup();
    const onRetry = jest.fn();

    render(
      <DashboardOverview
        summary={summary}
        loading={false}
        error={null}
        sectionErrors={{ activity: "Notifications unavailable" }}
        onRetry={onRetry}
      />
    );

    expect(screen.getByText("AgentForge")).toBeInTheDocument();
    expect(screen.getByText("Weekly Cost")).toBeInTheDocument();
    expect(screen.getByText("$42.75")).toBeInTheDocument();
    expect(screen.getByText("Deep review completed")).toBeInTheDocument();
    expect(screen.getByText("Budget pressure detected")).toBeInTheDocument();
    expect(screen.getByText("Notifications unavailable")).toBeInTheDocument();
    expect(screen.getByText("N")).toBeInTheDocument();
    expect(screen.getByText("A")).toBeInTheDocument();
    expect(screen.getByText("S")).toBeInTheDocument();
    expect(screen.getByText("R")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry Section" }));
    expect(onRetry).toHaveBeenCalledWith("activity");
  });

  it("renders an explicit empty state when the selected scope has no work", () => {
    render(
      <DashboardOverview
        summary={{
          ...summary,
          headline: {
            activeAgents: 0,
            tasksInProgress: 0,
            pendingReviews: 0,
            weeklyCost: 0,
          },
          progress: {
            total: 0,
            inbox: 0,
            triaged: 0,
            assigned: 0,
            inProgress: 0,
            inReview: 0,
            done: 0,
          },
          team: {
            totalMembers: 0,
            activeHumans: 0,
            activeAgents: 0,
            activeAgentRuns: 0,
            overloadedMembers: 0,
          },
          activity: [],
          risks: [],
          bootstrap: {
            unresolvedCount: 2,
            phases: [
              {
                id: "governance",
                title: "Governance",
                state: "attention",
                reason: "Repository or coding-agent defaults still need configuration.",
                href: "/settings?project=project-1&section=repository",
                actionLabel: "Configure Governance",
              },
              {
                id: "team",
                title: "Team",
                state: "attention",
                reason: "Add the first human or agent collaborator.",
                href: "/team?project=project-1&focus=add-member",
                actionLabel: "Add First Member",
              },
            ],
            nextActions: [
              {
                id: "configure-governance",
                label: "Configure Governance",
                href: "/settings?project=project-1&section=repository",
              },
              {
                id: "add-member",
                label: "Add First Member",
                href: "/team?project=project-1&focus=add-member",
              },
            ],
          },
        }}
        loading={false}
        error={null}
        sectionErrors={{}}
        onRetry={jest.fn()}
      />
    );

    expect(
      screen.getByText("No delivery signals yet for this scope.")
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Configure Governance" })
    ).toHaveAttribute("href", "/settings?project=project-1&section=repository");
  });

  it("surfaces lifecycle-aware bootstrap actions for incomplete projects", () => {
    render(
      <DashboardOverview
        summary={{
          ...summary,
          bootstrap: {
            unresolvedCount: 2,
            phases: [
              {
                id: "governance",
                title: "Governance",
                state: "attention",
                reason: "Repository or coding-agent defaults still need configuration.",
                href: "/settings?project=project-1&section=repository",
                actionLabel: "Configure Governance",
              },
              {
                id: "team",
                title: "Team",
                state: "attention",
                reason: "Add the first human or agent collaborator.",
                href: "/team?project=project-1&focus=add-member",
                actionLabel: "Add First Member",
              },
            ],
            nextActions: [
              {
                id: "configure-governance",
                label: "Configure Governance",
                href: "/settings?project=project-1&section=repository",
              },
              {
                id: "add-member",
                label: "Add First Member",
                href: "/team?project=project-1&focus=add-member",
              },
            ],
          },
        } as DashboardSummary}
        loading={false}
        error={null}
        sectionErrors={{}}
        onRetry={jest.fn()}
      />
    );

    expect(screen.getByText("Project Bootstrap")).toBeInTheDocument();
    expect(
      screen.getByText("Repository or coding-agent defaults still need configuration."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Configure Governance" }),
    ).toHaveAttribute("href", "/settings?project=project-1&section=repository");
    expect(
      screen.getByRole("link", { name: "Add First Member" }),
    ).toHaveAttribute("href", "/team?project=project-1&focus=add-member");
  });
});
