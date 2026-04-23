jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "teamHealth.title": "Team Health",
      "teamHealth.empty": "No team members yet.",
      "status.overloaded": "overloaded",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { TeamHealthWidget } from "./team-health-widget";

describe("TeamHealthWidget", () => {
  it("renders the empty state when there are no members", () => {
    render(<TeamHealthWidget members={[]} />);

    expect(screen.getByText("No team members yet.")).toBeInTheDocument();
  });

  it("renders member workload bars and statuses", () => {
    const { container } = render(
      <TeamHealthWidget
        members={[
          {
            id: "member-1",
            name: "Alice",
            role: "Lead",
            workloadPercent: 92,
            status: "overloaded",
          },
        ]}
      />,
    );
    const indicator = container.querySelector('[data-slot="progress-indicator"]');

    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("Lead")).toBeInTheDocument();
    expect(screen.getByText("overloaded")).toBeInTheDocument();
    expect(indicator).toHaveClass("bg-red-500");
    expect(indicator).toHaveStyle({ transform: "translateX(-8%)" });
  });
});
