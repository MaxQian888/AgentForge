import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import IMBridgePage from "./page";

const imState = {
  channels: [{ id: "channel-1", platform: "slack", channelId: "C123" }],
  loading: false,
  error: null as string | null,
  bridgeStatus: {
    health: "healthy",
    pendingDeliveries: 2,
    recentFailures: 1,
  },
  deliveries: [
    { id: "delivery-1", status: "delivered" },
    { id: "delivery-2", status: "failed" },
    { id: "delivery-3", status: "suppressed" },
  ],
  lastTestSendResult: null as
    | null
    | { status: string; failureReason?: string; downgradeReason?: string },
  fetchChannels: jest.fn(),
  fetchBridgeStatus: jest.fn(),
  fetchDeliveryHistory: jest.fn(),
  fetchEventTypes: jest.fn(),
  testSend: jest.fn(),
};

jest.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => (key: string) =>
    namespace ? `${namespace}.${key}` : key,
}));

jest.mock("@/hooks/use-breadcrumbs", () => ({
  useBreadcrumbs: jest.fn(),
}));

jest.mock("@/components/shared/page-header", () => ({
  PageHeader: ({
    title,
    actions,
  }: {
    title: string;
    actions?: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      {actions}
    </div>
  ),
}));

jest.mock("@/components/shared/error-banner", () => ({
  ErrorBanner: ({
    message,
    onRetry,
  }: {
    message: string;
    onRetry: () => void;
  }) => (
    <button type="button" onClick={onRetry}>
      {message}
    </button>
  ),
}));

jest.mock("@/components/im/im-channel-config", () => ({
  IMChannelConfig: () => <div data-testid="im-channel-config" />,
}));

jest.mock("@/components/im/im-bridge-health", () => ({
  IMBridgeHealth: () => <div data-testid="im-bridge-health" />,
}));

jest.mock("@/components/im/im-message-history", () => ({
  IMMessageHistory: () => <div data-testid="im-message-history" />,
}));

jest.mock("@/components/ui/tabs", () => ({
  Tabs: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TabsList: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TabsTrigger: ({ children }: { children: React.ReactNode }) => <button type="button">{children}</button>,
  TabsContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock("@/lib/stores/im-store", () => ({
  useIMStore: (selector: (state: typeof imState) => unknown) => selector(imState),
}));

describe("IMBridgePage", () => {
  beforeEach(() => {
    imState.channels = [{ id: "channel-1", platform: "slack", channelId: "C123" }];
    imState.loading = false;
    imState.error = null;
    imState.bridgeStatus.health = "healthy";
    imState.bridgeStatus.pendingDeliveries = 2;
    imState.bridgeStatus.recentFailures = 1;
    imState.deliveries = [
      { id: "delivery-1", status: "delivered" },
      { id: "delivery-2", status: "failed" },
      { id: "delivery-3", status: "suppressed" },
    ];
    imState.lastTestSendResult = null;
    imState.fetchChannels.mockReset();
    imState.fetchBridgeStatus.mockReset();
    imState.fetchDeliveryHistory.mockReset();
    imState.fetchEventTypes.mockReset();
    imState.testSend.mockReset();
  });

  it("loads all IM bridge datasets on mount", () => {
    render(<IMBridgePage />);

    expect(imState.fetchChannels).toHaveBeenCalledTimes(1);
    expect(imState.fetchBridgeStatus).toHaveBeenCalledTimes(1);
    expect(imState.fetchDeliveryHistory).toHaveBeenCalledTimes(1);
    expect(imState.fetchEventTypes).toHaveBeenCalledTimes(1);
    expect(screen.getByText("healthy")).toBeInTheDocument();
    expect(screen.getByText("im.summaryPending: 2")).toBeInTheDocument();
    expect(screen.getByText("im.summaryFailures: 1")).toBeInTheDocument();
    expect(screen.getByText("im.summarySuccessRate: 67%")).toBeInTheDocument();
    expect(screen.getByText("im.testSendTitle")).toBeInTheDocument();
    expect(screen.getByTestId("im-channel-config")).toBeInTheDocument();
    expect(screen.getByTestId("im-bridge-health")).toBeInTheDocument();
    expect(screen.getByTestId("im-message-history")).toBeInTheDocument();
  });

  it("retries the page fetches from both the refresh button and the error banner", async () => {
    const user = userEvent.setup();
    imState.error = "Bridge unavailable";

    render(<IMBridgePage />);

    await user.click(screen.getByRole("button", { name: "im.refresh" }));
    await user.click(screen.getByRole("button", { name: "Bridge unavailable" }));

    expect(imState.fetchChannels).toHaveBeenCalledTimes(3);
    expect(imState.fetchBridgeStatus).toHaveBeenCalledTimes(3);
    expect(imState.fetchDeliveryHistory).toHaveBeenCalledTimes(3);
    expect(imState.fetchEventTypes).toHaveBeenCalledTimes(3);
  });

  it("uses the configured channel for the currently selected test platform", async () => {
    const user = userEvent.setup();
    imState.channels = [
      {
        id: "channel-feishu",
        platform: "feishu",
        channelId: "oc_feishu",
      },
      {
        id: "channel-slack",
        platform: "slack",
        channelId: "C123",
      },
    ];

    render(<IMBridgePage />);

    await user.selectOptions(screen.getByLabelText("im.testSendPlatform"), "slack");
    await user.click(screen.getByRole("button", { name: "im.testSendButton" }));

    await waitFor(() => {
      expect(imState.testSend).toHaveBeenCalledWith({
        platform: "slack",
        channelId: "C123",
        text: "ping",
      });
    });
  });

  it("does not invent a fallback test target when no configured channel exists", () => {
    imState.channels = [];

    render(<IMBridgePage />);

    expect(screen.getByLabelText("im.testSendPlatform")).toBeDisabled();
    expect(screen.getByRole("button", { name: "im.testSendButton" })).toBeDisabled();
    expect(screen.getByDisplayValue("im.channels.noChannels")).toBeInTheDocument();
  });

  it("surfaces explicit test-send failure details from the backend result", () => {
    imState.lastTestSendResult = {
      status: "failed",
      failureReason: "notify URL not configured",
      downgradeReason: "compatibility_fallback",
    };

    render(<IMBridgePage />);

    expect(screen.getByText("im.testSendResult: failed")).toBeInTheDocument();
    expect(screen.getByText("notify URL not configured")).toBeInTheDocument();
    expect(screen.getByText("downgrade: compatibility_fallback")).toBeInTheDocument();
  });
});
