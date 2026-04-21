"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useAuthStore } from "@/lib/stores/auth-store";

const MAX = 500;

interface EventMsg {
  type?: string;
  source?: string;
  target?: string;
  timestamp?: number;
  metadata?: Record<string, unknown>;
  payload?: unknown;
  [key: string]: unknown;
}

function resolveWsBaseUrl(): string {
  const base = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";
  return base.replace(/^http/, "ws").replace(/\/$/, "");
}

function eventKey(e: EventMsg, idx: number): string {
  return `${e.timestamp ?? 0}-${idx}`;
}

export function LiveTailTab() {
  const [events, setEvents] = useState<EventMsg[]>([]);
  const [paused, setPaused] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const bufferRef = useRef<EventMsg[]>([]);
  const pausedRef = useRef(paused);
  const accessToken = useAuthStore((s) => s.accessToken);

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

  useEffect(() => {
    if (!accessToken) return;

    const url = `${resolveWsBaseUrl()}/ws?token=${encodeURIComponent(accessToken)}`;
    const ws = new WebSocket(url);

    ws.onopen = () => {
      setConnected(true);
      setError(null);
    };
    ws.onclose = () => setConnected(false);
    ws.onerror = () => {
      setError("WebSocket connection failed");
    };
    ws.onmessage = (e) => {
      if (pausedRef.current) return;
      try {
        const msg = JSON.parse(e.data as string) as EventMsg;
        bufferRef.current.push(msg);
        if (bufferRef.current.length > MAX) {
          bufferRef.current = bufferRef.current.slice(-MAX);
        }
        setEvents([...bufferRef.current]);
      } catch {
        // drop non-JSON frames
      }
    };

    return () => {
      ws.close();
    };
  }, [accessToken]);

  return (
    <div className="flex flex-col gap-[var(--space-section-gap)]">
      <div className="flex items-center gap-[var(--space-stack-sm)]">
        <Button
          variant={paused ? "default" : "outline"}
          onClick={() => setPaused((p) => !p)}
        >
          {paused ? "Resume" : "Pause"}
        </Button>
        <span className="text-sm text-muted-foreground">
          {connected ? "connected" : "disconnected"} — {events.length} events
          (cap {MAX})
        </span>
        {error && <span className="text-sm text-destructive">{error}</span>}
      </div>
      <div className="flex flex-col gap-[var(--space-stack-xs)] max-h-[70vh] overflow-y-auto">
        {events.length === 0 && (
          <p className="text-sm text-muted-foreground">
            {connected
              ? "Waiting for events…"
              : "Connect to a running backend to see live events."}
          </p>
        )}
        {events
          .slice()
          .reverse()
          .map((ev, idx) => (
            <Card
              key={eventKey(ev, idx)}
              className="flex gap-[var(--space-stack-sm)] p-[var(--space-card-padding)] font-mono text-xs"
            >
              <span className="text-muted-foreground whitespace-nowrap">
                {ev.timestamp ? new Date(ev.timestamp).toISOString() : ""}
              </span>
              <span className="text-green-600 whitespace-nowrap">
                {ev.type}
              </span>
              {ev.metadata?.trace_id ? (
                <span className="text-blue-600 whitespace-nowrap truncate">
                  {String(ev.metadata.trace_id)}
                </span>
              ) : null}
              <span className="truncate flex-1">
                {ev.source}
                {ev.target ? ` → ${ev.target}` : ""}
              </span>
            </Card>
          ))}
      </div>
    </div>
  );
}
