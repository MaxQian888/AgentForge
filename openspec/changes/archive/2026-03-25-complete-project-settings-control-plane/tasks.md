## 1. Project settings schema and API contract

- [ ] 1.1 Extend Go project settings storage/DTO types so project settings can carry structured governance sections in addition to existing coding-agent defaults.
- [ ] 1.2 Add backward-compatible defaulting and merge logic for legacy projects whose stored settings only contain coding-agent fields.
- [ ] 1.3 Return a server-authored project settings summary or diagnostics payload together with project settings and coding-agent catalog responses.

## 2. Frontend settings control plane

- [ ] 2.1 Expand `lib/stores/project-store.ts` normalization and update types so the frontend can read and save the structured project settings document plus settings summary.
- [ ] 2.2 Rework `app/(dashboard)/settings/page.tsx` into a sectioned settings workspace covering general, repository, coding-agent, budget and alerts, review policy, and operator summary.
- [ ] 2.3 Add unified save-state, validation, and fallback messaging so unavailable runtimes, defaulted governance values, and invalid threshold combinations are visible before save.

## 3. Review policy integration

- [ ] 3.1 Wire project-scoped review policy into the deep-review follow-up path so `approve` results honor project manual-approval requirements.
- [ ] 3.2 Expose the resulting pending-manual-approval state through the relevant review/task API and notification surfaces used by operators.

## 4. Verification

- [ ] 4.1 Extend frontend settings tests to cover legacy defaulted settings, unified save behavior, and runtime or governance diagnostics rendering.
- [ ] 4.2 Add backend tests for structured settings merge/default logic, summary generation, and review-policy-driven follow-up routing.
- [ ] 4.3 Run focused verification for the updated settings page and review-policy flow, and record any remaining scope limits before completion.
