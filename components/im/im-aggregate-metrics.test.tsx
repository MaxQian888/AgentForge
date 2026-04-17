import { render, screen } from "@testing-library/react";
import mockImMessages from "@/messages/en/im.json";
import { IMAggregateMetrics } from "./im-aggregate-metrics";
import type { IMBridgeStatus, IMDelivery } from "@/lib/stores/im-store";

jest.mock("next-intl", () => ({
  useTranslations: () =>
    (key: string, values?: Record<string, string | number>) => {
      const resolved = key.split(".").reduce((acc: unknown, part: string) => {
        if (acc && typeof acc === "object" && part in (acc as Record<string, unknown>)) {
          return (acc as Record<string, unknown>)[part];
        }
        return key;
      }, mockImMessages);
      if (typeof resolved !== "string") return key;
      if (!values) return resolved;
      return Object.entries(values).reduce(
        (out, [name, value]) => out.replace(new RegExp(`\\{${name}\\}`, "g"), String(value)),
        resolved,
      );
    },
}));

jest.mock("@/components/shared/metric-card", () => ({
  MetricCard: ({ label, value }: { label: string; value: string | number }) => (
    <div data-testid={`metric-${label}`}>{String(value)}</div>
  ),
}));

jest.mock("@/components/ui/alert", () => ({
  Alert: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="queue-alert">{children}</div>
  ),
  AlertTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  AlertDescription: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

const storeState: {
  bridgeStatus: IMBridgeStatus;
  deliveries: IMDelivery[];
} = {
  bridgeStatus: {
    registered: true,
    lastHeartbeat: "2026-04-16T10:00:00.000Z",
    providers: [],
    providerDetails: [],
    health: "healthy",
    pendingDeliveries: 0,
    recentFailures: 0,
    recentDowngrades: 0,
    averageLatencyMs: 0,
  },
  deliveries: [],
};

jest.mock("@/lib/stores/im-store", () => ({
  useIMStore: (selector: (state: typeof storeState) => unknown) => selector(storeState),
}));

describe("IMAggregateMetrics", () => {
  const realNow = Date.now;

  beforeEach(() => {
    Date.now = () => Date.parse("2026-04-16T12:00:00.000Z");
    storeState.bridgeStatus = {
      registered: true,
      lastHeartbeat: "2026-04-16T10:00:00.000Z",
      providers: [],
      providerDetails: [],
      health: "healthy",
      pendingDeliveries: 0,
      recentFailures: 0,
      recentDowngrades: 0,
      averageLatencyMs: 0,
    };
    storeState.deliveries = [];
  });

  afterEach(() => {
    Date.now = realNow;
  });

  it("renders computed totals, success rate, and queue depth", () => {
    storeState.bridgeStatus.pendingDeliveries = 12;
    storeState.deliveries = [
      {
        id: "d1",
        channelId: "c1",
        platform: "slack",
        eventType: "task.created",
        status: "delivered",
        createdAt: "2026-04-16T11:00:00.000Z",
        processedAt: "2026-04-16T11:00:01.000Z",
        latencyMs: 200,
      },
      {
        id: "d2",
        channelId: "c1",
        platform: "slack",
        eventType: "task.completed",
        status: "failed",
        createdAt: "2026-04-16T11:30:00.000Z",
        processedAt: "2026-04-16T11:30:02.000Z",
        latencyMs: 400,
      },
    ];

    render(<IMAggregateMetrics />);

    expect(screen.getByTestId("metric-Messages (24h)")).toHaveTextContent("2");
    expect(screen.getByTestId("metric-Success Rate")).toHaveTextContent("50%");
    expect(screen.getByTestId("metric-Avg Latency")).toHaveTextContent("300 ms");
    expect(screen.getByTestId("metric-Queue Depth")).toHaveTextContent("12");
    expect(screen.queryByTestId("queue-alert")).not.toBeInTheDocument();
  });

  it("shows a backed-up queue warning when pending exceeds the threshold", () => {
    storeState.bridgeStatus.pendingDeliveries = 250;
    storeState.deliveries = [
      {
        id: "d1",
        channelId: "c1",
        platform: "slack",
        eventType: "task.created",
        status: "delivered",
        createdAt: "2026-04-16T11:59:00.000Z",
        processedAt: "2026-04-16T11:59:00.500Z",
        latencyMs: 500,
      },
      {
        id: "d2",
        channelId: "c1",
        platform: "slack",
        eventType: "task.created",
        status: "delivered",
        createdAt: "2026-04-16T11:59:30.000Z",
        processedAt: "2026-04-16T11:59:30.200Z",
        latencyMs: 200,
      },
    ];

    render(<IMAggregateMetrics />);

    expect(screen.getByTestId("queue-alert")).toBeInTheDocument();
    expect(screen.getByText("Queue is backed up")).toBeInTheDocument();
  });

  it("handles an empty dataset without crashing", () => {
    render(<IMAggregateMetrics />);

    expect(screen.getByTestId("metric-Messages (24h)")).toHaveTextContent("0");
    expect(screen.getByTestId("metric-Success Rate")).toHaveTextContent("—");
  });
});
