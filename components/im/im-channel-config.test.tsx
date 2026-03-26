import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { IMChannelConfig } from "./im-channel-config";
import type { IMChannel } from "@/lib/stores/im-store";

const saveChannel = jest.fn();
const deleteChannel = jest.fn();

const storeState: {
  channels: IMChannel[];
  loading: boolean;
  saveChannel: typeof saveChannel;
  deleteChannel: typeof deleteChannel;
} = {
  channels: [],
  loading: false,
  saveChannel,
  deleteChannel,
};

jest.mock("@/lib/stores/im-store", () => ({
  useIMStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

describe("IMChannelConfig", () => {
  beforeEach(() => {
    storeState.channels = [];
    storeState.loading = false;
    saveChannel.mockReset().mockResolvedValue(undefined);
    deleteChannel.mockReset().mockResolvedValue(undefined);
  });

  it("creates a new channel with event subscriptions", async () => {
    const user = userEvent.setup();

    render(<IMChannelConfig />);

    expect(
      screen.getByText("No IM channels configured. Click New Channel to get started."),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "New Channel" }));
    await user.type(screen.getByLabelText("Channel Name"), "Ops Alerts");
    await user.type(screen.getByLabelText("Channel ID"), "channel-1");
    await user.type(screen.getByLabelText("Webhook URL"), "https://example.com/hook");
    await user.click(screen.getByLabelText("task.completed"));
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(saveChannel).toHaveBeenCalledWith({
        platform: "feishu",
        name: "Ops Alerts",
        channelId: "channel-1",
        webhookUrl: "https://example.com/hook",
        events: ["task.completed"],
        active: true,
      });
    });
  });

  it("clears the editing form after deleting the selected channel", async () => {
    const user = userEvent.setup();

    storeState.channels = [
      {
        id: "channel-1",
        platform: "slack",
        name: "Build Alerts",
        channelId: "C123",
        webhookUrl: "https://hooks.slack.com/services/test",
        events: ["task.created"],
        active: true,
      },
    ];

    render(<IMChannelConfig />);

    await user.click(screen.getByText("Build Alerts"));
    expect(screen.getByText("Edit Channel")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(deleteChannel).toHaveBeenCalledWith("channel-1");
      expect(screen.queryByText("Edit Channel")).not.toBeInTheDocument();
    });
  });
});
