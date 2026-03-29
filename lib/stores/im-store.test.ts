jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("./auth-store", () => ({
  useAuthStore: {
    getState: jest.fn(),
  },
}));

import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "./auth-store";
import {
  useIMStore,
  type IMBridgeStatus,
  type IMChannel,
  type IMDelivery,
} from "./im-store";

type MockIMApiClient = {
  get: jest.Mock;
  post: jest.Mock;
  put: jest.Mock;
  delete: jest.Mock;
};

const DEFAULT_BRIDGE_STATUS: IMBridgeStatus = {
  registered: false,
  lastHeartbeat: null,
  providers: [],
  providerDetails: [],
  health: "disconnected",
};

function makeApiClient(): MockIMApiClient {
  return {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn(),
  };
}

function makeChannel(overrides: Partial<IMChannel> = {}): IMChannel {
  return {
    id: "channel-1",
    platform: "feishu",
    name: "Alerts",
    channelId: "chat-1",
    webhookUrl: "https://example.com/webhook",
    platformConfig: {},
    events: ["workflow.completed"],
    active: true,
    ...overrides,
  };
}

function makeDelivery(overrides: Partial<IMDelivery> = {}): IMDelivery {
  return {
    id: "delivery-1",
    channelId: "channel-1",
    platform: "feishu",
    eventType: "workflow.completed",
    status: "delivered",
    createdAt: "2026-03-26T08:00:00.000Z",
    ...overrides,
  };
}

describe("useIMStore", () => {
  const mockCreateApiClient = createApiClient as jest.Mock;
  const mockGetAuthState = useAuthStore.getState as unknown as jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mockGetAuthState.mockReturnValue({ accessToken: "test-token" });
    useIMStore.setState({
      channels: [],
      bridgeStatus: DEFAULT_BRIDGE_STATUS,
      deliveries: [],
      loading: false,
      error: null,
    });
  });

  it("returns early when fetching channels without a token", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useIMStore.getState().fetchChannels();

    expect(mockCreateApiClient).not.toHaveBeenCalled();
    expect(useIMStore.getState()).toMatchObject({
      channels: [],
      loading: false,
      error: null,
    });
  });

  it("fetches IM channels and stores the response", async () => {
    const api = makeApiClient();
    const channels = [makeChannel()];
    api.get.mockResolvedValueOnce({ data: channels });
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().fetchChannels();

    expect(mockCreateApiClient).toHaveBeenCalledWith("http://localhost:7777");
    expect(api.get).toHaveBeenCalledWith("/api/v1/im/channels", {
      token: "test-token",
    });
    expect(useIMStore.getState()).toMatchObject({
      channels,
      loading: false,
      error: null,
    });
  });

  it("surfaces a channel load error when fetching channels fails", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(new Error("boom"));
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().fetchChannels();

    expect(useIMStore.getState()).toMatchObject({
      channels: [],
      loading: false,
      error: "Unable to load IM channels",
    });
  });

  it("returns early when fetching bridge status without a token", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useIMStore.getState().fetchBridgeStatus();

    expect(mockCreateApiClient).not.toHaveBeenCalled();
  });

  it("stores the latest bridge status", async () => {
    const api = makeApiClient();
    const bridgeStatus: IMBridgeStatus = {
      registered: true,
      lastHeartbeat: "2026-03-26T08:30:00.000Z",
      providers: ["feishu", "telegram"],
      providerDetails: [],
      health: "healthy",
    };
    api.get.mockResolvedValueOnce({ data: bridgeStatus });
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().fetchBridgeStatus();

    expect(api.get).toHaveBeenCalledWith("/api/v1/im/bridge/status", {
      token: "test-token",
    });
    expect(useIMStore.getState()).toMatchObject({
      bridgeStatus,
      error: null,
    });
  });

  it("falls back to the disconnected bridge state when bridge status fails", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(new Error("bridge offline"));
    mockCreateApiClient.mockReturnValue(api);
    useIMStore.setState({
      bridgeStatus: {
        registered: true,
        lastHeartbeat: "2026-03-26T08:30:00.000Z",
        providers: ["feishu"],
        providerDetails: [],
        health: "healthy",
      },
      error: "stale",
    });

    await useIMStore.getState().fetchBridgeStatus();

    expect(useIMStore.getState()).toMatchObject({
      bridgeStatus: DEFAULT_BRIDGE_STATUS,
      error: null,
    });
  });

  it("returns early when fetching delivery history without a token", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useIMStore.getState().fetchDeliveryHistory();

    expect(mockCreateApiClient).not.toHaveBeenCalled();
  });

  it("fetches delivery history and stores the response", async () => {
    const api = makeApiClient();
    const deliveries = [makeDelivery()];
    api.get.mockResolvedValueOnce({ data: deliveries });
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().fetchDeliveryHistory();

    expect(api.get).toHaveBeenCalledWith("/api/v1/im/deliveries", {
      token: "test-token",
    });
    expect(useIMStore.getState()).toMatchObject({
      deliveries,
      loading: false,
      error: null,
    });
  });

  it("surfaces a delivery history error when the request fails", async () => {
    const api = makeApiClient();
    api.get.mockRejectedValueOnce(new Error("timeout"));
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().fetchDeliveryHistory();

    expect(useIMStore.getState()).toMatchObject({
      deliveries: [],
      loading: false,
      error: "Unable to load delivery history",
    });
  });

  it("fetches dynamic IM event types", async () => {
    const api = makeApiClient();
    api.get.mockResolvedValueOnce({
      data: ["task.created", "review.requested", "workflow.failed"],
    });
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().fetchEventTypes();

    expect(api.get).toHaveBeenCalledWith("/api/v1/im/event-types", {
      token: "test-token",
    });
    expect(useIMStore.getState()).toMatchObject({
      eventTypes: ["task.created", "review.requested", "workflow.failed"],
      error: null,
    });
  });

  it("retries a failed delivery and refreshes history", async () => {
    const api = makeApiClient();
    api.post.mockResolvedValueOnce({ data: { id: "delivery-1" } });
    api.get.mockResolvedValueOnce({
      data: [makeDelivery({ id: "delivery-1", status: "pending" })],
    });
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().retryDelivery("delivery-1");

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/im/deliveries/delivery-1/retry",
      {},
      { token: "test-token" },
    );
    expect(api.get).toHaveBeenCalledWith("/api/v1/im/deliveries", {
      token: "test-token",
    });
    expect(useIMStore.getState()).toMatchObject({
      deliveries: [makeDelivery({ id: "delivery-1", status: "pending" })],
      loading: false,
      error: null,
    });
  });

  it("returns early when saving a channel without a token", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useIMStore.getState().saveChannel({
      platform: "feishu",
      name: "Alerts",
      channelId: "chat-1",
      webhookUrl: "https://example.com/webhook",
      platformConfig: {},
      events: ["workflow.completed"],
      active: true,
    });

    expect(mockCreateApiClient).not.toHaveBeenCalled();
  });

  it("creates a new channel and refreshes the store", async () => {
    const api = makeApiClient();
    const savedChannel = makeChannel();
    api.post.mockResolvedValueOnce({ data: null });
    api.get.mockResolvedValueOnce({ data: [savedChannel] });
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().saveChannel({
      platform: "feishu",
      name: "Alerts",
      channelId: "chat-1",
      webhookUrl: "https://example.com/webhook",
      platformConfig: {},
      events: ["workflow.completed"],
      active: true,
    });

    expect(api.post).toHaveBeenCalledWith(
      "/api/v1/im/channels",
      {
        platform: "feishu",
        name: "Alerts",
        channelId: "chat-1",
        webhookUrl: "https://example.com/webhook",
        platformConfig: {},
        events: ["workflow.completed"],
        active: true,
      },
      { token: "test-token" },
    );
    expect(api.get).toHaveBeenCalledWith("/api/v1/im/channels", {
      token: "test-token",
    });
    expect(useIMStore.getState()).toMatchObject({
      channels: [savedChannel],
      loading: false,
      error: null,
    });
  });

  it("updates an existing channel and refreshes the store", async () => {
    const api = makeApiClient();
    const savedChannel = makeChannel({ name: "Ops Alerts" });
    api.put.mockResolvedValueOnce({ data: null });
    api.get.mockResolvedValueOnce({ data: [savedChannel] });
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().saveChannel(savedChannel);

    expect(api.put).toHaveBeenCalledWith(
      "/api/v1/im/channels/channel-1",
      savedChannel,
      { token: "test-token" },
    );
    expect(useIMStore.getState()).toMatchObject({
      channels: [savedChannel],
      loading: false,
      error: null,
    });
  });

  it("surfaces a save error when persisting a channel fails", async () => {
    const api = makeApiClient();
    api.post.mockRejectedValueOnce(new Error("persist failed"));
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().saveChannel({
      platform: "feishu",
      name: "Alerts",
      channelId: "chat-1",
      webhookUrl: "https://example.com/webhook",
      platformConfig: {},
      events: ["workflow.completed"],
      active: true,
    });

    expect(useIMStore.getState()).toMatchObject({
      loading: false,
      error: "Failed to save channel",
    });
  });

  it("returns early when deleting a channel without a token", async () => {
    mockGetAuthState.mockReturnValueOnce({ accessToken: null });

    await useIMStore.getState().deleteChannel("channel-1");

    expect(mockCreateApiClient).not.toHaveBeenCalled();
  });

  it("deletes a channel and refreshes the store", async () => {
    const api = makeApiClient();
    api.delete.mockResolvedValueOnce({ data: null });
    api.get.mockResolvedValueOnce({ data: [] });
    mockCreateApiClient.mockReturnValue(api);
    useIMStore.setState({ channels: [makeChannel()] });

    await useIMStore.getState().deleteChannel("channel-1");

    expect(api.delete).toHaveBeenCalledWith("/api/v1/im/channels/channel-1", {
      token: "test-token",
    });
    expect(api.get).toHaveBeenCalledWith("/api/v1/im/channels", {
      token: "test-token",
    });
    expect(useIMStore.getState()).toMatchObject({
      channels: [],
      loading: false,
      error: null,
    });
  });

  it("surfaces a delete error when removing a channel fails", async () => {
    const api = makeApiClient();
    api.delete.mockRejectedValueOnce(new Error("delete failed"));
    mockCreateApiClient.mockReturnValue(api);

    await useIMStore.getState().deleteChannel("channel-1");

    expect(useIMStore.getState()).toMatchObject({
      loading: false,
      error: "Failed to delete channel",
    });
  });
});
