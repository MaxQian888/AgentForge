# Code Reviewer Digital Employee (Spec 2)

- **Date**: 2026-04-20
- **Status**: Approved (autonomous-mode brainstorm complete)
- **Owner**: Max Qian
- **Predecessor**: Spec 1 (Foundation Gaps) — `2026-04-20-foundation-gaps-design.md`. All Spec 1 mechanisms (HTTP node, secrets store, IM rich cards, wait_event resumption, outbound dispatcher) are assumed available.
- **Sibling**: Spec 3 (E-commerce Streaming Employee) — independent.

## 1. Problem

The existing review pipeline can run linters and LLM reviewers on a diff and store findings, but the loop never closes:

- No GitHub webhook → reviews must be manually triggered via `/review <url>` from Feishu;
- No `code_fixer` node → suggestions stay as text, never become applied patches;
- No PR-side feedback → findings live only in AgentForge's FE;
- No per-finding actionable UX → users can't approve / dismiss / apply at the finding granularity;
- `RouteFixRequest()` broadcasts `EventReviewFixRequested` but **no subscriber consumes it** — it's dead code.

Result: AgentForge has the bones of a "code reviewer digital employee" but it doesn't actually do the job a human reviewer does (post comments, propose fixes, follow up on push, apply approved changes).

## 2. Goals

- A configured GitHub repo's PR open / push events trigger an AgentForge review automatically; results post back as **a single summary PR comment + per-line inline review comments** within seconds.
- Each finding shows up in the original Feishu trigger thread (when initiated from Feishu) **and** in AgentForge's FE diff viewer with **per-finding Approve / Dismiss / Apply** buttons.
- Clicking **Apply** on a finding spawns a `code_fixer` flow that produces a unified diff, applies it in an isolated worktree, pushes a `fix/<review_id>/<finding_id>` branch, and opens a **separate fix PR** back to the original PR's base. Original PR gets a comment linking to the fix PR.
- VCS abstraction (`adsplatform`-style `vcs.Provider` interface) makes GitLab / Gitea / Bitbucket future additions structural, not architectural.
- Re-review on push uses **diff-of-diff**: only re-evaluate files that changed since the last review SHA. No wasted LLM tokens.
- Multi-PR concurrency safe: per-repo FIFO queue + per-employee concurrent run cap (default 5).

## 3. Non-Goals (deferred / out-of-scope)

- **Auto-apply fixes without human approval** ("D" mode from brainstorm) — risk classification is its own research problem.
- **GitHub `suggestion` review block** for tiny single-file patches — possible future optimization. v1 always opens a fix PR.
- **GitLab / Gitea / Bitbucket renderers** — interface defined, only GitHub implemented.
- **Cross-repo / mono-repo path-scoped reviewers** — assume one PR = one repo = one review run.
- **Reviewer plugin discovery / marketplace** — already covered by separate marketplace service; this spec consumes whatever plugins are configured.
- **Custom risk-level → reviewer-plan mapping** beyond the existing 3 layers (CI / Deep / Human) — orthogonal.
- **Stack-on-top-of-PR fixes (option B)** and **direct push to PR head (also option B)** — explicitly rejected; fix PR is the only branching strategy.
- **Re-review SLA / queue priority** — best-effort FIFO; SLA work deferred.
- **Old code cleanup**: per `feedback_autonomous_multistep` and `project_api_stability_stage`, delete `RouteFixRequest` + `EventReviewFixRequested` event in the same PR that introduces the new automation rule (no parallel paths).

## 4. Validated Design Decisions

| # | 决策 | 备选 (rejected) | 理由 |
|---|---|---|---|
| 1 | 范围 = 审查 + 修复 + 一键应用 | A 仅审查 / B 仅建议 / D 自动应用 | 数字员工应"做事"，非"提建议"；D 风险分级太重，留后续 |
| 2 | VCS provider-neutral，GitHub first | A 仅 GitHub / C 全平台 | 跟 Spec 1 card-provider neutral 模式一致；30% 上层成本换未来无锁死 |
| 3 | Patch 生成 = 混合（plugin patch 优先 + fixer agent 兜底） | A 仅 LLM / B 仅 plugin / C 仅 agent | 简单 finding 0 LLM 成本，复杂 finding 才付代价；agent 走 role 体系 |
| 4 | 修复分支策略 = 新 fix branch + 新 PR | B 直推原 PR / C suggestion-review | 不动作者分支；员工产物 = 一个独立可 review/merge/丢的 PR |
| 5 | 推送 re-review = diff-of-diff | A 全量 / C 仅手动 / D 配置 | 持续值守不浪费；只需 reviews 加一个 `last_reviewed_sha` 列 |
| 6 | PR 反馈 = summary comment + per-line inline review comments | 仅 summary / 仅 inline / status check | 摘要供决策、inline 供精读；CI status 是后续优化 |
| 7 | 并发 = per-employee 5 同时 + per-repo FIFO Redis 锁 | 全局队列 / 不限 / per-PR 锁 | 防止同 repo 多 PR 同时改 worktree；员工级软上限避雪崩 |
| 8 | 手动 `/review <url>` 保留并存 | webhook-only | 不破坏现有 IM 命令；webhook + manual 都走 `ReviewService.Trigger` |
| 9 | Webhook 安全 = HMAC-SHA256，shared secret 存 1B secrets | 明文 token / 仅 IP allowlist | 复用 1B 加密 + 轮换；secret name 约定 `vcs.github.<repo_id>.webhook_secret` |
| 10 | Patch 应用失败 = 标 `needs_manual_fix` + PR comment 解释 | 整 review fail / 重试 | 单 finding 失败不应污染整 review |
| 11 | Suggestion-review 小补丁优化 → 后续 | 集成进 v1 | 大部分 finding 跨多行，套 suggestion 价值有限；先做完 fix PR 路径再优化 |
| 12 | 大量复用 Spec 1：HTTP 节点（调 GitHub API）、密钥（PAT + webhook secret）、IM 卡片（带按钮）、wait_event 恢复、自动外发 | 重新写一套 | 一致性 + 不重复 |
| 13 | 删除 `RouteFixRequest` + `EventReviewFixRequested`（dead code） | 保留 + 跳过 | 项目内部测试期，对齐"breaking changes freely permitted" + "active cleanup" 记忆 |
| 14 | Fixer agent = 一个 **role**（同 Code Reviewer role 体系），不是硬编码节点逻辑 | 节点内联 | 跟 employee/role 心智一致；可项目级换 model/prompt/skills |

## 5. Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│  GitHub.com                                                      │
│  ├─ webhook → POST /api/v1/vcs/github/webhook (HMAC-verified)    │
│  ├─ PR comment / review API ← VCS Provider outbound calls        │
│  └─ Fix PR opens                                                 │
└──────────┬───────────────────────────────────────────────────────┘
           │
┌──────────┴───────────────────────────────────────────────────────┐
│  Go Backend (src-go)                                             │
│                                                                   │
│  S2-A vcs_webhook_handler (NEW)                                  │
│      └─ verify HMAC (secret from 1B store)                       │
│      └─ resolve vcs_integration → fire ReviewService.Trigger     │
│                                                                   │
│  S2-B vcs_outbound_dispatcher (NEW, parallel to 1D's IM版)        │
│      ←── EventReviewCompleted (existing)                          │
│      └─ build summary card + per-line inline comments            │
│      └─ adsplatform / vcs.Provider.PostReview()                  │
│                                                                   │
│  S2-C automation_rules.review_completed (NEW)                    │
│      ←── EventReviewCompleted                                     │
│      └─ for each eligible finding (severity≥threshold + has      │
│         fixable suggestion or pre_baked patch):                  │
│           emit IM card via 1E im_send w/ Apply/Dismiss buttons   │
│                                                                   │
│  S2-D code_fixer DAG (canonical, NEW)                            │
│      trigger → fetch file content (HTTP) → has pre_baked? then   │
│      patch ; else llm_agent role:'code_fixer' → produce patch    │
│      → fix_runner.Execute (via internal HTTP) → im_send result   │
│                                                                   │
│  S2-E fix_runner_service (NEW Go internal)                       │
│      POST /api/v1/internal/fix-runs/execute                      │
│      ├─ acquire per-repo FIFO Redis lock                         │
│      ├─ allocate worktree (existing worktree.Manager)            │
│      ├─ git apply <patch> ; commit ; push fix/<rid>/<fid>        │
│      ├─ vcs.Provider.OpenPR(base, head, title, body)             │
│      └─ release lock + return fix_pr_url                         │
│                                                                   │
│  S2-F findings_decision_handler (NEW)                            │
│      POST /api/v1/findings/:id/decision {approve|dismiss|defer}  │
│      └─ on approve → spawn code_fixer DAG                        │
│      └─ on dismiss → mark finding dismissed=true                 │
│      └─ on defer → no-op (just mark seen)                        │
│                                                                   │
│  S2-G internal/vcs/ provider abstraction (NEW)                   │
│      vcs.Provider interface + github/ implementation             │
│                                                                   │
│  S2-H DELETE: RouteFixRequest + EventReviewFixRequested          │
│      (dead code per investigation; clean cut, no parallel)       │
└──────────┬───────────────────────────────────────────────────────┘
           │
┌──────────┴───────────────────────────────────────────────────────┐
│  Frontend (Next.js)                                               │
│  ├─ /reviews/[id] — diff viewer (NEW component) + per-finding    │
│  │     Approve / Dismiss / Apply buttons (POST decision API)     │
│  ├─ /reviews/[id]/findings/[fid] — per-finding detail with       │
│  │     proposed patch preview, fix-run history                   │
│  └─ /projects/[pid]/integrations/vcs — VCS integrations CRUD     │
│        (add GitHub repo, store PAT via 1B secrets, webhook URL)  │
└──────────────────────────────────────────────────────────────────┘
```

**关键不变量**：
- 现有 `ReviewService.Trigger` / `ReviewService.Complete` 不变；webhook 与手动 `/review` 都路由到它。
- 现有 `human_review` 节点不变（仍可被作者在自定义 workflow 里使用）；本 spec 引入的是**专为代码审查→修复**的 canonical DAG `system:code_fixer`。
- 现有 worktree.Manager 不动；fix_runner_service 新增"per-repo FIFO 锁"层在它之上。

## 6. Data Model

### 6.1 New tables

```sql
-- one row per (project, repo) pair
vcs_integrations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  provider varchar(16) NOT NULL,          -- 'github' (only one in v1)
  host varchar(256) NOT NULL,             -- 'github.com' / 'github.acme.corp'
  owner varchar(128) NOT NULL,
  repo varchar(128) NOT NULL,
  default_branch varchar(128) NOT NULL DEFAULT 'main',
  webhook_id varchar(64),                 -- GitHub-side webhook ID (for cleanup)
  webhook_secret_ref varchar(128) NOT NULL,  -- → secrets.name in 1B store
  token_secret_ref varchar(128) NOT NULL,    -- → secrets.name (PAT)
  status varchar(16) NOT NULL DEFAULT 'active',  -- 'active' | 'auth_expired' | 'paused'
  acting_employee_id uuid REFERENCES employees(id) ON DELETE SET NULL,
  last_synced_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, provider, host, owner, repo)
);

vcs_webhook_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  integration_id uuid NOT NULL REFERENCES vcs_integrations(id) ON DELETE CASCADE,
  event_id varchar(128) NOT NULL,         -- GitHub X-GitHub-Delivery
  event_type varchar(32) NOT NULL,        -- 'pull_request', 'push', etc.
  payload_hash bytea NOT NULL,            -- sha256 of raw body for dedup
  received_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  processing_error text,
  UNIQUE (integration_id, event_id)       -- dedup window
);

fix_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  review_id uuid NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
  finding_id uuid NOT NULL,               -- review_findings.id
  source varchar(32) NOT NULL,            -- 'pre_baked' | 'agent_generated'
  worktree_path text,
  fix_branch_name varchar(128),
  fix_pr_url text,
  patch text,                             -- the unified diff applied
  status varchar(16) NOT NULL DEFAULT 'pending',
                                           -- pending | running | applied | conflict | failed
  apply_attempts int NOT NULL DEFAULT 0,
  decided_by uuid,                         -- user who clicked Apply
  decided_via varchar(16),                 -- 'fe' | 'feishu_card' | 'github_label' | 'auto'
  acting_employee_id uuid REFERENCES employees(id) ON DELETE SET NULL,
  error_message text,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);

CREATE INDEX idx_fix_runs_review ON fix_runs(review_id);
CREATE INDEX idx_fix_runs_finding ON fix_runs(finding_id);
```

### 6.2 reviews table extensions (add columns; do not drop)

```sql
ALTER TABLE reviews
  ADD COLUMN integration_id uuid REFERENCES vcs_integrations(id) ON DELETE SET NULL,
  ADD COLUMN head_sha varchar(40),
  ADD COLUMN base_sha varchar(40),
  ADD COLUMN last_reviewed_sha varchar(40),  -- for diff-of-diff
  ADD COLUMN summary_comment_id varchar(64),  -- VCS-side comment ID for idempotent edit
  ADD COLUMN automation_decision varchar(16) NOT NULL DEFAULT 'manual_only';
                                              -- 'manual_only' | 'auto_propose'
```

### 6.3 review_findings table extensions

```sql
ALTER TABLE review_findings
  ADD COLUMN suggested_patch text,           -- pre-baked patch from plugins (golangci-lint --fix etc.)
  ADD COLUMN decision varchar(16) NOT NULL DEFAULT 'pending',
                                              -- pending | approved | dismissed | needs_manual_fix
  ADD COLUMN decided_at timestamptz,
  ADD COLUMN decided_by uuid,
  ADD COLUMN inline_comment_id varchar(64),  -- VCS-side inline comment ID for idempotent edit
  ADD COLUMN active_fix_run_id uuid REFERENCES fix_runs(id) ON DELETE SET NULL;
```

## 7. API Endpoints (new)

```
S2-A VCS Webhook
  POST /api/v1/vcs/github/webhook
    Headers: X-GitHub-Event, X-GitHub-Delivery, X-Hub-Signature-256
    Body: GitHub event payload (raw bytes for HMAC verify)
    202 on accepted; 401 on bad signature; 409 on dedup

S2-G VCS Integrations CRUD
  GET    /api/v1/projects/:pid/vcs-integrations
  POST   /api/v1/projects/:pid/vcs-integrations
         body: {provider, host, owner, repo, default_branch, token_secret_ref,
                webhook_secret_ref, acting_employee_id}
         (creates GitHub-side webhook automatically)
  PATCH  /api/v1/vcs-integrations/:id
  DELETE /api/v1/vcs-integrations/:id  (also removes GitHub-side webhook)
  POST   /api/v1/vcs-integrations/:id/sync   (manual re-sync; pulls open PRs)

S2-F Finding decision
  POST /api/v1/findings/:id/decision
    body: {action: 'approve'|'dismiss'|'defer', comment?}
    on approve → 202 + spawned_fix_run_id

S2-E Internal fix runner (called by code_fixer DAG via http_call node)
  POST /api/v1/internal/fix-runs/execute
    body: {review_id, finding_id, patch, employee_id}
    runs synchronously: returns {fix_run_id, fix_pr_url, status, error?}

S2-D Manual code_fixer trigger (FE convenience)
  POST /api/v1/fix-runs
    body: {finding_id}
    (equivalent to /findings/:id/decision with approve)

S2-B (no new endpoint — vcs_outbound_dispatcher is an in-process subscriber)

  Error codes:
    vcs:webhook_signature_invalid, vcs:integration_not_found,
    vcs:repo_unreachable, vcs:rate_limited, vcs:auth_expired,
    fix_run:patch_conflict, fix_run:dry_run_failed, fix_run:repo_locked
```

## 8. VCS Provider Interface (Go)

```go
// src-go/internal/vcs/provider.go
package vcs

type Provider interface {
    Name() string                                        // "github" / "gitlab" / ...
    
    // PR lifecycle
    GetPullRequest(ctx, repo RepoRef, number int) (*PullRequest, error)
    ComparePullRequest(ctx, repo RepoRef, base, head string) (*Diff, error)
    
    // Comments
    PostSummaryComment(ctx, pr *PullRequest, body string) (commentID string, err error)
    EditSummaryComment(ctx, pr *PullRequest, commentID string, body string) error
    
    // Inline review comments (one per finding)
    PostReviewComments(ctx, pr *PullRequest, comments []InlineComment) (ids []string, err error)
    EditReviewComment(ctx, pr *PullRequest, commentID string, body string) error
    
    // Fix flow
    OpenPR(ctx, repo RepoRef, base, head, title, body string, opts OpenPROpts) (*PullRequest, error)
    
    // Webhook lifecycle
    CreateWebhook(ctx, repo RepoRef, callbackURL, secret string, events []string) (id string, err error)
    DeleteWebhook(ctx, repo RepoRef, id string) error
}

type RepoRef struct{ Host, Owner, Repo string }
type InlineComment struct{ Path string; Line int; Body string; Side string /* "RIGHT"|"LEFT" */ }
type OpenPROpts struct{ Draft, AutoMerge bool; Labels []string }
```

GitHub implementation in `src-go/internal/vcs/github/`. Mock provider in `src-go/internal/vcs/mock/` for tests. GitLab/Gitea stubs only — return `errors.ErrUnsupported`.

## 9. End-to-End Data Flows

### Trace A — PR opened → review → cards posted

```
[GitHub] PR #42 opened on owner/repo
     → POST /api/v1/vcs/github/webhook
        Headers: X-GitHub-Event=pull_request, X-Hub-Signature-256=sha256=<hmac>
[backend] vcs_webhook_handler:
     1. lookup vcs_integration by (host, owner, repo)
     2. resolve webhook_secret via 1B secrets store
     3. HMAC-SHA256 verify body → if mismatch → 401
     4. dedup via vcs_webhook_events.UNIQUE(integration_id, event_id) → if dup → 200 noop
     5. fetch PR head_sha, base_sha
     6. ReviewService.Trigger({
          pr_url, integration_id, head_sha, base_sha,
          replyTarget: vcs_pr_thread{integration_id, pr_number},
          acting_employee_id: integration.acting_employee_id
        })
[ReviewService] (existing, unchanged) plans + dispatches to bridge plugins
[bridge] plugins emit findings → ReviewService.Complete → broadcast EventReviewCompleted

[S2-B vcs_outbound_dispatcher] subscribes EventReviewCompleted
     → vcs.Provider.PostSummaryComment(pr, "## AgentForge Review\n…N findings…")
     → vcs.Provider.PostReviewComments(pr, [InlineComment per finding with file/line])
     → save returned IDs to reviews.summary_comment_id + review_findings.inline_comment_id
[Spec 1's IM dispatcher] also subscribes; if review.replyTarget is Feishu, posts default card
[S2-C automation_rule] subscribes; for each fixable finding emits Spec 1 im_send card
     with Apply/Dismiss buttons → card_action_correlations rows minted ✅
```

### Trace B — User clicks "Apply" → code_fixer DAG → fix PR opened

```
[Feishu user] clicks "Apply" button on a finding card
[IM Bridge] inbound card_action → POST /api/v1/im/card-actions {token, action_id:'apply', payload}
[Spec 1 card_action_router] lookup token → execution_id + node_id → wait_event resumer
     resumes parked code_fixer DAG (the automation rule emitted im_send THEN parked at wait_event)

[code_fixer DAG] resumes:
  Node fetch_file: http_call → GET https://api.github.com/repos/{owner}/{repo}/contents/{path}?ref={head_sha}
                   Headers: Authorization: Bearer {{secrets.<integration.token_secret_ref>}}
                   → dataStore[fetch_file] = {content_b64, sha}
  Node has_prebaked: condition → finding.suggested_patch != null
    yes:
      Node use_prebaked: function → patch = finding.suggested_patch
    no:
      Node generate: llm_agent role='code_fixer' (project-configurable model)
                     input: {file_content, finding, surrounding_context}
                     output: unified_diff
                     → dataStore[generate] = {patch}
  Node validate: function → validate patch syntax + dry-run apply via fix_runner.DryRun
  Node decide: condition → dry_run_ok
    yes:
      Node execute: http_call → POST /api/v1/internal/fix-runs/execute
                    body: {review_id, finding_id, patch, employee_id: acting_employee_id}
                    → fix_runner_service does:
                       1. acquire Redis lock key=`fixlock:<integration_id>:<repo>` (FIFO via list LPUSH/BRPOP)
                       2. worktree.Manager.Allocate(repo, branch=fix/<rid>/<fid> from head_sha)
                       3. exec git apply <patch>
                       4. exec git commit -m "fix(review #N): <finding.message>"
                       5. exec git push origin fix/<rid>/<fid>
                       6. vcs.Provider.OpenPR(base=pr.base_branch, head=fix/<rid>/<fid>,
                                              title="Fix: <finding.message>",
                                              body=<finding details + link to original PR + link to AgentForge review>,
                                              opts=Draft:false, Labels:["agentforge:fix"])
                       7. release lock; persist fix_runs row with fix_pr_url
                       → return {fix_run_id, fix_pr_url}
      Node update_original_pr: http_call → vcs.Provider.PostReviewComments
                               (reply to original inline comment) "✅ Applied as #43"
      Node card: im_send → "Applied. PR #43 opened" card to original Feishu thread
    no:
      Node mark_manual: function → finding.decision = needs_manual_fix
      Node comment_pr: http_call → post comment "Could not auto-apply: <reason>"
      Node card: im_send → "Could not apply (conflict). Marked for manual fix"
```

### Trace C — Push to PR → diff-of-diff re-review

```
[GitHub] PR #42 head moves: new commit pushed, head_sha=Y (was X)
     → POST /api/v1/vcs/github/webhook (event_type=push, branch=refs/pull/42/head OR a regular PR sync)
[vcs_webhook_handler]:
     1. dedup; 2. find existing reviews row for this PR with last_reviewed_sha=X
     3. vcs.Provider.ComparePullRequest(repo, base=X, head=Y) → changed_files
     4. if changed_files ⊆ already-reviewed-without-findings (heuristic skip) → 200 noop
     5. else: ReviewService.TriggerIncremental({
          pr_url, integration_id, head_sha=Y, base_sha=X,
          changed_files,                  -- ReviewService scopes plugins to this set
          parent_review_id, acting_employee_id
        })
[ReviewService] runs only on changed_files; produces delta findings
[ReviewService.Complete] update reviews.last_reviewed_sha = Y
[S2-B vcs_outbound_dispatcher]:
     - update existing summary comment via vcs.Provider.EditSummaryComment(summary_comment_id, new_body)
     - for new findings: PostReviewComments
     - for stale findings (file no longer changed in this diff): EditReviewComment to "(superseded)"
       OR delete via vcs.Provider — keep policy explicit in §11
```

### 边界与假设

- **manual `/review <url>` 路径不变**：existing IM bridge command path stays, ends in same `ReviewService.Trigger`. Only difference: replyTarget is the Feishu thread (existing), not vcs_pr_thread. Both dispatchers (1D IM + S2-B VCS) subscribe; each renders to its own target if applicable.
- **automation_decision='manual_only'**：S2-C 不发 cards, FE 仍可逐 finding apply.
- **acting_employee_id 链**：integration.acting_employee_id → reviews.acting_employee_id → fix_runs.acting_employee_id → 全链可被员工运行历史 (Spec 1A) 看到.
- **Fix PR 不会触发递归 review**：S2-C 在收到 EventReviewCompleted 时跳过 head branch 形如 `fix/*`（policy）.

## 10. Error Handling

| 场景 | 行为 |
|---|---|
| Webhook HMAC 不匹配 | 401，记 audit `vcs:webhook_signature_invalid`；不入 vcs_webhook_events |
| Dup webhook delivery | 200 noop（UNIQUE 约束捕获）；不影响 dedup window 内的合法重试 |
| GitHub 4xx (auth_expired) | 标 vcs_integrations.status='auth_expired'；emit `EventVCSAuthExpired`；FE 显示横幅；暂停所有该 integration 的自动 review |
| GitHub 5xx / 限流 | 指数退避 3 次（1s/4s/16s）；最终失败 emit `EventVCSDeliveryFailed{review_id, op}`；FE review 详情显示 "PR comment 发送失败" badge |
| Plugin 失败 | 单 plugin 失败不影响 review.complete；finding.sources 标 plugin 错误；layer 整体失败才 review.failed |
| `code_fixer` LLM 生成 patch 语法非法 | validate 节点 fail → mark needs_manual_fix + comment |
| Dry-run apply 失败（merge conflict） | execute 节点不进 fix_runner；mark needs_manual_fix + comment "patch conflicts with current head" |
| fix_runner 推 branch 失败（push rejected） | release lock + retry once with rebase; 仍失败 → fix_run.status=failed + comment + im_send "推送失败 <reason>" |
| Per-repo lock 等待超时（>2 min） | fix_run.status=timeout; rerun via FE button |
| Stale findings on push (file 不再 in diff) | default policy: EditReviewComment → "(已修复或被 superseded)"（不删除，保留审计） |
| automation rule emit fails (e.g. no Feishu thread) | log + emit metric; review 状态不受影响 |

## 11. Security

- **Webhook HMAC**：每 integration 独立 secret（存 1B secrets，名 `vcs.<provider>.<integration_id>.webhook_secret`），HMAC-SHA256 over raw body。Constant-time compare。
- **PAT 范围**：建议 fine-grained PAT，scope=`pull_requests:write, contents:write, metadata:read`。FE 创建 integration 时校验 PAT 有效性。
- **Internal endpoints**: `/api/v1/internal/fix-runs/execute` only callable from local network or with internal HMAC; configurable via `AGENTFORGE_INTERNAL_TOKEN`。
- **审计**：vcs_integrations CRUD / fix_runs / finding decisions 全写 audit；patch 内容**不写** audit payload（hash 即可，避免敏感代码泄露到 log 系统）。
- **Per-repo lock TTL**：默认 10 min；超时自动释放，避免锁泄露阻塞队列。
- **`code_fixer` role 默认沙箱**：no shell access；只能调 `git apply / git commit / git push` 通过 fix_runner_service；不能 spawn 进程。
- **Worktree 隔离**：每 fix_run 一个 worktree；run 结束自动 cleanup（保留最近 N=10 用于 debug，超过 LRU 淘汰）。
- **CORS / 鉴权**：所有非 webhook 端点走现有 JWT + project RBAC。

## 12. Old Code Deletion Checklist

```
delete  src-go/internal/service/review_service.go::RouteFixRequest()
        + 所有调用点（grep `RouteFixRequest`，全删）
delete  EventReviewFixRequested 事件类型
        （src-go/internal/eventbus/types.go + 任何 emit/handler）
delete  src-go/internal/handler/review_fix_handler.go (if exists)
delete  src-im-bridge/commands/review.go::renderFollowupTaskSuggestions
        （现在 IM card 直接带 Apply 按钮，不再需要"建议跑 /task create"提示）
modify  src-go/internal/service/review_service.go: 把现 broadcast EventReviewFixRequested 的代码段
        替换为 EventReviewCompleted 已经做的事；不再单独 emit fix-request 事件
⚠️ 不并行保留旧路径，不加 feature flag（项目内部测试期 + active cleanup 默认）
```

## 13. Testing

```
Unit Go
  ├─ vcs_webhook_handler: HMAC verify 矩阵（valid / mismatch / missing header / wrong secret）
  ├─ vcs_webhook_events dedup: same event_id → 200 noop
  ├─ vcs_outbound_dispatcher: summary post + per-line inline (mock vcs.Provider)
  ├─ vcs_outbound_dispatcher: edit on re-review (idempotent comment IDs)
  ├─ automation_rules.review_completed: emit cards only for fixable + severity≥threshold
  ├─ fix_runner_service: lock acquire/release; worktree allocate; patch apply success/conflict
  ├─ fix_runner_service: push reject + rebase + retry
  ├─ vcs/github: mock HTTP server snapshot tests (PostSummaryComment, OpenPR, ComparePullRequest)
  └─ findings_decision_handler: approve→spawn / dismiss→mark / defer→noop

Integration Go (PG + Redis + mock VCS HTTP)
  ├─ Trace A end-to-end: simulated webhook → review pipeline → both dispatchers fire (assert PR comment posted + Feishu im_send token minted)
  ├─ Trace B end-to-end: card action callback → wait_event resume → code_fixer DAG → fix_runner → mock GitHub gets OpenPR call
  └─ Trace C: push event w/ new SHA → diff-of-diff → only changed files re-reviewed → summary edited

E2E smoke
  ├─ smoke fixture: GitHub webhook payload (PR opened) → end-to-end via real DB + mock GitHub
  └─ smoke fixture: PR push event → diff-of-diff path

FE Jest
  ├─ Diff viewer: monaco-diff or react-diff-viewer (decide in plan); finding markers; Apply/Dismiss buttons → POST /findings/:id/decision
  ├─ VCS integrations CRUD form: PAT validation + webhook URL display + secret-ref selector (uses 1B secrets store)
  ├─ Per-finding detail view: patch preview (when generated); fix run history table
  └─ Snapshot: review summary card (FE) shows decision badges per finding
```

## 13.1 Spec Drifts Found During Brainstorm

(To be filled by plan-writer subagents — same convention as Spec 1.)

## 14. Open Risks

- **Bridge plugin → finding patch 字段需扩展**：要求 reviewer plugins 在 `findings/v1` 结构里加 `suggested_patch` 字段。已有 plugins 不会自动产 patch，但兼容（缺省 null 走 fixer agent 路径）。Plan 应包含一项：升级 `findings/v1` 至 `findings/v2`（带 patch 字段）+ 现有 plugins 的兼容映射。
- **`code_fixer` role 不存在**：当前 role 体系（`role/`）有 `default-code-reviewer`，需新增 `default-code-fixer`（YAML manifest），plan 包含。
- **Worktree pool 资源**：高并发时可能瓶颈。当前 `worktree.Manager` 不知道是否有上限。Plan 应 review。
- **GitHub PR comment 长度上限** (~65535 chars)：单 review 极端情况（数百 finding）的 summary comment 超长。Plan 应实现"截断 + 完整列表跳 FE"。
- **Diff-of-diff 漏判风险**：仅按文件名过滤会漏掉"未改但被改文件 import 的"语义影响。可接受（v1）；后续可加跨文件依赖图。
- **`fix/<rid>/<fid>` branch 命名冲突**：同一 finding 多次 apply（手动 retry）会冲突。命名加 `-attempt-N` 后缀；plan 包含。
- **Webhook URL 暴露**：integration 创建时生成的 callback URL 形如 `<AGENTFORGE_PUBLIC_BASE>/api/v1/vcs/github/webhook`，若实例无公网，作者需配 reverse proxy。文档中明确。

## 15. Successor Spec Hooks

- **Spec 4 (potential): GitLab / Gitea 扩展** — 仅实现 vcs.Provider 接口；零 schema/UX 改动。
- **Spec 4.1: Suggestion-review 优化** — 单文件 patch < N 行 → 走 GitHub `suggestion` review block 而非 fix PR；fix_runner 增加分支策略选择。
- **Spec 4.2: Auto-apply（D 模式）** — 风险分级 + 安全 finding 自动应用；需要 ML 模型 + 项目级配置。
- **Spec 4.3: 跨文件 / 多 finding 合并 fix PR** — 当前每 finding 一 PR；批量审批时合并到一 PR。
- **Spec 4.4: Status check 集成** — fix PR 在原 PR 上注册为 required CI check；阻塞 merge until applied。
