import { render, screen } from "@testing-library/react";
import { IMBridgeHealth } from "./im-bridge-health";
import type { IMBridgeStatus } from "@/lib/stores/im-store";

const storeState: { bridgeStatus: IMBridgeStatus } = {
  bridgeStatus: {
    registered: false,
    lastHeartbeat: null,
    providers: [],
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
      health: "disconnected",
    };
  });

  it("renders the connected bridge summary and registered providers", () => {
    const heartbeat = "2026-03-26T01:23:45.000Z";
    storeState.bridgeStatus = {
      registered: true,
      lastHeartbeat: heartbeat,
      providers: ["feishu", "slack"],
      health: "healthy",
    };

    render(<IMBridgeHealth />);

    expect(screen.getByText("Bridge Health")).toBeInTheDocument();
    expect(screen.getByText("Connected")).toBeInTheDocument();
    expect(screen.getByText("healthy")).toBeInTheDocument();
    expect(screen.getByText(new Date(heartbeat).toLocaleString())).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("feishu")).toBeInTheDocument();
    expect(screen.getByText("slack")).toBeInTheDocument();
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
});
