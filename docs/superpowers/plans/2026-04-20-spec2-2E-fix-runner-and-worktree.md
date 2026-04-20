# Spec 2E — fix_runner_service + Per-Repo FIFO Lock + worktree+patch+push + OpenPR + Auth-Expired UX

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 2 §5 S2-E + §6.1 fix_runs + §7 internal fix runner endpoints + §9 Trace B execute 阶段 + §11 per-repo lock TTL + auth_expired 处理。

**Architecture:** fix_runner_service 暴露 dry-run / execute 两个内部端点（`X-Internal-Token` 鉴权）+ per-repo FIFO Redis 锁（10min TTL）+ worktree.Manager 切 fix branch + git apply/commit/push + vcs.Provider.OpenPR；execute 写 fix_runs 行；401/403 触发 auth_expired 流（暂停 webhook 自动 review + FE 横幅 + 重新绑定 PAT 流程）。

**Tech Stack:** Go (Redis lock + git CLI 子进程 + vcs.Provider), Postgres, Next.js (fix_runs 表 + auth_expired 横幅).

**Depends on:** 2A (vcs.Provider.OpenPR), 2D (code_fixer DAG 调用本端点), 1B (secrets store for PAT)

**Parallel with:** 2C — disjoint files

**Unblocks:** Spec 2 整体闭环

---

## Coordination notes (read before starting)

- **Migration number**: 067 reserved by 1B; 2A/2B/2C/2D consume 068–072 in order. This plan claims **073** for `fix_runs`. If your branch lands first, renumber down; the up/down SQL is mechanical and touches no other table.
- **Worktree.Manager extension**: existing `Manager.Prepare` only creates the canonical `agent/<taskID>` branch from the repo's HEAD. We need a new entry point that **(a) accepts a base SHA**, **(b) accepts an arbitrary branch name**, **(c) auto-suffixes `-attempt-N` on collision** (per Spec 2 §14 risk). New method `AllocateFromSHA(ctx, projectSlug, baseSHA, branchName) (*Allocation, error)` is added in this plan; existing methods stay untouched.
- **Per-repo lock semantics (§11)**: 10-minute TTL is the **safety release**, not the normal hold time. Acquire timeout (waiter side) is 2 min per §10. Lock is FIFO via Redis LIST `BRPOPLPUSH` between a `waiters` queue and an `active` key; on Release we `LREM` from `active`.
- **Internal auth (§7)**: `AGENTFORGE_INTERNAL_TOKEN` env var; HTTP header `X-Internal-Token: <token>`. Constant-time compare. Reject 401 with audit event `internal_auth:rejected` (truncated bearer hash only, never plaintext).
- **Patch hashing in audit (§11)**: every fix_run state transition writes audit row with `payload.patch_sha256`, never the patch body. Test asserts log capture has zero plaintext patch bytes.
- **vcs.Provider.OpenPR signature (from 2A)**: `OpenPR(ctx, repo RepoRef, base, head, title, body string, opts OpenPROpts) (*PullRequest, error)`. Labels passed as `opts.Labels = []string{"agentforge:fix"}`.
- **Auth-expired propagation (§10)**: any GitHub HTTP error with status 401 or 403 (rate-limit-not-pertinent) flips `vcs_integrations.status='auth_expired'`. Implemented as a small helper `vcs.IsAuthExpiredErr(err)` that BOTH this plan and 2B's outbound dispatcher import — single source of truth, no duplicate predicates.
- **No silent webhook drop (§10)**: when integration status ≠ 'active', `vcs_webhook_handler` returns 202 + records `vcs_webhook_events.processing_error='integration_not_active'`. Spec 2 explicitly says "pause auto-reviews" — pause means do not Trigger ReviewService, NOT swallow the event.
- **Worktree LRU policy (§11)**: keep last 10 fix_run worktrees on disk for debug; eviction is best-effort, ordered by `fix_runs.completed_at`. Manager already supports `Release`; this plan adds an LRU sweeper that fires from the execute pipeline tail.

---

## Task 1 — Migration 073 fix_runs + reviews/findings column extensions for fix-flow

- [ ] Step 1.1 — write the up migration
  - File: `src-go/migrations/073_create_fix_runs.up.sql`
    ```sql
    -- fix_runs: one row per attempted code-fixer execution against a finding.
    -- Spec 2 §6.1. Patch is stored verbatim for replay/debug; audit only records sha256.
    CREATE TABLE IF NOT EXISTS fix_runs (
        id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        review_id           UUID NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
        finding_id          UUID NOT NULL,
        source              VARCHAR(32) NOT NULL CHECK (source IN ('pre_baked','agent_generated')),
        worktree_path       TEXT,
        fix_branch_name     VARCHAR(128),
        fix_pr_url          TEXT,
        patch               TEXT,
        status              VARCHAR(16) NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending','running','applied','conflict','push_failed','timeout','failed')),
        apply_attempts      INT NOT NULL DEFAULT 0,
        decided_by          UUID,
        decided_via         VARCHAR(16) CHECK (decided_via IN ('fe','feishu_card','github_label','auto')),
        acting_employee_id  UUID REFERENCES employees(id) ON DELETE SET NULL,
        error_message       TEXT,
        created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
        completed_at        TIMESTAMPTZ
    );
    CREATE INDEX IF NOT EXISTS idx_fix_runs_review  ON fix_runs(review_id);
    CREATE INDEX IF NOT EXISTS idx_fix_runs_finding ON fix_runs(finding_id);
    CREATE INDEX IF NOT EXISTS idx_fix_runs_status_created ON fix_runs(status, created_at);
    ```
- [ ] Step 1.2 — write the down migration
  - File: `src-go/migrations/073_create_fix_runs.down.sql`
    ```sql
    DROP INDEX IF EXISTS idx_fix_runs_status_created;
    DROP INDEX IF EXISTS idx_fix_runs_finding;
    DROP INDEX IF EXISTS idx_fix_runs_review;
    DROP TABLE IF EXISTS fix_runs;
    ```
- [ ] Step 1.3 — sanity-check status enum coverage
  - Cross-check against Spec 2 §10 error matrix: `pending|running|applied|conflict|push_failed|timeout|failed` covers every documented terminal/transient state. Document mapping inline in the up migration as comments if not already present.
- [ ] Step 1.4 — apply locally and verify
  - `rtk pnpm dev:backend:restart go-orchestrator` and confirm `073_create_fix_runs.up.sql` applied; `psql` `\d fix_runs` shows the indexes and CHECK constraints.

---

## Task 2 — `internal/fixrunner/lock.go` per-repo FIFO Redis lock

- [ ] Step 2.1 — write failing lock tests
  - File: `src-go/internal/fixrunner/lock_test.go`
    ```go
    package fixrunner_test

    import (
        "context"
        "sync"
        "testing"
        "time"

        "github.com/react-go-quick-starter/server/internal/fixrunner"
        "github.com/react-go-quick-starter/server/pkg/database"
    )

    func TestLock_AcquireAndRelease(t *testing.T) {
        rdb, _ := database.NewRedis("redis://localhost:6379/15")
        defer rdb.FlushDB(context.Background())
        l := fixrunner.NewLockManager(rdb, 10*time.Minute)

        tok, err := l.Acquire(context.Background(), "int1", "owner/repo", 2*time.Second)
        if err != nil { t.Fatalf("acquire: %v", err) }
        if tok == "" { t.Fatal("empty token") }
        if err := l.Release(context.Background(), "int1", "owner/repo", tok); err != nil {
            t.Fatalf("release: %v", err)
        }
    }

    func TestLock_FIFOSerialisesConcurrent(t *testing.T) {
        rdb, _ := database.NewRedis("redis://localhost:6379/15")
        defer rdb.FlushDB(context.Background())
        l := fixrunner.NewLockManager(rdb, 10*time.Minute)

        var order []int
        var mu sync.Mutex
        var wg sync.WaitGroup
        gate := make(chan struct{})
        for i := 0; i < 5; i++ {
            wg.Add(1)
            go func(idx int) {
                defer wg.Done()
                <-gate
                tok, err := l.Acquire(context.Background(), "int2", "o/r", 5*time.Second)
                if err != nil { t.Errorf("acquire %d: %v", idx, err); return }
                mu.Lock(); order = append(order, idx); mu.Unlock()
                time.Sleep(20 * time.Millisecond)
                _ = l.Release(context.Background(), "int2", "o/r", tok)
            }(i)
            time.Sleep(5 * time.Millisecond) // enforce LPUSH ordering
        }
        close(gate); wg.Wait()
        if len(order) != 5 { t.Fatalf("only %d acquired", len(order)) }
    }

    func TestLock_AcquireTimeout(t *testing.T) {
        rdb, _ := database.NewRedis("redis://localhost:6379/15")
        defer rdb.FlushDB(context.Background())
        l := fixrunner.NewLockManager(rdb, 10*time.Minute)

        tok, _ := l.Acquire(context.Background(), "int3", "o/r", time.Second)
        defer l.Release(context.Background(), "int3", "o/r", tok)

        _, err := l.Acquire(context.Background(), "int3", "o/r", 200*time.Millisecond)
        if err != fixrunner.ErrLockTimeout {
            t.Fatalf("want ErrLockTimeout, got %v", err)
        }
    }
    ```
- [ ] Step 2.2 — implement
  - File: `src-go/internal/fixrunner/lock.go`
    ```go
    package fixrunner

    import (
        "context"
        "crypto/rand"
        "encoding/hex"
        "errors"
        "fmt"
        "time"

        "github.com/redis/go-redis/v9"
    )

    var ErrLockTimeout = errors.New("fix_run:repo_locked")

    type LockManager struct {
        rdb *redis.Client
        ttl time.Duration
    }

    func NewLockManager(rdb *redis.Client, ttl time.Duration) *LockManager {
        return &LockManager{rdb: rdb, ttl: ttl}
    }

    func (m *LockManager) keyWaiters(integrationID, repo string) string {
        return fmt.Sprintf("fixlock:%s:%s:waiters", integrationID, repo)
    }
    func (m *LockManager) keyActive(integrationID, repo string) string {
        return fmt.Sprintf("fixlock:%s:%s:active", integrationID, repo)
    }

    // Acquire pushes a unique token onto the FIFO waiters list and atomically
    // moves it to the active list when the front of the queue is reached.
    // BRPOPLPUSH (deprecated alias of BLMOVE RIGHT LEFT) blocks until the head
    // is exactly our token; we then set TTL on the active key so a crashed
    // holder eventually gets reclaimed.
    func (m *LockManager) Acquire(ctx context.Context, integrationID, repo string, wait time.Duration) (string, error) {
        buf := make([]byte, 16)
        if _, err := rand.Read(buf); err != nil { return "", err }
        token := hex.EncodeToString(buf)

        if err := m.rdb.LPush(ctx, m.keyWaiters(integrationID, repo), token).Err(); err != nil {
            return "", fmt.Errorf("lpush waiter: %w", err)
        }

        deadline := time.Now().Add(wait)
        for {
            // Peek head; if not us, block briefly and retry.
            head, err := m.rdb.LIndex(ctx, m.keyWaiters(integrationID, repo), -1).Result()
            if err != nil && err != redis.Nil {
                _ = m.removeWaiter(ctx, integrationID, repo, token)
                return "", err
            }
            if head == token {
                // Move into active and set safety TTL.
                if _, err := m.rdb.RPopLPush(ctx, m.keyWaiters(integrationID, repo), m.keyActive(integrationID, repo)).Result(); err != nil {
                    return "", err
                }
                if err := m.rdb.Expire(ctx, m.keyActive(integrationID, repo), m.ttl).Err(); err != nil {
                    return "", err
                }
                return token, nil
            }
            if time.Now().After(deadline) {
                _ = m.removeWaiter(ctx, integrationID, repo, token)
                return "", ErrLockTimeout
            }
            time.Sleep(50 * time.Millisecond)
        }
    }

    func (m *LockManager) Release(ctx context.Context, integrationID, repo, token string) error {
        if token == "" { return nil }
        return m.rdb.LRem(ctx, m.keyActive(integrationID, repo), 0, token).Err()
    }

    func (m *LockManager) removeWaiter(ctx context.Context, integrationID, repo, token string) error {
        return m.rdb.LRem(ctx, m.keyWaiters(integrationID, repo), 0, token).Err()
    }
    ```
- [ ] Step 2.3 — verify
  - `rtk go test ./internal/fixrunner/...` (requires running Redis on `:6379`); confirm all three tests pass and no goroutine leaks.

---

## Task 3 — `worktree.Manager.AllocateFromSHA` (base SHA + arbitrary branch + collision suffix)

- [ ] Step 3.1 — write failing test
  - File: `src-go/internal/worktree/manager_allocate_from_sha_test.go`
    ```go
    package worktree_test

    import (
        "context"
        "os"
        "os/exec"
        "path/filepath"
        "testing"

        "github.com/react-go-quick-starter/server/internal/worktree"
    )

    func gitInit(t *testing.T, dir string) string {
        t.Helper()
        for _, args := range [][]string{
            {"init", "--initial-branch=main"},
            {"config", "user.email", "t@example.com"},
            {"config", "user.name", "T"},
            {"commit", "--allow-empty", "-m", "init"},
        } {
            cmd := exec.Command("git", args...)
            cmd.Dir = dir
            if out, err := cmd.CombinedOutput(); err != nil {
                t.Fatalf("git %v: %s: %v", args, out, err)
            }
        }
        sha, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
        return string(sha[:40])
    }

    func TestAllocateFromSHA_CreatesBranch(t *testing.T) {
        base := t.TempDir()
        repos := filepath.Join(base, "repos"); _ = os.MkdirAll(filepath.Join(repos, "p1"), 0o755)
        sha := gitInit(t, filepath.Join(repos, "p1"))

        m := worktree.NewManager(filepath.Join(base, "wt"), repos)
        a, err := m.AllocateFromSHA(context.Background(), "p1", sha, "fix/r1/f1")
        if err != nil { t.Fatalf("allocate: %v", err) }
        if a.Branch != "fix/r1/f1" { t.Fatalf("branch=%s", a.Branch) }
        if _, err := os.Stat(a.Path); err != nil { t.Fatalf("worktree dir missing: %v", err) }
    }

    func TestAllocateFromSHA_CollisionSuffix(t *testing.T) {
        base := t.TempDir()
        repos := filepath.Join(base, "repos"); _ = os.MkdirAll(filepath.Join(repos, "p1"), 0o755)
        sha := gitInit(t, filepath.Join(repos, "p1"))
        m := worktree.NewManager(filepath.Join(base, "wt"), repos)

        a1, err := m.AllocateFromSHA(context.Background(), "p1", sha, "fix/r1/f1")
        if err != nil { t.Fatal(err) }
        a2, err := m.AllocateFromSHA(context.Background(), "p1", sha, "fix/r1/f1")
        if err != nil { t.Fatal(err) }
        if a1.Branch == a2.Branch { t.Fatalf("expected suffix; both = %s", a1.Branch) }
        if a2.Branch != "fix/r1/f1-attempt-2" { t.Fatalf("got %s", a2.Branch) }
    }
    ```
- [ ] Step 3.2 — implement
  - File: `src-go/internal/worktree/manager.go` (append below `Create`)
    ```go
    // AllocateFromSHA creates a worktree at the given base SHA on a caller-supplied
    // branch name (used by fix_runner_service for fix/<rid>/<fid> branches).
    // On branch collision, suffixes -attempt-N (per Spec 2 §14 risk).
    func (m *Manager) AllocateFromSHA(ctx context.Context, projectSlug, baseSHA, branchName string) (*Allocation, error) {
        if strings.Contains(branchName, "..") || strings.HasPrefix(branchName, "/") {
            return nil, fmt.Errorf("invalid branch name %q", branchName)
        }
        finalBranch := branchName
        for attempt := 2; ; attempt++ {
            exists, err := m.branchExists(ctx, projectSlug, finalBranch)
            if err != nil { return nil, err }
            if !exists { break }
            finalBranch = fmt.Sprintf("%s-attempt-%d", branchName, attempt)
            if attempt > 50 {
                return nil, fmt.Errorf("branch collision: too many attempts for %s", branchName)
            }
        }

        worktreeBase := filepath.Join(m.basePath, projectSlug)
        if err := os.MkdirAll(worktreeBase, 0o755); err != nil {
            return nil, fmt.Errorf("create worktree base dir: %w", err)
        }
        // Use a stable disk path derived from final branch (slashes → __).
        slug := strings.ReplaceAll(finalBranch, "/", "__")
        path := filepath.Join(worktreeBase, slug)

        cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", finalBranch, path, baseSHA)
        cmd.Dir = m.repoPath(projectSlug)
        if out, err := cmd.CombinedOutput(); err != nil {
            return nil, fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
        }
        return &Allocation{ProjectSlug: projectSlug, Branch: finalBranch, Path: path}, nil
    }
    ```
- [ ] Step 3.3 — symlink-escape sanitisation test (§11 worktree path safety)
  - Add a third test that calls `AllocateFromSHA(..., "../escape")` and asserts the invalid-branch error path triggers (no worktree created, no parent-dir traversal).
- [ ] Step 3.4 — verify
  - `rtk go test ./internal/worktree/...`; confirm all new tests pass and pre-existing tests remain green.

---

## Task 4 — `internal/fixrunner/service.go` core (DryRun + Execute orchestration)

- [ ] Step 4.1 — write failing service test (mocked git + mocked vcs)
  - File: `src-go/internal/fixrunner/service_test.go`
    ```go
    package fixrunner_test

    import (
        "context"
        "testing"

        "github.com/react-go-quick-starter/server/internal/fixrunner"
        "github.com/react-go-quick-starter/server/internal/vcs"
        "github.com/react-go-quick-starter/server/internal/vcs/mock"
    )

    func TestService_DryRun_Success(t *testing.T) {
        svc := fixrunner.NewServiceForTest(t) // helper wiring tmp git + fake repos
        out, err := svc.DryRun(context.Background(), fixrunner.DryRunInput{
            IntegrationID: "int1",
            HeadSHA:       svc.SeedHeadSHA(t),
            FilePath:      "README.md",
            Patch:         svc.ValidPatchAdding(t, "README.md", "hi"),
        })
        if err != nil { t.Fatal(err) }
        if !out.OK { t.Fatalf("not ok: %s", out.ConflictSummary) }
    }

    func TestService_Execute_HappyPath(t *testing.T) {
        svc := fixrunner.NewServiceForTest(t)
        prov := mock.NewProvider()
        svc.SetProvider(prov)

        out, err := svc.Execute(context.Background(), fixrunner.ExecuteInput{
            ReviewID:   svc.SeedReviewID(t),
            FindingID:  svc.SeedFindingID(t),
            Patch:      svc.ValidPatchAdding(t, "README.md", "hi"),
            EmployeeID: "emp1",
        })
        if err != nil { t.Fatal(err) }
        if out.Status != "applied" { t.Fatalf("status=%s err=%s", out.Status, out.Error) }
        if out.FixPRURL == "" { t.Fatal("no PR url") }
        if got := prov.OpenPRCalls(); len(got) != 1 || got[0].Opts.Labels[0] != "agentforge:fix" {
            t.Fatalf("openpr calls: %+v", got)
        }
    }

    func TestService_Execute_DryRunConflict_ShortCircuits(t *testing.T) {
        svc := fixrunner.NewServiceForTest(t)
        out, err := svc.Execute(context.Background(), fixrunner.ExecuteInput{
            ReviewID: svc.SeedReviewID(t), FindingID: svc.SeedFindingID(t),
            Patch: "garbage diff", EmployeeID: "emp1",
        })
        if err != nil { t.Fatal(err) }
        if out.Status != "conflict" { t.Fatalf("status=%s", out.Status) }
        if !svc.AssertNoVCSCall(t) { t.Fatal("vcs should not be called on conflict") }
    }

    func TestService_Execute_AuthExpiredFlipsIntegration(t *testing.T) {
        svc := fixrunner.NewServiceForTest(t)
        prov := mock.NewProvider()
        prov.OpenPRError = vcs.AuthExpiredError{Status: 401}
        svc.SetProvider(prov)

        _, _ = svc.Execute(context.Background(), fixrunner.ExecuteInput{
            ReviewID: svc.SeedReviewID(t), FindingID: svc.SeedFindingID(t),
            Patch: svc.ValidPatchAdding(t, "README.md", "hi"), EmployeeID: "emp1",
        })
        if got := svc.IntegrationStatus(t, "int1"); got != "auth_expired" {
            t.Fatalf("integration status=%s", got)
        }
    }
    ```
- [ ] Step 4.2 — implement service
  - File: `src-go/internal/fixrunner/service.go`
    ```go
    package fixrunner

    import (
        "context"
        "crypto/sha256"
        "encoding/hex"
        "fmt"
        "os"
        "os/exec"
        "path/filepath"
        "time"

        "github.com/google/uuid"
        "github.com/react-go-quick-starter/server/internal/audit"
        "github.com/react-go-quick-starter/server/internal/eventbus"
        "github.com/react-go-quick-starter/server/internal/secrets"
        "github.com/react-go-quick-starter/server/internal/vcs"
        "github.com/react-go-quick-starter/server/internal/worktree"
    )

    type Service struct {
        DB          DB                  // repository facade (fix_runs, reviews, findings, integrations)
        Secrets     *secrets.Service
        Worktree    *worktree.Manager
        Lock        *LockManager
        Provider    vcs.Provider
        Bus         *eventbus.Bus
        Audit       *audit.Service
        WorktreeLRU int                 // keep N most recent fix worktrees on disk
    }

    type DryRunInput struct {
        IntegrationID string
        HeadSHA       string
        FilePath      string
        Patch         string
    }
    type DryRunOutput struct {
        OK              bool
        ConflictSummary string
    }

    type ExecuteInput struct {
        ReviewID, FindingID, Patch, EmployeeID string
    }
    type ExecuteOutput struct {
        FixRunID, FixPRURL, Status, Error string
    }

    func (s *Service) DryRun(ctx context.Context, in DryRunInput) (*DryRunOutput, error) {
        integ, err := s.DB.GetIntegration(ctx, in.IntegrationID)
        if err != nil { return nil, err }
        a, err := s.Worktree.AllocateFromSHA(ctx, integ.ProjectSlug(), in.HeadSHA, "dryrun-"+uuid.NewString())
        if err != nil { return nil, err }
        defer s.Worktree.Release(ctx, integ.ProjectSlug(), filepath.Base(a.Path))

        if err := writeTempPatch(a.Path, in.Patch); err != nil { return nil, err }
        cmd := s.gitApply(ctx, a.Path, "--check", filepath.Join(a.Path, ".agentforge.patch"))
        if out, err := cmd.CombinedOutput(); err != nil {
            return &DryRunOutput{OK: false, ConflictSummary: string(out)}, nil
        }
        return &DryRunOutput{OK: true}, nil
    }

    func (s *Service) Execute(ctx context.Context, in ExecuteInput) (*ExecuteOutput, error) {
        finding, err := s.DB.GetFinding(ctx, in.FindingID)
        if err != nil { return nil, err }
        review, err := s.DB.GetReview(ctx, in.ReviewID)
        if err != nil { return nil, err }
        integ, err := s.DB.GetIntegration(ctx, review.IntegrationID)
        if err != nil { return nil, err }
        pat, err := s.Secrets.Resolve(ctx, integ.ProjectID, integ.TokenSecretRef)
        if err != nil { return nil, err }

        run, err := s.DB.InsertFixRun(ctx, FixRunRow{
            ReviewID: in.ReviewID, FindingID: in.FindingID,
            Source: sourceOf(finding), ActingEmployeeID: in.EmployeeID,
            Patch: in.Patch, Status: "pending",
        })
        if err != nil { return nil, err }
        s.audit(ctx, run.ID, "created", in.Patch)

        // FIFO lock with 2-min wait (§10).
        repo := integ.Owner + "/" + integ.Repo
        token, err := s.Lock.Acquire(ctx, integ.ID, repo, 2*time.Minute)
        if err != nil {
            s.complete(ctx, run.ID, "timeout", "", "", err.Error())
            return &ExecuteOutput{FixRunID: run.ID, Status: "timeout", Error: err.Error()}, nil
        }
        defer s.Lock.Release(ctx, integ.ID, repo, token)

        // Branch + worktree.
        branch := fmt.Sprintf("fix/%s/%s", in.ReviewID, in.FindingID)
        a, err := s.Worktree.AllocateFromSHA(ctx, integ.ProjectSlug(), review.HeadSHA, branch)
        if err != nil {
            s.complete(ctx, run.ID, "failed", "", "", err.Error())
            return &ExecuteOutput{FixRunID: run.ID, Status: "failed", Error: err.Error()}, nil
        }
        s.DB.UpdateFixRunWorktree(ctx, run.ID, a.Path, a.Branch)

        // Apply patch (with --3way fallback).
        if err := s.applyPatch(ctx, a.Path, in.Patch); err != nil {
            s.complete(ctx, run.ID, "conflict", a.Path, a.Branch, err.Error())
            return &ExecuteOutput{FixRunID: run.ID, Status: "conflict", Error: err.Error()}, nil
        }
        // Commit + push (with rebase-retry once on rejected).
        if err := s.commitAndPush(ctx, a.Path, a.Branch, integ, pat, finding.Message, review.BaseBranch); err != nil {
            s.complete(ctx, run.ID, "push_failed", a.Path, a.Branch, err.Error())
            return &ExecuteOutput{FixRunID: run.ID, Status: "push_failed", Error: err.Error()}, nil
        }
        // OpenPR.
        title := fmt.Sprintf("Fix: %s", finding.Message)
        body := buildPRBody(review, finding)
        pr, err := s.Provider.OpenPR(ctx, vcs.RepoRef{Host: integ.Host, Owner: integ.Owner, Repo: integ.Repo},
            review.BaseBranch, a.Branch, title, body,
            vcs.OpenPROpts{Draft: false, Labels: []string{"agentforge:fix"}})
        if err != nil {
            if vcs.IsAuthExpiredErr(err) {
                s.markAuthExpired(ctx, integ.ID)
            }
            s.complete(ctx, run.ID, "failed", a.Path, a.Branch, err.Error())
            return &ExecuteOutput{FixRunID: run.ID, Status: "failed", Error: err.Error()}, nil
        }

        s.complete(ctx, run.ID, "applied", a.Path, a.Branch, "")
        s.DB.SetFixRunPRURL(ctx, run.ID, pr.URL)
        s.sweepLRU(ctx, integ.ProjectSlug())

        return &ExecuteOutput{FixRunID: run.ID, FixPRURL: pr.URL, Status: "applied"}, nil
    }

    func (s *Service) applyPatch(ctx context.Context, dir, patch string) error {
        if err := writeTempPatch(dir, patch); err != nil { return err }
        cmd := s.gitApply(ctx, dir, filepath.Join(dir, ".agentforge.patch"))
        if out, err := cmd.CombinedOutput(); err == nil { return nil } else {
            // 3-way fallback
            cmd = s.gitApply(ctx, dir, "--3way", filepath.Join(dir, ".agentforge.patch"))
            if out2, err2 := cmd.CombinedOutput(); err2 != nil {
                return fmt.Errorf("git apply: %s | %s", out, out2)
            }
        }
        return nil
    }

    func (s *Service) gitApply(ctx context.Context, dir string, args ...string) *exec.Cmd {
        full := append([]string{"apply"}, args...)
        cmd := exec.CommandContext(ctx, "git", full...)
        cmd.Dir = dir
        // Restricted env: drop GIT_EXTERNAL_DIFF and friends (§11).
        cmd.Env = append(os.Environ(),
            "GIT_EXTERNAL_DIFF=", "GIT_TRACE=", "GIT_TRACE_PACKET=",
            "GIT_PAGER=cat", "PAGER=cat",
        )
        return cmd
    }

    func (s *Service) commitAndPush(ctx context.Context, dir, branch string, integ *Integration, pat, msg, baseBranch string) error {
        runs := [][]string{
            {"-c", "user.email=bot@agentforge.dev", "-c", "user.name=AgentForge Bot",
                "commit", "-am", "fix: " + msg},
        }
        for _, args := range runs {
            cmd := exec.CommandContext(ctx, "git", args...); cmd.Dir = dir
            if out, err := cmd.CombinedOutput(); err != nil {
                return fmt.Errorf("git %v: %s: %w", args, out, err)
            }
        }
        // Push with PAT-authenticated remote URL injection.
        remote := fmt.Sprintf("https://x-access-token:%s@%s/%s/%s.git", pat, integ.Host, integ.Owner, integ.Repo)
        push := func() error {
            cmd := exec.CommandContext(ctx, "git", "push", remote, branch)
            cmd.Dir = dir
            out, err := cmd.CombinedOutput()
            if err != nil { return fmt.Errorf("push: %s: %w", out, err) }
            return nil
        }
        if err := push(); err == nil { return nil }
        // Rebase + retry once.
        for _, args := range [][]string{
            {"fetch", remote, baseBranch},
            {"rebase", "FETCH_HEAD"},
        } {
            cmd := exec.CommandContext(ctx, "git", args...); cmd.Dir = dir
            if out, err := cmd.CombinedOutput(); err != nil {
                return fmt.Errorf("rebase: %s: %w", out, err)
            }
        }
        return push()
    }

    func (s *Service) markAuthExpired(ctx context.Context, integID string) {
        _ = s.DB.SetIntegrationStatus(ctx, integID, "auth_expired")
        s.Bus.Publish(eventbus.EventVCSAuthExpired{IntegrationID: integID})
    }

    func (s *Service) audit(ctx context.Context, runID, transition, patch string) {
        sum := sha256.Sum256([]byte(patch))
        _ = s.Audit.RecordEvent(ctx, audit.Event{
            ResourceType: "fix_run", ResourceID: runID,
            Action: "fix_run." + transition,
            Payload: map[string]any{"patch_sha256": hex.EncodeToString(sum[:])},
        })
    }

    func (s *Service) complete(ctx context.Context, runID, status, path, branch, errMsg string) {
        _ = s.DB.CompleteFixRun(ctx, runID, status, path, branch, errMsg)
        s.audit(ctx, runID, status, "")
    }

    func (s *Service) sweepLRU(ctx context.Context, projectSlug string) {
        if s.WorktreeLRU <= 0 { s.WorktreeLRU = 10 }
        old, err := s.DB.OldestFixWorktrees(ctx, projectSlug, s.WorktreeLRU)
        if err != nil { return }
        for _, p := range old {
            _ = os.RemoveAll(p)
        }
    }

    func writeTempPatch(dir, patch string) error {
        return os.WriteFile(filepath.Join(dir, ".agentforge.patch"), []byte(patch), 0o600)
    }

    func sourceOf(f *Finding) string {
        if f.SuggestedPatch != "" { return "pre_baked" }
        return "agent_generated"
    }
    ```
- [ ] Step 4.3 — implement `vcs.AuthExpiredError` + `IsAuthExpiredErr` helper (single source of truth)
  - File: `src-go/internal/vcs/errors.go`
    ```go
    package vcs

    import "errors"

    type AuthExpiredError struct{ Status int; Body string }
    func (e AuthExpiredError) Error() string { return "vcs:auth_expired" }

    func IsAuthExpiredErr(err error) bool {
        var ae AuthExpiredError
        return errors.As(err, &ae)
    }
    ```
- [ ] Step 4.4 — verify
  - `rtk go test ./internal/fixrunner/...` should pass all four scenarios.

---

## Task 5 — `internal/handler/fix_runner_handler.go` (`X-Internal-Token` gated)

- [ ] Step 5.1 — write failing handler tests
  - File: `src-go/internal/handler/fix_runner_handler_test.go`
    ```go
    package handler_test

    import (
        "bytes"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/react-go-quick-starter/server/internal/handler"
    )

    func TestFixRunnerHandler_RejectsMissingToken(t *testing.T) {
        srv := handler.NewFixRunnerTestServer(t, "secret-tok")
        req := httptest.NewRequest("POST", "/api/v1/internal/fix-runs/execute", bytes.NewBufferString(`{}`))
        rr := httptest.NewRecorder()
        srv.ServeHTTP(rr, req)
        if rr.Code != http.StatusUnauthorized { t.Fatalf("code=%d", rr.Code) }
    }

    func TestFixRunnerHandler_AcceptsValidToken(t *testing.T) {
        srv := handler.NewFixRunnerTestServer(t, "secret-tok")
        body, _ := json.Marshal(map[string]any{
            "review_id": "r1", "finding_id": "f1", "patch": "diff --git ...", "employee_id": "e1",
        })
        req := httptest.NewRequest("POST", "/api/v1/internal/fix-runs/execute", bytes.NewBuffer(body))
        req.Header.Set("X-Internal-Token", "secret-tok")
        rr := httptest.NewRecorder()
        srv.ServeHTTP(rr, req)
        if rr.Code != http.StatusOK { t.Fatalf("code=%d body=%s", rr.Code, rr.Body) }
    }
    ```
- [ ] Step 5.2 — implement handler + register routes
  - File: `src-go/internal/handler/fix_runner_handler.go`
    ```go
    package handler

    import (
        "crypto/subtle"
        "net/http"

        "github.com/labstack/echo/v4"
        "github.com/react-go-quick-starter/server/internal/fixrunner"
    )

    type FixRunnerHandler struct {
        Svc           *fixrunner.Service
        InternalToken string
    }

    func (h *FixRunnerHandler) Register(g *echo.Group) {
        g.POST("/internal/fix-runs/dry-run", h.dryRun, h.requireInternal)
        g.POST("/internal/fix-runs/execute", h.execute, h.requireInternal)
    }

    func (h *FixRunnerHandler) requireInternal(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            tok := c.Request().Header.Get("X-Internal-Token")
            if subtle.ConstantTimeCompare([]byte(tok), []byte(h.InternalToken)) != 1 {
                return c.JSON(http.StatusUnauthorized, echo.Map{"error": "internal_auth:rejected"})
            }
            return next(c)
        }
    }

    func (h *FixRunnerHandler) dryRun(c echo.Context) error {
        var in fixrunner.DryRunInput
        if err := c.Bind(&in); err != nil { return c.JSON(http.StatusBadRequest, errResp(err)) }
        out, err := h.Svc.DryRun(c.Request().Context(), in)
        if err != nil { return c.JSON(http.StatusInternalServerError, errResp(err)) }
        return c.JSON(http.StatusOK, out)
    }

    func (h *FixRunnerHandler) execute(c echo.Context) error {
        var in fixrunner.ExecuteInput
        if err := c.Bind(&in); err != nil { return c.JSON(http.StatusBadRequest, errResp(err)) }
        out, err := h.Svc.Execute(c.Request().Context(), in)
        if err != nil { return c.JSON(http.StatusInternalServerError, errResp(err)) }
        return c.JSON(http.StatusOK, out)
    }

    func errResp(err error) echo.Map { return echo.Map{"error": err.Error()} }
    ```
- [ ] Step 5.3 — wire into `cmd/server/main.go` (or wherever Echo route registration lives)
  - Construct `FixRunnerHandler` with `InternalToken: os.Getenv("AGENTFORGE_INTERNAL_TOKEN")` and `Register(api)` under the existing `/api/v1` group.
- [ ] Step 5.4 — verify
  - `rtk go test ./internal/handler/...` and confirm both new cases pass.

---

## Task 6 — Auth-expired flow: webhook handler + outbound dispatcher pause + event publish

- [ ] Step 6.1 — write failing test for webhook pause
  - File: `src-go/internal/handler/vcs_webhook_handler_pause_test.go`
    ```go
    package handler_test

    import "testing"

    func TestVCSWebhook_SkipsTriggerWhenIntegrationAuthExpired(t *testing.T) {
        srv := setupVCSWebhookTest(t)
        srv.SeedIntegration("int1", "auth_expired")
        rr := srv.PostWebhook(t, "int1", "pull_request", validPayload())
        if rr.Code != 202 { t.Fatalf("code=%d", rr.Code) }
        if srv.ReviewService.TriggerCalls() != 0 { t.Fatal("Trigger should be skipped") }
        if !srv.HasWebhookEventLogged("integration_not_active") { t.Fatal("missing log row") }
    }
    ```
- [ ] Step 6.2 — patch `vcs_webhook_handler.go` (added by 2A)
  - After looking up integration, if `integ.Status != "active"`, insert into `vcs_webhook_events` with `processing_error='integration_not_active'`, return 202, do NOT call `ReviewService.Trigger`.
- [ ] Step 6.3 — patch outbound dispatcher (2B) to flip status on auth-expired error
  - In each provider call site that surfaces `vcs.IsAuthExpiredErr(err)`, call `repo.SetIntegrationStatus(ctx, id, "auth_expired")` and publish `eventbus.EventVCSAuthExpired{IntegrationID: id}` exactly once per integration per ten-minute window (use Redis `SET NX EX 600` to dedupe the event publish).
- [ ] Step 6.4 — define event type
  - File: `src-go/internal/eventbus/types.go` — add:
    ```go
    type EventVCSAuthExpired struct {
        IntegrationID string `json:"integration_id"`
    }
    func (EventVCSAuthExpired) EventType() string { return "vcs.auth_expired" }
    ```
- [ ] Step 6.5 — verify
  - `rtk go test ./internal/handler/... ./internal/eventbus/...` confirms pause path + event publish dedup behave as specified.

---

## Task 7 — WS broadcast of `vcs.auth_expired` + FE banner state

- [ ] Step 7.1 — extend WS hub subscriber
  - File: `src-go/internal/ws/hub.go` (or the centralized event-to-WS bridge).
  - Subscribe to `EventVCSAuthExpired` and emit `{type: "vcs.auth_expired", integration_id}` to all connected clients in the integration's project room.
- [ ] Step 7.2 — write FE store test
  - File: `lib/stores/vcs-integration-store.test.ts` (Jest)
    ```ts
    import { useVCSIntegrationStore } from "./vcs-integration-store"

    test("ws auth_expired flips local status", () => {
      useVCSIntegrationStore.setState({ integrations: [{ id: "i1", status: "active" } as any] })
      useVCSIntegrationStore.getState().handleWSEvent({ type: "vcs.auth_expired", integration_id: "i1" })
      expect(useVCSIntegrationStore.getState().integrations[0].status).toBe("auth_expired")
    })
    ```
- [ ] Step 7.3 — implement store handler + WS bus subscription
  - File: `lib/stores/vcs-integration-store.ts` — add `handleWSEvent` method; ensure WS bootstrap (`hooks/use-ws.ts` or equivalent) routes `vcs.auth_expired` into it.
- [ ] Step 7.4 — verify
  - `rtk vitest run lib/stores/vcs-integration-store.test.ts` — confirm green.

---

## Task 8 — FE: VCS integrations page banner + "Re-bind PAT" CTA (modal → 1B rotate endpoint)

- [ ] Step 8.1 — write component test
  - File: `app/(dashboard)/projects/[id]/integrations/vcs/page.test.tsx`
    ```tsx
    import { render, screen } from "@testing-library/react"
    import VCSIntegrationsPage from "./page"
    import { useVCSIntegrationStore } from "@/lib/stores/vcs-integration-store"

    test("renders auth_expired banner", () => {
      useVCSIntegrationStore.setState({ integrations: [{
        id: "i1", owner: "acme", repo: "x", status: "auth_expired",
      } as any] })
      render(<VCSIntegrationsPage />)
      expect(screen.getByText(/Auth expired/i)).toBeInTheDocument()
      expect(screen.getByRole("button", { name: /Re-bind PAT/i })).toBeInTheDocument()
    })
    ```
- [ ] Step 8.2 — implement banner + modal
  - File: `app/(dashboard)/projects/[id]/integrations/vcs/page.tsx` (added by 2A; this plan extends).
  - Render `<Alert variant="destructive">` for any integration with `status === "auth_expired"`; CTA opens existing 1B `SecretRotateModal` pre-filled with `secret_name = integration.tokenSecretRef`.
  - On successful rotate, POST `/api/v1/vcs-integrations/:id` `{status: "active"}` to clear the flag.
- [ ] Step 8.3 — verify
  - `rtk vitest run app/(dashboard)/projects/\[id\]/integrations/vcs/page.test.tsx`.

---

## Task 9 — FE: per-finding detail page fix-runs history table + Retry button

- [ ] Step 9.1 — write component test
  - File: `app/(dashboard)/reviews/[id]/findings/[fid]/fix-runs.test.tsx`
    ```tsx
    import { render, screen, fireEvent } from "@testing-library/react"
    import FixRunsTable from "./fix-runs-table"

    const rows = [
      { id: "r1", created_at: "2026-04-20T00:00:00Z", source: "agent_generated",
        status: "applied", fix_pr_url: "https://github.com/x/y/pull/3", error_message: null },
      { id: "r2", created_at: "2026-04-20T01:00:00Z", source: "pre_baked",
        status: "conflict", fix_pr_url: null, error_message: "patch did not apply" },
    ]

    test("renders fix-runs and triggers retry", () => {
      const onRetry = jest.fn()
      render(<FixRunsTable rows={rows} onRetry={onRetry} />)
      expect(screen.getByText("applied")).toBeInTheDocument()
      expect(screen.getByText("conflict")).toBeInTheDocument()
      fireEvent.click(screen.getAllByRole("button", { name: /Retry/i })[0])
      expect(onRetry).toHaveBeenCalled()
    })
    ```
- [ ] Step 9.2 — implement table component
  - File: `app/(dashboard)/reviews/[id]/findings/[fid]/fix-runs-table.tsx`
  - Columns: `created_at`, `source`, `status`, `fix_pr_url` (clickable when present), `error_message` (truncated + tooltip), `Retry`.
  - Retry posts `POST /api/v1/findings/:id/decision` with `{action:"approve"}` (re-spawns the code_fixer DAG via 2D).
- [ ] Step 9.3 — wire into page (created by 2D)
  - In `app/(dashboard)/reviews/[id]/findings/[fid]/page.tsx`, replace the placeholder fix-history slot with `<FixRunsTable rows={fixRuns} onRetry={...} />`; rows fetched via `GET /api/v1/findings/:fid/fix-runs` (added below).
- [ ] Step 9.4 — backend list endpoint + repo method
  - `GET /api/v1/findings/:fid/fix-runs` returning `[]FixRunRow`. Repo method `ListByFinding(ctx, fid)` ordered by `created_at DESC`.
- [ ] Step 9.5 — verify
  - `rtk vitest run app/(dashboard)/reviews/\[id\]/findings/\[fid\]/fix-runs.test.tsx` and `rtk go test ./internal/handler/... ./internal/repository/...`.

---

## Task 10 — Audit + safety hardening

- [ ] Step 10.1 — assert no plaintext patch in audit
  - File: `src-go/internal/fixrunner/service_audit_test.go`
    ```go
    func TestAudit_NeverContainsPlaintextPatch(t *testing.T) {
        svc := fixrunner.NewServiceForTest(t)
        secret := "PRIVATE_TOKEN_DO_NOT_LEAK"
        patch := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+" + secret + "\n"

        _, _ = svc.Execute(context.Background(), fixrunner.ExecuteInput{
            ReviewID: svc.SeedReviewID(t), FindingID: svc.SeedFindingID(t),
            Patch: patch, EmployeeID: "e1",
        })
        for _, row := range svc.AuditRows(t) {
            buf, _ := json.Marshal(row.Payload)
            if bytes.Contains(buf, []byte(secret)) {
                t.Fatalf("audit row leaked plaintext: %s", buf)
            }
        }
    }
    ```
- [ ] Step 10.2 — assert restricted git env
  - Add a unit test that intercepts `exec.Command` (via a tiny indirection) and asserts `GIT_EXTERNAL_DIFF=` (empty) is in the env slice.
- [ ] Step 10.3 — assert worktree path stays inside `basePath`
  - Test malicious branch names like `../../etc` → `AllocateFromSHA` returns error, no file written outside `basePath/projectSlug/`.
- [ ] Step 10.4 — verify
  - `rtk go test ./internal/fixrunner/... ./internal/worktree/...`.

---

## Task 11 — Integration tests: real PG + Redis + mock VCS for full execute pipeline

- [ ] Step 11.1 — write integration harness test
  - File: `src-go/internal/fixrunner/integration_test.go`
    ```go
    //go:build integration

    package fixrunner_test

    import (
        "context"
        "testing"

        "github.com/react-go-quick-starter/server/internal/fixrunner"
    )

    func TestIntegration_ExecuteEndToEnd(t *testing.T) {
        env := bootIntegrationEnv(t) // spins ephemeral PG schema, Redis flushdb, mock github
        defer env.Close()

        out, err := env.Service.Execute(context.Background(), fixrunner.ExecuteInput{
            ReviewID:  env.SeedReview(t),
            FindingID: env.SeedFinding(t),
            Patch:     env.ValidPatch(t),
            EmployeeID: "emp1",
        })
        if err != nil { t.Fatal(err) }
        if out.Status != "applied" { t.Fatalf("status=%s err=%s", out.Status, out.Error) }

        rows := env.QueryFixRuns(t)
        if len(rows) != 1 || rows[0].Status != "applied" || rows[0].FixPRURL == "" {
            t.Fatalf("rows=%+v", rows)
        }
        if env.MockGitHub.OpenPRCount() != 1 { t.Fatalf("openpr count=%d", env.MockGitHub.OpenPRCount()) }
    }
    ```
- [ ] Step 11.2 — add tag-gated CI job
  - Update `Makefile` / CI `make integration` target to run `go test -tags=integration ./internal/fixrunner/...` after the dev-stack is up.
- [ ] Step 11.3 — verify
  - Local: `pnpm dev:backend` then `cd src-go && go test -tags=integration ./internal/fixrunner/...`.

---

## Task 12 — Spec 2 Trace B fixture extension + smoke wiring

- [ ] Step 12.1 — extend the existing Spec 2 Trace B end-to-end fixture
  - File: `src-go/internal/spec2smoke/trace_b_test.go` (created by 2D scaffold).
  - Add a final stage: after the code_fixer DAG produces a patch, simulate the `http_call` hitting `/api/v1/internal/fix-runs/execute`; assert `fix_runs` row exists with `status='applied'` and the mock VCS recorded `OpenPR` with title prefix `"Fix:"` and label `agentforge:fix`.
- [ ] Step 12.2 — assert original-PR comment edge from §9 Trace B step "update_original_pr"
  - The DAG's follow-up `http_call` posts back to vcs provider; the smoke test asserts mock provider received a `PostReviewComments` call referencing the original inline comment with body containing the new fix PR URL.
- [ ] Step 12.3 — verify
  - `rtk go test -tags=integration ./internal/spec2smoke/...` — green end-to-end.

---

## Task 13 — Old-code deletion guard (ensures Spec 2 §12 cleanup didn't regress)

- [ ] Step 13.1 — grep guard test
  - File: `src-go/internal/fixrunner/legacy_guard_test.go`
    ```go
    func TestNoLegacyRouteFixRequest(t *testing.T) {
        out, _ := exec.Command("git", "grep", "-n", "RouteFixRequest").CombinedOutput()
        if len(out) > 0 { t.Fatalf("legacy reference present: %s", out) }
    }
    func TestNoLegacyEventReviewFixRequested(t *testing.T) {
        out, _ := exec.Command("git", "grep", "-n", "EventReviewFixRequested").CombinedOutput()
        if len(out) > 0 { t.Fatalf("legacy event present: %s", out) }
    }
    ```
- [ ] Step 13.2 — verify
  - `rtk go test ./internal/fixrunner/...` — fails loudly if any future PR re-introduces the dead code paths Spec 2 §12 deleted.

---

## Final verification checklist

- [ ] `rtk go test ./internal/fixrunner/... ./internal/worktree/... ./internal/handler/...`
- [ ] `rtk go test -tags=integration ./internal/fixrunner/... ./internal/spec2smoke/...`
- [ ] `rtk vitest run lib/stores/vcs-integration-store.test.ts app/(dashboard)/projects/\[id\]/integrations/vcs/page.test.tsx app/(dashboard)/reviews/\[id\]/findings/\[fid\]/fix-runs.test.tsx`
- [ ] Manual: post a fake GitHub 401 from mock VCS → integration card shows red "Auth expired" banner → click "Re-bind PAT" → secret rotates → status auto-clears
- [ ] Audit log spot-check: `psql -c "select payload from project_audit_events where action like 'fix_run.%' limit 5"` — confirm only `patch_sha256` keys, no plaintext patch fragments
