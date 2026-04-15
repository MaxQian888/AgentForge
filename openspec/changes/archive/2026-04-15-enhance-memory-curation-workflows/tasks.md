## 1. Normalize the backend memory curation model

- [x] 1.1 Extend `src-go/internal/model/agent_memory.go` and related normalization helpers so memory DTOs expose canonical `tags`, `kind`, and editability fields while accepting legacy `operator_note` input as a compatibility alias.
- [x] 1.2 Update `src-go/internal/repository/agent_memory_repo.go`, `src-go/internal/service/memory_service.go`, and `src-go/internal/service/memory_explorer_service.go` to persist metadata-backed tags, honor tag filters, and enforce controlled note update rules.
- [x] 1.3 Add or update repository/service tests covering operator note normalization, tag deduplication, tag-aware search/stats/export scope, and rejection of content edits on non-editable records.

## 2. Extend the authenticated memory API surface

- [x] 2.1 Update `src-go/internal/handler/memory_handler.go` and `src-go/internal/server/routes.go` so `/api/v1/projects/:pid/memory` supports operator note creation with tags and `/api/v1/projects/:pid/memory/:mid` supports controlled partial updates.
- [x] 2.2 Keep list/detail/stats/export/delete/cleanup responses aligned to the new curation contract, including tag-aware filtering, explicit editability fields, and clear error responses for unsupported edits.
- [x] 2.3 Refresh handler/OpenAPI tests and contract fixtures so legacy IM note writes, PATCH flows, and tag-aware detail/list payloads are covered.

## 3. Add memory workspace curation flows

- [x] 3.1 Extend `lib/stores/memory-store.ts` with note composer state, tag filters, update mutations, and single-entry export helpers while preserving existing explorer refresh semantics.
- [x] 3.2 Update `components/memory/*` and `app/(dashboard)/memory/page.tsx` to add note authoring, tag chips/filtering, editable-note flows, and explicit read-only guidance for system-generated memories.
- [x] 3.3 Add focused store/component tests for note creation, note editing, tag add/remove, tag filter application, single-entry export, and post-mutation synchronization.

## 4. Align dependent callers and verify the focused seam

- [x] 4.1 Update IM `/memory note` callers and related contract/docs surfaces so operator-authored notes use the canonical curation shape instead of depending on an undefined stored category.
- [x] 4.2 Run targeted backend and frontend verification for the new curation seam, and capture any follow-up gaps that remain intentionally out of scope for semantic/procedural runtime automation.
