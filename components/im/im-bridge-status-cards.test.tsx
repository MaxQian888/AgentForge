import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import mockImMessages from "@/messages/en/im.json";
import { IMBridgeStatusCards } from "./im-bridge-status-cards";
import type { IMBridgeStatus } from "@/lib/stores/im-store";

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

const storeState: {
  bridgeStatus: IMBridgeStatus;
} = {
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
  useIMStore: (selector: (state: typeof storeState) => unknown) => selector(storeState),
}));

describe("IMBridgeStatusCards", () => {
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

  it("renders an empty state when no bridges are registered", () => {
    render(<IMBridgeStatusCards />);

    expect(
      screen.getByText(/No bridges registered/i),
    ).toBeInTheDocument();
  });

  it("renders a card per provider with pending/failure metrics and status dot", () => {
    storeState.bridgeStatus = {
      registered: true,
      lastHeartbeat: "2026-04-16T10:00:00.000Z",
      providers: ["slack"],
      providerDetails: [
        {
          platform: "slack",
          status: "online",
          pendingDeliveries: 4,
          recentFailures: 2,
          recentDowngrades: 0,
          lastDeliveryAt: "2026-04-16T09:30:00.000Z",
        },
      ],
      health: "degraded",
      pendingDeliveries: 4,
      recentFailures: 2,
      recentDowngrades: 0,
      averageLatencyMs: 300,
    };

    render(<IMBridgeStatusCards />);

    const card = screen.getByTestId("im-bridge-card-slack");
    expect(card).toHaveAttribute("data-state", "degraded");
    expect(screen.getByTestId("bridge-pending-slack")).toHaveTextContent("4");
    expect(screen.getByTestId("bridge-failures-slack")).toHaveTextContent("2");
  });

  it("invokes the configure and send-test handlers with the platform id", async () => {
    const user = userEvent.setup();
    storeState.bridgeStatus = {
      registered: true,
      lastHeartbeat: "2026-04-16T10:00:00.000Z",
      providers: ["slack"],
      providerDetails: [
        {
          platform: "slack",
          status: "online",
          pendingDeliveries: 0,
          recentFailures: 0,
          recentDowngrades: 0,
        },
      ],
      health: "healthy",
      pendingDeliveries: 0,
      recentFailures: 0,
      recentDowngrades: 0,
      averageLatencyMs: 0,
    };
    const onConfigure = jest.fn();
    const onSendTest = jest.fn();

    render(
      <IMBridgeStatusCards onConfigureProvider={onConfigure} onSendTest={onSendTest} />,
    );

    await user.click(screen.getByRole("button", { name: "Configure" }));
    await user.click(screen.getByRole("button", { name: "Send test" }));

    expect(onConfigure).toHaveBeenCalledWith("slack");
    expect(onSendTest).toHaveBeenCalledWith("slack");
  });
});
