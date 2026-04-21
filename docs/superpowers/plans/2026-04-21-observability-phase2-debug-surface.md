# Observability Phase 2 — `/debug` Developer Surface

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans.

**Goal:** MVP of the `/debug` dashboard (spec §5) — Timeline view joining `logs` + `automation_logs` + persisted EventBus events by `trace_id`, Live Tail of the eventbus via WebSocket, and a `/metrics` endpoint for Prometheus exposition. Admin-gated via existing RBAC.

**Architecture:** A thin aggregation layer on top of Phase 1's data. One new handler aggregates rows from three existing repositories keyed by `detail->>'trace_id'` / `metadata->>'trace_id'`. One new Next.js page consumes that API and subscribes to the existing WebSocket hub.

**Tech Stack:** Go + Echo (`internal/handler`), Postgres JSONB queries, Next.js client-rendered page, existing `ws.Hub`.

**Source of truth:** `docs/superpowers/specs/2026-04-21-observability-system-design.md` §5.

---

## Prerequisites

Phase 1 complete — `trace_id` flows through `logs.detail`, `automation_logs.detail`, and the persisted events repo's `metadata`. Phase 3's `LOG_LEVEL`, pprof, panic-stack, and per-entry trace stamping merged.

---

## File Structure

Files created:
- `src-go/internal/handler/debug_handler.go` — `GET /api/v1/debug/trace/:trace_id` + `GET /metrics`
- `src-go/internal/handler/debug_handler_test.go`
- `app/(dashboard)/debug/page.tsx` — page shell + tab container
- `app/(dashboard)/debug/timeline.tsx` — Timeline tab (client component)
- `app/(dashboard)/debug/live-tail.tsx` — Live Tail tab (client component)
- `lib/stores/debug-store.ts` — Zustand store for timeline + live-tail state

Files modified:
- `src-go/internal/repository/log_repo.go` — `ListByTraceID`
- `src-go/internal/repository/automation_log_repo.go` — `ListByTraceID`
- `src-go/internal/repository/events_repo.go` (or wherever persisted events live) — `ListByTraceID`
- `src-go/internal/server/routes.go` — mount debug routes + /metrics
- `components/app-sidebar.tsx` (or equivalent nav) — add a "Debug" link visible only to admins

---

## Task 1: Repository ListByTraceID methods

**Files:**
- Modify: `src-go/internal/repository/log_repo.go`
- Modify: `src-go/internal/repository/automation_log_repo.go`
- Modify: the persisted-events repository (discover via `grep -rn "NewPersistFromRepos\|eventsRepo" src-go/cmd src-go/internal`)

All three need a new method that queries `WHERE <jsonb-col>->>'trace_id' = $1 ORDER BY <timestamp-col> ASC LIMIT 10000`.

**Discovery step first:** find the persisted-events repo name and file, and the exact JSONB column name on each table:

```bash
cd src-go
grep -rn "NewPersistFromRepos\|eventsRepo" cmd/ internal/ 2>&1 | head -10
grep -rn "CREATE TABLE.*events\|CREATE TABLE logs\|CREATE TABLE automation_logs" migrations/ 2>&1 | head -10
```

- [ ] **Step 1: Write failing test for logs repo**

```go
// src-go/internal/repository/log_repo_trace_test.go
package repository_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
)

func TestLogRepo_ListByTraceID(t *testing.T) {
    // Use the existing test DB harness; search for a similar test pattern in
    // log_repo_integration_test.go or similar. If no harness exists, skip the DB
    // portion and unit-test the SQL string via a mock Queryer.
    //
    // Expected behavior: given two logs with detail.trace_id = "tr_A" and one with
    // "tr_B", ListByTraceID(ctx, "tr_A", 100) returns exactly 2 ordered by created_at.
}
```

If the package uses an integration test harness, use it. Otherwise make this test a placeholder and test via the handler-level integration test in Task 2. Don't block on this if DB plumbing is heavy.

- [ ] **Step 2: Implement `ListByTraceID` on LogRepository**

```go
// Append to src-go/internal/repository/log_repo.go
func (r *LogRepository) ListByTraceID(ctx context.Context, traceID string, limit int) ([]*model.Log, error) {
    if limit <= 0 { limit = 10000 }
    const q = `
        SELECT id, project_id, tab, level, actor_type, actor_id, agent_id, session_id,
               event_type, action, resource_type, resource_id, summary, detail, created_at
        FROM logs
        WHERE detail->>'trace_id' = $1
        ORDER BY created_at ASC
        LIMIT $2
    `
    rows, err := r.db.QueryContext(ctx, q, traceID, limit)
    if err != nil { return nil, fmt.Errorf("list logs by trace: %w", err) }
    defer rows.Close()

    var out []*model.Log
    for rows.Next() {
        var l model.Log
        if err := rows.Scan(&l.ID, &l.ProjectID, &l.Tab, &l.Level, &l.ActorType, &l.ActorID,
            &l.AgentID, &l.SessionID, &l.EventType, &l.Action, &l.ResourceType, &l.ResourceID,
            &l.Summary, &l.Detail, &l.CreatedAt); err != nil {
            return nil, err
        }
        out = append(out, &l)
    }
    return out, rows.Err()
}
```

Match the existing `LogRepository` struct's DB field name (likely `r.db` of type `*sql.DB` or similar; follow the convention of `log_repo.go`'s existing `Create`).

- [ ] **Step 3: Implement `ListByTraceID` on AutomationLogRepository**

Same pattern. The `automation_logs` table's timestamp column is likely `triggered_at` — verify by reading `model/automation.go`. Adjust SQL accordingly.

- [ ] **Step 4: Implement `ListByTraceID` on the persisted-events repo**

Key difference: trace_id lives in `metadata->>'trace_id'`, NOT `detail->>'trace_id'`. Use the actual column name (check the migration).

- [ ] **Step 5: Build**

```bash
cd src-go && go build ./... && go test ./internal/repository/... -count=1 -short 2>&1 | tail -20
```

Clean. If integration tests require a live DB and fail in this environment, skip them; the handler integration test in Task 2 will cover via test fixtures.

- [ ] **Step 6: Commit**

```bash
git add src-go/internal/repository/
git commit -m "feat(obs): ListByTraceID on logs, automation_logs, events repos"
```

---

## Task 2: Debug handler + `/metrics` endpoint

**Files:**
- Create: `src-go/internal/handler/debug_handler.go`
- Create: `src-go/internal/handler/debug_handler_test.go`

The handler exposes two routes:
1. `GET /api/v1/debug/trace/:trace_id` — merges logs + automation_logs + events, sorts by timestamp, returns up to 10k rows with `truncated: true` if hit
2. `GET /metrics` — Prometheus exposition via `promhttp.Handler()`

- [ ] **Step 1: Add the handler struct + interface narrowing**

```go
// src-go/internal/handler/debug_handler.go
package handler

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/agentforge/server/internal/model"
)

// LogTraceQuery is the narrow slice of LogRepository used by the debug handler.
type LogTraceQuery interface {
    ListByTraceID(ctx context.Context, traceID string, limit int) ([]*model.Log, error)
}

// AutomationLogTraceQuery is the narrow slice of AutomationLogRepository.
type AutomationLogTraceQuery interface {
    ListByTraceID(ctx context.Context, traceID string, limit int) ([]*model.AutomationLog, error)
}

// EventTraceQuery is the narrow slice of the persisted-events repo.
type EventTraceQuery interface {
    ListByTraceID(ctx context.Context, traceID string, limit int) ([]*model.Event, error)
}

type DebugHandler struct {
    logs        LogTraceQuery
    automation  AutomationLogTraceQuery
    events      EventTraceQuery
}

func NewDebugHandler(logs LogTraceQuery, automation AutomationLogTraceQuery, events EventTraceQuery) *DebugHandler {
    return &DebugHandler{logs: logs, automation: automation, events: events}
}

type timelineEntry struct {
    Timestamp time.Time              `json:"timestamp"`
    Source    string                 `json:"source"` // "logs" | "automation" | "eventbus"
    Level     string                 `json:"level,omitempty"`
    EventType string                 `json:"eventType,omitempty"`
    Summary   string                 `json:"summary,omitempty"`
    Detail    map[string]interface{} `json:"detail,omitempty"`
}

type timelineResponse struct {
    TraceID   string          `json:"traceId"`
    Entries   []timelineEntry `json:"entries"`
    Truncated bool            `json:"truncated"`
}

func (h *DebugHandler) GetTrace(c echo.Context) error {
    traceID := c.Param("trace_id")
    if traceID == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "trace_id required"})
    }
    const perRepoLimit = 5000
    ctx := c.Request().Context()

    logs, err := h.logs.ListByTraceID(ctx, traceID, perRepoLimit)
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query logs failed"}) }
    auto, err := h.automation.ListByTraceID(ctx, traceID, perRepoLimit)
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query automation_logs failed"}) }
    evs, err := h.events.ListByTraceID(ctx, traceID, perRepoLimit)
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query events failed"}) }

    entries := make([]timelineEntry, 0, len(logs)+len(auto)+len(evs))
    for _, l := range logs {
        entries = append(entries, timelineEntry{
            Timestamp: l.CreatedAt, Source: "logs",
            Level: l.Level, EventType: l.EventType, Summary: l.Summary,
            Detail: parseDetailJSON(l.Detail),
        })
    }
    for _, a := range auto {
        entries = append(entries, timelineEntry{
            Timestamp: a.TriggeredAt, Source: "automation",
            EventType: a.EventType, Summary: a.Status,
            Detail: parseDetailJSON(a.Detail),
        })
    }
    for _, e := range evs {
        entries = append(entries, timelineEntry{
            Timestamp: time.Unix(0, e.Timestamp*int64(time.Millisecond)), // convert ms → time
            Source: "eventbus", EventType: e.Type,
            Detail: mergeEventPayload(e),
        })
    }
    sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp.Before(entries[j].Timestamp) })

    total := len(logs) + len(auto) + len(evs)
    return c.JSON(http.StatusOK, timelineResponse{
        TraceID: traceID, Entries: entries,
        Truncated: total >= perRepoLimit*3,
    })
}

// MetricsHandler is a convenience for mounting Prometheus exposition under Echo.
func MetricsHandler() echo.HandlerFunc {
    return echo.WrapHandler(promhttp.Handler())
}

func parseDetailJSON(raw []byte) map[string]interface{} {
    if len(raw) == 0 { return nil }
    var m map[string]interface{}
    _ = json.Unmarshal(raw, &m)
    return m
}

func mergeEventPayload(e *model.Event) map[string]interface{} {
    out := map[string]interface{}{}
    if e.Metadata != nil {
        for k, v := range e.Metadata {
            out[k] = v
        }
    }
    if len(e.Payload) > 0 {
        var p map[string]interface{}
        if json.Unmarshal(e.Payload, &p) == nil {
            out["payload"] = p
        }
    }
    return out
}
```

Adjust field names to match the actual `model.Event` struct:
- `Event.Timestamp` is an `int64` (milliseconds? or seconds? check)
- `Event.Metadata` is `map[string]any`
- `Event.Payload` is `json.RawMessage`

Check `internal/eventbus/event.go` struct fields and adjust the conversion accordingly.

- [ ] **Step 2: Write test**

```go
// src-go/internal/handler/debug_handler_test.go
package handler_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/labstack/echo/v4"

    "github.com/agentforge/server/internal/handler"
    "github.com/agentforge/server/internal/model"
)

type stubLogTraceQuery struct{ rows []*model.Log }
func (s *stubLogTraceQuery) ListByTraceID(_ context.Context, _ string, _ int) ([]*model.Log, error) {
    return s.rows, nil
}

type stubAutomationTraceQuery struct{ rows []*model.AutomationLog }
func (s *stubAutomationTraceQuery) ListByTraceID(_ context.Context, _ string, _ int) ([]*model.AutomationLog, error) {
    return s.rows, nil
}

type stubEventTraceQuery struct{ rows []*model.Event }
func (s *stubEventTraceQuery) ListByTraceID(_ context.Context, _ string, _ int) ([]*model.Event, error) {
    return s.rows, nil
}

func TestDebugHandler_GetTrace_MergesAndSorts(t *testing.T) {
    now := time.Now().UTC()
    h := handler.NewDebugHandler(
        &stubLogTraceQuery{rows: []*model.Log{{CreatedAt: now.Add(10*time.Millisecond), Level: "info", Summary: "a"}}},
        &stubAutomationTraceQuery{rows: []*model.AutomationLog{{TriggeredAt: now.Add(20*time.Millisecond), EventType: "rule.x", Status: "b"}}},
        &stubEventTraceQuery{rows: []*model.Event{{Timestamp: now.UnixMilli(), Type: "task.created"}}},
    )

    e := echo.New()
    req := httptest.NewRequest(http.MethodGet, "/debug/trace/tr_test", nil)
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    c.SetParamNames("trace_id")
    c.SetParamValues("tr_test")

    if err := h.GetTrace(c); err != nil { t.Fatalf("%v", err) }
    if rec.Code != 200 { t.Fatalf("want 200, got %d", rec.Code) }

    var resp struct {
        TraceID string `json:"traceId"`
        Entries []struct {
            Source    string `json:"source"`
            Timestamp string `json:"timestamp"`
        } `json:"entries"`
    }
    if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil { t.Fatal(err) }
    if resp.TraceID != "tr_test" { t.Fatalf("traceId: %q", resp.TraceID) }
    if len(resp.Entries) != 3 { t.Fatalf("want 3 entries, got %d", len(resp.Entries)) }
    // Sorted: event(now), log(now+10), automation(now+20)
    if resp.Entries[0].Source != "eventbus" { t.Fatalf("first = %q", resp.Entries[0].Source) }
    if resp.Entries[2].Source != "automation" { t.Fatalf("last = %q", resp.Entries[2].Source) }
}
```

- [ ] **Step 3: Run + commit**

```bash
cd src-go && go build ./... && go test ./internal/handler/... -count=1 -v 2>&1 | tail -20
git add src-go/internal/handler/debug_handler.go src-go/internal/handler/debug_handler_test.go
git commit -m "feat(obs): debug handler — /trace/:id merged timeline + /metrics"
```

---

## Task 3: Wire routes admin-gated + add sidebar link

**Files:**
- Modify: `src-go/internal/server/routes.go`
- Modify: `components/app-sidebar.tsx` (or whatever file renders the dashboard nav)

- [ ] **Step 1: Mount `/api/v1/debug/trace/:trace_id` gated by admin RBAC**

In `routes.go`, find where other `/api/v1/*` admin routes are mounted (look for something like `requireAdmin` middleware or `middleware.Require(ActionAdmin)`):

```bash
cd src-go && grep -n "appMiddleware.Require\|jwtMw\|admin" internal/server/routes.go | head -20
```

Add the handler wiring near where other repos/services are instantiated:

```go
debugH := handler.NewDebugHandler(logRepo, automationLogRepo, eventsRepo)
debugGroup := protected.Group("/debug", appMiddleware.Require(appMiddleware.ActionAdmin))
debugGroup.GET("/trace/:trace_id", debugH.GetTrace)
```

Substitute with the actual admin RBAC predicate. If the project uses a role-based gate like `ActionAdmin` or `requireAdminRole`, use that.

- [ ] **Step 2: Mount `/metrics` (NOT admin-gated by convention — Prometheus scrapers need open access, but keep it behind the same `DEBUG_TOKEN` check as pprof OR behind the RBAC admin check — prefer RBAC for consistency)**

```go
e.GET("/metrics", handler.MetricsHandler(), appMiddleware.Require(appMiddleware.ActionAdmin))
```

If the project has no notion of "admin user" yet, just mount it unauthenticated — operators can add a token check later. Document the choice in the commit message.

- [ ] **Step 3: Add sidebar link (frontend)**

In `components/app-sidebar.tsx` (or equivalent), find the existing nav link list. Add:

```tsx
{isAdmin && (
    <Link href="/debug" className="...existing item class...">
        <Bug className="..." />
        Debug
    </Link>
)}
```

Use whatever pattern already renders admin-only items in the file. If no such helper exists, render the link unconditionally and document that visibility enforcement is backend-only — the `/debug` page itself will redirect non-admins (Task 4).

- [ ] **Step 4: Build + typecheck**

```bash
cd src-go && go build ./... && go test ./internal/server/... -count=1 -short 2>&1 | tail -10
cd .. && pnpm exec tsc --noEmit 2>&1 | tail -5
```

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/server/routes.go components/
git commit -m "feat(obs): mount debug routes admin-gated + sidebar link"
```

---

## Task 4: `/debug` page shell + Timeline tab

**Files:**
- Create: `app/(dashboard)/debug/page.tsx`
- Create: `app/(dashboard)/debug/timeline.tsx`
- Create: `lib/stores/debug-store.ts`

- [ ] **Step 1: `debug-store.ts`** — fetches timeline via the debug endpoint.

```ts
"use client";

import { create } from "zustand";
import { createApiClient } from "@/lib/api-client";

interface TimelineEntry {
    timestamp: string;
    source: "logs" | "automation" | "eventbus";
    level?: string;
    eventType?: string;
    summary?: string;
    detail?: Record<string, unknown>;
}

interface DebugState {
    entries: TimelineEntry[];
    loading: boolean;
    error: string | null;
    truncated: boolean;
    fetchTrace: (traceId: string) => Promise<void>;
}

export const useDebugStore = create<DebugState>((set) => ({
    entries: [],
    loading: false,
    error: null,
    truncated: false,
    fetchTrace: async (traceId) => {
        set({ loading: true, error: null });
        try {
            const client = createApiClient(process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777");
            const resp = await client.get<{ entries: TimelineEntry[]; truncated: boolean }>(`/api/v1/debug/trace/${encodeURIComponent(traceId)}`);
            set({ entries: resp.data.entries ?? [], truncated: resp.data.truncated, loading: false });
        } catch (err) {
            set({ error: String(err), loading: false });
        }
    },
}));
```

Adjust based on the actual return shape of the `createApiClient`-style function — it wraps a fetch and returns `{ data, status }` or similar. Check `lib/api-client.ts` for the exact shape.

- [ ] **Step 2: Timeline tab** (`app/(dashboard)/debug/timeline.tsx`):

```tsx
"use client";

import { useState } from "react";
import { useDebugStore } from "@/lib/stores/debug-store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";

export function TimelineTab() {
    const [traceId, setTraceId] = useState("");
    const { entries, loading, error, truncated, fetchTrace } = useDebugStore();

    return (
        <div className="space-y-4">
            <div className="flex gap-2">
                <Input
                    placeholder="tr_…"
                    value={traceId}
                    onChange={(e) => setTraceId(e.target.value)}
                    className="font-mono"
                />
                <Button onClick={() => fetchTrace(traceId)} disabled={!traceId || loading}>
                    {loading ? "Loading…" : "Fetch trace"}
                </Button>
            </div>
            {error && <p className="text-destructive text-sm">{error}</p>}
            {truncated && <p className="text-sm text-yellow-600">Truncated — more than 15k entries matched.</p>}
            <div className="space-y-1">
                {entries.map((e, idx) => (
                    <Card key={idx} className="p-2 text-xs font-mono flex gap-3">
                        <span className="text-muted-foreground">{new Date(e.timestamp).toISOString()}</span>
                        <span className={sourceColor(e.source)}>{e.source}</span>
                        {e.level && <span>{e.level}</span>}
                        {e.eventType && <span>{e.eventType}</span>}
                        <span className="truncate">{e.summary}</span>
                    </Card>
                ))}
            </div>
        </div>
    );
}

function sourceColor(s: string): string {
    switch (s) {
        case "logs": return "text-blue-600";
        case "automation": return "text-purple-600";
        case "eventbus": return "text-green-600";
        default: return "";
    }
}
```

- [ ] **Step 3: Page shell** (`app/(dashboard)/debug/page.tsx`):

```tsx
"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TimelineTab } from "./timeline";
import { LiveTailTab } from "./live-tail";
import { useAuthStore } from "@/lib/stores/auth-store";
import { redirect } from "next/navigation";
import { useEffect } from "react";

export default function DebugPage() {
    const user = useAuthStore((s) => s.user);
    useEffect(() => {
        if (user && !isAdmin(user)) {
            redirect("/");
        }
    }, [user]);

    return (
        <div className="p-6">
            <h1 className="text-2xl font-semibold mb-4">Debug</h1>
            <Tabs defaultValue="timeline">
                <TabsList>
                    <TabsTrigger value="timeline">Timeline</TabsTrigger>
                    <TabsTrigger value="live">Live Tail</TabsTrigger>
                </TabsList>
                <TabsContent value="timeline">
                    <TimelineTab />
                </TabsContent>
                <TabsContent value="live">
                    <LiveTailTab />
                </TabsContent>
            </Tabs>
        </div>
    );
}

function isAdmin(user: { role?: string }): boolean {
    return user.role === "admin" || user.role === "owner";
}
```

Check `useAuthStore`'s user shape — the role field might be different (e.g., `user.roles` array, or `projectRole`). Match the real shape.

- [ ] **Step 4: Typecheck + commit**

```bash
cd .. && pnpm exec tsc --noEmit 2>&1 | tail -10
git add app/\(dashboard\)/debug/ lib/stores/debug-store.ts
git commit -m "feat(obs): /debug page shell + Timeline tab"
```

Note: `live-tail.tsx` is referenced but not yet created — build may warn. Add a temporary stub component:

```tsx
// app/(dashboard)/debug/live-tail.tsx (stub for this task; filled in Task 5)
"use client";
export function LiveTailTab() {
    return <div className="p-4 text-sm text-muted-foreground">Live tail coming in Task 5.</div>;
}
```

Commit that stub with this task.

---

## Task 5: Live Tail tab

**Files:**
- Modify: `app/(dashboard)/debug/live-tail.tsx` (replace stub)
- Possibly extend: `lib/stores/debug-store.ts` for live buffer state

Subscribe to the existing `ws.Hub` WebSocket, bounded 500-item buffer, pause/resume button.

- [ ] **Step 1: Understand the existing WebSocket contract**

```bash
grep -rn "wsUrl\|WS_URL\|websocket" lib/ app/ --include="*.ts" --include="*.tsx" | head -10
```

Find how the frontend currently connects to the Go WS hub (probably via `createApiClient(...)`.wsUrl(...)`). Reuse that.

- [ ] **Step 2: Implement the live-tail component**

```tsx
"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { createApiClient } from "@/lib/api-client";

const MAX = 500;

interface EventMsg {
    type: string;
    source?: string;
    metadata?: Record<string, unknown>;
    timestamp?: number;
}

export function LiveTailTab() {
    const [events, setEvents] = useState<EventMsg[]>([]);
    const [paused, setPaused] = useState(false);
    const bufRef = useRef<EventMsg[]>([]);
    const pausedRef = useRef(paused);
    pausedRef.current = paused;

    useEffect(() => {
        const client = createApiClient(process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777");
        // reuse the project's WS URL builder if one exists
        const ws = new WebSocket(client.wsUrl("/ws"));
        ws.onmessage = (e) => {
            if (pausedRef.current) return;
            try {
                const msg = JSON.parse(e.data);
                bufRef.current.push(msg);
                if (bufRef.current.length > MAX) {
                    bufRef.current = bufRef.current.slice(-MAX);
                }
                setEvents([...bufRef.current]);
            } catch { /* ignore non-JSON */ }
        };
        ws.onerror = (e) => { console.error("WS error", e); };
        return () => ws.close();
    }, []);

    return (
        <div className="space-y-3">
            <div className="flex items-center gap-2">
                <Button
                    variant={paused ? "default" : "outline"}
                    onClick={() => setPaused((p) => !p)}
                >
                    {paused ? "Resume" : "Pause"}
                </Button>
                <span className="text-sm text-muted-foreground">{events.length} events (cap {MAX})</span>
            </div>
            <div className="space-y-0.5 max-h-[70vh] overflow-y-auto">
                {events.slice().reverse().map((ev, idx) => (
                    <Card key={idx} className="p-2 text-xs font-mono flex gap-3">
                        <span>{ev.timestamp ? new Date(ev.timestamp).toISOString() : ""}</span>
                        <span className="text-green-600">{ev.type}</span>
                        {ev.metadata?.trace_id ? <span className="text-blue-600">{String(ev.metadata.trace_id)}</span> : null}
                    </Card>
                ))}
            </div>
        </div>
    );
}
```

`client.wsUrl("/ws")` may need adjustment — inspect the actual `api-client.ts` helper and the existing WS handshake path.

- [ ] **Step 3: Typecheck + commit**

```bash
pnpm exec tsc --noEmit 2>&1 | tail -10
git add app/\(dashboard\)/debug/live-tail.tsx
git commit -m "feat(obs): Live Tail tab — WebSocket subscribe with pause + buffered tail"
```

---

## Post-Phase-2 Smoke

- [ ] Open `/debug` as admin; paste a `trace_id` from a recent log; see merged timeline
- [ ] Switch to Live Tail; trigger an agent action; see events arriving
- [ ] Non-admin redirected away from `/debug`
