import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import mockImMessages from "@/messages/en/im.json";
import { IMMessageHistory } from "./im-message-history";
import type { IMDelivery } from "@/lib/stores/im-store";

jest.mock("@/components/ui/select", () => {
  function flattenOptions(children: ReactNode): Array<{ value: string; label: string }> {
    const options: Array<{ value: string; label: string }> = [];
    function visit(node: ReactNode) {
      Children.forEach(node, (child) => {
        if (!isValidElement(child)) return;
        const el = child as ReactElement<{ children?: ReactNode; value?: string }>;
        if (el.props.value !== undefined) {
          options.push({
            value: el.props.value,
            label: typeof el.props.children === "string" ? el.props.children : String(el.props.value),
          });
          return;
        }
        visit(el.props.children);
      });
    }
    visit(children);
    return options;
  }

  function extractAriaLabel(children: ReactNode): string | undefined {
    let label: string | undefined;
    Children.forEach(children, (child) => {
      if (!isValidElement(child)) return;
      const el = child as ReactElement<{ "aria-label"?: string }>;
      if (el.props["aria-label"]) {
        label = el.props["aria-label"];
      }
    });
    return label;
  }

  return {
    Select: ({
      value,
      onValueChange,
      children,
    }: {
      value?: string;
      onValueChange?: (value: string) => void;
      children?: ReactNode;
    }) => {
      const options = flattenOptions(children);
      const ariaLabel = extractAriaLabel(children);
      return (
        <select
          aria-label={ariaLabel}
          value={value}
          onChange={(e) => onValueChange?.(e.target.value)}
        >
          {options.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
      );
    },
    SelectTrigger: ({ children }: { children?: ReactNode; "aria-label"?: string }) => <>{children}</>,
    SelectValue: () => null,
    SelectContent: ({ children }: { children?: ReactNode }) => <>{children}</>,
    SelectItem: ({ children }: { children?: ReactNode }) => <>{children}</>,
  };
});

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
const retryDeliveries = jest.fn();
const fetchDeliveryHistory = jest.fn();
const setHistoryFilters = jest.fn();

const storeState: {
  deliveries: IMDelivery[];
  loading: boolean;
  retryDelivery: typeof retryDelivery;
  retryDeliveries: typeof retryDeliveries;
  fetchDeliveryHistory: typeof fetchDeliveryHistory;
  setHistoryFilters: typeof setHistoryFilters;
  historyFilters: {
    status?: string;
    platform?: string;
    eventType?: string;
  };
} = {
  deliveries: [],
  loading: false,
  retryDelivery,
  retryDeliveries,
  fetchDeliveryHistory,
  setHistoryFilters,
  historyFilters: {},
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
    retryDeliveries.mockReset().mockResolvedValue([]);
    fetchDeliveryHistory.mockReset().mockResolvedValue(undefined);
    setHistoryFilters.mockReset();
    storeState.historyFilters = {};
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
        processedAt: "2026-03-26T02:00:05.000Z",
        latencyMs: 5000,
      },
    ];

    render(<IMMessageHistory />);

    expect(screen.getByText("Message History")).toBeInTheDocument();
    expect(screen.getByText("review.requested")).toBeInTheDocument();
    expect(screen.getByText("actioncard_send_failed")).toBeInTheDocument();
    expect(screen.getByText("Webhook rejected payload")).toBeInTheDocument();
    expect(screen.getByText("5,000 ms")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry" }));

    expect(retryDelivery).toHaveBeenCalledWith("delivery-1");

    await user.click(screen.getByRole("checkbox", { name: "Select delivery-1" }));
    await user.click(screen.getByRole("button", { name: "Retry selected" }));

    expect(retryDeliveries).toHaveBeenCalledWith(["delivery-1"]);

    await user.click(screen.getByRole("button", { name: "Preview payload" }));

    expect(screen.getByText("Delivery payload")).toBeInTheDocument();
    expect(screen.getByText(/Review requested/)).toBeInTheDocument();
    expect(screen.getAllByText(new Date(createdAt).toLocaleString()).length).toBeGreaterThan(0);
    expect(screen.getByText("Queued at")).toBeInTheDocument();
    expect(screen.getByText("Processed at")).toBeInTheDocument();
    expect(screen.getAllByText("Latency").length).toBeGreaterThan(0);
  });

  it("applies and clears delivery filters", async () => {
    const user = userEvent.setup();

    render(<IMMessageHistory />);

    await user.selectOptions(screen.getByLabelText("Status"), "failed");
    await user.type(screen.getByLabelText("Event Type"), "task.created");
    await user.click(screen.getByRole("button", { name: "Apply filters" }));

    expect(setHistoryFilters).toHaveBeenCalledWith({
      status: "failed",
      platform: "",
      eventType: "task.created",
    });
    expect(fetchDeliveryHistory).toHaveBeenCalledWith({
      status: "failed",
      platform: "",
      eventType: "task.created",
    });

    await user.click(screen.getByRole("button", { name: "Clear filters" }));

    expect(setHistoryFilters).toHaveBeenLastCalledWith({});
    expect(fetchDeliveryHistory).toHaveBeenLastCalledWith({});
  });
});
