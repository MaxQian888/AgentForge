## 1. Correct discovery and install semantics

- [x] 1.1 Update the Go plugin control plane so built-in and catalog discovery remain read-only browse operations instead of implicitly creating installed plugin registry records.
- [x] 1.2 Preserve explicit local and catalog install flows while keeping installed-state visibility merged from the registry without discovery side effects.

## 2. Extend plugin store control-plane coverage

- [x] 2.1 Expand `lib/stores/plugin-store.ts` with actions and state for catalog install, deactivate, update, event history, workflow runs, and MCP diagnostics.
- [x] 2.2 Normalize installability, trust, approval, release, and source-channel data for installed, built-in, catalog, and browse-only marketplace entries.

## 3. Build the operator console surface

- [x] 3.1 Rework `app/(dashboard)/plugins/page.tsx` to render truthful source sections and explicit install entry points for local and catalog-backed plugins.
- [x] 3.2 Extend `components/plugins/*` so the selected plugin details surface trust and release metadata, runtime diagnostics, recent audit events, workflow run history, and the full supported lifecycle action set.
- [x] 3.3 Wire operator-triggered MCP refresh and inspection flows through the existing Go APIs with clear blocked-reason and empty-state messaging.

## 4. Verify the focused plugin seam

- [x] 4.1 Add or update frontend tests for source-aware install behavior, diagnostics rendering, lifecycle gating, and plugin detail interactions.
- [x] 4.2 Add or update Go service and handler tests for read-only discovery semantics and the operator-facing plugin control-plane routes consumed by the dashboard.
- [x] 4.3 Run focused verification for the touched plugin files and control-plane tests, then reconcile any task or artifact wording needed to match repo truth.
