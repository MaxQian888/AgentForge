import { render, screen } from "@testing-library/react";
import mockImMessages from "@/messages/en/im.json";
import { IMBridgeHealth } from "./im-bridge-health";
import type { IMBridgeStatus } from "@/lib/stores/im-store";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) =>
    key.split(".").reduce((value: unknown, part: string) => {
      if (value && typeof value === "object" && part in (value as Record<string, unknown>)) {
        return (value as Record<string, unknown>)[part];
      }
      return key;
    }, mockImMessages),
}));

const storeState: { bridgeStatus: IMBridgeStatus } = {
  bridgeStatus: {
    registered: false,
    lastHeartbeat: null,
    providers: [],
    providerDetails: [],
    health: "disconnected",
    pendingDeliveries: 0,
    recentFailures: 0,
    recentDowngrades: 0,
    averageLatencyMs: 0,
  },
};

jest.mock("@/lib/stores/im-store", () => ({
  useIMStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

describe("IMBridgeHealth", () => {
  beforeEach(() => {
    storeState.bridgeStatus = {
      registered: false,
      lastHeartbeat: null,
      providers: [],
      providerDetails: [],
      health: "disconnected",
      pendingDeliveries: 0,
      recentFailures: 0,
      recentDowngrades: 0,
      averageLatencyMs: 0,
    };
  });

  it("renders provider capability summaries", () => {
    const heartbeat = "2026-03-26T01:23:45.000Z";
    storeState.bridgeStatus = {
      registered: true,
      lastHeartbeat: heartbeat,
      providers: ["dingtalk", "qqbot"],
      providerDetails: [
        {
          platform: "dingtalk",
          status: "online",
          transport: "live",
          pendingDeliveries: 2,
          recentFailures: 1,
          recentDowngrades: 1,
          lastDeliveryAt: "2026-03-26T01:20:00.000Z",
          diagnostics: {
            provider_id: "dingtalk",
            webhook_status: "healthy",
          },
          capabilityMatrix: {
            commandSurface: "mixed",
            structuredSurface: "action_card",
            actionCallbackMode: "webhook",
            asyncUpdateModes: ["reply", "session_webhook"],
            messageScopes: ["chat"],
            mutability: {
              canEdit: false,
              canDelete: false,
              prefersInPlace: false,
            },
          },
        },
        {
          platform: "qqbot",
          status: "online",
          transport: "live",
          pendingDeliveries: 0,
          recentFailures: 0,
          recentDowngrades: 0,
          diagnostics: {},
          capabilityMatrix: {
            commandSurface: "mixed",
            structuredSurface: "none",
            actionCallbackMode: "webhook",
            asyncUpdateModes: ["reply"],
            messageScopes: ["chat"],
            mutability: {
              canEdit: false,
              canDelete: false,
              prefersInPlace: false,
            },
          },
        },
      ],
      health: "healthy",
      pendingDeliveries: 2,
      recentFailures: 1,
      recentDowngrades: 1,
      averageLatencyMs: 420,
    };

    render(<IMBridgeHealth />);

    expect(screen.getByText("Bridge Health")).toBeInTheDocument();
    expect(screen.getByText("Connected")).toBeInTheDocument();
    expect(screen.getAllByText("Healthy").length).toBeGreaterThan(0);
    expect(screen.getByText(new Date(heartbeat).toLocaleString())).toBeInTheDocument();
    expect(screen.getByText("Pending Deliveries")).toBeInTheDocument();
    expect(screen.getAllByText("2").length).toBeGreaterThan(0);
    expect(screen.getByText("Recent Failures")).toBeInTheDocument();
    expect(screen.getByText("420 ms")).toBeInTheDocument();
    expect(screen.getByText("DingTalk")).toBeInTheDocument();
    expect(screen.getByText("QQ Bot")).toBeInTheDocument();
    expect(screen.getByText("action_card")).toBeInTheDocument();
    expect(screen.getAllByText("webhook")).toHaveLength(2);
    expect(screen.getByText("session_webhook")).toBeInTheDocument();
    expect(screen.getByText("webhook_status")).toBeInTheDocument();
    expect(screen.getAllByText("Healthy").length).toBeGreaterThan(0);
  });

  it("shows disconnected fallbacks when no heartbeat or providers exist", () => {
    render(<IMBridgeHealth />);

    expect(screen.getAllByText("Disconnected").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("No heartbeat")).toBeInTheDocument();
    expect(
      screen.getByText(
        /No providers registered\. Ensure the IM bridge is running and configured\./i,
      ),
    ).toBeInTheDocument();
  });

  it("renders flat provider registrations when provider details are unavailable", () => {
    storeState.bridgeStatus = {
      registered: true,
      lastHeartbeat: "2026-03-26T01:23:45.000Z",
      providers: ["dingtalk", "qqbot"],
      providerDetails: [],
      health: "healthy",
      pendingDeliveries: 0,
      recentFailures: 0,
      recentDowngrades: 0,
      averageLatencyMs: 0,
    };

    render(<IMBridgeHealth />);

    expect(screen.getByText("DingTalk")).toBeInTheDocument();
    expect(screen.getByText("QQ Bot")).toBeInTheDocument();
  });
});
