import WebSocket from "ws";
import type { AgentEvent } from "../types.js";

export class EventStreamer {
  private ws: WebSocket | null = null;
  private readonly url: string;
  private reconnectDelay = 1000;
  private readonly maxReconnectDelay = 30000;
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;

  constructor(goWsUrl: string) {
    this.url = goWsUrl;
  }

  connect(): void {
    if (this.closed) return;

    this.ws = new WebSocket(this.url);

    this.ws.on("open", () => {
      console.log("[EventStreamer] Connected to Go WS");
      this.reconnectDelay = 1000;
      this.startHeartbeat();
    });

    this.ws.on("close", () => {
      console.log("[EventStreamer] Connection closed");
      this.stopHeartbeat();
      if (!this.closed) {
        this.scheduleReconnect();
      }
    });

    this.ws.on("error", (err) => {
      console.error("[EventStreamer] Error:", err.message);
    });
  }

  send(event: AgentEvent): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(event));
    } else {
      console.warn(
        `[EventStreamer] Cannot send event (readyState=${this.ws?.readyState}), buffering not implemented`,
      );
    }
  }

  private scheduleReconnect(): void {
    this.stopHeartbeat();
    console.log(`[EventStreamer] Reconnecting in ${this.reconnectDelay}ms...`);
    this.reconnectTimer = setTimeout(() => this.connect(), this.reconnectDelay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
  }

  private startHeartbeat(): void {
    this.heartbeatInterval = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.ping();
      }
    }, 30000);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  close(): void {
    this.closed = true;
    this.stopHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
  }
}
