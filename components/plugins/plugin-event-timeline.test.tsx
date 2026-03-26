import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginEventTimeline } from "./plugin-event-timeline";
import type { PluginEventRecord } from "@/lib/stores/plugin-store";

const fetchEvents = jest.fn();

const storeState: {
  events: Record<string, PluginEventRecord[]>;
  fetchEvents: typeof fetchEvents;
} = {
  events: {},
  fetchEvents,
};

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

describe("PluginEventTimeline", () => {
  beforeEach(() => {
    storeState.events = {};
    fetchEvents.mockReset().mockResolvedValue(undefined);
  });

  it("loads events on mount and shows an empty state when none are available", async () => {
    render(<PluginEventTimeline pluginId="repo-search" />);

    await waitFor(() => {
      expect(fetchEvents).toHaveBeenCalledWith("repo-search", 50);
    });
    expect(
      screen.getByText("No events recorded for this plugin yet."),
    ).toBeInTheDocument();
  });

  it("sorts events, expands payload details, and refreshes when the limit changes", async () => {
    const user = userEvent.setup();

    storeState.events = {
      "repo-search": [
        {
          id: "event-older",
          plugin_id: "repo-search",
          event_type: "enabled",
          event_source: "operator",
          summary: "Earlier event",
          created_at: "2026-03-26T00:00:00.000Z",
        },
        {
          id: "event-newer",
          plugin_id: "repo-search",
          event_type: "failed",
          event_source: "ts-bridge",
          summary: "Later event",
          payload: { detail: "retrying" },
          created_at: "2026-03-26T02:00:00.000Z",
        },
      ],
    };

    render(<PluginEventTimeline pluginId="repo-search" />);

    const summaries = screen.getAllByText(/Earlier event|Later event/);
    expect(summaries[0]).toHaveTextContent("Later event");

    await user.click(screen.getByText("Later event"));
    expect(screen.getByText(/"detail": "retrying"/)).toBeInTheDocument();

    await user.selectOptions(screen.getByRole("combobox"), "100");
    expect(fetchEvents).toHaveBeenCalledWith("repo-search", 100);
  });
});
