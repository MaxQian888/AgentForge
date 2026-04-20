# Spec 3A — adsplatform.Provider Interface + Qianchuan Client + Bindings CRUD

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 3 §6.1 qianchuan_bindings + adsplatform.Provider 抽象 + Qianchuan 实现（API 客户端 + 5 个安全 action primitive + 4 个 fetch 方法）+ bindings CRUD + FE 绑定列表页。

**Architecture:** Provider-neutral 接口（mirror Spec 2 vcs.Provider 模式）+ Qianchuan 实现走签名请求 + 仅 5 个动作 primitive 暴露（避免不可控的 OpenAPI 透传）+ tokens 经 1B secrets store 加密存储 + binding CRUD + FE 绑定列表（Plan 3B 加 OAuth 按钮）。

**Tech Stack:** Go (HTTP client + crypto signing), Postgres, Next.js 16 App Router, Zustand, shadcn/ui.

**Depends on:** Spec 1B (secrets store)

**Parallel with:** 3C (strategy authoring) — independent

**Unblocks:** 3B (OAuth flow refines token storage), 3D (workflow loop calls Provider.FetchMetrics + actions), 3E (action policy gates wrap action primitive calls)

---

## Coordination notes (read before starting)

- **Migration number**: Spec 3 §6 reserves 070–074 for the Qianchuan tables, but Spec 1 plans 1A–1E already claim 067–069 and Spec 2 plans (2A onward) are expected to claim 070+. The next free number after the last applied migration (`066_workflow_run_parent_link_parent_kind`) is dynamically assigned: when starting this plan, **glob `src-go/migrations/*.up.sql`, take `max(N)+1`** and reserve only that one number for `qianchuan_bindings`. The other Spec-3 tables (strategies/snapshots/action_logs/oauth_states) belong to other 3* plans. Reference labels (e.g. "[bindings migration]") in the spec are decoupled from numbers.
- **Spec drift**: Spec 3 §6.1 lists the binding row with both `employee_id` (NOT NULL) and `policy` / `strategy_id` / `tick_interval_sec` / `trigger_id`. The user-supplied 3A scope shrinks the row to: `acting_employee_id` (nullable), `display_name`, `status`, `access_token_secret_ref`, `refresh_token_secret_ref`, `token_expires_at`, `last_synced_at` and **omits** policy/strategy/trigger/tick_interval. Those columns are owned by Plan 3C (strategy) and Plan 3D (workflow loop) which `ALTER TABLE` later. **Do not** add them in 3A. Record this drift in §13.1 of the spec when this plan lands.
- **`acting_employee_id` vs `employee_id`**: Spec uses `employee_id NOT NULL`; this plan uses `acting_employee_id UUID NULL` (mirrors the Spec 2 vcs_integrations row + AgentForge convention from migration 064 / 065). One binding may have no employee yet at creation; the FE later attaches one. Drift recorded — Spec 3 §6.1 should be amended.
- **Signed request algorithm**: Qianchuan OpenAPI uses bearer-token auth (`Access-Token` header) — the reference repo `client.ts` injects `Access-Token: <accessToken>` per request and does **not** HMAC-sign the body. There is no body-signing step despite the user's "signed request" phrasing in §A3; what we implement is bearer-token + retry + redact. Document in spec drift.
- **Action surface is finite**: Provider exposes exactly 5 action methods (`AdjustBid`, `AdjustBudget`, `PauseAd`, `ResumeAd`, `ApplyMaterial`). No `RawCall` / `Passthrough`. Adding a 6th is a code-review gate per Spec 3 §11.
- **Token resolution**: Provider methods accept a `BindingRef` containing `AccessToken` already resolved by the caller (handler / future workflow node). The Provider package never imports `internal/secrets`. This keeps the Provider unit-testable without secrets bootstrap. The handler is responsible for calling `secrets.Service.Resolve` and passing plaintext into `BindingRef.AccessToken` for the lifetime of the call.
- **Audit hook**: New ActionIDs `qianchuan_binding.create`, `qianchuan_binding.update`, `qianchuan_binding.delete`, `qianchuan_binding.sync`, `qianchuan_binding.test` are added to `middleware/rbac.go`. The audit `resource_type` enum gets a new value `qianchuan_binding` via the same migration (mirror of Spec 1B's `secret` extension).
- **Big-int IDs**: Reference TS uses `safeJsonParse` to keep `room_id` / `order_id` as strings. Go `json.Number` solves the same problem; struct fields use either `json.Number` for unknown numeric fields or `string` tag for known ID fields. Document fix in spec drift.
- **FE project nav**: `app/(dashboard)/projects/[id]/` does not exist; Plan 1B creates the layout shell for `/secrets`. If 1B has not yet landed, this plan adds an analogous shell at `app/(dashboard)/projects/[id]/qianchuan/bindings/page.tsx` and a minimal layout that renders only the page (no shared sidebar yet). When 1B lands, the layouts merge.

---

## Task 1 — Migration: qianchuan_bindings table + audit resource_type extension

- [x] Step 1.1 — pick migration number
  - Run `rtk ls src-go/migrations/*.up.sql | sort | tail -1` → take the trailing `NNN`, set `MIG = NNN+1`. The plan text below uses `MIG` as a placeholder; substitute the real number when committing. **Used MIG = 071** (070 was claimed by Plan 3C qianchuan_strategies).

- [x] Step 1.2 — write the up migration
  - File: `src-go/migrations/MIG_create_qianchuan_bindings.up.sql`
    ```sql
    -- Project-scoped Qianchuan (巨量千川) advertiser bindings.
    -- Tokens are NOT stored here; the *_secret_ref columns reference
    -- secrets.name rows owned by Plan 1B's secrets store.
    --
    -- Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §6.1
    --   (Spec drift: 3A omits policy / strategy_id / trigger_id / tick_interval_sec;
    --    those columns are added by Plan 3C / 3D ALTER TABLEs.)
    CREATE TABLE IF NOT EXISTS qianchuan_bindings (
        id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        project_id               UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        advertiser_id            VARCHAR(64) NOT NULL,
        aweme_id                 VARCHAR(64),
        display_name             VARCHAR(128),
        status                   VARCHAR(16) NOT NULL DEFAULT 'active'
                                 CHECK (status IN ('active','auth_expired','paused')),
        acting_employee_id       UUID REFERENCES employees(id) ON DELETE SET NULL,
        access_token_secret_ref  VARCHAR(128) NOT NULL,
        refresh_token_secret_ref VARCHAR(128) NOT NULL,
        token_expires_at         TIMESTAMPTZ,
        last_synced_at           TIMESTAMPTZ,
        created_by               UUID NOT NULL,
        created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
        UNIQUE (project_id, advertiser_id, aweme_id)
    );
    CREATE INDEX IF NOT EXISTS qianchuan_bindings_project_idx
        ON qianchuan_bindings(project_id);
    CREATE INDEX IF NOT EXISTS qianchuan_bindings_status_idx
        ON qianchuan_bindings(status) WHERE status IN ('active','auth_expired');

    CREATE TRIGGER set_qianchuan_bindings_updated_at
        BEFORE UPDATE ON qianchuan_bindings
        FOR EACH ROW
        EXECUTE FUNCTION update_updated_at_column();

    -- Extend audit resource_type CHECK so qianchuan_binding.* events can persist.
    ALTER TABLE project_audit_events
        DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
    ALTER TABLE project_audit_events
        ADD CONSTRAINT project_audit_events_resource_type_check
        CHECK (resource_type IN (
            'project','member','task','team_run','workflow',
            'wiki','settings','automation','dashboard','auth',
            'invitation','secret','qianchuan_binding'
        ));
    ```
  - Note: the CHECK list **must** include every previously valid resource_type. If Plan 1B did not yet land `secret`, drop it from the list above and add `secret` separately when 1B lands. Coordinate via the migration number ordering.

- [x] Step 1.3 — write the down migration
  - File: `src-go/migrations/MIG_create_qianchuan_bindings.down.sql`
    ```sql
    DROP TRIGGER IF EXISTS set_qianchuan_bindings_updated_at ON qianchuan_bindings;
    DROP INDEX IF EXISTS qianchuan_bindings_status_idx;
    DROP INDEX IF EXISTS qianchuan_bindings_project_idx;
    DROP TABLE IF EXISTS qianchuan_bindings;

    ALTER TABLE project_audit_events
        DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
    ALTER TABLE project_audit_events
        ADD CONSTRAINT project_audit_events_resource_type_check
        CHECK (resource_type IN (
            'project','member','task','team_run','workflow',
            'wiki','settings','automation','dashboard','auth',
            'invitation','secret'
        ));
    ```

- [x] Step 1.4 — extend `internal/model/audit_event.go`
  - Add `AuditResourceTypeQianchuanBinding = "qianchuan_binding"`
  - Append it to the validity match in `IsValidAuditResourceType`.

- [x] Step 1.5 — verify
  - `rtk go test ./internal/model/...`
  - `rtk pnpm dev:backend:restart go-orchestrator` and confirm migration applied.

- [x] Step 1.6 — commit `feat(qianchuan): add qianchuan_bindings table + audit resource_type`

---

## Task 2 — `internal/adsplatform` interface + types + registry (TDD)

- [x] Step 2.1 — write failing registry test
  - File: `src-go/internal/adsplatform/registry_test.go`
    ```go
    package adsplatform_test

    import (
        "context"
        "errors"
        "testing"
        "time"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
    )

    type stubProvider struct{ name string }

    func (s *stubProvider) Name() string { return s.name }
    func (*stubProvider) OAuthAuthorizeURL(context.Context, string, string) (string, error) {
        return "", errors.ErrUnsupported
    }
    func (*stubProvider) OAuthExchange(context.Context, string, string) (*adsplatform.Tokens, error) {
        return nil, errors.ErrUnsupported
    }
    func (*stubProvider) RefreshToken(context.Context, string) (*adsplatform.Tokens, error) {
        return &adsplatform.Tokens{AccessToken: "a", RefreshToken: "r", ExpiresAt: time.Now()}, nil
    }
    func (*stubProvider) FetchMetrics(context.Context, adsplatform.BindingRef, adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
        return &adsplatform.MetricSnapshot{}, nil
    }
    func (*stubProvider) FetchLiveSession(context.Context, adsplatform.BindingRef, string) (*adsplatform.LiveSession, error) {
        return &adsplatform.LiveSession{}, nil
    }
    func (*stubProvider) FetchMaterialHealth(context.Context, adsplatform.BindingRef, []string) ([]adsplatform.MaterialHealth, error) {
        return nil, nil
    }
    func (*stubProvider) AdjustBid(context.Context, adsplatform.BindingRef, string, adsplatform.Money) error {
        return nil
    }
    func (*stubProvider) AdjustBudget(context.Context, adsplatform.BindingRef, string, adsplatform.Money) error {
        return nil
    }
    func (*stubProvider) PauseAd(context.Context, adsplatform.BindingRef, string) error  { return nil }
    func (*stubProvider) ResumeAd(context.Context, adsplatform.BindingRef, string) error { return nil }
    func (*stubProvider) ApplyMaterial(context.Context, adsplatform.BindingRef, string, string) error {
        return nil
    }

    func TestRegistry_RegisterAndResolve(t *testing.T) {
        reg := adsplatform.NewRegistry()
        reg.Register("stub", func() adsplatform.Provider { return &stubProvider{name: "stub"} })
        got, err := reg.Resolve("stub")
        if err != nil {
            t.Fatalf("Resolve: %v", err)
        }
        if got.Name() != "stub" {
            t.Errorf("got %q, want stub", got.Name())
        }
    }

    func TestRegistry_ResolveUnknown(t *testing.T) {
        reg := adsplatform.NewRegistry()
        if _, err := reg.Resolve("nope"); !errors.Is(err, adsplatform.ErrProviderNotFound) {
            t.Fatalf("want ErrProviderNotFound, got %v", err)
        }
    }

    func TestRegistry_RegisterDuplicate(t *testing.T) {
        reg := adsplatform.NewRegistry()
        ctor := func() adsplatform.Provider { return &stubProvider{name: "x"} }
        reg.Register("x", ctor)
        defer func() {
            if recover() == nil {
                t.Fatal("expected panic on duplicate register")
            }
        }()
        reg.Register("x", ctor)
    }
    ```
  - Run `rtk go test ./internal/adsplatform/...` — fails (package missing).

- [x] Step 2.2 — implement provider interface + types
  - File: `src-go/internal/adsplatform/types.go`
    ```go
    // Package adsplatform defines the provider-neutral interface AgentForge uses
    // to talk to advertising platforms (Qianchuan / 巨量千川 today; Taobao /
    // JD Cloud Ads / Kuaishou / TikTok Ads in the future).
    //
    // Design invariants:
    //   * Action surface is finite and auditable. Implementations expose ONLY
    //     the methods on Provider; no raw / passthrough call exists.
    //   * Tokens enter the package as plaintext via BindingRef.AccessToken;
    //     the package does not import internal/secrets. Callers resolve the
    //     secret and pass plaintext for the lifetime of the call.
    //   * Currency is integer minor units (e.g. fen for CNY) to avoid float
    //     drift; see Money.
    //
    // Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §8
    package adsplatform

    import "time"

    // BindingRef is the per-call binding context passed to every Provider
    // method. AccessToken is plaintext and lives only for the call frame.
    type BindingRef struct {
        AdvertiserID string
        AwemeID      string
        AccessToken  string
    }

    // Tokens is the OAuth token tuple returned by OAuthExchange / RefreshToken.
    type Tokens struct {
        AccessToken  string
        RefreshToken string
        ExpiresAt    time.Time
        Scopes       []string
    }

    // Money holds an integer amount of currency minor units (fen for CNY).
    type Money struct {
        Amount   int64  // minor units, e.g. 12345 = ¥123.45 when Currency=="CNY"
        Currency string // ISO 4217; "CNY" for Qianchuan
    }

    // MetricDimensions narrows a FetchMetrics call. Empty fields apply
    // provider-default behaviour (e.g. "today" for Range).
    type MetricDimensions struct {
        Range     string   // "today" | "yesterday" | "1h" | "5m"
        AdIDs     []string // empty → all
        AwemeIDs  []string
        Granular  string   // "minute" | "hour" | "day"
    }

    // MetricSnapshot is the normalized adsplatform metric form. Provider
    // implementations populate Live/Ads/Materials from their native shapes.
    type MetricSnapshot struct {
        BucketAt  time.Time              `json:"bucket_at"`
        Live      map[string]any         `json:"live,omitempty"`
        Ads       []AdMetric             `json:"ads,omitempty"`
        Materials []MaterialHealth       `json:"materials,omitempty"`
        Raw       map[string]any         `json:"raw,omitempty"` // diagnostic only
    }

    // AdMetric is one ad's per-bucket performance.
    type AdMetric struct {
        AdID   string  `json:"ad_id"`
        Status string  `json:"status"`
        Spend  float64 `json:"spend"`
        ROI    float64 `json:"roi"`
        CTR    float64 `json:"ctr"`
        CPM    float64 `json:"cpm"`
        Bid    float64 `json:"bid"`
        Budget float64 `json:"budget"`
    }

    // LiveSession is the current state of one Douyin live room.
    type LiveSession struct {
        AwemeID   string         `json:"aweme_id"`
        RoomID    string         `json:"room_id"`     // string to preserve big-int precision
        Status    string         `json:"status"`      // 'warming' | 'live' | 'ended'
        StartedAt time.Time      `json:"started_at"`
        Viewers   int64          `json:"viewers"`
        GMV       float64        `json:"gmv"`
        Raw       map[string]any `json:"raw,omitempty"`
    }

    // MaterialHealth is the per-creative-asset health snapshot.
    type MaterialHealth struct {
        MaterialID string  `json:"material_id"`
        Status     string  `json:"status"`
        Health     float64 `json:"health"` // 0..1
        Reason     string  `json:"reason,omitempty"`
    }
    ```
  - File: `src-go/internal/adsplatform/provider.go`
    ```go
    package adsplatform

    import "context"

    // Provider is the platform-neutral contract. ALL methods are scope-limited
    // to safe, auditable primitives. Adding a method requires a code-review
    // gate per Spec 3 §11 (no raw passthrough).
    type Provider interface {
        Name() string

        // OAuth.
        OAuthAuthorizeURL(ctx context.Context, state, redirectURI string) (string, error)
        OAuthExchange(ctx context.Context, code, redirectURI string) (*Tokens, error)
        RefreshToken(ctx context.Context, refreshToken string) (*Tokens, error)

        // Metrics (fetch-only, no side effects).
        FetchMetrics(ctx context.Context, b BindingRef, dims MetricDimensions) (*MetricSnapshot, error)
        FetchLiveSession(ctx context.Context, b BindingRef, awemeID string) (*LiveSession, error)
        FetchMaterialHealth(ctx context.Context, b BindingRef, materialIDs []string) ([]MaterialHealth, error)

        // Actions — exactly five. Each maps to a single ad-platform mutation.
        AdjustBid(ctx context.Context, b BindingRef, adID string, newBid Money) error
        AdjustBudget(ctx context.Context, b BindingRef, adID string, newBudget Money) error
        PauseAd(ctx context.Context, b BindingRef, adID string) error
        ResumeAd(ctx context.Context, b BindingRef, adID string) error
        ApplyMaterial(ctx context.Context, b BindingRef, adID string, materialID string) error
    }
    ```
  - File: `src-go/internal/adsplatform/errors.go`
    ```go
    package adsplatform

    import "errors"

    // Sentinel error categories. Provider implementations wrap their native
    // errors with these so callers can branch without provider-specific knowledge.
    var (
        ErrProviderNotFound  = errors.New("adsplatform: provider not registered")
        ErrAuthExpired       = errors.New("adsplatform: auth_expired")        // 401/403 from upstream
        ErrRateLimited       = errors.New("adsplatform: rate_limited")        // 429 / known throttle codes
        ErrTransientFailure  = errors.New("adsplatform: transient_failure")   // 5xx / network
        ErrInvalidRequest    = errors.New("adsplatform: invalid_request")     // 4xx other than auth
        ErrUpstreamRejected  = errors.New("adsplatform: upstream_rejected")   // platform-level business reject
    )
    ```
  - File: `src-go/internal/adsplatform/registry.go`
    ```go
    package adsplatform

    import (
        "fmt"
        "sync"
    )

    // Constructor builds a Provider on demand. Idempotent: registry caches
    // nothing; callers may invoke Resolve repeatedly.
    type Constructor func() Provider

    // Registry maps provider name → constructor.
    type Registry struct {
        mu    sync.RWMutex
        ctors map[string]Constructor
    }

    // NewRegistry returns an empty Registry.
    func NewRegistry() *Registry { return &Registry{ctors: map[string]Constructor{}} }

    // Register installs ctor under name. Panics on duplicate registration —
    // duplicate names are a programmer error, not a runtime condition.
    func (r *Registry) Register(name string, ctor Constructor) {
        r.mu.Lock()
        defer r.mu.Unlock()
        if _, exists := r.ctors[name]; exists {
            panic(fmt.Sprintf("adsplatform: duplicate registration for %q", name))
        }
        r.ctors[name] = ctor
    }

    // Resolve constructs the provider for name, or returns ErrProviderNotFound.
    func (r *Registry) Resolve(name string) (Provider, error) {
        r.mu.RLock()
        ctor, ok := r.ctors[name]
        r.mu.RUnlock()
        if !ok {
            return nil, fmt.Errorf("%w: %q", ErrProviderNotFound, name)
        }
        return ctor(), nil
    }
    ```
  - Run `rtk go test ./internal/adsplatform/...` — three tests pass.

- [x] Step 2.3 — commit `feat(adsplatform): add provider interface + registry + neutral types`

---

## Task 3 — Mock provider for tests (`internal/adsplatform/mock`)

- [x] Step 3.1 — failing tests
  - File: `src-go/internal/adsplatform/mock/provider_test.go`
    ```go
    package mock_test

    import (
        "context"
        "testing"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
        mockprov "github.com/react-go-quick-starter/server/internal/adsplatform/mock"
    )

    func TestProvider_RecordsCalls(t *testing.T) {
        p := mockprov.New("qianchuan")
        ctx := context.Background()
        ref := adsplatform.BindingRef{AdvertiserID: "A1", AccessToken: "tok"}
        if err := p.AdjustBid(ctx, ref, "AD7", adsplatform.Money{Amount: 4500, Currency: "CNY"}); err != nil {
            t.Fatal(err)
        }
        if err := p.PauseAd(ctx, ref, "AD7"); err != nil {
            t.Fatal(err)
        }
        calls := p.Calls()
        if len(calls) != 2 {
            t.Fatalf("calls=%d", len(calls))
        }
        if calls[0].Method != "AdjustBid" || calls[0].AdID != "AD7" {
            t.Errorf("call[0]=%+v", calls[0])
        }
    }

    func TestProvider_StubMetricsResponse(t *testing.T) {
        p := mockprov.New("qianchuan")
        p.SetMetrics(&adsplatform.MetricSnapshot{Ads: []adsplatform.AdMetric{{AdID: "AD1", ROI: 1.7}}})
        snap, err := p.FetchMetrics(context.Background(), adsplatform.BindingRef{}, adsplatform.MetricDimensions{})
        if err != nil {
            t.Fatal(err)
        }
        if len(snap.Ads) != 1 || snap.Ads[0].ROI != 1.7 {
            t.Errorf("got %+v", snap)
        }
    }
    ```

- [x] Step 3.2 — implement
  - File: `src-go/internal/adsplatform/mock/provider.go`
    ```go
    // Package mock provides an in-memory adsplatform.Provider that records
    // every call. Used by handler/integration tests to avoid live HTTP.
    package mock

    import (
        "context"
        "errors"
        "sync"
        "time"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
    )

    // Call is one recorded Provider invocation.
    type Call struct {
        Method     string
        AdID       string
        AwemeID    string
        Money      adsplatform.Money
        MaterialID string
    }

    // Provider is a configurable test double.
    type Provider struct {
        name string

        mu          sync.Mutex
        calls       []Call
        metrics     *adsplatform.MetricSnapshot
        liveSession *adsplatform.LiveSession
        material    []adsplatform.MaterialHealth
        nextErr     error
        tokens      *adsplatform.Tokens
    }

    // New returns a Provider that reports the given name.
    func New(name string) *Provider { return &Provider{name: name} }

    // Name returns the configured provider name.
    func (p *Provider) Name() string { return p.name }

    // SetMetrics installs the snapshot returned by FetchMetrics.
    func (p *Provider) SetMetrics(s *adsplatform.MetricSnapshot) { p.metrics = s }

    // SetLiveSession installs the response returned by FetchLiveSession.
    func (p *Provider) SetLiveSession(s *adsplatform.LiveSession) { p.liveSession = s }

    // SetMaterialHealth installs the response returned by FetchMaterialHealth.
    func (p *Provider) SetMaterialHealth(m []adsplatform.MaterialHealth) { p.material = m }

    // SetTokens installs the response returned by OAuthExchange / RefreshToken.
    func (p *Provider) SetTokens(t *adsplatform.Tokens) { p.tokens = t }

    // FailNext makes the next mutating call return err (one-shot).
    func (p *Provider) FailNext(err error) { p.nextErr = err }

    // Calls returns a copy of the recorded call log.
    func (p *Provider) Calls() []Call {
        p.mu.Lock()
        defer p.mu.Unlock()
        out := make([]Call, len(p.calls))
        copy(out, p.calls)
        return out
    }

    func (p *Provider) record(c Call) error {
        p.mu.Lock()
        defer p.mu.Unlock()
        p.calls = append(p.calls, c)
        if p.nextErr != nil {
            err := p.nextErr
            p.nextErr = nil
            return err
        }
        return nil
    }

    // OAuthAuthorizeURL returns a fixed URL for the configured name.
    func (p *Provider) OAuthAuthorizeURL(_ context.Context, state, redirectURI string) (string, error) {
        return "https://mock/" + p.name + "/oauth?state=" + state + "&redirect_uri=" + redirectURI, nil
    }

    // OAuthExchange returns the configured tokens or a sentinel default.
    func (p *Provider) OAuthExchange(_ context.Context, _ string, _ string) (*adsplatform.Tokens, error) {
        if p.tokens != nil {
            return p.tokens, nil
        }
        return &adsplatform.Tokens{AccessToken: "mock-access", RefreshToken: "mock-refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
    }

    // RefreshToken returns the configured tokens or a default fresh pair.
    func (p *Provider) RefreshToken(_ context.Context, _ string) (*adsplatform.Tokens, error) {
        if p.tokens != nil {
            return p.tokens, nil
        }
        return &adsplatform.Tokens{AccessToken: "mock-access", RefreshToken: "mock-refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
    }

    // FetchMetrics returns the configured snapshot.
    func (p *Provider) FetchMetrics(_ context.Context, _ adsplatform.BindingRef, _ adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
        if p.metrics == nil {
            return &adsplatform.MetricSnapshot{}, nil
        }
        return p.metrics, nil
    }

    // FetchLiveSession returns the configured live-session response.
    func (p *Provider) FetchLiveSession(_ context.Context, _ adsplatform.BindingRef, awemeID string) (*adsplatform.LiveSession, error) {
        if p.liveSession == nil {
            return &adsplatform.LiveSession{AwemeID: awemeID}, nil
        }
        return p.liveSession, nil
    }

    // FetchMaterialHealth returns the configured material health.
    func (p *Provider) FetchMaterialHealth(_ context.Context, _ adsplatform.BindingRef, _ []string) ([]adsplatform.MaterialHealth, error) {
        return p.material, nil
    }

    // AdjustBid records the call.
    func (p *Provider) AdjustBid(_ context.Context, _ adsplatform.BindingRef, adID string, money adsplatform.Money) error {
        return p.record(Call{Method: "AdjustBid", AdID: adID, Money: money})
    }

    // AdjustBudget records the call.
    func (p *Provider) AdjustBudget(_ context.Context, _ adsplatform.BindingRef, adID string, money adsplatform.Money) error {
        return p.record(Call{Method: "AdjustBudget", AdID: adID, Money: money})
    }

    // PauseAd records the call.
    func (p *Provider) PauseAd(_ context.Context, _ adsplatform.BindingRef, adID string) error {
        return p.record(Call{Method: "PauseAd", AdID: adID})
    }

    // ResumeAd records the call.
    func (p *Provider) ResumeAd(_ context.Context, _ adsplatform.BindingRef, adID string) error {
        return p.record(Call{Method: "ResumeAd", AdID: adID})
    }

    // ApplyMaterial records the call.
    func (p *Provider) ApplyMaterial(_ context.Context, _ adsplatform.BindingRef, adID, materialID string) error {
        return p.record(Call{Method: "ApplyMaterial", AdID: adID, MaterialID: materialID})
    }

    // compile-time check: Provider satisfies adsplatform.Provider.
    var _ adsplatform.Provider = (*Provider)(nil)

    // ErrSimulatedAuth is the sentinel mock callers use when injecting
    // an auth_expired-shaped failure.
    var ErrSimulatedAuth = errors.New("mock: simulated auth_expired")
    ```
  - Run `rtk go test ./internal/adsplatform/mock/...` — green.

- [x] Step 3.3 — commit `feat(adsplatform): add mock provider for tests`

---

## Task 4 — Qianchuan client core (HTTP + retry + error mapping)

- [x] Step 4.1 — failing tests using `httptest.Server`
  - File: `src-go/internal/adsplatform/qianchuan/client_test.go`
    ```go
    package qianchuan_test

    import (
        "context"
        "encoding/json"
        "errors"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
        "github.com/react-go-quick-starter/server/internal/adsplatform/qianchuan"
    )

    func TestClient_AccessTokenHeaderInjected(t *testing.T) {
        var gotToken string
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            gotToken = r.Header.Get("Access-Token")
            _ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
        }))
        defer srv.Close()
        c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "x", AppSecret: "y"})
        _, err := c.GetJSON(context.Background(), "tok123", "/qianchuan/ad/get/", nil)
        if err != nil {
            t.Fatal(err)
        }
        if gotToken != "tok123" {
            t.Errorf("Access-Token=%q", gotToken)
        }
    }

    func TestClient_MapsAuthExpired(t *testing.T) {
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusUnauthorized)
            _, _ = w.Write([]byte(`{"code":40104,"message":"access_token expired"}`))
        }))
        defer srv.Close()
        c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "x", AppSecret: "y"})
        _, err := c.GetJSON(context.Background(), "t", "/x", nil)
        if !errors.Is(err, adsplatform.ErrAuthExpired) {
            t.Fatalf("want ErrAuthExpired, got %v", err)
        }
    }

    func TestClient_MapsRateLimited(t *testing.T) {
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusTooManyRequests)
            _, _ = w.Write([]byte(`{"code":40100,"message":"rate limit exceeded"}`))
        }))
        defer srv.Close()
        c := qianchuan.NewClient(qianchuan.Options{
            Host: srv.URL, AppID: "x", AppSecret: "y",
            MaxRetries: 0, // do not retry; we want the surface error
        })
        _, err := c.GetJSON(context.Background(), "t", "/x", nil)
        if !errors.Is(err, adsplatform.ErrRateLimited) {
            t.Fatalf("want ErrRateLimited, got %v", err)
        }
    }

    func TestClient_RetriesTransientThenSucceeds(t *testing.T) {
        var hits int
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            hits++
            if hits < 3 {
                w.WriteHeader(http.StatusBadGateway)
                _, _ = w.Write([]byte(`{"code":51010,"message":"系统开小差"}`))
                return
            }
            _ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"ok": true}})
        }))
        defer srv.Close()
        c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "x", AppSecret: "y", MaxRetries: 3})
        body, err := c.GetJSON(context.Background(), "t", "/x", nil)
        if err != nil {
            t.Fatal(err)
        }
        if hits != 3 {
            t.Errorf("hits=%d, want 3", hits)
        }
        if body["data"] == nil {
            t.Error("data missing")
        }
    }
    ```

- [x] Step 4.2 — implement client
  - File: `src-go/internal/adsplatform/qianchuan/client.go`
    ```go
    // Package qianchuan implements adsplatform.Provider against the Qianchuan
    // (巨量千川) OpenAPI hosted at api.oceanengine.com / ad.oceanengine.com.
    //
    // Authentication: every request carries `Access-Token: <accessToken>`.
    // The reference TS project uses the same scheme (no body HMAC). The
    // App ID / App Secret pair is required only by the OAuth code-exchange
    // endpoint, not by data-plane requests.
    //
    // Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §8
    package qianchuan

    import (
        "bytes"
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "io"
        "math/rand"
        "net/http"
        "strings"
        "time"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
    )

    const (
        DefaultHost   = "https://ad.oceanengine.com"
        APIV1Prefix   = "/open_api/v1.0"
        OAuth2Prefix  = "/open_api/oauth2"
        defaultRetries = 3
        defaultTimeout = 15 * time.Second
    )

    // Options configures Client.
    type Options struct {
        Host       string        // e.g. https://ad.oceanengine.com (no trailing slash)
        AppID      string        // QIANCHUAN_APP_ID
        AppSecret  string        // QIANCHUAN_APP_SECRET
        HTTPClient *http.Client  // optional; defaults to a 15s-timeout client
        MaxRetries int           // 0 disables retry; default 3
    }

    // Client is the lower-level HTTP client. Higher-level Provider methods
    // live in provider.go and call into Client.
    type Client struct {
        host    string
        appID   string
        secret  string
        httpc   *http.Client
        retries int
    }

    // NewClient returns a configured Client. Missing host falls back to DefaultHost.
    func NewClient(opts Options) *Client {
        host := strings.TrimRight(opts.Host, "/")
        if host == "" {
            host = DefaultHost
        }
        httpc := opts.HTTPClient
        if httpc == nil {
            httpc = &http.Client{Timeout: defaultTimeout}
        }
        retries := opts.MaxRetries
        if retries == 0 && opts.HTTPClient == nil {
            retries = defaultRetries
        }
        return &Client{host: host, appID: opts.AppID, secret: opts.AppSecret, httpc: httpc, retries: retries}
    }

    // GetJSON issues a signed GET against path with query params. Returns the
    // decoded top-level object (the wrapper {code, message, request_id, data}).
    func (c *Client) GetJSON(ctx context.Context, accessToken, path string, query map[string]string) (map[string]any, error) {
        u := c.host + APIV1Prefix + path
        if len(query) > 0 {
            sep := "?"
            for k, v := range query {
                u += sep + k + "=" + v
                sep = "&"
            }
        }
        return c.do(ctx, http.MethodGet, u, accessToken, nil)
    }

    // PostJSON issues a signed POST with body marshalled as JSON.
    func (c *Client) PostJSON(ctx context.Context, accessToken, path string, body any) (map[string]any, error) {
        u := c.host + APIV1Prefix + path
        return c.do(ctx, http.MethodPost, u, accessToken, body)
    }

    // OAuthExchange swaps an authorization code for a token pair.
    // Endpoint: <host>/open_api/oauth2/access_token/
    func (c *Client) OAuthExchange(ctx context.Context, code, redirectURI string) (map[string]any, error) {
        u := c.host + OAuth2Prefix + "/access_token/"
        body := map[string]any{
            "app_id":       c.appID,
            "secret":       c.secret,
            "grant_type":   "auth_code",
            "auth_code":    code,
            "redirect_uri": redirectURI,
        }
        return c.do(ctx, http.MethodPost, u, "", body)
    }

    // OAuthRefresh refreshes an expired access token.
    func (c *Client) OAuthRefresh(ctx context.Context, refreshToken string) (map[string]any, error) {
        u := c.host + OAuth2Prefix + "/refresh_token/"
        body := map[string]any{
            "app_id":        c.appID,
            "secret":        c.secret,
            "grant_type":    "refresh_token",
            "refresh_token": refreshToken,
        }
        return c.do(ctx, http.MethodPost, u, "", body)
    }

    func (c *Client) do(ctx context.Context, method, url, accessToken string, body any) (map[string]any, error) {
        var lastErr error
        for attempt := 0; attempt <= c.retries; attempt++ {
            obj, err := c.attempt(ctx, method, url, accessToken, body)
            if err == nil {
                return obj, nil
            }
            lastErr = err
            if !isRetryable(err) {
                return nil, err
            }
            // exponential backoff with jitter: 1s, 2s, 4s (+50% jitter)
            base := time.Duration(1<<attempt) * time.Second
            jitter := time.Duration(rand.Int63n(int64(base / 2)))
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(base + jitter):
            }
        }
        return nil, lastErr
    }

    func (c *Client) attempt(ctx context.Context, method, url, accessToken string, body any) (map[string]any, error) {
        var reader io.Reader
        if body != nil {
            buf, err := json.Marshal(body)
            if err != nil {
                return nil, fmt.Errorf("qianchuan: marshal: %w", err)
            }
            reader = bytes.NewReader(buf)
        }
        req, err := http.NewRequestWithContext(ctx, method, url, reader)
        if err != nil {
            return nil, fmt.Errorf("qianchuan: build req: %w", err)
        }
        if accessToken != "" {
            req.Header.Set("Access-Token", accessToken)
        }
        if body != nil {
            req.Header.Set("Content-Type", "application/json")
        }
        resp, err := c.httpc.Do(req)
        if err != nil {
            return nil, fmt.Errorf("%w: %v", adsplatform.ErrTransientFailure, err)
        }
        defer resp.Body.Close()
        raw, _ := io.ReadAll(resp.Body)
        // Map HTTP status first.
        if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
            return nil, fmt.Errorf("%w: http %d", adsplatform.ErrAuthExpired, resp.StatusCode)
        }
        if resp.StatusCode == http.StatusTooManyRequests {
            return nil, fmt.Errorf("%w: http 429", adsplatform.ErrRateLimited)
        }
        if resp.StatusCode >= 500 {
            return nil, fmt.Errorf("%w: http %d", adsplatform.ErrTransientFailure, resp.StatusCode)
        }
        if resp.StatusCode >= 400 {
            return nil, fmt.Errorf("%w: http %d body=%s", adsplatform.ErrInvalidRequest, resp.StatusCode, truncate(raw))
        }
        // Use json.Number to avoid float64 precision loss on big-int IDs.
        dec := json.NewDecoder(bytes.NewReader(raw))
        dec.UseNumber()
        var obj map[string]any
        if err := dec.Decode(&obj); err != nil {
            return nil, fmt.Errorf("qianchuan: decode: %w body=%s", err, truncate(raw))
        }
        // Map Qianchuan-level error codes.
        if codeJSON, ok := obj["code"].(json.Number); ok {
            code, _ := codeJSON.Int64()
            switch {
            case code == 0:
                return obj, nil
            case code == 40100:
                return nil, fmt.Errorf("%w: code=%d message=%v", adsplatform.ErrRateLimited, code, obj["message"])
            case code == 40104 || code == 40105:
                return nil, fmt.Errorf("%w: code=%d", adsplatform.ErrAuthExpired, code)
            case code == 51010 || code == 51011:
                return nil, fmt.Errorf("%w: code=%d", adsplatform.ErrTransientFailure, code)
            default:
                return nil, fmt.Errorf("%w: code=%d message=%v", adsplatform.ErrUpstreamRejected, code, obj["message"])
            }
        }
        return obj, nil
    }

    func isRetryable(err error) bool {
        return errors.Is(err, adsplatform.ErrTransientFailure) || errors.Is(err, adsplatform.ErrRateLimited)
    }

    func truncate(b []byte) string {
        if len(b) <= 200 {
            return string(b)
        }
        return string(b[:200]) + "...(truncated)"
    }
    ```
  - Run `rtk go test ./internal/adsplatform/qianchuan/...` — four tests pass.

- [x] Step 4.3 — commit `feat(qianchuan): add HTTP client with retry + error mapping`

---

## Task 5 — Qianchuan provider implementation (Provider methods + mapping)

- [ ] Step 5.1 — failing tests for each Provider method
  - File: `src-go/internal/adsplatform/qianchuan/provider_test.go`
    ```go
    package qianchuan_test

    import (
        "context"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
        "github.com/react-go-quick-starter/server/internal/adsplatform/qianchuan"
    )

    func newJSONStub(t *testing.T, route string, payload map[string]any) *httptest.Server {
        t.Helper()
        return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !strings.HasSuffix(r.URL.Path, route) {
                t.Errorf("unexpected path %s", r.URL.Path)
            }
            _ = json.NewEncoder(w).Encode(payload)
        }))
    }

    func TestProvider_FetchMetrics_MapsBigIntRoomID(t *testing.T) {
        srv := newJSONStub(t, "/qianchuan/report/live/get/", map[string]any{
            "code": 0,
            "data": map[string]any{
                "list": []map[string]any{
                    {"ad_id": "AD7", "stat_cost": 10.0, "roi": 1.7, "show_cnt": 100, "click_cnt": 5, "bid": 5.0, "budget": 100.0, "status": "STATUS_DELIVERY_OK"},
                },
            },
        })
        defer srv.Close()
        c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "a", AppSecret: "s"})
        p := qianchuan.NewProvider(c)
        snap, err := p.FetchMetrics(context.Background(),
            adsplatform.BindingRef{AdvertiserID: "1234567890", AccessToken: "t"},
            adsplatform.MetricDimensions{Range: "today"})
        if err != nil {
            t.Fatal(err)
        }
        if len(snap.Ads) != 1 || snap.Ads[0].AdID != "AD7" || snap.Ads[0].ROI != 1.7 {
            t.Fatalf("ads=%+v", snap.Ads)
        }
    }

    func TestProvider_AdjustBid_PostsBidUpdate(t *testing.T) {
        var seen map[string]any
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            _ = json.NewDecoder(r.Body).Decode(&seen)
            _ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
        }))
        defer srv.Close()
        c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "a", AppSecret: "s"})
        p := qianchuan.NewProvider(c)
        err := p.AdjustBid(context.Background(),
            adsplatform.BindingRef{AdvertiserID: "100", AccessToken: "t"},
            "AD7", adsplatform.Money{Amount: 4500, Currency: "CNY"})
        if err != nil {
            t.Fatal(err)
        }
        if seen["ad_id"] != "AD7" {
            t.Errorf("ad_id=%v", seen["ad_id"])
        }
    }

    func TestProvider_PauseAd_SetsStatus(t *testing.T) {
        var seen map[string]any
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            _ = json.NewDecoder(r.Body).Decode(&seen)
            _ = json.NewEncoder(w).Encode(map[string]any{"code": 0})
        }))
        defer srv.Close()
        c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "a", AppSecret: "s"})
        p := qianchuan.NewProvider(c)
        if err := p.PauseAd(context.Background(),
            adsplatform.BindingRef{AdvertiserID: "100", AccessToken: "t"}, "AD7"); err != nil {
            t.Fatal(err)
        }
        if seen["opt_status"] != "disable" {
            t.Errorf("opt_status=%v", seen["opt_status"])
        }
    }
    ```

- [ ] Step 5.2 — implement provider + mapping
  - File: `src-go/internal/adsplatform/qianchuan/provider.go`
    ```go
    package qianchuan

    import (
        "context"
        "encoding/json"
        "fmt"
        "time"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
    )

    // Provider is the Qianchuan implementation of adsplatform.Provider.
    // All methods funnel through Client and use mapping.go to neutralize the
    // Qianchuan response shape.
    type Provider struct {
        client *Client
    }

    // NewProvider wraps c.
    func NewProvider(c *Client) *Provider { return &Provider{client: c} }

    // Name returns the provider id used in registry / DB rows.
    func (*Provider) Name() string { return "qianchuan" }

    // OAuthAuthorizeURL builds the user-facing authorize URL.
    // Reference: https://open.oceanengine.com/labels/7/docs/1696710606895111
    func (p *Provider) OAuthAuthorizeURL(_ context.Context, state, redirectURI string) (string, error) {
        return fmt.Sprintf(
            "%s/openauth/index?app_id=%s&state=%s&material_auth=1&redirect_uri=%s",
            p.client.host, p.client.appID, state, redirectURI,
        ), nil
    }

    // OAuthExchange swaps an authorization code for tokens.
    func (p *Provider) OAuthExchange(ctx context.Context, code, redirectURI string) (*adsplatform.Tokens, error) {
        obj, err := p.client.OAuthExchange(ctx, code, redirectURI)
        if err != nil {
            return nil, err
        }
        return mapTokens(obj)
    }

    // RefreshToken exchanges a refresh token for a fresh pair.
    func (p *Provider) RefreshToken(ctx context.Context, refreshToken string) (*adsplatform.Tokens, error) {
        obj, err := p.client.OAuthRefresh(ctx, refreshToken)
        if err != nil {
            return nil, err
        }
        return mapTokens(obj)
    }

    // FetchMetrics fetches the live-room metrics report and maps to the
    // neutral shape. Spec calls out `today_live` series; we use
    // /qianchuan/report/live/get/ as the canonical primary report.
    func (p *Provider) FetchMetrics(ctx context.Context, b adsplatform.BindingRef, dims adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
        q := map[string]string{
            "advertiser_id": b.AdvertiserID,
            "time_granularity": defaultStr(dims.Granular, "STAT_TIME_GRANULARITY_HOURLY"),
        }
        obj, err := p.client.GetJSON(ctx, b.AccessToken, "/qianchuan/report/live/get/", q)
        if err != nil {
            return nil, err
        }
        return mapMetrics(obj)
    }

    // FetchLiveSession returns the current state of a Douyin live room.
    func (p *Provider) FetchLiveSession(ctx context.Context, b adsplatform.BindingRef, awemeID string) (*adsplatform.LiveSession, error) {
        q := map[string]string{
            "advertiser_id": b.AdvertiserID,
            "aweme_id":      awemeID,
        }
        obj, err := p.client.GetJSON(ctx, b.AccessToken, "/qianchuan/today_live/room/get/", q)
        if err != nil {
            return nil, err
        }
        return mapLiveSession(obj, awemeID)
    }

    // FetchMaterialHealth returns per-material health.
    func (p *Provider) FetchMaterialHealth(ctx context.Context, b adsplatform.BindingRef, materialIDs []string) ([]adsplatform.MaterialHealth, error) {
        body := map[string]any{
            "advertiser_id": b.AdvertiserID,
            "material_ids":  materialIDs,
        }
        obj, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/material/health/get/", body)
        if err != nil {
            return nil, err
        }
        return mapMaterialHealth(obj)
    }

    // AdjustBid maps to /qianchuan/ad/bid/update/ on ad.oceanengine.com.
    // newBid is in fen (CNY minor units); the upstream API expects yuan-as-decimal.
    func (p *Provider) AdjustBid(ctx context.Context, b adsplatform.BindingRef, adID string, newBid adsplatform.Money) error {
        body := map[string]any{
            "advertiser_id": b.AdvertiserID,
            "ad_id":         adID,
            "bid":           float64(newBid.Amount) / 100.0,
        }
        _, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/bid/update/", body)
        return err
    }

    // AdjustBudget maps to /qianchuan/ad/budget/update/.
    func (p *Provider) AdjustBudget(ctx context.Context, b adsplatform.BindingRef, adID string, newBudget adsplatform.Money) error {
        body := map[string]any{
            "advertiser_id": b.AdvertiserID,
            "ad_id":         adID,
            "budget":        float64(newBudget.Amount) / 100.0,
        }
        _, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/budget/update/", body)
        return err
    }

    // PauseAd maps to /qianchuan/ad/status/update/ with opt_status="disable".
    func (p *Provider) PauseAd(ctx context.Context, b adsplatform.BindingRef, adID string) error {
        return p.setAdStatus(ctx, b, adID, "disable")
    }

    // ResumeAd maps to the same endpoint with opt_status="enable".
    func (p *Provider) ResumeAd(ctx context.Context, b adsplatform.BindingRef, adID string) error {
        return p.setAdStatus(ctx, b, adID, "enable")
    }

    func (p *Provider) setAdStatus(ctx context.Context, b adsplatform.BindingRef, adID, status string) error {
        body := map[string]any{
            "advertiser_id": b.AdvertiserID,
            "ad_ids":        []string{adID},
            "opt_status":    status,
        }
        _, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/status/update/", body)
        return err
    }

    // ApplyMaterial swaps a creative on a specific ad.
    func (p *Provider) ApplyMaterial(ctx context.Context, b adsplatform.BindingRef, adID, materialID string) error {
        body := map[string]any{
            "advertiser_id": b.AdvertiserID,
            "ad_id":         adID,
            "material_id":   materialID,
        }
        _, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/creative/update/", body)
        return err
    }

    func defaultStr(v, fallback string) string {
        if v == "" {
            return fallback
        }
        return v
    }

    // compile-time check.
    var _ adsplatform.Provider = (*Provider)(nil)

    // Ensure json package is referenced for mapping.go (avoid unused import in
    // case mapping.go is split late).
    var _ = json.Unmarshal
    var _ = time.Now
    ```
  - File: `src-go/internal/adsplatform/qianchuan/mapping.go`
    ```go
    package qianchuan

    import (
        "encoding/json"
        "fmt"
        "time"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
    )

    // mapTokens reads {data: {access_token, refresh_token, expires_in, refresh_token_expires_in}}.
    func mapTokens(obj map[string]any) (*adsplatform.Tokens, error) {
        data, _ := obj["data"].(map[string]any)
        if data == nil {
            return nil, fmt.Errorf("qianchuan: missing data in token response")
        }
        access, _ := data["access_token"].(string)
        refresh, _ := data["refresh_token"].(string)
        expires := time.Now()
        if e, ok := data["expires_in"].(json.Number); ok {
            secs, _ := e.Int64()
            expires = time.Now().Add(time.Duration(secs) * time.Second)
        }
        return &adsplatform.Tokens{AccessToken: access, RefreshToken: refresh, ExpiresAt: expires}, nil
    }

    // mapMetrics reads /qianchuan/report/live/get/ data.list rows.
    func mapMetrics(obj map[string]any) (*adsplatform.MetricSnapshot, error) {
        data, _ := obj["data"].(map[string]any)
        if data == nil {
            return &adsplatform.MetricSnapshot{BucketAt: time.Now().UTC()}, nil
        }
        rows, _ := data["list"].([]any)
        ads := make([]adsplatform.AdMetric, 0, len(rows))
        for _, r := range rows {
            row, ok := r.(map[string]any)
            if !ok {
                continue
            }
            ads = append(ads, adsplatform.AdMetric{
                AdID:   asString(row["ad_id"]),
                Status: asString(row["status"]),
                Spend:  asFloat(row["stat_cost"]),
                ROI:    asFloat(row["roi"]),
                CTR:    asFloat(row["ctr"]),
                CPM:    asFloat(row["cpm"]),
                Bid:    asFloat(row["bid"]),
                Budget: asFloat(row["budget"]),
            })
        }
        return &adsplatform.MetricSnapshot{
            BucketAt: time.Now().UTC().Truncate(time.Minute),
            Ads:      ads,
            Raw:      data,
        }, nil
    }

    // mapLiveSession reads /qianchuan/today_live/room/get/ shape.
    func mapLiveSession(obj map[string]any, awemeID string) (*adsplatform.LiveSession, error) {
        data, _ := obj["data"].(map[string]any)
        if data == nil {
            return &adsplatform.LiveSession{AwemeID: awemeID, Status: "unknown"}, nil
        }
        return &adsplatform.LiveSession{
            AwemeID:   awemeID,
            RoomID:    asString(data["room_id"]),
            Status:    asString(data["live_status"]),
            Viewers:   asInt(data["audience_count"]),
            GMV:       asFloat(data["gmv"]),
            Raw:       data,
        }, nil
    }

    // mapMaterialHealth reads /qianchuan/material/health/get/ shape.
    func mapMaterialHealth(obj map[string]any) ([]adsplatform.MaterialHealth, error) {
        data, _ := obj["data"].(map[string]any)
        if data == nil {
            return nil, nil
        }
        rows, _ := data["list"].([]any)
        out := make([]adsplatform.MaterialHealth, 0, len(rows))
        for _, r := range rows {
            row, ok := r.(map[string]any)
            if !ok {
                continue
            }
            out = append(out, adsplatform.MaterialHealth{
                MaterialID: asString(row["material_id"]),
                Status:     asString(row["status"]),
                Health:     asFloat(row["health_score"]),
                Reason:     asString(row["reason"]),
            })
        }
        return out, nil
    }

    // asString preserves string-valued numbers (big-int IDs) without precision loss.
    func asString(v any) string {
        switch x := v.(type) {
        case string:
            return x
        case json.Number:
            return x.String()
        case float64:
            return fmt.Sprintf("%.0f", x)
        case nil:
            return ""
        default:
            return fmt.Sprintf("%v", x)
        }
    }

    func asFloat(v any) float64 {
        switch x := v.(type) {
        case float64:
            return x
        case json.Number:
            f, _ := x.Float64()
            return f
        }
        return 0
    }

    func asInt(v any) int64 {
        switch x := v.(type) {
        case json.Number:
            i, _ := x.Int64()
            return i
        case float64:
            return int64(x)
        }
        return 0
    }
    ```
  - Run `rtk go test ./internal/adsplatform/qianchuan/...` — green.

- [ ] Step 5.3 — register Qianchuan in a default registry constructor
  - File: `src-go/internal/adsplatform/qianchuan/register.go`
    ```go
    package qianchuan

    import (
        "os"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
    )

    // Register installs Qianchuan into reg using env-driven config:
    //   QIANCHUAN_HOST       (default: ad.oceanengine.com)
    //   QIANCHUAN_APP_ID     (required for OAuth)
    //   QIANCHUAN_APP_SECRET (required for OAuth)
    func Register(reg *adsplatform.Registry) {
        reg.Register("qianchuan", func() adsplatform.Provider {
            host := os.Getenv("QIANCHUAN_HOST")
            return NewProvider(NewClient(Options{
                Host:      host,
                AppID:     os.Getenv("QIANCHUAN_APP_ID"),
                AppSecret: os.Getenv("QIANCHUAN_APP_SECRET"),
            }))
        })
    }
    ```

- [ ] Step 5.4 — commit `feat(qianchuan): provider impl + neutral mapping + registry hook`

---

## Task 6 — Repository: `internal/qianchuanbinding/repo.go`

- [ ] Step 6.1 — failing repo tests (in-memory contract test)
  - File: `src-go/internal/qianchuanbinding/repo_test.go`
    ```go
    package qianchuanbinding_test

    import (
        "context"
        "testing"
        "time"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/qianchuanbinding"
    )

    type memRepo struct{ rows map[uuid.UUID]*qianchuanbinding.Record }

    func newMem() *memRepo { return &memRepo{rows: map[uuid.UUID]*qianchuanbinding.Record{}} }

    func (m *memRepo) Create(_ context.Context, r *qianchuanbinding.Record) error {
        for _, ex := range m.rows {
            if ex.ProjectID == r.ProjectID && ex.AdvertiserID == r.AdvertiserID && ex.AwemeID == r.AwemeID {
                return qianchuanbinding.ErrAdvertiserAlreadyBound
            }
        }
        if r.ID == uuid.Nil {
            r.ID = uuid.New()
        }
        cp := *r
        m.rows[r.ID] = &cp
        return nil
    }
    func (m *memRepo) Get(_ context.Context, id uuid.UUID) (*qianchuanbinding.Record, error) {
        r, ok := m.rows[id]
        if !ok {
            return nil, qianchuanbinding.ErrNotFound
        }
        cp := *r
        return &cp, nil
    }
    func (m *memRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*qianchuanbinding.Record, error) {
        out := []*qianchuanbinding.Record{}
        for _, r := range m.rows {
            if r.ProjectID == projectID {
                cp := *r
                out = append(out, &cp)
            }
        }
        return out, nil
    }
    func (m *memRepo) Update(_ context.Context, r *qianchuanbinding.Record) error {
        if _, ok := m.rows[r.ID]; !ok {
            return qianchuanbinding.ErrNotFound
        }
        cp := *r
        m.rows[r.ID] = &cp
        return nil
    }
    func (m *memRepo) Delete(_ context.Context, id uuid.UUID) error {
        if _, ok := m.rows[id]; !ok {
            return qianchuanbinding.ErrNotFound
        }
        delete(m.rows, id)
        return nil
    }
    func (m *memRepo) TouchSync(_ context.Context, id uuid.UUID, when time.Time) error {
        r, ok := m.rows[id]
        if !ok {
            return qianchuanbinding.ErrNotFound
        }
        r.LastSyncedAt = &when
        return nil
    }

    func TestRepo_Contract(t *testing.T) {
        var _ qianchuanbinding.Repository = newMem()
    }

    func TestRepo_DuplicateRejected(t *testing.T) {
        m := newMem()
        ctx := context.Background()
        proj := uuid.New()
        r := &qianchuanbinding.Record{ProjectID: proj, AdvertiserID: "A1", AwemeID: "W1", AccessTokenSecretRef: "s.access", RefreshTokenSecretRef: "s.refresh", CreatedBy: uuid.New(), Status: "active"}
        if err := m.Create(ctx, r); err != nil {
            t.Fatal(err)
        }
        if err := m.Create(ctx, &qianchuanbinding.Record{ProjectID: proj, AdvertiserID: "A1", AwemeID: "W1", AccessTokenSecretRef: "s.access", RefreshTokenSecretRef: "s.refresh", CreatedBy: uuid.New(), Status: "active"}); err != qianchuanbinding.ErrAdvertiserAlreadyBound {
            t.Fatalf("want ErrAdvertiserAlreadyBound, got %v", err)
        }
    }
    ```

- [ ] Step 6.2 — implement record + Repository interface + GORM impl
  - File: `src-go/internal/qianchuanbinding/repo.go`
    ```go
    // Package qianchuanbinding owns persistence + business operations for
    // qianchuan_bindings rows. Token plaintext NEVER lives here; the
    // *_secret_ref columns hold secrets.name strings owned by Plan 1B.
    //
    // Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §6.1
    package qianchuanbinding

    import (
        "context"
        "errors"
        "time"

        "github.com/google/uuid"
        "github.com/jackc/pgx/v5/pgconn"
        "gorm.io/gorm"
    )

    // ErrNotFound is returned when no row matches the lookup.
    var ErrNotFound = errors.New("qianchuanbinding: not found")

    // ErrAdvertiserAlreadyBound is returned on (project, advertiser, aweme) UNIQUE conflict.
    var ErrAdvertiserAlreadyBound = errors.New("qianchuanbinding: advertiser_already_bound")

    // Status enum.
    const (
        StatusActive       = "active"
        StatusAuthExpired  = "auth_expired"
        StatusPaused       = "paused"
    )

    // Record is the in-memory representation of one qianchuan_bindings row.
    type Record struct {
        ID                    uuid.UUID
        ProjectID             uuid.UUID
        AdvertiserID          string
        AwemeID               string
        DisplayName           string
        Status                string
        ActingEmployeeID      *uuid.UUID
        AccessTokenSecretRef  string
        RefreshTokenSecretRef string
        TokenExpiresAt        *time.Time
        LastSyncedAt          *time.Time
        CreatedBy             uuid.UUID
        CreatedAt             time.Time
        UpdatedAt             time.Time
    }

    // Repository is the persistence contract used by the service layer.
    type Repository interface {
        Create(ctx context.Context, r *Record) error
        Get(ctx context.Context, id uuid.UUID) (*Record, error)
        ListByProject(ctx context.Context, projectID uuid.UUID) ([]*Record, error)
        Update(ctx context.Context, r *Record) error
        Delete(ctx context.Context, id uuid.UUID) error
        TouchSync(ctx context.Context, id uuid.UUID, when time.Time) error
    }

    // ---------------- GORM impl ----------------

    type bindingRow struct {
        ID                    uuid.UUID  `gorm:"column:id;primaryKey"`
        ProjectID             uuid.UUID  `gorm:"column:project_id"`
        AdvertiserID          string     `gorm:"column:advertiser_id"`
        AwemeID               string     `gorm:"column:aweme_id"`
        DisplayName           string     `gorm:"column:display_name"`
        Status                string     `gorm:"column:status"`
        ActingEmployeeID      *uuid.UUID `gorm:"column:acting_employee_id"`
        AccessTokenSecretRef  string     `gorm:"column:access_token_secret_ref"`
        RefreshTokenSecretRef string     `gorm:"column:refresh_token_secret_ref"`
        TokenExpiresAt        *time.Time `gorm:"column:token_expires_at"`
        LastSyncedAt          *time.Time `gorm:"column:last_synced_at"`
        CreatedBy             uuid.UUID  `gorm:"column:created_by"`
        CreatedAt             time.Time  `gorm:"column:created_at"`
        UpdatedAt             time.Time  `gorm:"column:updated_at"`
    }

    func (bindingRow) TableName() string { return "qianchuan_bindings" }

    func toRecord(r *bindingRow) *Record {
        return &Record{
            ID: r.ID, ProjectID: r.ProjectID, AdvertiserID: r.AdvertiserID, AwemeID: r.AwemeID,
            DisplayName: r.DisplayName, Status: r.Status, ActingEmployeeID: r.ActingEmployeeID,
            AccessTokenSecretRef: r.AccessTokenSecretRef, RefreshTokenSecretRef: r.RefreshTokenSecretRef,
            TokenExpiresAt: r.TokenExpiresAt, LastSyncedAt: r.LastSyncedAt,
            CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
        }
    }

    func fromRecord(r *Record) *bindingRow {
        return &bindingRow{
            ID: r.ID, ProjectID: r.ProjectID, AdvertiserID: r.AdvertiserID, AwemeID: r.AwemeID,
            DisplayName: r.DisplayName, Status: r.Status, ActingEmployeeID: r.ActingEmployeeID,
            AccessTokenSecretRef: r.AccessTokenSecretRef, RefreshTokenSecretRef: r.RefreshTokenSecretRef,
            TokenExpiresAt: r.TokenExpiresAt, LastSyncedAt: r.LastSyncedAt,
            CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
        }
    }

    // GormRepo is the production Repository implementation.
    type GormRepo struct{ db *gorm.DB }

    // NewGormRepo wires a Repository on top of the shared GORM DB.
    func NewGormRepo(db *gorm.DB) *GormRepo { return &GormRepo{db: db} }

    func (r *GormRepo) Create(ctx context.Context, rec *Record) error {
        if rec.ID == uuid.Nil {
            rec.ID = uuid.New()
        }
        now := time.Now().UTC()
        if rec.CreatedAt.IsZero() {
            rec.CreatedAt = now
        }
        rec.UpdatedAt = now
        if rec.Status == "" {
            rec.Status = StatusActive
        }
        if err := r.db.WithContext(ctx).Create(fromRecord(rec)).Error; err != nil {
            var pgErr *pgconn.PgError
            if errors.As(err, &pgErr) && pgErr.Code == "23505" {
                return ErrAdvertiserAlreadyBound
            }
            return err
        }
        return nil
    }

    func (r *GormRepo) Get(ctx context.Context, id uuid.UUID) (*Record, error) {
        var row bindingRow
        if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return nil, ErrNotFound
            }
            return nil, err
        }
        return toRecord(&row), nil
    }

    func (r *GormRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*Record, error) {
        var rows []bindingRow
        if err := r.db.WithContext(ctx).
            Where("project_id = ?", projectID).
            Order("created_at DESC").
            Find(&rows).Error; err != nil {
            return nil, err
        }
        out := make([]*Record, 0, len(rows))
        for i := range rows {
            out = append(out, toRecord(&rows[i]))
        }
        return out, nil
    }

    func (r *GormRepo) Update(ctx context.Context, rec *Record) error {
        rec.UpdatedAt = time.Now().UTC()
        res := r.db.WithContext(ctx).
            Model(&bindingRow{}).
            Where("id = ?", rec.ID).
            Updates(map[string]any{
                "display_name":        rec.DisplayName,
                "status":              rec.Status,
                "acting_employee_id":  rec.ActingEmployeeID,
                "token_expires_at":    rec.TokenExpiresAt,
                "last_synced_at":      rec.LastSyncedAt,
                "updated_at":          rec.UpdatedAt,
            })
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrNotFound
        }
        return nil
    }

    func (r *GormRepo) Delete(ctx context.Context, id uuid.UUID) error {
        res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&bindingRow{})
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrNotFound
        }
        return nil
    }

    func (r *GormRepo) TouchSync(ctx context.Context, id uuid.UUID, when time.Time) error {
        return r.db.WithContext(ctx).Model(&bindingRow{}).
            Where("id = ?", id).
            Update("last_synced_at", when).Error
    }
    ```

- [ ] Step 6.3 — commit `feat(qianchuanbinding): repository + record + gorm impl`

---

## Task 7 — Service layer: validation + secret-existence checks + provider integration

- [ ] Step 7.1 — failing service tests using mock provider + in-memory secrets
  - File: `src-go/internal/qianchuanbinding/service_test.go`
    ```go
    package qianchuanbinding_test

    import (
        "context"
        "errors"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/adsplatform"
        mockprov "github.com/react-go-quick-starter/server/internal/adsplatform/mock"
        "github.com/react-go-quick-starter/server/internal/qianchuanbinding"
    )

    type fakeSecrets struct {
        present map[string]string // secret name → plaintext
    }

    func (f *fakeSecrets) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
        if v, ok := f.present[name]; ok {
            return v, nil
        }
        return "", errors.New("secret:not_found")
    }

    func TestService_Create_RejectsMissingSecret(t *testing.T) {
        repo := newMem()
        secrets := &fakeSecrets{present: map[string]string{"qc.access": "tok"}}
        prov := mockprov.New("qianchuan")
        svc := qianchuanbinding.NewService(repo, secrets, prov)
        _, err := svc.Create(context.Background(), qianchuanbinding.CreateInput{
            ProjectID:             uuid.New(),
            AdvertiserID:          "A1",
            AccessTokenSecretRef:  "qc.access",
            RefreshTokenSecretRef: "qc.refresh-missing", // not in secrets store
            CreatedBy:             uuid.New(),
        })
        if !errors.Is(err, qianchuanbinding.ErrSecretMissing) {
            t.Fatalf("want ErrSecretMissing, got %v", err)
        }
    }

    func TestService_Create_VerifiesAuthViaProvider(t *testing.T) {
        repo := newMem()
        secrets := &fakeSecrets{present: map[string]string{"qc.access": "tok", "qc.refresh": "ref"}}
        prov := mockprov.New("qianchuan")
        prov.SetMetrics(&adsplatform.MetricSnapshot{Ads: []adsplatform.AdMetric{{AdID: "AD1"}}})
        svc := qianchuanbinding.NewService(repo, secrets, prov)
        rec, err := svc.Create(context.Background(), qianchuanbinding.CreateInput{
            ProjectID:             uuid.New(),
            AdvertiserID:          "A1",
            AccessTokenSecretRef:  "qc.access",
            RefreshTokenSecretRef: "qc.refresh",
            DisplayName:           "店铺A",
            CreatedBy:             uuid.New(),
        })
        if err != nil {
            t.Fatal(err)
        }
        if rec.Status != qianchuanbinding.StatusActive {
            t.Errorf("status=%s", rec.Status)
        }
    }

    func TestService_Create_AuthExpiredFailsClosed(t *testing.T) {
        repo := newMem()
        secrets := &fakeSecrets{present: map[string]string{"qc.access": "tok", "qc.refresh": "ref"}}
        prov := mockprov.New("qianchuan")
        prov.FailNext(adsplatform.ErrAuthExpired)
        // make FetchMetrics return that error: mock has no metric-fail toggle, so use a wrapper.
        // For this stub, swap to a provider whose FetchMetrics returns ErrAuthExpired.
        svc := qianchuanbinding.NewService(repo, secrets, &authExpiredProvider{})
        _, err := svc.Create(context.Background(), qianchuanbinding.CreateInput{
            ProjectID:             uuid.New(),
            AdvertiserID:          "A1",
            AccessTokenSecretRef:  "qc.access",
            RefreshTokenSecretRef: "qc.refresh",
            CreatedBy:             uuid.New(),
        })
        if !errors.Is(err, adsplatform.ErrAuthExpired) {
            t.Fatalf("want ErrAuthExpired, got %v", err)
        }
    }

    type authExpiredProvider struct{ mockprov.Provider }

    func (*authExpiredProvider) Name() string { return "qianchuan" }
    func (*authExpiredProvider) FetchMetrics(context.Context, adsplatform.BindingRef, adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
        return nil, adsplatform.ErrAuthExpired
    }
    ```

- [ ] Step 7.2 — implement service
  - File: `src-go/internal/qianchuanbinding/service.go`
    ```go
    package qianchuanbinding

    import (
        "context"
        "errors"
        "fmt"
        "time"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/adsplatform"
    )

    // ErrSecretMissing is returned when one of the *_secret_ref values does
    // not resolve in the secrets store.
    var ErrSecretMissing = errors.New("qianchuanbinding: secret_missing")

    // SecretsResolver is the narrow contract Service depends on. It mirrors
    // the surface of internal/secrets.Service.Resolve so service tests can
    // pass an in-memory fake without bootstrapping the cipher.
    type SecretsResolver interface {
        Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
    }

    // CreateInput is the bag passed to Service.Create.
    type CreateInput struct {
        ProjectID             uuid.UUID
        AdvertiserID          string
        AwemeID               string
        DisplayName           string
        ActingEmployeeID      *uuid.UUID
        AccessTokenSecretRef  string
        RefreshTokenSecretRef string
        CreatedBy             uuid.UUID
    }

    // UpdateInput patches one binding's mutable fields.
    type UpdateInput struct {
        DisplayName      *string
        Status           *string
        ActingEmployeeID *uuid.UUID
    }

    // Service composes repository + secrets + provider and exposes the
    // operations the HTTP handler invokes.
    type Service struct {
        repo     Repository
        secrets  SecretsResolver
        provider adsplatform.Provider
    }

    // NewService wires the dependencies.
    func NewService(repo Repository, secrets SecretsResolver, provider adsplatform.Provider) *Service {
        return &Service{repo: repo, secrets: secrets, provider: provider}
    }

    // Create validates inputs, ensures both secret refs resolve, runs a
    // verification FetchMetrics call, and persists the row in 'active' state.
    // Any provider error short-circuits the create — we never persist a
    // binding whose tokens we could not validate.
    func (s *Service) Create(ctx context.Context, in CreateInput) (*Record, error) {
        if in.AdvertiserID == "" {
            return nil, fmt.Errorf("qianchuanbinding: advertiser_id required")
        }
        access, err := s.secrets.Resolve(ctx, in.ProjectID, in.AccessTokenSecretRef)
        if err != nil {
            return nil, fmt.Errorf("%w: %s", ErrSecretMissing, in.AccessTokenSecretRef)
        }
        if _, err := s.secrets.Resolve(ctx, in.ProjectID, in.RefreshTokenSecretRef); err != nil {
            return nil, fmt.Errorf("%w: %s", ErrSecretMissing, in.RefreshTokenSecretRef)
        }
        // Auth probe — we do NOT persist if the token is dead on arrival.
        if _, err := s.provider.FetchMetrics(ctx, adsplatform.BindingRef{
            AdvertiserID: in.AdvertiserID, AwemeID: in.AwemeID, AccessToken: access,
        }, adsplatform.MetricDimensions{Range: "today"}); err != nil {
            return nil, err
        }
        rec := &Record{
            ProjectID:             in.ProjectID,
            AdvertiserID:          in.AdvertiserID,
            AwemeID:               in.AwemeID,
            DisplayName:           in.DisplayName,
            Status:                StatusActive,
            ActingEmployeeID:      in.ActingEmployeeID,
            AccessTokenSecretRef:  in.AccessTokenSecretRef,
            RefreshTokenSecretRef: in.RefreshTokenSecretRef,
            CreatedBy:             in.CreatedBy,
        }
        if err := s.repo.Create(ctx, rec); err != nil {
            return nil, err
        }
        return rec, nil
    }

    // Get fetches a binding by id.
    func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Record, error) {
        return s.repo.Get(ctx, id)
    }

    // List lists bindings under a project.
    func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]*Record, error) {
        return s.repo.ListByProject(ctx, projectID)
    }

    // Update patches mutable fields.
    func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Record, error) {
        rec, err := s.repo.Get(ctx, id)
        if err != nil {
            return nil, err
        }
        if in.DisplayName != nil {
            rec.DisplayName = *in.DisplayName
        }
        if in.Status != nil {
            rec.Status = *in.Status
        }
        if in.ActingEmployeeID != nil {
            rec.ActingEmployeeID = in.ActingEmployeeID
        }
        if err := s.repo.Update(ctx, rec); err != nil {
            return nil, err
        }
        return rec, nil
    }

    // Delete removes a binding (cascades action_logs once Plan 3D lands).
    func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
        return s.repo.Delete(ctx, id)
    }

    // Sync resolves the access token and pulls a fresh metrics snapshot,
    // updating last_synced_at on success.
    func (s *Service) Sync(ctx context.Context, id uuid.UUID) error {
        rec, err := s.repo.Get(ctx, id)
        if err != nil {
            return err
        }
        access, err := s.secrets.Resolve(ctx, rec.ProjectID, rec.AccessTokenSecretRef)
        if err != nil {
            return fmt.Errorf("%w: %s", ErrSecretMissing, rec.AccessTokenSecretRef)
        }
        if _, err := s.provider.FetchMetrics(ctx, adsplatform.BindingRef{
            AdvertiserID: rec.AdvertiserID, AwemeID: rec.AwemeID, AccessToken: access,
        }, adsplatform.MetricDimensions{Range: "today"}); err != nil {
            return err
        }
        return s.repo.TouchSync(ctx, id, time.Now().UTC())
    }

    // Test runs a sample FetchMetrics and returns the result for FE health-check.
    func (s *Service) Test(ctx context.Context, id uuid.UUID) (*adsplatform.MetricSnapshot, error) {
        rec, err := s.repo.Get(ctx, id)
        if err != nil {
            return nil, err
        }
        access, err := s.secrets.Resolve(ctx, rec.ProjectID, rec.AccessTokenSecretRef)
        if err != nil {
            return nil, fmt.Errorf("%w: %s", ErrSecretMissing, rec.AccessTokenSecretRef)
        }
        return s.provider.FetchMetrics(ctx, adsplatform.BindingRef{
            AdvertiserID: rec.AdvertiserID, AwemeID: rec.AwemeID, AccessToken: access,
        }, adsplatform.MetricDimensions{Range: "today"})
    }
    ```

- [ ] Step 7.3 — commit `feat(qianchuanbinding): service with secret-resolve + auth-probe`

---

## Task 8 — Handler: REST endpoints + audit + RBAC wiring

- [ ] Step 8.1 — failing handler tests
  - File: `src-go/internal/handler/qianchuan_bindings_handler_test.go`
    ```go
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
        "github.com/react-go-quick-starter/server/internal/handler"
        "github.com/react-go-quick-starter/server/internal/qianchuanbinding"
    )

    func TestHandler_Create_201(t *testing.T) {
        svc := newFakeBindingsService(t)
        e := echo.New()
        h := handler.NewQianchuanBindingsHandler(svc, nil)
        h.Register(e.Group("/api/v1/projects/:pid"))
        body := map[string]any{
            "advertiser_id":            "A1",
            "aweme_id":                 "W1",
            "display_name":             "店铺A",
            "access_token_secret_ref":  "qc.A1.access",
            "refresh_token_secret_ref": "qc.A1.refresh",
        }
        buf, _ := json.Marshal(body)
        req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+uuid.New().String()+"/qianchuan/bindings", bytes.NewReader(buf))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        e.ServeHTTP(rec, req)
        if rec.Code != http.StatusCreated {
            t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
        }
    }

    // newFakeBindingsService constructs a Service backed by in-memory repo +
    // fake secrets + mock provider. (Implementation lives in helpers_test.go.)
    func newFakeBindingsService(t *testing.T) handler.QianchuanBindingsService { /* see helpers_test.go */ return nil }

    var _ context.Context // keep import
    var _ qianchuanbinding.CreateInput
    ```
  - Add the helper construction in `helpers_test.go` next to existing test helpers.

- [ ] Step 8.2 — implement handler
  - File: `src-go/internal/handler/qianchuan_bindings_handler.go`
    ```go
    package handler

    import (
        "context"
        "errors"
        "net/http"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"

        "github.com/react-go-quick-starter/server/internal/adsplatform"
        "github.com/react-go-quick-starter/server/internal/i18n"
        appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/qianchuanbinding"
    )

    // QianchuanBindingsService is the narrow contract the handler depends on.
    type QianchuanBindingsService interface {
        Create(ctx context.Context, in qianchuanbinding.CreateInput) (*qianchuanbinding.Record, error)
        Get(ctx context.Context, id uuid.UUID) (*qianchuanbinding.Record, error)
        List(ctx context.Context, projectID uuid.UUID) ([]*qianchuanbinding.Record, error)
        Update(ctx context.Context, id uuid.UUID, in qianchuanbinding.UpdateInput) (*qianchuanbinding.Record, error)
        Delete(ctx context.Context, id uuid.UUID) error
        Sync(ctx context.Context, id uuid.UUID) error
        Test(ctx context.Context, id uuid.UUID) (*adsplatform.MetricSnapshot, error)
    }

    // QianchuanBindingsAuditEmitter is the narrow audit contract.
    type QianchuanBindingsAuditEmitter interface {
        Emit(ctx context.Context, projectID, actorUserID, bindingID uuid.UUID, action string, payload string)
    }

    // QianchuanBindingsHandler exposes /api/v1/.../qianchuan/bindings endpoints.
    type QianchuanBindingsHandler struct {
        service QianchuanBindingsService
        audit   QianchuanBindingsAuditEmitter
    }

    // NewQianchuanBindingsHandler wires the handler.
    func NewQianchuanBindingsHandler(svc QianchuanBindingsService, audit QianchuanBindingsAuditEmitter) *QianchuanBindingsHandler {
        return &QianchuanBindingsHandler{service: svc, audit: audit}
    }

    // Register attaches routes onto a project-scoped Echo group (/api/v1/projects/:pid).
    func (h *QianchuanBindingsHandler) Register(g *echo.Group) {
        g.GET("/qianchuan/bindings", h.List, appMiddleware.Require(appMiddleware.ActionQianchuanBindingRead))
        g.POST("/qianchuan/bindings", h.Create, appMiddleware.Require(appMiddleware.ActionQianchuanBindingCreate))
    }

    // RegisterFlat attaches the per-binding endpoints (project-id is read from the row).
    func (h *QianchuanBindingsHandler) RegisterFlat(e *echo.Echo) {
        g := e.Group("/api/v1/qianchuan/bindings")
        g.PATCH("/:id", h.Update, appMiddleware.Require(appMiddleware.ActionQianchuanBindingUpdate))
        g.DELETE("/:id", h.Delete, appMiddleware.Require(appMiddleware.ActionQianchuanBindingDelete))
        g.POST("/:id/sync", h.Sync, appMiddleware.Require(appMiddleware.ActionQianchuanBindingSync))
        g.POST("/:id/test", h.Test, appMiddleware.Require(appMiddleware.ActionQianchuanBindingTest))
    }

    // ---------------- request / response shapes ----------------

    type createBindingRequest struct {
        AdvertiserID          string  `json:"advertiser_id"`
        AwemeID               string  `json:"aweme_id"`
        DisplayName           string  `json:"display_name"`
        ActingEmployeeID      *string `json:"acting_employee_id,omitempty"`
        AccessTokenSecretRef  string  `json:"access_token_secret_ref"`
        RefreshTokenSecretRef string  `json:"refresh_token_secret_ref"`
    }

    type updateBindingRequest struct {
        DisplayName      *string `json:"display_name,omitempty"`
        Status           *string `json:"status,omitempty"`
        ActingEmployeeID *string `json:"acting_employee_id,omitempty"`
    }

    type bindingDTO struct {
        ID                    uuid.UUID  `json:"id"`
        ProjectID             uuid.UUID  `json:"project_id"`
        AdvertiserID          string     `json:"advertiser_id"`
        AwemeID               string     `json:"aweme_id,omitempty"`
        DisplayName           string     `json:"display_name,omitempty"`
        Status                string     `json:"status"`
        ActingEmployeeID      *uuid.UUID `json:"acting_employee_id,omitempty"`
        AccessTokenSecretRef  string     `json:"access_token_secret_ref"`
        RefreshTokenSecretRef string     `json:"refresh_token_secret_ref"`
        TokenExpiresAt        any        `json:"token_expires_at,omitempty"`
        LastSyncedAt          any        `json:"last_synced_at,omitempty"`
    }

    func toBindingDTO(r *qianchuanbinding.Record) *bindingDTO {
        return &bindingDTO{
            ID: r.ID, ProjectID: r.ProjectID, AdvertiserID: r.AdvertiserID, AwemeID: r.AwemeID,
            DisplayName: r.DisplayName, Status: r.Status, ActingEmployeeID: r.ActingEmployeeID,
            AccessTokenSecretRef: r.AccessTokenSecretRef, RefreshTokenSecretRef: r.RefreshTokenSecretRef,
            TokenExpiresAt: r.TokenExpiresAt, LastSyncedAt: r.LastSyncedAt,
        }
    }

    // ---------------- endpoints ----------------

    // List GET /api/v1/projects/:pid/qianchuan/bindings.
    func (h *QianchuanBindingsHandler) List(c echo.Context) error {
        projectID, err := uuid.Parse(c.Param("pid"))
        if err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        rows, err := h.service.List(c.Request().Context(), projectID)
        if err != nil {
            return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
        }
        out := make([]*bindingDTO, 0, len(rows))
        for _, r := range rows {
            out = append(out, toBindingDTO(r))
        }
        return c.JSON(http.StatusOK, out)
    }

    // Create POST /api/v1/projects/:pid/qianchuan/bindings.
    func (h *QianchuanBindingsHandler) Create(c echo.Context) error {
        projectID, err := uuid.Parse(c.Param("pid"))
        if err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        var req createBindingRequest
        if err := c.Bind(&req); err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        if req.AdvertiserID == "" || req.AccessTokenSecretRef == "" || req.RefreshTokenSecretRef == "" {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        actor := actorUserID(c)
        var actingEmp *uuid.UUID
        if req.ActingEmployeeID != nil {
            id, err := uuid.Parse(*req.ActingEmployeeID)
            if err != nil {
                return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
            }
            actingEmp = &id
        }
        rec, err := h.service.Create(c.Request().Context(), qianchuanbinding.CreateInput{
            ProjectID: projectID,
            AdvertiserID: req.AdvertiserID, AwemeID: req.AwemeID, DisplayName: req.DisplayName,
            ActingEmployeeID:      actingEmp,
            AccessTokenSecretRef:  req.AccessTokenSecretRef,
            RefreshTokenSecretRef: req.RefreshTokenSecretRef,
            CreatedBy:             actor,
        })
        if err != nil {
            return mapBindingError(c, err)
        }
        h.emitAudit(c.Request().Context(), projectID, actor, rec.ID, "qianchuan_binding.create", "")
        return c.JSON(http.StatusCreated, toBindingDTO(rec))
    }

    // Update PATCH /api/v1/qianchuan/bindings/:id.
    func (h *QianchuanBindingsHandler) Update(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        var req updateBindingRequest
        if err := c.Bind(&req); err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        in := qianchuanbinding.UpdateInput{DisplayName: req.DisplayName, Status: req.Status}
        if req.ActingEmployeeID != nil {
            empID, err := uuid.Parse(*req.ActingEmployeeID)
            if err != nil {
                return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
            }
            in.ActingEmployeeID = &empID
        }
        rec, err := h.service.Update(c.Request().Context(), id, in)
        if err != nil {
            return mapBindingError(c, err)
        }
        h.emitAudit(c.Request().Context(), rec.ProjectID, actorUserID(c), rec.ID, "qianchuan_binding.update", "")
        return c.JSON(http.StatusOK, toBindingDTO(rec))
    }

    // Delete DELETE /api/v1/qianchuan/bindings/:id.
    func (h *QianchuanBindingsHandler) Delete(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        rec, err := h.service.Get(c.Request().Context(), id)
        if err != nil {
            return mapBindingError(c, err)
        }
        if err := h.service.Delete(c.Request().Context(), id); err != nil {
            return mapBindingError(c, err)
        }
        h.emitAudit(c.Request().Context(), rec.ProjectID, actorUserID(c), id, "qianchuan_binding.delete", "")
        return c.NoContent(http.StatusNoContent)
    }

    // Sync POST /api/v1/qianchuan/bindings/:id/sync.
    func (h *QianchuanBindingsHandler) Sync(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        if err := h.service.Sync(c.Request().Context(), id); err != nil {
            return mapBindingError(c, err)
        }
        return c.NoContent(http.StatusNoContent)
    }

    // Test POST /api/v1/qianchuan/bindings/:id/test.
    func (h *QianchuanBindingsHandler) Test(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
        }
        snap, err := h.service.Test(c.Request().Context(), id)
        if err != nil {
            return mapBindingError(c, err)
        }
        return c.JSON(http.StatusOK, snap)
    }

    // mapBindingError converts service-layer errors to HTTP status + i18n message key.
    func mapBindingError(c echo.Context, err error) error {
        switch {
        case errors.Is(err, qianchuanbinding.ErrAdvertiserAlreadyBound):
            return localizedError(c, http.StatusConflict, i18n.MsgQianchuanAdvertiserAlreadyBound)
        case errors.Is(err, qianchuanbinding.ErrSecretMissing):
            return localizedError(c, http.StatusBadRequest, i18n.MsgQianchuanSecretMissing)
        case errors.Is(err, qianchuanbinding.ErrNotFound):
            return localizedError(c, http.StatusNotFound, i18n.MsgQianchuanBindingNotFound)
        case errors.Is(err, adsplatform.ErrAuthExpired):
            return localizedError(c, http.StatusUnauthorized, i18n.MsgQianchuanAuthExpired)
        case errors.Is(err, adsplatform.ErrRateLimited):
            return localizedError(c, http.StatusTooManyRequests, i18n.MsgQianchuanRateLimited)
        default:
            return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
        }
    }

    func (h *QianchuanBindingsHandler) emitAudit(ctx context.Context, projectID, actor, bindingID uuid.UUID, action, payload string) {
        if h.audit == nil {
            return
        }
        h.audit.Emit(ctx, projectID, actor, bindingID, action, payload)
    }

    // actorUserID is supplied by middleware; falls back to nil-uuid in tests.
    func actorUserID(c echo.Context) uuid.UUID {
        if v, ok := c.Get("user_id").(uuid.UUID); ok {
            return v
        }
        return uuid.Nil
    }

    var _ = model.AuditResourceTypeQianchuanBinding // ensure constant lands
    ```

- [ ] Step 8.3 — extend `internal/middleware/rbac.go` and `internal/i18n` with new ActionIDs + message keys
  - Add to `middleware/rbac.go`:
    - `ActionQianchuanBindingRead = "qianchuan_binding.read"` → viewer+
    - `ActionQianchuanBindingCreate = "qianchuan_binding.create"` → admin+
    - `ActionQianchuanBindingUpdate = "qianchuan_binding.update"` → editor+
    - `ActionQianchuanBindingDelete = "qianchuan_binding.delete"` → admin+
    - `ActionQianchuanBindingSync = "qianchuan_binding.sync"` → editor+
    - `ActionQianchuanBindingTest = "qianchuan_binding.test"` → editor+
  - Add to `internal/i18n/messages.go`:
    - `MsgQianchuanAdvertiserAlreadyBound`, `MsgQianchuanSecretMissing`, `MsgQianchuanBindingNotFound`, `MsgQianchuanAuthExpired`, `MsgQianchuanRateLimited`
  - Add corresponding entries to `lib/i18n/locales/{en,zh-CN}.json`.

- [ ] Step 8.4 — wire into `internal/server/routes.go`
  - In the bootstrap section, after secrets/service is constructed (Plan 1B owns that), add:
    ```go
    bindingRepo := qianchuanbinding.NewGormRepo(db)
    qcRegistry := adsplatform.NewRegistry()
    qianchuan.Register(qcRegistry)
    qcProvider, err := qcRegistry.Resolve("qianchuan")
    if err != nil {
        log.Fatalf("qianchuan provider not registered: %v", err)
    }
    bindingSvc := qianchuanbinding.NewService(bindingRepo, secretsService, qcProvider)
    bindingAudit := bindingsAuditEmitter{auditSvc: auditSvc}
    bindingH := handler.NewQianchuanBindingsHandler(bindingSvc, bindingAudit)
    ```
  - Within `projectGroup` block:
    ```go
    bindingH.Register(projectGroup)
    ```
  - Below the project group:
    ```go
    bindingH.RegisterFlat(e)
    ```
  - Define `bindingsAuditEmitter` adapter at the bottom of `routes.go` (mirror `projectTemplateAuditEmitter`).

- [ ] Step 8.5 — verify
  - `rtk go test ./internal/handler/...` — handler tests pass.
  - `rtk go build ./...` — compile.

- [ ] Step 8.6 — commit `feat(qianchuan): bindings handler + REST endpoints + audit + rbac`

---

## Task 9 — Integration test: real PG + mock provider end-to-end

- [ ] Step 9.1 — write integration test (build tag `integration`)
  - File: `src-go/internal/qianchuanbinding/repo_integration_test.go`
    ```go
    //go:build integration
    // +build integration

    package qianchuanbinding_test

    import (
        "context"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/qianchuanbinding"
        "github.com/react-go-quick-starter/server/internal/testdb"
    )

    func TestGormRepo_RoundTrip(t *testing.T) {
        db := testdb.OpenForTest(t)
        repo := qianchuanbinding.NewGormRepo(db)
        ctx := context.Background()
        proj := testdb.SeedProject(t, db)
        rec := &qianchuanbinding.Record{
            ProjectID: proj, AdvertiserID: "A1", AwemeID: "W1",
            DisplayName: "店铺A", Status: "active",
            AccessTokenSecretRef: "qc.A1.access", RefreshTokenSecretRef: "qc.A1.refresh",
            CreatedBy: testdb.SeedUser(t, db),
        }
        if err := repo.Create(ctx, rec); err != nil {
            t.Fatal(err)
        }
        got, err := repo.Get(ctx, rec.ID)
        if err != nil || got.AdvertiserID != "A1" {
            t.Fatalf("Get: %+v err=%v", got, err)
        }
        // Duplicate insert is rejected.
        if err := repo.Create(ctx, &qianchuanbinding.Record{
            ProjectID: proj, AdvertiserID: "A1", AwemeID: "W1",
            AccessTokenSecretRef: "x", RefreshTokenSecretRef: "y", CreatedBy: uuid.New(), Status: "active",
        }); err != qianchuanbinding.ErrAdvertiserAlreadyBound {
            t.Fatalf("want ErrAdvertiserAlreadyBound, got %v", err)
        }
    }
    ```

- [ ] Step 9.2 — run
  - `rtk go test -tags=integration ./internal/qianchuanbinding/...`

- [ ] Step 9.3 — commit `test(qianchuanbinding): integration round-trip + duplicate rejection`

---

## Task 10 — FE store + types

- [ ] Step 10.1 — write failing Jest store test
  - File: `lib/stores/qianchuan-bindings-store.test.ts`
    ```ts
    import { act } from "@testing-library/react";
    import { useQianchuanBindingsStore } from "./qianchuan-bindings-store";

    jest.mock("@/lib/api-client", () => ({
      createApiClient: () => ({
        get: jest.fn().mockResolvedValue({ data: [{ id: "b1", project_id: "p1", advertiser_id: "A1", status: "active", access_token_secret_ref: "qc.access", refresh_token_secret_ref: "qc.refresh" }] }),
        post: jest.fn().mockResolvedValue({ data: { id: "b2" } }),
      }),
    }));
    jest.mock("./auth-store", () => ({
      useAuthStore: { getState: () => ({ accessToken: "t" }) },
    }));

    describe("useQianchuanBindingsStore", () => {
      beforeEach(() => useQianchuanBindingsStore.setState({ byProject: {}, loading: {} }));

      it("loads bindings into byProject", async () => {
        await act(async () => {
          await useQianchuanBindingsStore.getState().fetchBindings("p1");
        });
        const rows = useQianchuanBindingsStore.getState().byProject["p1"];
        expect(rows).toHaveLength(1);
        expect(rows[0].advertiserId).toBe("A1");
      });
    });
    ```

- [ ] Step 10.2 — implement store
  - File: `lib/stores/qianchuan-bindings-store.ts`
    ```ts
    "use client";

    import { create } from "zustand";
    import { toast } from "sonner";
    import { createApiClient } from "@/lib/api-client";
    import { useAuthStore } from "./auth-store";

    const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

    export type QianchuanBindingStatus = "active" | "auth_expired" | "paused";

    export interface QianchuanBinding {
      id: string;
      projectId: string;
      advertiserId: string;
      awemeId?: string;
      displayName?: string;
      status: QianchuanBindingStatus;
      actingEmployeeId?: string;
      accessTokenSecretRef: string;
      refreshTokenSecretRef: string;
      tokenExpiresAt?: string;
      lastSyncedAt?: string;
    }

    interface CreateInput {
      advertiserId: string;
      awemeId?: string;
      displayName?: string;
      actingEmployeeId?: string;
      accessTokenSecretRef: string;
      refreshTokenSecretRef: string;
    }

    interface State {
      byProject: Record<string, QianchuanBinding[]>;
      loading: Record<string, boolean>;
      fetchBindings: (projectId: string) => Promise<void>;
      createBinding: (projectId: string, input: CreateInput) => Promise<QianchuanBinding | null>;
      updateBinding: (id: string, patch: Partial<Pick<QianchuanBinding, "displayName" | "status" | "actingEmployeeId">>) => Promise<void>;
      deleteBinding: (projectId: string, id: string) => Promise<void>;
      syncBinding: (id: string) => Promise<void>;
      testBinding: (id: string) => Promise<{ ok: boolean; detail?: string }>;
    }

    const getApi = () => createApiClient(API_URL);
    const tok = () => (useAuthStore.getState() as { accessToken?: string | null }).accessToken ?? null;

    type Wire = {
      id: string;
      project_id: string;
      advertiser_id: string;
      aweme_id?: string;
      display_name?: string;
      status: QianchuanBindingStatus;
      acting_employee_id?: string;
      access_token_secret_ref: string;
      refresh_token_secret_ref: string;
      token_expires_at?: string;
      last_synced_at?: string;
    };

    const fromWire = (w: Wire): QianchuanBinding => ({
      id: w.id,
      projectId: w.project_id,
      advertiserId: w.advertiser_id,
      awemeId: w.aweme_id,
      displayName: w.display_name,
      status: w.status,
      actingEmployeeId: w.acting_employee_id,
      accessTokenSecretRef: w.access_token_secret_ref,
      refreshTokenSecretRef: w.refresh_token_secret_ref,
      tokenExpiresAt: w.token_expires_at,
      lastSyncedAt: w.last_synced_at,
    });

    export const useQianchuanBindingsStore = create<State>()((set, get) => ({
      byProject: {},
      loading: {},

      fetchBindings: async (projectId) => {
        const token = tok();
        if (!token) return;
        set((s) => ({ loading: { ...s.loading, [projectId]: true } }));
        try {
          const { data } = await getApi().get<Wire[]>(`/api/v1/projects/${projectId}/qianchuan/bindings`, { token });
          set((s) => ({ byProject: { ...s.byProject, [projectId]: (data ?? []).map(fromWire) } }));
        } catch (e) {
          toast.error(`加载千川绑定失败：${(e as Error).message}`);
        } finally {
          set((s) => ({ loading: { ...s.loading, [projectId]: false } }));
        }
      },

      createBinding: async (projectId, input) => {
        const token = tok();
        if (!token) return null;
        try {
          const { data } = await getApi().post<Wire>(`/api/v1/projects/${projectId}/qianchuan/bindings`, {
            advertiser_id: input.advertiserId,
            aweme_id: input.awemeId,
            display_name: input.displayName,
            acting_employee_id: input.actingEmployeeId,
            access_token_secret_ref: input.accessTokenSecretRef,
            refresh_token_secret_ref: input.refreshTokenSecretRef,
          }, { token });
          await get().fetchBindings(projectId);
          toast.success("绑定已创建");
          return data ? fromWire(data) : null;
        } catch (e) {
          toast.error(`创建失败：${(e as Error).message}`);
          return null;
        }
      },

      updateBinding: async (id, patch) => {
        const token = tok();
        if (!token) return;
        try {
          await getApi().patch(`/api/v1/qianchuan/bindings/${id}`, {
            display_name: patch.displayName,
            status: patch.status,
            acting_employee_id: patch.actingEmployeeId,
          }, { token });
          // refetch each project that contains this binding
          for (const [pid, rows] of Object.entries(get().byProject)) {
            if (rows.some((b) => b.id === id)) {
              await get().fetchBindings(pid);
            }
          }
        } catch (e) {
          toast.error(`更新失败：${(e as Error).message}`);
        }
      },

      deleteBinding: async (projectId, id) => {
        const token = tok();
        if (!token) return;
        try {
          await getApi().delete(`/api/v1/qianchuan/bindings/${id}`, { token });
          await get().fetchBindings(projectId);
          toast.success("绑定已删除");
        } catch (e) {
          toast.error(`删除失败：${(e as Error).message}`);
        }
      },

      syncBinding: async (id) => {
        const token = tok();
        if (!token) return;
        try {
          await getApi().post(`/api/v1/qianchuan/bindings/${id}/sync`, {}, { token });
          toast.success("已触发同步");
        } catch (e) {
          toast.error(`同步失败：${(e as Error).message}`);
        }
      },

      testBinding: async (id) => {
        const token = tok();
        if (!token) return { ok: false, detail: "未登录" };
        try {
          await getApi().post(`/api/v1/qianchuan/bindings/${id}/test`, {}, { token });
          return { ok: true };
        } catch (e) {
          return { ok: false, detail: (e as Error).message };
        }
      },
    }));
    ```
  - Run `rtk pnpm test lib/stores/qianchuan-bindings-store.test.ts` — green.

- [ ] Step 10.3 — commit `feat(fe): qianchuan bindings zustand store`

---

## Task 11 — FE bindings list page + create form

- [ ] Step 11.1 — page route
  - File: `app/(dashboard)/projects/[id]/qianchuan/bindings/page.tsx`
    ```tsx
    "use client";

    import { useEffect, useMemo, useState } from "react";
    import { useParams } from "next/navigation";
    import { Button } from "@/components/ui/button";
    import { Badge } from "@/components/ui/badge";
    import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
    import { useQianchuanBindingsStore, type QianchuanBindingStatus } from "@/lib/stores/qianchuan-bindings-store";
    import { CreateBindingDialog } from "@/components/qianchuan/create-binding-dialog";

    const STATUS_LABEL: Record<QianchuanBindingStatus, { label: string; variant: "default" | "secondary" | "destructive" }> = {
      active: { label: "运行中", variant: "default" },
      auth_expired: { label: "授权过期", variant: "destructive" },
      paused: { label: "已暂停", variant: "secondary" },
    };

    export default function QianchuanBindingsPage() {
      const { id: projectId } = useParams<{ id: string }>();
      const rows = useQianchuanBindingsStore((s) => s.byProject[projectId] ?? []);
      const loading = useQianchuanBindingsStore((s) => s.loading[projectId] ?? false);
      const fetchBindings = useQianchuanBindingsStore((s) => s.fetchBindings);
      const updateBinding = useQianchuanBindingsStore((s) => s.updateBinding);
      const syncBinding = useQianchuanBindingsStore((s) => s.syncBinding);
      const testBinding = useQianchuanBindingsStore((s) => s.testBinding);
      const [open, setOpen] = useState(false);

      useEffect(() => {
        void fetchBindings(projectId);
      }, [projectId, fetchBindings]);

      const sorted = useMemo(() => [...rows].sort((a, b) => (a.displayName ?? "").localeCompare(b.displayName ?? "")), [rows]);

      return (
        <div className="space-y-4 p-4">
          <div className="flex items-center justify-between">
            <h1 className="text-2xl font-semibold">千川账号绑定</h1>
            <Button onClick={() => setOpen(true)}>新增绑定</Button>
          </div>
          {loading && <p className="text-sm text-muted-foreground">加载中…</p>}
          {!loading && sorted.length === 0 && (
            <Card><CardContent className="py-8 text-center text-muted-foreground">尚未绑定任何千川账号。点击右上角"新增绑定"开始。</CardContent></Card>
          )}
          <div className="grid gap-3">
            {sorted.map((b) => (
              <Card key={b.id}>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-base">
                    {b.displayName || b.advertiserId}
                    <span className="ml-2 text-xs text-muted-foreground">advertiser_id={b.advertiserId}{b.awemeId ? ` · aweme_id=${b.awemeId}` : ""}</span>
                  </CardTitle>
                  <Badge variant={STATUS_LABEL[b.status].variant}>{STATUS_LABEL[b.status].label}</Badge>
                </CardHeader>
                <CardContent className="flex items-center gap-2 text-sm">
                  <span className="text-muted-foreground">最近同步：{b.lastSyncedAt ?? "—"}</span>
                  <span className="ml-auto flex gap-2">
                    <Button size="sm" variant="outline" onClick={async () => {
                      const r = await testBinding(b.id);
                      if (r.ok) { await syncBinding(b.id); }
                    }}>测试</Button>
                    <Button size="sm" variant="outline" onClick={() => syncBinding(b.id)}>同步</Button>
                    {b.status === "paused"
                      ? <Button size="sm" onClick={() => updateBinding(b.id, { status: "active" })}>恢复</Button>
                      : <Button size="sm" variant="secondary" onClick={() => updateBinding(b.id, { status: "paused" })}>暂停</Button>}
                  </span>
                </CardContent>
              </Card>
            ))}
          </div>
          <CreateBindingDialog projectId={projectId} open={open} onOpenChange={setOpen} />
        </div>
      );
    }
    ```

- [ ] Step 11.2 — create-binding dialog with secret-ref dropdown
  - File: `components/qianchuan/create-binding-dialog.tsx`
    ```tsx
    "use client";

    import { useEffect, useState } from "react";
    import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
    import { Input } from "@/components/ui/input";
    import { Label } from "@/components/ui/label";
    import { Button } from "@/components/ui/button";
    import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
    import { useQianchuanBindingsStore } from "@/lib/stores/qianchuan-bindings-store";
    import { useSecretsStore } from "@/lib/stores/secrets-store"; // owned by Plan 1B; if absent, fall back to a plain text input

    interface Props {
      projectId: string;
      open: boolean;
      onOpenChange: (next: boolean) => void;
    }

    /**
     * Plan 3A only supports MANUAL token pasting via secret-ref selection.
     * The user is expected to have previously created two secrets (one for the
     * access token, one for the refresh token) on the project secrets page (1B).
     * Plan 3B replaces this dialog with an OAuth-driven flow.
     */
    export function CreateBindingDialog({ projectId, open, onOpenChange }: Props) {
      const create = useQianchuanBindingsStore((s) => s.createBinding);
      const fetchSecrets = useSecretsStore?.((s) => s.fetchSecrets);
      const secretNames: string[] = useSecretsStore?.((s) => s.byProject[projectId]?.map((x) => x.name) ?? []) ?? [];
      const [advertiserId, setAdvertiserId] = useState("");
      const [awemeId, setAwemeId] = useState("");
      const [displayName, setDisplayName] = useState("");
      const [accessRef, setAccessRef] = useState("");
      const [refreshRef, setRefreshRef] = useState("");
      const [submitting, setSubmitting] = useState(false);

      useEffect(() => {
        if (open && fetchSecrets) {
          void fetchSecrets(projectId);
        }
      }, [open, projectId, fetchSecrets]);

      const reset = () => { setAdvertiserId(""); setAwemeId(""); setDisplayName(""); setAccessRef(""); setRefreshRef(""); };

      const onSubmit = async () => {
        if (!advertiserId || !accessRef || !refreshRef) return;
        setSubmitting(true);
        const out = await create(projectId, {
          advertiserId, awemeId: awemeId || undefined, displayName: displayName || undefined,
          accessTokenSecretRef: accessRef, refreshTokenSecretRef: refreshRef,
        });
        setSubmitting(false);
        if (out) {
          reset();
          onOpenChange(false);
        }
      };

      return (
        <Dialog open={open} onOpenChange={onOpenChange}>
          <DialogContent>
            <DialogHeader><DialogTitle>新增千川绑定（手动 token）</DialogTitle></DialogHeader>
            <div className="grid gap-3 py-2">
              <div className="grid gap-1.5">
                <Label>advertiser_id *</Label>
                <Input value={advertiserId} onChange={(e) => setAdvertiserId(e.target.value)} placeholder="如 1234567890" />
              </div>
              <div className="grid gap-1.5">
                <Label>aweme_id（可选）</Label>
                <Input value={awemeId} onChange={(e) => setAwemeId(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label>显示名</Label>
                <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="店铺A 主直播间" />
              </div>
              <div className="grid gap-1.5">
                <Label>access_token 密钥 *</Label>
                <SecretRefSelect value={accessRef} onChange={setAccessRef} options={secretNames} />
              </div>
              <div className="grid gap-1.5">
                <Label>refresh_token 密钥 *</Label>
                <SecretRefSelect value={refreshRef} onChange={setRefreshRef} options={secretNames} />
              </div>
              <p className="text-xs text-muted-foreground">提示：OAuth 一键绑定将在 Plan 3B 推出。当前需先在 项目设置 → 密钥管理 创建两个密钥。</p>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
              <Button onClick={onSubmit} disabled={submitting}>{submitting ? "创建中…" : "创建"}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      );
    }

    function SecretRefSelect({ value, onChange, options }: { value: string; onChange: (v: string) => void; options: string[] }) {
      if (!options || options.length === 0) {
        return <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder="未发现项目密钥；请先在密钥管理页创建" />;
      }
      return (
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger><SelectValue placeholder="选择密钥…" /></SelectTrigger>
          <SelectContent>
            {options.map((name) => (<SelectItem key={name} value={name}>{name}</SelectItem>))}
          </SelectContent>
        </Select>
      );
    }
    ```

- [ ] Step 11.3 — failing Jest test for the form
  - File: `components/qianchuan/create-binding-dialog.test.tsx`
    ```tsx
    import { render, screen, fireEvent } from "@testing-library/react";
    import { CreateBindingDialog } from "./create-binding-dialog";

    jest.mock("@/lib/stores/qianchuan-bindings-store", () => ({
      useQianchuanBindingsStore: (sel: any) => sel({ createBinding: jest.fn().mockResolvedValue({ id: "b1" }) }),
    }));
    jest.mock("@/lib/stores/secrets-store", () => ({
      useSecretsStore: (sel: any) => sel({ byProject: { p1: [{ name: "qc.A1.access" }, { name: "qc.A1.refresh" }] }, fetchSecrets: jest.fn() }),
    }));

    describe("CreateBindingDialog", () => {
      it("requires advertiser_id + both secret refs", () => {
        render(<CreateBindingDialog projectId="p1" open onOpenChange={() => {}} />);
        const submit = screen.getByText("创建") as HTMLButtonElement;
        // initial state: button enabled but onSubmit guards on validation; click is a no-op
        fireEvent.click(submit);
        expect(screen.getByText(/Plan 3B/)).toBeInTheDocument();
      });
    });
    ```

- [ ] Step 11.4 — add nav link to project sidebar
  - Edit `components/sidebar.tsx` (or whichever file owns project nav) — search for `qianchuan` / `bindings` insertion point. If a project sub-nav does not yet exist (1B is creating it), add an entry to the new nav schema; otherwise add a NavItem `{ label: "千川绑定", href: \`/projects/${id}/qianchuan/bindings\` }`. Coordinate with 1B.

- [ ] Step 11.5 — verify
  - `rtk pnpm test components/qianchuan/`
  - `rtk pnpm exec tsc --noEmit`
  - `rtk pnpm lint`

- [ ] Step 11.6 — commit `feat(fe): qianchuan bindings list page + create dialog`

---

## Task 12 — End-to-end smoke + docs touch-up

- [ ] Step 12.1 — manual smoke (no CI gate)
  - With backend running and a valid `QIANCHUAN_APP_ID` / `QIANCHUAN_APP_SECRET` (or mock provider when env vars absent), exercise:
    1. `POST /api/v1/projects/:pid/qianchuan/bindings` with two existing secret refs → 201.
    2. `GET /api/v1/projects/:pid/qianchuan/bindings` → list contains the row.
    3. `POST /api/v1/qianchuan/bindings/:id/test` → 200.
    4. `POST /api/v1/qianchuan/bindings/:id/sync` → 204; confirm `last_synced_at` updates.
    5. `PATCH /api/v1/qianchuan/bindings/:id` `{status: "paused"}` → 200; FE list badge changes.
    6. `DELETE /api/v1/qianchuan/bindings/:id` → 204.

- [ ] Step 12.2 — update Spec 3 §13.1 with the drifts surfaced here
  - Edit `docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md`
  - Append three bullets to §14 / new §13.1:
    - "Plan 3A omits binding row's policy / strategy_id / trigger_id / tick_interval_sec; Plan 3C / 3D ALTER TABLE later."
    - "Plan 3A renames `employee_id NOT NULL` to `acting_employee_id UUID NULL` to align with AgentForge's existing acting-employee convention."
    - "Qianchuan OpenAPI uses bearer-token (`Access-Token` header), not body-HMAC; Provider implementation reflects that."
    - "Big-int Qianchuan IDs (room_id, order_id, advertiser_id) are decoded with `json.Number` and surfaced as `string` in adsplatform structs to avoid float64 precision loss."

- [ ] Step 12.3 — commit `docs(spec3): record 3A drifts + bearer-auth note`

---

## Final verification

- [ ] `rtk go test ./internal/adsplatform/... ./internal/qianchuanbinding/... ./internal/handler/...`
- [ ] `rtk go build ./...`
- [ ] `rtk pnpm test lib/stores/qianchuan-bindings-store.test.ts components/qianchuan/`
- [ ] `rtk pnpm exec tsc --noEmit`
- [ ] `rtk pnpm lint`
- [ ] Manual smoke per Task 12.1

---

## Out of scope (deferred to siblings / successors)

- OAuth-driven binding creation (button + redirect + state CSRF) → **Plan 3B**.
- Strategy YAML CRUD + dry-run + binding.strategy_id wire-up → **Plan 3C**.
- Workflow loop (`qianchuan_strategy_loop` DAG) that calls `Provider.FetchMetrics` + actions every tick → **Plan 3D**.
- Action policy gate (max_bid_change_pct etc.) wrapping the action primitives → **Plan 3E**.
- Per-binding Redis lock + token-refresh goroutine → **Plan 3D**.
- Snapshot table + chart endpoints + FE chart UI → **Plan 3D / 3F**.
- Multi-platform (Taobao / JD / TikTok Ads) — interface only, no impls.
