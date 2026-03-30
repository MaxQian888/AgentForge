import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import mockImMessages from "@/messages/en/im.json";
import { IMChannelConfig } from "./im-channel-config";
import type { IMChannel } from "@/lib/stores/im-store";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) =>
    key.split(".").reduce((value: unknown, part: string) => {
      if (value && typeof value === "object" && part in (value as Record<string, unknown>)) {
        return (value as Record<string, unknown>)[part];
      }
      return key;
    }, mockImMessages),
}));

const saveChannel = jest.fn();
const deleteChannel = jest.fn();
const fetchEventTypes = jest.fn();

const storeState: {
  channels: IMChannel[];
  loading: boolean;
  eventTypes: string[];
  saveChannel: typeof saveChannel;
  deleteChannel: typeof deleteChannel;
  fetchEventTypes: typeof fetchEventTypes;
} = {
  channels: [],
  loading: false,
  eventTypes: [
    "task.created",
    "task.completed",
    "review.completed",
    "agent.started",
    "agent.completed",
    "budget.warning",
    "sprint.completed",
  ],
  saveChannel,
  deleteChannel,
  fetchEventTypes,
};

jest.mock("@/lib/stores/im-store", () => ({
  useIMStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

describe("IMChannelConfig", () => {
  function setFieldValue(label: string, value: string) {
    fireEvent.change(screen.getByLabelText(label), {
      target: { value },
    });
  }

  beforeEach(() => {
    storeState.channels = [];
    storeState.loading = false;
    storeState.eventTypes = [
      "task.created",
      "task.completed",
      "review.completed",
      "agent.started",
      "agent.completed",
      "budget.warning",
      "sprint.completed",
    ];
    saveChannel.mockReset().mockResolvedValue(undefined);
    deleteChannel.mockReset().mockResolvedValue(undefined);
    fetchEventTypes.mockReset().mockResolvedValue(undefined);
  });

  it("creates a qq bot channel with dynamic event subscriptions and platform config", async () => {
    const user = userEvent.setup();

    render(<IMChannelConfig />);

    expect(
      screen.getByText("No IM channels configured. Click New Channel to get started."),
    ).toBeInTheDocument();
    expect(fetchEventTypes).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "New Channel" }));
    await user.click(screen.getByRole("combobox", { name: "Platform" }));
    await user.click(screen.getByRole("option", { name: "QQ Bot" }));

    expect(screen.getByLabelText("App ID")).toBeInTheDocument();
    expect(screen.getByLabelText("App Secret")).toBeInTheDocument();
    expect(screen.queryByLabelText("Corp ID")).not.toBeInTheDocument();

    setFieldValue("Channel Name", "Ops Alerts");
    setFieldValue("Channel ID", "channel-1");
    setFieldValue("Webhook URL", "https://example.com/hook");
    setFieldValue("App ID", "qqbot-app-id");
    setFieldValue("App Secret", "qqbot-secret");
    await user.click(screen.getByLabelText("sprint.completed"));
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(saveChannel).toHaveBeenCalledWith({
        platform: "qqbot",
        name: "Ops Alerts",
        channelId: "channel-1",
        webhookUrl: "https://example.com/hook",
        platformConfig: {
          appId: "qqbot-app-id",
          appSecret: "qqbot-secret",
        },
        events: ["sprint.completed"],
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
        platformConfig: {},
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
