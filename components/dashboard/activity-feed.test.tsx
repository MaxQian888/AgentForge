jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "activityFeed.title": "Activity Feed",
      "activityFeed.empty": "No activity yet.",
    };
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import { ActivityFeed } from "./activity-feed";

describe("ActivityFeed", () => {
  beforeEach(() => {
    jest.spyOn(Date, "now").mockReturnValue(
      new Date("2026-03-30T12:00:00.000Z").getTime(),
    );
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("renders the empty state when there are no events", () => {
    render(<ActivityFeed events={[]} />);

    expect(screen.getByText("Activity Feed")).toBeInTheDocument();
    expect(screen.getByText("No activity yet.")).toBeInTheDocument();
  });

  it("limits visible events to eight items and shows relative timestamps", () => {
    render(
      <ActivityFeed
        events={Array.from({ length: 9 }).map((_, index) => ({
          id: `event-${index}`,
          type: "review_completed",
          title: `Event ${index}`,
          status: index % 2 === 0 ? "active" : "pending",
          timestamp: "2026-03-30T11:59:00.000Z",
        }))}
      />,
    );

    expect(screen.getByText("Event 0")).toBeInTheDocument();
    expect(screen.getByText("Event 7")).toBeInTheDocument();
    expect(screen.queryByText("Event 8")).not.toBeInTheDocument();
    expect(screen.getAllByText("1m ago")).toHaveLength(8);
  });
});
