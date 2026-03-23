## Why

AgentForge currently has only a minimal Layer 1 PR review workflow in `.github/workflows/agent-review.yml`, while the repository's PRD and review design docs already define a much richer Layer 2 deep-review pipeline. We need to turn that design into an implementable change now so high-risk and agent-authored PRs can be routed through a real deep-review loop instead of stopping at a single lightweight review pass.

## What Changes

- Add a Layer 2 deep-review capability that can be triggered for agent-authored PRs, Layer 1 escalations, and manual review requests.
- Add Go-side review orchestration APIs and services to create review runs, persist aggregated findings, update task/review status, and emit notification and WebSocket events.
- Extend the TS bridge with a review orchestrator that runs logic, security, performance, and compliance reviewers in parallel and returns a single aggregated result.
- Add GitHub workflow support to invoke Layer 2 after Layer 1 and pass along the PR context needed for deep review.
- Add verification coverage for trigger routing, bridge orchestration, finding aggregation, persistence, and result handling.

## Capabilities

### New Capabilities
- `deep-review-pipeline`: Run Layer 2 deep reviews from trigger through aggregation, persistence, and recommendation delivery.

### Modified Capabilities
- None.

## Impact

- `.github/workflows/agent-review.yml` and a new Layer 2 workflow for GitHub-triggered escalation
- `src-go` review-related handlers, services, repositories, models, routes, notifications, and task transitions
- `src-bridge` review request schemas, orchestrator modules, and aggregation logic
- Tests covering backend review flow and bridge review execution
