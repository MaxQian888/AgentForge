# Observability Phase 3 — Infrastructure Gap Closure

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Close the six smallest, highest-leverage infrastructure gaps from spec §6 (G1, G2, G3, G4, G7) plus one Phase-1 follow-up (browser logger per-entry trace_id). Defers G6 OTel and prod-hardening items (ingest auth, rate-limiter LRU) to a later pass.

**Architecture:** Each item is a small, independent change to an existing file. No new services, no cross-service coordination. The follow-up (F1) refines the Phase 1 browser logger to stamp `trace_id` at push time rather than flush time, so Phase 2's timeline view gets correct attribution.

**Tech Stack:** Go (orchestrator), Rust (tauri-plugin-log), TypeScript (Zustand devtools, browser logger).

**Source of truth:** `docs/superpowers/specs/2026-04-21-observability-system-design.md` §6 (G1–G7).

---

## File Structure

Files modified:
- `src-go/internal/config/config.go` — add `LogLevel` field
- `src-go/cmd/server/main.go` — honor `LOG_LEVEL` env var
- `src-go/internal/server/server.go` — mount pprof + stack-on-panic
- `src-go/internal/instruction/router.go:283-291` — stack capture on existing panic site
- `src-tauri/src/lib.rs` + `src-tauri/tauri.conf.json` — tauri-plugin-log init with file sink
- `lib/stores/*.ts` — Zustand `devtools` wrapper in dev builds (one central utility, not per-store)
- `lib/log.ts` — stamp `trace_id` per-entry at push time; ingest handler already accepts `detail.trace_id` via Phase 1 Task 5

---

## Task 1: G1 — LOG_LEVEL env var for orchestrator

**Files:**
- Modify: `src-go/internal/config/config.go`
- Modify: `src-go/cmd/server/main.go`

Align the orchestrator with IM Bridge, which already honors `LOG_LEVEL` (`src-im-bridge/cmd/bridge/logger.go:22-26`).

- [ ] **Step 1: Add LogLevel field to config**

In `src-go/internal/config/config.go`, add to the `Config` struct:

```go
type Config struct {
    // existing fields...
    LogLevel string // one of: debug, info, warn, error. Empty = env-based default (debug in dev, warn in prod).
}
```

In the `Load()` function (or equivalent loader), read:

```go
cfg.LogLevel = os.Getenv("LOG_LEVEL")
```

- [ ] **Step 2: Honor LOG_LEVEL in main.go**

Replace the existing block at `src-go/cmd/server/main.go:49-57`:

```go
log.SetOutput(os.Stdout)
if cfg.Env == "production" {
    log.SetFormatter(&log.JSONFormatter{})
    log.SetLevel(log.WarnLevel)
} else {
    log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
    log.SetLevel(log.DebugLevel)
}
```

with:

```go
log.SetOutput(os.Stdout)
if cfg.Env == "production" {
    log.SetFormatter(&log.JSONFormatter{})
    log.SetLevel(log.WarnLevel)
} else {
    log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
    log.SetLevel(log.DebugLevel)
}
// LOG_LEVEL env var overrides env-based default on both paths.
if cfg.LogLevel != "" {
    if lvl, err := log.ParseLevel(cfg.LogLevel); err == nil {
        log.SetLevel(lvl)
    } else {
        log.WithError(err).WithField("value", cfg.LogLevel).Warn("invalid LOG_LEVEL, using default")
    }
}
```

- [ ] **Step 3: Test the override**

Add to `src-go/internal/config/config_test.go` (create if absent):

```go
func TestConfig_LogLevel_FromEnv(t *testing.T) {
    t.Setenv("LOG_LEVEL", "warn")
    cfg := config.Load()
    if cfg.LogLevel != "warn" {
        t.Fatalf("want warn, got %q", cfg.LogLevel)
    }
}
```

- [ ] **Step 4: Run + commit**

```bash
cd src-go && go build ./... && go test ./internal/config/... -count=1
git add src-go/internal/config/config.go src-go/internal/config/config_test.go src-go/cmd/server/main.go
git commit -m "feat(obs): orchestrator honors LOG_LEVEL env var (aligns with im-bridge)"
```

---

## Task 2: G2 — pprof endpoint (admin-gated)

**Files:**
- Modify: `src-go/internal/server/server.go` (import + conditional mount)

Mount `net/http/pprof` handlers under `/debug/pprof/*` only when a `DEBUG_TOKEN` env var is set AND (if in production) the request supplies that token in `X-Debug-Token` header.

- [ ] **Step 1: Blank import + middleware**

Add to `server.go` imports:

```go
import (
    _ "net/http/pprof" // registers handlers on http.DefaultServeMux
    // ... existing imports
)
```

Add a small middleware defined in the same file (or `internal/middleware/debug_auth.go` if preferred):

```go
// requireDebugToken is a middleware that gates /debug/pprof/* behind a shared secret.
// If DEBUG_TOKEN is unset, the endpoint is not registered at all.
func requireDebugToken(token string) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            if c.Request().Header.Get("X-Debug-Token") != token {
                return c.NoContent(http.StatusNotFound) // hide existence
            }
            return next(c)
        }
    }
}
```

- [ ] **Step 2: Mount the route**

In the route-registration section, add:

```go
if token := os.Getenv("DEBUG_TOKEN"); token != "" {
    pprofGroup := e.Group("/debug/pprof", requireDebugToken(token))
    // Echo can't directly adopt net/http handlers as a group; use echo.WrapHandler.
    pprofGroup.Any("/*", echo.WrapHandler(http.DefaultServeMux))
    log.WithField("path", "/debug/pprof/*").Info("pprof enabled")
}
```

If the server is mounted on a mux other than `DefaultServeMux`, adjust accordingly — the blank `_ "net/http/pprof"` import registers handlers on `DefaultServeMux` by default.

- [ ] **Step 3: Integration test**

Add to `server_test.go`:

```go
func TestPprof_Disabled_WhenTokenUnset(t *testing.T) {
    t.Setenv("DEBUG_TOKEN", "")
    // build the server, hit /debug/pprof/heap, expect 404
}

func TestPprof_RequiresToken(t *testing.T) {
    t.Setenv("DEBUG_TOKEN", "secret")
    // build the server, hit /debug/pprof/heap WITHOUT X-Debug-Token, expect 404
    // hit WITH X-Debug-Token: secret, expect 200
}
```

Match existing server test harness pattern.

- [ ] **Step 4: Run + commit**

```bash
cd src-go && go build ./... && go test ./internal/server/... -count=1
git add src-go/internal/server/server.go src-go/internal/server/server_test.go
git commit -m "feat(obs): admin-gated pprof under /debug/pprof (requires DEBUG_TOKEN)"
```

---

## Task 3: G3 — stack trace on panic

**Files:**
- Modify: `src-go/internal/server/server.go` (extend Echo Recover config)
- Modify: `src-go/internal/instruction/router.go:283-291` (stack capture at existing panic site)

Goal: any panic recovers with a full goroutine stack trace logged at `error` level with the `trace_id` attached.

- [ ] **Step 1: Extend Echo Recover config**

In `server.go`, replace `e.Use(echomiddleware.Recover())` with:

```go
e.Use(echomiddleware.RecoverWithConfig(echomiddleware.RecoverConfig{
    StackSize:         4 << 10, // 4 KB
    DisableStackAll:   false,
    DisablePrintStack: false, // echo's default logger prints; we add structured log too
    LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
        log.WithFields(log.Fields{
            "trace_id": applog.TraceID(c.Request().Context()),
            "path":     c.Request().URL.Path,
            "method":   c.Request().Method,
            "stack":    string(stack),
        }).WithError(err).Error("panic recovered")
        return err
    },
}))
```

Ensure `applog` is imported.

- [ ] **Step 2: Fix the instruction-router site**

At `src-go/internal/instruction/router.go:283-291`, the current panic recovery catches the panic but doesn't capture stack. Change:

```go
defer func() {
    if r := recover(); r != nil {
        err = fmt.Errorf("panic: %v", r)
    }
}()
```

to:

```go
defer func() {
    if r := recover(); r != nil {
        buf := make([]byte, 4<<10)
        n := runtime.Stack(buf, false)
        log.WithFields(log.Fields{
            "trace_id": applog.TraceID(ctx),
            "stack":    string(buf[:n]),
        }).Error("panic in instruction router")
        err = fmt.Errorf("panic: %v", r)
    }
}()
```

Add imports: `"runtime"`, `log "github.com/sirupsen/logrus"`, `applog "github.com/agentforge/server/internal/log"`.

- [ ] **Step 3: Test**

Add to `server_test.go`:

```go
func TestPanicRecovery_LogsStack(t *testing.T) {
    var buf bytes.Buffer
    old := log.StandardLogger().Out
    log.SetOutput(&buf)
    t.Cleanup(func() { log.SetOutput(old) })

    // Register a test-only route that panics, call it, assert 500 + log contains "stack":"goroutine "
    // Use the existing server test harness.
}
```

- [ ] **Step 4: Run + commit**

```bash
cd src-go && go build ./... && go test ./internal/server/... ./internal/instruction/... -count=1
git add src-go/internal/server/server.go src-go/internal/server/server_test.go src-go/internal/instruction/router.go
git commit -m "feat(obs): capture stack trace on panic recovery"
```

---

## Task 4: G4 — tauri-plugin-log file sink

**Files:**
- Modify: `src-tauri/src/lib.rs` (init the plugin)
- Modify: `src-tauri/tauri.conf.json` (plugin config)

The dependency `tauri-plugin-log = "2"` already exists in `Cargo.toml` but is never initialized. This task turns on a file sink with rotation so desktop-mode crashes leave a local log trail.

- [ ] **Step 1: Initialize the plugin**

In `src-tauri/src/lib.rs`, find the `tauri::Builder::default()` chain (typically in the `run()` function). Add:

```rust
.plugin(
    tauri_plugin_log::Builder::default()
        .level(log::LevelFilter::Info)
        .targets([
            tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Stdout),
            tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::LogDir { file_name: Some("AgentForge".to_string()) }),
            tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Webview),
        ])
        .max_file_size(10_000_000) // 10 MB
        .rotation_strategy(tauri_plugin_log::RotationStrategy::KeepAll)
        .build(),
)
```

Adjust API surface to match the installed version — `tauri-plugin-log = "2"`. If any API mismatch, consult: `https://v2.tauri.app/plugin/logging/` (cached in crate docs). The key invariant: a file sink at the OS log dir, ≤10MB per file, bounded rotation.

- [ ] **Step 2: No separate config in tauri.conf.json unless the plugin version requires it.**

If building succeeds without config entries, skip the JSON edit. If the plugin version requires a permissions entry, add to `tauri.conf.json` under `app.security.capabilities` or similar.

- [ ] **Step 3: Build**

```bash
cd src-tauri && cargo build 2>&1 | tail -10
```

Must compile cleanly. If the API signatures differ from the above, adjust to match and note in the commit message.

- [ ] **Step 4: Smoke test (manual, optional)**

```bash
pnpm tauri dev
# Trigger a sidecar spawn, observe the log file appears at the OS data dir.
# Windows: %APPDATA%\com.agentforge.app\logs\AgentForge.log
# macOS:   ~/Library/Logs/com.agentforge.app/AgentForge.log
# Linux:   ~/.local/share/com.agentforge.app/logs/AgentForge.log
```

- [ ] **Step 5: Commit**

```bash
git add src-tauri/src/lib.rs src-tauri/tauri.conf.json
git commit -m "feat(obs): initialize tauri-plugin-log with rotating file sink"
```

---

## Task 5: G7 — Zustand devtools in dev builds

**Files:**
- Create: `lib/stores/_devtools.ts` (small helper)
- Modify: 3-5 representative stores to use the helper

Zustand's `devtools` middleware lets the Redux DevTools browser extension inspect store state. Off in production to avoid the overhead.

- [ ] **Step 1: Write the helper**

`lib/stores/_devtools.ts`:

```ts
import { devtools } from "zustand/middleware";
import type { StateCreator } from "zustand";

type DevtoolsFn = <T>(
  initializer: StateCreator<T, [], []>,
  options?: { name?: string },
) => StateCreator<T, [], [["zustand/devtools", never]]>;

const noop: DevtoolsFn = (initializer) => initializer as unknown as ReturnType<DevtoolsFn>;

/**
 * Conditionally wraps a Zustand state creator with the devtools middleware.
 * Active in development builds only; a no-op in production.
 */
export const withDevtools: DevtoolsFn =
  process.env.NODE_ENV === "production" ? noop : (devtools as unknown as DevtoolsFn);
```

- [ ] **Step 2: Apply the helper to 3-5 high-signal stores**

Pick stores that are useful to inspect during development (e.g., `lib/stores/auth-store.ts`, `lib/stores/log-store.ts`, `lib/stores/automation-store.ts`, `lib/stores/audit-store.ts`).

For each, change:

```ts
export const useAuthStore = create<AuthState>((set) => ({ ... }));
```

to:

```ts
import { withDevtools } from "./_devtools";

export const useAuthStore = create<AuthState>()(
  withDevtools((set) => ({ ... }), { name: "auth-store" }),
);
```

> Note the `create<AuthState>()(...)` double-call syntax — required when using middleware in zustand v5.

- [ ] **Step 3: Typecheck**

```bash
pnpm exec tsc --noEmit 2>&1 | tail -10
```

Clean.

- [ ] **Step 4: Run jest for the modified stores**

```bash
pnpm jest --testPathIgnorePatterns "/node_modules/" --testPathPatterns "lib/stores" 2>&1 | tail -10
```

All pass. Devtools middleware should not change state behavior.

- [ ] **Step 5: Commit**

```bash
git add lib/stores/
git commit -m "feat(obs): enable Zustand devtools in dev builds (no-op in prod)"
```

---

## Task 6: F1 — Browser logger stamps trace_id per entry

**Files:**
- Modify: `lib/log.ts`
- Modify: `lib/log.test.ts`

The Phase 1 implementation attached one `X-Trace-ID` header to the whole batched ingest request. If two user-initiated requests happen within a single flush window, both their warn/error entries batch together under whichever trace was active at flush time — a semantic conflation.

Fix: stamp `trace_id` on each entry at push time. Each entry already goes into `detail` JSONB; add the trace field there. The ingest handler (Phase 1 Task 5) already merges `detail.trace_id` from ctx → this task replaces the header-level trace with per-entry trace so the orchestrator picks up the entry's own trace.

- [ ] **Step 1: Update the test**

In `lib/log.test.ts`, extend the existing "batches and POSTs warn+ to …" test:

```ts
it("stamps per-entry trace_id at push time, not flush time", async () => {
  const calls: Array<{ init: RequestInit }> = [];
  const fetchMock: typeof fetch = (async (_i, init) => {
    calls.push({ init: init! });
    return new Response(null, { status: 202 });
  }) as typeof fetch;

  let currentTrace = "tr_first0000000000000000000";
  const log = createBrowserLogger({
    fetch: fetchMock,
    flushMs: 20,
    bufferSize: 10,
    traceId: () => currentTrace,
  });

  log.warn("a");                    // stamped with tr_first
  currentTrace = "tr_second00000000000000000";
  log.error("b");                   // stamped with tr_second

  await new Promise((r) => setTimeout(r, 40));
  const body = JSON.parse(String(calls[0]!.init.body));
  expect(body[0].detail?.trace_id).toBe("tr_first0000000000000000000");
  expect(body[1].detail?.trace_id).toBe("tr_second00000000000000000");
});
```

- [ ] **Step 2: Adjust the implementation**

In `lib/log.ts`, modify `push()` (inside `createBrowserLogger`) to capture the trace at push time:

```ts
function push(level: Level, summary: string, detail?: Record<string, unknown>) {
    // ... console mirroring unchanged ...

    if (!INGEST_LEVELS.has(level)) {
      return;
    }
    const traceAtPush = opts.traceId();
    const stampedDetail = { ...(detail ?? {}), trace_id: traceAtPush };
    buffer.push({ tab: "system", level, source: "frontend", summary, detail: stampedDetail, ts: Date.now() });
    if (buffer.length >= cap) {
      void flush();
    } else {
      schedule();
    }
}
```

And keep the batch-level header for backward compatibility but it's no longer authoritative:

```ts
async function flush() {
    timer = null;
    if (buffer.length === 0) return;
    const batch = buffer.splice(0, buffer.length);
    try {
      await f(endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          // Last-write-wins header for request tracing; per-entry detail.trace_id is authoritative.
          "X-Trace-ID": opts.traceId(),
        },
        body: JSON.stringify(batch),
        keepalive: true,
      });
    } catch {
      // never block UI on ingest failure
    }
}
```

- [ ] **Step 3: Verify existing tests still pass**

```bash
pnpm jest --testPathIgnorePatterns "/node_modules/" --testPathPatterns "lib/log.test.ts" 2>&1 | tail -10
pnpm exec tsc --noEmit 2>&1 | tail -5
```

Both PASS.

- [ ] **Step 4: Commit**

```bash
git add lib/log.ts lib/log.test.ts
git commit -m "feat(obs): browser logger stamps trace_id per entry at push time"
```

---

## Post-Phase-3 Checklist

- [ ] `LOG_LEVEL=debug` on orchestrator changes log level without rebuild
- [ ] `curl -H "X-Debug-Token: …" http://localhost:7777/debug/pprof/heap` returns a heap dump
- [ ] A deliberate panic produces a log line with a full Go stack trace
- [ ] After a desktop run, a log file exists at the OS-dependent data dir
- [ ] Redux DevTools browser extension shows at least one Zustand store
- [ ] Two consecutive `log.warn` calls with different `setTraceId` values produce per-entry traces in the ingested batch
