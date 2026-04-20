# Spec 2A — VCS Provider Abstraction + GitHub Implementation + Integrations CRUD

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 2 §5 S2-G + §8 vcs.Provider 接口 + §6.1 vcs_integrations 表 + §7 integrations CRUD 端点 + FE 集成管理页。Spec 2 余下所有 plan 的基石。

**Architecture:** Provider-neutral interface（mirror Spec 1 IM card-provider 模式）+ GitHub 实现（go-github v60，PAT 通过 1B secrets 解析）+ GitLab/Gitea stub + mock provider for tests + integrations CRUD + FE 配置页（与 1B secrets 联动选 token / webhook secret）。

**Tech Stack:** Go (`github.com/google/go-github/v60`), Postgres, Next.js 16 App Router, Zustand, shadcn/ui.

**Depends on:** Spec 1B (secrets store for PAT + webhook secret refs)

**Parallel with:** Spec 2B can start once vcs.Provider interface signatures are committed (mock provider satisfies dispatcher tests)

**Unblocks:** 2B, 2C, 2D, 2E (all reference vcs.Provider; 2E uses OpenPR)

---

## Coordination notes (read before starting)

- **Migration number**: 066 is the latest committed (`workflow_run_parent_link_parent_kind`); 1B reserves 067 (`secrets`). This plan uses **068** for `vcs_integrations`. If 1B's number shifts, mechanically renumber.
- **Audit hook**: extend `middleware/rbac.go` ActionID enum with `vcs.integration.*` and add `AuditResourceTypeVCSIntegration = "vcs_integration"` to `model/audit_event.go` + the SQL CHECK constraint extension lives in this migration. Without that the audit insert will fail at the DB layer.
- **Secret resolution at call time only**: PAT plaintext is resolved via `secrets.Service.Resolve` immediately before each outbound GitHub API call and is **never** cached on the provider struct. Mirror 1B §11 plaintext-lifecycle invariant; tests assert no plaintext appears in logs.
- **Public base URL env var**: this plan introduces `AGENTFORGE_PUBLIC_BASE_URL` (e.g. `https://agentforge.acme.corp`) — used to compute the webhook callback URL handed to GitHub. Bootstrap reads it; if unset on integration POST, return `vcs:public_base_url_not_configured` with a clear remediation message.
- **`acting_employee_id` lineage**: every `vcs_integrations` row carries an `acting_employee_id`; this is the employee whose runs will appear in dashboards (Spec 1A) when reviews triggered by this integration finish. Validation: employee must exist + belong to same `project_id`.
- **FE project nav**: `app/(dashboard)/projects/[id]/` is being introduced by 1B (`/secrets`). This plan creates `[id]/integrations/vcs/page.tsx` under the same shell. If 1B is behind schedule, this plan creates a minimal `app/(dashboard)/projects/[id]/layout.tsx` — owned by whichever plan ships first; mark it `// shared with 1B/2A nav` so the next plan absorbs it.
- **Provider registry over global state**: registry is a value type (`vcs.Registry`) wired at server bootstrap, not a package-level mutable singleton. Tests construct a fresh registry per case so parallel tests don't collide.
- **GitLab / Gitea stubs**: registered with the registry so configurations referring to them surface a typed `vcs:provider_unsupported` error rather than panic on `nil` lookup. Stubs return `errors.ErrUnsupported` from every method.

---

## Task 1 — Migration 068 vcs_integrations + audit resource_type extension

- [x] Step 1.1 — write the up migration
  - File: `src-go/migrations/068_create_vcs_integrations.up.sql`
    ```sql
    -- Per-(project, repo) VCS integration. Authoritative store for webhook
    -- registration metadata and the secret-store refs used at outbound time.
    -- Plaintext PAT / webhook_secret are NEVER stored here — only the
    -- secrets.name they map to in the 1B secrets store.
    CREATE TABLE IF NOT EXISTS vcs_integrations (
        id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        provider            VARCHAR(16) NOT NULL,
        host                VARCHAR(256) NOT NULL,
        owner               VARCHAR(128) NOT NULL,
        repo                VARCHAR(128) NOT NULL,
        default_branch      VARCHAR(128) NOT NULL DEFAULT 'main',
        webhook_id          VARCHAR(64),
        webhook_secret_ref  VARCHAR(128) NOT NULL,
        token_secret_ref    VARCHAR(128) NOT NULL,
        status              VARCHAR(16) NOT NULL DEFAULT 'active',
        acting_employee_id  UUID REFERENCES employees(id) ON DELETE SET NULL,
        last_synced_at      TIMESTAMPTZ,
        created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
        UNIQUE (project_id, provider, host, owner, repo)
    );
    CREATE INDEX IF NOT EXISTS vcs_integrations_project_idx ON vcs_integrations(project_id);
    CREATE INDEX IF NOT EXISTS vcs_integrations_status_idx ON vcs_integrations(status);

    CREATE TRIGGER set_vcs_integrations_updated_at
        BEFORE UPDATE ON vcs_integrations
        FOR EACH ROW
        EXECUTE FUNCTION update_updated_at_column();

    -- Extend audit resource_type CHECK so vcs_integration.* events can persist.
    -- (1B added 'secret'; we add 'vcs_integration' on top of that list.)
    ALTER TABLE project_audit_events
        DROP CONSTRAINT IF EXISTS project_audit_events_resource_type_check;
    ALTER TABLE project_audit_events
        ADD CONSTRAINT project_audit_events_resource_type_check
        CHECK (resource_type IN (
            'project','member','task','team_run','workflow',
            'wiki','settings','automation','dashboard','auth',
            'invitation','secret','vcs_integration'
        ));
    ```

- [x] Step 1.2 — write the down migration
  - File: `src-go/migrations/068_create_vcs_integrations.down.sql`
    ```sql
    DROP TRIGGER IF EXISTS set_vcs_integrations_updated_at ON vcs_integrations;
    DROP INDEX IF EXISTS vcs_integrations_status_idx;
    DROP INDEX IF EXISTS vcs_integrations_project_idx;
    DROP TABLE IF EXISTS vcs_integrations;

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

- [x] Step 1.3 — extend audit resource type enum + RBAC ActionIDs
  - File: `src-go/internal/model/audit_event.go`
    - Add `AuditResourceTypeVCSIntegration = "vcs_integration"` to the existing block (after `AuditResourceTypeSecret` from 1B).
    - Append `AuditResourceTypeVCSIntegration` to the matched case list inside `IsValidAuditResourceType`.
  - File: `src-go/internal/middleware/rbac.go`
    - Add ActionIDs:
      ```go
      ActionVCSIntegrationRead   ActionID = "vcs.integration.read"
      ActionVCSIntegrationCreate ActionID = "vcs.integration.create"
      ActionVCSIntegrationUpdate ActionID = "vcs.integration.update"
      ActionVCSIntegrationDelete ActionID = "vcs.integration.delete"
      ActionVCSIntegrationSync   ActionID = "vcs.integration.sync"
      ```
    - Add to `matrix`:
      ```go
      ActionVCSIntegrationRead:   model.ProjectRoleViewer,
      ActionVCSIntegrationCreate: model.ProjectRoleAdmin,
      ActionVCSIntegrationUpdate: model.ProjectRoleAdmin,
      ActionVCSIntegrationDelete: model.ProjectRoleAdmin,
      ActionVCSIntegrationSync:   model.ProjectRoleEditor,
      ```

- [x] Step 1.4 — verify
  - Run `rtk go test ./internal/model/... ./internal/middleware/...` — enum + matrix changes compile.
  - Apply migration locally: `rtk pnpm dev:backend:restart go-orchestrator` and confirm `068_create_vcs_integrations.up.sql` applied.
  - Commit: `feat(vcs): migration 068 vcs_integrations table + audit resource type`

---

## Task 2 — `internal/vcs/types.go` shared types

- [x] Step 2.1 — write failing type compile-check test
  - File: `src-go/internal/vcs/types_test.go`
    ```go
    package vcs_test

    import (
        "testing"

        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    func TestRepoRefStringIsStable(t *testing.T) {
        r := vcs.RepoRef{Host: "github.com", Owner: "octocat", Repo: "hello"}
        if r.String() != "github.com/octocat/hello" {
            t.Errorf("unexpected RepoRef.String(): %q", r.String())
        }
    }

    func TestInlineCommentDefaults(t *testing.T) {
        c := vcs.InlineComment{Path: "a.go", Line: 10, Body: "x"}
        if c.Side != "" {
            t.Errorf("expected zero-value Side, got %q", c.Side)
        }
    }
    ```
  - Run `rtk go test ./internal/vcs/...` — fails (package missing).

- [x] Step 2.2 — implement shared types
  - File: `src-go/internal/vcs/types.go`
    ```go
    // Package vcs is the provider-neutral seam for source-control hosts
    // (GitHub, GitLab, Gitea, ...). Concrete implementations live under
    // internal/vcs/<provider>/. Tests use internal/vcs/mock.
    //
    // Spec reference: docs/superpowers/specs/2026-04-20-code-reviewer-employee-design.md
    //   §5 S2-G architecture, §8 Provider interface, §11 Security.
    package vcs

    import "fmt"

    // RepoRef identifies one repository on one host.
    type RepoRef struct {
        Host  string
        Owner string
        Repo  string
    }

    func (r RepoRef) String() string {
        return fmt.Sprintf("%s/%s/%s", r.Host, r.Owner, r.Repo)
    }

    // PullRequest is the provider-neutral PR snapshot. number is the
    // host-side numeric identifier (#42).
    type PullRequest struct {
        Number     int
        Title      string
        Body       string
        BaseBranch string
        BaseSHA    string
        HeadBranch string
        HeadSHA    string
        State      string // "open" | "closed" | "merged"
        URL        string
        AuthorLogin string
    }

    // Diff is a coarse PR-level diff used by ComparePullRequest. File
    // entries follow GitHub's compare-API shape but the field names are
    // provider-neutral so adapters can populate them directly.
    type Diff struct {
        BaseSHA      string
        HeadSHA      string
        ChangedFiles []ChangedFile
    }

    // ChangedFile is one file's worth of compare-API metadata. patch is
    // the unified-diff hunk for that file (may be empty for binary files
    // or large changes that the host elides).
    type ChangedFile struct {
        Path      string
        Status    string // "added" | "modified" | "removed" | "renamed"
        Additions int
        Deletions int
        Patch     string
    }

    // InlineComment is one PR-line comment. Side is "RIGHT" for added
    // lines and "LEFT" for removed lines (matches GitHub's review-comment
    // semantics; other providers map equivalently).
    type InlineComment struct {
        Path string
        Line int
        Body string
        Side string
    }

    // OpenPROpts holds optional PR-creation flags. Zero value = non-draft,
    // no auto-merge, no labels.
    type OpenPROpts struct {
        Draft     bool
        AutoMerge bool
        Labels    []string
    }
    ```
  - Run `rtk go test ./internal/vcs/...` — passes.

- [x] Step 2.3 — commit: `feat(vcs): shared neutral types (RepoRef, PullRequest, Diff, InlineComment, OpenPROpts)`

---

## Task 3 — `internal/vcs/provider.go` interface + typed errors

- [x] Step 3.1 — write failing interface contract test
  - File: `src-go/internal/vcs/provider_test.go`
    ```go
    package vcs_test

    import (
        "errors"
        "testing"

        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    func TestErrorSentinelsAreDistinct(t *testing.T) {
        if errors.Is(vcs.ErrAuthExpired, vcs.ErrRateLimited) {
            t.Fatal("ErrAuthExpired and ErrRateLimited must be distinct")
        }
        if vcs.ErrAuthExpired.Error() != "vcs:auth_expired" {
            t.Errorf("ErrAuthExpired must serialize as vcs:auth_expired, got %q", vcs.ErrAuthExpired.Error())
        }
        if vcs.ErrRateLimited.Error() != "vcs:rate_limited" {
            t.Errorf("ErrRateLimited must serialize as vcs:rate_limited, got %q", vcs.ErrRateLimited.Error())
        }
        if vcs.ErrTransientFailure.Error() != "vcs:transient_failure" {
            t.Errorf("ErrTransientFailure must serialize as vcs:transient_failure, got %q", vcs.ErrTransientFailure.Error())
        }
    }
    ```
  - Run `rtk go test ./internal/vcs/...` — fails (sentinels missing).

- [x] Step 3.2 — implement interface + typed errors
  - File: `src-go/internal/vcs/provider.go`
    ```go
    package vcs

    import (
        "context"
        "errors"
        "time"
    )

    // Provider is the provider-neutral surface for source-control hosts.
    // Implementations MUST resolve credentials at call time, never cache
    // plaintext PAT on the struct, and translate host-specific errors
    // through the typed sentinels declared below.
    type Provider interface {
        Name() string

        // PR lifecycle.
        GetPullRequest(ctx context.Context, repo RepoRef, number int) (*PullRequest, error)
        ComparePullRequest(ctx context.Context, repo RepoRef, base, head string) (*Diff, error)

        // Summary comment (one per review) — kept editable for diff-of-diff.
        PostSummaryComment(ctx context.Context, pr *PullRequest, body string) (commentID string, err error)
        EditSummaryComment(ctx context.Context, pr *PullRequest, commentID string, body string) error

        // Inline review comments (one per finding).
        PostReviewComments(ctx context.Context, pr *PullRequest, comments []InlineComment) (ids []string, err error)
        EditReviewComment(ctx context.Context, pr *PullRequest, commentID string, body string) error

        // Fix-PR opening (used by 2E).
        OpenPR(ctx context.Context, repo RepoRef, base, head, title, body string, opts OpenPROpts) (*PullRequest, error)

        // Webhook lifecycle.
        CreateWebhook(ctx context.Context, repo RepoRef, callbackURL, secret string, events []string) (id string, err error)
        DeleteWebhook(ctx context.Context, repo RepoRef, id string) error
    }

    // ErrAuthExpired indicates the credential resolved at call time is no
    // longer accepted by the host (401/403). Callers should mark
    // vcs_integrations.status = 'auth_expired' and pause downstream work.
    var ErrAuthExpired = errors.New("vcs:auth_expired")

    // ErrRateLimited indicates the host returned 429. Wrap with
    // RateLimitedError to convey Retry-After hints.
    var ErrRateLimited = errors.New("vcs:rate_limited")

    // ErrTransientFailure indicates a 5xx or network-level failure that the
    // caller should retry with backoff.
    var ErrTransientFailure = errors.New("vcs:transient_failure")

    // ErrProviderUnsupported is returned by the registry when a stubbed
    // provider (gitlab/gitea pre-impl) is requested.
    var ErrProviderUnsupported = errors.New("vcs:provider_unsupported")

    // RateLimitedError wraps ErrRateLimited with the Retry-After hint
    // returned by the host. Callers may use errors.As to extract.
    type RateLimitedError struct {
        RetryAfter time.Duration
    }

    func (e *RateLimitedError) Error() string { return ErrRateLimited.Error() }
    func (e *RateLimitedError) Unwrap() error { return ErrRateLimited }
    ```
  - Run `rtk go test ./internal/vcs/...` — passes.

- [x] Step 3.3 — commit: `feat(vcs): Provider interface + typed sentinel errors`

---

## Task 4 — `internal/vcs/registry.go` provider registry

- [x] Step 4.1 — write failing registry test
  - File: `src-go/internal/vcs/registry_test.go`
    ```go
    package vcs_test

    import (
        "context"
        "errors"
        "testing"

        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    type stubProvider struct{ name string }

    func (s *stubProvider) Name() string { return s.name }
    func (s *stubProvider) GetPullRequest(context.Context, vcs.RepoRef, int) (*vcs.PullRequest, error) {
        return nil, nil
    }
    func (s *stubProvider) ComparePullRequest(context.Context, vcs.RepoRef, string, string) (*vcs.Diff, error) {
        return nil, nil
    }
    func (s *stubProvider) PostSummaryComment(context.Context, *vcs.PullRequest, string) (string, error) {
        return "", nil
    }
    func (s *stubProvider) EditSummaryComment(context.Context, *vcs.PullRequest, string, string) error {
        return nil
    }
    func (s *stubProvider) PostReviewComments(context.Context, *vcs.PullRequest, []vcs.InlineComment) ([]string, error) {
        return nil, nil
    }
    func (s *stubProvider) EditReviewComment(context.Context, *vcs.PullRequest, string, string) error {
        return nil
    }
    func (s *stubProvider) OpenPR(context.Context, vcs.RepoRef, string, string, string, string, vcs.OpenPROpts) (*vcs.PullRequest, error) {
        return nil, nil
    }
    func (s *stubProvider) CreateWebhook(context.Context, vcs.RepoRef, string, string, []string) (string, error) {
        return "", nil
    }
    func (s *stubProvider) DeleteWebhook(context.Context, vcs.RepoRef, string) error { return nil }

    func TestRegistry_RegisterAndResolve(t *testing.T) {
        reg := vcs.NewRegistry()
        reg.Register("github", func(host, token string) (vcs.Provider, error) {
            return &stubProvider{name: "github"}, nil
        })
        p, err := reg.Resolve("github", "github.com", "tok")
        if err != nil {
            t.Fatalf("Resolve: %v", err)
        }
        if p.Name() != "github" {
            t.Errorf("expected name=github, got %q", p.Name())
        }
    }

    func TestRegistry_UnknownProvider(t *testing.T) {
        reg := vcs.NewRegistry()
        _, err := reg.Resolve("svn", "x", "")
        if !errors.Is(err, vcs.ErrProviderUnsupported) {
            t.Fatalf("expected ErrProviderUnsupported, got %v", err)
        }
    }

    func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
        reg := vcs.NewRegistry()
        reg.Register("x", func(string, string) (vcs.Provider, error) { return nil, nil })
        defer func() {
            if r := recover(); r == nil {
                t.Fatal("expected panic on duplicate Register")
            }
        }()
        reg.Register("x", func(string, string) (vcs.Provider, error) { return nil, nil })
    }
    ```
  - Run `rtk go test ./internal/vcs/...` — fails (registry missing).

- [x] Step 4.2 — implement registry
  - File: `src-go/internal/vcs/registry.go`
    ```go
    package vcs

    import (
        "fmt"
        "sync"
    )

    // Constructor builds a Provider for a given (host, token). token is
    // the resolved-at-call-time PAT plaintext from the 1B secrets store
    // — it MUST NOT be cached on the returned Provider value beyond the
    // single outbound request being constructed.
    type Constructor func(host, token string) (Provider, error)

    // Registry maps provider names ("github", "gitlab", ...) to their
    // constructors. Wired once at server bootstrap. Tests build a fresh
    // Registry per case to avoid global mutation.
    type Registry struct {
        mu  sync.RWMutex
        ctors map[string]Constructor
    }

    func NewRegistry() *Registry {
        return &Registry{ctors: map[string]Constructor{}}
    }

    // Register installs ctor under name. Panics on duplicate to surface
    // wiring bugs at boot rather than silently shadowing.
    func (r *Registry) Register(name string, ctor Constructor) {
        r.mu.Lock()
        defer r.mu.Unlock()
        if _, exists := r.ctors[name]; exists {
            panic(fmt.Sprintf("vcs: provider %q already registered", name))
        }
        r.ctors[name] = ctor
    }

    // Resolve constructs a Provider for the given configuration. Returns
    // ErrProviderUnsupported if no constructor is registered for name.
    func (r *Registry) Resolve(name, host, token string) (Provider, error) {
        r.mu.RLock()
        ctor, ok := r.ctors[name]
        r.mu.RUnlock()
        if !ok {
            return nil, fmt.Errorf("%w: %s", ErrProviderUnsupported, name)
        }
        return ctor(host, token)
    }

    // Names returns registered provider names (sorted for stable FE display).
    func (r *Registry) Names() []string {
        r.mu.RLock()
        defer r.mu.RUnlock()
        out := make([]string, 0, len(r.ctors))
        for n := range r.ctors {
            out = append(out, n)
        }
        return out
    }
    ```
  - Run `rtk go test ./internal/vcs/...` — passes.

- [x] Step 4.3 — commit: `feat(vcs): provider registry with typed Constructor seam`

---

## Task 5 — `internal/vcs/mock` recording provider for tests

- [x] Step 5.1 — write failing recording test
  - File: `src-go/internal/vcs/mock/provider_test.go`
    ```go
    package mock_test

    import (
        "context"
        "testing"

        "github.com/react-go-quick-starter/server/internal/vcs"
        "github.com/react-go-quick-starter/server/internal/vcs/mock"
    )

    func TestMockProvider_RecordsCalls(t *testing.T) {
        p := mock.New()
        ctx := context.Background()
        repo := vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}

        if _, err := p.PostSummaryComment(ctx, &vcs.PullRequest{Number: 7}, "hello"); err != nil {
            t.Fatalf("PostSummaryComment: %v", err)
        }
        _, _ = p.PostReviewComments(ctx, &vcs.PullRequest{Number: 7}, []vcs.InlineComment{{Path: "a.go", Line: 10, Body: "x", Side: "RIGHT"}})
        _, _ = p.OpenPR(ctx, repo, "main", "fix/abc", "title", "body", vcs.OpenPROpts{Labels: []string{"agentforge:fix"}})

        calls := p.Calls()
        if len(calls) != 3 {
            t.Fatalf("expected 3 calls, got %d", len(calls))
        }
        if calls[0].Op != "PostSummaryComment" || calls[0].Args["body"] != "hello" {
            t.Errorf("call[0] mismatch: %+v", calls[0])
        }
        if calls[2].Op != "OpenPR" {
            t.Errorf("call[2] expected OpenPR, got %s", calls[2].Op)
        }
    }

    func TestMockProvider_ScriptedError(t *testing.T) {
        p := mock.New()
        p.NextError(vcs.ErrAuthExpired)
        if _, err := p.PostSummaryComment(context.Background(), &vcs.PullRequest{Number: 1}, "x"); err == nil {
            t.Fatal("expected scripted error")
        } else if err != vcs.ErrAuthExpired {
            t.Errorf("expected ErrAuthExpired, got %v", err)
        }
    }
    ```
  - Run `rtk go test ./internal/vcs/mock/...` — fails (package missing).

- [x] Step 5.2 — implement mock provider
  - File: `src-go/internal/vcs/mock/provider.go`
    ```go
    // Package mock supplies a recording vcs.Provider for tests. Every
    // outbound method appends a Call to the recording so tests can assert
    // on the full interaction sequence. Use NextError to script a single
    // failure for the next call (resets after one consumption).
    package mock

    import (
        "context"
        "sync"
        "sync/atomic"

        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    // Call captures one provider invocation.
    type Call struct {
        Op   string
        Args map[string]any
    }

    // Provider is a recording, in-memory vcs.Provider.
    type Provider struct {
        mu       sync.Mutex
        calls    []Call
        nextErr  atomic.Value // error
        commentID atomic.Int64
        prID      atomic.Int64
    }

    // New returns an empty recorder.
    func New() *Provider { return &Provider{} }

    // Calls returns a snapshot of recorded calls.
    func (p *Provider) Calls() []Call {
        p.mu.Lock()
        defer p.mu.Unlock()
        out := make([]Call, len(p.calls))
        copy(out, p.calls)
        return out
    }

    // NextError scripts err to be returned by the next outbound call.
    func (p *Provider) NextError(err error) { p.nextErr.Store(err) }

    func (p *Provider) consumeErr() error {
        v := p.nextErr.Swap(error(nil))
        if v == nil {
            return nil
        }
        if e, ok := v.(error); ok {
            return e
        }
        return nil
    }

    func (p *Provider) record(op string, args map[string]any) {
        p.mu.Lock()
        defer p.mu.Unlock()
        p.calls = append(p.calls, Call{Op: op, Args: args})
    }

    func (p *Provider) Name() string { return "mock" }

    func (p *Provider) GetPullRequest(ctx context.Context, repo vcs.RepoRef, n int) (*vcs.PullRequest, error) {
        p.record("GetPullRequest", map[string]any{"repo": repo.String(), "n": n})
        if err := p.consumeErr(); err != nil {
            return nil, err
        }
        return &vcs.PullRequest{Number: n, BaseBranch: "main", BaseSHA: "base", HeadSHA: "head", State: "open", URL: "https://mock/pr/" + repo.String()}, nil
    }

    func (p *Provider) ComparePullRequest(ctx context.Context, repo vcs.RepoRef, base, head string) (*vcs.Diff, error) {
        p.record("ComparePullRequest", map[string]any{"repo": repo.String(), "base": base, "head": head})
        if err := p.consumeErr(); err != nil {
            return nil, err
        }
        return &vcs.Diff{BaseSHA: base, HeadSHA: head}, nil
    }

    func (p *Provider) PostSummaryComment(ctx context.Context, pr *vcs.PullRequest, body string) (string, error) {
        p.record("PostSummaryComment", map[string]any{"pr": pr.Number, "body": body})
        if err := p.consumeErr(); err != nil {
            return "", err
        }
        id := p.commentID.Add(1)
        return "summary-" + itoa(id), nil
    }

    func (p *Provider) EditSummaryComment(ctx context.Context, pr *vcs.PullRequest, id, body string) error {
        p.record("EditSummaryComment", map[string]any{"pr": pr.Number, "id": id, "body": body})
        return p.consumeErr()
    }

    func (p *Provider) PostReviewComments(ctx context.Context, pr *vcs.PullRequest, comments []vcs.InlineComment) ([]string, error) {
        p.record("PostReviewComments", map[string]any{"pr": pr.Number, "count": len(comments)})
        if err := p.consumeErr(); err != nil {
            return nil, err
        }
        out := make([]string, len(comments))
        for i := range comments {
            out[i] = "inline-" + itoa(p.commentID.Add(1))
        }
        return out, nil
    }

    func (p *Provider) EditReviewComment(ctx context.Context, pr *vcs.PullRequest, id, body string) error {
        p.record("EditReviewComment", map[string]any{"pr": pr.Number, "id": id, "body": body})
        return p.consumeErr()
    }

    func (p *Provider) OpenPR(ctx context.Context, repo vcs.RepoRef, base, head, title, body string, opts vcs.OpenPROpts) (*vcs.PullRequest, error) {
        p.record("OpenPR", map[string]any{"repo": repo.String(), "base": base, "head": head, "title": title, "labels": opts.Labels})
        if err := p.consumeErr(); err != nil {
            return nil, err
        }
        n := int(p.prID.Add(1))
        return &vcs.PullRequest{Number: n, Title: title, BaseBranch: base, HeadBranch: head, State: "open", URL: "https://mock/pr/" + itoa(int64(n))}, nil
    }

    func (p *Provider) CreateWebhook(ctx context.Context, repo vcs.RepoRef, cb, secret string, events []string) (string, error) {
        // never record raw secret value
        p.record("CreateWebhook", map[string]any{"repo": repo.String(), "callback": cb, "events": events})
        if err := p.consumeErr(); err != nil {
            return "", err
        }
        return "hook-" + itoa(p.commentID.Add(1)), nil
    }

    func (p *Provider) DeleteWebhook(ctx context.Context, repo vcs.RepoRef, id string) error {
        p.record("DeleteWebhook", map[string]any{"repo": repo.String(), "id": id})
        return p.consumeErr()
    }

    func itoa(n int64) string {
        // tiny inline to avoid pulling strconv into a hot path
        if n == 0 {
            return "0"
        }
        var buf [20]byte
        i := len(buf)
        neg := n < 0
        if neg {
            n = -n
        }
        for n > 0 {
            i--
            buf[i] = byte('0' + n%10)
            n /= 10
        }
        if neg {
            i--
            buf[i] = '-'
        }
        return string(buf[i:])
    }
    ```
  - Run `rtk go test ./internal/vcs/mock/...` — passes.

- [x] Step 5.3 — commit: `feat(vcs): mock recording provider for tests`

---

## Task 6 — Add `go-github` dep + `internal/vcs/github/client.go` GitHub provider

- [x] Step 6.1 — add dep
  - Run: `cd src-go && go get github.com/google/go-github/v60/github && go mod tidy` (use `rtk` wrapper).
  - Verify `go.mod` now lists `github.com/google/go-github/v60` and `golang.org/x/oauth2` (transitive — accept).

- [x] Step 6.2 — write failing GitHub client test using `httptest`
  - File: `src-go/internal/vcs/github/client_test.go`
    ```go
    package github_test

    import (
        "context"
        "encoding/json"
        "errors"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"

        "github.com/react-go-quick-starter/server/internal/vcs"
        ghimpl "github.com/react-go-quick-starter/server/internal/vcs/github"
    )

    func newServer(t *testing.T, h http.Handler) (*httptest.Server, *ghimpl.Client) {
        t.Helper()
        srv := httptest.NewServer(h)
        t.Cleanup(srv.Close)
        c, err := ghimpl.NewClient(srv.URL+"/", "test-pat")
        if err != nil {
            t.Fatalf("NewClient: %v", err)
        }
        return srv, c
    }

    func TestGetPullRequest_Happy(t *testing.T) {
        _, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !strings.HasSuffix(r.URL.Path, "/repos/o/r/pulls/42") {
                t.Errorf("unexpected path: %s", r.URL.Path)
            }
            if got := r.Header.Get("Authorization"); got != "Bearer test-pat" {
                t.Errorf("expected bearer auth, got %q", got)
            }
            _ = json.NewEncoder(w).Encode(map[string]any{
                "number": 42, "title": "T", "state": "open",
                "html_url": "https://github.com/o/r/pull/42",
                "base":     map[string]any{"ref": "main", "sha": "BASE"},
                "head":     map[string]any{"ref": "feat", "sha": "HEAD"},
                "user":     map[string]any{"login": "alice"},
            })
        }))
        pr, err := c.GetPullRequest(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}, 42)
        if err != nil {
            t.Fatalf("GetPullRequest: %v", err)
        }
        if pr.Number != 42 || pr.BaseSHA != "BASE" || pr.HeadSHA != "HEAD" || pr.AuthorLogin != "alice" {
            t.Errorf("unexpected PR mapping: %+v", pr)
        }
    }

    func TestGetPullRequest_AuthExpiredMaps401(t *testing.T) {
        _, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusUnauthorized)
            _, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
        }))
        _, err := c.GetPullRequest(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}, 1)
        if !errors.Is(err, vcs.ErrAuthExpired) {
            t.Fatalf("expected ErrAuthExpired, got %v", err)
        }
    }

    func TestPostSummaryComment_ReturnsID(t *testing.T) {
        _, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusCreated)
            _ = json.NewEncoder(w).Encode(map[string]any{"id": 9911})
        }))
        id, err := c.PostSummaryComment(context.Background(), &vcs.PullRequest{Number: 42, URL: "https://github.com/o/r/pull/42"}, "summary")
        if err != nil {
            t.Fatalf("PostSummaryComment: %v", err)
        }
        if id != "9911" {
            t.Errorf("expected id=9911, got %q", id)
        }
    }

    func TestRateLimitedMaps429WithRetryAfter(t *testing.T) {
        _, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Retry-After", "37")
            w.WriteHeader(http.StatusTooManyRequests)
        }))
        _, err := c.GetPullRequest(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}, 1)
        if !errors.Is(err, vcs.ErrRateLimited) {
            t.Fatalf("expected ErrRateLimited, got %v", err)
        }
        var rl *vcs.RateLimitedError
        if !errors.As(err, &rl) {
            t.Fatalf("expected RateLimitedError, got %v", err)
        }
        if rl.RetryAfter.Seconds() != 37 {
            t.Errorf("expected 37s RetryAfter, got %v", rl.RetryAfter)
        }
    }

    func TestCreateWebhook_PassesSecret(t *testing.T) {
        var bodyJSON map[string]any
        _, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            _ = json.NewDecoder(r.Body).Decode(&bodyJSON)
            w.WriteHeader(http.StatusCreated)
            _ = json.NewEncoder(w).Encode(map[string]any{"id": 4242})
        }))
        id, err := c.CreateWebhook(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"},
            "https://agentforge.acme.corp/api/v1/vcs/github/webhook", "shh", []string{"pull_request", "push"})
        if err != nil {
            t.Fatalf("CreateWebhook: %v", err)
        }
        if id != "4242" {
            t.Errorf("expected hook id 4242, got %q", id)
        }
        cfg, _ := bodyJSON["config"].(map[string]any)
        if cfg["secret"] != "shh" || cfg["url"] == "" {
            t.Errorf("expected config to carry secret + url; got %+v", cfg)
        }
    }
    ```
  - Run `rtk go test ./internal/vcs/github/...` — fails (client missing).

- [x] Step 6.3 — implement GitHub client
  - File: `src-go/internal/vcs/github/client.go`
    ```go
    // Package github implements vcs.Provider for GitHub.com and GitHub
    // Enterprise. The constructor takes a base URL (so ghimpl works for
    // both github.com and self-hosted) and a PAT plaintext. The PAT is
    // resolved at the call site immediately before construction; this
    // package MUST NOT cache it beyond the lifetime of the returned Client.
    package github

    import (
        "context"
        "errors"
        "fmt"
        "net/http"
        "strconv"
        "strings"
        "time"

        gogh "github.com/google/go-github/v60/github"

        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    // Client wraps go-github's *Client with vcs.Provider semantics.
    type Client struct {
        gh *gogh.Client
    }

    // NewClient builds a Client. baseURL may be "" for github.com, or a
    // GitHub Enterprise base ("https://github.acme.corp/api/v3/"). pat is
    // the resolved PAT; the constructor wires it via a Bearer token
    // round-tripper so every request carries Authorization.
    func NewClient(baseURL, pat string) (*Client, error) {
        httpClient := &http.Client{
            Timeout: 30 * time.Second,
            Transport: &bearerTransport{token: pat, base: http.DefaultTransport},
        }
        gh := gogh.NewClient(httpClient)
        if baseURL != "" && baseURL != "https://api.github.com/" {
            // Enterprise: same host serves API + uploads
            var err error
            gh, err = gh.WithEnterpriseURLs(baseURL, baseURL)
            if err != nil {
                return nil, fmt.Errorf("github: enterprise URL: %w", err)
            }
        }
        return &Client{gh: gh}, nil
    }

    type bearerTransport struct {
        token string
        base  http.RoundTripper
    }

    func (b *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
        req2 := req.Clone(req.Context())
        req2.Header.Set("Authorization", "Bearer "+b.token)
        req2.Header.Set("Accept", "application/vnd.github+json")
        return b.base.RoundTrip(req2)
    }

    func (c *Client) Name() string { return "github" }

    func (c *Client) GetPullRequest(ctx context.Context, repo vcs.RepoRef, n int) (*vcs.PullRequest, error) {
        pr, _, err := c.gh.PullRequests.Get(ctx, repo.Owner, repo.Repo, n)
        if err != nil {
            return nil, mapErr(err)
        }
        return toPR(pr), nil
    }

    func (c *Client) ComparePullRequest(ctx context.Context, repo vcs.RepoRef, base, head string) (*vcs.Diff, error) {
        cmp, _, err := c.gh.Repositories.CompareCommits(ctx, repo.Owner, repo.Repo, base, head, nil)
        if err != nil {
            return nil, mapErr(err)
        }
        out := &vcs.Diff{BaseSHA: cmp.GetBaseCommit().GetSHA(), HeadSHA: cmp.GetMergeBaseCommit().GetSHA()}
        for _, f := range cmp.Files {
            out.ChangedFiles = append(out.ChangedFiles, vcs.ChangedFile{
                Path: f.GetFilename(), Status: f.GetStatus(),
                Additions: f.GetAdditions(), Deletions: f.GetDeletions(),
                Patch: f.GetPatch(),
            })
        }
        return out, nil
    }

    func (c *Client) PostSummaryComment(ctx context.Context, pr *vcs.PullRequest, body string) (string, error) {
        owner, repo, err := splitURL(pr.URL)
        if err != nil {
            return "", err
        }
        com, _, err := c.gh.Issues.CreateComment(ctx, owner, repo, pr.Number, &gogh.IssueComment{Body: gogh.String(body)})
        if err != nil {
            return "", mapErr(err)
        }
        return strconv.FormatInt(com.GetID(), 10), nil
    }

    func (c *Client) EditSummaryComment(ctx context.Context, pr *vcs.PullRequest, commentID, body string) error {
        owner, repo, err := splitURL(pr.URL)
        if err != nil {
            return err
        }
        id, err := strconv.ParseInt(commentID, 10, 64)
        if err != nil {
            return fmt.Errorf("github: invalid commentID %q", commentID)
        }
        _, _, err = c.gh.Issues.EditComment(ctx, owner, repo, id, &gogh.IssueComment{Body: gogh.String(body)})
        return mapErr(err)
    }

    func (c *Client) PostReviewComments(ctx context.Context, pr *vcs.PullRequest, comments []vcs.InlineComment) ([]string, error) {
        owner, repo, err := splitURL(pr.URL)
        if err != nil {
            return nil, err
        }
        ids := make([]string, 0, len(comments))
        for _, ic := range comments {
            side := ic.Side
            if side == "" {
                side = "RIGHT"
            }
            review, _, err := c.gh.PullRequests.CreateComment(ctx, owner, repo, pr.Number, &gogh.PullRequestComment{
                CommitID: gogh.String(pr.HeadSHA),
                Path:     gogh.String(ic.Path),
                Line:     gogh.Int(ic.Line),
                Side:     gogh.String(side),
                Body:     gogh.String(ic.Body),
            })
            if err != nil {
                return ids, mapErr(err)
            }
            ids = append(ids, strconv.FormatInt(review.GetID(), 10))
        }
        return ids, nil
    }

    func (c *Client) EditReviewComment(ctx context.Context, pr *vcs.PullRequest, commentID, body string) error {
        owner, repo, err := splitURL(pr.URL)
        if err != nil {
            return err
        }
        id, err := strconv.ParseInt(commentID, 10, 64)
        if err != nil {
            return fmt.Errorf("github: invalid commentID %q", commentID)
        }
        _, _, err = c.gh.PullRequests.EditComment(ctx, owner, repo, id, &gogh.PullRequestComment{Body: gogh.String(body)})
        return mapErr(err)
    }

    func (c *Client) OpenPR(ctx context.Context, repo vcs.RepoRef, base, head, title, body string, opts vcs.OpenPROpts) (*vcs.PullRequest, error) {
        pr, _, err := c.gh.PullRequests.Create(ctx, repo.Owner, repo.Repo, &gogh.NewPullRequest{
            Title: gogh.String(title), Body: gogh.String(body),
            Base: gogh.String(base), Head: gogh.String(head),
            Draft: gogh.Bool(opts.Draft),
        })
        if err != nil {
            return nil, mapErr(err)
        }
        if len(opts.Labels) > 0 {
            _, _, _ = c.gh.Issues.AddLabelsToIssue(ctx, repo.Owner, repo.Repo, pr.GetNumber(), opts.Labels)
        }
        return toPR(pr), nil
    }

    func (c *Client) CreateWebhook(ctx context.Context, repo vcs.RepoRef, callbackURL, secret string, events []string) (string, error) {
        hook, _, err := c.gh.Repositories.CreateHook(ctx, repo.Owner, repo.Repo, &gogh.Hook{
            Name:   gogh.String("web"),
            Active: gogh.Bool(true),
            Events: events,
            Config: map[string]interface{}{
                "url":          callbackURL,
                "content_type": "json",
                "secret":       secret,
                "insecure_ssl": "0",
            },
        })
        if err != nil {
            return "", mapErr(err)
        }
        return strconv.FormatInt(hook.GetID(), 10), nil
    }

    func (c *Client) DeleteWebhook(ctx context.Context, repo vcs.RepoRef, id string) error {
        n, err := strconv.ParseInt(id, 10, 64)
        if err != nil {
            return fmt.Errorf("github: invalid webhook id %q", id)
        }
        _, err = c.gh.Repositories.DeleteHook(ctx, repo.Owner, repo.Repo, n)
        return mapErr(err)
    }

    // ---------- helpers ----------

    func toPR(pr *gogh.PullRequest) *vcs.PullRequest {
        out := &vcs.PullRequest{
            Number: pr.GetNumber(), Title: pr.GetTitle(), Body: pr.GetBody(),
            URL: pr.GetHTMLURL(), State: pr.GetState(),
            BaseBranch: pr.GetBase().GetRef(), BaseSHA: pr.GetBase().GetSHA(),
            HeadBranch: pr.GetHead().GetRef(), HeadSHA: pr.GetHead().GetSHA(),
            AuthorLogin: pr.GetUser().GetLogin(),
        }
        if pr.GetMerged() {
            out.State = "merged"
        }
        return out
    }

    // splitURL extracts (owner, repo) from a PR HTML URL like
    // https://github.com/octocat/hello/pull/42. We accept enterprise
    // hosts too.
    func splitURL(u string) (string, string, error) {
        // strip scheme
        i := strings.Index(u, "://")
        if i < 0 {
            return "", "", fmt.Errorf("github: bad PR URL %q", u)
        }
        rest := u[i+3:]
        parts := strings.Split(rest, "/")
        if len(parts) < 4 {
            return "", "", fmt.Errorf("github: bad PR URL %q", u)
        }
        // parts: [host, owner, repo, "pull", num]
        return parts[1], parts[2], nil
    }

    // mapErr translates go-github errors into vcs typed sentinels. Any
    // unmapped err is returned as-is so callers can inspect.
    func mapErr(err error) error {
        if err == nil {
            return nil
        }
        var rerr *gogh.ErrorResponse
        if errors.As(err, &rerr) && rerr.Response != nil {
            switch rerr.Response.StatusCode {
            case http.StatusUnauthorized, http.StatusForbidden:
                return vcs.ErrAuthExpired
            case http.StatusTooManyRequests:
                ra := rerr.Response.Header.Get("Retry-After")
                d, _ := strconv.Atoi(ra)
                return &vcs.RateLimitedError{RetryAfter: time.Duration(d) * time.Second}
            }
            if rerr.Response.StatusCode >= 500 {
                return fmt.Errorf("%w: %s", vcs.ErrTransientFailure, rerr.Message)
            }
        }
        return err
    }
    ```
  - Run `rtk go test ./internal/vcs/github/...` — passes.

- [x] Step 6.4 — commit: `feat(vcs/github): GitHub provider via go-github v60 with typed error mapping`

---

## Task 7 — `internal/vcs/gitlab` + `internal/vcs/gitea` stubs

- [x] Step 7.1 — write failing stub test (one for each)
  - File: `src-go/internal/vcs/gitlab/stub_test.go`
    ```go
    package gitlab_test

    import (
        "context"
        "errors"
        "testing"

        "github.com/react-go-quick-starter/server/internal/vcs"
        "github.com/react-go-quick-starter/server/internal/vcs/gitlab"
    )

    func TestStubReturnsUnsupportedFromEveryMethod(t *testing.T) {
        s, err := gitlab.NewStub("gitlab.com", "tok")
        if err != nil {
            t.Fatalf("NewStub: %v", err)
        }
        _, err = s.GetPullRequest(context.Background(), vcs.RepoRef{}, 1)
        if !errors.Is(err, errors.ErrUnsupported) {
            t.Errorf("expected ErrUnsupported, got %v", err)
        }
    }
    ```
  - File: `src-go/internal/vcs/gitea/stub_test.go` — analogous (replace package + import path).
  - Run `rtk go test ./internal/vcs/gitlab/... ./internal/vcs/gitea/...` — fails.

- [x] Step 7.2 — implement stubs
  - File: `src-go/internal/vcs/gitlab/stub.go`
    ```go
    // Package gitlab is a placeholder implementation of vcs.Provider for
    // GitLab. Every method returns errors.ErrUnsupported. The stub is
    // registered with the vcs.Registry so configurations referring to
    // "gitlab" surface a clean error rather than a nil-pointer panic.
    package gitlab

    import (
        "context"
        "errors"

        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    // Stub satisfies vcs.Provider. All methods return errors.ErrUnsupported.
    type Stub struct{ host string }

    // NewStub is the registry Constructor entry point.
    func NewStub(host, _ string) (vcs.Provider, error) { return &Stub{host: host}, nil }

    func (s *Stub) Name() string { return "gitlab" }

    func (s *Stub) GetPullRequest(context.Context, vcs.RepoRef, int) (*vcs.PullRequest, error) {
        return nil, errors.ErrUnsupported
    }
    func (s *Stub) ComparePullRequest(context.Context, vcs.RepoRef, string, string) (*vcs.Diff, error) {
        return nil, errors.ErrUnsupported
    }
    func (s *Stub) PostSummaryComment(context.Context, *vcs.PullRequest, string) (string, error) {
        return "", errors.ErrUnsupported
    }
    func (s *Stub) EditSummaryComment(context.Context, *vcs.PullRequest, string, string) error {
        return errors.ErrUnsupported
    }
    func (s *Stub) PostReviewComments(context.Context, *vcs.PullRequest, []vcs.InlineComment) ([]string, error) {
        return nil, errors.ErrUnsupported
    }
    func (s *Stub) EditReviewComment(context.Context, *vcs.PullRequest, string, string) error {
        return errors.ErrUnsupported
    }
    func (s *Stub) OpenPR(context.Context, vcs.RepoRef, string, string, string, string, vcs.OpenPROpts) (*vcs.PullRequest, error) {
        return nil, errors.ErrUnsupported
    }
    func (s *Stub) CreateWebhook(context.Context, vcs.RepoRef, string, string, []string) (string, error) {
        return "", errors.ErrUnsupported
    }
    func (s *Stub) DeleteWebhook(context.Context, vcs.RepoRef, string) error {
        return errors.ErrUnsupported
    }
    ```
  - File: `src-go/internal/vcs/gitea/stub.go` — identical structure with package `gitea` and `Name() string { return "gitea" }`.
  - Run `rtk go test ./internal/vcs/gitlab/... ./internal/vcs/gitea/...` — passes.

- [x] Step 7.3 — commit: `feat(vcs): gitlab + gitea stubs returning ErrUnsupported`

---

## Task 8 — `internal/repository/vcs_integration_repo.go` persistence

- [x] Step 8.1 — write failing repo test (matches existing repo conventions)
  - File: `src-go/internal/repository/vcs_integration_repo_test.go`
    ```go
    package repository_test

    import (
        "context"
        "testing"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/repository"
    )

    func TestVCSIntegrationRepo_CreateGetUpdateDelete(t *testing.T) {
        db := setupTestDB(t) // helper from foundation_repo_test_helpers_test.go
        repo := repository.NewVCSIntegrationRepo(db)
        ctx := context.Background()
        proj := seedProject(t, db)

        rec := &model.VCSIntegration{
            ID: uuid.New(), ProjectID: proj.ID,
            Provider: "github", Host: "github.com", Owner: "octocat", Repo: "hello",
            DefaultBranch: "main",
            WebhookSecretRef: "vcs.github.octocat-hello.webhook",
            TokenSecretRef:   "vcs.github.octocat-hello.pat",
            Status:           "active",
        }
        if err := repo.Create(ctx, rec); err != nil {
            t.Fatalf("Create: %v", err)
        }

        got, err := repo.Get(ctx, rec.ID)
        if err != nil || got.Repo != "hello" {
            t.Fatalf("Get: %v %+v", err, got)
        }

        list, err := repo.ListByProject(ctx, proj.ID)
        if err != nil || len(list) != 1 {
            t.Fatalf("ListByProject: %v len=%d", err, len(list))
        }

        rec.Status = "auth_expired"
        if err := repo.Update(ctx, rec); err != nil {
            t.Fatalf("Update: %v", err)
        }
        got, _ = repo.Get(ctx, rec.ID)
        if got.Status != "auth_expired" {
            t.Errorf("expected status update to persist; got %q", got.Status)
        }

        if err := repo.Delete(ctx, rec.ID); err != nil {
            t.Fatalf("Delete: %v", err)
        }
        if _, err := repo.Get(ctx, rec.ID); err == nil {
            t.Fatal("expected ErrNotFound after delete")
        }
    }

    func TestVCSIntegrationRepo_UniqueConflict(t *testing.T) {
        db := setupTestDB(t)
        repo := repository.NewVCSIntegrationRepo(db)
        ctx := context.Background()
        proj := seedProject(t, db)

        a := &model.VCSIntegration{
            ID: uuid.New(), ProjectID: proj.ID,
            Provider: "github", Host: "github.com", Owner: "o", Repo: "r",
            WebhookSecretRef: "w", TokenSecretRef: "t",
            DefaultBranch: "main", Status: "active",
        }
        if err := repo.Create(ctx, a); err != nil {
            t.Fatalf("Create a: %v", err)
        }
        b := *a
        b.ID = uuid.New()
        if err := repo.Create(ctx, &b); err == nil {
            t.Fatal("expected unique-constraint violation on duplicate (project,provider,host,owner,repo)")
        }
    }
    ```
  - Run `rtk go test ./internal/repository/...` (-run VCSIntegration) — fails.

- [x] Step 8.2 — implement model + repo
  - File: `src-go/internal/model/vcs_integration.go`
    ```go
    package model

    import (
        "time"

        "github.com/google/uuid"
    )

    // VCSIntegration is one (project, repo) link to a source-control host.
    // Plaintext PAT and webhook secret live in the 1B secrets store —
    // this row only carries the secrets.name refs.
    type VCSIntegration struct {
        ID                uuid.UUID  `json:"id" gorm:"column:id;primaryKey"`
        ProjectID         uuid.UUID  `json:"projectId" gorm:"column:project_id"`
        Provider          string     `json:"provider" gorm:"column:provider"`
        Host              string     `json:"host" gorm:"column:host"`
        Owner             string     `json:"owner" gorm:"column:owner"`
        Repo              string     `json:"repo" gorm:"column:repo"`
        DefaultBranch     string     `json:"defaultBranch" gorm:"column:default_branch"`
        WebhookID         *string    `json:"webhookId,omitempty" gorm:"column:webhook_id"`
        WebhookSecretRef  string     `json:"webhookSecretRef" gorm:"column:webhook_secret_ref"`
        TokenSecretRef    string     `json:"tokenSecretRef" gorm:"column:token_secret_ref"`
        Status            string     `json:"status" gorm:"column:status"`
        ActingEmployeeID  *uuid.UUID `json:"actingEmployeeId,omitempty" gorm:"column:acting_employee_id"`
        LastSyncedAt      *time.Time `json:"lastSyncedAt,omitempty" gorm:"column:last_synced_at"`
        CreatedAt         time.Time  `json:"createdAt" gorm:"column:created_at"`
        UpdatedAt         time.Time  `json:"updatedAt" gorm:"column:updated_at"`
    }

    func (VCSIntegration) TableName() string { return "vcs_integrations" }
    ```
  - File: `src-go/internal/repository/vcs_integration_repo.go`
    ```go
    package repository

    import (
        "context"
        "errors"
        "time"

        "github.com/google/uuid"
        "gorm.io/gorm"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    // VCSIntegrationRepo is the persistence seam for vcs_integrations.
    type VCSIntegrationRepo struct{ db *gorm.DB }

    func NewVCSIntegrationRepo(db *gorm.DB) *VCSIntegrationRepo { return &VCSIntegrationRepo{db: db} }

    func (r *VCSIntegrationRepo) Create(ctx context.Context, rec *model.VCSIntegration) error {
        if rec.ID == uuid.Nil {
            rec.ID = uuid.New()
        }
        now := time.Now().UTC()
        if rec.CreatedAt.IsZero() {
            rec.CreatedAt = now
        }
        rec.UpdatedAt = now
        return r.db.WithContext(ctx).Create(rec).Error
    }

    func (r *VCSIntegrationRepo) Get(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error) {
        var rec model.VCSIntegration
        if err := r.db.WithContext(ctx).First(&rec, "id = ?", id).Error; err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return nil, ErrNotFound
            }
            return nil, err
        }
        return &rec, nil
    }

    func (r *VCSIntegrationRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error) {
        var rows []*model.VCSIntegration
        err := r.db.WithContext(ctx).
            Where("project_id = ?", projectID).
            Order("created_at DESC").
            Find(&rows).Error
        return rows, err
    }

    func (r *VCSIntegrationRepo) FindByRepo(ctx context.Context, host, owner, repo string) ([]*model.VCSIntegration, error) {
        var rows []*model.VCSIntegration
        err := r.db.WithContext(ctx).
            Where("host = ? AND owner = ? AND repo = ?", host, owner, repo).
            Find(&rows).Error
        return rows, err
    }

    func (r *VCSIntegrationRepo) Update(ctx context.Context, rec *model.VCSIntegration) error {
        rec.UpdatedAt = time.Now().UTC()
        res := r.db.WithContext(ctx).
            Model(&model.VCSIntegration{}).
            Where("id = ?", rec.ID).
            Updates(map[string]any{
                "status":              rec.Status,
                "token_secret_ref":    rec.TokenSecretRef,
                "webhook_secret_ref":  rec.WebhookSecretRef,
                "webhook_id":          rec.WebhookID,
                "acting_employee_id":  rec.ActingEmployeeID,
                "default_branch":      rec.DefaultBranch,
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

    func (r *VCSIntegrationRepo) Delete(ctx context.Context, id uuid.UUID) error {
        res := r.db.WithContext(ctx).Delete(&model.VCSIntegration{}, "id = ?", id)
        if res.Error != nil {
            return res.Error
        }
        if res.RowsAffected == 0 {
            return ErrNotFound
        }
        return nil
    }
    ```
  - Run `rtk go test ./internal/repository/...` (-run VCSIntegration) — passes.

- [x] Step 8.3 — commit: `feat(repo): vcs_integrations CRUD with unique-conflict mapping`

---

## Task 9 — `internal/vcs/service.go` integration service (validates + creates webhook)

- [ ] Step 9.1 — write failing service test against the mock provider
  - File: `src-go/internal/vcs/service_test.go`
    ```go
    package vcs_test

    import (
        "context"
        "testing"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/vcs"
        "github.com/react-go-quick-starter/server/internal/vcs/mock"
    )

    type fakeRepo struct{ rows map[uuid.UUID]*model.VCSIntegration }

    func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[uuid.UUID]*model.VCSIntegration{}} }
    func (f *fakeRepo) Create(_ context.Context, r *model.VCSIntegration) error { f.rows[r.ID] = r; return nil }
    func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (*model.VCSIntegration, error) {
        return f.rows[id], nil
    }
    func (f *fakeRepo) ListByProject(_ context.Context, p uuid.UUID) ([]*model.VCSIntegration, error) {
        var out []*model.VCSIntegration
        for _, v := range f.rows {
            if v.ProjectID == p {
                out = append(out, v)
            }
        }
        return out, nil
    }
    func (f *fakeRepo) Update(_ context.Context, r *model.VCSIntegration) error { f.rows[r.ID] = r; return nil }
    func (f *fakeRepo) Delete(_ context.Context, id uuid.UUID) error { delete(f.rows, id); return nil }

    type fakeSecrets struct{ values map[string]string }

    func (f *fakeSecrets) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
        v, ok := f.values[name]
        if !ok {
            return "", vcs.ErrSecretNotResolvable
        }
        return v, nil
    }

    func TestService_CreateValidatesPATAndCreatesWebhook(t *testing.T) {
        reg := vcs.NewRegistry()
        mp := mock.New()
        reg.Register("github", func(host, token string) (vcs.Provider, error) { return mp, nil })

        svc := vcs.NewService(newFakeRepo(), reg, &fakeSecrets{values: map[string]string{
            "vcs.github.demo.pat":     "ghp_xxx",
            "vcs.github.demo.webhook": "shh",
        }}, "https://agentforge.example/api/v1/vcs/github/webhook")

        rec, err := svc.Create(context.Background(), vcs.CreateInput{
            ProjectID:        uuid.New(),
            Provider:         "github",
            Host:             "github.com",
            Owner:            "octocat",
            Repo:             "hello",
            DefaultBranch:    "main",
            TokenSecretRef:   "vcs.github.demo.pat",
            WebhookSecretRef: "vcs.github.demo.webhook",
        })
        if err != nil {
            t.Fatalf("Create: %v", err)
        }
        if rec.WebhookID == nil || *rec.WebhookID == "" {
            t.Errorf("expected webhook_id to be persisted; got %+v", rec.WebhookID)
        }

        // Mock recorder must show the validate call (GetPullRequest sentinel) AND the CreateWebhook call.
        ops := opsOf(mp.Calls())
        if !contains(ops, "GetPullRequest") || !contains(ops, "CreateWebhook") {
            t.Errorf("expected validate+webhook calls; got %v", ops)
        }
    }

    func TestService_CreateRejectsUnknownProvider(t *testing.T) {
        reg := vcs.NewRegistry()
        svc := vcs.NewService(newFakeRepo(), reg, &fakeSecrets{}, "https://x")
        _, err := svc.Create(context.Background(), vcs.CreateInput{Provider: "svn"})
        if err == nil {
            t.Fatal("expected ErrProviderUnsupported")
        }
    }

    func TestService_DeleteRemovesWebhookFirst(t *testing.T) {
        reg := vcs.NewRegistry()
        mp := mock.New()
        reg.Register("github", func(host, token string) (vcs.Provider, error) { return mp, nil })
        repo := newFakeRepo()
        wh := "hook-1"
        rec := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Provider: "github",
            Host: "github.com", Owner: "o", Repo: "r", WebhookID: &wh,
            TokenSecretRef: "vcs.github.demo.pat", WebhookSecretRef: "vcs.github.demo.webhook"}
        repo.rows[rec.ID] = rec

        svc := vcs.NewService(repo, reg, &fakeSecrets{values: map[string]string{
            "vcs.github.demo.pat":     "ghp_xxx",
            "vcs.github.demo.webhook": "shh",
        }}, "https://x")

        if err := svc.Delete(context.Background(), rec.ID); err != nil {
            t.Fatalf("Delete: %v", err)
        }
        if _, ok := repo.rows[rec.ID]; ok {
            t.Errorf("expected row removed")
        }
        if !contains(opsOf(mp.Calls()), "DeleteWebhook") {
            t.Errorf("expected DeleteWebhook to be called before row delete")
        }
    }

    func opsOf(calls []mock.Call) []string {
        out := make([]string, len(calls))
        for i, c := range calls {
            out[i] = c.Op
        }
        return out
    }
    func contains(s []string, want string) bool {
        for _, v := range s {
            if v == want {
                return true
            }
        }
        return false
    }
    ```
  - Run `rtk go test ./internal/vcs/...` — fails (Service missing).

- [ ] Step 9.2 — implement service
  - File: `src-go/internal/vcs/service.go`
    ```go
    package vcs

    import (
        "context"
        "errors"
        "fmt"
        "time"

        "github.com/google/uuid"

        "github.com/react-go-quick-starter/server/internal/model"
    )

    // ErrSecretNotResolvable signals the (project, secret_ref) tuple did
    // not resolve. The handler maps it to a 4xx so the user can fix the
    // ref before persisting the integration.
    var ErrSecretNotResolvable = errors.New("vcs:secret_not_resolvable")

    // Repo is the narrow persistence contract the Service consumes.
    type Repo interface {
        Create(ctx context.Context, r *model.VCSIntegration) error
        Get(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error)
        ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error)
        Update(ctx context.Context, r *model.VCSIntegration) error
        Delete(ctx context.Context, id uuid.UUID) error
    }

    // SecretsResolver is the narrow seam the Service uses to resolve
    // secret refs. Implemented by an adapter that forwards into the 1B
    // secrets.Service. Plaintext is consumed inside the same call frame
    // and never persisted on the Service struct.
    type SecretsResolver interface {
        Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
    }

    // CreateInput is the request payload accepted by Service.Create.
    type CreateInput struct {
        ProjectID        uuid.UUID
        Provider         string
        Host             string
        Owner            string
        Repo             string
        DefaultBranch    string
        TokenSecretRef   string
        WebhookSecretRef string
        ActingEmployeeID *uuid.UUID
    }

    // PatchInput captures the fields PATCH /vcs-integrations/:id may modify.
    type PatchInput struct {
        Status            *string
        TokenSecretRef    *string
        ActingEmployeeID  *uuid.UUID
    }

    // Service orchestrates registry + repo + secrets + webhook lifecycle.
    type Service struct {
        repo            Repo
        registry        *Registry
        secrets         SecretsResolver
        publicCallback  string // e.g. https://agentforge.example/api/v1/vcs/github/webhook
    }

    // NewService wires the Service.
    func NewService(repo Repo, reg *Registry, secrets SecretsResolver, callbackURL string) *Service {
        return &Service{repo: repo, registry: reg, secrets: secrets, publicCallback: callbackURL}
    }

    // Create validates the configuration end-to-end then persists.
    //   1. registry must know provider
    //   2. both secret refs must resolve in the project's secret scope
    //   3. PAT must validate against host (sentinel GetPullRequest call;
    //      we expect ErrAuthExpired XOR a non-auth response — anything else means PAT works)
    //   4. CreateWebhook on the host
    //   5. persist row with returned webhook_id
    func (s *Service) Create(ctx context.Context, in CreateInput) (*model.VCSIntegration, error) {
        if s.publicCallback == "" {
            return nil, fmt.Errorf("vcs:public_base_url_not_configured")
        }

        token, err := s.secrets.Resolve(ctx, in.ProjectID, in.TokenSecretRef)
        if err != nil {
            return nil, fmt.Errorf("%w: token ref %q", ErrSecretNotResolvable, in.TokenSecretRef)
        }
        whSecret, err := s.secrets.Resolve(ctx, in.ProjectID, in.WebhookSecretRef)
        if err != nil {
            return nil, fmt.Errorf("%w: webhook ref %q", ErrSecretNotResolvable, in.WebhookSecretRef)
        }

        prov, err := s.registry.Resolve(in.Provider, in.Host, token)
        if err != nil {
            return nil, err
        }

        repoRef := RepoRef{Host: in.Host, Owner: in.Owner, Repo: in.Repo}

        // Sentinel auth-validation call. We expect either a 404 (PR doesn't
        // exist) which proves auth works, OR ErrAuthExpired which we pass
        // back. Any other error is treated as transient and surfaced.
        _, err = prov.GetPullRequest(ctx, repoRef, 0)
        if errors.Is(err, ErrAuthExpired) {
            return nil, ErrAuthExpired
        }

        hookID, err := prov.CreateWebhook(ctx, repoRef, s.publicCallback, whSecret, []string{"pull_request", "push"})
        if err != nil {
            return nil, fmt.Errorf("vcs: create webhook: %w", err)
        }

        rec := &model.VCSIntegration{
            ID:               uuid.New(),
            ProjectID:        in.ProjectID,
            Provider:         in.Provider,
            Host:             in.Host,
            Owner:            in.Owner,
            Repo:             in.Repo,
            DefaultBranch:    coalesce(in.DefaultBranch, "main"),
            WebhookID:        &hookID,
            WebhookSecretRef: in.WebhookSecretRef,
            TokenSecretRef:   in.TokenSecretRef,
            Status:           "active",
            ActingEmployeeID: in.ActingEmployeeID,
        }
        if err := s.repo.Create(ctx, rec); err != nil {
            // best-effort webhook cleanup
            _ = prov.DeleteWebhook(ctx, repoRef, hookID)
            return nil, err
        }
        return rec, nil
    }

    // Patch updates mutable fields. Caller is responsible for re-validating
    // the new token if TokenSecretRef changed (we re-run the sentinel
    // GetPullRequest call here).
    func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*model.VCSIntegration, error) {
        rec, err := s.repo.Get(ctx, id)
        if err != nil {
            return nil, err
        }
        if in.Status != nil {
            rec.Status = *in.Status
        }
        if in.ActingEmployeeID != nil {
            rec.ActingEmployeeID = in.ActingEmployeeID
        }
        if in.TokenSecretRef != nil {
            token, err := s.secrets.Resolve(ctx, rec.ProjectID, *in.TokenSecretRef)
            if err != nil {
                return nil, ErrSecretNotResolvable
            }
            prov, err := s.registry.Resolve(rec.Provider, rec.Host, token)
            if err != nil {
                return nil, err
            }
            if _, err := prov.GetPullRequest(ctx, RepoRef{Host: rec.Host, Owner: rec.Owner, Repo: rec.Repo}, 0); errors.Is(err, ErrAuthExpired) {
                return nil, ErrAuthExpired
            }
            rec.TokenSecretRef = *in.TokenSecretRef
        }
        if err := s.repo.Update(ctx, rec); err != nil {
            return nil, err
        }
        return rec, nil
    }

    // Delete removes the host-side webhook first, then deletes the row.
    func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
        rec, err := s.repo.Get(ctx, id)
        if err != nil {
            return err
        }
        if rec.WebhookID != nil && *rec.WebhookID != "" {
            token, terr := s.secrets.Resolve(ctx, rec.ProjectID, rec.TokenSecretRef)
            if terr == nil {
                if prov, perr := s.registry.Resolve(rec.Provider, rec.Host, token); perr == nil {
                    _ = prov.DeleteWebhook(ctx, RepoRef{Host: rec.Host, Owner: rec.Owner, Repo: rec.Repo}, *rec.WebhookID)
                }
            }
        }
        return s.repo.Delete(ctx, id)
    }

    // List returns all integrations for the project.
    func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error) {
        return s.repo.ListByProject(ctx, projectID)
    }

    // QueueSync stamps last_synced_at and returns the row. The actual
    // background pull of open PRs lives in 2B; v1 returns 202.
    func (s *Service) QueueSync(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error) {
        rec, err := s.repo.Get(ctx, id)
        if err != nil {
            return nil, err
        }
        now := time.Now().UTC()
        rec.LastSyncedAt = &now
        if err := s.repo.Update(ctx, rec); err != nil {
            return nil, err
        }
        return rec, nil
    }

    func coalesce(a, b string) string {
        if a != "" {
            return a
        }
        return b
    }
    ```
  - Run `rtk go test ./internal/vcs/...` — passes.

- [ ] Step 9.3 — commit: `feat(vcs): integration service with secret-resolved auth probe + webhook lifecycle`

---

## Task 10 — `internal/handler/vcs_integrations_handler.go` HTTP CRUD

- [ ] Step 10.1 — write failing handler test
  - File: `src-go/internal/handler/vcs_integrations_handler_test.go`
    ```go
    package handler_test

    import (
        "bytes"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"

        "github.com/react-go-quick-starter/server/internal/handler"
    )

    func TestVCSIntegrationsHandler_RejectsBadProvider(t *testing.T) {
        e := echo.New()
        h := handler.NewVCSIntegrationsHandler(newFakeIntegSvc())
        body, _ := json.Marshal(map[string]any{"provider": "svn", "host": "x", "owner": "o", "repo": "r",
            "tokenSecretRef": "t", "webhookSecretRef": "w"})
        req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.SetParamNames("pid")
        c.SetParamValues(uuid.New().String())
        if err := h.Create(c); err != nil {
            t.Fatalf("Create: %v", err)
        }
        if rec.Code != http.StatusBadRequest {
            t.Errorf("expected 400, got %d", rec.Code)
        }
    }

    func TestVCSIntegrationsHandler_DeleteReturnsNoContent(t *testing.T) {
        e := echo.New()
        h := handler.NewVCSIntegrationsHandler(newFakeIntegSvc())
        req := httptest.NewRequest(http.MethodDelete, "/", nil)
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.SetParamNames("id")
        c.SetParamValues(uuid.New().String())
        if err := h.Delete(c); err != nil {
            t.Fatalf("Delete: %v", err)
        }
        if rec.Code != http.StatusNoContent {
            t.Errorf("expected 204, got %d", rec.Code)
        }
    }

    func TestVCSIntegrationsHandler_SyncReturns202(t *testing.T) {
        e := echo.New()
        h := handler.NewVCSIntegrationsHandler(newFakeIntegSvc())
        req := httptest.NewRequest(http.MethodPost, "/", nil)
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.SetParamNames("id")
        c.SetParamValues(uuid.New().String())
        if err := h.Sync(c); err != nil {
            t.Fatalf("Sync: %v", err)
        }
        if rec.Code != http.StatusAccepted {
            t.Errorf("expected 202, got %d", rec.Code)
        }
    }
    ```
  - File: `src-go/internal/handler/vcs_integrations_handler_helpers_test.go` — define `newFakeIntegSvc()` returning a stub satisfying the handler's narrow service interface. Stub responds with valid records / nil errors so the negative cases assert on the handler's own validation paths.
  - Run `rtk go test ./internal/handler/...` — fails (handler missing).

- [ ] Step 10.2 — implement handler
  - File: `src-go/internal/handler/vcs_integrations_handler.go`
    ```go
    package handler

    import (
        "context"
        "errors"
        "net/http"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"

        appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/vcs"
    )

    // vcsIntegrationsService is the narrow surface the handler consumes.
    type vcsIntegrationsService interface {
        Create(ctx context.Context, in vcs.CreateInput) (*model.VCSIntegration, error)
        Patch(ctx context.Context, id uuid.UUID, in vcs.PatchInput) (*model.VCSIntegration, error)
        Delete(ctx context.Context, id uuid.UUID) error
        List(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error)
        QueueSync(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error)
    }

    // VCSIntegrationsHandler exposes /vcs-integrations CRUD.
    type VCSIntegrationsHandler struct{ svc vcsIntegrationsService }

    // NewVCSIntegrationsHandler returns a wired handler.
    func NewVCSIntegrationsHandler(svc vcsIntegrationsService) *VCSIntegrationsHandler {
        return &VCSIntegrationsHandler{svc: svc}
    }

    // Register attaches both project-scoped and id-scoped routes.
    // projectGroup carries the :pid param + project RBAC; protected is the
    // top-level authenticated group used for /vcs-integrations/:id.
    func (h *VCSIntegrationsHandler) Register(projectGroup *echo.Group, protected *echo.Group) {
        projectGroup.GET("/vcs-integrations", h.List, appMiddleware.Require(appMiddleware.ActionVCSIntegrationRead))
        projectGroup.POST("/vcs-integrations", h.Create, appMiddleware.Require(appMiddleware.ActionVCSIntegrationCreate))
        protected.PATCH("/vcs-integrations/:id", h.Patch)
        protected.DELETE("/vcs-integrations/:id", h.Delete)
        protected.POST("/vcs-integrations/:id/sync", h.Sync)
    }

    type createIntegrationRequest struct {
        Provider         string  `json:"provider"`
        Host             string  `json:"host"`
        Owner            string  `json:"owner"`
        Repo             string  `json:"repo"`
        DefaultBranch    string  `json:"defaultBranch"`
        TokenSecretRef   string  `json:"tokenSecretRef"`
        WebhookSecretRef string  `json:"webhookSecretRef"`
        ActingEmployeeID *string `json:"actingEmployeeId"`
    }

    func (h *VCSIntegrationsHandler) List(c echo.Context) error {
        pid := appMiddleware.GetProjectID(c)
        rows, err := h.svc.List(c.Request().Context(), pid)
        if err != nil {
            return mapVCSError(c, err)
        }
        return c.JSON(http.StatusOK, rows)
    }

    func (h *VCSIntegrationsHandler) Create(c echo.Context) error {
        pid, err := uuid.Parse(c.Param("pid"))
        if err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid project id"})
        }
        req := new(createIntegrationRequest)
        if err := c.Bind(req); err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
        }
        if req.Provider == "" || req.Host == "" || req.Owner == "" || req.Repo == "" ||
            req.TokenSecretRef == "" || req.WebhookSecretRef == "" {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider/host/owner/repo/tokenSecretRef/webhookSecretRef are required"})
        }
        in := vcs.CreateInput{
            ProjectID:        pid,
            Provider:         req.Provider,
            Host:             req.Host,
            Owner:            req.Owner,
            Repo:             req.Repo,
            DefaultBranch:    req.DefaultBranch,
            TokenSecretRef:   req.TokenSecretRef,
            WebhookSecretRef: req.WebhookSecretRef,
        }
        if req.ActingEmployeeID != nil && *req.ActingEmployeeID != "" {
            id, err := uuid.Parse(*req.ActingEmployeeID)
            if err != nil {
                return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid actingEmployeeId"})
            }
            in.ActingEmployeeID = &id
        }
        rec, err := h.svc.Create(c.Request().Context(), in)
        if err != nil {
            return mapVCSError(c, err)
        }
        return c.JSON(http.StatusCreated, rec)
    }

    type patchIntegrationRequest struct {
        Status           *string `json:"status"`
        TokenSecretRef   *string `json:"tokenSecretRef"`
        ActingEmployeeID *string `json:"actingEmployeeId"`
    }

    func (h *VCSIntegrationsHandler) Patch(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
        }
        req := new(patchIntegrationRequest)
        if err := c.Bind(req); err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
        }
        in := vcs.PatchInput{Status: req.Status, TokenSecretRef: req.TokenSecretRef}
        if req.ActingEmployeeID != nil && *req.ActingEmployeeID != "" {
            empID, perr := uuid.Parse(*req.ActingEmployeeID)
            if perr != nil {
                return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid actingEmployeeId"})
            }
            in.ActingEmployeeID = &empID
        }
        rec, err := h.svc.Patch(c.Request().Context(), id, in)
        if err != nil {
            return mapVCSError(c, err)
        }
        return c.JSON(http.StatusOK, rec)
    }

    func (h *VCSIntegrationsHandler) Delete(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
        }
        if err := h.svc.Delete(c.Request().Context(), id); err != nil {
            return mapVCSError(c, err)
        }
        return c.NoContent(http.StatusNoContent)
    }

    func (h *VCSIntegrationsHandler) Sync(c echo.Context) error {
        id, err := uuid.Parse(c.Param("id"))
        if err != nil {
            return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
        }
        rec, err := h.svc.QueueSync(c.Request().Context(), id)
        if err != nil {
            return mapVCSError(c, err)
        }
        return c.JSON(http.StatusAccepted, map[string]any{
            "integration": rec,
            "note":        "background sync pending — full implementation in Spec 2B",
        })
    }

    func mapVCSError(c echo.Context, err error) error {
        switch {
        case errors.Is(err, vcs.ErrProviderUnsupported):
            return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
        case errors.Is(err, vcs.ErrSecretNotResolvable):
            return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
        case errors.Is(err, vcs.ErrAuthExpired):
            return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
        }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
    }
    ```
  - Run `rtk go test ./internal/handler/...` (-run VCSIntegrations) — passes.

- [ ] Step 10.3 — audit emission: in service Create / Patch / Delete / Sync, emit audit events via the existing `service.AuditService` (use the same adapter pattern 1B uses for `secret.*` actions). Use `ResourceType=AuditResourceTypeVCSIntegration` and `ResourceID=integration.ID.String()`. Payload JSON includes `{provider, host, owner, repo, op}` only — never tokens or webhook IDs.

- [ ] Step 10.4 — commit: `feat(handler): vcs-integrations CRUD with audit + sync stub`

---

## Task 11 — Wire registry, service, handler in `internal/server/routes.go`

- [ ] Step 11.1 — register providers + service at bootstrap
  - File: `src-go/internal/server/server.go` (or wherever the server-level wiring helper lives)
    - Construct `vcs.NewRegistry()` once.
    - Register concrete providers:
      ```go
      vcsRegistry := vcs.NewRegistry()
      vcsRegistry.Register("github", func(host, token string) (vcs.Provider, error) {
          base := ""
          if host != "github.com" {
              base = "https://" + host + "/api/v3/"
          }
          return ghimpl.NewClient(base, token)
      })
      vcsRegistry.Register("gitlab", gitlab.NewStub)
      vcsRegistry.Register("gitea", gitea.NewStub)
      ```
    - Build `vcs.NewService(vcsIntegrationRepo, vcsRegistry, secretsResolverAdapter, callbackURL)` where `callbackURL = strings.TrimRight(cfg.PublicBaseURL, "/") + "/api/v1/vcs/github/webhook"`.
    - `secretsResolverAdapter` is a tiny in-package wrapper that calls `secretsSvc.Resolve(ctx, projectID, name)` (1B's Service).
- [ ] Step 11.2 — wire routes
  - File: `src-go/internal/server/routes.go` (after the `employeeH.Register(projectGroup)` line, ~1038)
    ```go
    vcsIntegrationH := handler.NewVCSIntegrationsHandler(vcsIntegrationSvc)
    vcsIntegrationH.Register(projectGroup, protected)
    ```
- [ ] Step 11.3 — add config field
  - File: `src-go/internal/config/config.go` (or equivalent) — add `PublicBaseURL string` reading env `AGENTFORGE_PUBLIC_BASE_URL`. Default `http://localhost:7777` for dev with a `log.Warn` if the env is unset (real deployments must set it).
- [ ] Step 11.4 — verify route wiring test
  - Update `src-go/internal/server/routes_wiring_test.go` — assert the four new routes exist:
    - `GET /api/v1/projects/:pid/vcs-integrations`
    - `POST /api/v1/projects/:pid/vcs-integrations`
    - `PATCH /api/v1/vcs-integrations/:id`
    - `DELETE /api/v1/vcs-integrations/:id`
    - `POST /api/v1/vcs-integrations/:id/sync`
  - Run `rtk go test ./internal/server/...` — passes.
- [ ] Step 11.5 — commit: `feat(server): wire vcs registry, providers, service, and integrations handler`

---

## Task 12 — Frontend store `lib/stores/vcs-integrations-store.ts`

- [ ] Step 12.1 — write failing store test
  - File: `lib/stores/vcs-integrations-store.test.ts`
    ```ts
    /** @jest-environment jsdom */
    import { act } from "react";
    import { useVCSIntegrationsStore, type VCSIntegration } from "./vcs-integrations-store";
    import { useAuthStore } from "./auth-store";

    describe("useVCSIntegrationsStore", () => {
      beforeEach(() => {
        useVCSIntegrationsStore.setState({ integrationsByProject: {}, loadingByProject: {} });
        useAuthStore.setState({ accessToken: "tok" } as never);
      });

      it("fetches and stores integrations by project", async () => {
        const sample: VCSIntegration = {
          id: "i1", projectId: "p1", provider: "github", host: "github.com",
          owner: "o", repo: "r", defaultBranch: "main",
          tokenSecretRef: "vcs.github.demo.pat",
          webhookSecretRef: "vcs.github.demo.webhook",
          status: "active", createdAt: "", updatedAt: "",
        };
        global.fetch = jest.fn().mockResolvedValue({
          ok: true, status: 200, json: async () => [sample],
        }) as never;

        await act(async () => {
          await useVCSIntegrationsStore.getState().fetchIntegrations("p1");
        });

        expect(useVCSIntegrationsStore.getState().integrationsByProject["p1"]).toEqual([sample]);
      });

      it("creates an integration and prepends it", async () => {
        const created: VCSIntegration = {
          id: "i2", projectId: "p1", provider: "github", host: "github.com",
          owner: "o", repo: "r", defaultBranch: "main",
          tokenSecretRef: "t", webhookSecretRef: "w",
          status: "active", webhookId: "hook-1",
          createdAt: "", updatedAt: "",
        };
        global.fetch = jest.fn().mockResolvedValue({
          ok: true, status: 201, json: async () => created,
        }) as never;

        const result = await useVCSIntegrationsStore.getState().createIntegration("p1", {
          provider: "github", host: "github.com", owner: "o", repo: "r",
          defaultBranch: "main",
          tokenSecretRef: "t", webhookSecretRef: "w",
        });
        expect(result?.webhookId).toBe("hook-1");
        expect(useVCSIntegrationsStore.getState().integrationsByProject["p1"][0]).toEqual(created);
      });
    });
    ```
  - Run `rtk pnpm test -- lib/stores/vcs-integrations-store.test.ts` — fails (store missing).

- [ ] Step 12.2 — implement store
  - File: `lib/stores/vcs-integrations-store.ts`
    ```ts
    "use client";

    import { create } from "zustand";
    import { toast } from "sonner";
    import { createApiClient } from "@/lib/api-client";
    import { useAuthStore } from "./auth-store";

    const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:7777";

    export type VCSProvider = "github" | "gitlab" | "gitea";

    export interface VCSIntegration {
      id: string;
      projectId: string;
      provider: VCSProvider;
      host: string;
      owner: string;
      repo: string;
      defaultBranch: string;
      webhookId?: string;
      webhookSecretRef: string;
      tokenSecretRef: string;
      status: "active" | "auth_expired" | "paused";
      actingEmployeeId?: string;
      lastSyncedAt?: string;
      createdAt: string;
      updatedAt: string;
    }

    export interface CreateIntegrationInput {
      provider: VCSProvider;
      host: string;
      owner: string;
      repo: string;
      defaultBranch?: string;
      tokenSecretRef: string;
      webhookSecretRef: string;
      actingEmployeeId?: string;
    }

    export interface PatchIntegrationInput {
      status?: VCSIntegration["status"];
      tokenSecretRef?: string;
      actingEmployeeId?: string;
    }

    interface VCSIntegrationsStoreState {
      integrationsByProject: Record<string, VCSIntegration[]>;
      loadingByProject: Record<string, boolean>;
      fetchIntegrations: (projectId: string) => Promise<void>;
      createIntegration: (projectId: string, input: CreateIntegrationInput) => Promise<VCSIntegration | null>;
      patchIntegration: (id: string, input: PatchIntegrationInput) => Promise<VCSIntegration | null>;
      deleteIntegration: (projectId: string, id: string) => Promise<void>;
      syncIntegration: (id: string) => Promise<void>;
    }

    const getApi = () => createApiClient(API_URL);
    const getToken = () => {
      const s = useAuthStore.getState() as { accessToken?: string | null; token?: string | null };
      return s.accessToken ?? s.token ?? null;
    };

    export const useVCSIntegrationsStore = create<VCSIntegrationsStoreState>()((set, get) => ({
      integrationsByProject: {},
      loadingByProject: {},

      fetchIntegrations: async (projectId) => {
        const token = getToken();
        if (!token) return;
        set((s) => ({ loadingByProject: { ...s.loadingByProject, [projectId]: true } }));
        try {
          const { data } = await getApi().get<VCSIntegration[]>(
            `/api/v1/projects/${projectId}/vcs-integrations`,
            { token },
          );
          set((s) => ({ integrationsByProject: { ...s.integrationsByProject, [projectId]: data ?? [] } }));
        } catch (err) {
          toast.error(`加载 VCS 集成失败: ${(err as Error).message}`);
        } finally {
          set((s) => ({ loadingByProject: { ...s.loadingByProject, [projectId]: false } }));
        }
      },

      createIntegration: async (projectId, input) => {
        const token = getToken();
        if (!token) return null;
        try {
          const { data } = await getApi().post<VCSIntegration>(
            `/api/v1/projects/${projectId}/vcs-integrations`,
            input,
            { token },
          );
          set((s) => ({
            integrationsByProject: {
              ...s.integrationsByProject,
              [projectId]: [data, ...(s.integrationsByProject[projectId] ?? [])],
            },
          }));
          toast.success(`已连接 ${input.owner}/${input.repo}`);
          return data;
        } catch (err) {
          toast.error(`连接仓库失败: ${(err as Error).message}`);
          return null;
        }
      },

      patchIntegration: async (id, input) => {
        const token = getToken();
        if (!token) return null;
        try {
          const { data } = await getApi().patch<VCSIntegration>(
            `/api/v1/vcs-integrations/${id}`, input, { token },
          );
          set((s) => {
            const next: Record<string, VCSIntegration[]> = { ...s.integrationsByProject };
            for (const [pid, list] of Object.entries(next)) {
              next[pid] = list.map((it) => (it.id === id ? data : it));
            }
            return { integrationsByProject: next };
          });
          return data;
        } catch (err) {
          toast.error(`更新集成失败: ${(err as Error).message}`);
          return null;
        }
      },

      deleteIntegration: async (projectId, id) => {
        const token = getToken();
        if (!token) return;
        try {
          await getApi().delete(`/api/v1/vcs-integrations/${id}`, { token });
          set((s) => ({
            integrationsByProject: {
              ...s.integrationsByProject,
              [projectId]: (s.integrationsByProject[projectId] ?? []).filter((it) => it.id !== id),
            },
          }));
          toast.success("集成已删除");
        } catch (err) {
          toast.error(`删除集成失败: ${(err as Error).message}`);
        }
      },

      syncIntegration: async (id) => {
        const token = getToken();
        if (!token) return;
        try {
          await getApi().post(`/api/v1/vcs-integrations/${id}/sync`, {}, { token });
          toast.message("已排队后台同步");
        } catch (err) {
          toast.error(`触发同步失败: ${(err as Error).message}`);
        }
      },
    }));
    ```
  - Run `rtk pnpm test -- lib/stores/vcs-integrations-store.test.ts` — passes.

- [ ] Step 12.3 — commit: `feat(fe): vcs-integrations zustand store with CRUD + sync`

---

## Task 13 — FE page `app/(dashboard)/projects/[id]/integrations/vcs/page.tsx`

- [ ] Step 13.1 — write failing component test
  - File: `app/(dashboard)/projects/[id]/integrations/vcs/page.test.tsx`
    ```tsx
    /** @jest-environment jsdom */
    import { render, screen, waitFor } from "@testing-library/react";
    import userEvent from "@testing-library/user-event";
    import VCSIntegrationsPage from "./page";
    import { useVCSIntegrationsStore } from "@/lib/stores/vcs-integrations-store";
    import { useSecretsStore } from "@/lib/stores/secrets-store";

    jest.mock("next/navigation", () => ({ useParams: () => ({ id: "p1" }) }));

    describe("VCSIntegrationsPage", () => {
      beforeEach(() => {
        useVCSIntegrationsStore.setState({
          integrationsByProject: {
            p1: [{
              id: "i1", projectId: "p1", provider: "github", host: "github.com",
              owner: "octocat", repo: "hello", defaultBranch: "main",
              webhookId: "hook-99", tokenSecretRef: "vcs.github.demo.pat",
              webhookSecretRef: "vcs.github.demo.webhook", status: "active",
              createdAt: "2026-04-20", updatedAt: "2026-04-20",
            }],
          },
          loadingByProject: {},
          fetchIntegrations: jest.fn().mockResolvedValue(undefined),
          createIntegration: jest.fn().mockResolvedValue(null),
          patchIntegration: jest.fn().mockResolvedValue(null),
          deleteIntegration: jest.fn().mockResolvedValue(undefined),
          syncIntegration: jest.fn().mockResolvedValue(undefined),
        });
        useSecretsStore.setState({
          secretsByProject: { p1: [
            { name: "vcs.github.demo.pat", createdBy: "u", createdAt: "", updatedAt: "" },
            { name: "vcs.github.demo.webhook", createdBy: "u", createdAt: "", updatedAt: "" },
          ] },
          loadingByProject: {}, lastRevealedValue: null,
        } as never);
      });

      it("renders existing integration with status + webhook URL", async () => {
        render(<VCSIntegrationsPage />);
        await waitFor(() => expect(screen.getByText("octocat/hello")).toBeInTheDocument());
        expect(screen.getByText(/active/i)).toBeInTheDocument();
      });

      it("disables non-github providers in selector", async () => {
        render(<VCSIntegrationsPage />);
        const user = userEvent.setup();
        await user.click(screen.getByRole("button", { name: /add repo|connect/i }));
        const gitlab = await screen.findByRole("option", { name: /gitlab/i });
        expect(gitlab).toHaveAttribute("aria-disabled", "true");
      });

      it("shows webhook URL preview after create", async () => {
        const created = {
          id: "i9", projectId: "p1", provider: "github" as const,
          host: "github.com", owner: "x", repo: "y", defaultBranch: "main",
          webhookId: "hook-7", tokenSecretRef: "vcs.github.demo.pat",
          webhookSecretRef: "vcs.github.demo.webhook", status: "active" as const,
          createdAt: "", updatedAt: "",
        };
        useVCSIntegrationsStore.getState().createIntegration = jest.fn().mockResolvedValue(created);
        render(<VCSIntegrationsPage />);
        const user = userEvent.setup();
        await user.click(screen.getByRole("button", { name: /add repo|connect/i }));
        await user.type(screen.getByLabelText(/host/i), "github.com");
        await user.type(screen.getByLabelText(/owner/i), "x");
        await user.type(screen.getByLabelText(/repo/i), "y");
        await user.click(screen.getByRole("button", { name: /create|connect/i }));
        await waitFor(() => expect(screen.getByText(/hook-7/)).toBeInTheDocument());
      });
    });
    ```
  - Run `rtk pnpm test -- app/(dashboard)/projects/[id]/integrations/vcs` — fails (page missing).

- [ ] Step 13.2 — implement page
  - File: `app/(dashboard)/projects/[id]/integrations/vcs/page.tsx`
    ```tsx
    "use client";

    import { useEffect, useMemo, useState } from "react";
    import { useParams } from "next/navigation";
    import { Plus, Github, RefreshCw, Trash2, AlertTriangle } from "lucide-react";

    import { Button } from "@/components/ui/button";
    import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
    import { Badge } from "@/components/ui/badge";
    import { Input } from "@/components/ui/input";
    import { Label } from "@/components/ui/label";
    import {
      Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger,
    } from "@/components/ui/dialog";
    import {
      Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
    } from "@/components/ui/select";
    import {
      useVCSIntegrationsStore,
      type CreateIntegrationInput,
      type VCSIntegration,
      type VCSProvider,
    } from "@/lib/stores/vcs-integrations-store";
    import { useSecretsStore } from "@/lib/stores/secrets-store";

    const PROVIDERS: { value: VCSProvider; label: string; enabled: boolean }[] = [
      { value: "github", label: "GitHub", enabled: true },
      { value: "gitlab", label: "GitLab (coming soon)", enabled: false },
      { value: "gitea", label: "Gitea (coming soon)", enabled: false },
    ];

    function StatusBadge({ status }: { status: VCSIntegration["status"] }) {
      switch (status) {
        case "active": return <Badge variant="default">active</Badge>;
        case "auth_expired": return <Badge variant="destructive">auth expired</Badge>;
        case "paused": return <Badge variant="secondary">paused</Badge>;
      }
    }

    export default function VCSIntegrationsPage() {
      const params = useParams<{ id: string }>();
      const projectId = params.id;
      const integrations = useVCSIntegrationsStore(
        (s) => s.integrationsByProject[projectId] ?? [],
      );
      const fetchIntegrations = useVCSIntegrationsStore((s) => s.fetchIntegrations);
      const deleteIntegration = useVCSIntegrationsStore((s) => s.deleteIntegration);
      const syncIntegration = useVCSIntegrationsStore((s) => s.syncIntegration);
      const fetchSecrets = useSecretsStore((s) => s.fetchSecrets);

      useEffect(() => {
        if (projectId) {
          fetchIntegrations(projectId);
          fetchSecrets(projectId);
        }
      }, [projectId, fetchIntegrations, fetchSecrets]);

      return (
        <div className="flex flex-col gap-6 p-6">
          <header className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-semibold">VCS Integrations</h1>
              <p className="text-sm text-muted-foreground">连接代码仓库以启用 PR 审查与自动修复。</p>
            </div>
            <CreateIntegrationDialog projectId={projectId} />
          </header>

          {integrations.length === 0 ? (
            <Card><CardContent className="py-12 text-center text-muted-foreground">尚未连接仓库。</CardContent></Card>
          ) : (
            <div className="grid gap-4">
              {integrations.map((it) => (
                <Card key={it.id}>
                  <CardHeader className="flex flex-row items-start justify-between">
                    <div>
                      <CardTitle className="flex items-center gap-2">
                        <Github className="size-4" />
                        {it.owner}/{it.repo}
                      </CardTitle>
                      <p className="text-xs text-muted-foreground">{it.host} · {it.defaultBranch}</p>
                    </div>
                    <StatusBadge status={it.status} />
                  </CardHeader>
                  <CardContent className="flex flex-col gap-2 text-xs">
                    <div><span className="text-muted-foreground">Webhook ID: </span>{it.webhookId ?? "—"}</div>
                    <div><span className="text-muted-foreground">PAT secret: </span><code>{it.tokenSecretRef}</code></div>
                    <div><span className="text-muted-foreground">Webhook secret: </span><code>{it.webhookSecretRef}</code></div>
                    <div className="flex gap-2 pt-2">
                      <Button size="sm" variant="outline" onClick={() => syncIntegration(it.id)}>
                        <RefreshCw className="mr-1 size-3" /> Re-sync
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => deleteIntegration(projectId, it.id)}>
                        <Trash2 className="mr-1 size-3" /> Delete
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      );
    }

    function CreateIntegrationDialog({ projectId }: { projectId: string }) {
      const [open, setOpen] = useState(false);
      const [provider, setProvider] = useState<VCSProvider>("github");
      const [host, setHost] = useState("github.com");
      const [owner, setOwner] = useState("");
      const [repo, setRepo] = useState("");
      const [branch, setBranch] = useState("main");
      const [tokenSecretRef, setTokenSecretRef] = useState("");
      const [webhookSecretRef, setWebhookSecretRef] = useState("");
      const [createdWebhookID, setCreatedWebhookID] = useState<string | null>(null);

      const secrets = useSecretsStore((s) => s.secretsByProject[projectId] ?? []);
      const createIntegration = useVCSIntegrationsStore((s) => s.createIntegration);

      const onSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        const input: CreateIntegrationInput = {
          provider, host, owner, repo, defaultBranch: branch,
          tokenSecretRef, webhookSecretRef,
        };
        const result = await createIntegration(projectId, input);
        if (result?.webhookId) {
          setCreatedWebhookID(result.webhookId);
        }
      };

      return (
        <Dialog open={open} onOpenChange={(v) => { setOpen(v); if (!v) setCreatedWebhookID(null); }}>
          <DialogTrigger asChild>
            <Button size="sm"><Plus className="mr-1 size-4" /> Add repo</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader><DialogTitle>Connect a repository</DialogTitle></DialogHeader>
            {createdWebhookID ? (
              <div className="flex flex-col gap-2 text-sm">
                <p>已创建 webhook <code>{createdWebhookID}</code>。</p>
                <p className="text-xs text-muted-foreground">回调 URL 已自动注册到仓库；如需查看可在 GitHub Settings → Webhooks 验证。</p>
                <Button size="sm" onClick={() => setOpen(false)}>Done</Button>
              </div>
            ) : (
              <form onSubmit={onSubmit} className="flex flex-col gap-3">
                <div className="flex flex-col gap-1">
                  <Label>Provider</Label>
                  <Select value={provider} onValueChange={(v) => setProvider(v as VCSProvider)}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      {PROVIDERS.map((p) => (
                        <SelectItem key={p.value} value={p.value} disabled={!p.enabled}>{p.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-1">
                  <Label htmlFor="host">Host</Label>
                  <Input id="host" value={host} onChange={(e) => setHost(e.target.value)} required />
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <div className="flex flex-col gap-1">
                    <Label htmlFor="owner">Owner</Label>
                    <Input id="owner" value={owner} onChange={(e) => setOwner(e.target.value)} required />
                  </div>
                  <div className="flex flex-col gap-1">
                    <Label htmlFor="repo">Repo</Label>
                    <Input id="repo" value={repo} onChange={(e) => setRepo(e.target.value)} required />
                  </div>
                </div>
                <div className="flex flex-col gap-1">
                  <Label htmlFor="branch">Default branch</Label>
                  <Input id="branch" value={branch} onChange={(e) => setBranch(e.target.value)} />
                </div>
                <div className="flex flex-col gap-1">
                  <Label>PAT secret</Label>
                  <Select value={tokenSecretRef} onValueChange={setTokenSecretRef}>
                    <SelectTrigger><SelectValue placeholder="选择密钥" /></SelectTrigger>
                    <SelectContent>
                      {secrets.map((s) => <SelectItem key={s.name} value={s.name}>{s.name}</SelectItem>)}
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-1">
                  <Label>Webhook secret</Label>
                  <Select value={webhookSecretRef} onValueChange={setWebhookSecretRef}>
                    <SelectTrigger><SelectValue placeholder="选择密钥" /></SelectTrigger>
                    <SelectContent>
                      {secrets.map((s) => <SelectItem key={s.name} value={s.name}>{s.name}</SelectItem>)}
                    </SelectContent>
                  </Select>
                </div>
                {!tokenSecretRef && (
                  <p className="flex items-center gap-1 text-xs text-amber-600">
                    <AlertTriangle className="size-3" />
                    需要先在 /secrets 页面创建 PAT 与 webhook secret。
                  </p>
                )}
                <Button type="submit" disabled={!tokenSecretRef || !webhookSecretRef}>Create</Button>
              </form>
            )}
          </DialogContent>
        </Dialog>
      );
    }
    ```
  - Run `rtk pnpm test -- app/(dashboard)/projects/[id]/integrations/vcs` — passes.

- [ ] Step 13.3 — add nav link
  - File: `app/(dashboard)/projects/[id]/layout.tsx` (extend the shell 1B introduces; add an "Integrations" entry pointing at `./integrations/vcs` alongside the "Secrets" entry).
- [ ] Step 13.4 — commit: `feat(fe): vcs integrations management page with secret-ref selectors`

---

## Task 14 — Integration test (real PG + mock provider end-to-end)

- [ ] Step 14.1 — author integration test
  - File: `src-go/internal/handler/vcs_integrations_handler_integration_test.go`
    ```go
    //go:build integration

    package handler_test

    import (
        "bytes"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/google/uuid"
        "github.com/labstack/echo/v4"

        "github.com/react-go-quick-starter/server/internal/handler"
        "github.com/react-go-quick-starter/server/internal/repository"
        "github.com/react-go-quick-starter/server/internal/vcs"
        "github.com/react-go-quick-starter/server/internal/vcs/mock"
    )

    func TestEndToEndCRUDWithMockProvider(t *testing.T) {
        db := setupTestDB(t)
        repo := repository.NewVCSIntegrationRepo(db)

        reg := vcs.NewRegistry()
        mp := mock.New()
        reg.Register("github", func(host, token string) (vcs.Provider, error) { return mp, nil })

        secrets := stubSecrets{values: map[string]string{
            "vcs.github.demo.pat":     "ghp_xxx",
            "vcs.github.demo.webhook": "shh",
        }}
        svc := vcs.NewService(repo, reg, &secrets, "https://agentforge.example/api/v1/vcs/github/webhook")
        h := handler.NewVCSIntegrationsHandler(svc)

        e := echo.New()
        proj := seedProject(t, db)

        // CREATE
        body, _ := json.Marshal(map[string]any{
            "provider": "github", "host": "github.com",
            "owner": "octocat", "repo": "hello", "defaultBranch": "main",
            "tokenSecretRef": "vcs.github.demo.pat",
            "webhookSecretRef": "vcs.github.demo.webhook",
        })
        req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        c := e.NewContext(req, rec)
        c.SetParamNames("pid")
        c.SetParamValues(proj.ID.String())
        if err := h.Create(c); err != nil || rec.Code != http.StatusCreated {
            t.Fatalf("create: code=%d err=%v", rec.Code, err)
        }

        // verify mock saw PAT validate + webhook create
        ops := []string{}
        for _, call := range mp.Calls() {
            ops = append(ops, call.Op)
        }
        if !sliceContains(ops, "GetPullRequest") || !sliceContains(ops, "CreateWebhook") {
            t.Errorf("expected validate+webhook ops, got %v", ops)
        }

        // LIST
        rec = httptest.NewRecorder()
        c = e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
        c.SetParamNames("pid")
        c.SetParamValues(proj.ID.String())
        if err := h.List(c); err != nil || rec.Code != http.StatusOK {
            t.Fatalf("list: code=%d err=%v", rec.Code, err)
        }
        var got []map[string]any
        _ = json.Unmarshal(rec.Body.Bytes(), &got)
        if len(got) != 1 {
            t.Fatalf("expected 1 integration, got %d", len(got))
        }
        id := got[0]["id"].(string)

        // DELETE — must invoke DeleteWebhook before row delete
        rec = httptest.NewRecorder()
        c = e.NewContext(httptest.NewRequest(http.MethodDelete, "/", nil), rec)
        c.SetParamNames("id")
        c.SetParamValues(id)
        if err := h.Delete(c); err != nil || rec.Code != http.StatusNoContent {
            t.Fatalf("delete: code=%d err=%v", rec.Code, err)
        }
        if !sliceContains(opsOf(mp.Calls()), "DeleteWebhook") {
            t.Errorf("expected DeleteWebhook to fire on row delete")
        }

        // verify row gone
        if _, err := repo.Get(c.Request().Context(), uuid.MustParse(id)); err == nil {
            t.Errorf("expected row removed from PG")
        }
    }

    type stubSecrets struct{ values map[string]string }

    func (s *stubSecrets) Resolve(_ any, _ uuid.UUID, name string) (string, error) {
        v, ok := s.values[name]
        if !ok {
            return "", vcs.ErrSecretNotResolvable
        }
        return v, nil
    }

    func sliceContains(s []string, v string) bool {
        for _, x := range s {
            if x == v {
                return true
            }
        }
        return false
    }

    func opsOf(calls []mock.Call) []string {
        out := make([]string, len(calls))
        for i, c := range calls {
            out[i] = c.Op
        }
        return out
    }
    ```
- [ ] Step 14.2 — run integration suite
  - `rtk go test -tags=integration ./internal/handler/... -run VCSIntegrations` — passes.
- [ ] Step 14.3 — commit: `test(vcs): integration coverage of integrations CRUD with mock provider`

---

## Task 15 — Documentation + Spec Drifts

- [ ] Step 15.1 — append to Spec 2 §13.1 "Spec Drifts Found During Brainstorm"
  - File: `docs/superpowers/specs/2026-04-20-code-reviewer-employee-design.md`
    - Note that `webhook_secret_ref` callback URL constant is `${AGENTFORGE_PUBLIC_BASE_URL}/api/v1/vcs/github/webhook` (Plan 2A introduces the env var; webhook handler arrives in Plan 2B).
    - Note that the registry exposes `Names()` so FE can drive the provider selector dynamically; v1 hard-codes the disabled GitLab/Gitea entries since stubs are unusable.
    - Note that `vcs.Service.Create` performs a sentinel `GetPullRequest(repo, 0)` to validate PAT; non-`vcs:auth_expired` errors are tolerated (proves auth works) — this is intentional per §11 and not a bug.
- [ ] Step 15.2 — verify backend bootstraps cleanly with new env
  - `rtk pnpm dev:backend:verify` — passes.
- [ ] Step 15.3 — commit: `docs(spec2): record 2A spec drifts (callback URL env, registry Names, validate sentinel)`

---

## Acceptance Criteria

- `rtk go test ./internal/vcs/... ./internal/handler/... ./internal/repository/...` green.
- `rtk go test -tags=integration ./internal/handler/... -run VCSIntegrations` green.
- `rtk pnpm test -- vcs-integrations-store` and `rtk pnpm test -- app/(dashboard)/projects/[id]/integrations/vcs` green.
- `rtk pnpm exec tsc --noEmit` clean.
- Migration 068 applies + reverts cleanly via `rtk pnpm dev:backend:restart go-orchestrator`.
- Creating a GitHub integration through the FE registers a real webhook (against a sandbox repo) and persists `webhook_id`.
- Deleting an integration removes the host-side webhook before deleting the row.
- Audit log shows `vcs.integration.create/update/delete/sync` events with `resource_type=vcs_integration`; payloads contain only `{provider, host, owner, repo, op}` — never tokens.
- GitLab / Gitea selections in FE are visible-but-disabled; backend rejects with `vcs:provider_unsupported` if forced.
