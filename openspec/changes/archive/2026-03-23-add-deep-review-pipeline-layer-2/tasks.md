## 1. Backend Review Flow

- [x] 1.1 Align Go review models and repositories with the existing review table fields needed for Layer 2 results
- [x] 1.2 Add review service, handlers, and routes for triggering, querying, and completing Layer 2 deep reviews
- [x] 1.3 Wire review completion into task updates, WebSocket review events, and notification creation
- [x] 1.4 Add backend tests for review trigger validation, persistence, and recommendation handling

## 2. Bridge Deep Review Execution

- [x] 2.1 Add bridge request and response schemas for deep review execution
- [x] 2.2 Implement a review orchestrator that runs logic, security, performance, and compliance reviewers in parallel
- [x] 2.3 Add aggregation and deduplication logic that returns findings, risk level, recommendation, and cost metadata
- [x] 2.4 Add bridge tests for success, partial-failure, and duplicate-finding scenarios

## 3. GitHub Workflow Integration

- [x] 3.1 Extend Layer 1 review output or metadata so Layer 2 can consume escalation context reliably
- [x] 3.2 Add a dedicated Layer 2 workflow that triggers deep review for escalated or agent-authored pull requests
- [x] 3.3 Configure secure backend invocation for the Layer 2 workflow and document required secrets or environment variables

## 4. Verification And Rollout

- [x] 4.1 Verify the end-to-end deep review flow with targeted backend and bridge test runs
- [x] 4.2 Dry-run or manually trigger Layer 2 against a sample pull request and confirm persisted review output plus emitted events
- [x] 4.3 Update any implementation-facing docs needed to explain Layer 2 trigger and runtime behavior
