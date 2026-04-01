import { Children, isValidElement, type ReactNode, type ReactElement } from "react";

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) return;
        const element = child as ReactElement<{ children?: ReactNode; value?: string }>;
        if (element.props.value !== undefined) {
          options.push({
            value: element.props.value,
            label: typeof element.props.children === "string" ? element.props.children : String(element.props.value),
          });
          return;
        }
        visit(element.props.children);
      });
    }
    visit(children);
    return options;
  }

  return {
    Select: ({ value, onValueChange, children }: { value?: string; onValueChange?: (v: string) => void; children?: ReactNode }) => {
      const options = flattenOptions(children);
      return (
        <select value={value} onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onValueChange?.(e.target.value)}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectItem: ({ children }: { children?: ReactNode }) => <>{children}</>,
  };
});

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const map: Record<string, string> = {
      "activityFeed.title": "Activity Feed",
      "activityFeed.empty": "No activity yet.",
      "activityFeed.typeLabel": "Type",
      "activityFeed.timeRangeLabel": "Time Range",
      "activityFeed.type.all": "All",
      "activityFeed.type.task": "Tasks",
      "activityFeed.type.review": "Reviews",
      "activityFeed.type.agent": "Agents",
      "activityFeed.type.system": "System",
      "activityFeed.timeRange.all": "All Time",
      "activityFeed.timeRange.last24h": "Last 24 Hours",
      "activityFeed.timeRange.last7d": "Last 7 Days",
    };
    if (key === "activityFeed.count") {
      return `Events: ${values?.count ?? 0}`;
    }
    return map[key] ?? key;
  },
}));

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

  it("filters events by category and time range while updating the visible count", async () => {
    const user = userEvent.setup();

    render(
      <ActivityFeed
        events={[
          {
            id: "event-agent",
            type: "agent_started",
            title: "Agent started",
            status: "active",
            timestamp: "2026-03-30T11:30:00.000Z",
          },
          {
            id: "event-review",
            type: "review_completed",
            title: "Review completed",
            status: "pending",
            timestamp: "2026-03-29T09:00:00.000Z",
          },
          {
            id: "event-system",
            type: "deploy-start",
            title: "Deploy started",
            status: "running",
            timestamp: "2026-03-30T11:55:00.000Z",
          },
        ]}
      />,
    );

    expect(screen.getByText("Events: 3")).toBeInTheDocument();

    const selects = screen.getAllByRole("combobox");
    await user.selectOptions(selects[0], "agent");

    expect(screen.getByText("Agent started")).toBeInTheDocument();
    expect(screen.queryByText("Review completed")).not.toBeInTheDocument();
    expect(screen.getByText("Events: 1")).toBeInTheDocument();

    await user.selectOptions(selects[0], "all");
    await user.selectOptions(selects[1], "last24h");

    expect(screen.getByText("Agent started")).toBeInTheDocument();
    expect(screen.getByText("Deploy started")).toBeInTheDocument();
    expect(screen.queryByText("Review completed")).not.toBeInTheDocument();
    expect(screen.getByText("Events: 2")).toBeInTheDocument();
  });
});
