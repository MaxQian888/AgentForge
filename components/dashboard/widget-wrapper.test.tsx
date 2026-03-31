import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WidgetWrapper } from "./widget-wrapper";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const translations: Record<string, string> = {
      "widget.refresh": "Refresh",
      "widget.configure": "Configure Widget",
      "widget.remove": "Remove Widget",
      "widget.retry": "Retry Widget",
      "widget.emptyFallback": "No widget data yet.",
      "widget.autoRefresh.pause": "Pause Auto Refresh",
      "widget.autoRefresh.resume": "Resume Auto Refresh",
      "widget.autoRefresh.label": "Auto Refresh",
      "widget.autoRefresh.interval.30s": "30s",
      "widget.autoRefresh.interval.60s": "60s",
      "widget.autoRefresh.interval.300s": "5m",
      "widget.autoRefresh.interval.off": "Off",
    };
    if (key === "widget.lastUpdated") {
      return `Last updated ${String(values?.time ?? "")} ago`;
    }

    return translations[key] ?? key;
  },
}));

describe("WidgetWrapper", () => {
  it("renders refresh, configure, and remove actions", async () => {
    const user = userEvent.setup();
    const onRefresh = jest.fn();
    const onConfigure = jest.fn();
    const onRemove = jest.fn();

    render(
      <WidgetWrapper
        title="Throughput"
        onRefresh={onRefresh}
        onConfigure={onConfigure}
        onRemove={onRemove}
      >
        <div>Chart body</div>
      </WidgetWrapper>
    );

    await user.click(screen.getByRole("button", { name: "Refresh" }));
    await user.click(screen.getByRole("button", { name: "Configure Widget" }));
    await user.click(screen.getByRole("button", { name: "Remove Widget" }));

    expect(onRefresh).toHaveBeenCalledTimes(1);
    expect(onConfigure).toHaveBeenCalledTimes(1);
    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it("renders retryable error and empty states", async () => {
    const user = userEvent.setup();
    const onRetry = jest.fn();

    const { rerender } = render(
      <WidgetWrapper
        title="Throughput"
        state="error"
        message="Widget request failed."
        onRetry={onRetry}
      />
    );

    expect(screen.getByText("Widget request failed.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry Widget" }));
    expect(onRetry).toHaveBeenCalledTimes(1);

    rerender(
      <WidgetWrapper title="Throughput" state="empty" message="No widget data yet." />
    );

    expect(screen.getByText("No widget data yet.")).toBeInTheDocument();
  });

  it("renders auto-refresh controls and last-updated metadata when provided", async () => {
    const user = userEvent.setup();
    const onPauseToggle = jest.fn();
    const onIntervalChange = jest.fn();

    render(
      <WidgetWrapper
        title="Throughput"
        onRefresh={jest.fn()}
        autoRefresh={{
          interval: "30s",
          paused: false,
          lastUpdatedLabel: "Last updated 30s ago",
          onPauseToggle,
          onIntervalChange,
        }}
      >
        <div>Chart body</div>
      </WidgetWrapper>
    );

    expect(screen.getByText("Last updated 30s ago")).toBeInTheDocument();
    expect(screen.getByLabelText("Auto Refresh")).toHaveValue("30s");

    await user.selectOptions(screen.getByLabelText("Auto Refresh"), "60s");
    await user.click(screen.getByRole("button", { name: "Pause Auto Refresh" }));

    expect(onIntervalChange).toHaveBeenCalledWith("60s");
    expect(onPauseToggle).toHaveBeenCalledTimes(1);
  });
});
