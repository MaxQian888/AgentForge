# Observability & Debugging System Design

**Date:** 2026-04-21
**Scope:** Developer-facing observability (tracing, debugging, live inspection) across the AgentForge stack — Next.js frontend, Tauri shell, Go orchestrator, TypeScript Bridge, IM Bridge, Marketplace service
**Approach:** Reuse existing investments (persisted EventBus, `logs` / `automation_logs` tables, Prometheus registry, audit sink) and add the minimum connective tissue needed to turn five isolated log sinks into one coherent, traceable system

---

## 1. Background & Goals

AgentForge already has substantial logging infrastructure, but it is **fragmented**: the Go orchestrator writes structured `logrus` output, persists events through `ebmods.NewPersistFromRepos`, stores per-project entries in a `logs` table, stores automation outcomes in `automation_logs`, and records compliance events through `audit_service`. None of these sinks share a correlation ID, no UI joins them, and the TypeScript and frontend surfaces log nothing but raw `console.log`. The consequence is that debugging a single "user clicks button → agent runs → IM message fires" flow requires manually cross-referencing four unrelated log streams by timestamp.

This spec addresses three goals:

1. **Trace correlation** — every user-initiated action gets one `trace_id` that survives the full frontend → Bridge → orchestrator → IM Bridge round trip and lands in every relevant log sink
2. **Unified developer debug surface** — a single `/debug` page in the dashboard that timelines existing persisted data by `trace_id`, exposes live event streams, and surfaces Prometheus metrics / scheduler state / plugin state
3. **Close the obvious infrastructure gaps** — configurable log levels on the orchestrator, `pprof`, stack traces on panic, Tauri file logging, frontend error reporting, and the OpenTelemetry stub wired up

**Non-goals:** replacing `logrus`, introducing a new event store, building a full APM (Datadog/Grafana) replacement, adding a step-level `transcript` table (the persisted EventBus already captures step transitions for workflows), integrating Sentry or commercial tools.

---

## 2. Current State Inventory (what already exists)

Before designing new infrastructure, this spec assumes the following facts hold. Any change to them invalidates the design.

### 2.1 Go orchestrator (`src-go/`)

| Component | File | Status |
|-----------|------|--------|
| Structured logger | `cmd/server/main.go:49-57` | `logrus` JSON (prod) / text (dev); level hard-coded by `cfg.Env`. **No `LOG_LEVEL` env var.** |
| Request middleware | `internal/server/server.go` | Echo `RequestID`, request log (method/URI/status/latency_ms/request_id), `Recover`, CORS, secure, gzip, 30s timeout |
| Health | `internal/handler/health_handler.go` | `/health`, `/api/v1/health` with version / commit / buildDate / env |
| Audit sink | `internal/service/audit_service.go`, `audit_sink.go` | Async queue, 8-attempt exponential backoff, disk spill to `logs/audit_backlog.jsonl`. Compliance-focused, not observability |
| EventBus persistence | `cmd/server/main.go:151` via `ebmods.NewPersistFromRepos(eventsRepo, dlqRepo)` | **Events + DLQ land in DB**; 82+ event constants in `internal/eventbus/types.go` |
| Prometheus metrics | `pkg/metrics/prometheus.go` | 7 registered: `AgentSpawnTotal`, `AgentPoolActive`, `TaskDecomposeTotal`, `ReviewTotal`, `CostUsdTotal`, `BridgeCallDuration`, `TeamRunTotal` + `eventbus_events_observed_total` |
| `logs` table | `internal/model/log.go` | Fields: `project_id`, `tab` (agent/system), `level`, `actor_type`, `actor_id`, `agent_id`, `session_id`, `event_type`, `action`, `resource_type`, `resource_id`, `summary`, `detail` (JSONB), `created_at` |
| `/logs` API | `internal/handler/log_handler.go` | `GET/POST /api/v1/projects/{id}/logs` with filters (tab, level, search, date range, pagination) |
| `automation_logs` table | `internal/model/automation.go:39-47` | `rule_id`, `task_id`, `event_type`, `triggered_at`, `status`, `detail` (JSONB); written by `automation_engine_service.go:33-71` |
| Internal inspect | routes.go ~L1418 | `/internal/scheduler/jobs` GET (list) / POST (fire) exists; no equivalent for plugin registry, queue, trigger engine, agent pool |
| OTel | `pkg/metrics/otel.go` | Literal stub: `InitTracer()` logs "OTel tracing not configured" |
| pprof | — | **Not registered** |
| Panic stack capture | `internal/instruction/router.go:283-291` | Only site; catches panic to error string, **no stack trace** |

### 2.2 TypeScript Bridge (`src-bridge/`)

Hono on Bun, raw `console.log("[Bridge] ...")` everywhere. No request-logging middleware, no `requestId`, no structured logs, no pino/winston. `EventStreamer` (`src/ws/event-stream.ts`) reconnects to Go's WebSocket with a 50-message buffer.

### 2.3 IM Bridge (`src-im-bridge/`)

Go + `logrus`. `cmd/bridge/logger.go:10,22-26` **honors `LOG_LEVEL` and `LOG_FORMAT` env vars**. `audit/writer.go` implements JSONL append-only + rotating file writers.

### 2.4 Frontend (`app/`, `components/`, `lib/`)

- No logger abstraction — `app/error.tsx` and `app/(dashboard)/error.tsx` use `console.error` only, do **not** report to the backend
- `lib/stores/log-store.ts` consumes `/api/v1/projects/{id}/logs` with full filter/pagination
- `components/project/audit-log-panel.tsx` + `components/automations/automation-log-viewer.tsx` are the only user-facing log surfaces
- No `/debug`, `/diagnostics`, `/observability`, or `/admin` page
- Zustand has no `devtools` middleware; React Query is not in use

### 2.5 Tauri (`src-tauri/`)

- `tauri-plugin-log = "2"` in `Cargo.toml` — **installed but never initialized** in `tauri.conf.json` or `lib.rs`
- Sidecar stdout/stderr captured at `lib.rs:862` (approx.) and forwarded to `log::info!("[{label}] ...")`; no file sink, no rotation

### 2.6 Dev scripts (`scripts/dev/`)

- `dev-all.js` writes per-service stdout/stderr files to `runtime-logs/{service}.stdout.log` / `.stderr.log`
- `dev-workflow.js:113-122` provides `tailFile()` — read-last-N-lines, no aggregation, no correlation, no search
- `pnpm dev:all:logs` and `pnpm dev:backend:logs` print paths, not content

---

## 3. Design Overview

Three phases, each independently shippable. Each phase produces measurable developer-facing value without requiring the next phase to exist.

```
Phase 1 — Correlation ID vertebra           (2–3 days)
  frontend → Bridge → Go → IM Bridge, one trace_id plumbed end-to-end
  every existing log sink writes trace_id into its JSONB detail column
  POST /api/v1/internal/logs/ingest introduced for Bridge/frontend writes

Phase 2 — /debug developer surface          (3–4 days MVP: Timeline + Live Tail)
  Next.js /debug page joins logs + automation_logs + persisted EventBus by trace_id
  live WebSocket tail of EventBus
  Metrics + Inspect tabs are post-MVP (additional ~1–2 days)

Phase 3 — Infrastructure gap closure        (2–3 days total)
  LOG_LEVEL env var, pprof, stack-on-panic, tauri-plugin-log init,
  frontend error reporting, OTel OTLP exporter, Zustand devtools
```

Phase 1 is the hinge. Phases 2 and 3 both depend on it existing but do not depend on each other.

---

## 4. Phase 1 — Correlation ID Plumbing

### 4.1 Design

A single request-scoped string `trace_id` — format `tr_` + 24-char crockford base32 (12 bytes) — is generated once at the origin (usually a user click) and travels as the HTTP header `X-Trace-ID`. Every service that receives the header adopts it; any service that does not receive it generates a fresh one. Every structured log line and every row written to `logs` / `automation_logs` / persisted EventBus events carries `trace_id` in its `detail` JSONB.

The existing Echo `RequestID` middleware already generates an ID per request and exposes it as `X-Request-ID`. Phase 1 does **not** replace it — it *promotes* `X-Request-ID` to a cross-service trace by:

1. Accepting inbound `X-Trace-ID`, falling back to `X-Request-ID`, falling back to a fresh generated ID
2. Storing the resolved ID in `context.Context` under a typed key
3. Writing the resolved ID back on the response header
4. Making every log-emitting call site include it

### 4.2 Component changes

**Go orchestrator (`src-go/`)**

- `internal/middleware/trace.go` (new) — Echo middleware: read `X-Trace-ID`, fall back to `X-Request-ID`, fall back to newly generated. Attach to `context.Context` via `type contextKey struct{}` (unexported). Set response header. Order: **after** `RequestID`, **before** any domain middleware.
- `internal/log/context.go` (new) — helpers: `TraceID(ctx) string`, `WithTrace(ctx, id) context.Context`, `Logger(ctx) *logrus.Entry` that returns `logrus.WithField("trace_id", TraceID(ctx))`.
- `internal/service/log_service.go` — when `CreateLogInput.Detail` is nil, initialize; always merge `trace_id` from context into `detail`.
- `internal/service/automation_engine_service.go` — same treatment on `CreateLog`.
- `internal/eventbus` — extend publish path so persisted events include `trace_id` in their payload (already a JSONB-equivalent). The simplest implementation: add `TraceID string` field to the `Event` struct, persist it as a top-level column **or** merge into the existing payload JSON. Choice: **merge into payload JSON** to avoid a schema migration; index later if query performance demands it.
- `pkg/metrics/prometheus.go` — no change. Metrics stay aggregate; trace joining happens via logs.
- Bridge client (`internal/bridge/client.go`) — propagate `trace_id` from ctx to outbound HTTP calls via `X-Trace-ID` header.
- IM Bridge client (if any) — same.

**TypeScript Bridge (`src-bridge/`)**

- `src/middleware/trace.ts` (new) — Hono middleware mirroring the Go logic: accept inbound `X-Trace-ID`, generate if missing, stash on `c.var.traceId`.
- `src/lib/logger.ts` (new) — thin wrapper around `pino` (add dep). API: `log.info({ traceId, …fields }, msg)`. Replace every `console.log("[Bridge] …")` in `server.ts`, `plugins/*`, `mcp/*`, `runtime/*`, `ws/event-stream.ts`.
- Outbound fetch calls to Go — inject `X-Trace-ID` from `c.var.traceId`.
- Selected high-signal events (plugin load/unload/error, runtime spawn/exit, MCP call failures) — after logging locally, `POST /api/v1/internal/logs/ingest` (new internal endpoint) so the Bridge shows up in the unified view. Payload: `{ projectId?, traceId, tab:"system", level, summary, detail }`. Rate-limited on the Go side (see §7).

**IM Bridge (`src-im-bridge/`)**

- Already uses `logrus` and honors `LOG_LEVEL`. Add a minimal HTTP middleware (`cmd/bridge/middleware_trace.go`, new) that accepts `X-Trace-ID` and places it on `context.Context`; update the existing logrus call sites to use `WithField("trace_id", …)`.
- For inbound IM events (no upstream header), generate a fresh `trace_id` and include it in the internal-ingest call so the orchestrator can correlate.

**Ingest endpoint (new, Phase 1 deliverable)**

- `internal/handler/ingest_handler.go` (new) — `POST /api/v1/internal/logs/ingest`. Accepts `{ projectId?, traceId, tab, level, summary, detail, source }` and writes to the `logs` table via the existing `log_service.Create`. Required because the Bridge and frontend cannot write directly to the DB.
- Auth: existing API-token middleware. No new auth primitives.
- Rate-limit: token-bucket 100 req/s per source, burst 200, 429 on overflow (see §7.2).
- This endpoint MUST exist before the Bridge / frontend logger changes ship, because those call sites depend on it. It is reused unchanged by Phase 2.

**Frontend (`lib/`, `hooks/`, `app/`)**

- `lib/log.ts` (new) — small logger API: `log.info/warn/error(event, detail?)`. Default transport: `console.<level>` + `POST /api/v1/internal/logs/ingest` at `level >= warn` (batched, 1s flush or 10-item buffer; dropped silently on failure, never blocks UI).
- `lib/fetch.ts` (new or extend existing API client) — intercept every outbound fetch: if no `X-Trace-ID` header, generate one and stash on `window.__traceContext` keyed by an ergonomic caller-supplied label; attach to the request.
- `app/error.tsx`, `app/(dashboard)/error.tsx` — call `log.error("render.error", { digest, stack })` in their `useEffect`.

**Tauri (`src-tauri/`)**

- No trace plumbing here (sidecars forward through HTTP, and the Rust shell is not a request-path node). Phase 3 adds file logging.

### 4.3 `trace_id` lifetime rules

- **One trace per user-initiated top-level action.** Example: a click that triggers "Run agent" yields a single `tr_…` that carries through agent spawn, plugin calls, IM delivery.
- Background jobs (scheduler, automation engine, event-driven workflow executors) **generate their own** trace when the job starts; subsequent fan-out inherits it via context.
- When a new trace is generated mid-chain (because the upstream lost it), the logger writes one `log.warn("trace.generated_midchain", { upstream: "<service>" })` so the gap is visible.

### 4.4 Acceptance criteria for Phase 1

- `SELECT * FROM logs WHERE detail->>'trace_id' = $1` returns rows spanning at least two services for any user-initiated agent run
- The Echo request log line includes `trace_id` as a first-class field
- `console.log` count in `src-bridge/src/` drops to zero in non-test code (ESLint rule added)
- A synthetic request sent through `curl -H "X-Trace-ID: tr_test123456789012345678abcd" …/api/v1/…` is observable end-to-end in the `logs` table by that ID

---

## 5. Phase 2 — `/debug` Developer Surface

### 5.1 Page structure

New route: `app/(dashboard)/debug/page.tsx` (client-rendered). Four tabs, only the first two are MVP:

1. **Timeline** (MVP) — input: `trace_id` OR `session_id` OR date range. Output: merged, chronologically-ordered view of `logs` + `automation_logs` + persisted EventBus events. Each row shows timestamp, service source, level, summary; row expansion reveals `detail` JSON. Services are colour-coded (orchestrator / bridge / im-bridge / frontend).
2. **Live Tail** (MVP) — WebSocket subscription to EventBus (reuse the existing orchestrator WS hub). Displays incoming events in real time. Filter by event type, level, resource. Pause/resume button. Bounded in-memory buffer (latest 500).
3. **Metrics** (post-MVP) — embeds a minimal Prometheus scrape view. Calls `/metrics` (add handler if not already exposed) and renders the 7 registered metric families as tables + tiny sparklines. No Grafana — this is "good enough to read during a bug hunt."
4. **Inspect** (post-MVP) — wraps existing `/internal/scheduler/jobs` and new inspect endpoints for plugin registry, queue depth, agent pool active count. Each is a simple "fetch JSON → render table" panel.

### 5.2 Backend changes

- `internal/handler/debug_handler.go` (new) — three endpoints, all admin-gated:
  - `GET /api/v1/debug/trace/{trace_id}` — joins `logs`, `automation_logs`, and the persisted events repo by `trace_id`, returns a merged, time-sorted array.
  - `GET /api/v1/debug/plugins` — dumps the in-memory plugin registry state (names, versions, statuses).
  - `GET /api/v1/debug/queue` — dumps current queue depth + per-priority buckets.
  - `GET /api/v1/debug/pool` — dumps agent pool active / reserved counts.
- `internal/handler/metrics_handler.go` (may already exist for Prometheus; if not, add `GET /metrics` using `promhttp.Handler()`)
- `internal/repository/events_repo.go` — new method `ListByTraceID(traceID string) ([]Event, error)` that queries the persisted-events table where `metadata->>'trace_id' = $1` (trace_id is stored in `Event.Metadata`, not `Payload`; confirmed by Phase 1 implementation).
- The ingest endpoint (`POST /api/v1/internal/logs/ingest`) already exists from Phase 1 and is reused here unchanged.

### 5.3 Permission model

- Page and all endpoints require role `admin` or `owner` at the project level; the `/debug/trace/:id` route is project-scoped if the trace is linked to a project, else it requires global admin.
- Permission gate uses existing RBAC middleware; no new primitives.

### 5.4 Acceptance criteria for Phase 2

- A developer can paste a `trace_id` into `/debug` and see, within 2 seconds, a merged timeline spanning the request
- Live Tail shows events as they occur with ≤1s latency and does not crash the page on bursts (tested with 200 events/sec)
- Non-admin users cannot see the page or hit its endpoints

---

## 6. Phase 3 — Infrastructure Gap Closure

Each item below is a small change. **G1, G2, G3, G4, G7 are independent** of Phases 1 and 2 and can ship in any order, including before Phase 1. **G5 and G6 depend on Phase 1** (G5 uses `lib/log.ts` from §4.2; G6 reuses the `trace_id` middleware).

| ID | Change | File(s) |
|----|--------|---------|
| G1 | Add `LOG_LEVEL` env var to orchestrator, aligning with IM Bridge | `cmd/server/main.go:51-57`, `internal/config/config.go` |
| G2 | Register `pprof` under `/debug/pprof/*`, admin-gated | `internal/server/server.go` — import `_ "net/http/pprof"`, attach via Echo adapter |
| G3 | Capture stack trace on panic — extend Echo `Recover` config and the `instruction/router.go` panic recovery to call `debug.Stack()`; log via `logrus` at `error` level with `stack` field | `internal/server/server.go`, `internal/instruction/router.go:283-291` |
| G4 | Initialize `tauri-plugin-log` with file sink + rotation (daily, 10-file retention) | `src-tauri/src/lib.rs`, `src-tauri/tauri.conf.json` |
| G5 | Frontend error reporting from `app/error.tsx` and `app/(dashboard)/error.tsx` using `lib/log.ts` (Phase 1) | `app/error.tsx`, `app/(dashboard)/error.tsx` |
| G6 | OTel OTLP exporter — replace the stub, reuse `trace_id` as the trace identifier, emit spans for HTTP requests and Bridge calls. Configurable via `OTEL_EXPORTER_OTLP_ENDPOINT`; no-op if unset | `pkg/metrics/otel.go`, `internal/middleware/trace.go` |
| G7 | Zustand `devtools` middleware in dev builds only | `lib/stores/*.ts` — guarded by `process.env.NODE_ENV !== "production"` |

G1, G3, G5, G7 are strictly additive and safe. G2 requires a permission check (avoid exposing pprof to untrusted users). G4 introduces a disk-writing background task; retention bounded to prevent disk fill. G6 is optional — ships only if an OTLP endpoint is available; otherwise remains a no-op.

---

## 7. Data & Contract Decisions

### 7.1 `trace_id` storage

- **Choice:** merge into existing `detail` JSONB columns for `logs`, `automation_logs`, and the persisted events payload. **No new columns, no migration.**
- **Why:** a migration blocks the spec; JSONB query on `detail->>'trace_id'` is acceptable at the scale of a developer tool (single-tenant developer workloads). If query volume grows, a partial index on `(detail->>'trace_id')` can be added later without schema change.

### 7.2 Internal ingest endpoint rate limits

- **Choice:** token-bucket per API token, 100 req/s, burst 200; reject with 429.
- **Why:** the endpoint is writable from browsers and external Bridges. Abuse should not fill the `logs` table.

### 7.3 `trace_id` format

- **Choice:** `tr_` + 24-char Crockford base32 (15 random bytes; 120 bits → 24 chars at 5 bits/char). Fits in standard headers, URL-safe, visually distinguishable from request IDs.

### 7.4 Logging library choices

- **Go services:** keep `logrus`. No migration to `zap`/`slog` — the cost-benefit is not there for a dev-tools spec.
- **TS Bridge:** `pino` — smallest, fastest, Bun-compatible.
- **Frontend:** hand-rolled `lib/log.ts`; no third-party dep. The feature set is small enough that a library adds more weight than it removes.

### 7.5 What Phase 2 does NOT do

- Does not introduce a new `step_transcript` table. The persisted EventBus (see §2.1) already captures step transitions; the Timeline view is the replay UI.
- Does not integrate Sentry, Datadog, or any commercial tool.
- Does not rewrite the existing audit-log UI. `AuditLogPanel` and `AutomationLogViewer` remain for their business-facing use; `/debug` is the developer-facing companion.

---

## 8. Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| `trace_id` in JSONB becomes slow at scale | Add a partial GIN index on `(detail->>'trace_id')` when and if `EXPLAIN` shows it matters. Not a Phase 1 prerequisite. |
| `/debug/trace/:id` unbounded join crashes DB | Hard cap at 10 000 rows per trace in the query; return a `truncated: true` flag to the UI. |
| `pprof` exposed accidentally in prod | Admin-gated; separately require `DEBUG_TOKEN` env var to enable in `production` env. |
| Bridge ingest flood fills `logs` table | Rate-limit + per-project row cap (prune > 30 days via existing cleanup job; add one if absent). |
| `tauri-plugin-log` disk fill on desktop | 10-file rotation, daily buckets; total <50MB worst case. |
| OTel export failure degrades request latency | Export is async non-blocking (OTel SDK default); on persistent failure, log once and stop retrying for 5 minutes. |
| Frontend trace generation collides across tabs | `trace_id` is per-request, not per-session; collisions harmless. |

---

## 9. Delivery Sequencing

Phase 1 must land first. Phases 2 and 3 are independent after that.

```
Phase 1 (2–3 days) — trace_id plumbed end-to-end
    ├─ Phase 2 (3–4 days) — /debug UI + ingest endpoint + trace query
    └─ Phase 3 (2–3 days) — G1..G7 in any order
```

Recommended merge strategy: one PR per phase. Phase 3 may be split into 2–3 PRs if reviewer prefers (G1+G3+G5 is a natural cluster; G2 alone; G4 alone; G6 alone; G7 alone).

---

## 10. Success Criteria

A Phase-1+2+3 rollout is considered successful when:

1. **Trace roundtrip** — a developer can trigger a UI action, open `/debug`, paste the trace ID, and see entries from frontend, Bridge, orchestrator, and IM Bridge within 2 seconds.
2. **Live tail** — `/debug` live-tails eventbus with ≤1s latency at ≥200 events/sec without client-side crash.
3. **Configurable log levels** — setting `LOG_LEVEL=debug` on the orchestrator (no rebuild) surfaces debug logs; default production is `warn`.
4. **Panic stack visible** — a deliberately-triggered panic produces a log line with a full Go stack trace.
5. **Tauri file log** — after a desktop run, a `log/AgentForge.log` file exists under the OS data directory with sidecar output.
6. **Frontend errors reported** — a deliberate render error produces a row in `logs` with `level=error`, `tab=system`.
7. **Zero `console.log` in `src-bridge/src/` non-test code** — ESLint rule `no-console` passing.
8. **pprof reachable** — `curl -H "X-Debug-Token: …" http://localhost:7777/debug/pprof/heap` returns a valid heap dump.

---

## 11. Open Questions (to be resolved during plan phase)

1. Should Phase 2 Timeline support joining by `session_id` as a fallback when `trace_id` is absent? (Leaning yes — cheap to add; the spec already allows this input in §5.1.)
2. Should `/debug` be hidden behind a feature flag even for admins in production? (Leaning no — admins should always have it.)
3. Should the OTel span IDs be separate from `trace_id`, or should `trace_id` double as the OTel `traceparent` value? (Leaning: double-duty, with the Crockford ID converted to the OTel hex format at the exporter boundary.)

These are flagged for the implementation plan phase; none block the spec.
