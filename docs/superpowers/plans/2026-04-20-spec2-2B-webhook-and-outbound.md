# Spec 2B — VCS Webhook Handler + Dedup + Outbound Dispatcher

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 2 §5 S2-A 入站 webhook + S2-B 出站 dispatcher + §6.1 vcs_webhook_events + §6.2/6.3 reviews/findings 扩展 + §9 Trace A 完整。

**Architecture:** Webhook handler 验 HMAC（secret from 1B 密钥库）+ 去重（UNIQUE event_id）+ 路由到 ReviewService.Trigger（沿用现有 pipeline，仅传新 replyTarget=vcs_pr_thread）；vcs_outbound_dispatcher 订阅 EventReviewCompleted，调 vcs.Provider 发 summary + per-line inline comments，3 次指数退避失败 emit EventVCSDeliveryFailed。

**Tech Stack:** Go (Echo + raw-body middleware + crypto/hmac + crypto/sha256), Postgres, eventbus.

**Depends on:** Spec 2A (vcs.Provider interface + integrations + GitHub impl + secrets refs); Spec 1B (secrets Resolve); Spec 1A (system_metadata column for reply_target storage)

**Parallel with:** 2C (diff-of-diff) starts after this plan's webhook_router scaffold lands

**Unblocks:** 2C (extends webhook_router for push events), 2D (consumes EventReviewCompleted alongside this dispatcher), 2E (uses inline_comment_id for fix-PR linking)

---

## Coordination notes (read before starting)

- **`vcs.Provider` is owned by Plan 2A.** This plan calls `Provider.PostSummaryComment / EditSummaryComment / PostReviewComments / EditReviewComment` — those signatures must already be in `src-go/internal/vcs/provider.go` (per Spec §8). If 2A hasn't merged when starting, stub the interface with the same names/args and coordinate before T7.
- **`vcs_integrations` table is owned by Plan 2A.** This plan adds a new migration that ONLY creates `vcs_webhook_events` + ALTERs `reviews` / `review_findings`. Do NOT recreate `vcs_integrations`.
- **`ReviewService.Trigger` and `ReviewService.Complete` are NOT modified.** This plan injects the new `replyTarget` shape into `model.TriggerReviewRequest` (extension) and the new `IntegrationID / HeadSHA / BaseSHA` fields onto the review row (extension via 6.2 columns) — but the function bodies stay intact. Only callers changes are within the new webhook handler.
- **`webhook_router.RouteEvent` is the seam for Plan 2C.** This plan delivers a router that handles `pull_request{opened|reopened|ready_for_review}` and stubs `push / pull_request{synchronize}` to a `routePush` method that returns `nil, ErrPushHandlerNotImplemented`. 2C replaces that body; the seam stays.
- **`EventReviewCompleted` already exists** (`src-go/internal/eventbus/types.go:23`) — this plan adds a subscriber, not a new event type. The `EventVCSDeliveryFailed` and `EventVCSAuthExpired` types ARE new.
- **Secrets resolver from 1B** has signature `Resolve(ctx, projectID uuid.UUID, name string) (string, error)`. Webhook handler resolves `integration.WebhookSecretRef` → plaintext, runs HMAC compare, then discards.
- **HMAC raw-body capture**: Echo's default `c.Bind()` consumes the body; the new middleware must `io.ReadAll(c.Request().Body)` once, replace `c.Request().Body` with a fresh `io.NopCloser(bytes.NewReader(raw))` so downstream handlers can re-read, and stash the raw bytes via `c.Set("raw_body", raw)`.
- **Migration number**: latest existing migration is `066_workflow_run_parent_link_parent_kind`. Plan 2A is expected to introduce `067_vcs_integrations` (per its own scope). This plan uses **`068`** for `vcs_webhook_events_and_review_extensions`. If 2A lands earlier numbers, bump accordingly — confirm with `ls src-go/migrations/` before writing.
- **Old code cleanup is NOT in this plan.** Spec 2 §12 deletes `RouteFixRequest` + `EventReviewFixRequested` — those land in Plan 2D (`code_fixer DAG`) which is the PR replacing the dead path. Do not delete here.

---

## Task 1 — Migration: `vcs_webhook_events` + `reviews` + `review_findings` extensions

- [x] Step 1.1 — confirm migration number is free
  - `rtk ls src-go/migrations/` — pick the next free integer after Plan 2A's `vcs_integrations` migration. Default assumption in this plan: `068_vcs_webhook_events_and_review_extensions`.

- [x] Step 1.2 — write up migration
  - File: `src-go/migrations/068_vcs_webhook_events_and_review_extensions.up.sql` (new)
    ```sql
    CREATE TABLE IF NOT EXISTS vcs_webhook_events (
      id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
      integration_id uuid NOT NULL REFERENCES vcs_integrations(id) ON DELETE CASCADE,
      event_id varchar(128) NOT NULL,
      event_type varchar(32) NOT NULL,
      payload_hash bytea NOT NULL,
      received_at timestamptz NOT NULL DEFAULT now(),
      processed_at timestamptz,
      processing_error text,
      UNIQUE (integration_id, event_id)
    );
    CREATE INDEX idx_vcs_webhook_events_received_at ON vcs_webhook_events(received_at DESC);

    ALTER TABLE reviews
      ADD COLUMN IF NOT EXISTS integration_id uuid REFERENCES vcs_integrations(id) ON DELETE SET NULL,
      ADD COLUMN IF NOT EXISTS head_sha varchar(40),
      ADD COLUMN IF NOT EXISTS base_sha varchar(40),
      ADD COLUMN IF NOT EXISTS last_reviewed_sha varchar(40),
      ADD COLUMN IF NOT EXISTS summary_comment_id varchar(64),
      ADD COLUMN IF NOT EXISTS automation_decision varchar(16) NOT NULL DEFAULT 'manual_only';

    ALTER TABLE review_findings
      ADD COLUMN IF NOT EXISTS suggested_patch text,
      ADD COLUMN IF NOT EXISTS decision varchar(16) NOT NULL DEFAULT 'pending',
      ADD COLUMN IF NOT EXISTS decided_at timestamptz,
      ADD COLUMN IF NOT EXISTS decided_by uuid,
      ADD COLUMN IF NOT EXISTS inline_comment_id varchar(64),
      ADD COLUMN IF NOT EXISTS active_fix_run_id uuid;
    -- active_fix_run_id FK is added later by Plan 2D (which creates fix_runs).
    ```
  - File: `src-go/migrations/068_vcs_webhook_events_and_review_extensions.down.sql` (new)
    ```sql
    ALTER TABLE review_findings
      DROP COLUMN IF EXISTS active_fix_run_id,
      DROP COLUMN IF EXISTS inline_comment_id,
      DROP COLUMN IF EXISTS decided_by,
      DROP COLUMN IF EXISTS decided_at,
      DROP COLUMN IF EXISTS decision,
      DROP COLUMN IF EXISTS suggested_patch;

    ALTER TABLE reviews
      DROP COLUMN IF EXISTS automation_decision,
      DROP COLUMN IF EXISTS summary_comment_id,
      DROP COLUMN IF EXISTS last_reviewed_sha,
      DROP COLUMN IF EXISTS base_sha,
      DROP COLUMN IF EXISTS head_sha,
      DROP COLUMN IF EXISTS integration_id;

    DROP INDEX IF EXISTS idx_vcs_webhook_events_received_at;
    DROP TABLE IF EXISTS vcs_webhook_events;
    ```

- [x] Step 1.3 — extend models
  - In `src-go/internal/model/review.go` add to the `Review` struct:
    ```go
    IntegrationID      *uuid.UUID `db:"integration_id" json:"integrationId,omitempty"`
    HeadSHA            string     `db:"head_sha" json:"headSha,omitempty"`
    BaseSHA            string     `db:"base_sha" json:"baseSha,omitempty"`
    LastReviewedSHA    string     `db:"last_reviewed_sha" json:"lastReviewedSha,omitempty"`
    SummaryCommentID   string     `db:"summary_comment_id" json:"summaryCommentId,omitempty"`
    AutomationDecision string     `db:"automation_decision" json:"automationDecision"`
    ```
  - In `src-go/internal/model/review_finding.go` add to the `ReviewFinding` struct:
    ```go
    SuggestedPatch    string     `db:"suggested_patch" json:"suggestedPatch,omitempty"`
    Decision          string     `db:"decision" json:"decision"`
    DecidedAt         *time.Time `db:"decided_at" json:"decidedAt,omitempty"`
    DecidedBy         *uuid.UUID `db:"decided_by" json:"decidedBy,omitempty"`
    InlineCommentID   string     `db:"inline_comment_id" json:"inlineCommentId,omitempty"`
    ActiveFixRunID    *uuid.UUID `db:"active_fix_run_id" json:"activeFixRunId,omitempty"`
    ```
  - File: new `src-go/internal/model/vcs_webhook_event.go`
    ```go
    package model

    import (
        "time"

        "github.com/google/uuid"
    )

    type VCSWebhookEvent struct {
        ID              uuid.UUID  `db:"id" json:"id"`
        IntegrationID   uuid.UUID  `db:"integration_id" json:"integrationId"`
        EventID         string     `db:"event_id" json:"eventId"`
        EventType       string     `db:"event_type" json:"eventType"`
        PayloadHash     []byte     `db:"payload_hash" json:"-"`
        ReceivedAt      time.Time  `db:"received_at" json:"receivedAt"`
        ProcessedAt     *time.Time `db:"processed_at" json:"processedAt,omitempty"`
        ProcessingError string     `db:"processing_error" json:"processingError,omitempty"`
    }
    ```

- [x] Step 1.4 — verify
  - `rtk go test ./internal/model/...`
  - Apply migrations against a scratch PG (manual): `rtk go run ./cmd/server` once with `MIGRATE_ON_BOOT=true`, then `psql -c '\d vcs_webhook_events'` shows the table; `\d reviews` shows new columns; rollback `068.down.sql` reverses cleanly.

---

## Task 2 — Repository: `VCSWebhookEventsRepo`

- [x] Step 2.1 — failing test: insert + dedup via UNIQUE
  - File: `src-go/internal/repository/vcs_webhook_events_repo_test.go` (new)
    ```go
    package repository_test

    import (
        "context"
        "errors"
        "testing"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/repository"
        "github.com/react-go-quick-starter/server/internal/repository/repotest"
    )

    func TestVCSWebhookEventsRepo_InsertDedup(t *testing.T) {
        db := repotest.OpenTestDB(t) // existing test helper; spins migrated PG
        repo := repository.NewVCSWebhookEventsRepo(db)
        integrationID := repotest.SeedVCSIntegration(t, db) // helper provided by Plan 2A

        ctx := context.Background()
        e := &model.VCSWebhookEvent{
            ID: uuid.New(), IntegrationID: integrationID,
            EventID: "delivery-1", EventType: "pull_request",
            PayloadHash: []byte{0x01, 0x02},
        }
        if err := repo.Insert(ctx, e); err != nil { t.Fatalf("first insert: %v", err) }

        dup := &model.VCSWebhookEvent{
            ID: uuid.New(), IntegrationID: integrationID,
            EventID: "delivery-1", EventType: "pull_request",
            PayloadHash: []byte{0x03},
        }
        err := repo.Insert(ctx, dup)
        if !errors.Is(err, repository.ErrVCSWebhookEventDuplicate) {
            t.Fatalf("expected ErrVCSWebhookEventDuplicate, got %v", err)
        }
    }
    ```

- [x] Step 2.2 — implement repo
  - File: `src-go/internal/repository/vcs_webhook_events_repo.go` (new)
    ```go
    package repository

    import (
        "context"
        "errors"
        "time"

        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgconn"
        "github.com/jackc/pgx/v5/pgxpool"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    var ErrVCSWebhookEventDuplicate = errors.New("vcs_webhook_event: duplicate (integration_id, event_id)")

    type VCSWebhookEventsRepo struct{ pool *pgxpool.Pool }

    func NewVCSWebhookEventsRepo(pool *pgxpool.Pool) *VCSWebhookEventsRepo {
        return &VCSWebhookEventsRepo{pool: pool}
    }

    func (r *VCSWebhookEventsRepo) Insert(ctx context.Context, e *model.VCSWebhookEvent) error {
        const q = `INSERT INTO vcs_webhook_events
            (id, integration_id, event_id, event_type, payload_hash, received_at)
            VALUES ($1, $2, $3, $4, $5, $6)`
        _, err := r.pool.Exec(ctx, q, e.ID, e.IntegrationID, e.EventID, e.EventType, e.PayloadHash, e.ReceivedAt)
        if err != nil {
            var pgErr *pgconn.PgError
            if errors.As(err, &pgErr) && pgErr.Code == "23505" {
                return ErrVCSWebhookEventDuplicate
            }
            return err
        }
        return nil
    }

    func (r *VCSWebhookEventsRepo) MarkProcessed(ctx context.Context, id [16]byte, procErr string) error {
        const q = `UPDATE vcs_webhook_events SET processed_at = $1, processing_error = NULLIF($2, '') WHERE id = $3`
        _, err := r.pool.Exec(ctx, q, time.Now().UTC(), procErr, id)
        if err != nil && errors.Is(err, pgx.ErrNoRows) {
            return nil
        }
        return err
    }
    ```

- [x] Step 2.3 — verify
  - `rtk go test ./internal/repository/... -run VCSWebhookEvents` — both insert paths green.

---

## Task 3 — Raw-body middleware

- [x] Step 3.1 — failing test: middleware preserves body for downstream + stashes raw bytes
  - File: `src-go/internal/middleware/raw_body_test.go` (new)
    ```go
    package middleware_test

    import (
        "bytes"
        "io"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"

        "github.com/labstack/echo/v4"
        "github.com/react-go-quick-starter/server/internal/middleware"
    )

    func TestCaptureRawBody_StashesAndRestores(t *testing.T) {
        e := echo.New()
        body := strings.NewReader(`{"hello":"world"}`)
        req := httptest.NewRequest(http.MethodPost, "/", body)
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)

        h := middleware.CaptureRawBody()(func(c echo.Context) error {
            raw, ok := c.Get(middleware.RawBodyKey).([]byte)
            if !ok || string(raw) != `{"hello":"world"}` {
                return echo.NewHTTPError(500, "raw body not stashed")
            }
            // downstream handler can also re-read
            b, _ := io.ReadAll(c.Request().Body)
            if !bytes.Equal(b, []byte(`{"hello":"world"}`)) {
                return echo.NewHTTPError(500, "request body not restored")
            }
            return c.NoContent(204)
        })

        if err := h(c); err != nil { t.Fatal(err) }
        if rec.Code != 204 { t.Fatalf("status %d", rec.Code) }
    }
    ```

- [x] Step 3.2 — implement
  - File: `src-go/internal/middleware/raw_body.go` (new)
    ```go
    package middleware

    import (
        "bytes"
        "io"

        "github.com/labstack/echo/v4"
    )

    const RawBodyKey = "raw_body"

    // CaptureRawBody reads the entire request body once, stashes it in c under
    // RawBodyKey, and replaces c.Request().Body with a fresh reader so downstream
    // handlers (Bind, validators) work normally. Required for HMAC-verified
    // webhooks where the signed payload is the EXACT bytes on the wire.
    func CaptureRawBody() echo.MiddlewareFunc {
        return func(next echo.HandlerFunc) echo.HandlerFunc {
            return func(c echo.Context) error {
                raw, err := io.ReadAll(c.Request().Body)
                if err != nil {
                    return echo.NewHTTPError(400, "read body: "+err.Error())
                }
                _ = c.Request().Body.Close()
                c.Request().Body = io.NopCloser(bytes.NewReader(raw))
                c.Set(RawBodyKey, raw)
                return next(c)
            }
        }
    }
    ```

- [x] Step 3.3 — verify
  - `rtk go test ./internal/middleware/... -run RawBody`

---

## Task 4 — Webhook router (event-type dispatch seam)

- [x] Step 4.1 — failing tests: route by event type
  - File: `src-go/internal/service/vcs_webhook_router_test.go` (new)
    ```go
    package service_test

    import (
        "context"
        "errors"
        "testing"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/service"
    )

    type fakeReviewTrigger struct{ called bool; lastReq *model.TriggerReviewRequest }

    func (f *fakeReviewTrigger) Trigger(_ context.Context, r *model.TriggerReviewRequest) (*model.Review, error) {
        f.called = true; f.lastReq = r
        return &model.Review{ID: uuid.New()}, nil
    }

    func TestRouter_PullRequestOpened_TriggersReview(t *testing.T) {
        rt := &fakeReviewTrigger{}
        r := service.NewVCSWebhookRouter(rt)
        integ := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Owner: "o", Repo: "r"}

        body := []byte(`{"action":"opened","pull_request":{"number":42,
          "head":{"sha":"abc"},"base":{"sha":"def"},
          "html_url":"https://github.com/o/r/pull/42"}}`)

        if err := r.RouteEvent(context.Background(), integ, "pull_request", "delivery-1", body); err != nil {
            t.Fatalf("route: %v", err)
        }
        if !rt.called { t.Fatal("ReviewService.Trigger not called") }
        if rt.lastReq.PRNumber != 42 || rt.lastReq.HeadSHA != "abc" {
            t.Fatalf("review req fields wrong: %+v", rt.lastReq)
        }
    }

    func TestRouter_PullRequestSynchronize_DelegatesToPushHandler(t *testing.T) {
        r := service.NewVCSWebhookRouter(&fakeReviewTrigger{})
        integ := &model.VCSIntegration{ID: uuid.New()}
        body := []byte(`{"action":"synchronize","pull_request":{"number":42}}`)
        err := r.RouteEvent(context.Background(), integ, "pull_request", "d", body)
        if !errors.Is(err, service.ErrPushHandlerNotImplemented) {
            t.Fatalf("expected ErrPushHandlerNotImplemented (Plan 2C seam), got %v", err)
        }
    }

    func TestRouter_UnknownEvent_NoOp(t *testing.T) {
        r := service.NewVCSWebhookRouter(&fakeReviewTrigger{})
        if err := r.RouteEvent(context.Background(), &model.VCSIntegration{}, "ping", "d", []byte(`{}`)); err != nil {
            t.Fatalf("ping should be no-op, got %v", err)
        }
    }
    ```

- [x] Step 4.2 — implement router
  - File: `src-go/internal/service/vcs_webhook_router.go` (new)
    ```go
    package service

    import (
        "context"
        "encoding/json"
        "errors"
        "strings"

        log "github.com/sirupsen/logrus"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    var ErrPushHandlerNotImplemented = errors.New("vcs_webhook_router: push/synchronize handler is owned by Plan 2C")

    // ReviewTrigger is the narrow surface of ReviewService this router needs.
    // Real impl: *service.ReviewService satisfies it (Trigger method already exists).
    type ReviewTrigger interface {
        Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error)
    }

    type VCSWebhookRouter struct{ reviews ReviewTrigger }

    func NewVCSWebhookRouter(rt ReviewTrigger) *VCSWebhookRouter {
        return &VCSWebhookRouter{reviews: rt}
    }

    type prPayload struct {
        Action      string `json:"action"`
        PullRequest struct {
            Number  int    `json:"number"`
            HTMLURL string `json:"html_url"`
            Head    struct{ SHA string `json:"sha"` } `json:"head"`
            Base    struct{ SHA string `json:"sha"` } `json:"base"`
        } `json:"pull_request"`
    }

    func (r *VCSWebhookRouter) RouteEvent(ctx context.Context, integ *model.VCSIntegration,
        eventType, deliveryID string, body []byte) error {

        switch eventType {
        case "pull_request":
            var p prPayload
            if err := json.Unmarshal(body, &p); err != nil {
                return err
            }
            switch p.Action {
            case "opened", "reopened", "ready_for_review":
                return r.triggerReview(ctx, integ, p)
            case "synchronize":
                // Plan 2C plugs in here.
                return ErrPushHandlerNotImplemented
            default:
                log.WithFields(log.Fields{
                    "integration": integ.ID, "delivery": deliveryID, "action": p.Action,
                }).Debug("vcs_webhook_router: ignoring pull_request action")
                return nil
            }
        case "push":
            // Plan 2C plugs in here.
            return ErrPushHandlerNotImplemented
        default:
            log.WithFields(log.Fields{
                "integration": integ.ID, "delivery": deliveryID, "event": eventType,
            }).Debug("vcs_webhook_router: ignoring event type")
            return nil
        }
    }

    func (r *VCSWebhookRouter) triggerReview(ctx context.Context, integ *model.VCSIntegration, p prPayload) error {
        replyTarget := map[string]any{
            "kind":           "vcs_pr_thread",
            "integration_id": integ.ID.String(),
            "pr_number":      p.PullRequest.Number,
            "host":           integ.Host,
            "owner":          integ.Owner,
            "repo":           integ.Repo,
        }
        req := &model.TriggerReviewRequest{
            Trigger:       "vcs_webhook",
            PRURL:         strings.TrimSpace(p.PullRequest.HTMLURL),
            PRNumber:      p.PullRequest.Number,
            ProjectID:     integ.ProjectID.String(),
            IntegrationID: integ.ID.String(),
            HeadSHA:       p.PullRequest.Head.SHA,
            BaseSHA:       p.PullRequest.Base.SHA,
            ReplyTarget:   replyTarget, // ReviewService writes into execution.system_metadata.reply_target
        }
        if integ.ActingEmployeeID != nil {
            req.ActingEmployeeID = integ.ActingEmployeeID.String()
        }
        _, err := r.reviews.Trigger(ctx, req)
        return err
    }
    ```
  - Add to `src-go/internal/model/review.go` `TriggerReviewRequest` struct (extension only; don't reorder existing fields):
    ```go
    IntegrationID    string         `json:"integrationId,omitempty"`
    HeadSHA          string         `json:"headSha,omitempty"`
    BaseSHA          string         `json:"baseSha,omitempty"`
    ReplyTarget      map[string]any `json:"replyTarget,omitempty"`
    ActingEmployeeID string         `json:"actingEmployeeId,omitempty"`
    ```
  - In `ReviewService.Trigger` (only edit needed): when persisting the new `Review` row, copy `req.IntegrationID / HeadSHA / BaseSHA` into the review (parsing IntegrationID via `uuid.Parse`). Do NOT touch the rest of the function. Also: if `req.ReplyTarget != nil`, pass it through to the workflow execution's `system_metadata.reply_target` (which 1A's `launchWorkflowBackedReview` already understands). Match the existing 1C/1D pattern for the IM trigger path.

- [x] Step 4.3 — verify
  - `rtk go test ./internal/service/... -run Router` — three router tests green.

---

## Task 5 — Webhook handler: HMAC verify + dedup + route

- [x] Step 5.1 — failing tests
  - File: `src-go/internal/handler/vcs_webhook_handler_test.go` (new)
    ```go
    package handler_test

    import (
        "bytes"
        "context"
        "crypto/hmac"
        "crypto/sha256"
        "encoding/hex"
        "errors"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"

        "github.com/react-go-quick-starter/server/internal/handler"
        "github.com/react-go-quick-starter/server/internal/middleware"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    type stubIntegrations struct{ integ *model.VCSIntegration; err error }
    func (s *stubIntegrations) ResolveByRepo(_ context.Context, host, owner, repo string) (*model.VCSIntegration, error) {
        return s.integ, s.err
    }

    type stubSecrets struct{ value string; err error }
    func (s *stubSecrets) Resolve(_ context.Context, _ uuid.UUID, _ string) (string, error) {
        return s.value, s.err
    }

    type stubRouter struct{ called bool; err error }
    func (s *stubRouter) RouteEvent(_ context.Context, _ *model.VCSIntegration, _, _ string, _ []byte) error {
        s.called = true; return s.err
    }

    type stubEventsRepo struct{ insertErr error; insertedCount int }
    func (s *stubEventsRepo) Insert(_ context.Context, _ *model.VCSWebhookEvent) error {
        s.insertedCount++; return s.insertErr
    }
    func (s *stubEventsRepo) MarkProcessed(_ context.Context, _ [16]byte, _ string) error { return nil }

    func sign(secret, body []byte) string {
        m := hmac.New(sha256.New, secret); m.Write(body)
        return "sha256=" + hex.EncodeToString(m.Sum(nil))
    }

    func newCtx(t *testing.T, body []byte, sig, event, delivery string) (echo.Context, *httptest.ResponseRecorder) {
        e := echo.New()
        req := httptest.NewRequest(http.MethodPost, "/api/v1/vcs/github/webhook", bytes.NewReader(body))
        req.Header.Set("X-Hub-Signature-256", sig)
        req.Header.Set("X-GitHub-Event", event)
        req.Header.Set("X-GitHub-Delivery", delivery)
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        // Mimic the middleware contract:
        c.Set(middleware.RawBodyKey, body)
        return c, rec
    }

    func TestWebhook_ValidSignature_RoutesAndAccepts(t *testing.T) {
        body := []byte(`{"action":"opened","pull_request":{"number":1,"head":{"sha":"a"},"base":{"sha":"b"},"html_url":"https://github.com/o/r/pull/1"},"repository":{"owner":{"login":"o"},"name":"r"}}`)
        secret := []byte("s3cr3t")
        integ := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Host: "github.com", Owner: "o", Repo: "r", WebhookSecretRef: "vcs.github.x.webhook"}
        router := &stubRouter{}
        repo := &stubEventsRepo{}
        h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: integ}, &stubSecrets{value: string(secret)}, router, repo, nil)

        c, rec := newCtx(t, body, sign(secret, body), "pull_request", "delivery-1")
        if err := h.HandleGitHubWebhook(c); err != nil { t.Fatal(err) }
        if rec.Code != http.StatusAccepted { t.Fatalf("status %d", rec.Code) }
        if !router.called { t.Fatal("router not invoked") }
        if repo.insertedCount != 1 { t.Fatal("event not persisted") }
    }

    func TestWebhook_BadSignature_401(t *testing.T) {
        body := []byte(`{"action":"opened"}`)
        integ := &model.VCSIntegration{ID: uuid.New(), Host: "github.com", Owner: "o", Repo: "r"}
        h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: integ}, &stubSecrets{value: "right"}, &stubRouter{}, &stubEventsRepo{}, nil)
        c, rec := newCtx(t, body, sign([]byte("WRONG"), body), "pull_request", "d")
        _ = h.HandleGitHubWebhook(c)
        if rec.Code != http.StatusUnauthorized { t.Fatalf("status %d", rec.Code) }
    }

    func TestWebhook_MissingSignature_401(t *testing.T) {
        body := []byte(`{}`)
        h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: &model.VCSIntegration{}}, &stubSecrets{value: "s"}, &stubRouter{}, &stubEventsRepo{}, nil)
        c, rec := newCtx(t, body, "", "pull_request", "d")
        _ = h.HandleGitHubWebhook(c)
        if rec.Code != http.StatusUnauthorized { t.Fatalf("status %d", rec.Code) }
    }

    func TestWebhook_DuplicateEvent_200NoOp(t *testing.T) {
        body := []byte(`{"action":"opened","pull_request":{"number":1,"head":{"sha":"a"},"base":{"sha":"b"},"html_url":"x"},"repository":{"owner":{"login":"o"},"name":"r"}}`)
        secret := []byte("s")
        integ := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Host: "github.com", Owner: "o", Repo: "r"}
        router := &stubRouter{}
        repo := &stubEventsRepo{insertErr: repository.ErrVCSWebhookEventDuplicate}
        h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: integ}, &stubSecrets{value: string(secret)}, router, repo, nil)

        c, rec := newCtx(t, body, sign(secret, body), "pull_request", "delivery-1")
        if err := h.HandleGitHubWebhook(c); err != nil { t.Fatal(err) }
        if rec.Code != http.StatusOK { t.Fatalf("dup expected 200 noop, got %d", rec.Code) }
        if router.called { t.Fatal("router must NOT be invoked on dup") }
    }

    func TestWebhook_IntegrationNotFound_404(t *testing.T) {
        body := []byte(`{"repository":{"owner":{"login":"o"},"name":"r"}}`)
        h := handler.NewVCSWebhookHandler(&stubIntegrations{err: repository.ErrVCSIntegrationNotFound}, &stubSecrets{value: "s"}, &stubRouter{}, &stubEventsRepo{}, nil)
        c, rec := newCtx(t, body, sign([]byte("s"), body), "pull_request", "d")
        _ = h.HandleGitHubWebhook(c)
        if rec.Code != http.StatusNotFound { t.Fatalf("status %d", rec.Code) }
        if !errors.Is(repository.ErrVCSIntegrationNotFound, repository.ErrVCSIntegrationNotFound) {
            t.Fatal("sentinel marker")
        }
    }
    ```

- [x] Step 5.2 — implement handler
  - File: `src-go/internal/handler/vcs_webhook_handler.go` (new)
    ```go
    package handler

    import (
        "context"
        "crypto/hmac"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "net/http"
        "strings"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"
        log "github.com/sirupsen/logrus"

        "github.com/react-go-quick-starter/server/internal/middleware"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    // Surfaces this handler depends on (all injected so tests can stub).
    type IntegrationResolver interface {
        ResolveByRepo(ctx context.Context, host, owner, repo string) (*model.VCSIntegration, error)
    }
    type SecretsResolver interface {
        Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
    }
    type WebhookRouter interface {
        RouteEvent(ctx context.Context, integ *model.VCSIntegration, eventType, deliveryID string, body []byte) error
    }
    type WebhookEventsWriter interface {
        Insert(ctx context.Context, e *model.VCSWebhookEvent) error
        MarkProcessed(ctx context.Context, id [16]byte, procErr string) error
    }
    type AuditRecorder interface {
        RecordEvent(ctx context.Context, e *model.AuditEvent) error
    }

    type VCSWebhookHandler struct {
        integrations IntegrationResolver
        secrets      SecretsResolver
        router       WebhookRouter
        events       WebhookEventsWriter
        audit        AuditRecorder
    }

    func NewVCSWebhookHandler(i IntegrationResolver, s SecretsResolver, r WebhookRouter, ev WebhookEventsWriter, a AuditRecorder) *VCSWebhookHandler {
        return &VCSWebhookHandler{integrations: i, secrets: s, router: r, events: ev, audit: a}
    }

    type repoRefPayload struct {
        Repository struct {
            Owner struct{ Login string `json:"login"` } `json:"owner"`
            Name  string `json:"name"`
        } `json:"repository"`
    }

    func (h *VCSWebhookHandler) HandleGitHubWebhook(c echo.Context) error {
        ctx := c.Request().Context()

        raw, _ := c.Get(middleware.RawBodyKey).([]byte)
        if len(raw) == 0 {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "vcs:webhook_empty_body"})
        }
        sig := strings.TrimSpace(c.Request().Header.Get("X-Hub-Signature-256"))
        eventType := strings.TrimSpace(c.Request().Header.Get("X-GitHub-Event"))
        deliveryID := strings.TrimSpace(c.Request().Header.Get("X-GitHub-Delivery"))
        if sig == "" || deliveryID == "" || eventType == "" {
            h.recordSignatureInvalid(ctx, "", "missing_headers")
            return c.JSON(http.StatusUnauthorized, map[string]string{"error": "vcs:webhook_signature_invalid"})
        }

        // Parse repo coordinates from payload (always present in GitHub events except `ping`).
        var ref repoRefPayload
        _ = json.Unmarshal(raw, &ref)
        host := inferGitHubHost(c.Request().UserAgent())
        owner := ref.Repository.Owner.Login
        repo := ref.Repository.Name
        if owner == "" || repo == "" {
            // ping / installation / etc. — accept silently.
            log.WithField("event", eventType).Debug("vcs_webhook: no repo in payload, accepting noop")
            return c.NoContent(http.StatusAccepted)
        }

        integ, err := h.integrations.ResolveByRepo(ctx, host, owner, repo)
        if err != nil {
            if errors.Is(err, repository.ErrVCSIntegrationNotFound) {
                return c.JSON(http.StatusNotFound, map[string]string{"error": "vcs:integration_not_found"})
            }
            log.WithError(err).Warn("vcs_webhook: resolve integration")
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "vcs:integration_lookup_failed"})
        }

        secret, err := h.secrets.Resolve(ctx, integ.ProjectID, integ.WebhookSecretRef)
        if err != nil || secret == "" {
            log.WithError(err).Warn("vcs_webhook: resolve webhook secret")
            return c.JSON(http.StatusUnauthorized, map[string]string{"error": "vcs:webhook_signature_invalid"})
        }

        if !verifyGitHubSignature([]byte(secret), raw, sig) {
            h.recordSignatureInvalid(ctx, integ.ID.String(), "hmac_mismatch")
            return c.JSON(http.StatusUnauthorized, map[string]string{"error": "vcs:webhook_signature_invalid"})
        }

        sum := sha256.Sum256(raw)
        ev := &model.VCSWebhookEvent{
            ID: uuid.New(), IntegrationID: integ.ID,
            EventID: deliveryID, EventType: eventType,
            PayloadHash: sum[:],
        }
        if err := h.events.Insert(ctx, ev); err != nil {
            if errors.Is(err, repository.ErrVCSWebhookEventDuplicate) {
                return c.JSON(http.StatusOK, map[string]string{"status": "duplicate"})
            }
            log.WithError(err).Warn("vcs_webhook: persist event")
            return c.JSON(http.StatusInternalServerError, map[string]string{"error": "vcs:event_persist_failed"})
        }
        h.recordReceived(ctx, integ.ID.String(), eventType, deliveryID)

        if err := h.router.RouteEvent(ctx, integ, eventType, deliveryID, raw); err != nil {
            if errors.Is(err, errors.New("vcs_webhook_router: push/synchronize handler is owned by Plan 2C")) {
                // Plan 2C will plug in. Accept now so GitHub doesn't retry.
                _ = h.events.MarkProcessed(ctx, ev.ID, "push_handler_pending_plan_2c")
                return c.NoContent(http.StatusAccepted)
            }
            log.WithError(err).WithField("event", eventType).Warn("vcs_webhook: route")
            _ = h.events.MarkProcessed(ctx, ev.ID, err.Error())
            return c.JSON(http.StatusAccepted, map[string]string{"status": "routed_with_error"})
        }
        _ = h.events.MarkProcessed(ctx, ev.ID, "")
        return c.NoContent(http.StatusAccepted)
    }

    func verifyGitHubSignature(secret, body []byte, headerVal string) bool {
        const prefix = "sha256="
        if !strings.HasPrefix(headerVal, prefix) {
            return false
        }
        expected, err := hex.DecodeString(strings.TrimPrefix(headerVal, prefix))
        if err != nil {
            return false
        }
        m := hmac.New(sha256.New, secret); m.Write(body)
        return hmac.Equal(expected, m.Sum(nil))
    }

    func inferGitHubHost(userAgent string) string {
        // GitHub.com sends "GitHub-Hookshot/<id>". Enterprise installations send the
        // same UA shape, so default to github.com unless an X-GitHub-Enterprise-Host
        // header (set by GHES) overrides downstream. v1: github.com only.
        return "github.com"
    }

    func (h *VCSWebhookHandler) recordSignatureInvalid(ctx context.Context, integID, reason string) {
        if h.audit == nil { return }
        _ = h.audit.RecordEvent(ctx, &model.AuditEvent{
            Action: "vcs.webhook_signature_invalid",
            Resource: "vcs_integration",
            ResourceID: integID,
            Metadata: map[string]any{"reason": reason},
        })
    }

    func (h *VCSWebhookHandler) recordReceived(ctx context.Context, integID, eventType, deliveryID string) {
        if h.audit == nil { return }
        _ = h.audit.RecordEvent(ctx, &model.AuditEvent{
            Action: "vcs.webhook_received",
            Resource: "vcs_integration",
            ResourceID: integID,
            Metadata: map[string]any{"event_type": eventType, "delivery_id": deliveryID},
        })
    }
    ```
  - The literal-string compare against `ErrPushHandlerNotImplemented.Error()` above is intentionally awkward; replace with `errors.Is(err, service.ErrPushHandlerNotImplemented)` once the import isn't a cycle (the handler package can import service). Confirm during implementation.

- [x] Step 5.3 — verify
  - `rtk go test ./internal/handler/... -run Webhook` — five handler tests green.

---

## Task 6 — Route registration + rate limit + raw-body wiring

- [x] Step 6.1 — wire route in `src-go/internal/server/routes.go` (or wherever Echo group setup lives):
  ```go
  // VCS webhooks: no JWT (HMAC-verified); raw body required for HMAC.
  vcsGroup := api.Group("/vcs")
  vcsGroup.POST("/github/webhook",
      vcsWebhookHandler.HandleGitHubWebhook,
      mw.CaptureRawBody(),
      mw.RateLimit(mw.RateLimitOpts{Key: "vcs_webhook", PerMinute: 100, ScopeBy: mw.ScopeByHeader("X-GitHub-Delivery")}),
  )
  ```
  - If a generic `RateLimit` middleware doesn't yet exist, fall back to the per-IP rate limiter already used by `form_handler` (`form_service.checkRateLimit`). Document the chosen approach inline.

- [x] Step 6.2 — bootstrap construction (in the same file or `cmd/server/main.go` wiring section):
  ```go
  vcsEventsRepo := repository.NewVCSWebhookEventsRepo(pool)
  vcsRouter := service.NewVCSWebhookRouter(reviewService)
  vcsWebhookHandler := handler.NewVCSWebhookHandler(
      vcsIntegrationsRepo, // owned by Plan 2A
      secretsService,      // owned by Plan 1B
      vcsRouter,
      vcsEventsRepo,
      auditService,
  )
  ```

- [x] Step 6.3 — verify
  - `rtk go build ./...`
  - Smoke: `curl -i -X POST http://localhost:7777/api/v1/vcs/github/webhook -H 'X-GitHub-Event: ping' -d '{}'` → 401 (no signature) — confirms the route is mounted and the middleware fires.

---

## Task 7 — `EventVCSDeliveryFailed` + `EventVCSAuthExpired` event types

- [x] Step 7.1 — failing test: constants stable
  - File: `src-go/internal/eventbus/types_vcs_test.go` (new)
    ```go
    package eventbus

    import "testing"

    func TestEventVCSConstantsStable(t *testing.T) {
        if EventVCSDeliveryFailed != "vcs.delivery.failed" {
            t.Fatalf("renamed: %s", EventVCSDeliveryFailed)
        }
        if EventVCSAuthExpired != "vcs.auth.expired" {
            t.Fatalf("renamed: %s", EventVCSAuthExpired)
        }
    }
    ```

- [x] Step 7.2 — add constants
  - In `src-go/internal/eventbus/types.go`, append next to `EventReviewCompleted`:
    ```go
    // VCS outbound delivery + auth expiration; emitted by vcs_outbound_dispatcher.
    EventVCSDeliveryFailed = "vcs.delivery.failed"
    EventVCSAuthExpired    = "vcs.auth.expired"
    ```

- [x] Step 7.3 — register in WS event allowlist
  - In `src-go/internal/ws/events.go` (or wherever the broadcast filter lives), add the two new types so the WS hub forwards them to the FE subscription.

- [x] Step 7.4 — verify
  - `rtk go test ./internal/eventbus/... ./internal/ws/...`

---

## Task 8 — `vcs_outbound_dispatcher` (subscriber for `EventReviewCompleted`)

- [x] Step 8.1 — failing tests using mock provider
  - File: `src-go/internal/service/vcs_outbound_dispatcher_test.go` (new)
    ```go
    package service_test

    import (
        "context"
        "testing"
        "time"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/eventbus"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/service"
        "github.com/react-go-quick-starter/server/internal/vcs"
        "github.com/react-go-quick-starter/server/internal/vcs/mock" // owned by Plan 2A
    )

    type stubReviews struct{ review *model.Review; findings []model.ReviewFinding }
    func (s *stubReviews) GetByID(_ context.Context, _ uuid.UUID) (*model.Review, error) { return s.review, nil }
    func (s *stubReviews) ListFindings(_ context.Context, _ uuid.UUID) ([]model.ReviewFinding, error) { return s.findings, nil }
    func (s *stubReviews) UpdateSummaryCommentID(_ context.Context, _ uuid.UUID, id string) error { s.review.SummaryCommentID = id; return nil }
    func (s *stubReviews) UpdateFindingInlineCommentID(_ context.Context, _ uuid.UUID, id string) error { return nil }

    type stubProviderRegistry struct{ p vcs.Provider }
    func (s *stubProviderRegistry) Resolve(_ string) (vcs.Provider, error) { return s.p, nil }

    func newDispatcher(t *testing.T, prov vcs.Provider, rev *model.Review, findings []model.ReviewFinding) (*service.VCSOutboundDispatcher, *eventbus.MemoryBus) {
        bus := eventbus.NewMemoryBus()
        d := service.NewVCSOutboundDispatcher(
            &stubReviews{review: rev, findings: findings},
            &stubProviderRegistry{p: prov},
            &stubSecretsResolver{value: "pat"},
            bus,
            "https://fe.example",
        )
        d.SetRetryDelays(0, 0, 0) // disable backoff in tests
        return d, bus
    }

    func TestDispatcher_PostsSummaryAndInline_FirstTime(t *testing.T) {
        prov := mock.New()
        integID := uuid.New()
        rev := &model.Review{ID: uuid.New(), IntegrationID: &integID, PRNumber: 42, ProjectID: uuid.New()}
        findings := []model.ReviewFinding{
            {ID: uuid.New(), File: "x.go", Line: 10, Severity: "warning", Message: "oops"},
        }
        d, _ := newDispatcher(t, prov, rev, findings)

        d.HandleReviewCompleted(context.Background(), rev.ID)
        if len(prov.SummaryPosts) != 1 { t.Fatalf("summary posts: %+v", prov.SummaryPosts) }
        if len(prov.InlinePosts) != 1 || prov.InlinePosts[0].Comments[0].Path != "x.go" {
            t.Fatalf("inline posts: %+v", prov.InlinePosts)
        }
        if rev.SummaryCommentID == "" { t.Fatal("summary comment id not persisted") }
    }

    func TestDispatcher_EditsExistingSummary_OnReReview(t *testing.T) {
        prov := mock.New()
        integID := uuid.New()
        rev := &model.Review{ID: uuid.New(), IntegrationID: &integID, PRNumber: 42, SummaryCommentID: "existing-1"}
        d, _ := newDispatcher(t, prov, rev, nil)
        d.HandleReviewCompleted(context.Background(), rev.ID)
        if len(prov.SummaryEdits) != 1 || prov.SummaryEdits[0].CommentID != "existing-1" {
            t.Fatalf("expected edit, got %+v", prov.SummaryEdits)
        }
    }

    func TestDispatcher_RetriesOnTransientError_ThenEmitsFailureEvent(t *testing.T) {
        prov := mock.New()
        prov.SummaryError = errSummaryTransient // mock returns this 3 times then succeeds = always
        integID := uuid.New()
        rev := &model.Review{ID: uuid.New(), IntegrationID: &integID, PRNumber: 42, ProjectID: uuid.New()}
        d, bus := newDispatcher(t, prov, rev, nil)

        d.HandleReviewCompleted(context.Background(), rev.ID)

        if got := prov.SummaryPostAttempts; got != 3 {
            t.Fatalf("expected 3 attempts, got %d", got)
        }
        events := bus.Drain(50 * time.Millisecond)
        if len(events) != 1 || events[0].Type != eventbus.EventVCSDeliveryFailed {
            t.Fatalf("expected one EventVCSDeliveryFailed, got %+v", events)
        }
    }

    func TestDispatcher_NoIntegrationID_Skips(t *testing.T) {
        prov := mock.New()
        rev := &model.Review{ID: uuid.New(), IntegrationID: nil}
        d, _ := newDispatcher(t, prov, rev, nil)
        d.HandleReviewCompleted(context.Background(), rev.ID)
        if len(prov.SummaryPosts) != 0 { t.Fatal("must not post when no integration_id") }
    }
    ```
  - The mock provider (`internal/vcs/mock`) is provided by Plan 2A; coordinate to ensure it exposes `SummaryPosts / SummaryEdits / InlinePosts / InlineEdits` slices and `SummaryError / SummaryPostAttempts` knobs.

- [x] Step 8.2 — implement dispatcher
  - File: `src-go/internal/service/vcs_outbound_dispatcher.go` (new)
    ```go
    package service

    import (
        "context"
        "encoding/json"
        "fmt"
        "strings"
        "time"

        "github.com/google/uuid"
        log "github.com/sirupsen/logrus"

        eb "github.com/react-go-quick-starter/server/internal/eventbus"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    const summaryBodyHardLimit = 50 * 1024 // 50KB safety cap; Trace A's "see full review at <url>" tail.

    type ReviewReader interface {
        GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
        ListFindings(ctx context.Context, reviewID uuid.UUID) ([]model.ReviewFinding, error)
        UpdateSummaryCommentID(ctx context.Context, reviewID uuid.UUID, commentID string) error
        UpdateFindingInlineCommentID(ctx context.Context, findingID uuid.UUID, commentID string) error
    }

    type ProviderRegistry interface {
        Resolve(name string) (vcs.Provider, error)
    }

    type VCSOutboundDispatcher struct {
        reviews    ReviewReader
        providers  ProviderRegistry
        secrets    SecretsResolver
        bus        eb.Publisher
        feBaseURL  string
        delays     [3]time.Duration
    }

    func NewVCSOutboundDispatcher(r ReviewReader, p ProviderRegistry, s SecretsResolver, bus eb.Publisher, feBaseURL string) *VCSOutboundDispatcher {
        return &VCSOutboundDispatcher{
            reviews: r, providers: p, secrets: s, bus: bus, feBaseURL: feBaseURL,
            delays: [3]time.Duration{1 * time.Second, 4 * time.Second, 16 * time.Second},
        }
    }

    func (d *VCSOutboundDispatcher) SetRetryDelays(d1, d2, d3 time.Duration) {
        d.delays = [3]time.Duration{d1, d2, d3}
    }

    // --- eventbus.Mod surface ---
    func (d *VCSOutboundDispatcher) Name() string         { return "service.vcs-outbound-dispatcher" }
    func (d *VCSOutboundDispatcher) Intercepts() []string { return []string{eb.EventReviewCompleted} }
    func (d *VCSOutboundDispatcher) Priority() int        { return 90 }
    func (d *VCSOutboundDispatcher) Mode() eb.Mode        { return eb.ModeObserve }

    type reviewCompletedPayload struct {
        ID string `json:"id"`
    }

    func (d *VCSOutboundDispatcher) Observe(ctx context.Context, e *eb.Event, _ *eb.PipelineCtx) {
        var p reviewCompletedPayload
        if err := json.Unmarshal(e.Payload, &p); err != nil {
            log.WithError(err).Warn("vcs_outbound_dispatcher: payload decode")
            return
        }
        id, err := uuid.Parse(p.ID)
        if err != nil { return }
        go d.HandleReviewCompleted(context.Background(), id)
    }

    // HandleReviewCompleted is exported for direct test invocation.
    func (d *VCSOutboundDispatcher) HandleReviewCompleted(ctx context.Context, reviewID uuid.UUID) {
        rev, err := d.reviews.GetByID(ctx, reviewID)
        if err != nil || rev == nil {
            log.WithError(err).WithField("reviewId", reviewID).Warn("vcs_dispatcher: load review")
            return
        }
        if rev.IntegrationID == nil {
            log.WithField("reviewId", reviewID).Debug("vcs_dispatcher: skip (no integration_id)")
            return
        }
        // Resolve integration + PAT. (Loader is part of integrations repo wired in T9 bootstrap.)
        integ, err := d.loadIntegration(ctx, *rev.IntegrationID)
        if err != nil {
            log.WithError(err).Warn("vcs_dispatcher: load integration"); return
        }
        pat, err := d.secrets.Resolve(ctx, integ.ProjectID, integ.TokenSecretRef)
        if err != nil || pat == "" {
            log.WithError(err).Warn("vcs_dispatcher: resolve PAT"); return
        }
        prov, err := d.providers.Resolve(integ.Provider)
        if err != nil { log.WithError(err).Warn("vcs_dispatcher: provider"); return }
        prov = prov.WithCredentials(pat) // see vcs.Provider contract from Plan 2A

        findings, _ := d.reviews.ListFindings(ctx, reviewID)
        pr := &vcs.PullRequest{
            Repo:   vcs.RepoRef{Host: integ.Host, Owner: integ.Owner, Repo: integ.Repo},
            Number: rev.PRNumber,
            HeadSHA: rev.HeadSHA,
        }

        body := d.buildSummaryBody(rev, findings)

        if err := d.deliverSummary(ctx, prov, pr, rev, body); err != nil {
            d.emitFailure(ctx, rev, "summary", err); return
        }
        d.deliverInline(ctx, prov, pr, rev, findings)
    }

    func (d *VCSOutboundDispatcher) deliverSummary(ctx context.Context, prov vcs.Provider, pr *vcs.PullRequest, rev *model.Review, body string) error {
        var lastErr error
        for attempt := 0; attempt < 3; attempt++ {
            if attempt > 0 { time.Sleep(d.delays[attempt-1]) }
            if rev.SummaryCommentID != "" {
                if err := prov.EditSummaryComment(ctx, pr, rev.SummaryCommentID, body); err == nil {
                    return nil
                } else {
                    lastErr = err
                }
            } else {
                id, err := prov.PostSummaryComment(ctx, pr, body)
                if err == nil {
                    rev.SummaryCommentID = id
                    _ = d.reviews.UpdateSummaryCommentID(ctx, rev.ID, id)
                    return nil
                }
                lastErr = err
            }
            log.WithError(lastErr).WithField("attempt", attempt+1).Warn("vcs_dispatcher: summary failed")
        }
        return lastErr
    }

    func (d *VCSOutboundDispatcher) deliverInline(ctx context.Context, prov vcs.Provider, pr *vcs.PullRequest, rev *model.Review, findings []model.ReviewFinding) {
        var fresh []vcs.InlineComment
        var freshIdx []int
        for i, f := range findings {
            if f.File == "" || f.Line <= 0 { continue }
            body := fmt.Sprintf("**[%s]** %s", strings.ToUpper(f.Severity), f.Message)
            if f.SuggestedPatch != "" {
                body += "\n```suggestion\n" + f.SuggestedPatch + "\n```"
            }
            ic := vcs.InlineComment{Path: f.File, Line: f.Line, Body: body, Side: "RIGHT"}
            if f.InlineCommentID != "" {
                _ = retry(d.delays, func() error { return prov.EditReviewComment(ctx, pr, f.InlineCommentID, body) })
                continue
            }
            fresh = append(fresh, ic)
            freshIdx = append(freshIdx, i)
        }
        if len(fresh) == 0 { return }
        ids, err := prov.PostReviewComments(ctx, pr, fresh)
        if err != nil {
            d.emitFailure(ctx, rev, "inline", err); return
        }
        for n, id := range ids {
            if n >= len(freshIdx) { break }
            f := findings[freshIdx[n]]
            _ = d.reviews.UpdateFindingInlineCommentID(ctx, f.ID, id)
        }
    }

    func retry(delays [3]time.Duration, fn func() error) error {
        var lastErr error
        for attempt := 0; attempt < 3; attempt++ {
            if attempt > 0 { time.Sleep(delays[attempt-1]) }
            if err := fn(); err == nil { return nil } else { lastErr = err }
        }
        return lastErr
    }

    func (d *VCSOutboundDispatcher) buildSummaryBody(rev *model.Review, findings []model.ReviewFinding) string {
        var crit, warn int
        for _, f := range findings {
            switch strings.ToLower(f.Severity) {
            case "critical", "error": crit++
            case "warning", "warn":   warn++
            }
        }
        var b strings.Builder
        b.WriteString("## AgentForge Review\n\n")
        fmt.Fprintf(&b, "- 🔴 %d critical / 🟡 %d warnings\n", crit, warn)
        fmt.Fprintf(&b, "- [Open full review](%s/reviews/%s)\n\n", strings.TrimRight(d.feBaseURL, "/"), rev.ID)
        if len(findings) > 0 {
            b.WriteString("## Findings\n\n")
            for _, f := range findings {
                fmt.Fprintf(&b, "- **[%s]** `%s:%d` — %s\n", strings.ToUpper(f.Severity), f.File, f.Line, f.Message)
            }
        }
        out := b.String()
        if len(out) > summaryBodyHardLimit {
            cut := summaryBodyHardLimit - 200
            if cut < 0 { cut = 0 }
            out = out[:cut] + fmt.Sprintf("\n\n…truncated; see full review at %s/reviews/%s\n", strings.TrimRight(d.feBaseURL, "/"), rev.ID)
        }
        return out
    }

    func (d *VCSOutboundDispatcher) emitFailure(ctx context.Context, rev *model.Review, op string, lastErr error) {
        if d.bus == nil { return }
        msg := ""
        if lastErr != nil { msg = lastErr.Error() }
        _ = eb.PublishLegacy(ctx, d.bus, eb.EventVCSDeliveryFailed, rev.ProjectID.String(), map[string]any{
            "review_id": rev.ID.String(),
            "op":        op,
            "error":     msg,
        })
    }

    // loadIntegration — implemented by injecting an `IntegrationLoader` interface
    // (mirror of repository.VCSIntegrationsRepo.GetByID) at construction; omitted
    // here to keep the file readable. Add field + constructor arg.
    func (d *VCSOutboundDispatcher) loadIntegration(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error) {
        return nil, fmt.Errorf("loadIntegration not wired — add IntegrationLoader to constructor")
    }
    ```
  - Replace the `loadIntegration` stub with a real `IntegrationLoader` interface + constructor field. The existing `vcs_integrations` repo from Plan 2A already exposes `GetByID(ctx, id)`.
  - Auth-expired detection: if `prov.PostSummaryComment` returns the sentinel `vcs.ErrAuthExpired` (defined by Plan 2A), emit `EventVCSAuthExpired` instead of `EventVCSDeliveryFailed` and call an integration-status setter (`integrations.MarkAuthExpired(ctx, integ.ID)` — extend Plan 2A's repo if missing). Add a fifth test `TestDispatcher_AuthExpired_EmitsAuthEvent_AndMarks`.

- [x] Step 8.3 — verify
  - `rtk go test ./internal/service/... -run Dispatcher` — five dispatcher tests green.

---

## Task 9 — Bootstrap dispatcher subscription

- [x] Step 9.1 — register the dispatcher with the eventbus alongside the existing IM outbound dispatcher (Plan 1D) at server startup:
  ```go
  vcsOutbound := service.NewVCSOutboundDispatcher(
      reviewsRepo,           // satisfies ReviewReader
      vcsProviderRegistry,   // owned by Plan 2A
      secretsService,        // owned by Plan 1B
      eventBus,
      cfg.PublicBaseURL,     // FE base URL
  )
  eventBus.Register(vcsOutbound)
  ```

- [x] Step 9.2 — verify
  - `rtk go build ./...`
  - Dev-stack smoke (manual): Trigger a fake review via `POST /api/v1/reviews` with `complete` request → confirm dispatcher fires by tailing `logs/go-orchestrator.log` for `vcs_dispatcher` lines (skipped because no `integration_id` — that's correct).

---

## Task 10 — FE: "PR comment 发送失败" badge on review/execution view

- [x] Step 10.1 — failing test
  - File: `components/workflow/workflow-execution-view.test.tsx` (existing — extend, do NOT touch unrelated tests)
    ```tsx
    it("renders VCS delivery failed badge when review run carries vcs.delivery.failed", async () => {
      const run = makeRun({ events: [{ type: "vcs.delivery.failed", payload: { op: "summary", error: "HTTP 401" } }] });
      renderWith(run);
      expect(await screen.findByText(/PR comment 发送失败/i)).toBeInTheDocument();
    });
    ```

- [x] Step 10.2 — implement
  - In `components/workflow/workflow-execution-view.tsx`: subscribe to `vcs.delivery.failed` events alongside the existing `workflow.outbound_delivery.failed` handler (mirror the badge logic); badge text from `lib/i18n/zh-CN/workflow.ts` key `vcs.delivery_failed_badge`. Add the en-US translation as a sibling.
  - If a dedicated review-detail page exists (`app/(dashboard)/reviews/[id]/page.tsx`) — surface the same badge there too.

- [x] Step 10.3 — verify
  - `rtk pnpm test workflow-execution-view`
  - `rtk pnpm lint`

---

## Task 11 — Audit + rate-limit verification

- [x] Step 11.1 — failing tests
  - File: `src-go/internal/handler/vcs_webhook_audit_test.go` (new)
    ```go
    func TestWebhook_AuditRecordedOnReceived(t *testing.T) {
        rec := &recordingAudit{}
        h := handler.NewVCSWebhookHandler(/*...*/, rec)
        // ... happy-path call ...
        if !rec.HasAction("vcs.webhook_received") { t.Fatal("missing audit") }
    }

    func TestWebhook_AuditRecordedOnSignatureInvalid(t *testing.T) {
        rec := &recordingAudit{}
        h := handler.NewVCSWebhookHandler(/*...*/, rec)
        // ... bad-signature call ...
        if !rec.HasAction("vcs.webhook_signature_invalid") { t.Fatal("missing audit") }
    }
    ```

- [x] Step 11.2 — confirm rate limit fires
  - File: `src-go/internal/handler/vcs_webhook_ratelimit_test.go` (new)
    ```go
    func TestWebhook_RateLimit_101stRequestRejected(t *testing.T) {
        // Use the real middleware chain via httptest.NewServer; fire 101 requests with
        // distinct X-GitHub-Delivery values to the same integration; expect 429 on the 101st.
    }
    ```

- [x] Step 11.3 — verify
  - `rtk go test ./internal/handler/... -run Webhook` — all pass.

---

## Task 12 — Integration test: real PG + mock provider, end-to-end Trace A

- [x] Step 12.1 — failing E2E test
  - File: `src-go/internal/service/vcs_trace_a_integration_test.go` (new)
    ```go
    //go:build integration

    package service_test

    import (
        "context"
        "encoding/json"
        "io/os"
        "testing"
        "time"

        "github.com/react-go-quick-starter/server/internal/repository/repotest"
        "github.com/react-go-quick-starter/server/internal/service"
        "github.com/react-go-quick-starter/server/internal/vcs/mock"
    )

    func TestTraceA_PROpened_EndToEnd(t *testing.T) {
        ctx := context.Background()
        env := repotest.NewIntegrationEnv(t) // PG + Redis + bus, owned by existing test infra
        prov := mock.New()

        integID := repotest.SeedVCSIntegration(t, env.DB, repotest.IntegOpts{
            Owner: "o", Repo: "r", Provider: "github", WebhookSecret: "s3cr3t",
        })

        // 1) Fire fake GitHub webhook payload via the handler under test.
        payload, _ := os.ReadFile("testdata/github_pr_opened.json")
        rec := env.PostWebhook("/api/v1/vcs/github/webhook", payload, "pull_request", "delivery-trace-a", "s3cr3t")
        if rec.Code != 202 { t.Fatalf("webhook: %d", rec.Code) }

        // 2) Wait for ReviewService.Trigger → review row created.
        review := repotest.WaitForReview(t, env.DB, integID, 5*time.Second)
        if review.HeadSHA == "" { t.Fatal("head_sha not propagated") }

        // 3) Simulate ReviewService.Complete → bus emits EventReviewCompleted.
        env.Reviews.Complete(ctx, review.ID, &model.CompleteReviewRequest{
            Findings: []model.ReviewFinding{{File: "x.go", Line: 7, Severity: "warning", Message: "x"}},
        })

        // 4) Dispatcher fires; mock provider receives summary + inline.
        repotest.Eventually(t, 5*time.Second, func() bool {
            return len(prov.SummaryPosts) == 1 && len(prov.InlinePosts) == 1
        })
    }
    ```
  - Fixture file: `src-go/internal/service/testdata/github_pr_opened.json` — minimal PR-opened payload (use a real GitHub sample; ~2KB).

- [x] Step 12.2 — verify
  - `rtk go test -tags=integration ./internal/service/... -run TraceA`

---

## Task 13 — Self-review + spec drift writeback

- [x] Step 13.1 — re-read Spec 2 §5 / §6.1-6.3 / §7 / §9 Trace A / §10 against the diff.
- [x] Step 13.2 — for any divergence (e.g. a renamed event type, a different table column ordering, a behavior choice not mandated by spec), append a row to `docs/superpowers/specs/2026-04-20-code-reviewer-employee-design.md` §13.1 "Spec Drifts Found During Brainstorm" with: file path · spec-line · what we did · why.
- [x] Step 13.3 — verify
  - `rtk go test ./...`
  - `rtk go build ./...`
  - `rtk pnpm lint`
  - `rtk pnpm test`

---

## Done criteria

- A POST to `/api/v1/vcs/github/webhook` with a valid HMAC signature and a `pull_request{action:opened}` payload causes:
  - `vcs_webhook_events` row inserted (UNIQUE on `(integration_id, X-GitHub-Delivery)`).
  - `ReviewService.Trigger` invoked with `IntegrationID / HeadSHA / BaseSHA / ReplyTarget{kind:vcs_pr_thread,…}`.
  - On `EventReviewCompleted`, the mock VCS provider receives `PostSummaryComment` + `PostReviewComments` and the persisted `reviews.summary_comment_id` + `review_findings.inline_comment_id` are populated.
  - On three consecutive provider failures, `EventVCSDeliveryFailed{review_id, op}` is emitted and the FE execution view shows the failure badge.
- Bad signature → 401 + audit `vcs.webhook_signature_invalid`; duplicate delivery → 200 noop without router invocation; auth-expired error → integration status flips and `EventVCSAuthExpired` fires.
- `RouteEvent` for `push` and `pull_request{synchronize}` returns the explicit `ErrPushHandlerNotImplemented` sentinel that Plan 2C will replace.
