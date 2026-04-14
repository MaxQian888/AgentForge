## 1. Expand the memory explorer state contract

- [x] 1.1 Extend `lib/stores/memory-store.ts` with canonical explorer query state plus list, stats, detail, selection, export, bulk-delete, and cleanup actions that reuse the existing `/api/v1/projects/:pid/memory*` endpoints.
- [x] 1.2 Implement refresh semantics in the memory store so filter changes, selection changes, and successful destructive actions keep list results, stats, and detail state synchronized without a full page reload.
- [x] 1.3 Add or update focused store tests in `lib/stores/memory-store.test.ts` for list/stats loading, detail fetch, filtered export, bulk delete, cleanup, and failure preservation.

## 2. Build the memory workspace UI

- [x] 2.1 Refactor `components/memory/memory-panel.tsx` into a workspace shell that preserves the current project gate while adding summary cards and a shared filter/action toolbar.
- [x] 2.2 Implement the filtered result surface with active-filter context, selection support, and explicit loading, empty, and error states for the memory explorer list.
- [x] 2.3 Implement a responsive memory detail surface that lazy-loads full entry content and explorer metadata, using split layout on wide screens and a focused sheet/dialog flow on narrow screens.

## 3. Add operator management flows

- [x] 3.1 Add safe single-delete and bulk-delete flows with explicit confirmation, affected-count feedback, and selection cleanup after successful mutations.
- [x] 3.2 Add episodic cleanup and filtered JSON export flows that use the current explorer filter scope and surface success or failure feedback without resetting explorer context.
- [x] 3.3 Update `messages/en/memory.json` and `messages/zh-CN/memory.json` so the workspace, detail, export, and cleanup flows have complete localized copy.

## 4. Verify the completed workspace

- [x] 4.1 Add or extend component tests in `components/memory/*.test.tsx` to cover filter-driven refresh, detail inspection, and destructive/export action flows.
- [x] 4.2 Run targeted verification for the touched memory workspace/store tests and any directly affected lint or typecheck slices, then record remaining scope limits before apply completion.
