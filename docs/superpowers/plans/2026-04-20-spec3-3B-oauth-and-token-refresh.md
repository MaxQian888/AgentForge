# Spec 3B — Qianchuan OAuth Bind Flow + Background Token Refresh + auth_expired Handling

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 3 OAuth 绑定流程 + 后台 token 自动刷新（minute cron） + auth_expired 通知与 FE 横幅。

**Architecture:** OAuth 标准 code-flow，state 通过新表 qianchuan_oauth_states 防 CSRF；callback 用 1B secrets.Service.Create 落地两份 token；后台 goroutine 每 60s 扫即将过期的 binding 调用 RefreshToken + 1B Rotate；refresh 失败标 auth_expired + emit event + WS + FE 横幅；workflow 触发前检查 binding status。

**Tech Stack:** Go (HTTP redirect handler + background goroutine), Postgres, Next.js 16 App Router, Zustand.

**Depends on:** 3A (binding row + adsplatform.Provider OAuth methods); 1B (secrets Create/Rotate)

**Parallel with:** 3C (strategy authoring) — independent

**Unblocks:** 3D (strategy loop relies on healthy tokens); 3E (action calls fail fast on auth_expired)

---

## Coordination notes (read before starting)

- **Migration number**: spec §6.5 reserves 074 for `qianchuan_oauth_states`. 3A is expected to claim 070 (strategies) + 071 (bindings); Spec 2 may claim 069 between 1B's 067 and 3A's 070. If migration numbers shift in lockstep across specs, this plan keeps its file as the **next free slot** at the time of execution and updates the cross-references in §6 of Spec 3 — none of the SQL content depends on the number. Confirm before writing files: `ls src-go/migrations/ | tail -5`.
- **Provider call shape (3A contract)**: this plan assumes `adsplatform.Provider` exposes `OAuthAuthorizeURL(state, redirectURI string, scopes []string) string`, `OAuthExchange(ctx, code, redirectURI string) (TokenSet, error)`, and `RefreshToken(ctx, refresh string) (TokenSet, error)` per spec §8. Plan 3A delivers the Qianchuan implementation; 3B binds to the registry-resolved interface and never imports `internal/qianchuan` directly.
- **Secrets API**: spec text reads "secrets.Service.Create / Rotate"; the actual 1B method names from plan 1B are `CreateSecret(ctx, projectID, name, plaintext, description, actor) (*Record, error)` and `RotateSecret(ctx, projectID, name, plaintext, actor) error`. Use those exact names. New secret names follow the convention `qianchuan.<advertiser_id>.access_token` and `qianchuan.<advertiser_id>.refresh_token` so 1B's `{{secrets.X}}` resolver can later be referenced without case-folding.
- **advertiser_id source**: Qianchuan's OAuth code-exchange response includes `advertiser_ids` (array of granted advertiser ids). When the array has exactly one entry, use it directly; when it has multiple, the FE's bind initiate body MUST already constrain to a single advertiser via the `display_name` form (operator picks one before clicking "Bind"). The callback rejects multi-advertiser tokens with `qianchuan:advertiser_ambiguous` and asks the user to bind one at a time. Document this in the FE help text.
- **Background refresher leadership**: v1 ships single-instance only. The goroutine starts unconditionally in `cmd/server/main.go` next to `scheduler.RunLoop`; if you run two backend processes the same binding will be refreshed twice in the same minute. Spec §15 already calls this out as the OAuth refresh storm risk; for v1 we accept double-refresh as a no-op (Qianchuan's refresh endpoint is idempotent — both calls return the same new tokens, last-write-wins on Rotate). Multi-instance leader election is a successor task — leave a `// TODO(spec3-multi-instance)` comment at the goroutine bootstrap.
- **Refresh window**: spec §11 says "refresh @≤10min to expiry". The query is `WHERE status='active' AND access_expires_at < now() + interval '10 minutes'`. This must use the partial index on `qianchuan_bindings.status`; run `EXPLAIN` in tests to confirm the planner picks `qianchuan_bindings_status_idx`.
- **EventAdsPlatformAuthExpired**: this is a NEW eventbus event type. Add it to `src-go/internal/eventbus/types.go` (or the equivalent type registry) so persist + ws_fanout + the new pause subscriber all see the same payload contract. Naming follows existing convention `EventAdsPlatform*` (camel-cased event constant + underscore-cased channel string `adsplatform.auth_expired`).
- **Trigger gating**: spec §11 says auth_expired must "暂停所有以该 binding 为 trigger 的 schedule fire". Two paths exist; this plan uses the runtime check (cheaper, no DB writes, reversible by status flip): the schedule trigger router consults `qianchuan_bindings.status` for the binding referenced by the trigger before spawning the workflow. The handler-side "pause workflow_triggers row" is documented as a successor toggle (kept as a comment in code, not implemented) so 3D / 3E can opt into it later without spec drift.
- **Time mocking**: refresher tests rely on a clock interface. Reuse the existing `clock.Clock` abstraction if present; if not, add a tiny `clock.go` to `internal/qianchuan_runtime/` with `Now()`, defaulting to `time.Now`, and inject a fake in tests. Do NOT introduce a third-party clock dependency.

---

## Task 1 — Migration: `qianchuan_oauth_states` table

- [ ] Step 1.1 — write the up migration
  - File: `src-go/migrations/<NEXT>_create_qianchuan_oauth_states.up.sql`
    ```sql
    -- Spec 3 §6.5 — short-lived CSRF nonces for Qianchuan OAuth bind flow.
    -- Rows expire after 10 minutes; consumed_at marks single-use semantics.
    CREATE TABLE IF NOT EXISTS qianchuan_oauth_states (
        state_token         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        redirect_uri        TEXT NOT NULL,
        initiated_by        UUID NOT NULL,
        display_name        VARCHAR(128),
        acting_employee_id  UUID REFERENCES employees(id) ON DELETE SET NULL,
        expires_at          TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '10 minutes'),
        consumed_at         TIMESTAMPTZ,
        created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
    );
    CREATE INDEX IF NOT EXISTS idx_qoauth_active
        ON qianchuan_oauth_states (expires_at)
        WHERE consumed_at IS NULL;
    ```
  - Notes:
    - Column name diverges from spec §6.5 draft (`state VARCHAR(64)`) — 3B uses `state_token UUID` per the Plan 3B brief. The spec's intent (single-use CSRF nonce) is preserved; record this drift in §13 of the spec on next edit.
    - `acting_employee_id` is nullable + `ON DELETE SET NULL`: an employee may be archived between initiate and callback; we still want the binding to land (under no employee) rather than 500.
    - `display_name` is optional — when omitted the callback synthesizes `Qianchuan <advertiser_id>` so the binding row's NOT NULL constraint never fires.

- [ ] Step 1.2 — write the down migration
  - File: `src-go/migrations/<NEXT>_create_qianchuan_oauth_states.down.sql`
    ```sql
    DROP INDEX IF EXISTS idx_qoauth_active;
    DROP TABLE IF EXISTS qianchuan_oauth_states;
    ```

- [ ] Step 1.3 — verify
  - `rtk pnpm dev:backend:restart go-orchestrator` and confirm the up migration applies cleanly.
  - Manually `INSERT` one row in psql, then `SELECT * FROM qianchuan_oauth_states WHERE expires_at < now() + interval '11 minutes'` and confirm the partial index is used (`EXPLAIN` shows `Index Scan using idx_qoauth_active`).

---

## Task 2 — `qianchuan_oauth_states` repository

- [ ] Step 2.1 — write failing repo tests
  - File: `src-go/internal/qianchuan/oauth_state_repo_test.go`
    - `TestRepo_CreateAndLookup` — insert via `Create`, fetch via `Lookup`, assert all fields preserved.
    - `TestRepo_LookupConsumed` — insert + `MarkConsumed`, `Lookup` returns `ErrStateConsumed`.
    - `TestRepo_LookupExpired` — insert with `expires_at = now() - 1 second`, `Lookup` returns `ErrStateExpired`.
    - `TestRepo_LookupMissing` — random uuid → `ErrStateNotFound`.
    - `TestRepo_DeleteExpired` — sweeper helper: insert 3 expired + 1 fresh, `DeleteExpired(ctx, now())` returns 3 and the fresh row remains.

- [ ] Step 2.2 — implement
  - File: `src-go/internal/qianchuan/oauth_state_repo.go`
    ```go
    package qianchuan

    import (
        "context"
        "errors"
        "time"

        "github.com/google/uuid"
        "github.com/jmoiron/sqlx"
    )

    var (
        ErrStateNotFound = errors.New("oauth_state: not found")
        ErrStateExpired  = errors.New("oauth_state: expired")
        ErrStateConsumed = errors.New("oauth_state: already consumed")
    )

    type OAuthState struct {
        StateToken       uuid.UUID  `db:"state_token"`
        ProjectID        uuid.UUID  `db:"project_id"`
        RedirectURI      string     `db:"redirect_uri"`
        InitiatedBy      uuid.UUID  `db:"initiated_by"`
        DisplayName      *string    `db:"display_name"`
        ActingEmployeeID *uuid.UUID `db:"acting_employee_id"`
        ExpiresAt        time.Time  `db:"expires_at"`
        ConsumedAt       *time.Time `db:"consumed_at"`
        CreatedAt        time.Time  `db:"created_at"`
    }

    type OAuthStateRepo struct{ db *sqlx.DB }

    func NewOAuthStateRepo(db *sqlx.DB) *OAuthStateRepo { return &OAuthStateRepo{db: db} }

    func (r *OAuthStateRepo) Create(ctx context.Context, s *OAuthState) error { /* INSERT ... RETURNING state_token, expires_at, created_at */ }
    func (r *OAuthStateRepo) Lookup(ctx context.Context, token uuid.UUID) (*OAuthState, error) { /* SELECT; raise ErrStateConsumed if consumed_at IS NOT NULL; raise ErrStateExpired if expires_at < now() */ }
    func (r *OAuthStateRepo) MarkConsumed(ctx context.Context, token uuid.UUID) error { /* UPDATE ... SET consumed_at = now() WHERE consumed_at IS NULL */ }
    func (r *OAuthStateRepo) DeleteExpired(ctx context.Context, before time.Time) (int64, error) { /* DELETE WHERE expires_at < $1 — returns rows affected */ }
    ```
  - All errors map to spec error code `qianchuan:oauth_state_invalid` at the handler boundary.

- [ ] Step 2.3 — verify
  - `rtk go test ./internal/qianchuan/... -run TestRepo_` — all 5 cases green.

---

## Task 3 — OAuth bind initiate handler

- [ ] Step 3.1 — write failing handler tests
  - File: `src-go/internal/handler/qianchuan_oauth_handler_test.go`
    - `TestInitiate_Success` — POST with body `{display_name:"店铺A", acting_employee_id:<uuid>}` → 200, response has `authorize_url` (containing `state=<uuid>` and `redirect_uri=<callback>`) and `state_token`.
    - `TestInitiate_RequiresProjectAdmin` — viewer/editor → 403; admin/owner → 200. (Spec §12 RBAC matrix: admin or owner can OAuth bind.)
    - `TestInitiate_BodyValidation` — missing required fields, malformed `acting_employee_id` → 400 with structured error code.
    - `TestInitiate_PersistsStateRow` — after a successful call, the `qianchuan_oauth_states` table contains exactly one row with the returned `state_token` and `consumed_at IS NULL`.

- [ ] Step 3.2 — implement
  - File: `src-go/internal/handler/qianchuan_oauth_handler.go`
    ```go
    package handler

    type QianchuanOAuthHandler struct {
        Bindings    QianchuanBindingService     // 3A-provided
        States      *qianchuan.OAuthStateRepo
        Providers   adsplatform.Registry         // resolves "qianchuan" → Provider
        PublicBase  string                       // env BACKEND_PUBLIC_BASE_URL or fallback
    }

    type initiateReq struct {
        DisplayName      *string    `json:"display_name,omitempty"`
        ActingEmployeeID *uuid.UUID `json:"acting_employee_id,omitempty"`
    }

    type initiateResp struct {
        AuthorizeURL string    `json:"authorize_url"`
        StateToken   uuid.UUID `json:"state_token"`
    }

    // Initiate handles POST /api/v1/projects/:pid/qianchuan/oauth/bind/initiate
    func (h *QianchuanOAuthHandler) Initiate(c echo.Context) error { /* see steps below */ }
    ```
    Behavior:
    1. Resolve `project_id` from path; resolve `actor` from JWT context.
    2. Decode body via `c.Bind(&req)`; validate.
    3. `state := uuid.New()`; `redirectURI := h.PublicBase + "/api/v1/qianchuan/oauth/callback"`.
    4. `States.Create(...)` with `expires_at = nil` (DB default of `now() + 10 min` applies).
    5. `provider := h.Providers.MustGet("qianchuan")`; `authorizeURL := provider.OAuthAuthorizeURL(state.String(), redirectURI, []string{ /* default scopes from config */ })`.
    6. Return `200 initiateResp{authorizeURL, state}`.

- [ ] Step 3.3 — wire route
  - File: `src-go/internal/server/routes.go`
  - Register: `g.POST("/projects/:pid/qianchuan/oauth/bind/initiate", h.QianchuanOAuth.Initiate, appMiddleware.Require(appMiddleware.ActionQianchuanBindWrite))`
  - Add the new ActionID `ActionQianchuanBindWrite` to `middleware/rbac.go` mapped to `admin` + `owner` per spec §12.

- [ ] Step 3.4 — verify
  - `rtk go test ./internal/handler/... -run TestInitiate_` — all 4 cases green.
  - `rtk go test ./internal/middleware/... -run TestRBAC_QianchuanBind` — covers the role gating.

---

## Task 4 — OAuth callback handler

- [ ] Step 4.1 — write failing callback tests
  - File: `src-go/internal/handler/qianchuan_oauth_callback_test.go`
    - `TestCallback_HappyPath` — seed state row + mock `OAuthExchange` returning a `TokenSet` with one advertiser; GET callback with `code=abc&state=<uuid>` → 302 redirect to `/projects/<pid>/qianchuan/bindings?bind=success&advertiser=<id>`. Assert: 2 secrets created (`qianchuan.<id>.access_token`, `qianchuan.<id>.refresh_token`), 1 binding row inserted, oauth_state `consumed_at` is set.
    - `TestCallback_MissingState` — GET without `state` → 400 HTML error page (server-rendered template).
    - `TestCallback_StateNotFound` → 400 + page text "OAuth state invalid or expired".
    - `TestCallback_StateExpired` → 400 + same page.
    - `TestCallback_StateAlreadyConsumed` → 400 + page text "Bind already completed; please re-initiate".
    - `TestCallback_ExchangeFailure` — mock provider returns error → 502 page "Qianchuan exchange failed; please retry".
    - `TestCallback_AdvertiserAmbiguous` — token returns 2 advertiser ids → 400 page "Multiple advertisers granted; bind one at a time".
    - `TestCallback_Idempotency` — replaying the same `(code, state)` after success → 400 (state already consumed); the binding row is NOT duplicated.

- [ ] Step 4.2 — implement
  - File: `src-go/internal/handler/qianchuan_oauth_handler.go` (extend)
    ```go
    // Callback handles GET /api/v1/qianchuan/oauth/callback?code=...&state=...
    func (h *QianchuanOAuthHandler) Callback(c echo.Context) error { /* see steps */ }
    ```
    Steps:
    1. Parse `code`, `state` from query; if missing → render error page (Step 4.3).
    2. `stateUUID, err := uuid.Parse(state)`; on error → 400 page.
    3. `row, err := h.States.Lookup(ctx, stateUUID)`; map `ErrStateNotFound|Expired|Consumed` → 400 page.
    4. `provider := h.Providers.MustGet("qianchuan")`; `tokens, err := provider.OAuthExchange(ctx, code, row.RedirectURI)`; on error → 502 page.
    5. Extract advertiser id from `tokens` (Provider must surface the advertiser via `TokenSet.Scopes` or a dedicated field — confirm 3A's TokenSet shape; if 3A only returns access/refresh strings, call a follow-up `provider.IntrospectToken(ctx, tokens)` — defer this hook to 3A coordination if the interface is incomplete, document the drift in §13 of Spec 3).
    6. If multiple advertiser ids returned → 400 page `qianchuan:advertiser_ambiguous`.
    7. Two `secrets.Service.CreateSecret` calls under the same project:
       - `qianchuan.<advertiser_id>.access_token` = `tokens.AccessToken`, description `"Qianchuan OAuth access token (auto-managed)"`.
       - `qianchuan.<advertiser_id>.refresh_token` = `tokens.RefreshToken`, description `"Qianchuan OAuth refresh token (auto-managed)"`.
       - On `secret:already_exists` (operator re-bound after manual delete) → fall back to `RotateSecret` (covers re-bind-same-advertiser path).
    8. `h.Bindings.Create(...)` (3A's binding service): persist `display_name` (default `"Qianchuan " + advertiserID` if `row.DisplayName` is nil), `advertiser_id`, secret refs, `access_expires_at = tokens.AccessExpiresAt`, `refresh_expires_at = tokens.RefreshExpiresAt`, `status='active'`, `created_by = row.InitiatedBy`, `employee_id = row.ActingEmployeeID` (or sentinel "unassigned" if nil — 3A defines this).
    9. `h.States.MarkConsumed(ctx, stateUUID)`.
    10. Audit: `auditSvc.RecordEvent(...)` with `resource_type='qianchuan_binding'`, `action='qianchuan.binding.oauth_completed'`, payload `{advertiser_id, display_name}` — do NOT include token plaintext or secret ids.
    11. Redirect to `${PUBLIC_BASE_FE}/projects/<row.ProjectID>/qianchuan/bindings?bind=success&advertiser=<id>`.

- [ ] Step 4.3 — error HTML template
  - File: `src-go/internal/handler/templates/qianchuan_oauth_error.html`
    - Minimal hand-rolled HTML (no framework): centered card, error code, error message, and a "Back to AgentForge" link to `${PUBLIC_BASE_FE}`.
    - Embed via `//go:embed` into `qianchuan_oauth_handler.go`.
    - i18n: render in English by default; pass `?lang=zh` if Spec 1's i18n cookie can be read (best-effort, fall back to English).

- [ ] Step 4.4 — wire route + verify
  - Register `e.GET("/api/v1/qianchuan/oauth/callback", h.QianchuanOAuth.Callback)` (NO RBAC middleware — Qianchuan posts back as the user's browser; the state token is the trust anchor).
  - `rtk go test ./internal/handler/... -run TestCallback_` — all 8 cases green.

---

## Task 5 — Background token refresher: scaffold + scan loop

- [ ] Step 5.1 — write failing scan-loop tests
  - File: `src-go/internal/qianchuan/refresher_test.go`
    - `TestRefresher_PicksDueBindings` — seed 3 bindings: A active expiring in 5min, B active expiring in 30min, C status='paused' expiring in 5min. Run one tick. Assert refresher attempted refresh on A only (B too far in future, C wrong status).
    - `TestRefresher_QueryUsesIndex` — `EXPLAIN ANALYZE` the scan query; assert plan contains `qianchuan_bindings_status_idx` (skip on non-PG test backends).
    - `TestRefresher_TickerStartsAndStops` — start refresher in goroutine with mock clock; cancel context → goroutine returns within 100ms.

- [ ] Step 5.2 — implement scan loop
  - File: `src-go/internal/qianchuan/refresher.go`
    ```go
    package qianchuan

    import (
        "context"
        "time"

        log "github.com/sirupsen/logrus"
    )

    type Refresher struct {
        Bindings    QianchuanBindingRepo
        Secrets     SecretsService            // 1B
        Providers   adsplatform.Registry
        Bus         eventbus.Publisher
        Audit       AuditRecorder
        Clock       func() time.Time          // injected for tests; default time.Now
        TickEvery   time.Duration             // default 60s
        EarlyWindow time.Duration             // default 10 * time.Minute
    }

    func (r *Refresher) Run(ctx context.Context) {
        if r.TickEvery <= 0 { r.TickEvery = 60 * time.Second }
        if r.EarlyWindow <= 0 { r.EarlyWindow = 10 * time.Minute }
        if r.Clock == nil { r.Clock = time.Now }
        ticker := time.NewTicker(r.TickEvery)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                if err := r.tick(ctx); err != nil {
                    log.WithError(err).Warn("qianchuan refresher tick failed")
                }
            }
        }
    }

    func (r *Refresher) tick(ctx context.Context) error {
        due, err := r.Bindings.FindDueForRefresh(ctx, r.Clock().Add(r.EarlyWindow))
        if err != nil { return err }
        for _, b := range due {
            r.refreshOne(ctx, b)   // implemented in Task 6
        }
        return nil
    }
    ```
  - Add to `qianchuan/binding_repo.go` (3A surface) the method:
    ```go
    // FindDueForRefresh returns active bindings whose access_expires_at is
    // before the supplied threshold. Caller passes now()+earlyWindow.
    FindDueForRefresh(ctx context.Context, before time.Time) ([]Binding, error)
    ```
    SQL: `SELECT ... FROM qianchuan_bindings WHERE status='active' AND access_expires_at < $1`
    — confirms partial index is used.

- [ ] Step 5.3 — verify
  - `rtk go test ./internal/qianchuan/... -run TestRefresher_` — 3 tests green.

---

## Task 6 — Background token refresher: per-binding refresh + auth_expired transition

- [ ] Step 6.1 — write failing tests
  - File: `src-go/internal/qianchuan/refresher_test.go` (extend)
    - `TestRefreshOne_Success` — mock provider returns new TokenSet; assert: `Secrets.RotateSecret` called twice (access + refresh) with correct names, binding `access_expires_at` updated, audit row written with `action='qianchuan.token.refreshed'` (no plaintext in payload), no event emitted.
    - `TestRefreshOne_RefreshInvalid_401` — mock provider returns HTTP 401; assert: binding marked `status='auth_expired'`, `EventAdsPlatformAuthExpired` published, audit row `action='qianchuan.token.refresh_failed'` with reason.
    - `TestRefreshOne_RefreshInvalid_403` — same as 401.
    - `TestRefreshOne_TransientError_5xx` — mock returns 503 once → refresher does NOT mark auth_expired; binding remains active (next tick will retry). Spec §11 calls for exponential backoff 1m/5m/15m; v1 keeps it simpler: each tick is independently scheduled by the outer loop; if 3 consecutive ticks fail with 5xx, the refresher then marks `auth_expired`. Track the per-binding consecutive-failure counter in an in-process `sync.Map[bindingID]int`; reset on success.
    - `TestRefreshOne_NetworkTimeout` — mock provider call panics with `context.DeadlineExceeded`; refresher logs + treats as transient (counter increment).
    - `TestRefreshOne_SecretsResolveFails` — `Secrets.Resolve("...refresh_token")` returns `secret:not_found` → mark `auth_expired` (the binding is unsalvageable without the refresh secret).

- [ ] Step 6.2 — implement
  - File: `src-go/internal/qianchuan/refresher.go` (extend)
    ```go
    func (r *Refresher) refreshOne(ctx context.Context, b Binding) {
        prov := r.Providers.MustGet(b.ProviderID)
        rt, err := r.Secrets.Resolve(ctx, b.ProjectID, b.RefreshTokenSecret)
        if err != nil {
            r.markAuthExpired(ctx, b, "refresh_secret_missing")
            return
        }

        tokens, err := prov.RefreshToken(ctx, rt)
        if err != nil {
            if isAuthInvalid(err) {
                r.markAuthExpired(ctx, b, "refresh_invalid")
                return
            }
            r.bumpTransientFailure(b.ID)
            if r.transientFailures(b.ID) >= 3 {
                r.markAuthExpired(ctx, b, "transient_threshold_exceeded")
            }
            return
        }
        r.resetTransientFailure(b.ID)

        // Rotate both secrets (refresh tokens may rotate too in OAuth2-with-rotation).
        if err := r.Secrets.RotateSecret(ctx, b.ProjectID, b.AccessTokenSecret, tokens.AccessToken, systemActor); err != nil { /* log + return */ }
        if tokens.RefreshToken != "" && tokens.RefreshToken != rt {
            if err := r.Secrets.RotateSecret(ctx, b.ProjectID, b.RefreshTokenSecret, tokens.RefreshToken, systemActor); err != nil { /* log + return */ }
        }
        if err := r.Bindings.UpdateExpiry(ctx, b.ID, tokens.AccessExpiresAt, tokens.RefreshExpiresAt); err != nil { /* log */ }

        r.Audit.Record(ctx, b.ProjectID, "qianchuan.token.refreshed", map[string]any{
            "binding_id":         b.ID,
            "advertiser_id":      b.AdvertiserID,
            "access_expires_at":  tokens.AccessExpiresAt,
            "refresh_rotated":    tokens.RefreshToken != "" && tokens.RefreshToken != rt,
        })
    }

    func (r *Refresher) markAuthExpired(ctx context.Context, b Binding, reason string) {
        if err := r.Bindings.MarkAuthExpired(ctx, b.ID, reason); err != nil { /* log */ }
        r.Bus.Publish(ctx, eventbus.EventAdsPlatformAuthExpired, AuthExpiredPayload{
            BindingID:    b.ID,
            ProjectID:    b.ProjectID,
            EmployeeID:   b.EmployeeID,
            ProviderID:   b.ProviderID,
            AdvertiserID: b.AdvertiserID,
            Reason:       reason,
            DetectedAt:   r.Clock(),
        })
        r.Audit.Record(ctx, b.ProjectID, "qianchuan.token.refresh_failed", map[string]any{
            "binding_id": b.ID, "reason": reason,
        })
    }

    // isAuthInvalid sniffs the error chain for a 401/403 / explicit refresh_invalid
    // surfaced by the Qianchuan adapter; concrete shape lives in 3A's adsplatform errors.
    func isAuthInvalid(err error) bool { /* errors.Is(err, adsplatform.ErrRefreshInvalid) || HTTP 401/403 wrappers */ }
    ```
    Add the binding repo methods (3A coordination — if not already in 3A, add here and let 3A absorb via merge):
    ```go
    UpdateExpiry(ctx context.Context, id uuid.UUID, accessExp, refreshExp time.Time) error
    MarkAuthExpired(ctx context.Context, id uuid.UUID, reason string) error
    ```

- [ ] Step 6.3 — verify
  - `rtk go test ./internal/qianchuan/... -run TestRefreshOne_` — 6 tests green.
  - Manual log review: tail backend logs and confirm refresher logs are structured JSON with `binding_id` field, never `access_token` / `refresh_token`.

---

## Task 7 — `EventAdsPlatformAuthExpired` event + subscriber

- [ ] Step 7.1 — register event type
  - File: `src-go/internal/eventbus/types.go` (or whatever the central event-name list is)
  - Add constant: `EventAdsPlatformAuthExpired = "adsplatform.auth_expired"`.
  - File: `src-go/internal/qianchuan/events.go` (new)
    ```go
    package qianchuan

    type AuthExpiredPayload struct {
        BindingID    uuid.UUID `json:"binding_id"`
        ProjectID    uuid.UUID `json:"project_id"`
        EmployeeID   uuid.UUID `json:"employee_id"`
        ProviderID   string    `json:"provider_id"`
        AdvertiserID string    `json:"advertiser_id"`
        Reason       string    `json:"reason"`
        DetectedAt   time.Time `json:"detected_at"`
    }
    ```

- [ ] Step 7.2 — write failing subscriber tests
  - File: `src-go/internal/qianchuan/auth_expired_subscriber_test.go`
    - `TestSubscriber_WSBroadcast` — publish event → ws_fanout receives it on channel `project:<pid>` (and/or `employee:<eid>`); payload preserved.
    - `TestSubscriber_AuditTrail` — subscriber writes a row in `project_audit_events` with action `qianchuan.binding.auth_expired`.
    - `TestSubscriber_FeishuAlert_WhenChannelConfigured` — project has a `system_notification_chat_id` configured (1D-style); subscriber posts a Feishu card titled `Binding "<display_name>" auth expired — please re-bind`. If not configured, subscriber emits no IM.
    - `TestSubscriber_FeishuAlert_FailsClosed` — IM Bridge POST returns 500 → subscriber logs error, does NOT retry from this path (the FE banner is the canonical UX; IM is best-effort).

- [ ] Step 7.3 — implement subscriber
  - File: `src-go/internal/qianchuan/auth_expired_subscriber.go`
    - Subscribes to `EventAdsPlatformAuthExpired` via `eventbus.Subscribe`.
    - Three side effects (in this order, all best-effort): WS fanout (already wired by `mods/ws_fanout.go` if the event is in the registry — confirm), audit event, optional Feishu alert via 1D's outbound channel resolver if `project.settings.system_notification_chat_id` is set.
    - Wire in `cmd/server/main.go` next to other subscribers.

- [ ] Step 7.4 — runtime gate on schedule trigger
  - File: `src-go/internal/trigger/schedule_trigger.go` (or the equivalent entry-point used by Spec 1's schedule trigger)
  - Before spawning a workflow execution for a trigger whose `target_workflow_id` is the canonical `qianchuan_strategy_loop`, look up the binding referenced by the trigger's input mapping (`binding_id` is in `trigger.config`). If `binding.status != 'active'`, skip the spawn and emit a debug log. Do NOT mark the trigger paused (3D / 3E own that flow).
  - Add a unit test `TestScheduleTrigger_SkipsWhenAuthExpired` that asserts the spawn is short-circuited.

- [ ] Step 7.5 — verify
  - `rtk go test ./internal/qianchuan/... -run TestSubscriber_` — 4 tests green.
  - `rtk go test ./internal/trigger/... -run TestScheduleTrigger_SkipsWhenAuthExpired` — green.

---

## Task 8 — Bootstrap refresher in `cmd/server/main.go`

- [ ] Step 8.1 — wire refresher
  - File: `src-go/cmd/server/main.go`
  - After the existing `go scheduler.RunLoop(...)` line (~279):
    ```go
    qcRefresherCtx, qcRefresherCancel := context.WithCancel(context.Background())
    defer qcRefresherCancel()
    qcRefresher := &qianchuan.Refresher{
        Bindings:  qcBindingRepo,            // 3A
        Secrets:   secretsSvc,                // 1B
        Providers: adsplatformRegistry,       // 3A
        Bus:       eventbusPublisher,
        Audit:     routeServices.AuditSink,   // sanitized recorder
        TickEvery: 60 * time.Second,
        EarlyWindow: 10 * time.Minute,
    }
    // TODO(spec3-multi-instance): leader-elect this loop before scaling backend horizontally.
    go qcRefresher.Run(qcRefresherCtx)
    ```
  - In the SIGTERM handler (`<-quit` block) call `qcRefresherCancel()` before `bridgeHealthCancel()`.
  - Also wire the auth_expired subscriber: `qianchuan.NewAuthExpiredSubscriber(deps...).Start(ctx)`.

- [ ] Step 8.2 — wire OAuth handler
  - In the route-builder block: instantiate `QianchuanOAuthHandler` and pass to `routes.go`.
  - `PublicBase` resolution: env `BACKEND_PUBLIC_BASE_URL` if set, else fall back to `cfg.PublicBaseURL`, else use `http://localhost:7777` (and log a warning that OAuth callbacks won't work over the public internet without a configured base).

- [ ] Step 8.3 — verify
  - `rtk pnpm dev:backend:verify` — startup log shows `qianchuan refresher started (tick=60s, earlyWindow=10m)`.
  - Send `kill -SIGTERM` and confirm log line `qianchuan refresher stopped` appears within 1 second.

---

## Task 9 — Integration test: end-to-end OAuth + refresh

- [ ] Step 9.1 — fixture
  - File: `src-go/internal/qianchuan/integration_test.go` (build tag `//go:build integration`)
  - Spin up: real Postgres (test container), real 1B `secrets.Service` with a test key, in-process mock Qianchuan OAuth server (`httptest.Server`) that:
    - `/oauth2/authorize` returns 302 to the registered `redirect_uri` with `?code=fake&state=<echoed>`.
    - `/oauth2/access_token` returns `{access_token:"AT1", refresh_token:"RT1", expires_in:7200, advertiser_ids:["AD1"]}`.
    - `/oauth2/refresh_token` returns `{access_token:"AT2", refresh_token:"RT2", expires_in:7200}`.

- [ ] Step 9.2 — `TestIntegration_FullOAuthBindAndRefresh`
  - Steps:
    1. POST to initiate → capture `state_token`.
    2. Simulate browser GET to `/api/v1/qianchuan/oauth/callback?code=fake&state=<state>` (use `httptest.ResponseRecorder`).
    3. Assert 302 redirect to `/projects/.../qianchuan/bindings?bind=success&advertiser=AD1`.
    4. Query DB: 1 binding row (status='active', advertiser_id='AD1'), 2 secret rows; `qianchuan_oauth_states.consumed_at` is set.
    5. Time-warp the binding's `access_expires_at` to `now() + 5min` via direct UPDATE.
    6. Run `refresher.tick(ctx)` synchronously.
    7. Assert: `secrets.Resolve(...access_token)` now returns `AT2`; `secrets.Resolve(...refresh_token)` returns `RT2`; binding's `access_expires_at` is ~`now()+2h`.
    8. Mock Qianchuan refresh endpoint to return 401; run another tick.
    9. Assert: binding `status='auth_expired'`; one `EventAdsPlatformAuthExpired` event published; audit row recorded.

- [ ] Step 9.3 — verify
  - `rtk go test -tags=integration ./internal/qianchuan/... -run TestIntegration_FullOAuthBindAndRefresh` — green.

---

## Task 10 — FE: bindings page OAuth button + auth_expired banner

- [ ] Step 10.1 — write failing FE tests
  - File: `app/(dashboard)/projects/[id]/qianchuan/bindings/__tests__/page.test.tsx` (extends 3A's scaffolded page)
    - `renders bind via OAuth button` — admin role → button visible; viewer → disabled with tooltip.
    - `clicking bind calls initiate then redirects` — mock fetch resolves `{authorize_url:"https://oauth.../authorize?...", state_token:"abc"}`; assert `window.location.assign` called with that URL.
    - `displays auth_expired banner` — render with one binding `status='auth_expired'`; assert red banner present, "Re-bind" button present.
    - `re-bind reuses display_name + advertiser_id` — clicking "Re-bind" POSTs to initiate with `display_name = <existing binding.display_name>` and stores the binding id in session-storage so the callback can show a success toast scoped to the right row.
    - `success toast on ?bind=success` — render page with `useSearchParams` returning `bind=success&advertiser=AD1`; assert sonner `toast.success` invoked with localized success copy.
    - `failure toast on ?bind=error` — symmetric; renders error toast with the error code echoed in URL.

- [ ] Step 10.2 — implement
  - File: `app/(dashboard)/projects/[id]/qianchuan/bindings/page.tsx`
  - Add a `<BindOAuthButton>` component:
    ```tsx
    function BindOAuthButton({ projectId, employees, onRebind }: Props) {
      const [open, setOpen] = useState(false);
      const [form, setForm] = useState({ displayName: "", actingEmployeeId: "" });
      const initiate = async () => {
        const r = await fetch(`/api/v1/projects/${projectId}/qianchuan/oauth/bind/initiate`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            display_name: form.displayName || undefined,
            acting_employee_id: form.actingEmployeeId || undefined,
          }),
        }).then((r) => r.json());
        window.location.assign(r.authorize_url);
      };
      // ... dialog UI
    }
    ```
  - Add `<AuthExpiredBanner>`:
    ```tsx
    function AuthExpiredBanner({ binding, onRebind }) {
      return (
        <div className="rounded-md border border-destructive bg-destructive/10 p-4 flex items-center justify-between">
          <div>
            <p className="font-medium text-destructive">绑定 "{binding.display_name}" 授权已过期</p>
            <p className="text-sm text-muted-foreground">请重新绑定以恢复策略循环。原因：{binding.status_reason}</p>
          </div>
          <Button variant="destructive" onClick={() => onRebind(binding)}>重新绑定</Button>
        </div>
      );
    }
    ```
  - Listen for the `bind` query param on mount (`useSearchParams`) → fire the appropriate sonner toast; `router.replace` to strip the query string after handling.

- [ ] Step 10.3 — i18n
  - Add the new strings to `lib/i18n/messages/{en,zh}.json` under `qianchuan.bindings.*`.
  - Run `rtk pnpm i18n:audit` (or invoke the `i18n-fill` skill) to confirm parity.

- [ ] Step 10.4 — verify
  - `rtk vitest run app/\(dashboard\)/projects/\[id\]/qianchuan/bindings/__tests__/page.test.tsx` — all tests green.
  - `rtk pnpm lint` — no new violations.

---

## Task 11 — WS reactivity: FE picks up auth_expired without refresh

- [ ] Step 11.1 — extend the bindings store
  - File: `lib/stores/qianchuan-bindings-store.ts` (3A surface)
  - Subscribe to WS event `adsplatform.auth_expired` filtered by current `projectId`. On message, mutate the matching binding row in the store: `status='auth_expired'`, `status_reason=<payload.reason>`.

- [ ] Step 11.2 — write failing test
  - File: `lib/stores/__tests__/qianchuan-bindings-store.test.ts`
    - Seed store with one binding `status='active'`; emit a synthetic WS event matching the binding id; assert store updates within one microtask.

- [ ] Step 11.3 — verify
  - `rtk vitest run lib/stores/__tests__/qianchuan-bindings-store.test.ts` — green.
  - Manual check: kill access_token validity in PG (set `access_expires_at = now() - interval '1 minute'`, `status='auth_expired'`), open the bindings page, confirm banner appears within 1 tick of the WS event without page reload.

---

## Task 12 — Smoke test + docs

- [ ] Step 12.1 — IM Bridge smoke fixture
  - File: `src-im-bridge/scripts/smoke/qianchuan-oauth-expired.json`
  - Scenario: posts a fake `EventAdsPlatformAuthExpired` to the backend bus → asserts a Feishu card lands in the configured system-notification chat with the expected title and the "Re-bind" URL action that deep-links to the bindings page.

- [ ] Step 12.2 — operator runbook entry
  - File: `docs/operations/qianchuan-token-refresh.md` (new, short — < 60 lines)
  - Sections: "What happens when a token expires", "How to re-bind from FE", "How to inspect refresher logs", "Multi-instance caveat (v1 limitation)".
  - Cross-link from Spec 3 §11 and §15.

- [ ] Step 12.3 — verify
  - Run smoke: `rtk pnpm --filter src-im-bridge smoke:qianchuan-oauth-expired` (or whatever the existing smoke runner expects).
  - Final cross-cutting check: `rtk pnpm test` and `rtk go test ./...` green; `rtk pnpm lint` clean; `rtk pnpm exec tsc --noEmit` clean.

---

## Done criteria

- A user with `admin` on a project can click "Bind via OAuth", complete the Qianchuan consent in another tab, and land back on the bindings page with a green "bind succeeded" toast and the new row visible — all within one minute end-to-end.
- A binding whose access_token is < 10 minutes from expiry is silently refreshed by the next 60s tick; FE state is unchanged; audit log shows one `qianchuan.token.refreshed` row per refresh.
- A binding whose refresh fails with 401/403 immediately transitions to `status='auth_expired'`, a WS event lands on the FE within one tick, the bindings page renders a red banner, the schedule trigger short-circuits future ticks, and (if configured) a Feishu alert lands in the system-notification chat.
- All tests in this plan green; no plaintext token ever appears in logs, audit payloads, dataStore, or WS events.
