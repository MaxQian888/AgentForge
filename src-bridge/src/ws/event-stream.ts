import WebSocket from "ws";
import type { AgentEvent } from "../types.js";
import type { MCPServerStatus } from "../mcp/types.js";

/** Provider for heartbeat status data. */
export interface HeartbeatStatusProvider {
  getActiveAgentCount(): number;
  getMCPServerStatuses(): MCPServerStatus[];
}

export class EventStreamer {
  private ws: WebSocket | null = null;
  private readonly url: string;
  private reconnectDelay = 1000;
  private readonly maxReconnectDelay = 30000;
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;
  private statusProvider: HeartbeatStatusProvider | null = null;

  /** Ring buffer for events sent while disconnected (capacity 50). */
  private readonly buffer: AgentEvent[] = [];
  private readonly maxBuffer: number;

  constructor(goWsUrl: string, options?: { maxBuffer?: number }) {
    this.url = goWsUrl;
    this.maxBuffer = options?.maxBuffer ?? 50;
  }

  /** Set the provider for heartbeat status data. */
  setStatusProvider(provider: HeartbeatStatusProvider): void {
    this.statusProvider = provider;
  }

  connect(): void {
    if (this.closed) return;

    this.ws = new WebSocket(this.url);

    this.ws.on("open", () => {
      console.log("[EventStreamer] Connected to Go WS");
      this.reconnectDelay = 1000;
      this.flushBuffer();
      this.startHeartbeat();

      // Emit ready signal so Go knows TS Bridge is operational
      this.send({
        task_id: "__bridge__",
        session_id: "",
        timestamp_ms: Date.now(),
        type: "status_change",
        data: { status: "ready", new_status: "ready" },
      });
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
      this.flushBuffer();
      this.ws.send(JSON.stringify(event));
    } else {
      if (this.buffer.length >= this.maxBuffer) {
        this.buffer.shift();
      }
      this.buffer.push(event);
    }
  }

  private flushBuffer(): void {
    while (this.buffer.length > 0 && this.ws?.readyState === WebSocket.OPEN) {
      const event = this.buffer.shift()!;
      this.ws.send(JSON.stringify(event));
    }
  }

  /** Expose buffer length for testing. */
  get bufferedCount(): number {
    return this.buffer.length;
  }

  private scheduleReconnect(): void {
    this.stopHeartbeat();
    console.log(`[EventStreamer] Reconnecting in ${this.reconnectDelay}ms...`);
    this.reconnectTimer = setTimeout(() => this.connect(), this.reconnectDelay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
  }

  /**
   * Application-level heartbeat every 10s.
   * Sends a JSON message with bridge status and MCP server statuses.
   */
  private startHeartbeat(): void {
    this.heartbeatInterval = setInterval(() => {
      if (this.ws?.readyState !== WebSocket.OPEN) return;

      const heartbeat: AgentEvent = {
        task_id: "__heartbeat__",
        session_id: "",
        timestamp_ms: Date.now(),
        type: "heartbeat" as AgentEvent["type"],
        data: {
          bridge_status: "healthy",
          active_agents: this.statusProvider?.getActiveAgentCount() ?? 0,
          mcp_servers: this.statusProvider?.getMCPServerStatuses() ?? [],
        },
      };

      this.ws.send(JSON.stringify(heartbeat));
    }, 10000);
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
