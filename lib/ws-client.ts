export type WSHandler = (data: unknown) => void;

export class WSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private token: string;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private handlers: Map<string, Set<WSHandler>> = new Map();
  private shouldReconnect = true;

  constructor(url: string, token: string) {
    this.url = url;
    this.token = token;
  }

  connect(): void {
    this.shouldReconnect = true;
    this.ws = new WebSocket(`${this.url}?token=${this.token}`);

    this.ws.onopen = () => {
      this.reconnectDelay = 1000;
      this.emit("connected", null);
    };

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data as string) as {
          type: string;
          [key: string]: unknown;
        };
        this.emit(msg.type, msg);
      } catch {
        // ignore malformed messages
      }
    };

    this.ws.onclose = () => {
      this.emit("disconnected", null);
      if (this.shouldReconnect) {
        setTimeout(() => this.connect(), this.reconnectDelay);
        this.reconnectDelay = Math.min(
          this.reconnectDelay * 2,
          this.maxReconnectDelay
        );
      }
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  subscribe(channel: string): void {
    this.send({ type: "subscribe", channel });
  }

  unsubscribe(channel: string): void {
    this.send({ type: "unsubscribe", channel });
  }

  on(eventType: string, handler: WSHandler): void {
    if (!this.handlers.has(eventType)) {
      this.handlers.set(eventType, new Set());
    }
    this.handlers.get(eventType)!.add(handler);
  }

  off(eventType: string, handler: WSHandler): void {
    this.handlers.get(eventType)?.delete(handler);
  }

  close(): void {
    this.shouldReconnect = false;
    this.ws?.close();
    this.ws = null;
  }

  private send(data: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  private emit(eventType: string, data: unknown): void {
    this.handlers.get(eventType)?.forEach((handler) => handler(data));
  }
}
