import { render, screen } from "@testing-library/react";
import { IMMessageHistory } from "./im-message-history";
import type { IMDelivery } from "@/lib/stores/im-store";

const storeState: {
  deliveries: IMDelivery[];
  loading: boolean;
} = {
  deliveries: [],
  loading: false,
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
  });

  it("shows a loading state while delivery history is being fetched", () => {
    storeState.loading = true;

    render(<IMMessageHistory />);

    expect(screen.getByText("Loading deliveries...")).toBeInTheDocument();
  });

  it("renders delivery rows and failure details", () => {
    const createdAt = "2026-03-26T02:00:00.000Z";
    storeState.deliveries = [
      {
        id: "delivery-1",
        channelId: "ops-alerts",
        platform: "feishu",
        eventType: "task.completed",
        status: "delivered",
        createdAt,
      },
      {
        id: "delivery-2",
        channelId: "ops-alerts",
        platform: "slack",
        eventType: "review.completed",
        status: "failed",
        failureReason: "Webhook rejected payload",
        createdAt,
      },
    ];

    render(<IMMessageHistory />);

    expect(screen.getByText("Message History")).toBeInTheDocument();
    expect(screen.getByText("task.completed")).toBeInTheDocument();
    expect(screen.getByText("review.completed")).toBeInTheDocument();
    expect(screen.getByText("delivered")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
    expect(screen.getByText("Webhook rejected payload")).toBeInTheDocument();
    expect(screen.getAllByText(new Date(createdAt).toLocaleString())).toHaveLength(2);
  });
});
