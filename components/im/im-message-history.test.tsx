import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import mockImMessages from "@/messages/en/im.json";
import { IMMessageHistory } from "./im-message-history";
import type { IMDelivery } from "@/lib/stores/im-store";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) =>
    key.split(".").reduce((value: unknown, part: string) => {
      if (value && typeof value === "object" && part in (value as Record<string, unknown>)) {
        return (value as Record<string, unknown>)[part];
      }
      return key;
    }, mockImMessages),
}));

const retryDelivery = jest.fn();

const storeState: {
  deliveries: IMDelivery[];
  loading: boolean;
  retryDelivery: typeof retryDelivery;
} = {
  deliveries: [],
  loading: false,
  retryDelivery,
};

jest.mock("@/lib/stores/im-store", () => ({
  useIMStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

describe("IMMessageHistory", () => {
  beforeEach(() => {
    storeState.deliveries = [];
    storeState.loading = false;
    retryDelivery.mockReset().mockResolvedValue(undefined);
  });

  it("shows a loading state while delivery history is being fetched", () => {
    storeState.loading = true;

    render(<IMMessageHistory />);

    expect(screen.getByText("Loading deliveries...")).toBeInTheDocument();
  });

  it("renders downgrade diagnostics, payload preview, and retry", async () => {
    const user = userEvent.setup();
    const createdAt = "2026-03-26T02:00:00.000Z";
    storeState.deliveries = [
      {
        id: "delivery-1",
        channelId: "ops-alerts",
        platform: "dingtalk",
        eventType: "review.requested",
        status: "failed",
        failureReason: "Webhook rejected payload",
        downgradeReason: "actioncard_send_failed",
        content: "Review requested",
        metadata: {
          fallback_reason: "actioncard_send_failed",
        },
        createdAt,
      },
    ];

    render(<IMMessageHistory />);

    expect(screen.getByText("Message History")).toBeInTheDocument();
    expect(screen.getByText("review.requested")).toBeInTheDocument();
    expect(screen.getByText("actioncard_send_failed")).toBeInTheDocument();
    expect(screen.getByText("Webhook rejected payload")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry" }));

    expect(retryDelivery).toHaveBeenCalledWith("delivery-1");

    await user.click(screen.getByRole("button", { name: "Preview payload" }));

    expect(screen.getByText("Delivery payload")).toBeInTheDocument();
    expect(screen.getByText(/Review requested/)).toBeInTheDocument();
    expect(screen.getByText(new Date(createdAt).toLocaleString())).toBeInTheDocument();
  });
});
