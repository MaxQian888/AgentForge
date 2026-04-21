# Observability Phase 1 — Correlation ID Plumbing

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Plumb one `trace_id` end-to-end — frontend → TypeScript Bridge → Go orchestrator → IM Bridge — and have every log sink (`logs` / `automation_logs` / persisted EventBus) tag its rows with it, so `SELECT * FROM logs WHERE detail->>'trace_id' = $X` returns a complete cross-service timeline.

**Architecture:** A request-scoped string `trace_id` (`tr_` + 24-char Crockford base32) is accepted or generated at each service boundary via header `X-Trace-ID`, stored on `context.Context` (Go) / `c.var.traceId` (Hono) / `window.__traceContext` (browser), and merged into every log write's JSONB `detail` and EventBus `Event.Metadata`. Existing structured loggers (`logrus`, new `pino`, new `lib/log.ts`) include `trace_id` as a first-class field. A new `POST /api/v1/internal/logs/ingest` endpoint lets the Bridge and browser write into the existing `logs` table.

**Tech Stack:** Go 1.22 / Echo / logrus; Bun / Hono / pino; Next.js 16 / React 19; Postgres (JSONB); existing `logs`, `automation_logs`, `events` tables (no schema migration).

**Source of truth:** `docs/superpowers/specs/2026-04-21-observability-system-design.md` §4 and §7.

---

## File Structure

Files created:
- `src-go/internal/log/context.go` — typed ctx helpers + trace_id generator
- `src-go/internal/log/context_test.go`
- `src-go/internal/middleware/trace.go` — Echo middleware
- `src-go/internal/middleware/trace_test.go`
- `src-go/internal/handler/ingest_handler.go` — `POST /api/v1/internal/logs/ingest`
- `src-go/internal/handler/ingest_handler_test.go`
- `src-go/internal/middleware/ratelimit_ingest.go` — per-source token bucket
- `src-go/internal/middleware/ratelimit_ingest_test.go`
- `src-bridge/src/lib/logger.ts` — pino wrapper
- `src-bridge/src/middleware/trace.ts` — Hono middleware
- `src-bridge/src/middleware/trace.test.ts`
- `src-im-bridge/internal/tracectx/context.go` — ctx helpers
- `src-im-bridge/internal/tracectx/context_test.go`
- `lib/log.ts` — browser logger with batched ingest
- `lib/log.test.ts`

Files modified:
- `src-go/internal/server/server.go` — wire trace middleware + ingest route
- `src-go/internal/service/log_service.go` — merge `trace_id` into `detail`
- `src-go/internal/service/automation_engine_service.go` — merge `trace_id` into automation log detail
- `src-go/internal/eventbus/publisher.go` — merge `trace_id` into `Event.Metadata`
- `src-go/internal/bridge/client.go` — propagate `X-Trace-ID` outbound
- `src-bridge/src/server.ts` — register trace middleware, replace console.log
- `src-bridge/package.json` — add `pino`, add ESLint `no-console`
- `src-bridge/src/plugins/tool-plugin-manager.ts`, `src/mcp/client-hub.ts`, `src/runtime/agent-runtime.ts`, `src/ws/event-stream.ts`, `src/handlers/execute.ts` — replace `console.log` with logger, propagate trace
- `src-im-bridge/client/agentforge.go` (or wherever outbound HTTP is constructed) — add `X-Trace-ID` header
- `src-im-bridge/cmd/bridge/main.go` — generate trace per inbound IM event and thread through ctx
- `lib/api-client.ts` — inject `X-Trace-ID` on outbound
- `app/error.tsx`, `app/(dashboard)/error.tsx` — call `log.error`

---

## Task 1: Go Trace Context Helpers

**Files:**
- Create: `src-go/internal/log/context.go`
- Test: `src-go/internal/log/context_test.go`

- [ ] **Step 1: Write the failing test**

```go
// src-go/internal/log/context_test.go
package log_test

import (
	"context"
	"testing"

	applog "github.com/agentforge/server/internal/log"
)

func TestTraceID_EmptyContext(t *testing.T) {
	if got := applog.TraceID(context.Background()); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestWithTrace_RoundTrip(t *testing.T) {
	ctx := applog.WithTrace(context.Background(), "tr_abc")
	if got := applog.TraceID(ctx); got != "tr_abc" {
		t.Fatalf("want tr_abc, got %q", got)
	}
}

func TestNewTraceID_FormatAndUniqueness(t *testing.T) {
	a := applog.NewTraceID()
	b := applog.NewTraceID()
	if a == b {
		t.Fatal("IDs must be unique")
	}
	if len(a) != 27 { // "tr_" + 24 chars
		t.Fatalf("want length 27, got %d (%q)", len(a), a)
	}
	if a[:3] != "tr_" {
		t.Fatalf("want prefix tr_, got %q", a[:3])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/log/... -run TestTraceID -v`
Expected: FAIL with "package ... has no test files" or "undefined: applog.TraceID"

- [ ] **Step 3: Write minimal implementation**

```go
// src-go/internal/log/context.go
// Package log exposes context-scoped correlation helpers for structured logging.
package log

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"strings"
)

type ctxKey struct{}

var traceIDKey = ctxKey{}

// TraceID returns the trace_id attached to ctx, or "" if none.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// WithTrace returns a copy of ctx carrying the given trace_id.
func WithTrace(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// crockford is Douglas Crockford's base32 alphabet (no I, L, O, U).
var crockford = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// NewTraceID returns a fresh, URL-safe trace identifier "tr_" + 24-char crockford-base32 (15 random bytes).
func NewTraceID() string {
	buf := make([]byte, 15) // 15 bytes → 24 base32 chars
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read never returns err on supported platforms; fail loud if it does.
		panic("trace id: " + err.Error())
	}
	return "tr_" + strings.ToLower(crockford.EncodeToString(buf))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/log/... -run TestTraceID -v`
Expected: PASS — 3 tests

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/log/context.go src-go/internal/log/context_test.go
git commit -m "feat(obs): add trace_id context helpers"
```

---

## Task 2: Go Trace Middleware

**Files:**
- Create: `src-go/internal/middleware/trace.go`
- Test: `src-go/internal/middleware/trace_test.go`

- [ ] **Step 1: Write the failing test**

```go
// src-go/internal/middleware/trace_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	applog "github.com/agentforge/server/internal/log"
	mw "github.com/agentforge/server/internal/middleware"
)

func TestTrace_UsesInboundHeader(t *testing.T) {
	e := echo.New()
	e.Use(mw.Trace())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, applog.TraceID(c.Request().Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "tr_inbound0000000000000000")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Body.String() != "tr_inbound0000000000000000" {
		t.Fatalf("want inbound trace, got %q", rec.Body.String())
	}
	if got := rec.Header().Get("X-Trace-ID"); got != "tr_inbound0000000000000000" {
		t.Fatalf("want echo response header, got %q", got)
	}
}

func TestTrace_GeneratesWhenMissing(t *testing.T) {
	e := echo.New()
	e.Use(mw.Trace())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, applog.TraceID(c.Request().Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.HasPrefix(body, "tr_") || len(body) != 27 {
		t.Fatalf("want generated trace_id, got %q", body)
	}
	if rec.Header().Get("X-Trace-ID") != body {
		t.Fatalf("response header must match ctx trace")
	}
}

func TestTrace_FallsBackToRequestID(t *testing.T) {
	e := echo.New()
	e.Use(mw.Trace())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, applog.TraceID(c.Request().Context()))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "req-abc")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Body.String() != "req-abc" {
		t.Fatalf("want fallback to X-Request-ID, got %q", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/middleware/... -run TestTrace -v`
Expected: FAIL with "undefined: mw.Trace"

- [ ] **Step 3: Write minimal implementation**

```go
// src-go/internal/middleware/trace.go
package middleware

import (
	"github.com/labstack/echo/v4"

	applog "github.com/agentforge/server/internal/log"
)

const traceHeader = "X-Trace-ID"

// Trace returns an Echo middleware that resolves a correlation id for every request.
// Resolution order: X-Trace-ID header → X-Request-ID header → freshly generated.
// The resolved id is attached to the request context and echoed on the response.
func Trace() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			id := req.Header.Get(traceHeader)
			if id == "" {
				id = req.Header.Get(echo.HeaderXRequestID)
			}
			if id == "" {
				id = applog.NewTraceID()
			}
			ctx := applog.WithTrace(req.Context(), id)
			c.SetRequest(req.WithContext(ctx))
			c.Response().Header().Set(traceHeader, id)
			return next(c)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/middleware/... -run TestTrace -v`
Expected: PASS — 3 tests

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/middleware/trace.go src-go/internal/middleware/trace_test.go
git commit -m "feat(obs): add trace_id Echo middleware"
```

---

## Task 3: Wire Trace Middleware + Expose trace_id to Request Logger

**Files:**
- Modify: `src-go/internal/server/server.go` (lines 39-85, middleware stack and CORS headers)

- [ ] **Step 1: Edit CORS to accept/expose X-Trace-ID**

In `src-go/internal/server/server.go`, locate the `CORSWithConfig` block and add `"X-Trace-ID"` to both `AllowHeaders` and `ExposeHeaders`:

```go
AllowHeaders:     []string{"Content-Type", "Authorization", "X-Request-ID", "X-Trace-ID", "Accept", "Accept-Language"},
ExposeHeaders:    []string{"X-Request-ID", "X-Trace-ID"},
```

- [ ] **Step 2: Register Trace middleware after RequestID, before the request logger**

In the same file, locate `e.Use(echomiddleware.RequestID())` and insert right after it:

```go
e.Use(echomiddleware.RequestID())
e.Use(appMiddleware.Trace()) // NEW — attaches trace_id to context; must precede request logger
```

Replace `appMiddleware` with whatever alias is used for `internal/middleware` in this file. If it isn't aliased yet, add the import:

```go
appMiddleware "github.com/agentforge/server/internal/middleware"
```

- [ ] **Step 3: Add trace_id to the request logger fields**

In the `LogValuesFunc`, extend `fields` to include the trace:

```go
LogValuesFunc: func(c echo.Context, v echomiddleware.RequestLoggerValues) error {
    fields := log.Fields{
        "method":     v.Method,
        "uri":        v.URI,
        "path":       c.Path(),
        "status":     v.Status,
        "latency_ms": v.Latency.Milliseconds(),
        "reqid":      v.RequestID,
        "trace_id":   applog.TraceID(c.Request().Context()), // NEW
        "remote_ip":  c.RealIP(),
    }
    // ... rest unchanged
},
```

Add import `applog "github.com/agentforge/server/internal/log"` at the top.

- [ ] **Step 4: Run vet + build + existing server tests**

```bash
cd src-go
go build ./...
go test ./internal/server/... ./internal/middleware/... -v
```

Expected: all pass. If there are pre-existing server tests asserting exact header lists, update them to include `X-Trace-ID`.

- [ ] **Step 5: Manual smoke**

```bash
cd src-go && go run ./cmd/server &
sleep 3
curl -sv -H "X-Trace-ID: tr_smoketest00000000000000" http://localhost:7777/health 2>&1 | grep -i x-trace-id
```

Expected: response header `X-Trace-ID: tr_smoketest00000000000000`. Kill the server afterward.

- [ ] **Step 6: Commit**

```bash
git add src-go/internal/server/server.go
git commit -m "feat(obs): register trace middleware and surface trace_id on request logs"
```

---

## Task 4: Ingest Rate Limiter

**Files:**
- Create: `src-go/internal/middleware/ratelimit_ingest.go`
- Test: `src-go/internal/middleware/ratelimit_ingest_test.go`

- [ ] **Step 1: Write the failing test**

```go
// src-go/internal/middleware/ratelimit_ingest_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	mw "github.com/agentforge/server/internal/middleware"
)

func TestIngestRateLimit_AllowsUnderLimit(t *testing.T) {
	e := echo.New()
	e.Use(mw.IngestRateLimit(5, 5)) // 5 req/s, burst 5
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("req %d: want 204, got %d", i, rec.Code)
		}
	}
}

func TestIngestRateLimit_Rejects429OnBurst(t *testing.T) {
	e := echo.New()
	e.Use(mw.IngestRateLimit(1, 1))
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.RemoteAddr = "10.0.0.2:1234"
		e.ServeHTTP(rec, req)
		if i == 0 && rec.Code != http.StatusNoContent {
			t.Fatalf("first req: want 204, got %d", rec.Code)
		}
		if i >= 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("req %d: want 429, got %d", i, rec.Code)
		}
	}
}

func TestIngestRateLimit_PerSourceIsolation(t *testing.T) {
	e := echo.New()
	e.Use(mw.IngestRateLimit(1, 1))
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	recA := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodPost, "/x", nil)
	reqA.RemoteAddr = "10.0.0.3:1234"
	e.ServeHTTP(recA, reqA)

	recB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodPost, "/x", nil)
	reqB.RemoteAddr = "10.0.0.4:1234"
	e.ServeHTTP(recB, reqB)

	if recA.Code != http.StatusNoContent || recB.Code != http.StatusNoContent {
		t.Fatalf("each IP gets its own bucket: A=%d B=%d", recA.Code, recB.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/middleware/... -run TestIngestRateLimit -v`
Expected: FAIL with "undefined: mw.IngestRateLimit"

- [ ] **Step 3: Write minimal implementation**

```go
// src-go/internal/middleware/ratelimit_ingest.go
package middleware

import (
	"net/http"
	"sync"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// IngestRateLimit returns a per-remote-IP token bucket limiter.
// rps is sustained rate, burst is the initial bucket depth.
// Exceeded requests get HTTP 429 with an empty body.
func IngestRateLimit(rps float64, burst int) echo.MiddlewareFunc {
	var mu sync.Mutex
	buckets := map[string]*rate.Limiter{}

	limiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := buckets[ip]
		if !ok {
			l = rate.NewLimiter(rate.Limit(rps), burst)
			buckets[ip] = l
		}
		return l
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !limiter(c.RealIP()).Allow() {
				return c.NoContent(http.StatusTooManyRequests)
			}
			return next(c)
		}
	}
}
```

Ensure `golang.org/x/time/rate` is in go.mod; if not, `go get golang.org/x/time@latest`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/middleware/... -run TestIngestRateLimit -v`
Expected: PASS — 3 tests

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/middleware/ratelimit_ingest.go src-go/internal/middleware/ratelimit_ingest_test.go go.mod go.sum
git commit -m "feat(obs): per-source token-bucket limiter for ingest endpoint"
```

---

## Task 5: Ingest Handler

**Files:**
- Create: `src-go/internal/handler/ingest_handler.go`
- Test: `src-go/internal/handler/ingest_handler_test.go`

This handler writes incoming log lines from the Bridge or browser into the existing `logs` table via the existing `LogService.CreateLog`.

- [ ] **Step 1: Write the failing test**

```go
// src-go/internal/handler/ingest_handler_test.go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
)

type stubLogService struct{ got model.CreateLogInput }

func (s *stubLogService) CreateLog(_ context.Context, in model.CreateLogInput) (*model.Log, error) {
	s.got = in
	return &model.Log{ID: uuid.New(), ProjectID: in.ProjectID}, nil
}

func TestIngest_PersistsWithTraceID(t *testing.T) {
	svc := &stubLogService{}
	h := handler.NewIngestHandler(svc)

	projectID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"projectId": projectID.String(),
		"tab":       "system",
		"level":     "warn",
		"source":    "ts-bridge",
		"summary":   "plugin load failed",
		"detail":    map[string]any{"plugin": "acme"},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trace-ID", "tr_fromclient000000000000")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Simulate the Trace middleware having run.
	c.SetRequest(req.WithContext(
		/* applog.WithTrace replaced by helper imported in real code */
		contextWithTrace(req.Context(), "tr_fromclient000000000000"),
	))

	if err := h.Ingest(c); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", rec.Code)
	}
	if svc.got.Summary != "plugin load failed" {
		t.Fatalf("want summary pass-through, got %q", svc.got.Summary)
	}
	if svc.got.Detail["trace_id"] != "tr_fromclient000000000000" {
		t.Fatalf("detail must carry trace_id; got %+v", svc.got.Detail)
	}
	if svc.got.Detail["source"] != "ts-bridge" {
		t.Fatalf("detail must carry source; got %+v", svc.got.Detail)
	}
}

func TestIngest_RejectsInvalidLevel(t *testing.T) {
	svc := &stubLogService{}
	h := handler.NewIngestHandler(svc)
	body, _ := json.Marshal(map[string]any{
		"projectId": uuid.New().String(),
		"tab":       "system",
		"level":     "bogus",
		"summary":   "x",
	})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	if err := h.Ingest(e.NewContext(req, rec)); err == nil && rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (err=%v)", rec.Code, err)
	}
}

// contextWithTrace is a test helper that mirrors applog.WithTrace without a cyclic import.
func contextWithTrace(ctx context.Context, id string) context.Context {
	type k struct{}
	return context.WithValue(ctx, k{}, id)
}
```

> **Note:** the last helper uses a different key type from the real `applog.WithTrace`, which will make one assertion fail. Fix this in Step 3 by importing `applog` directly and using `applog.WithTrace(req.Context(), "tr_…")`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/handler/... -run TestIngest -v`
Expected: FAIL with "undefined: handler.NewIngestHandler" (or "undefined: handler.IngestHandler")

- [ ] **Step 3: Write minimal implementation AND fix the test helper**

Fix the test by replacing `contextWithTrace(...)` with the real helper:

```go
// top of test file
import (
    applog "github.com/agentforge/server/internal/log"
)
// … in TestIngest_PersistsWithTraceID:
c.SetRequest(req.WithContext(applog.WithTrace(req.Context(), "tr_fromclient000000000000")))
```

Remove the local `contextWithTrace` helper.

Define the service interface + handler:

```go
// src-go/internal/handler/ingest_handler.go
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
)

// LogCreator is the narrow slice of LogService this handler needs.
type LogCreator interface {
	CreateLog(ctx context.Context, in model.CreateLogInput) (*model.Log, error)
}

type IngestHandler struct{ svc LogCreator }

func NewIngestHandler(svc LogCreator) *IngestHandler { return &IngestHandler{svc: svc} }

type ingestRequest struct {
	ProjectID string                 `json:"projectId"`
	Tab       string                 `json:"tab"`
	Level     string                 `json:"level"`
	Source    string                 `json:"source"`
	Summary   string                 `json:"summary"`
	Detail    map[string]any         `json:"detail,omitempty"`
	EventType string                 `json:"eventType,omitempty"`
	Action    string                 `json:"action,omitempty"`
}

var allowedLevels = map[string]struct{}{
	model.LogLevelDebug: {},
	model.LogLevelInfo:  {},
	model.LogLevelWarn:  {},
	model.LogLevelError: {},
}

func (h *IngestHandler) Ingest(c echo.Context) error {
	var req ingestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
	}
	if _, ok := allowedLevels[req.Level]; !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid level"})
	}
	if req.Tab == "" {
		req.Tab = model.LogTabSystem
	}
	if req.Summary == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "summary required"})
	}

	detail := req.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	if tid := applog.TraceID(c.Request().Context()); tid != "" {
		detail["trace_id"] = tid
	}
	if req.Source != "" {
		detail["source"] = req.Source
	}

	var projectID uuid.UUID
	if req.ProjectID != "" {
		id, err := uuid.Parse(req.ProjectID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid projectId"})
		}
		projectID = id
	}

	in := model.CreateLogInput{
		ProjectID: projectID,
		Tab:       req.Tab,
		Level:     req.Level,
		ActorType: "service",
		ActorID:   req.Source,
		EventType: req.EventType,
		Action:    req.Action,
		Summary:   req.Summary,
		Detail:    detail,
	}
	if _, err := h.svc.CreateLog(c.Request().Context(), in); err != nil {
		if errors.Is(err, context.Canceled) {
			return c.NoContent(http.StatusRequestTimeout)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "ingest failed"})
	}
	return c.NoContent(http.StatusAccepted)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/handler/... -run TestIngest -v`
Expected: PASS — 2 tests

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/handler/ingest_handler.go src-go/internal/handler/ingest_handler_test.go
git commit -m "feat(obs): add POST /api/v1/internal/logs/ingest handler"
```

---

## Task 6: Wire Ingest Route

**Files:**
- Modify: `src-go/internal/server/server.go` (route registration)

- [ ] **Step 1: Find where routes are registered**

```bash
grep -n "api/v1" src-go/internal/server/server.go | head -20
```

Locate the block that constructs handlers and registers API routes, typically near the end of `NewServer` or in a dedicated `routes.go`.

- [ ] **Step 2: Construct the handler and register the route under a rate-limited internal group**

Within the route registration, add:

```go
import (
    // existing imports...
    "github.com/agentforge/server/internal/handler"
    appMiddleware "github.com/agentforge/server/internal/middleware"
)

// … where you wire handlers:
ingestH := handler.NewIngestHandler(logSvc) // logSvc is the existing *service.LogService

internalGroup := e.Group("/api/v1/internal", appMiddleware.IngestRateLimit(100, 200))
internalGroup.POST("/logs/ingest", ingestH.Ingest)
```

> **Auth note:** the ingest endpoint is called by the Bridge (server-side) and the browser. The Bridge uses an API token shared via environment; the browser uses the existing session cookie. Reuse whatever middleware already guards `/api/v1/internal/*` routes. If no such group exists, put the new group behind the existing auth middleware that protects other `/api/v1` routes.

- [ ] **Step 3: Integration test**

Add to `src-go/internal/server/server_test.go` (create file if absent):

```go
// src-go/internal/server/server_test.go (append)
func TestIngest_EndToEnd(t *testing.T) {
    // Build server with a stub LogService, POST to /api/v1/internal/logs/ingest,
    // assert 202 and that the stub captured trace_id in detail.
    // Use the project's existing test-harness pattern.
}
```

> Prefer re-using the existing test harness (look for `NewTestServer` / `setupTestEcho` helpers); copy the pattern used by another integration test in the same package. No need to reinvent it.

- [ ] **Step 4: Run build + all tests**

```bash
cd src-go && go build ./... && go test ./...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/server/server.go src-go/internal/server/server_test.go
git commit -m "feat(obs): wire /api/v1/internal/logs/ingest with rate limit"
```

---

## Task 7: Merge trace_id Into log_service.CreateLog

**Files:**
- Modify: `src-go/internal/service/log_service.go` (the `CreateLog` method)
- Modify: `src-go/internal/service/log_service_test.go` (add test; create file if absent)

- [ ] **Step 1: Write the failing test**

```go
// src-go/internal/service/log_service_test.go
package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
)

type fakeLogRepo struct{ last *model.Log }

func (r *fakeLogRepo) Create(_ context.Context, log *model.Log) error {
	r.last = log
	return nil
}

func TestCreateLog_MergesTraceIDIntoDetail(t *testing.T) {
	repo := &fakeLogRepo{}
	svc := service.NewLogService(repo, nil) // nil eventbus is OK for this assertion

	ctx := applog.WithTrace(context.Background(), "tr_service00000000000000")
	in := model.CreateLogInput{
		ProjectID: uuid.New(),
		Tab:       model.LogTabSystem,
		Level:     model.LogLevelInfo,
		Summary:   "x",
		Detail:    map[string]any{"foo": "bar"},
	}
	if _, err := svc.CreateLog(ctx, in); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(repo.last.Detail, &got); err != nil {
		t.Fatal(err)
	}
	if got["trace_id"] != "tr_service00000000000000" {
		t.Fatalf("missing trace_id in detail: %+v", got)
	}
	if got["foo"] != "bar" {
		t.Fatalf("preexisting detail clobbered: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/service/... -run TestCreateLog_Merges -v`
Expected: FAIL — "missing trace_id in detail"

- [ ] **Step 3: Modify `CreateLog`**

In `src-go/internal/service/log_service.go`, within `CreateLog` just before `detailJSON, _ := json.Marshal(input.Detail)`:

```go
if tid := applog.TraceID(ctx); tid != "" {
    if input.Detail == nil {
        input.Detail = map[string]any{}
    }
    input.Detail["trace_id"] = tid
}
```

Add import `applog "github.com/agentforge/server/internal/log"` at the top.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src-go && go test ./internal/service/... -run TestCreateLog -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/service/log_service.go src-go/internal/service/log_service_test.go
git commit -m "feat(obs): log_service merges trace_id into detail"
```

---

## Task 8: Merge trace_id Into Automation Engine Log Writes

**Files:**
- Modify: `src-go/internal/service/automation_engine_service.go`

Automation log writes use the `automation_logs` table directly via an `AutomationLogRepository`. The fix mirrors Task 7: where the service builds the `Detail` JSON for an `AutomationLog`, merge `trace_id` from `ctx`.

- [ ] **Step 1: Locate the write site**

```bash
grep -n "automationLogRepo\|AutomationLog{" src-go/internal/service/automation_engine_service.go
```

- [ ] **Step 2: Write the failing test**

Follow the existing automation-engine test style (look for `automation_engine_service_test.go`). Add a test asserting that after running a rule, `repo.last.Detail` decodes to a map containing `"trace_id": "tr_auto…"`. If no test file exists, create `automation_engine_service_trace_test.go` in the same package.

- [ ] **Step 3: Modify the write site**

Wherever the service constructs `model.AutomationLog{... Detail: ...}`, build a map first and merge:

```go
detail := map[string]any{ /* existing fields */ }
if tid := applog.TraceID(ctx); tid != "" {
    detail["trace_id"] = tid
}
raw, _ := json.Marshal(detail)
// then use `raw` for AutomationLog.Detail
```

Add import `applog "github.com/agentforge/server/internal/log"`.

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/service/... -run Automation -v
```

Expected: PASS including the new test.

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/service/automation_engine_service.go src-go/internal/service/automation_engine_service_trace_test.go
git commit -m "feat(obs): automation engine writes trace_id into automation_logs.detail"
```

---

## Task 9: Merge trace_id Into EventBus Publish

**Files:**
- Modify: `src-go/internal/eventbus/publisher.go` (or wherever `Publish` is implemented)
- Modify: existing publisher test (or create one)

The persisted-events observer serializes `Event.Metadata` as JSONB. Merging `trace_id` there gives the Phase 2 timeline one source to query.

- [ ] **Step 1: Write the failing test**

```go
// src-go/internal/eventbus/publisher_trace_test.go
package eventbus_test

import (
	"context"
	"testing"

	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/eventbus"
)

type recorderSink struct{ last *eventbus.Event }

func (r *recorderSink) Publish(_ context.Context, e *eventbus.Event) error {
	r.last = e
	return nil
}

func TestPublish_AttachesTraceIDToMetadata(t *testing.T) {
	rec := &recorderSink{}
	pub := eventbus.NewPublisherWithSink(rec) // adjust to actual constructor

	ctx := applog.WithTrace(context.Background(), "tr_bus0000000000000000000000")
	err := pub.Publish(ctx, &eventbus.Event{Type: "task.created", Source: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.last.Metadata["trace_id"] != "tr_bus0000000000000000000000" {
		t.Fatalf("missing trace_id in metadata: %+v", rec.last.Metadata)
	}
}
```

> **Constructor note:** if the existing publisher exposes no factory that accepts a sink, either add one for testing or assert against an existing test-only helper. Match the package's existing test pattern.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src-go && go test ./internal/eventbus/... -run TestPublish_AttachesTraceID -v`
Expected: FAIL

- [ ] **Step 3: Modify the publish path**

In the publisher implementation, before forwarding the event to subscribers/persistence:

```go
func (p *publisher) Publish(ctx context.Context, e *Event) error {
    if tid := applog.TraceID(ctx); tid != "" {
        if e.Metadata == nil {
            e.Metadata = map[string]any{}
        }
        if _, set := e.Metadata["trace_id"]; !set {
            e.Metadata["trace_id"] = tid
        }
    }
    // ... existing publish logic
}
```

Add import `applog "github.com/agentforge/server/internal/log"`.

- [ ] **Step 4: Verify test passes + run full eventbus tests**

```bash
cd src-go && go test ./internal/eventbus/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/eventbus/publisher.go src-go/internal/eventbus/publisher_trace_test.go
git commit -m "feat(obs): eventbus publish attaches trace_id to Event.Metadata"
```

---

## Task 10: Bridge Client Propagates X-Trace-ID

**Files:**
- Modify: `src-go/internal/bridge/client.go`

- [ ] **Step 1: Find the central request builder**

All outbound methods build `http.NewRequestWithContext(...)` and set `Content-Type`. Locate each occurrence. Count should be small (≤10).

```bash
grep -n "http.NewRequestWithContext" src-go/internal/bridge/client.go
```

- [ ] **Step 2: Extract a tiny helper and use it everywhere**

Add to `client.go`:

```go
import (
    applog "github.com/agentforge/server/internal/log"
)

// attachTraceHeader copies the ctx trace_id onto req if present. No-op otherwise.
func attachTraceHeader(req *http.Request) {
    if tid := applog.TraceID(req.Context()); tid != "" {
        req.Header.Set("X-Trace-ID", tid)
    }
}
```

In every place that reads `httpReq.Header.Set("Content-Type", "application/json")`, add the call:

```go
httpReq.Header.Set("Content-Type", "application/json")
attachTraceHeader(httpReq) // NEW
```

- [ ] **Step 3: Write the test**

Add to `src-go/internal/bridge/client_test.go` (or an appropriate existing test file):

```go
func TestExecute_SendsTraceHeader(t *testing.T) {
    got := ""
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        got = r.Header.Get("X-Trace-ID")
        _ = json.NewEncoder(w).Encode(bridge.ExecuteResponse{})
    }))
    defer srv.Close()

    c := bridge.NewClient(srv.URL, http.DefaultClient)
    ctx := applog.WithTrace(context.Background(), "tr_outbound00000000000000")
    _, err := c.Execute(ctx, bridge.ExecuteRequest{})
    if err != nil { t.Fatal(err) }
    if got != "tr_outbound00000000000000" {
        t.Fatalf("want outbound header, got %q", got)
    }
}
```

Adjust the constructor name / signature to match the existing `client.go`.

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/bridge/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/bridge/client.go src-go/internal/bridge/client_test.go
git commit -m "feat(obs): bridge client propagates X-Trace-ID outbound"
```

---

## Task 10b: Background-Job Trace Generation (Go)

**Files:**
- Modify: `src-go/internal/scheduler/` (tick / job-execution entry point)
- Modify: `src-go/internal/service/automation_engine_service.go` (event-driven dispatch entry)
- Modify: any `EventDrivenExecutor` / `HierarchicalExecutor` tick-handler in `internal/service/`

Per spec §4.3: background jobs (scheduler, event-driven executors, automation rules) have no inbound HTTP request to inherit a trace from. They must generate one at the top of each job/handler so their downstream writes are correlatable.

- [ ] **Step 1: Locate each background entry point**

```bash
grep -rn "func.*\(ctx context\.Context" src-go/internal/scheduler/ src-go/internal/service/ | grep -iE "(tick|run|dispatch|execute|process)" | head -20
```

You are looking for functions that start their own `context.Background()` or receive a short-lived ctx from a timer. Typical candidates:
- `scheduler/ticker.go` (or similar) — cron tick
- `service/automation_engine_service.go` — `HandleEvent` / `executeStartWorkflow`
- `service/event_driven_executor.go` — event-bus subscriber callback
- `service/hierarchical_executor.go` — step dispatcher

- [ ] **Step 2: Wrap each entry with a fresh trace**

At the top of each located function, add:

```go
if applog.TraceID(ctx) == "" {
    ctx = applog.WithTrace(ctx, applog.NewTraceID())
    log.WithFields(log.Fields{
        "trace_id": applog.TraceID(ctx),
        "origin":   "scheduler.tick", // or "automation.rule", "event.subscriber", "workflow.step"
    }).Info("trace.generated_for_background_job")
}
```

Add imports as needed:
```go
applog "github.com/agentforge/server/internal/log"
log "github.com/sirupsen/logrus"
```

Use a distinct `origin` value per entry so operators can tell which subsystem created each trace.

- [ ] **Step 3: Test at one representative site**

Pick the scheduler tick and add a table test asserting that after the tick handler runs, downstream `logrus` output contained a `trace_id` field. If existing tests don't make this easy, spy by replacing the logrus output:

```go
func TestSchedulerTick_AssignsTraceID(t *testing.T) {
    var buf bytes.Buffer
    old := log.StandardLogger().Out
    log.SetOutput(&buf)
    t.Cleanup(func() { log.SetOutput(old) })

    // ... invoke the tick handler with a fresh ctx ...

    if !strings.Contains(buf.String(), `"trace_id":"tr_`) {
        t.Fatalf("scheduler tick did not emit a trace_id; log=%s", buf.String())
    }
}
```

- [ ] **Step 4: Run**

```bash
cd src-go && go build ./... && go test ./internal/scheduler/... ./internal/service/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/scheduler/ src-go/internal/service/
git commit -m "feat(obs): background jobs generate trace_id at entry points"
```

---

## Task 11: TS Bridge — Install pino + Logger Wrapper

**Files:**
- Modify: `src-bridge/package.json`
- Create: `src-bridge/src/lib/logger.ts`
- Create: `src-bridge/src/lib/logger.test.ts`

- [ ] **Step 1: Add pino dependency**

```bash
cd src-bridge && bun add pino
```

Verify `package.json` now lists `"pino": "^x.y.z"` under `dependencies`.

- [ ] **Step 2: Write the failing test**

```ts
// src-bridge/src/lib/logger.test.ts
import { describe, it, expect } from "bun:test";
import { createLogger, withTrace } from "./logger.js";

describe("logger", () => {
  it("includes trace_id when withTrace is used", () => {
    const captured: string[] = [];
    const base = createLogger({ level: "info", write: (s) => captured.push(s) });
    const child = withTrace(base, "tr_abc");
    child.info({ event: "unit.test" }, "hello");
    expect(captured.length).toBe(1);
    const parsed = JSON.parse(captured[0]);
    expect(parsed.trace_id).toBe("tr_abc");
    expect(parsed.event).toBe("unit.test");
    expect(parsed.msg).toBe("hello");
  });

  it("omits trace_id when none bound", () => {
    const captured: string[] = [];
    const base = createLogger({ level: "info", write: (s) => captured.push(s) });
    base.info({ event: "unit.test" }, "hello");
    const parsed = JSON.parse(captured[0]);
    expect(parsed.trace_id).toBeUndefined();
  });
});
```

- [ ] **Step 3: Run to fail**

```bash
cd src-bridge && bun test src/lib/logger.test.ts
```

Expected: FAIL (module not found).

- [ ] **Step 4: Implement**

```ts
// src-bridge/src/lib/logger.ts
import pino from "pino";
import type { Logger } from "pino";

export interface CreateLoggerOpts {
  level?: "debug" | "info" | "warn" | "error";
  write?: (chunk: string) => void;
}

export function createLogger(opts: CreateLoggerOpts = {}): Logger {
  const level = opts.level ?? (process.env.LOG_LEVEL as CreateLoggerOpts["level"]) ?? "info";
  const destination = opts.write
    ? { write: opts.write }
    : undefined;
  return pino(
    { level, base: { service: "ts-bridge" } },
    destination ? (pino as unknown as { destination: (d: unknown) => unknown }).destination(destination) : undefined,
  );
}

export function withTrace(logger: Logger, traceId: string): Logger {
  return logger.child({ trace_id: traceId });
}
```

- [ ] **Step 5: Run to pass**

```bash
cd src-bridge && bun test src/lib/logger.test.ts
```

Expected: PASS — 2 tests.

- [ ] **Step 6: Commit**

```bash
git add src-bridge/package.json src-bridge/bun.lock src-bridge/src/lib/logger.ts src-bridge/src/lib/logger.test.ts
git commit -m "feat(obs): add pino logger with trace_id child support in ts-bridge"
```

---

## Task 12: TS Bridge — Trace Middleware

**Files:**
- Create: `src-bridge/src/middleware/trace.ts`
- Create: `src-bridge/src/middleware/trace.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// src-bridge/src/middleware/trace.test.ts
import { describe, it, expect } from "bun:test";
import { Hono } from "hono";
import { traceMiddleware } from "./trace.js";

describe("traceMiddleware", () => {
  it("uses inbound X-Trace-ID when present", async () => {
    const app = new Hono();
    app.use("*", traceMiddleware());
    app.get("/", (c) => c.text(c.get("traceId") ?? ""));
    const res = await app.request("/", { headers: { "X-Trace-ID": "tr_inbound" } });
    expect(await res.text()).toBe("tr_inbound");
    expect(res.headers.get("X-Trace-ID")).toBe("tr_inbound");
  });

  it("generates a trace_id when missing", async () => {
    const app = new Hono();
    app.use("*", traceMiddleware());
    app.get("/", (c) => c.text(c.get("traceId") ?? ""));
    const res = await app.request("/");
    const body = await res.text();
    expect(body.startsWith("tr_")).toBe(true);
    expect(body.length).toBe(27);
    expect(res.headers.get("X-Trace-ID")).toBe(body);
  });
});
```

- [ ] **Step 2: Run to fail**

```bash
cd src-bridge && bun test src/middleware/trace.test.ts
```

Expected: FAIL (module not found)

- [ ] **Step 3: Implement**

```ts
// src-bridge/src/middleware/trace.ts
import type { MiddlewareHandler } from "hono";
import { randomBytes } from "node:crypto";

const CROCKFORD = "0123456789abcdefghjkmnpqrstvwxyz";

function newTraceId(): string {
  const bytes = randomBytes(15);
  let out = "";
  // 15 bytes → 24 base32 chars. Simple custom encoder to match Go side.
  let bits = 0;
  let buf = 0;
  for (let i = 0; i < bytes.length; i++) {
    buf = (buf << 8) | bytes[i]!;
    bits += 8;
    while (bits >= 5) {
      bits -= 5;
      out += CROCKFORD[(buf >> bits) & 0x1f];
    }
  }
  if (bits > 0) out += CROCKFORD[(buf << (5 - bits)) & 0x1f];
  return "tr_" + out.slice(0, 24);
}

declare module "hono" {
  interface ContextVariableMap {
    traceId: string;
  }
}

export function traceMiddleware(opts: { onMidchain?: (id: string) => void } = {}): MiddlewareHandler {
  return async (c, next) => {
    const inbound = c.req.header("X-Trace-ID") ?? c.req.header("X-Request-ID") ?? "";
    const id = inbound || newTraceId();
    if (!inbound && opts.onMidchain) {
      // ts-bridge is never called by a human directly — every caller is expected to forward a trace.
      // A missing inbound trace means an upstream dropped it; surface it once per request.
      opts.onMidchain(id);
    }
    c.set("traceId", id);
    c.header("X-Trace-ID", id);
    await next();
  };
}
```

When wiring in Task 13, pass `onMidchain` to log the warning once:

```ts
app.use("*", traceMiddleware({
  onMidchain: (id) => log.warn({ trace_id: id }, "trace.generated_midchain"),
}));
```

- [ ] **Step 4: Run to pass**

```bash
cd src-bridge && bun test src/middleware/trace.test.ts
```

Expected: PASS — 2 tests.

- [ ] **Step 5: Commit**

```bash
git add src-bridge/src/middleware/trace.ts src-bridge/src/middleware/trace.test.ts
git commit -m "feat(obs): trace middleware for ts-bridge Hono app"
```

---

## Task 13: TS Bridge — Wire Middleware + Replace server.ts console.log

**Files:**
- Modify: `src-bridge/src/server.ts`

- [ ] **Step 1: Register middleware + create server-scoped logger**

Near the top of `src-bridge/src/server.ts`, after the Hono `app` is created (search for `const app = new Hono()` or equivalent — if it's created inside `buildApp`, place this inside that function):

```ts
import { createLogger, withTrace } from "./lib/logger.js";
import { traceMiddleware } from "./middleware/trace.js";

const log = createLogger();
app.use("*", traceMiddleware());
app.use("*", async (c, next) => {
  const reqLog = withTrace(log, c.get("traceId"));
  c.set("log", reqLog as unknown as typeof log);
  const start = Date.now();
  await next();
  reqLog.info(
    { method: c.req.method, path: c.req.path, status: c.res.status, ms: Date.now() - start },
    "request",
  );
});
```

Extend the Hono context typing (add to the module augmentation in `middleware/trace.ts`):

```ts
declare module "hono" {
  interface ContextVariableMap {
    traceId: string;
    log: import("pino").Logger;
  }
}
```

- [ ] **Step 2: Replace every `console.log` / `console.error` in server.ts with `log.info` / `log.error`**

```bash
grep -n "console\." src-bridge/src/server.ts
```

For each hit, convert:
- `console.log("[Bridge] Starting on port", port)` → `log.info({ port }, "bridge starting")`
- `console.error("[Bridge] Failed to save snapshot", err)` → `log.error({ err }, "snapshot save failed")`

If the call site has a request context available, prefer `c.var.log` (the per-request child).

- [ ] **Step 3: Verify with typecheck + start**

```bash
cd src-bridge && bun run typecheck
bun run dev &
sleep 3
curl -sv -H "X-Trace-ID: tr_bridgesmoke0000000000000" http://localhost:7778/health 2>&1 | grep -i x-trace-id
pkill -f "bun run src/server.ts"
```

Expected: response header echoes `tr_bridgesmoke0000000000000`; stdout shows a JSON log line with `"trace_id":"tr_bridgesmoke0000000000000"`.

- [ ] **Step 4: Commit**

```bash
git add src-bridge/src/server.ts src-bridge/src/middleware/trace.ts
git commit -m "feat(obs): wire trace middleware and per-request logger in ts-bridge server"
```

---

## Task 14: TS Bridge — Replace console.log Across Remaining Modules

**Files:**
- Modify: every `src-bridge/src/**/*.ts` using `console.*` (non-test)

- [ ] **Step 1: Enumerate hits**

```bash
cd src-bridge && grep -rn "console\." src/ --include="*.ts" | grep -v "\.test\.ts"
```

- [ ] **Step 2: For each hit, replace with a logger**

Rules:
- If the call site is inside a Hono handler, use `c.var.log` (already trace-scoped).
- If the module has no context, import `createLogger` at top of file: `const log = createLogger().child({ module: "plugins" });`
- For WebSocket or event-stream callbacks (`src/ws/event-stream.ts`), accept a `logger` argument in the constructor and have the caller pass `withTrace(log, traceId)` from the Hono handler that created the stream.

Touch these files (known hot spots):
- `src/ws/event-stream.ts`
- `src/runtime/agent-runtime.ts`
- `src/runtime/*.ts`
- `src/plugins/tool-plugin-manager.ts`
- `src/mcp/client-hub.ts`
- `src/handlers/execute.ts`
- `src/handlers/claude-runtime.ts`
- `src/handlers/codex-runtime.ts`

- [ ] **Step 3: Add ESLint `no-console` rule**

Find the lint config (`src-bridge/eslint.config.js` or `.eslintrc.*`). If it doesn't exist, add the smallest config that the existing `pnpm lint` script runs:

```js
// src-bridge/eslint.config.js (augment existing)
export default [
  // ... existing config
  {
    files: ["src/**/*.ts"],
    ignores: ["src/**/*.test.ts"],
    rules: {
      "no-console": "error",
    },
  },
];
```

- [ ] **Step 4: Run lint + tests**

```bash
cd src-bridge && bun run typecheck && bun test
# If root pnpm lint covers bridge, also:
cd .. && pnpm lint
```

Expected: zero `no-console` violations; all tests pass.

- [ ] **Step 5: Commit**

```bash
git add src-bridge/
git commit -m "refactor(obs): replace console.log with pino logger across ts-bridge"
```

---

## Task 15: TS Bridge — Outbound X-Trace-ID Propagation

**Files:**
- Modify: every outbound `fetch(...)` call in `src-bridge/src/**/*.ts`

The Bridge calls the Go orchestrator (e.g., for ingest, for session callbacks) and LLM providers. Go-orchestrator calls need `X-Trace-ID`; third-party API calls do not.

- [ ] **Step 1: Find orchestrator-bound fetch calls**

```bash
cd src-bridge && grep -rn "fetch(" src/ --include="*.ts" | grep -v "\.test\.ts"
```

Classify each: calls to `process.env.ORCHESTRATOR_URL` (or similar) → add header; calls to LLM APIs → leave alone.

- [ ] **Step 2: Introduce a tiny helper**

Create `src-bridge/src/lib/orchestrator-fetch.ts`:

```ts
import type { Context } from "hono";

const ORCHESTRATOR_URL = process.env.ORCHESTRATOR_URL ?? "http://localhost:7777";

export async function orchestratorFetch(
  c: Context | { var: { traceId: string } },
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  const headers = new Headers(init.headers);
  headers.set("X-Trace-ID", c.var.traceId);
  return fetch(`${ORCHESTRATOR_URL}${path}`, { ...init, headers });
}
```

- [ ] **Step 3: Replace orchestrator-bound fetches with `orchestratorFetch`**

For each call site that targets the orchestrator, swap `fetch(url, init)` → `orchestratorFetch(c, path, init)`.

- [ ] **Step 4: Add ingest call on selected high-signal events**

In `src/plugins/tool-plugin-manager.ts` wherever a plugin load/unload/error happens, after logging locally, send to the orchestrator:

```ts
void orchestratorFetch(c, "/api/v1/internal/logs/ingest", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    tab: "system",
    level: "warn",
    source: "ts-bridge",
    summary: "plugin load failed",
    detail: { plugin: name, err: String(err) },
  }),
}).catch(() => { /* never block on ingest failure */ });
```

Apply the same pattern in: runtime spawn/exit failure (`src/runtime/*`), MCP call failure (`src/mcp/client-hub.ts`).

- [ ] **Step 5: Typecheck + test**

```bash
cd src-bridge && bun run typecheck && bun test
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add src-bridge/src/lib/orchestrator-fetch.ts src-bridge/src/
git commit -m "feat(obs): propagate X-Trace-ID and ingest high-signal events from ts-bridge"
```

---

## Task 16: IM Bridge — Generate trace_id Per Inbound Event

**Files:**
- Create: `src-im-bridge/internal/tracectx/context.go`
- Create: `src-im-bridge/internal/tracectx/context_test.go`
- Modify: the inbound-event entry points in `src-im-bridge/cmd/bridge/main.go` (search for webhook/poll handler registration — `platform_registry.go` and related)

- [ ] **Step 1: Write context helpers (mirror Go orchestrator Task 1)**

```go
// src-im-bridge/internal/tracectx/context.go
package tracectx

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"strings"
)

type ctxKey struct{}

var key = ctxKey{}

func TraceID(ctx context.Context) string {
	if v, ok := ctx.Value(key).(string); ok {
		return v
	}
	return ""
}

func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key, id)
}

var crockford = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

func New() string {
	buf := make([]byte, 15)
	_, _ = rand.Read(buf)
	return "tr_" + strings.ToLower(crockford.EncodeToString(buf))
}
```

```go
// src-im-bridge/internal/tracectx/context_test.go
package tracectx_test

import (
	"context"
	"testing"

	"github.com/agentforge/im-bridge/internal/tracectx"
)

func TestRoundTrip(t *testing.T) {
	ctx := tracectx.With(context.Background(), "tr_x")
	if got := tracectx.TraceID(ctx); got != "tr_x" {
		t.Fatalf("got %q", got)
	}
}

func TestNew(t *testing.T) {
	if len(tracectx.New()) != 27 {
		t.Fatal("length")
	}
}
```

- [ ] **Step 2: Run tests**

```bash
cd src-im-bridge && go test ./internal/tracectx/... -v
```

Expected: PASS

- [ ] **Step 3: Attach trace_id at inbound event entry points**

In `src-im-bridge/cmd/bridge/main.go` (and wherever IM platform events enter — platform_registry, notify/*), at the top of each event-handling function:

```go
ctx = tracectx.With(ctx, tracectx.New())
log.WithField("trace_id", tracectx.TraceID(ctx)).Info("inbound im event")
```

> Do not change call signatures of existing methods — the ctx is already threaded through. Add the two lines at the handler's entry.

**Control plane HTTP callbacks (orchestrator → im-bridge):** if `src-im-bridge/cmd/bridge/control_plane.go` exposes any HTTP handler (orchestrator-originated callbacks such as reply dispatch or action relay), it receives `X-Trace-ID` from the orchestrator. At the top of each such handler, do:

```go
tid := r.Header.Get("X-Trace-ID")
if tid == "" { tid = tracectx.New() }
ctx := tracectx.With(r.Context(), tid)
r = r.WithContext(ctx)
w.Header().Set("X-Trace-ID", tid)
```

If control_plane.go uses a router (mux/chi/echo), add a per-router middleware instead of editing each handler. One middleware function, registered once.

- [ ] **Step 4: Build + existing tests**

```bash
cd src-im-bridge && go build ./... && go test ./...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src-im-bridge/internal/tracectx/ src-im-bridge/cmd/bridge/main.go
git commit -m "feat(obs): generate trace_id per inbound im event"
```

---

## Task 17: IM Bridge — Outbound AgentForgeClient Sends X-Trace-ID

**Files:**
- Modify: `src-im-bridge/client/agentforge.go` (or whichever file constructs outbound HTTP — grep confirms)

- [ ] **Step 1: Locate the request builder**

```bash
grep -rn "http.NewRequestWithContext\|http.NewRequest" src-im-bridge/client/
```

- [ ] **Step 2: Attach the header in one central helper**

If the client has a `func (c *AgentForgeClient) do(ctx, method, path, body …)` helper, add right before `c.http.Do(req)`:

```go
if tid := tracectx.TraceID(ctx); tid != "" {
    req.Header.Set("X-Trace-ID", tid)
}
```

If no central helper exists, add it at every `NewRequestWithContext` site. Prefer centralizing if it takes <20 lines.

Add import `"github.com/agentforge/im-bridge/internal/tracectx"`.

- [ ] **Step 3: Add a test using httptest.Server**

Pattern:

```go
func TestAgentForgeClient_SendsTraceHeader(t *testing.T) {
    got := ""
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        got = r.Header.Get("X-Trace-ID")
        w.WriteHeader(200)
    }))
    defer srv.Close()
    c := client.NewAgentForgeClient(srv.URL, "token")
    ctx := tracectx.With(context.Background(), "tr_imout0000000000000000000")
    _ = c.PostReaction(ctx, client.ReactionEvent{}) // or whatever method is simplest
    if got != "tr_imout0000000000000000000" { t.Fatalf("got %q", got) }
}
```

Use the simplest public method that triggers a real request.

- [ ] **Step 4: Run tests**

```bash
cd src-im-bridge && go test ./client/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src-im-bridge/client/
git commit -m "feat(obs): im-bridge outbound client propagates X-Trace-ID"
```

---

## Task 18: Frontend — Browser Logger with Batched Ingest

**Files:**
- Create: `lib/log.ts`
- Create: `lib/log.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// lib/log.test.ts
import { describe, it, expect, jest } from "@jest/globals";
import { createBrowserLogger } from "./log";

describe("browser logger", () => {
  it("batches and POSTs warn+ to /api/v1/internal/logs/ingest", async () => {
    const fetchMock = jest.fn(async () => new Response(null, { status: 202 }));
    const log = createBrowserLogger({
      fetch: fetchMock as unknown as typeof fetch,
      flushMs: 10,
      bufferSize: 10,
      traceId: () => "tr_front000000000000000000",
    });
    log.info("ignored", { k: 1 });
    log.warn("kept", { k: 2 });
    log.error("kept2", { k: 3 });

    await new Promise((r) => setTimeout(r, 30));

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/api/v1/internal/logs/ingest");
    expect((init.headers as Record<string, string>)["X-Trace-ID"]).toBe("tr_front000000000000000000");
    const body = JSON.parse(String(init.body));
    expect(Array.isArray(body)).toBe(true);
    expect(body).toHaveLength(2);
    expect(body[0].level).toBe("warn");
    expect(body[1].level).toBe("error");
  });

  it("never throws on fetch failure", async () => {
    const fetchMock = jest.fn(async () => { throw new Error("offline"); });
    const log = createBrowserLogger({
      fetch: fetchMock as unknown as typeof fetch,
      flushMs: 5,
      bufferSize: 10,
      traceId: () => "",
    });
    log.error("boom");
    await new Promise((r) => setTimeout(r, 20));
    // no throw → pass
  });
});
```

- [ ] **Step 2: Run to fail**

```bash
pnpm test -- lib/log.test.ts
```

Expected: FAIL (module not found)

- [ ] **Step 3: Implement**

```ts
// lib/log.ts
export type Level = "debug" | "info" | "warn" | "error";

interface Entry {
  tab: "system";
  level: Level;
  source: "frontend";
  summary: string;
  detail?: Record<string, unknown>;
  ts: number;
}

interface BrowserLoggerOpts {
  fetch?: typeof fetch;
  flushMs?: number;
  bufferSize?: number;
  traceId: () => string;
  endpoint?: string;
}

export interface BrowserLogger {
  debug(summary: string, detail?: Record<string, unknown>): void;
  info(summary: string, detail?: Record<string, unknown>): void;
  warn(summary: string, detail?: Record<string, unknown>): void;
  error(summary: string, detail?: Record<string, unknown>): void;
}

export function createBrowserLogger(opts: BrowserLoggerOpts): BrowserLogger {
  const f = opts.fetch ?? fetch.bind(globalThis);
  const flushMs = opts.flushMs ?? 1000;
  const cap = opts.bufferSize ?? 20;
  const endpoint = opts.endpoint ?? "/api/v1/internal/logs/ingest";
  const buffer: Entry[] = [];
  let timer: ReturnType<typeof setTimeout> | null = null;

  function schedule() {
    if (timer !== null) return;
    timer = setTimeout(flush, flushMs);
  }

  async function flush() {
    timer = null;
    if (buffer.length === 0) return;
    const batch = buffer.splice(0, buffer.length);
    try {
      await f(endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Trace-ID": opts.traceId(),
        },
        body: JSON.stringify(batch),
        keepalive: true,
      });
    } catch {
      // drop the batch; never block UI
    }
  }

  function push(level: Level, summary: string, detail?: Record<string, unknown>) {
    const chosen: Level[] = ["warn", "error"];
    if (!chosen.includes(level)) {
      // info/debug stay console-only
      // eslint-disable-next-line no-console
      (console[level] ?? console.log)(`[${level}]`, summary, detail ?? "");
      return;
    }
    buffer.push({ tab: "system", level, source: "frontend", summary, detail, ts: Date.now() });
    if (buffer.length >= cap) {
      void flush();
    } else {
      schedule();
    }
    // also mirror to console for local dev
    // eslint-disable-next-line no-console
    (console[level] ?? console.log)(`[${level}]`, summary, detail ?? "");
  }

  return {
    debug: (s, d) => push("debug", s, d),
    info:  (s, d) => push("info",  s, d),
    warn:  (s, d) => push("warn",  s, d),
    error: (s, d) => push("error", s, d),
  };
}

let current: BrowserLogger | null = null;
let currentTrace = "";

/** The app's shared singleton logger. Call `initLog` once in the app shell. */
export function log(): BrowserLogger {
  if (!current) {
    current = createBrowserLogger({ traceId: () => currentTrace });
  }
  return current;
}

/** Rotate the current trace_id (called by the fetch interceptor or page root). */
export function setTraceId(id: string) {
  currentTrace = id;
}

export function getTraceId(): string {
  return currentTrace;
}
```

> **Note on batching endpoint:** the ingest handler in Task 5 accepts a single object, not a batch. Either (a) extend it to accept either a single object or an array, or (b) POST one entry at a time. Pick (a): add in `ingest_handler.go` a single decoder path that accepts `[]ingestRequest` when the body starts with `[`. Update Task 5's tests correspondingly OR add a follow-up test in this task. The simplest implementation: try to decode as array first, fall back to single object.

Implement the server-side accommodation:

```go
// in ingest_handler.go, replace the single-decode with:
raw, err := io.ReadAll(c.Request().Body)
if err != nil { return c.JSON(http.StatusBadRequest, map[string]string{"error": "read body"}) }
var batch []ingestRequest
if len(raw) > 0 && raw[0] == '[' {
    if err := json.Unmarshal(raw, &batch); err != nil { return c.JSON(http.StatusBadRequest, map[string]string{"error":"invalid json array"}) }
} else {
    var one ingestRequest
    if err := json.Unmarshal(raw, &one); err != nil { return c.JSON(http.StatusBadRequest, map[string]string{"error":"invalid json"}) }
    batch = []ingestRequest{one}
}
// then iterate batch and call h.svc.CreateLog for each
```

Add an extra ingest handler test asserting batch works.

- [ ] **Step 4: Run to pass**

```bash
pnpm test -- lib/log.test.ts
cd src-go && go test ./internal/handler/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add lib/log.ts lib/log.test.ts src-go/internal/handler/ingest_handler.go src-go/internal/handler/ingest_handler_test.go
git commit -m "feat(obs): browser logger with batched ingest; server accepts batch or single"
```

---

## Task 19: Frontend — Inject X-Trace-ID on Outbound API Calls

**Files:**
- Modify: `lib/api-client.ts`

- [ ] **Step 1: Generate a trace per request and thread it**

Inside the `request<T>(...)` function (around lines 57-100), at the very top, compute a trace_id and call `setTraceId` so the logger uses it for whatever batches get flushed during this request window:

```ts
import { getTraceId, setTraceId } from "./log";

// utility copied once, colocated with api-client for now
function newTraceId(): string {
  const bytes = new Uint8Array(15);
  (globalThis.crypto ?? require("node:crypto").webcrypto).getRandomValues(bytes);
  const alphabet = "0123456789abcdefghjkmnpqrstvwxyz";
  let bits = 0, buf = 0, out = "";
  for (const b of bytes) {
    buf = (buf << 8) | b;
    bits += 8;
    while (bits >= 5) { bits -= 5; out += alphabet[(buf >> bits) & 0x1f]; }
  }
  if (bits > 0) out += alphabet[(buf << (5 - bits)) & 0x1f];
  return "tr_" + out.slice(0, 24);
}

// in request<T>(...):
const existing = (init.headers as Record<string, string> | undefined)?.["X-Trace-ID"];
const trace = existing && existing.length > 0 ? existing : newTraceId();
setTraceId(trace);
const headers = new Headers(init.headers);
headers.set("X-Trace-ID", trace);
init = { ...init, headers };
```

- [ ] **Step 2: Test the header injection**

Add to `lib/api-client.test.ts` (create if missing):

```ts
it("adds X-Trace-ID header when absent", async () => {
  let capturedTrace = "";
  const fakeFetch: typeof fetch = async (_url, init) => {
    const h = new Headers(init?.headers);
    capturedTrace = h.get("X-Trace-ID") ?? "";
    return new Response("{}", { status: 200, headers: { "content-type": "application/json" } });
  };
  globalThis.fetch = fakeFetch;
  const c = createApiClient("http://x");
  await c.get("/whatever");
  expect(capturedTrace).toMatch(/^tr_/);
});
```

- [ ] **Step 3: Run tests**

```bash
pnpm test -- lib/api-client.test.ts
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add lib/api-client.ts lib/api-client.test.ts
git commit -m "feat(obs): frontend api-client injects X-Trace-ID on outbound requests"
```

---

## Task 20: Frontend — Error Boundaries Report via log.error

**Files:**
- Modify: `app/error.tsx`
- Modify: `app/(dashboard)/error.tsx`

- [ ] **Step 1: Edit `app/error.tsx`**

Replace lines 15-17:

```tsx
  useEffect(() => {
    console.error("[GlobalError]", error);
  }, [error]);
```

with:

```tsx
import { log } from "@/lib/log";
// ...
  useEffect(() => {
    log().error("render.global_error", {
      message: error.message,
      digest: error.digest,
      stack: error.stack,
    });
  }, [error]);
```

- [ ] **Step 2: Edit `app/(dashboard)/error.tsx`** — same pattern, event name `render.dashboard_error`.

- [ ] **Step 3: Sanity-check**

```bash
pnpm exec tsc --noEmit
```

Expected: zero errors.

- [ ] **Step 4: Commit**

```bash
git add app/error.tsx app/\(dashboard\)/error.tsx
git commit -m "feat(obs): error boundaries report to ingest via log.error"
```

---

## Task 21: End-to-End Smoke Test

**Files:** none created — manual verification.

- [ ] **Step 1: Start the full stack**

```bash
pnpm dev:all
```

Wait for all health checks to pass (~30s).

- [ ] **Step 2: Synthetic request**

```bash
TRACE=tr_smoke000000000000000000
curl -s -H "X-Trace-ID: $TRACE" http://localhost:7777/health
curl -s -H "X-Trace-ID: $TRACE" http://localhost:7778/health
```

- [ ] **Step 3: Query the logs table**

```bash
psql "$POSTGRES_URL" -c "SELECT created_at, level, summary, detail->>'source' AS source FROM logs WHERE detail->>'trace_id' = '$TRACE' ORDER BY created_at;"
```

Expected: at least one row with `source = ts-bridge` (if the bridge emitted an ingest call during its health path — if not, trigger a known code path that does, e.g., a runtime spawn). At minimum, you should observe the request log line in the Go orchestrator stdout carrying `"trace_id":"tr_smoke..."`.

- [ ] **Step 4: Trigger a real UI action**

Open `http://localhost:3000`, log in, click any button that calls the backend (e.g., load the projects page). Inspect the network tab for an outbound `X-Trace-ID` header. Copy that trace_id.

- [ ] **Step 5: Assert cross-service rows**

```bash
psql "$POSTGRES_URL" -c "SELECT count(*), array_agg(DISTINCT detail->>'source') FROM logs WHERE detail->>'trace_id' = '<copied id>';"
```

Expected: `count ≥ 1`, array includes at least `frontend` **or** observed in Go request logs. If the button path triggers the Bridge, ts-bridge should appear too.

- [ ] **Step 6: Document findings**

Append an entry to `docs/superpowers/specs/2026-04-21-observability-system-design.md` §10 (Success Criteria) marking item 1 "Trace roundtrip" as met, including the trace_id you used.

- [ ] **Step 7: Commit**

```bash
git add docs/superpowers/specs/2026-04-21-observability-system-design.md
git commit -m "docs(obs): record phase-1 trace roundtrip verification"
```

---

## Post-Phase-1 Checklist

- [ ] `grep -rn "console\." src-bridge/src --include="*.ts" | grep -v "\.test\.ts"` returns zero matches
- [ ] `curl -v` to any orchestrator endpoint shows `X-Trace-ID` in the response headers
- [ ] `SELECT ... WHERE detail->>'trace_id' = X` returns cross-service rows
- [ ] All `go test ./...`, `bun test`, `pnpm test` green
- [ ] No new schema migrations — confirm with `git diff --stat src-go/migrations/`

After this, Phase 2 (`/debug` UI) and Phase 3 (infrastructure gap closure) get their own plans, written once Phase 1 has been validated in real use.
