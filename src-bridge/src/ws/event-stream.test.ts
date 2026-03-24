import { afterEach, describe, expect, test } from "bun:test";
import { EventStreamer } from "./event-stream.js";

type TestSocket = {
  readyState: number;
  send?: (payload: string) => void;
  ping?: () => void;
  close?: () => void;
};

type TestableEventStreamer = {
  ws: TestSocket | null;
  reconnectDelay: number;
  reconnectTimer: ReturnType<typeof setTimeout> | null;
  connect: () => void;
  send: EventStreamer["send"];
  close: () => void;
  startHeartbeat: () => void;
  scheduleReconnect: () => void;
};

const originalConsoleWarn = console.warn;
const originalConsoleLog = console.log;
const originalSetTimeout = globalThis.setTimeout;
const originalClearTimeout = globalThis.clearTimeout;
const originalSetInterval = globalThis.setInterval;
const originalClearInterval = globalThis.clearInterval;

afterEach(() => {
  console.warn = originalConsoleWarn;
  console.log = originalConsoleLog;
  globalThis.setTimeout = originalSetTimeout;
  globalThis.clearTimeout = originalClearTimeout;
  globalThis.setInterval = originalSetInterval;
  globalThis.clearInterval = originalClearInterval;
});

describe("EventStreamer", () => {
  test("sends websocket payloads only when the socket is open", () => {
    const streamer = new EventStreamer(
      "ws://localhost:7777/ws/bridge",
    ) as unknown as TestableEventStreamer;
    const sent: string[] = [];
    const warnings: string[] = [];

    console.warn = (...args: unknown[]) => {
      warnings.push(args.join(" "));
    };

    streamer.ws = {
      readyState: 1,
      send(payload: string) {
        sent.push(payload);
      },
    };

    streamer.send({
      task_id: "task-123",
      session_id: "session-123",
      timestamp_ms: 123,
      type: "output",
      data: { content: "hello" },
    });

    streamer.ws.readyState = 0;
    streamer.send({
      task_id: "task-123",
      session_id: "session-123",
      timestamp_ms: 124,
      type: "output",
      data: { content: "buffer me" },
    });

    expect(sent).toEqual([
      JSON.stringify({
        task_id: "task-123",
        session_id: "session-123",
        timestamp_ms: 123,
        type: "output",
        data: { content: "hello" },
      }),
    ]);
    expect(warnings[0]).toContain("Cannot send event");
  });

  test("starts heartbeats, schedules reconnects with backoff, and closes cleanly", () => {
    const streamer = new EventStreamer(
      "ws://localhost:7777/ws/bridge",
    ) as unknown as TestableEventStreamer;
    let intervalCleared = false;
    let timeoutCleared = false;
    let reconnectDelay: number | undefined;
    let reconnectInvoked = false;
    let pingCount = 0;
    let closeCount = 0;

    console.log = () => {};
    globalThis.setInterval = (((fn: TimerHandler) => {
      if (typeof fn === "function") {
        fn();
      }
      return 111 as unknown as ReturnType<typeof setInterval>;
    }) as unknown) as typeof setInterval;
    globalThis.clearInterval = (() => {
      intervalCleared = true;
    }) as typeof clearInterval;
    globalThis.setTimeout = (((fn: TimerHandler, delay?: number) => {
      reconnectDelay = Number(delay);
      if (typeof fn === "function") {
        fn();
      }
      return 222 as unknown as ReturnType<typeof setTimeout>;
    }) as unknown) as typeof setTimeout;
    globalThis.clearTimeout = (() => {
      timeoutCleared = true;
    }) as typeof clearTimeout;

    streamer.connect = () => {
      reconnectInvoked = true;
    };
    streamer.ws = {
      readyState: 1,
      ping() {
        pingCount += 1;
      },
      close() {
        closeCount += 1;
      },
    };

    streamer.startHeartbeat();
    streamer.scheduleReconnect();
    streamer.reconnectTimer = 222 as unknown as ReturnType<typeof setTimeout>;
    streamer.close();

    expect(pingCount).toBe(1);
    expect(reconnectDelay).toBe(1000);
    expect(reconnectInvoked).toBe(true);
    expect(streamer.reconnectDelay).toBe(2000);
    expect(intervalCleared).toBe(true);
    expect(timeoutCleared).toBe(true);
    expect(closeCount).toBe(1);
  });
});
