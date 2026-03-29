## 1. Settings draft model and error plumbing

- [x] 1.1 Extend the settings-facing client state and helpers so the frontend can derive a persisted snapshot, editable draft, dirty state, and draft-based diagnostics for project settings.
- [x] 1.2 Update `lib/stores/project-store.ts` so project settings saves can propagate validation or request failures back to the settings page instead of only updating happy-path state.

## 2. Settings workspace lifecycle and feedback

- [x] 2.1 Refactor `app/(dashboard)/settings/page.tsx` into a settings workspace that exposes unsaved-changes state plus save and discard/reset actions for the project settings draft.
- [x] 2.2 Add client-visible validation and submission feedback for budget, review policy, coding-agent, and webhook inputs, including pending, success, and failure states.
- [x] 2.3 Rework the operator summary or diagnostics area so it reflects current draft values, runtime blocking reasons, fallback-backed values, and integration readiness instead of static badges alone.

## 3. Focused settings workspace verification

- [x] 3.1 Expand `app/(dashboard)/settings/page.test.tsx` to cover legacy fallback rendering, dirty-state transitions, and discard or reset behavior.
- [x] 3.2 Add focused tests for invalid input feedback, save failure retention of draft values, successful save state reset, and diagnostics updates driven by current draft changes.
