## 1. Snapshot And Type Contracts

- [x] 1.1 Extend `src-bridge/src/types.ts` and any related schema/types to represent Claude continuity metadata, resume readiness, and machine-readable blocking reasons without breaking existing status fields.
- [x] 1.2 Update `src-bridge/src/session/manager.ts` and snapshot read/write helpers so persisted session snapshots can store the new continuity structure and still load legacy request-only snapshots safely.
- [x] 1.3 Add or update focused unit tests for session snapshot serialization and backward-compatible restore behavior.

## 2. Claude Runtime Launch And Continuity Capture

- [x] 2.1 Refactor `src-bridge/src/handlers/claude-runtime.ts` to build one canonical Claude launch context from the normalized execute request, including resolved model, permission mode, tool allowlist, MCP configuration, and supported continuity inputs.
- [x] 2.2 Add a Claude continuity extractor/update path that captures resumable runtime metadata from SDK events or terminal results and writes it back to runtime state and snapshots.
- [x] 2.3 Expand Claude runtime tests to cover launch tuple preservation, continuity capture, and terminal snapshot persistence for pause, failure, and budget-stop paths.

## 3. Resume And Status Lifecycle Behavior

- [x] 3.1 Change `/bridge/resume` handling in `src-bridge/src/server.ts` and related execution helpers so `claude_code` resumes through persisted continuity state instead of replaying a fresh execute request.
- [x] 3.2 Return explicit resume-readiness errors for Claude snapshots that only contain legacy request data or otherwise fail continuity preconditions.
- [x] 3.3 Extend active/paused status or snapshot payload generation so Claude-backed runs report continuity readiness and blocking reason while preserving existing runtime/provider/model identity fields.

## 4. Focused Verification And Documentation

- [x] 4.1 Add or update bridge integration tests in `src-bridge/src/server.test.ts` and `src-bridge/src/handlers/execute.test.ts` for execute -> pause -> resume, missing continuity -> explicit failure, and legacy snapshot compatibility.
- [x] 4.2 Verify focused bridge tests pass for the touched Claude runtime and session modules, and record any remaining gaps if SDK-level behavior must still be mocked.
- [x] 4.3 Update any TS Bridge lifecycle documentation or inline contract notes that currently describe replay-based resume as sufficient for Claude-backed runs.
