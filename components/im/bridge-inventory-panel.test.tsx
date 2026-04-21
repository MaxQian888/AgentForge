import { render, screen } from "@testing-library/react";
import { BridgeInventoryPanel } from "./bridge-inventory-panel";
import type { IMBridgeInstance } from "@/lib/stores/im-store";

const sampleBridges: IMBridgeInstance[] = [
  {
    bridgeId: "bridge-abc",
    platform: "feishu",
    transport: "live",
    status: "online",
    providers: [
      {
        id: "feishu",
        transport: "live",
        readinessTier: "full_native_lifecycle",
        capabilityMatrix: { supportsRichMessages: true, supportsAttachments: true },
        tenants: ["acme"],
        metadataSource: "builtin",
      },
      {
        id: "slack",
        transport: "stub",
        tenants: ["beta"],
        metadataSource: "builtin",
      },
    ],
    commandPlugins: [
      { id: "@acme/jira", version: "1.0.0", commands: ["/jira"], tenants: ["acme"] },
    ],
  },
];

describe("BridgeInventoryPanel", () => {
  it("renders providers with readiness tier and tenant badges", () => {
    render(<BridgeInventoryPanel bridges={sampleBridges} />);
    expect(screen.getByText("feishu")).toBeInTheDocument();
    expect(screen.getByText("full_native_lifecycle")).toBeInTheDocument();
    expect(screen.getByText("acme")).toBeInTheDocument();
    expect(screen.getByText("slack")).toBeInTheDocument();
  });

  it("renders command plugins with command list", () => {
    render(<BridgeInventoryPanel bridges={sampleBridges} />);
    expect(screen.getByText("@acme/jira")).toBeInTheDocument();
    expect(screen.getByText("/jira")).toBeInTheDocument();
  });

  it("renders empty state when no bridges are online", () => {
    render(<BridgeInventoryPanel bridges={[]} />);
    expect(screen.getByText(/no IM bridges online/i)).toBeInTheDocument();
  });

  it("dims offline bridges", () => {
    const offline: IMBridgeInstance[] = [{ ...sampleBridges[0], status: "offline" }];
    render(<BridgeInventoryPanel bridges={offline} />);
    const card = screen.getByTestId("bridge-card-bridge-abc");
    expect(card.className).toMatch(/opacity-/);
  });
});
