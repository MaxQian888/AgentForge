## 1. Bridge runtime setup

- [x] 1.1 Confirm the Claude Agent SDK package/version to use in `src-bridge` and add the dependency plus any required bridge configuration.
- [x] 1.2 Introduce a bridge-local SDK adapter that converts canonical execute requests into Claude Agent SDK query options without changing unrelated server responsibilities.

## 2. Real execution flow

- [x] 2.1 Replace `simulateAgentExecution(...)` in `src-bridge/src/handlers/execute.ts` with the real SDK-backed execution path.
- [x] 2.2 Normalize Claude Agent SDK output into the existing `AgentEvent` categories and keep `AgentRuntime` bookkeeping in sync for status, turn count, last tool, and spend.
- [x] 2.3 Enforce bridge-local budget and abort behavior during real execution, including terminal error handling for cancellation and budget exhaustion.

## 3. Contract alignment and continuity

- [x] 3.1 Align the Go bridge client with the canonical `/bridge/*` HTTP contract and payload naming used by the TypeScript bridge.
- [x] 3.2 Extend bridge session/runtime handling so the latest continuity metadata or snapshot state is stored when runs complete, fail, or are cancelled.
- [x] 3.3 Ensure status, cancel, and health endpoints report truthful runtime state after the real execution path is wired in.

## 4. Verification

- [x] 4.1 Add or update focused bridge tests for contract validation, event normalization, and cancellation/budget behavior.
- [x] 4.2 Run the relevant bridge and Go verification commands for the changed surfaces and confirm the real execution contract is apply-ready.
