# Unified Event Bus — M1 (Go Core) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship M1 of the Unified Event Bus spec — a new `internal/eventbus` package with Event/Address/Bus/Pipeline/Mod, 7 built-in mods, `events` + `events_dead_letter` tables, and migration of every Go event producer onto `bus.Publish`. Delete the four legacy event structs.

**Architecture:** Introduce `eventbus` as the single ingress. Persistence, WS fan-out, IM legacy delivery, and metrics are all `ObserveMod`s on the same pipeline. WebSocket Hub becomes a pure client-connection registry. The `im_event_routing` path is wrapped as a transitional mod (`im.forward-legacy`); a proper replacement ships in M4.

**Tech Stack:** Go 1.25, Echo, Gorilla WebSocket, PostgreSQL (sqlx), Prometheus, standard `testing` with `testify`, existing migration harness under `src-go/migrations/`.

**Spec:** `docs/superpowers/specs/2026-04-16-unified-event-bus-design.md`

**Scope boundary:** This plan covers only M1. M2 (frontend channel subscribe) and M3 (TS Bridge envelope) are separate plans written after M1 lands.

---

## Pre-flight

- Read the spec end to end before starting.
- Read `src-go/internal/ws/events.go` (the 80+ event-type constants stay alive; M1 moves them into the new package but the string values are frozen).
- Read `src-go/internal/ws/hub.go:L97-L116` to understand the current broadcast surface.
- Read one existing migration pair (e.g. `021_create_agent_events.up.sql` / `.down.sql`) to match house style.
- Confirm local DB + Redis are running: `pnpm dev:backend:status`. If not, run `pnpm dev:backend` in another terminal.
- Create a working branch (or use a worktree): `git checkout -b feat/eventbus-m1`.

---

## Task 1: Discovery — inventory all event producers

**Files:**
- Create: `docs/superpowers/plans/callsite-inventory-eventbus-m1.md` (working artifact — not a permanent doc; may be committed then removed at end of M1)

- [ ] **Step 1: Run discovery greps and paste output into inventory file**

Run:
```bash
rtk grep -n 'hub\.BroadcastEvent' src-go/
rtk grep -n 'BroadcastEvent(' src-go/
rtk grep -n 'AgentEventRepository\|agentEventRepo' src-go/
rtk grep -n 'PluginEventRepository\|pluginEventRepo\|PluginEventRecord' src-go/
rtk grep -n 'BridgeAgentEvent' src-go/
rtk grep -n 'IMEventRouter\|imRouter\|im_event_routing' src-go/
```

Paste the raw hits into `callsite-inventory-eventbus-m1.md`. Group by file.

- [ ] **Step 2: Commit the inventory**

```bash
rtk git add docs/superpowers/plans/callsite-inventory-eventbus-m1.md
rtk git commit -m "chore(eventbus): snapshot callsite inventory for M1 migration"
```

This artifact drives Tasks 17–22. Keep it open while you work.

---

## Task 2: Create eventbus package scaffold

**Files:**
- Create: `src-go/internal/eventbus/doc.go`
- Create: `src-go/internal/eventbus/types.go` (event-type string constants only, copied from `ws/events.go`)

- [ ] **Step 1: Create package doc**

```go
// src-go/internal/eventbus/doc.go
// Package eventbus is the single ingress for all realtime and persisted events
// in AgentForge. It provides a canonical Event envelope, URI addressing,
// and a guard/transform/observe pipeline of Mods.
//
// See docs/superpowers/specs/2026-04-16-unified-event-bus-design.md for design.
package eventbus
```

- [ ] **Step 2: Copy event-type string constants to types.go**

Open `src-go/internal/ws/events.go`, copy all `const` blocks that enumerate event types (both `Event*` and `BridgeEvent*` groups). Paste into `src-go/internal/eventbus/types.go`. Do not delete anything from `ws/events.go` yet — that happens in Task 23. Strings are frozen; do not rename.

Also add these new protocol constants at the bottom:

```go
// Protocol/error events introduced by the bus itself.
const (
    EventErrorRejected  = "event.error.rejected"
    EventErrorTransform = "event.error.transform"
)
```

- [ ] **Step 3: Compile check**

Run: `cd src-go && go build ./internal/eventbus/...`
Expected: success (no symbols used yet, just constants).

- [ ] **Step 4: Commit**

```bash
rtk git add src-go/internal/eventbus/doc.go src-go/internal/eventbus/types.go
rtk git commit -m "feat(eventbus): scaffold package and copy event-type constants"
```

---

## Task 3: Event struct + Visibility + validation (TDD)

**Files:**
- Create: `src-go/internal/eventbus/event.go`
- Test: `src-go/internal/eventbus/event_test.go`

- [ ] **Step 1: Write failing tests for Event.Validate**

```go
// src-go/internal/eventbus/event_test.go
package eventbus

import (
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEventValidate_AcceptsWellFormed(t *testing.T) {
    e := &Event{
        ID:         "01HQZE5MR1T7YBX2A8V3JPZK7M",
        Type:       "task.created",
        Source:     "agent:run-1",
        Target:     "task:7b2e",
        Payload:    json.RawMessage(`{"id":"7b2e"}`),
        Metadata:   map[string]any{},
        Timestamp:  1700000000000,
        Visibility: VisibilityChannel,
    }
    assert.NoError(t, e.Validate())
}

func TestEventValidate_RejectsEmptyRequired(t *testing.T) {
    base := func() *Event {
        return &Event{
            ID: "01HQZE5MR1T7YBX2A8V3JPZK7M", Type: "task.created",
            Source: "agent:run-1", Target: "task:7b2e",
            Timestamp: 1, Visibility: VisibilityChannel,
        }
    }
    cases := []struct {
        name  string
        mut   func(e *Event)
        field string
    }{
        {"id", func(e *Event) { e.ID = "" }, "id"},
        {"type", func(e *Event) { e.Type = "" }, "type"},
        {"source", func(e *Event) { e.Source = "" }, "source"},
        {"target", func(e *Event) { e.Target = "" }, "target"},
        {"timestamp", func(e *Event) { e.Timestamp = 0 }, "timestamp"},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            e := base()
            c.mut(e)
            err := e.Validate()
            require.Error(t, err)
            assert.Contains(t, err.Error(), c.field)
        })
    }
}

func TestEventValidate_TypePattern(t *testing.T) {
    e := &Event{
        ID: "x", Source: "a:b", Target: "c:d",
        Timestamp: 1, Visibility: VisibilityChannel,
    }
    for _, bad := range []string{"Task.Created", "task", "task.created.", ".task", "task created"} {
        e.Type = bad
        assert.Error(t, e.Validate(), "expected reject: %q", bad)
    }
    for _, good := range []string{"task.created", "workflow.execution.started", "a.b.c.d.e"} {
        e.Type = good
        assert.NoError(t, e.Validate(), "expected accept: %q", good)
    }
}

func TestNewEvent_SetsSaneDefaults(t *testing.T) {
    e := NewEvent("task.created", "core", "task:7b2e")
    assert.NotEmpty(t, e.ID)
    assert.Equal(t, VisibilityChannel, e.Visibility)
    assert.NotZero(t, e.Timestamp)
    assert.NotNil(t, e.Metadata)
}
```

- [ ] **Step 2: Run tests, verify failure**

Run: `cd src-go && go test ./internal/eventbus/ -run TestEvent -v`
Expected: build fail (Event undefined).

- [ ] **Step 3: Implement event.go**

```go
// src-go/internal/eventbus/event.go
package eventbus

import (
    "encoding/json"
    "fmt"
    "regexp"
    "time"

    "github.com/oklog/ulid/v2"
)

type Visibility string

const (
    VisibilityPublic  Visibility = "public"
    VisibilityChannel Visibility = "channel"
    VisibilityDirect  Visibility = "direct"
    VisibilityModOnly Visibility = "mod_only"
)

type Event struct {
    ID         string          `json:"id"`
    Type       string          `json:"type"`
    Source     string          `json:"source"`
    Target     string          `json:"target"`
    Payload    json.RawMessage `json:"payload,omitempty"`
    Metadata   map[string]any  `json:"metadata,omitempty"`
    Timestamp  int64           `json:"timestamp"`
    Visibility Visibility      `json:"visibility"`
}

var typeRegexp = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

func (e *Event) Validate() error {
    if e == nil {
        return fmt.Errorf("event: nil")
    }
    if e.ID == "" {
        return fmt.Errorf("event: id required")
    }
    if e.Type == "" {
        return fmt.Errorf("event: type required")
    }
    if !typeRegexp.MatchString(e.Type) {
        return fmt.Errorf("event: type %q violates {domain}.{entity}.{action} lowercase dot-notation", e.Type)
    }
    if e.Source == "" {
        return fmt.Errorf("event: source required")
    }
    if e.Target == "" {
        return fmt.Errorf("event: target required")
    }
    if e.Timestamp == 0 {
        return fmt.Errorf("event: timestamp required")
    }
    if e.Visibility == "" {
        e.Visibility = VisibilityChannel
    }
    return nil
}

func NewEvent(typ, source, target string) *Event {
    return &Event{
        ID:         ulid.Make().String(),
        Type:       typ,
        Source:     source,
        Target:     target,
        Metadata:   map[string]any{},
        Timestamp:  time.Now().UnixMilli(),
        Visibility: VisibilityChannel,
    }
}
```

Add dependency if needed:
```bash
cd src-go && go get github.com/oklog/ulid/v2@latest
```

- [ ] **Step 4: Run tests, verify pass**

Run: `cd src-go && go test ./internal/eventbus/ -run TestEvent -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/eventbus/event.go src-go/internal/eventbus/event_test.go src-go/go.mod src-go/go.sum
rtk git commit -m "feat(eventbus): Event envelope with validation (TDD)"
```

---

## Task 4: Address parsing (TDD)

**Files:**
- Create: `src-go/internal/eventbus/address.go`
- Test: `src-go/internal/eventbus/address_test.go`

- [ ] **Step 1: Failing tests**

```go
// src-go/internal/eventbus/address_test.go
package eventbus

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestParseAddress_Canonical(t *testing.T) {
    cases := []struct {
        raw     string
        scheme  string
        name    string
    }{
        {"agent:run-abc", "agent", "run-abc"},
        {"role:planner", "role", "planner"},
        {"task:7b2e9a", "task", "7b2e9a"},
        {"project:demo", "project", "demo"},
        {"team:alpha", "team", "alpha"},
        {"workflow:wf-42", "workflow", "wf-42"},
        {"plugin:review-linter", "plugin", "review-linter"},
        {"user:max", "user", "max"},
        {"core", "core", ""},
        {"channel:project:demo", "channel", "project:demo"},
        {"channel:task:7b2e", "channel", "task:7b2e"},
    }
    for _, c := range cases {
        t.Run(c.raw, func(t *testing.T) {
            a, err := ParseAddress(c.raw)
            require.NoError(t, err)
            assert.Equal(t, c.scheme, a.Scheme)
            assert.Equal(t, c.name, a.Name)
            assert.Equal(t, c.raw, a.Raw)
        })
    }
}

func TestParseAddress_Rejects(t *testing.T) {
    bad := []string{"", "badscheme:x", "agent:", ":empty"}
    for _, s := range bad {
        _, err := ParseAddress(s)
        assert.Error(t, err, "expected reject: %q", s)
    }
}

func TestAddress_ChannelScope(t *testing.T) {
    a, _ := ParseAddress("channel:task:7b2e")
    inner, ok := a.ChannelScope()
    require.True(t, ok)
    assert.Equal(t, "task", inner.Scheme)
    assert.Equal(t, "7b2e", inner.Name)
}

func TestMakeChannel(t *testing.T) {
    assert.Equal(t, "channel:project:demo", MakeChannel("project", "demo"))
    assert.Equal(t, "channel:task:abc", MakeChannel("task", "abc"))
}
```

- [ ] **Step 2: Run, verify fail**

Run: `cd src-go && go test ./internal/eventbus/ -run TestParseAddress -v`
Expected: build fail.

- [ ] **Step 3: Implement address.go**

```go
// src-go/internal/eventbus/address.go
package eventbus

import (
    "fmt"
    "strings"
)

type Address struct {
    Scheme string
    Name   string
    Raw    string
}

var validSchemes = map[string]struct{}{
    "agent":    {},
    "role":     {},
    "task":     {},
    "project":  {},
    "team":     {},
    "workflow": {},
    "plugin":   {},
    "skill":    {},
    "user":     {},
    "channel":  {},
}

func ParseAddress(s string) (Address, error) {
    if s == "" {
        return Address{}, fmt.Errorf("address: empty")
    }
    if s == "core" {
        return Address{Scheme: "core", Raw: s}, nil
    }
    idx := strings.IndexByte(s, ':')
    if idx <= 0 || idx == len(s)-1 {
        return Address{}, fmt.Errorf("address: %q malformed", s)
    }
    scheme, rest := s[:idx], s[idx+1:]
    if _, ok := validSchemes[scheme]; !ok {
        return Address{}, fmt.Errorf("address: unknown scheme %q", scheme)
    }
    return Address{Scheme: scheme, Name: rest, Raw: s}, nil
}

// ChannelScope returns the inner address embedded in a channel:xxx URI,
// e.g. channel:task:7b2e -> (task:7b2e, true).
func (a Address) ChannelScope() (Address, bool) {
    if a.Scheme != "channel" {
        return Address{}, false
    }
    inner, err := ParseAddress(a.Name)
    if err != nil {
        return Address{}, false
    }
    return inner, true
}

func MakeChannel(scope, name string) string {
    return "channel:" + scope + ":" + name
}

func MakeAgent(runID string) string   { return "agent:" + runID }
func MakeTask(taskID string) string   { return "task:" + taskID }
func MakeProject(pid string) string   { return "project:" + pid }
func MakeTeam(tid string) string      { return "team:" + tid }
func MakeWorkflow(wid string) string  { return "workflow:" + wid }
func MakePlugin(pid string) string    { return "plugin:" + pid }
func MakeUser(uid string) string      { return "user:" + uid }
```

- [ ] **Step 4: Run, verify pass**

Run: `cd src-go && go test ./internal/eventbus/ -run "TestParseAddress|TestAddress|TestMakeChannel" -v`

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/eventbus/address.go src-go/internal/eventbus/address_test.go
rtk git commit -m "feat(eventbus): URI address parsing with helpers (TDD)"
```

---

## Task 5: Metadata helpers (TDD)

**Files:**
- Create: `src-go/internal/eventbus/metadata.go`
- Test: `src-go/internal/eventbus/metadata_test.go`

- [ ] **Step 1: Tests**

```go
// src-go/internal/eventbus/metadata_test.go
package eventbus

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestGetSetChannels(t *testing.T) {
    e := NewEvent("task.created", "core", "task:1")
    assert.Empty(t, GetChannels(e))

    SetChannels(e, []string{"channel:project:demo", "channel:task:1"})
    got := GetChannels(e)
    assert.Equal(t, []string{"channel:project:demo", "channel:task:1"}, got)
}

func TestGetChannels_TypeMismatch(t *testing.T) {
    e := NewEvent("task.created", "core", "task:1")
    e.Metadata["channels"] = "not-a-slice"
    got := GetChannels(e)
    assert.Empty(t, got, "malformed channels must degrade to empty, not panic")
}

func TestGetStringMetadata(t *testing.T) {
    e := NewEvent("x.y", "core", "task:1")
    e.Metadata["user_id"] = "u-1"
    assert.Equal(t, "u-1", GetString(e, "user_id"))
    assert.Equal(t, "", GetString(e, "nope"))
}

func TestCausationDepth(t *testing.T) {
    e := NewEvent("x.y", "core", "task:1")
    assert.Equal(t, 0, GetCausationDepth(e))
    IncrementCausationDepth(e)
    require.Equal(t, 1, GetCausationDepth(e))
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement metadata.go**

```go
// src-go/internal/eventbus/metadata.go
package eventbus

const (
    MetaChannels        = "channels"
    MetaSpanID          = "span_id"
    MetaTraceID         = "trace_id"
    MetaCausationID     = "causation_id"
    MetaCorrelationID   = "correlation_id"
    MetaUserID          = "user_id"
    MetaProjectID       = "project_id"
    MetaCausationDepth  = "causation_depth"
)

func ensureMeta(e *Event) {
    if e.Metadata == nil {
        e.Metadata = map[string]any{}
    }
}

func GetChannels(e *Event) []string {
    v, ok := e.Metadata[MetaChannels]
    if !ok {
        return nil
    }
    switch x := v.(type) {
    case []string:
        out := make([]string, len(x))
        copy(out, x)
        return out
    case []any:
        out := make([]string, 0, len(x))
        for _, item := range x {
            if s, ok := item.(string); ok {
                out = append(out, s)
            }
        }
        return out
    default:
        return nil
    }
}

func SetChannels(e *Event, channels []string) {
    ensureMeta(e)
    e.Metadata[MetaChannels] = append([]string(nil), channels...)
}

func GetString(e *Event, key string) string {
    if e.Metadata == nil {
        return ""
    }
    if s, ok := e.Metadata[key].(string); ok {
        return s
    }
    return ""
}

func SetString(e *Event, key, value string) {
    ensureMeta(e)
    e.Metadata[key] = value
}

func GetCausationDepth(e *Event) int {
    if e.Metadata == nil {
        return 0
    }
    switch n := e.Metadata[MetaCausationDepth].(type) {
    case int:
        return n
    case int64:
        return int(n)
    case float64:
        return int(n)
    default:
        return 0
    }
}

func IncrementCausationDepth(e *Event) {
    ensureMeta(e)
    e.Metadata[MetaCausationDepth] = GetCausationDepth(e) + 1
}
```

- [ ] **Step 4: Run, pass**

Run: `cd src-go && go test ./internal/eventbus/ -run Meta -v`

- [ ] **Step 5: Commit**

```bash
rtk git commit -am "feat(eventbus): reserved metadata accessors (TDD)"
```

---

## Task 6: Mod interfaces + PipelineCtx (types only, no behaviour)

**Files:**
- Create: `src-go/internal/eventbus/mod.go`

- [ ] **Step 1: Write mod.go**

```go
// src-go/internal/eventbus/mod.go
package eventbus

import "context"

type Mode uint8

const (
    ModeGuard     Mode = 1
    ModeTransform Mode = 2
    ModeObserve   Mode = 3
)

type Mod interface {
    Name() string
    Intercepts() []string // glob-ish patterns, "*" matches all, prefix.* matches prefix.X
    Priority() int
    Mode() Mode
}

type GuardMod interface {
    Mod
    Guard(ctx context.Context, e *Event, pc *PipelineCtx) error
}

type TransformMod interface {
    Mod
    Transform(ctx context.Context, e *Event, pc *PipelineCtx) (*Event, error)
}

type ObserveMod interface {
    Mod
    Observe(ctx context.Context, e *Event, pc *PipelineCtx)
}

type PipelineCtx struct {
    NetworkID string
    Emits     []Event
    SpanID    string
    Attrs     map[string]any
}

func (pc *PipelineCtx) Emit(e Event) {
    pc.Emits = append(pc.Emits, e)
}

// MatchesType reports whether a glob pattern matches an event type.
// Supported forms: "*" | "prefix.*" | exact string.
func MatchesType(pattern, typ string) bool {
    if pattern == "*" {
        return true
    }
    if len(pattern) > 2 && pattern[len(pattern)-2:] == ".*" {
        prefix := pattern[:len(pattern)-2]
        if typ == prefix {
            return true
        }
        if len(typ) > len(prefix) && typ[:len(prefix)+1] == prefix+"." {
            return true
        }
        return false
    }
    return pattern == typ
}
```

- [ ] **Step 2: Quick unit test for MatchesType**

Add to `src-go/internal/eventbus/mod_test.go`:

```go
package eventbus

import "testing"

func TestMatchesType(t *testing.T) {
    cases := []struct {
        pat, typ string
        want     bool
    }{
        {"*", "anything.at.all", true},
        {"task.*", "task.created", true},
        {"task.*", "task", true},
        {"task.*", "tasks.created", false},
        {"task.created", "task.created", true},
        {"task.created", "task.updated", false},
        {"workflow.execution.*", "workflow.execution.started", true},
        {"workflow.execution.*", "workflow.other", false},
    }
    for _, c := range cases {
        got := MatchesType(c.pat, c.typ)
        if got != c.want {
            t.Errorf("MatchesType(%q,%q)=%v want %v", c.pat, c.typ, got, c.want)
        }
    }
}
```

- [ ] **Step 3: Run, pass**

Run: `cd src-go && go test ./internal/eventbus/ -run TestMatchesType -v`

- [ ] **Step 4: Commit**

```bash
rtk git add src-go/internal/eventbus/mod.go src-go/internal/eventbus/mod_test.go
rtk git commit -m "feat(eventbus): Mod interfaces, modes, and glob matcher (TDD)"
```

---

## Task 7: Pipeline executor (TDD)

**Files:**
- Create: `src-go/internal/eventbus/pipeline.go`
- Test: `src-go/internal/eventbus/pipeline_test.go`

- [ ] **Step 1: Tests**

```go
// src-go/internal/eventbus/pipeline_test.go
package eventbus

import (
    "context"
    "errors"
    "sync/atomic"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Test helpers ---------------------------------------------------

type fakeGuard struct {
    name string; prio int; inter []string
    err  error
    calls int32
}
func (f *fakeGuard) Name() string         { return f.name }
func (f *fakeGuard) Intercepts() []string { return f.inter }
func (f *fakeGuard) Priority() int        { return f.prio }
func (f *fakeGuard) Mode() Mode           { return ModeGuard }
func (f *fakeGuard) Guard(ctx context.Context, e *Event, pc *PipelineCtx) error {
    atomic.AddInt32(&f.calls, 1)
    return f.err
}

type fakeTransform struct {
    name string; prio int; inter []string
    mutate func(*Event)
    err    error
}
func (f *fakeTransform) Name() string         { return f.name }
func (f *fakeTransform) Intercepts() []string { return f.inter }
func (f *fakeTransform) Priority() int        { return f.prio }
func (f *fakeTransform) Mode() Mode           { return ModeTransform }
func (f *fakeTransform) Transform(ctx context.Context, e *Event, pc *PipelineCtx) (*Event, error) {
    if f.err != nil { return nil, f.err }
    if f.mutate != nil { f.mutate(e) }
    return e, nil
}

type fakeObserve struct {
    name string; prio int; inter []string
    calls int32
    panic bool
}
func (f *fakeObserve) Name() string         { return f.name }
func (f *fakeObserve) Intercepts() []string { return f.inter }
func (f *fakeObserve) Priority() int        { return f.prio }
func (f *fakeObserve) Mode() Mode           { return ModeObserve }
func (f *fakeObserve) Observe(ctx context.Context, e *Event, pc *PipelineCtx) {
    atomic.AddInt32(&f.calls, 1)
    if f.panic { panic("kaboom") }
}

// Tests -----------------------------------------------------------

func TestPipeline_Ordering(t *testing.T) {
    order := []string{}
    var mu = make(chan struct{}, 1); mu <- struct{}{}
    track := func(name string) func() {
        return func() {
            <-mu
            order = append(order, name)
            mu <- struct{}{}
        }
    }
    _ = track // guard against unused in refactors

    g1 := &fakeGuard{name: "g1", prio: 10, inter: []string{"*"}}
    t1 := &fakeTransform{name: "t1", prio: 10, inter: []string{"*"}, mutate: func(e *Event) { SetString(e, "t1", "ok") }}
    o1 := &fakeObserve{name: "o1", prio: 10, inter: []string{"*"}}

    p := NewPipeline([]Mod{o1, t1, g1})
    e := NewEvent("task.created", "core", "task:1")
    out, err := p.Process(context.Background(), e, &PipelineCtx{})
    require.NoError(t, err)
    assert.Equal(t, "ok", GetString(out, "t1"))
    time.Sleep(20 * time.Millisecond) // allow parallel observe to finish
    assert.Equal(t, int32(1), atomic.LoadInt32(&g1.calls))
    assert.Equal(t, int32(1), atomic.LoadInt32(&o1.calls))
}

func TestPipeline_GuardRejects(t *testing.T) {
    g := &fakeGuard{name: "g", prio: 1, inter: []string{"*"}, err: errors.New("nope")}
    o := &fakeObserve{name: "o", prio: 1, inter: []string{"*"}}
    p := NewPipeline([]Mod{g, o})
    e := NewEvent("task.created", "core", "task:1")
    _, err := p.Process(context.Background(), e, &PipelineCtx{})
    require.Error(t, err)
    time.Sleep(10 * time.Millisecond)
    assert.Equal(t, int32(0), atomic.LoadInt32(&o.calls), "observer must not run after guard rejects")
}

func TestPipeline_IntercePattern(t *testing.T) {
    g := &fakeGuard{name: "g", prio: 1, inter: []string{"workflow.*"}}
    p := NewPipeline([]Mod{g})
    _, err := p.Process(context.Background(), NewEvent("task.created", "c", "t:1"), &PipelineCtx{})
    require.NoError(t, err)
    assert.Equal(t, int32(0), atomic.LoadInt32(&g.calls))

    _, err = p.Process(context.Background(), NewEvent("workflow.execution.started", "c", "t:1"), &PipelineCtx{})
    require.NoError(t, err)
    assert.Equal(t, int32(1), atomic.LoadInt32(&g.calls))
}

func TestPipeline_ObservePanicIsolated(t *testing.T) {
    o1 := &fakeObserve{name: "o1", prio: 1, inter: []string{"*"}, panic: true}
    o2 := &fakeObserve{name: "o2", prio: 2, inter: []string{"*"}}
    p := NewPipeline([]Mod{o1, o2})
    _, err := p.Process(context.Background(), NewEvent("x.y", "c", "t:1"), &PipelineCtx{})
    require.NoError(t, err)
    time.Sleep(50 * time.Millisecond)
    assert.Equal(t, int32(1), atomic.LoadInt32(&o2.calls))
}

func TestPipeline_PriorityWithinMode(t *testing.T) {
    seen := []string{}
    var mu = make(chan struct{}, 1); mu <- struct{}{}
    addSeen := func(n string) {
        <-mu; seen = append(seen, n); mu <- struct{}{}
    }
    t1 := &fakeTransform{name: "tA", prio: 20, inter: []string{"*"}, mutate: func(e *Event) { addSeen("tA") }}
    t2 := &fakeTransform{name: "tB", prio: 10, inter: []string{"*"}, mutate: func(e *Event) { addSeen("tB") }}
    p := NewPipeline([]Mod{t1, t2})
    _, err := p.Process(context.Background(), NewEvent("x.y", "c", "t:1"), &PipelineCtx{})
    require.NoError(t, err)
    assert.Equal(t, []string{"tB", "tA"}, seen)
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement pipeline.go**

```go
// src-go/internal/eventbus/pipeline.go
package eventbus

import (
    "context"
    "fmt"
    "sort"
    "sync"
    "time"

    log "github.com/sirupsen/logrus"
)

const observerTimeout = 5 * time.Second

type Pipeline struct {
    guards     []GuardMod
    transforms []TransformMod
    observes   []ObserveMod
}

func NewPipeline(mods []Mod) *Pipeline {
    p := &Pipeline{}
    for _, m := range mods {
        switch m.Mode() {
        case ModeGuard:
            if g, ok := m.(GuardMod); ok {
                p.guards = append(p.guards, g)
            }
        case ModeTransform:
            if t, ok := m.(TransformMod); ok {
                p.transforms = append(p.transforms, t)
            }
        case ModeObserve:
            if o, ok := m.(ObserveMod); ok {
                p.observes = append(p.observes, o)
            }
        }
    }
    sort.SliceStable(p.guards, func(i, j int) bool { return p.guards[i].Priority() < p.guards[j].Priority() })
    sort.SliceStable(p.transforms, func(i, j int) bool { return p.transforms[i].Priority() < p.transforms[j].Priority() })
    sort.SliceStable(p.observes, func(i, j int) bool { return p.observes[i].Priority() < p.observes[j].Priority() })
    return p
}

func (p *Pipeline) Process(ctx context.Context, e *Event, pc *PipelineCtx) (*Event, error) {
    if err := e.Validate(); err != nil {
        return nil, err
    }
    for _, g := range p.guards {
        if !intercepts(g, e.Type) {
            continue
        }
        if err := g.Guard(ctx, e, pc); err != nil {
            return nil, fmt.Errorf("guard %q rejected: %w", g.Name(), err)
        }
    }
    cur := e
    for _, t := range p.transforms {
        if !intercepts(t, cur.Type) {
            continue
        }
        out, err := t.Transform(ctx, cur, pc)
        if err != nil {
            return nil, fmt.Errorf("transform %q failed: %w", t.Name(), err)
        }
        if out != nil {
            cur = out
        }
    }
    // Fan out observers in parallel; cap with deadline; recover panics.
    var wg sync.WaitGroup
    for _, o := range p.observes {
        if !intercepts(o, cur.Type) {
            continue
        }
        wg.Add(1)
        go func(obs ObserveMod) {
            defer wg.Done()
            defer func() {
                if r := recover(); r != nil {
                    log.WithFields(log.Fields{"mod": obs.Name(), "panic": r}).
                        Error("eventbus: observer panic")
                }
            }()
            cctx, cancel := context.WithTimeout(ctx, observerTimeout)
            defer cancel()
            obs.Observe(cctx, cur, pc)
        }(o)
    }
    wg.Wait()
    return cur, nil
}

func intercepts(m Mod, typ string) bool {
    for _, pat := range m.Intercepts() {
        if MatchesType(pat, typ) {
            return true
        }
    }
    return false
}
```

- [ ] **Step 4: Run all pipeline tests**

Run: `cd src-go && go test ./internal/eventbus/ -run TestPipeline -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
rtk git commit -am "feat(eventbus): Pipeline executor with three-mode ordering (TDD)"
```

---

## Task 8: Bus with Publish + Register (TDD)

**Files:**
- Create: `src-go/internal/eventbus/bus.go`
- Test: `src-go/internal/eventbus/bus_test.go`

- [ ] **Step 1: Tests**

```go
// src-go/internal/eventbus/bus_test.go
package eventbus

import (
    "context"
    "sync/atomic"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestBus_Publish_InvokesPipeline(t *testing.T) {
    o := &fakeObserve{name: "o", prio: 1, inter: []string{"*"}}
    bus := NewBus()
    bus.Register(o)
    require.NoError(t, bus.Publish(context.Background(), NewEvent("task.created", "c", "task:1")))
    time.Sleep(20 * time.Millisecond)
    assert.Equal(t, int32(1), atomic.LoadInt32(&o.calls))
}

func TestBus_RegisterPanicsAfterStart(t *testing.T) {
    bus := NewBus()
    bus.Register(&fakeObserve{name: "a", prio: 1, inter: []string{"*"}})
    require.NoError(t, bus.Publish(context.Background(), NewEvent("x.y", "c", "t:1")))
    assert.Panics(t, func() {
        bus.Register(&fakeObserve{name: "late", prio: 1, inter: []string{"*"}})
    })
}

func TestBus_EmitsFollowThroughPipeline(t *testing.T) {
    // a transform that emits a child
    emitter := &emittingTransform{inter: []string{"task.created"}, childType: "audit.trail"}
    collector := &fakeObserve{name: "obs", prio: 1, inter: []string{"*"}}
    bus := NewBus()
    bus.Register(emitter); bus.Register(collector)

    require.NoError(t, bus.Publish(context.Background(), NewEvent("task.created", "c", "task:1")))
    time.Sleep(50 * time.Millisecond)
    assert.Equal(t, int32(2), atomic.LoadInt32(&collector.calls), "both original and emitted must observe")
}

func TestBus_DepthLimit(t *testing.T) {
    emitter := &emittingTransform{inter: []string{"*"}, childType: "x.y"}
    bus := NewBus()
    bus.Register(emitter)
    err := bus.Publish(context.Background(), NewEvent("x.y", "c", "t:1"))
    require.NoError(t, err) // publish itself succeeds
    // Infinite recursion is bounded by depth limit; no stack overflow.
    time.Sleep(50 * time.Millisecond)
}

// helper
type emittingTransform struct {
    inter     []string
    childType string
}
func (e *emittingTransform) Name() string         { return "emitter" }
func (e *emittingTransform) Intercepts() []string { return e.inter }
func (e *emittingTransform) Priority() int        { return 1 }
func (e *emittingTransform) Mode() Mode           { return ModeTransform }
func (e *emittingTransform) Transform(ctx context.Context, ev *Event, pc *PipelineCtx) (*Event, error) {
    pc.Emit(*NewEvent(e.childType, "core", "task:1"))
    return ev, nil
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement bus.go**

```go
// src-go/internal/eventbus/bus.go
package eventbus

import (
    "context"
    "sync"

    log "github.com/sirupsen/logrus"
)

const maxCausationDepth = 3

type Bus struct {
    mu       sync.Mutex
    mods     []Mod
    pipeline *Pipeline
    started  bool
}

func NewBus() *Bus {
    return &Bus{}
}

// Register adds a mod. Registration is only allowed before the first Publish.
func (b *Bus) Register(m Mod) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.started {
        panic("eventbus: Register called after Publish; wire all mods during startup")
    }
    b.mods = append(b.mods, m)
}

func (b *Bus) ensurePipeline() {
    b.mu.Lock()
    defer b.mu.Unlock()
    if !b.started {
        b.pipeline = NewPipeline(b.mods)
        b.started = true
    }
}

// Publish runs the event through the pipeline. Emit-children are recursively
// re-published up to maxCausationDepth.
func (b *Bus) Publish(ctx context.Context, e *Event) error {
    b.ensurePipeline()
    return b.publishInternal(ctx, e)
}

func (b *Bus) publishInternal(ctx context.Context, e *Event) error {
    pc := &PipelineCtx{Attrs: map[string]any{}}
    out, err := b.pipeline.Process(ctx, e, pc)
    if err != nil {
        log.WithFields(log.Fields{"event_id": e.ID, "event_type": e.Type, "err": err}).
            Warn("eventbus: publish rejected")
        return err
    }
    depth := GetCausationDepth(out) + 1
    for i := range pc.Emits {
        child := pc.Emits[i]
        if depth > maxCausationDepth {
            log.WithFields(log.Fields{"parent": out.ID, "child_type": child.Type}).
                Warn("eventbus: causation depth limit, child dropped")
            continue
        }
        ensureMeta(&child)
        child.Metadata[MetaCausationID] = out.ID
        child.Metadata[MetaCausationDepth] = depth
        if err := b.publishInternal(ctx, &child); err != nil {
            log.WithFields(log.Fields{"parent": out.ID, "child_type": child.Type, "err": err}).
                Warn("eventbus: emitted child publish failed")
        }
    }
    return nil
}
```

- [ ] **Step 4: Run bus tests**

Run: `cd src-go && go test ./internal/eventbus/ -run TestBus -v`

- [ ] **Step 5: Commit**

```bash
rtk git commit -am "feat(eventbus): Bus with registration lock and depth-limited emits (TDD)"
```

---

## Task 9: core.validate + core.auth guards (TDD)

**Files:**
- Create: `src-go/internal/eventbus/mods/validate.go`
- Create: `src-go/internal/eventbus/mods/auth.go`
- Test: `src-go/internal/eventbus/mods/validate_test.go`
- Test: `src-go/internal/eventbus/mods/auth_test.go`

`mods` is a subpackage so built-ins are discoverable but not cyclically coupled.

- [ ] **Step 1: Tests for validate**

```go
// src-go/internal/eventbus/mods/validate_test.go
package mods

import (
    "context"
    "testing"

    "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestValidate_RejectsMalformedChannels(t *testing.T) {
    g := NewValidate()
    e := eventbus.NewEvent("task.created", "core", "task:1")
    e.Metadata["channels"] = "not-a-slice"
    err := g.Guard(context.Background(), e, &eventbus.PipelineCtx{})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "channels")
}

func TestValidate_AcceptsStringSliceChannels(t *testing.T) {
    g := NewValidate()
    e := eventbus.NewEvent("task.created", "core", "task:1")
    eventbus.SetChannels(e, []string{"channel:project:demo"})
    require.NoError(t, g.Guard(context.Background(), e, &eventbus.PipelineCtx{}))
}

func TestValidate_RejectsUnknownSourceScheme(t *testing.T) {
    g := NewValidate()
    e := eventbus.NewEvent("task.created", "wtf:xx", "task:1")
    require.Error(t, g.Guard(context.Background(), e, &eventbus.PipelineCtx{}))
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement validate.go**

```go
// src-go/internal/eventbus/mods/validate.go
package mods

import (
    "context"
    "fmt"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type Validate struct{}

func NewValidate() *Validate { return &Validate{} }

func (v *Validate) Name() string           { return "core.validate" }
func (v *Validate) Intercepts() []string   { return []string{"*"} }
func (v *Validate) Priority() int          { return 10 }
func (v *Validate) Mode() eb.Mode          { return eb.ModeGuard }

func (v *Validate) Guard(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) error {
    if _, err := eb.ParseAddress(e.Source); err != nil {
        return fmt.Errorf("invalid source: %w", err)
    }
    if _, err := eb.ParseAddress(e.Target); err != nil {
        return fmt.Errorf("invalid target: %w", err)
    }
    if raw, ok := e.Metadata[eb.MetaChannels]; ok {
        switch raw.(type) {
        case []string, []any, nil:
            // fine
        default:
            return fmt.Errorf("metadata.channels must be []string, got %T", raw)
        }
    }
    return nil
}
```

- [ ] **Step 4: Tests for auth**

```go
// src-go/internal/eventbus/mods/auth_test.go
package mods

import (
    "context"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/stretchr/testify/assert"
)

func TestAuth_AcceptsCoreSource(t *testing.T) {
    // M1: lenient — system events (source=core or source=plugin:*) always pass.
    a := NewAuth()
    e := eb.NewEvent("task.created", "core", "task:1")
    assert.NoError(t, a.Guard(context.Background(), e, &eb.PipelineCtx{}))
}

func TestAuth_AcceptsMatchingUser(t *testing.T) {
    a := NewAuth()
    e := eb.NewEvent("task.created", "user:u-1", "task:1")
    eb.SetString(e, eb.MetaUserID, "u-1")
    assert.NoError(t, a.Guard(context.Background(), e, &eb.PipelineCtx{}))
}

func TestAuth_RejectsSpoofedUser(t *testing.T) {
    a := NewAuth()
    e := eb.NewEvent("task.created", "user:alice", "task:1")
    eb.SetString(e, eb.MetaUserID, "bob")
    assert.Error(t, a.Guard(context.Background(), e, &eb.PipelineCtx{}))
}
```

- [ ] **Step 5: Implement auth.go (M1 minimal — source/user consistency)**

```go
// src-go/internal/eventbus/mods/auth.go
package mods

import (
    "context"
    "fmt"
    "strings"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type Auth struct{}

func NewAuth() *Auth { return &Auth{} }

func (a *Auth) Name() string           { return "core.auth" }
func (a *Auth) Intercepts() []string   { return []string{"*"} }
func (a *Auth) Priority() int          { return 20 }
func (a *Auth) Mode() eb.Mode          { return eb.ModeGuard }

func (a *Auth) Guard(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) error {
    if e.Source == "core" || strings.HasPrefix(e.Source, "plugin:") || strings.HasPrefix(e.Source, "agent:") {
        return nil
    }
    if strings.HasPrefix(e.Source, "user:") {
        claimed := strings.TrimPrefix(e.Source, "user:")
        ctxUser := eb.GetString(e, eb.MetaUserID)
        if ctxUser == "" || ctxUser == claimed {
            return nil
        }
        return fmt.Errorf("source user %q does not match context user %q", claimed, ctxUser)
    }
    // Defer other schemes to transform/enrich.
    return nil
}
```

- [ ] **Step 6: Run all mod tests**

Run: `cd src-go && go test ./internal/eventbus/mods/ -v`

- [ ] **Step 7: Commit**

```bash
rtk git add src-go/internal/eventbus/mods
rtk git commit -m "feat(eventbus): core.validate and core.auth guards (TDD)"
```

---

## Task 10: core.enrich + core.channel-router transforms (TDD)

**Files:**
- Create: `src-go/internal/eventbus/mods/enrich.go`
- Create: `src-go/internal/eventbus/mods/channel_router.go`
- Test: `src-go/internal/eventbus/mods/enrich_test.go`
- Test: `src-go/internal/eventbus/mods/channel_router_test.go`

- [ ] **Step 1: Tests**

```go
// src-go/internal/eventbus/mods/enrich_test.go
package mods

import (
    "context"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEnrich_AddsSpanIDWhenMissing(t *testing.T) {
    tr := NewEnrich()
    e := eb.NewEvent("task.created", "core", "task:1")
    out, err := tr.Transform(context.Background(), e, &eb.PipelineCtx{SpanID: "span-42"})
    require.NoError(t, err)
    assert.Equal(t, "span-42", eb.GetString(out, eb.MetaSpanID))
}

func TestEnrich_KeepsExistingSpanID(t *testing.T) {
    tr := NewEnrich()
    e := eb.NewEvent("task.created", "core", "task:1")
    eb.SetString(e, eb.MetaSpanID, "caller-set")
    out, _ := tr.Transform(context.Background(), e, &eb.PipelineCtx{SpanID: "span-42"})
    assert.Equal(t, "caller-set", eb.GetString(out, eb.MetaSpanID))
}
```

```go
// src-go/internal/eventbus/mods/channel_router_test.go
package mods

import (
    "context"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestChannelRouter_TaskTargetAddsTaskAndProject(t *testing.T) {
    tr := NewChannelRouter()
    e := eb.NewEvent("task.updated", "core", "task:abc-1")
    eb.SetString(e, eb.MetaProjectID, "p-1")
    out, err := tr.Transform(context.Background(), e, &eb.PipelineCtx{})
    require.NoError(t, err)
    channels := eb.GetChannels(out)
    assert.Contains(t, channels, "channel:task:abc-1")
    assert.Contains(t, channels, "channel:project:p-1")
}

func TestChannelRouter_AgentTargetAddsAgentAndProject(t *testing.T) {
    tr := NewChannelRouter()
    e := eb.NewEvent("agent.started", "core", "agent:run-7")
    eb.SetString(e, eb.MetaProjectID, "p-1")
    out, _ := tr.Transform(context.Background(), e, &eb.PipelineCtx{})
    assert.Contains(t, eb.GetChannels(out), "channel:agent:run-7")
    assert.Contains(t, eb.GetChannels(out), "channel:project:p-1")
}

func TestChannelRouter_NoProjectJustTarget(t *testing.T) {
    tr := NewChannelRouter()
    e := eb.NewEvent("plugin.lifecycle", "core", "plugin:review")
    out, _ := tr.Transform(context.Background(), e, &eb.PipelineCtx{})
    assert.Equal(t, []string{"channel:plugin:review"}, eb.GetChannels(out))
}
```

- [ ] **Step 2: Run, fail**

- [ ] **Step 3: Implement enrich.go**

```go
// src-go/internal/eventbus/mods/enrich.go
package mods

import (
    "context"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type Enrich struct{}
func NewEnrich() *Enrich { return &Enrich{} }

func (e *Enrich) Name() string         { return "core.enrich" }
func (e *Enrich) Intercepts() []string { return []string{"*"} }
func (e *Enrich) Priority() int        { return 10 }
func (e *Enrich) Mode() eb.Mode        { return eb.ModeTransform }

func (e *Enrich) Transform(ctx context.Context, ev *eb.Event, pc *eb.PipelineCtx) (*eb.Event, error) {
    if pc.SpanID != "" && eb.GetString(ev, eb.MetaSpanID) == "" {
        eb.SetString(ev, eb.MetaSpanID, pc.SpanID)
    }
    return ev, nil
}
```

- [ ] **Step 4: Implement channel_router.go**

```go
// src-go/internal/eventbus/mods/channel_router.go
package mods

import (
    "context"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type ChannelRouter struct{}
func NewChannelRouter() *ChannelRouter { return &ChannelRouter{} }

func (c *ChannelRouter) Name() string         { return "core.channel-router" }
func (c *ChannelRouter) Intercepts() []string { return []string{"*"} }
func (c *ChannelRouter) Priority() int        { return 20 }
func (c *ChannelRouter) Mode() eb.Mode        { return eb.ModeTransform }

func (c *ChannelRouter) Transform(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) (*eb.Event, error) {
    channels := eb.GetChannels(e)
    seen := map[string]struct{}{}
    for _, ch := range channels {
        seen[ch] = struct{}{}
    }
    add := func(ch string) {
        if _, ok := seen[ch]; ok {
            return
        }
        seen[ch] = struct{}{}
        channels = append(channels, ch)
    }
    addr, err := eb.ParseAddress(e.Target)
    if err == nil {
        add("channel:" + addr.Scheme + ":" + addr.Name)
    }
    if pid := eb.GetString(e, eb.MetaProjectID); pid != "" {
        add(eb.MakeChannel("project", pid))
    }
    eb.SetChannels(e, channels)
    return e, nil
}
```

- [ ] **Step 5: Run, pass**

Run: `cd src-go && go test ./internal/eventbus/mods/ -v`

- [ ] **Step 6: Commit**

```bash
rtk git commit -am "feat(eventbus): core.enrich and core.channel-router transforms (TDD)"
```

---

## Task 11: core.metrics observer (TDD)

**Files:**
- Create: `src-go/internal/eventbus/mods/metrics.go`
- Test: `src-go/internal/eventbus/mods/metrics_test.go`

- [ ] **Step 1: Tests**

```go
// src-go/internal/eventbus/mods/metrics_test.go
package mods

import (
    "context"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    dto "github.com/prometheus/client_model/go"
    "github.com/stretchr/testify/require"
)

func TestMetrics_CountsByType(t *testing.T) {
    m := NewMetrics()
    for i := 0; i < 3; i++ {
        m.Observe(context.Background(), eb.NewEvent("task.created", "core", "task:1"), &eb.PipelineCtx{})
    }
    m.Observe(context.Background(), eb.NewEvent("agent.started", "core", "agent:r-1"), &eb.PipelineCtx{})

    count := func(typ string) float64 {
        mets := &dto.Metric{}
        require.NoError(t, m.counter.WithLabelValues(typ).Write(mets))
        return mets.Counter.GetValue()
    }
    require.Equal(t, 3.0, count("task.created"))
    require.Equal(t, 1.0, count("agent.started"))
}
```

- [ ] **Step 2: Implement metrics.go**

```go
// src-go/internal/eventbus/mods/metrics.go
package mods

import (
    "context"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
    counter *prometheus.CounterVec
}

func NewMetrics() *Metrics {
    m := &Metrics{
        counter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: "agentforge",
                Subsystem: "eventbus",
                Name:      "events_observed_total",
                Help:      "Events passing observe stage, labelled by type.",
            },
            []string{"type"},
        ),
    }
    return m
}

func (m *Metrics) Collector() prometheus.Collector { return m.counter }

func (m *Metrics) Name() string         { return "core.metrics" }
func (m *Metrics) Intercepts() []string { return []string{"*"} }
func (m *Metrics) Priority() int        { return 90 }
func (m *Metrics) Mode() eb.Mode        { return eb.ModeObserve }

func (m *Metrics) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
    m.counter.WithLabelValues(e.Type).Inc()
}
```

If `prometheus` is not already in `go.mod`:
```bash
cd src-go && go get github.com/prometheus/client_golang/prometheus
```

- [ ] **Step 3: Run, pass, commit**

Run: `cd src-go && go test ./internal/eventbus/mods/ -run TestMetrics -v`

```bash
rtk git commit -am "feat(eventbus): core.metrics observer with prometheus counter (TDD)"
```

---

## Task 12: Database migration — events + events_dead_letter; drop agent_events

**Files:**
- Create: `src-go/migrations/054_create_event_bus_tables.up.sql`
- Create: `src-go/migrations/054_create_event_bus_tables.down.sql`

- [ ] **Step 1: Confirm the next migration number**

```bash
ls src-go/migrations/*.up.sql | sort | tail -1
```

If the highest existing number is not `053`, bump the two new filenames accordingly. The rest of this task assumes `054`; rename if needed.

- [ ] **Step 2: Write up migration**

```sql
-- src-go/migrations/054_create_event_bus_tables.up.sql
BEGIN;

CREATE TABLE IF NOT EXISTS events (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,
    source       TEXT NOT NULL,
    target       TEXT NOT NULL,
    visibility   TEXT NOT NULL DEFAULT 'channel',
    payload      JSONB,
    metadata     JSONB NOT NULL DEFAULT '{}'::jsonb,
    project_id   UUID,
    occurred_at  BIGINT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS events_type_idx        ON events (type, occurred_at DESC);
CREATE INDEX IF NOT EXISTS events_target_idx      ON events (target, occurred_at DESC);
CREATE INDEX IF NOT EXISTS events_project_idx     ON events (project_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS events_dead_letter (
    id            BIGSERIAL PRIMARY KEY,
    event_id      TEXT NOT NULL,
    envelope      JSONB NOT NULL,
    last_error    TEXT NOT NULL,
    retry_count   INTEGER NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS events_dlq_event_idx ON events_dead_letter (event_id);

DROP TABLE IF EXISTS agent_events;

COMMIT;
```

- [ ] **Step 3: Write down migration**

```sql
-- src-go/migrations/054_create_event_bus_tables.down.sql
BEGIN;

DROP TABLE IF EXISTS events_dead_letter;
DROP TABLE IF EXISTS events;

CREATE TABLE IF NOT EXISTS agent_events (
    id          UUID PRIMARY KEY,
    run_id      UUID NOT NULL,
    task_id     UUID NOT NULL,
    project_id  UUID NOT NULL,
    event_type  TEXT NOT NULL,
    payload     TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
```

- [ ] **Step 4: Apply migration locally**

Run: `pnpm dev:backend:restart go-orchestrator` (picks up migration on startup) or the project's standard migration command (e.g. `cd src-go && go run ./cmd/migrate up` if present — check the repo for how migrations are executed; adapt this step to match).

Verify: connect to local Postgres, `\d events` and `\d events_dead_letter` show expected columns; `\d agent_events` reports "no relation".

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/migrations/054_create_event_bus_tables.*.sql
rtk git commit -m "feat(eventbus): migration 054 creates events + dead-letter, drops agent_events"
```

---

## Task 13: EventsRepository + DeadLetterRepository (TDD against real DB)

> **Note:** The new package `internal/eventbus/repository` sits beside — not inside — `internal/repository`. The test-DB helper used by existing repo tests is likely unexported inside `internal/repository`. Two options, pick one:
> 1. Move the helper into an exported shared test helper (e.g. `internal/testdb`), then import from both packages.
> 2. Duplicate the minimal helper inside `internal/eventbus/repository/helpers_test.go`. Acceptable for v1; remove when (1) happens.

**Files:**
- Create: `src-go/internal/eventbus/repository/events_repo.go`
- Create: `src-go/internal/eventbus/repository/dlq_repo.go`
- Test: `src-go/internal/eventbus/repository/events_repo_test.go`

Use the repo's existing test DB harness (see `src-go/internal/repository/*_test.go` for pattern; probably a `testdb.Setup(t)` helper). Follow existing conventions verbatim.

- [ ] **Step 1: Read an existing repository + its test for pattern**

Read `src-go/internal/repository/agent_event_repo.go` and `agent_event_repo_test.go` fully. Mirror the test setup helper and naming conventions.

- [ ] **Step 2: Write failing test for Insert + FindByID**

```go
// src-go/internal/eventbus/repository/events_repo_test.go
package repository_test

import (
    "context"
    "encoding/json"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/react-go-quick-starter/server/internal/eventbus/repository"
    // use the same test-DB helper as existing repo tests
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEventsRepo_InsertThenFind(t *testing.T) {
    db := setupTestDB(t) // copy from existing repo tests
    repo := repository.NewEventsRepository(db)

    e := eb.NewEvent("task.created", "core", "task:abc-1")
    e.Payload = json.RawMessage(`{"id":"abc-1"}`)
    eb.SetString(e, eb.MetaProjectID, "11111111-1111-1111-1111-111111111111")
    require.NoError(t, repo.Insert(context.Background(), e))

    got, err := repo.FindByID(context.Background(), e.ID)
    require.NoError(t, err)
    assert.Equal(t, "task.created", got.Type)
    assert.Equal(t, "task:abc-1", got.Target)
}
```

- [ ] **Step 3: Implement events_repo.go**

```go
// src-go/internal/eventbus/repository/events_repo.go
package repository

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/jmoiron/sqlx"
)

type EventsRepository struct {
    db *sqlx.DB
}

func NewEventsRepository(db *sqlx.DB) *EventsRepository {
    return &EventsRepository{db: db}
}

const insertSQL = `
INSERT INTO events (id, type, source, target, visibility, payload, metadata, project_id, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO NOTHING
`

func (r *EventsRepository) Insert(ctx context.Context, e *eb.Event) error {
    meta, err := json.Marshal(e.Metadata)
    if err != nil {
        return fmt.Errorf("encode metadata: %w", err)
    }
    var projectID sql.NullString
    if pid := eb.GetString(e, eb.MetaProjectID); pid != "" {
        projectID.String = pid
        projectID.Valid = true
    }
    _, err = r.db.ExecContext(ctx, insertSQL,
        e.ID, e.Type, e.Source, e.Target, string(e.Visibility),
        []byte(e.Payload), meta, projectID, e.Timestamp,
    )
    return err
}

const findByIDSQL = `
SELECT id, type, source, target, visibility, payload, metadata, occurred_at
FROM events WHERE id = $1
`

type row struct {
    ID         string          `db:"id"`
    Type       string          `db:"type"`
    Source     string          `db:"source"`
    Target     string          `db:"target"`
    Visibility string          `db:"visibility"`
    Payload    []byte          `db:"payload"`
    Metadata   []byte          `db:"metadata"`
    OccurredAt int64           `db:"occurred_at"`
}

func (r *EventsRepository) FindByID(ctx context.Context, id string) (*eb.Event, error) {
    var rr row
    if err := r.db.GetContext(ctx, &rr, findByIDSQL, id); err != nil {
        return nil, err
    }
    e := &eb.Event{
        ID: rr.ID, Type: rr.Type, Source: rr.Source, Target: rr.Target,
        Visibility: eb.Visibility(rr.Visibility),
        Payload:    json.RawMessage(rr.Payload),
        Timestamp:  rr.OccurredAt,
    }
    if len(rr.Metadata) > 0 {
        _ = json.Unmarshal(rr.Metadata, &e.Metadata)
    }
    return e, nil
}
```

- [ ] **Step 4: Implement dlq_repo.go**

```go
// src-go/internal/eventbus/repository/dlq_repo.go
package repository

import (
    "context"
    "encoding/json"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/jmoiron/sqlx"
)

type DeadLetterRepository struct { db *sqlx.DB }

func NewDeadLetterRepository(db *sqlx.DB) *DeadLetterRepository {
    return &DeadLetterRepository{db: db}
}

const dlqInsertSQL = `
INSERT INTO events_dead_letter (event_id, envelope, last_error, retry_count)
VALUES ($1, $2, $3, $4)
`

func (r *DeadLetterRepository) Record(ctx context.Context, e *eb.Event, err error, retries int) error {
    env, jerr := json.Marshal(e)
    if jerr != nil {
        return jerr
    }
    _, dberr := r.db.ExecContext(ctx, dlqInsertSQL, e.ID, env, err.Error(), retries)
    return dberr
}
```

- [ ] **Step 5: Run repo tests**

Run: `cd src-go && go test ./internal/eventbus/repository/ -v`

- [ ] **Step 6: Commit**

```bash
rtk git commit -am "feat(eventbus): events + DLQ repositories with real-DB tests"
```

---

## Task 14: core.persist observer (TDD with DLQ)

**Files:**
- Create: `src-go/internal/eventbus/mods/persist.go`
- Test: `src-go/internal/eventbus/mods/persist_test.go`

- [ ] **Step 1: Tests**

Use a small interface so the test can inject fakes:

```go
// src-go/internal/eventbus/mods/persist_test.go
package mods

import (
    "context"
    "errors"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/stretchr/testify/assert"
)

type fakeWriter struct {
    inserts  []*eb.Event
    failOnce error
    dlq      []*eb.Event
    dlqErr   []error
}
func (f *fakeWriter) Insert(ctx context.Context, e *eb.Event) error {
    if f.failOnce != nil {
        err := f.failOnce
        f.failOnce = nil
        return err
    }
    f.inserts = append(f.inserts, e)
    return nil
}
func (f *fakeWriter) RecordDead(ctx context.Context, e *eb.Event, err error, retries int) error {
    f.dlq = append(f.dlq, e)
    f.dlqErr = append(f.dlqErr, err)
    return nil
}

func TestPersist_HappyPath(t *testing.T) {
    w := &fakeWriter{}
    p := NewPersistWithDeps(w)
    p.Observe(context.Background(), eb.NewEvent("task.created", "core", "task:1"), &eb.PipelineCtx{})
    assert.Len(t, w.inserts, 1)
}

func TestPersist_FailureRoutesToDLQ(t *testing.T) {
    w := &fakeWriter{failOnce: errors.New("boom")}
    p := NewPersistWithDeps(w)
    p.retries = 0 // no retry for test
    p.Observe(context.Background(), eb.NewEvent("task.created", "core", "task:1"), &eb.PipelineCtx{})
    assert.Len(t, w.dlq, 1)
    assert.EqualError(t, w.dlqErr[0], "boom")
}
```

- [ ] **Step 2: Implement persist.go**

```go
// src-go/internal/eventbus/mods/persist.go
package mods

import (
    "context"
    "time"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    log "github.com/sirupsen/logrus"
)

// eventWriter is the minimal interface needed by Persist so tests can fake it.
type eventWriter interface {
    Insert(ctx context.Context, e *eb.Event) error
    RecordDead(ctx context.Context, e *eb.Event, cause error, retries int) error
}

type Persist struct {
    writer  eventWriter
    retries int
    backoff time.Duration
}

func NewPersistWithDeps(w eventWriter) *Persist {
    return &Persist{writer: w, retries: 2, backoff: 100 * time.Millisecond}
}

func (p *Persist) Name() string         { return "core.persist" }
func (p *Persist) Intercepts() []string { return []string{"*"} }
func (p *Persist) Priority() int        { return 10 }
func (p *Persist) Mode() eb.Mode        { return eb.ModeObserve }

func (p *Persist) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
    var lastErr error
    for attempt := 0; attempt <= p.retries; attempt++ {
        if err := p.writer.Insert(ctx, e); err == nil {
            return
        } else {
            lastErr = err
        }
        if attempt < p.retries {
            time.Sleep(p.backoff)
        }
    }
    if lastErr != nil {
        log.WithFields(log.Fields{"event_id": e.ID, "err": lastErr}).Warn("eventbus: persist failed, routing to DLQ")
        if dlErr := p.writer.RecordDead(ctx, e, lastErr, p.retries); dlErr != nil {
            log.WithFields(log.Fields{"event_id": e.ID, "err": dlErr}).Error("eventbus: DLQ write failed")
        }
    }
}
```

Add an adapter to compose with the two repos in main.go:

```go
// src-go/internal/eventbus/mods/persist_adapter.go
package mods

import (
    "context"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/react-go-quick-starter/server/internal/eventbus/repository"
)

type repoWriter struct {
    events *repository.EventsRepository
    dlq    *repository.DeadLetterRepository
}

func (w *repoWriter) Insert(ctx context.Context, e *eb.Event) error {
    return w.events.Insert(ctx, e)
}
func (w *repoWriter) RecordDead(ctx context.Context, e *eb.Event, cause error, retries int) error {
    return w.dlq.Record(ctx, e, cause, retries)
}

func NewPersistFromRepos(events *repository.EventsRepository, dlq *repository.DeadLetterRepository) *Persist {
    return NewPersistWithDeps(&repoWriter{events: events, dlq: dlq})
}
```

- [ ] **Step 3: Run, pass, commit**

Run: `cd src-go && go test ./internal/eventbus/mods/ -run Persist -v`

```bash
rtk git commit -am "feat(eventbus): core.persist observer with DLQ fallback (TDD)"
```

---

## Task 15: Refactor ws.Hub — strip Event from hub, keep only client registry

**Files:**
- Modify: `src-go/internal/ws/hub.go`
- Modify: `src-go/internal/ws/events.go` (remove the `Event` struct; keep connection logic untouched in `handler.go`, just remove `BroadcastEvent` on Hub)
- Modify: any callers that refer to `ws.Hub.BroadcastEvent` — **do not fix them yet**. They will be migrated in later tasks. Keep this task strictly about Hub surface.

This is a **breaking** change; the project allows it. Expect the tree to not compile until Tasks 17-22 land. To manage this, introduce the new Hub API in parallel and add a deprecated shim that fails loudly.

> **Tree-broken window:** Tasks 15 through 22 leave the repo in a state where `go build ./...` fails. This is intentional and matches the spec's "full replacement over parallel-old/new patterns". Commit small and often so work is not lost, but **do not push or merge until Task 23 makes the tree green again**. If bisectability matters for your review, squash Tasks 15-23 into a single commit at PR time.

- [ ] **Step 1: Extend Hub with channel-index and per-client subscription set**

```go
// patch src-go/internal/ws/hub.go

// Client gains a subscription set.
type Client struct {
    hub           *Hub
    conn          *websocket.Conn
    send          chan []byte
    userID        string
    projectID     string
    remoteAddr    string
    mu            sync.Mutex
    subscriptions map[string]struct{}
}

func (c *Client) subscribe(channels []string) {
    c.mu.Lock(); defer c.mu.Unlock()
    if c.subscriptions == nil {
        c.subscriptions = map[string]struct{}{}
    }
    for _, ch := range channels {
        c.subscriptions[ch] = struct{}{}
    }
}
func (c *Client) unsubscribe(channels []string) {
    c.mu.Lock(); defer c.mu.Unlock()
    for _, ch := range channels {
        delete(c.subscriptions, ch)
    }
}
func (c *Client) matchesAny(channels []string) bool {
    c.mu.Lock(); defer c.mu.Unlock()
    for _, ch := range channels {
        if _, ok := c.subscriptions[ch]; ok {
            return true
        }
    }
    return false
}

// Hub exposes two new methods that the eventbus ws-fanout mod calls.

// FanoutBytes sends to every client subscribed to any of the given channels.
func (h *Hub) FanoutBytes(data []byte, channels []string) {
    h.mu.RLock(); defer h.mu.RUnlock()
    for c := range h.clients {
        if len(channels) > 0 && !c.matchesAny(channels) {
            continue
        }
        select {
        case c.send <- data:
        default:
        }
    }
}

// BroadcastAllBytes sends to every connected client regardless of subscriptions.
// Use only for Visibility=public events.
func (h *Hub) BroadcastAllBytes(data []byte) {
    h.mu.RLock(); defer h.mu.RUnlock()
    for c := range h.clients {
        select {
        case c.send <- data:
        default:
        }
    }
}
```

- [ ] **Step 2: Delete `BroadcastEvent(event *Event)` from `hub.go`** and the `Event` struct from `events.go`

Before deleting, double-check callers by `grep -rn 'BroadcastEvent' src-go/` — every remaining caller is tracked by Task 1's inventory and will be migrated. You can either:

  a) Comment out `BroadcastEvent` and gradually inline-replace (simpler), or
  b) Delete and accept a broken tree until Tasks 17-22 complete.

Given this project permits full replacement (per spec), pick (b). Ensure your branch is still clean before starting the migration tasks.

- [ ] **Step 3: Add subscribe/unsubscribe message handling**

Extend the WS read loop in `src-go/internal/ws/handler.go` (or wherever incoming frames are parsed) to recognise:

```json
{"op": "subscribe",   "channels": ["..."] }
{"op": "unsubscribe", "channels": ["..."] }
```

Ignore unknown ops (no auto-disconnect). On malformed JSON, send one-shot `{"type":"event.error.rejected","payload":{"reason":"bad frame"}}` and keep connection.

- [ ] **Step 4: Hub tests**

Adapt `src-go/internal/ws/handler_test.go` (currently calls `BroadcastEvent`): update tests to use `FanoutBytes` and `BroadcastAllBytes`. Add tests for subscribe/unsubscribe frame handling.

- [ ] **Step 5: Run Hub tests**

Run: `cd src-go && go test ./internal/ws/ -v`
Expected: passes for Hub; other callers still broken — this is intentional.

- [ ] **Step 6: Commit**

```bash
rtk git commit -am "refactor(ws): strip BroadcastEvent; Hub becomes client registry with channel fanout"
```

---

## Task 16: core.ws-fanout observer (TDD)

**Files:**
- Create: `src-go/internal/eventbus/mods/ws_fanout.go`
- Test: `src-go/internal/eventbus/mods/ws_fanout_test.go`

- [ ] **Step 1: Tests**

```go
// src-go/internal/eventbus/mods/ws_fanout_test.go
package mods

import (
    "context"
    "encoding/json"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/stretchr/testify/assert"
)

type fakeHub struct {
    fanout  [][]byte
    fanoutChs [][]string
    bcast   [][]byte
}
func (h *fakeHub) FanoutBytes(data []byte, channels []string) {
    h.fanout = append(h.fanout, append([]byte(nil), data...))
    h.fanoutChs = append(h.fanoutChs, append([]string(nil), channels...))
}
func (h *fakeHub) BroadcastAllBytes(data []byte) {
    h.bcast = append(h.bcast, append([]byte(nil), data...))
}

func TestWSFanout_ChannelVisibilityUsesChannels(t *testing.T) {
    h := &fakeHub{}
    m := NewWSFanout(h)
    e := eb.NewEvent("task.created", "core", "task:1")
    eb.SetChannels(e, []string{"channel:task:1"})
    m.Observe(context.Background(), e, &eb.PipelineCtx{})
    assert.Len(t, h.fanout, 1)
    assert.Len(t, h.bcast, 0)
    assert.Equal(t, []string{"channel:task:1"}, h.fanoutChs[0])

    // framing sanity
    var envelope struct {
        Channel string           `json:"channel"`
        Event   json.RawMessage  `json:"event"`
    }
    // framing is per-channel or aggregate? spec §4: one frame per delivery.
    // For M1 we send one frame per channel to keep client code symmetric with future M2.
}

func TestWSFanout_PublicUsesBroadcastAll(t *testing.T) {
    h := &fakeHub{}
    m := NewWSFanout(h)
    e := eb.NewEvent("system.notice", "core", "project:p")
    e.Visibility = eb.VisibilityPublic
    m.Observe(context.Background(), e, &eb.PipelineCtx{})
    assert.Len(t, h.bcast, 1)
    assert.Len(t, h.fanout, 0)
}

func TestWSFanout_ModOnlyDoesNothing(t *testing.T) {
    h := &fakeHub{}
    m := NewWSFanout(h)
    e := eb.NewEvent("x.y", "core", "task:1")
    e.Visibility = eb.VisibilityModOnly
    m.Observe(context.Background(), e, &eb.PipelineCtx{})
    assert.Len(t, h.fanout, 0)
    assert.Len(t, h.bcast, 0)
}
```

- [ ] **Step 2: Implement ws_fanout.go**

```go
// src-go/internal/eventbus/mods/ws_fanout.go
package mods

import (
    "context"
    "encoding/json"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    log "github.com/sirupsen/logrus"
)

type HubClient interface {
    FanoutBytes(data []byte, channels []string)
    BroadcastAllBytes(data []byte)
}

type WSFanout struct {
    hub HubClient
}

func NewWSFanout(h HubClient) *WSFanout { return &WSFanout{hub: h} }

func (w *WSFanout) Name() string         { return "core.ws-fanout" }
func (w *WSFanout) Intercepts() []string { return []string{"*"} }
func (w *WSFanout) Priority() int        { return 50 }
func (w *WSFanout) Mode() eb.Mode        { return eb.ModeObserve }

type wsFrame struct {
    Channel string   `json:"channel,omitempty"`
    Event   *eb.Event `json:"event"`
}

func (w *WSFanout) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
    switch e.Visibility {
    case eb.VisibilityModOnly, eb.VisibilityDirect:
        // M1: direct delivery is deferred to M2; mod_only never reaches WS.
        return
    case eb.VisibilityPublic:
        data, err := json.Marshal(wsFrame{Event: e})
        if err != nil {
            log.WithFields(log.Fields{"event_id": e.ID, "err": err}).Warn("ws-fanout: marshal")
            return
        }
        w.hub.BroadcastAllBytes(data)
        return
    }
    // channel visibility: emit one frame per target channel
    for _, ch := range eb.GetChannels(e) {
        data, err := json.Marshal(wsFrame{Channel: ch, Event: e})
        if err != nil {
            continue
        }
        w.hub.FanoutBytes(data, []string{ch})
    }
}
```

- [ ] **Step 3: Run, pass**

Run: `cd src-go && go test ./internal/eventbus/mods/ -run WSFanout -v`

- [ ] **Step 4: Commit**

```bash
rtk git commit -am "feat(eventbus): core.ws-fanout with visibility-aware routing (TDD)"
```

---

## Task 17: im.forward-legacy observer (transitional)

**Files:**
- Create: `src-go/internal/eventbus/mods/im_forward_legacy.go`
- Test: `src-go/internal/eventbus/mods/im_forward_legacy_test.go`

The current `im_event_routing` path has complex fan-out into IM bridge deliveries. For M1 we do not rewrite it; we wrap its existing entrypoint behind an ObserveMod so the bus is the sole producer. Replacement with a clean `core.im-forward` is M4.

- [ ] **Step 1: Read the existing entrypoint**

Grep for `imRouter`, `ImForward`, or the `im_event_routing` file under `src-go/internal/service/`. Identify a single function like `IMRouter.Dispatch(ctx, projectID, eventType, payload)` or similar. Note its signature and call sites.

- [ ] **Step 2: Write the observer wrapping that entrypoint**

```go
// src-go/internal/eventbus/mods/im_forward_legacy.go
package mods

import (
    "context"
    "encoding/json"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    log "github.com/sirupsen/logrus"
)

// LegacyIMRouter is the minimal shape we need from the existing router.
// Adapt its method name to the actual signature discovered in Step 1.
type LegacyIMRouter interface {
    Dispatch(ctx context.Context, projectID, eventType string, payload json.RawMessage) error
}

type IMForwardLegacy struct {
    router LegacyIMRouter
}

func NewIMForwardLegacy(r LegacyIMRouter) *IMForwardLegacy {
    return &IMForwardLegacy{router: r}
}

func (m *IMForwardLegacy) Name() string         { return "im.forward-legacy" }
// Forward only the types the legacy router cared about.
// TODO(M4): replace with a proper core.im-forward covering all categories.
func (m *IMForwardLegacy) Intercepts() []string {
    return []string{"task.*", "review.*", "agent.*", "notification"}
}
func (m *IMForwardLegacy) Priority() int { return 80 }
func (m *IMForwardLegacy) Mode() eb.Mode { return eb.ModeObserve }

func (m *IMForwardLegacy) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
    if m.router == nil {
        return
    }
    pid := eb.GetString(e, eb.MetaProjectID)
    if pid == "" {
        return
    }
    if err := m.router.Dispatch(ctx, pid, e.Type, e.Payload); err != nil {
        log.WithFields(log.Fields{"event_id": e.ID, "err": err}).Warn("im.forward-legacy dispatch failed")
    }
}
```

- [ ] **Step 3: Unit-test with a fake router**

```go
// src-go/internal/eventbus/mods/im_forward_legacy_test.go
package mods

import (
    "context"
    "encoding/json"
    "testing"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/stretchr/testify/assert"
)

type fakeIM struct {
    hits []string
}
func (f *fakeIM) Dispatch(ctx context.Context, projectID, eventType string, payload json.RawMessage) error {
    f.hits = append(f.hits, projectID+"/"+eventType)
    return nil
}

func TestIMForwardLegacy_OnlyForwardsMatchingTypes(t *testing.T) {
    im := &fakeIM{}
    m := NewIMForwardLegacy(im)

    send := func(typ string) {
        e := eb.NewEvent(typ, "core", "task:1")
        eb.SetString(e, eb.MetaProjectID, "p-1")
        m.Observe(context.Background(), e, &eb.PipelineCtx{})
    }
    send("task.created")
    send("review.completed")
    send("workflow.execution.started") // not in Intercepts — but caller (pipeline) filters

    // At observer level, we don't re-check Intercepts; simulate pipeline calling us only for matched types.
    assert.Contains(t, im.hits, "p-1/task.created")
    assert.Contains(t, im.hits, "p-1/review.completed")
}

func TestIMForwardLegacy_NoProjectSkips(t *testing.T) {
    im := &fakeIM{}
    m := NewIMForwardLegacy(im)
    e := eb.NewEvent("task.created", "core", "task:1")
    m.Observe(context.Background(), e, &eb.PipelineCtx{})
    assert.Empty(t, im.hits)
}
```

Run: `cd src-go && go test ./internal/eventbus/mods/ -run IMForwardLegacy -v`

- [ ] **Step 4: Commit**

```bash
rtk git commit -am "feat(eventbus): im.forward-legacy transitional observer (TDD)"
```

---

## Task 18: Wire bus + mods in cmd/server/main.go

**Files:**
- Create: `src-go/internal/eventbus/publisher.go` (Publisher interface — referenced by services in Tasks 19-22)
- Modify: `src-go/cmd/server/main.go`

- [ ] **Step 0: Create the Publisher interface up front**

```go
// src-go/internal/eventbus/publisher.go
package eventbus

import "context"

// Publisher is the minimal interface services take so they can be tested
// with a fake bus. *Bus satisfies it.
type Publisher interface {
    Publish(ctx context.Context, e *Event) error
}
```

Compile check: `cd src-go && go build ./internal/eventbus/...` — should pass independently.

- [ ] **Step 1: Import and construct bus**

Locate where `hub := ws.NewHub()` is created; right after, wire the bus:

```go
import (
    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    ebrepo "github.com/react-go-quick-starter/server/internal/eventbus/repository"
    "github.com/react-go-quick-starter/server/internal/eventbus/mods"
)

// ...
hub := ws.NewHub()
go hub.Run()

eventsRepo := ebrepo.NewEventsRepository(db)
dlqRepo    := ebrepo.NewDeadLetterRepository(db)

bus := eb.NewBus()
bus.Register(mods.NewValidate())
bus.Register(mods.NewAuth())
bus.Register(mods.NewEnrich())
bus.Register(mods.NewChannelRouter())
bus.Register(mods.NewPersistFromRepos(eventsRepo, dlqRepo))
bus.Register(mods.NewWSFanout(hub))
bus.Register(mods.NewMetrics())
if imRouter != nil { // wire existing router instance
    bus.Register(mods.NewIMForwardLegacy(imRouter))
}
// Prometheus registration for the counter:
prometheus.MustRegister(metricsMod.Collector())
```

Adjust to whatever variable names already exist in main.go (`db`, `imRouter`, `metricsMod`). Pass `bus` into every service constructor that previously took `hub` for event-sending:

```go
taskSvc := service.NewTaskService(taskRepo, bus, ...)
agentSvc := service.NewAgentService(agentRepo, bus, ...)
// ... and so on for every service listed in the Task 1 inventory.
```

**Do not** remove `hub` from service constructors that use it for anything other than BroadcastEvent (e.g. plugin_broadcaster uses hub for raw writes too — keep those separate for now; they are migrated in Task 21).

- [ ] **Step 2: Compile**

Run: `cd src-go && go build ./cmd/server`
Expected: likely fails; fix imports and wire-ups incrementally until it builds. The tree is still compiling this task — most service callsites haven't been migrated, so expect method-signature mismatches. Hold commits until Task 18 is done.

Actually: since we changed constructor signatures, services will not compile until every producer service in Tasks 19-22 is updated. To avoid an unmanageable patch, **defer this commit** to Task 23.

- [ ] **Step 3: Stop here and proceed to Task 19**

Leave the tree broken locally. Do not push.

---

## Task 19: Migrate call-site batch 1 — agent lifecycle (agent_service, task_dispatch_service)

**Files:**
- Modify: `src-go/internal/service/agent_service.go`
- Modify: `src-go/internal/service/task_dispatch_service.go`
- Delete: `src-go/internal/model/agent_event.go` (struct gone; table dropped by migration)
- Delete: `src-go/internal/repository/agent_event_repo.go` and `..._test.go`

- [ ] **Step 1: In `agent_service.go`**, replace every:

```go
s.hub.BroadcastEvent(&ws.Event{Type: ws.EventAgentStarted, ProjectID: pid, Payload: X})
```

with:

```go
e := eb.NewEvent(eb.EventAgentStarted, eb.MakeAgent(runID), eb.MakeAgent(runID))
eb.SetString(e, eb.MetaProjectID, pid)
payload, _ := json.Marshal(X)
e.Payload = payload
_ = s.bus.Publish(ctx, e)
```

Where `eb.EventAgentStarted` is the constant now living in `internal/eventbus/types.go` (moved in Task 2).

Also replace every `s.agentEventRepo.Create(...)` — those rows are now written by `core.persist`. Delete the call entirely.

- [ ] **Step 2: Same for `task_dispatch_service.go`** — see Task 1 inventory for the 5 call sites.

- [ ] **Step 3: Delete `agent_event.go` model + its repo**

```bash
rm src-go/internal/model/agent_event.go
rm src-go/internal/repository/agent_event_repo.go
rm src-go/internal/repository/agent_event_repo_test.go
```

- [ ] **Step 4: Update agent_service tests**

Any test that asserted on `hub.BroadcastEvent` mocks — swap to a `FakeBus` implementing `Publish(ctx, *eb.Event) error` and assert on event Type/Target.

```go
// src-go/internal/service/fakebus_test.go (shared helper)
package service_test

import (
    "context"
    eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type FakeBus struct { Events []*eb.Event }
func (f *FakeBus) Publish(ctx context.Context, e *eb.Event) error { f.Events = append(f.Events, e); return nil }
```

- [ ] **Step 5: Run the tests for this batch**

Run: `cd src-go && go test ./internal/service/ -run "TestAgent|TestTaskDispatch" -v`

- [ ] **Step 6: Partial commit**

Tree may still not build; commit locally without push.

```bash
rtk git add -A
rtk git commit -m "refactor(eventbus): migrate agent lifecycle callsites onto bus.Publish"
```

---

## Task 20: Migrate call-site batch 2 — tasks, reviews, notifications

**Files (from Task 1 inventory):**
- `src-go/internal/service/task_service.go` (6 sites)
- `src-go/internal/service/task_workflow_service.go` (1)
- `src-go/internal/service/task_progress_service.go` (3)
- `src-go/internal/service/task_comment_service.go` (1)
- `src-go/internal/service/entity_link_service.go` (1)
- `src-go/internal/service/review_service.go` (1)
- `src-go/internal/service/notification_service.go` (1)
- `src-go/internal/service/team_service.go` (1)
- `src-go/internal/service/wiki_service.go` — check for BroadcastEvent usage
- `src-go/internal/service/cost_service.go` (2)
- `src-go/internal/service/log_service.go` (1)
- `src-go/internal/service/dag_workflow_service.go` (1)
- `src-go/internal/handler/task_handler.go` (3)
- `src-go/internal/handler/sprint_handler.go` (1)

- [ ] **Step 1: For each file, convert every `hub.BroadcastEvent(&ws.Event{...})` to `bus.Publish(ctx, eb.NewEvent(...))`**

Use this transformation pattern:

| Before | After |
| --- | --- |
| `&ws.Event{Type: ws.EventX, ProjectID: pid, Payload: p}` | `eb.NewEvent(eb.EventX, source, target)` + set project metadata + marshal payload + `bus.Publish(ctx, e)` |

Pick `source` and `target` with judgment:
- Task events: `source = "core"` (or `user:<uid>` if the mutation is user-driven); `target = task:<taskID>`
- Review events: `source = "core"`; `target = "task:<taskID>"` (or `review:<id>` if reviews have IDs exposed)
- Notifications: `source = "core"`; `target = user:<uid>`

- [ ] **Step 2: Thread `bus` into constructors** — add `bus eb.Publisher` parameter to every service that used `hub.BroadcastEvent`. Define a small interface in `eventbus`:

```go
// src-go/internal/eventbus/publisher.go
package eventbus

import "context"

type Publisher interface {
    Publish(ctx context.Context, e *Event) error
}
```

Services take `Publisher`, not `*Bus`, for testability.

- [ ] **Step 3: Update each service's test to inject FakeBus**

- [ ] **Step 4: Commit per sub-file (to keep reviewable diffs)**

```bash
rtk git add src-go/internal/service/task_service.go src-go/internal/service/task_service_test.go
rtk git commit -m "refactor(eventbus): migrate task_service to bus.Publish"
# repeat for each file
```

---

## Task 21: Migrate call-site batch 3 — broadcasters (plugin, scheduler)

**Files:**
- Modify: `src-go/internal/ws/plugin_broadcaster.go`
- Modify: `src-go/internal/ws/plugin_broadcaster_test.go`
- Modify: `src-go/internal/ws/scheduler_broadcaster.go`
- Modify: `src-go/internal/service/plugin_service.go` and its tests (13 sites for PluginEventRecord)
- Delete: `src-go/internal/repository/plugin_event_repo.go` + test
- Delete: the `PluginEventRecord` type from `src-go/internal/model/plugin.go` (keep other types in that file)

- [ ] **Step 1: Migrate `plugin_broadcaster.go`** from calling `hub.BroadcastEvent` to calling `bus.Publish`. The broadcaster becomes a thin adapter that converts its typed input into an `eb.Event` with `source = plugin:<id>`, `target = plugin:<id>`.

- [ ] **Step 2: Delete PluginEventRecord + its repo**

```bash
rm src-go/internal/repository/plugin_event_repo.go src-go/internal/repository/plugin_event_repo_test.go
# Edit src-go/internal/model/plugin.go — remove PluginEventRecord struct only
```

In every plugin_service.go callsite that wrote PluginEventRecord, replace with `bus.Publish(ctx, eb.NewEvent(eb.EventPluginLifecycle, ...))`.

- [ ] **Step 3: Same for scheduler_broadcaster.go** — one site.

- [ ] **Step 4: Run**

```bash
cd src-go && go test ./internal/ws/ ./internal/service/ -run "Plugin|Scheduler" -v
```

- [ ] **Step 5: Commit**

```bash
rtk git add -A
rtk git commit -m "refactor(eventbus): migrate plugin/scheduler broadcasters; remove PluginEventRecord"
```

---

## Task 22: Migrate call-site batch 4 — bridge ingress

**Files:**
- Modify: `src-go/internal/ws/bridge_handler.go`
- Modify: `src-go/internal/ws/bridge_handler_test.go`
- Delete: `BridgeAgentEvent` struct from `src-go/internal/ws/events.go`
- Delete: `src-go/internal/ws/events_test.go` entries that reference `BridgeAgentEvent`

- [ ] **Step 1: In `bridge_handler.go`**, when the TS Bridge sends a frame (currently decoded into `BridgeAgentEvent`), decode it into a minimal ingress DTO and **translate it inside the handler** into an `eb.Event`. Do not yet share a full schema with the TS Bridge — that is M3.

```go
// inline DTO (private to the handler, M1-only)
type bridgeIngress struct {
    TaskID      string          `json:"task_id"`
    SessionID   string          `json:"session_id"`
    TimestampMS int64           `json:"timestamp_ms"`
    Type        string          `json:"type"`
    Data        json.RawMessage `json:"data"`
}

// ... inside the handler ...
e := &eb.Event{
    ID:         ulid.Make().String(),
    Type:       "agent." + in.Type, // agent.output, agent.tool_call, etc.
    Source:     "agent:" + in.SessionID,
    Target:     "task:" + in.TaskID,
    Payload:    in.Data,
    Metadata:   map[string]any{},
    Timestamp:  in.TimestampMS,
    Visibility: eb.VisibilityChannel,
}
// Look up project ID for this task, attach to metadata.
if pid, err := h.tasks.GetProjectID(ctx, in.TaskID); err == nil {
    eb.SetString(e, eb.MetaProjectID, pid)
}
_ = h.bus.Publish(ctx, e)
```

- [ ] **Step 2: Delete the `BridgeAgentEvent` struct** from `ws/events.go`.

- [ ] **Step 3: Update tests**

Run: `cd src-go && go test ./internal/ws/ -run Bridge -v`

- [ ] **Step 4: Commit**

```bash
rtk git add -A
rtk git commit -m "refactor(eventbus): migrate bridge ingress to bus.Publish; remove BridgeAgentEvent"
```

---

## Task 23: Finish main.go wiring + full-tree build

**Files:**
- Modify: `src-go/cmd/server/main.go` (finish what Task 18 started)

- [ ] **Step 1: Confirm every service constructor now takes `eb.Publisher`**

Search: `rtk grep -n 'func New.*Service' src-go/internal/service/`
For each, verify the signature has been updated.

- [ ] **Step 2: Update main.go to pass `bus` to every service**

- [ ] **Step 3: Full build**

Run: `cd src-go && go build ./...`
Expected: passes. Fix any remaining compile errors iteratively.

- [ ] **Step 4: Full unit test suite**

Run: `cd src-go && go test ./...`
Expected: passes.

- [ ] **Step 5: Commit**

```bash
rtk git add -A
rtk git commit -m "refactor(eventbus): finalize main.go wiring; tree builds and tests pass"
```

---

## Task 24: Legacy cleanup

**Files:**
- Modify: `src-go/internal/ws/events.go` — keep only type-related helper funcs, no structs; or delete file entirely if empty
- Delete: `src-go/internal/ws/events_test.go` (if its only tests were for deleted structs)
- Modify: `docs/api/asyncapi.yaml` (mark channels as deprecated v1; full redesign in M3)

- [ ] **Step 1: Verify no remaining references to deleted types**

```bash
rtk grep -rn 'AgentEvent\|PluginEventRecord\|BridgeAgentEvent\|ws\.Event\b' src-go/
```
Expected: no results in non-comment lines. (If any, they must be removed before this task closes.)

- [ ] **Step 2: Delete or empty `src-go/internal/ws/events.go`**

If the string constants moved to `eventbus/types.go` in Task 2, this file has nothing left. Delete it.

- [ ] **Step 3: Update `docs/api/asyncapi.yaml`** — add a prominent comment at the top:

```yaml
# NOTE: M1 of the Unified Event Bus (2026-04-16) changed the WebSocket envelope.
# This document is accurate for the M1 channel subscription protocol; the bridge
# ingress envelope becomes canonical in M3 and this doc will then be regenerated.
```

- [ ] **Step 4: Delete the callsite inventory artifact from Task 1**

```bash
rm docs/superpowers/plans/callsite-inventory-eventbus-m1.md
```

- [ ] **Step 5: Commit**

```bash
rtk git add -A
rtk git commit -m "chore(eventbus): delete dead code; document M1 state in asyncapi"
```

---

## Task 25: Integration test — end-to-end event flow

**Files:**
- Create: `src-go/internal/eventbus/integration_test.go`

- [ ] **Step 1: Wire a real Bus with all 7 core mods against a real test DB**

```go
// src-go/internal/eventbus/integration_test.go
package eventbus_test

import (
    "context"
    "testing"
    "time"

    eb "github.com/react-go-quick-starter/server/internal/eventbus"
    "github.com/react-go-quick-starter/server/internal/eventbus/mods"
    "github.com/react-go-quick-starter/server/internal/eventbus/repository"
    "github.com/stretchr/testify/require"
)

func TestIntegration_TaskCreatedFlowsEndToEnd(t *testing.T) {
    db := setupIntegrationDB(t)

    bus := eb.NewBus()
    bus.Register(mods.NewValidate())
    bus.Register(mods.NewAuth())
    bus.Register(mods.NewEnrich())
    bus.Register(mods.NewChannelRouter())
    bus.Register(mods.NewPersistFromRepos(
        repository.NewEventsRepository(db),
        repository.NewDeadLetterRepository(db),
    ))
    // skip ws-fanout/im-forward in this test — they're mocked elsewhere

    e := eb.NewEvent(eb.EventTaskCreated, "core", "task:abc-1")
    eb.SetString(e, eb.MetaProjectID, "11111111-1111-1111-1111-111111111111")
    require.NoError(t, bus.Publish(context.Background(), e))

    time.Sleep(200 * time.Millisecond) // allow observe parallel fan-out

    got, err := repository.NewEventsRepository(db).FindByID(context.Background(), e.ID)
    require.NoError(t, err)
    require.Equal(t, eb.EventTaskCreated, got.Type)
    require.Contains(t, eb.GetChannels(got), "channel:task:abc-1")
    require.Contains(t, eb.GetChannels(got), "channel:project:11111111-1111-1111-1111-111111111111")
}
```

- [ ] **Step 2: Chaos test — panic in persist still lets metrics run**

```go
func TestIntegration_ObservePanicIsolated(t *testing.T) {
    bus := eb.NewBus()
    panicMod := &panicObserver{}
    goodMod := &countObserver{}
    bus.Register(panicMod); bus.Register(goodMod)
    _ = bus.Publish(context.Background(), eb.NewEvent("x.y", "core", "task:1"))
    time.Sleep(50 * time.Millisecond)
    require.Equal(t, 1, goodMod.n)
}
```

(Define `panicObserver` / `countObserver` helpers in the test file.)

- [ ] **Step 3: Run**

```bash
cd src-go && go test ./internal/eventbus/ -run TestIntegration -v
```

- [ ] **Step 4: Commit**

```bash
rtk git commit -am "test(eventbus): integration tests with DB + chaos"
```

---

## Task 26: Full regression — verify nothing user-visible broke

- [ ] **Step 1: Run full Go test suite**

```bash
cd src-go && go test ./...
```

- [ ] **Step 2: Run frontend test suite (stores/components may depend on payload shape)**

```bash
pnpm test
```

- [ ] **Step 3: Start the full stack and smoke**

```bash
pnpm dev:backend:stop    # clean slate
pnpm dev:backend:verify  # brings up PG + Redis + Go + Bridge + IM Bridge with health checks + smoke
```

Then manually:
- Log in, create a project, create a task, assign an agent run.
- Confirm WS frames arrive with the new envelope (open devtools Network → WS frames, check one has `{"channel": ..., "event": {...}}`).
- Confirm `SELECT COUNT(*) FROM events;` > 0 in Postgres.
- Confirm `SELECT COUNT(*) FROM events_dead_letter;` = 0.

- [ ] **Step 4: Record findings**

Append a short verification note to `docs/superpowers/specs/2026-04-16-unified-event-bus-design.md` under a new "M1 Verification" section:
- Pass/fail of each check above
- Total event count after smoke
- DLQ size

- [ ] **Step 5: Final commit**

```bash
rtk git add docs/superpowers/specs/2026-04-16-unified-event-bus-design.md
rtk git commit -m "docs(eventbus): record M1 verification results"
```

---

## Task 27: PR

- [ ] **Step 1: Push and open PR**

```bash
rtk git push -u origin feat/eventbus-m1
```

Open PR with this template body:

```
## Summary
- Introduce `internal/eventbus` package: Event envelope, URI addressing, guard/transform/observe pipeline, 7 core mods
- Replace `AgentEvent`, `PluginEventRecord`, `ws.Event`, `BridgeAgentEvent` with single envelope
- Migration 054: new `events` + `events_dead_letter` tables; `agent_events` dropped
- `hub.BroadcastEvent` removed; `core.ws-fanout` owns fan-out; Hub now pure client registry
- IM routing wrapped as transitional `im.forward-legacy` observer (replaced in M4)

## Test plan
- [ ] go test ./... (src-go)
- [ ] pnpm test
- [ ] pnpm dev:backend:verify
- [ ] Manual smoke: create task, start agent run, verify WS frame shape and DB rows

## Spec
docs/superpowers/specs/2026-04-16-unified-event-bus-design.md

## Follow-ups (not in this PR)
- M2: frontend channel subscribe migration
- M3: Bridge envelope convergence
- M4: IM Bridge proper rewrite; rate-limit + audit mods
```

---

## Cross-cutting notes

**Commits:** aim for 15-25 commits total. Prefer many small commits over few big ones.

**If tests at Task 26 fail in subtle ways:** do NOT skip them. The most common failure mode is an event that was fire-and-forget on the old path but now requires a project_id enrichment; fix by writing a targeted enrich rule or by making the service call provide `source` and `target` correctly. Add a regression test when you do.

**If a service really has no meaningful `target`** (e.g. pure system heartbeat), use `target = "core"`. Do not leave it empty — `core.validate` will reject.

**Skills to use while executing this plan:**
- @superpowers:test-driven-development for every non-integration task
- @superpowers:systematic-debugging when any test surprises you
- @superpowers:verification-before-completion before marking Task 26 green
- @superpowers:subagent-driven-development or @superpowers:executing-plans depending on chosen execution mode
