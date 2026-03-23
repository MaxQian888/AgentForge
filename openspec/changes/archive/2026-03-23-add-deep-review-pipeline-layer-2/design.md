## Context

The repository already describes a three-layer review architecture in `docs/part/REVIEW_PIPELINE_DESIGN.md` and `docs/PRD.md`, but the runnable codebase is still in an earlier state:

- Layer 1 exists only as a minimal GitHub Action in `.github/workflows/agent-review.yml`.
- The Go server already has task, agent, notification, and review persistence building blocks, including a `reviews` table migration and WebSocket review event names, but no review service or API for Layer 2 orchestration.
- The TS bridge already exposes agent execution and runtime-pool management in `src-bridge/src/server.ts`, but it has no review-specific entrypoint or multi-reviewer orchestration.

Layer 2 therefore needs to be implemented as a repo-grounded extension of the current Orchestrator and Bridge seams rather than as a greenfield subsystem.

## Goals / Non-Goals

**Goals:**

- Add a concrete Layer 2 review flow that can be triggered from GitHub and from authenticated application requests.
- Reuse the existing Go Orchestrator + TS Bridge split so all AI-heavy review execution still flows through the bridge.
- Persist one aggregated Layer 2 review result with structured findings, recommendation, and risk level.
- Feed review completion back into existing task, notification, and real-time event surfaces.
- Make the implementation verifiable with focused backend and bridge tests.

**Non-Goals:**

- Building the full Review Plugin marketplace or general plugin runtime from the broader design docs
- Implementing the full Layer 3 IM approval UI and all downstream cc-connect integrations
- Adding every future review dimension or advanced budget-optimization feature described in research docs
- Replacing Layer 1; this change extends it with escalation and deep review, not a new first-pass reviewer

## Decisions

### 1. Add a dedicated Go review service and API surface

Layer 2 should be represented as a first-class review flow in the Go backend, with explicit trigger and result-handling endpoints instead of hiding review behavior inside task or agent handlers.

- Recommended approach: add a review service plus review handlers and routes such as trigger, status/detail, and result callback endpoints under `/api/v1/reviews`.
- Alternative considered: trigger deep review through task transition endpoints.
- Why not: that would couple review lifecycle semantics to task mutation endpoints and make GitHub and manual triggers harder to reason about.

### 2. Reuse the existing TS bridge as the single AI execution exit

The bridge already owns runtime pooling and request validation for agent execution. Layer 2 should extend that same service with a review-specific execution path instead of introducing a separate review microservice.

- Recommended approach: add a review orchestrator module and request schema in `src-bridge`, with one aggregated response containing dimension outputs, deduplicated findings, recommendation, and cost metadata.
- Alternative considered: GitHub Action calls Claude review agents directly.
- Why not: it would bypass the repository's Go/TS split, fragment review state, and make it harder to persist and replay review outcomes inside AgentForge.

### 3. Store one aggregated Layer 2 review per run in the existing reviews table

The migrations already define fields for `pr_url`, `layer`, `risk_level`, `findings`, `summary`, `recommendation`, and `cost_usd`. The MVP should use that shape directly and place per-dimension metadata inside `findings` entries instead of introducing multiple new review tables.

- Recommended approach: persist one review row per Layer 2 run, with findings carrying dimension-specific metadata such as `category`, `severity`, `file`, and `line`.
- Alternative considered: one row per dimension plus a separate aggregation table.
- Why not: it adds schema and coordination overhead before the repository has any working Layer 2 loop.

### 4. Introduce a separate Layer 2 GitHub workflow driven by Layer 1 outputs

Layer 1 should remain the universal lightweight pass. Layer 2 should run only when escalation criteria are met, either from structured Layer 1 output, agent-authored PR rules, sensitive-file rules, or manual trigger.

- Recommended approach: keep `.github/workflows/agent-review.yml` as Layer 1 and add a dedicated Layer 2 workflow that gathers escalation context and calls the Go review trigger API.
- Alternative considered: fold Layer 2 directly into the existing Layer 1 workflow.
- Why not: that makes escalation harder to control, complicates retries, and blurs the boundary between low-cost review and paid deep review.

## Risks / Trade-offs

- [Layer 1 output is currently too minimal to drive escalation deterministically] -> Extend the workflow to publish or pass the structured data Layer 2 needs, with fallback rules for agent PRs and sensitive files.
- [Current Go review model/repository code does not yet match the richer review migration shape] -> Align model and repository access with the migration-backed schema before wiring orchestration.
- [Parallel review dimensions can increase cost and latency] -> Keep the first implementation fixed to four dimensions and aggregate once, with explicit timeout/error handling per dimension.
- [Automated request-changes handling can create state drift between reviews and tasks] -> Centralize recommendation-to-task transition logic in the Go review service and cover it with unit tests.

## Migration Plan

1. Align Go-side review domain objects and repository methods with the existing database schema for Layer 2 fields.
2. Add backend review APIs and bridge review execution support behind configuration that can be deployed without enabling the GitHub workflow yet.
3. Add the Layer 2 GitHub workflow and secrets/config wiring after backend endpoints are reachable in the target environment.
4. Validate with focused tests plus a dry-run or manual trigger against a non-production PR before broad rollout.

Rollback:

- Disable the Layer 2 GitHub workflow or secret-based trigger path.
- Keep Layer 1 review active so PR review coverage degrades gracefully instead of disappearing.
- Leave persisted review records in place for auditability; no destructive rollback is required.

## Open Questions

- Should Layer 2 trigger authentication use a shared API token only, or should GitHub OIDC be added in a later hardening pass?
- Should manual deep review entry live only in backend APIs for now, or also surface immediately through dashboard actions and `/review deep` command paths?
- Do we want automatic agent session resume on `request_changes` in the first slice, or should the first implementation stop at persisted recommendation plus notification?
