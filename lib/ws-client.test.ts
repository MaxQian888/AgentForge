/** @jest-environment jsdom */

import { WSClient } from "./ws-client";

class MockWebSocket {
  static OPEN = 1;
  static CLOSED = 3;
  static instances: MockWebSocket[] = [];

  readyState = 0;
  sentMessages: string[] = [];
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(public readonly url: string) {
    MockWebSocket.instances.push(this);
  }

  send(message: string) {
    this.sentMessages.push(message);
  }

  close() {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(new Event("close") as CloseEvent);
  }
}

describe("WSClient", () => {
  beforeEach(() => {
    jest.useFakeTimers();
    MockWebSocket.instances = [];
    global.WebSocket = MockWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("connects with the token query string and emits a connected event", () => {
    const client = new WSClient("ws://localhost:7777/ws", "token-123");
    const onConnected = jest.fn();
    client.on("connected", onConnected);

    client.connect();

    expect(MockWebSocket.instances[0]?.url).toBe(
      "ws://localhost:7777/ws?token=token-123",
    );

    MockWebSocket.instances[0]!.onopen?.(new Event("open"));

    expect(onConnected).toHaveBeenCalledWith(null);
  });

  it("emits parsed message envelopes and ignores malformed payloads", () => {
    const client = new WSClient("ws://localhost:7777/ws", "token-123");
    const onTaskUpdate = jest.fn();
    client.on("task.updated", onTaskUpdate);

    client.connect();

    MockWebSocket.instances[0]!.onmessage?.({
      data: JSON.stringify({ type: "task.updated", payload: { id: "task-1" } }),
    } as MessageEvent);
    MockWebSocket.instances[0]!.onmessage?.({
      data: "{not-json",
    } as MessageEvent);

    expect(onTaskUpdate).toHaveBeenCalledTimes(1);
    expect(onTaskUpdate).toHaveBeenCalledWith({
      type: "task.updated",
      payload: { id: "task-1" },
    });
  });

  it("sends subscribe and unsubscribe commands only when the socket is open", () => {
    const client = new WSClient("ws://localhost:7777/ws", "token-123");
    client.connect();

    client.subscribe("project-1");

    const socket = MockWebSocket.instances[0]!;
    socket.readyState = MockWebSocket.OPEN;

    client.subscribe("project-1");
    client.unsubscribe("project-1");

    expect(socket.sentMessages).toEqual([
      JSON.stringify({ type: "subscribe", channel: "project-1" }),
      JSON.stringify({ type: "unsubscribe", channel: "project-1" }),
    ]);
  });

  it("removes handlers after off is called", () => {
    const client = new WSClient("ws://localhost:7777/ws", "token-123");
    const onDisconnected = jest.fn();
    client.on("disconnected", onDisconnected);
    client.off("disconnected", onDisconnected);

    client.connect();
    MockWebSocket.instances[0]!.onclose?.(new Event("close") as CloseEvent);

    expect(onDisconnected).not.toHaveBeenCalled();
  });

  it("reconnects with exponential backoff after an unexpected close", () => {
    const client = new WSClient("ws://localhost:7777/ws", "token-123");
    const onDisconnected = jest.fn();
    client.on("disconnected", onDisconnected);

    client.connect();
    MockWebSocket.instances[0]!.onclose?.(new Event("close") as CloseEvent);

    expect(onDisconnected).toHaveBeenCalledWith(null);
    expect(MockWebSocket.instances).toHaveLength(1);

    jest.advanceTimersByTime(1000);
    expect(MockWebSocket.instances).toHaveLength(2);

    MockWebSocket.instances[1]!.onclose?.(new Event("close") as CloseEvent);
    jest.advanceTimersByTime(2000);
    expect(MockWebSocket.instances).toHaveLength(3);
  });

  it("closes the socket without scheduling a reconnect when closed intentionally", () => {
    const client = new WSClient("ws://localhost:7777/ws", "token-123");
    client.connect();

    client.close();
    jest.runOnlyPendingTimers();

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0]!.readyState).toBe(MockWebSocket.CLOSED);
  });

  it("closes the underlying socket when an error occurs", () => {
    const client = new WSClient("ws://localhost:7777/ws", "token-123");
    client.connect();

    const socket = MockWebSocket.instances[0]!;
    const closeSpy = jest.spyOn(socket, "close");

    socket.onerror?.(new Event("error"));

    expect(closeSpy).toHaveBeenCalledTimes(1);
  });
});
