import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import IMBridgePage from "./page";

const imState = {
  loading: false,
  error: null as string | null,
  bridgeStatus: {
    health: "healthy",
  },
  fetchChannels: jest.fn(),
  fetchBridgeStatus: jest.fn(),
  fetchDeliveryHistory: jest.fn(),
  fetchEventTypes: jest.fn(),
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
    imState.loading = false;
    imState.error = null;
    imState.bridgeStatus.health = "healthy";
    imState.fetchChannels.mockReset();
    imState.fetchBridgeStatus.mockReset();
    imState.fetchDeliveryHistory.mockReset();
    imState.fetchEventTypes.mockReset();
  });

  it("loads all IM bridge datasets on mount", () => {
    render(<IMBridgePage />);

    expect(imState.fetchChannels).toHaveBeenCalledTimes(1);
    expect(imState.fetchBridgeStatus).toHaveBeenCalledTimes(1);
    expect(imState.fetchDeliveryHistory).toHaveBeenCalledTimes(1);
    expect(imState.fetchEventTypes).toHaveBeenCalledTimes(1);
    expect(screen.getByText("healthy")).toBeInTheDocument();
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
});
