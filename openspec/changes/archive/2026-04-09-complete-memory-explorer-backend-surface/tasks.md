## 1. Align the core memory explorer contract

- [x] 1.1 Audit `src-go/internal/handler/memory_handler.go`, `src-go/internal/service/memory_service.go`, `src-go/internal/model/agent_memory.go`, and `lib/stores/memory-store.ts` to define the canonical explorer query/response shape, including `query`/legacy `q`, `scope`, `category`, `roleId`, `startAt`, `endAt`, and `limit`.
- [x] 1.2 Extend the Go-side request parsing and DTO shaping so memory list responses expose explorer-ready timestamps and metadata fields, and add a detail path for `/api/v1/projects/:pid/memory/:mid`.
- [x] 1.3 Update repository/service seams so explorer list and detail requests honor the aligned filters instead of silently ignoring them, while preserving access-count updates and existing project isolation.

## 2. Expose episodic explorer workflows through the backend API

- [x] 2.1 Wire `src-go/internal/service/episodic_memory_service.go` into the routed backend surface so explorer history queries can reuse existing episodic date-range and role-scope logic instead of duplicating it in handlers.
- [x] 2.2 Add authenticated stats and export endpoints for memory explorer, reusing existing repository/service data to return filtered summary counts, approximate storage usage, and JSON episodic export payloads.
- [x] 2.3 Add authenticated management endpoints for bulk deletion and age-based episodic cleanup, ensuring project isolation and current role-scoped access rules remain authoritative.

## 3. Verify backend truth and consumer alignment

- [x] 3.1 Add or extend focused Go tests for memory handlers, services, and repository filters covering `query`/`q` compatibility, detail retrieval, stats, export, bulk delete, cleanup, and role-scoped access boundaries.
- [x] 3.2 Update `lib/stores/memory-store.ts` (and any directly affected memory consumer seams) to call the canonical backend contract instead of relying on currently ignored query parameters.
- [x] 3.3 Run targeted verification for the touched Go handler/service/repository packages plus the affected frontend memory store/tests, and record any remaining scope gaps explicitly before marking the change complete.

> Verification note: targeted memory handler/service/repository/server tests and `lib/stores/memory-store.test.ts` passed. Full `go test ./internal/service -count=1` still hits the pre-existing unrelated failure `TestVerifySpawnStartedFallsBackToGetStatusWithoutBridgeActivity` in `agent_service.go:2132`.
