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
    };

    render(<IMBridgeHealth />);

    expect(screen.getByText("Bridge Health")).toBeInTheDocument();
    expect(screen.getByText("Connected")).toBeInTheDocument();
    expect(screen.getByText("healthy")).toBeInTheDocument();
    expect(screen.getByText(new Date(heartbeat).toLocaleString())).toBeInTheDocument();
    expect(screen.getByText("DingTalk")).toBeInTheDocument();
    expect(screen.getByText("QQ Bot")).toBeInTheDocument();
    expect(screen.getByText("action_card")).toBeInTheDocument();
    expect(screen.getAllByText("webhook")).toHaveLength(2);
    expect(screen.getByText("session_webhook")).toBeInTheDocument();
  });

  it("shows disconnected fallbacks when no heartbeat or providers exist", () => {
    render(<IMBridgeHealth />);

    expect(screen.getByText("Disconnected")).toBeInTheDocument();
    expect(screen.getByText("disconnected")).toBeInTheDocument();
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
    };

    render(<IMBridgeHealth />);

    expect(screen.getByText("DingTalk")).toBeInTheDocument();
    expect(screen.getByText("QQ Bot")).toBeInTheDocument();
  });
});
