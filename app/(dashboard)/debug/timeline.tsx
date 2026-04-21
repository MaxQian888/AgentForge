"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { useDebugStore } from "@/lib/stores/debug-store";
import { useAuthStore } from "@/lib/stores/auth-store";

function resolveBaseUrl(): string {
  return process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";
}

function sourceColor(s: string): string {
  switch (s) {
    case "logs":
      return "text-blue-600";
    case "automation":
      return "text-purple-600";
    case "eventbus":
      return "text-green-600";
    default:
      return "";
  }
}

export function TimelineTab() {
  const [traceId, setTraceId] = useState("");
  const { entries, loading, error, truncated, fetchTrace } = useDebugStore();
  // accessToken is the real field name in this project's auth-store
  const accessToken = useAuthStore((s) => s.accessToken);

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <Input
          placeholder="tr_…"
          value={traceId}
          onChange={(e) => setTraceId(e.target.value)}
          className="font-mono"
        />
        <Button
          onClick={() =>
            fetchTrace(traceId, resolveBaseUrl(), accessToken ?? undefined)
          }
          disabled={!traceId || loading}
        >
          {loading ? "Loading…" : "Fetch"}
        </Button>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {truncated && (
        <p className="text-sm text-yellow-600">
          Truncated — more than 15,000 entries matched this trace.
        </p>
      )}
      <div className="space-y-1">
        {entries.length === 0 && !loading && !error && (
          <p className="text-sm text-muted-foreground">
            Enter a trace_id to see its merged timeline.
          </p>
        )}
        {entries.map((e, idx) => (
          <Card key={idx} className="p-2 font-mono text-xs flex gap-3 items-start">
            <span className="text-muted-foreground whitespace-nowrap">
              {new Date(e.timestamp).toISOString()}
            </span>
            <span className={`${sourceColor(e.source)} whitespace-nowrap`}>
              {e.source}
            </span>
            {e.level && (
              <span className="text-muted-foreground">{e.level}</span>
            )}
            {e.eventType && <span>{e.eventType}</span>}
            <span className="truncate flex-1">{e.summary}</span>
          </Card>
        ))}
      </div>
    </div>
  );
}
