## Context

AgentForge maintains two parallel verification planes for governed skills. Maintainer workflows run through `scripts/skills/internal-skill-governance.js` and `scripts/skills/verify-built-in-skill-bundle.js`; the operator-facing `/skills` workspace calls `POST /api/v1/skills/verify` through `src-go/internal/skills/service.go`. Before this change the Go `Verify()` path only replayed health derived from `List()`, so it caught preview/lock/bundle-root/mirror-drift states but missed several verifier-only failures the CLI already enforced: duplicate registry ids/roots, invalid family or sourceType values, missing bundle metadata, missing required frontmatter, missing required agent configs, and unregistered skill packages.

That mismatch matters because the current docs and specs already describe `/skills` as a truthful bounded operator surface for verification and sync, not a best-effort summary view. Any design here must preserve the packaged-runtime viability of the Go control plane and keep `/skills` bounded to existing repository workflows.

## Goals / Non-Goals

**Goals:**
- Make operator-facing skill verification derive from the same governance rule set and issue taxonomy as the repository's maintainer verification flows.
- Preserve the existing `/skills` inventory, detail, and mirror-sync surfaces while upgrading verification results to verifier-grade diagnostics.
- Keep the operator API machine-readable so the workspace can render per-skill failures without parsing CLI stdout.
- Add tests and docs that lock CLI/API/workspace parity for the focused verification seam.

**Non-Goals:**
- Rebuild the `/skills` workspace information architecture, inventory, or package preview surfaces.
- Add new skill-management actions such as upstream refresh, registry editing, or package scaffolding.
- Replace the existing maintainer CLI entrypoints or require the UI to shell out to them directly.

## Decisions

### 1. Consolidate inventory and verification into a single `evaluate()` pass in the Go service

`src-go/internal/skills/service.go` now has a single `evaluate(families []Family) ([]InventoryItem, []VerificationResult, error)` function that both `List()` and `Verify()` call. `evaluate()` performs one sequential pass over the registry, applying all governed rule classes in deterministic order. `List()` returns the items slice; `Verify()` returns the results slice. This eliminates the previous two-phase model where `Verify()` re-called `List()` and inferred issues from inventory health.

Why this approach:
- Avoids duplicated rule logic that could drift between the list and verify paths.
- Keeps the operator API packaged-runtime-safe — no Node subprocess dependency.
- The single pass is cheaper than calling List then re-deriving health: registry is read once, lock and bundle files are read once.

Alternatives considered:
- **Invoke the Node verifiers from Go**: rejected — introduces runtime coupling to Node for a core operator API and complicates desktop packaging.
- **Keep separate List/Verify implementations**: rejected — the previous implementation already drifted; two separate passes guaranteed the same outcome.

### 2. 28 stable issue codes covering 8 rule classes

The `evaluate()` and `validateRegistryEntry()` functions produce issues with stable string codes, not inferred status strings. The 28 codes map to 8 rule classes:

| Rule class | Issue codes |
|---|---|
| Registry validity | `duplicate_skill_id`, `duplicate_canonical_root`, `missing_skill_id`, `invalid_family`, `missing_verification_profile`, `missing_canonical_root`, `invalid_source_type`, `unknown_allowed_exception` |
| Family-root placement | `family_root_mismatch` |
| Provenance / lock | `missing_lock_key`, `missing_lock_entry` |
| Built-in bundle alignment | `built_in_bundle_missing`, `built_in_bundle_missing_root`, `built_in_bundle_root_mismatch`, `built_in_bundle_missing_category`, `built_in_bundle_missing_tags`, `missing_registry_entry` |
| Package existence | `missing_canonical_root_path`, `missing_skill_document` |
| Frontmatter | `missing_frontmatter_name`, `missing_frontmatter_description` |
| Agent-config contract | `built_in_agent_config_missing`, `noncanonical_agent_config_extension`, `invalid_agent_yaml` |
| Mirror / unregistered | `mirror_drift`, `canonical_skill_missing`, `missing_mirror_targets`, `unregistered_skill_package`, `preview_unavailable` |

`SkillIssue` carries `{code, message, targetPath?, family?, sourceType?}`. `HealthStatus` uses the four-value enum `healthy | warning | drifted | blocked` where `blocked` > `drifted` > `warning` > `healthy` by integer severity.

Why this approach:
- The workspace needs to explain exactly which skill failed and why without parsing CLI text.
- Built-in verification has stricter bundle-specific rules than a family filter; it needs first-class result paths.
- Stable codes allow tests to assert specific failure modes without coupling to message text.

Alternatives considered:
- **Return only health summaries from inventory**: rejected — loses verifier-only failures and weakens action results compared to the CLI.
- **Expose raw script text in the API**: rejected — brittle, not machine-readable, hard to render or test.

### 3. `verificationState` as an accumulation helper

`evaluate()` uses an internal `verificationState` struct with `items`, `itemByID`, and `resultByID` maps. Issues are appended to both the `InventoryItem.Health.Issues` slice (so `List()` returns them) and the `VerificationResult.Issues` slice (so `Verify()` returns them). The `addIssue` / `addStandaloneIssue` helpers deduplicate the write. For workflow-mirror skills, `syncFromItem` copies item health directly into the result rather than re-running the mirror check.

### 4. Per-family `supportedActions` and `blockedActions` resolved at evaluation time

`supportedActions(family)` and `blockedActions(family)` are deterministic functions called during the evaluation pass, not inferred from health state. This means the workspace can always render which actions are available and which are explicitly blocked (with a reason) for any selected skill, regardless of verification state.

Current mapping:
- `built-in-runtime`: supported `[verify-internal, verify-builtins, open-roles, open-marketplace]`; blocked `[sync-mirrors: "only workflow-mirror skills can sync mirrors"]`
- `repo-assistant`: supported `[verify-internal]`; blocked `[sync-mirrors, refresh-upstream: "upstream refresh remains a maintainer workflow outside the operator UI"]`
- `workflow-mirror`: supported `[verify-internal, sync-mirrors]`; blocked `[refresh-upstream: "workflow mirror skills are repo-authored and do not support upstream refresh"]`

### 5. Frontend renders per-skill result card from `lastVerificationResult`

`lib/stores/skills-store.ts` stores `lastVerificationResult: SkillsVerifyResult | null` and `verifySkills()` sets it before refreshing inventory. `components/skills/skills-workspace.tsx` renders a "Latest verification" card when `lastVerificationResult.results.length > 0`, iterating per-skill results and their issues with code, message, and targetPath. The workspace does not collapse the result to a toast.

### 6. Tests as a parity contract

Tests use temp-dir fixture repos (`writeGovernedSkillFixture`) that contain real file layouts matching the governance rules. Verifier-only failure tests mutate the fixture (e.g., overwrite `builtin-bundle.yaml` to drop category/tags; add a rogue unregistered `SKILL.md`) and assert specific issue codes in the response. This prevents CLI/API drift from escaping silently.

## Risks / Trade-offs

- [Go and CLI verification may drift again as new rules are added] → Mitigation: new rule classes must add issue codes to both the Go `evaluate()` pass and the relevant fixture tests; the fixture tests fail if the code changes without the test.
- [The workspace may surface more blocked or warning states than before] → Mitigation: preserve stable issue codes and keep unsupported actions clearly separated from actionable verification failures.
- [Unregistered package scanning can grow expensive on large repos] → Mitigation: scans are bounded to the governed scan roots already declared in the registry (`skills/`, `.agents/skills/`, `.codex/skills/`, `.claude/skills/`, `.github/skills/`); no global filesystem walk.

## Migration Plan

1. ✅ Extend Go `evaluate()` and API/result DTOs behind the existing `/api/v1/skills/verify` endpoint.
2. ✅ Update workspace rendering to surface per-skill diagnostics without changing route structure or action names.
3. ✅ Backfill Go service, handler, and Jest coverage for verifier-only failures and parity expectations.
4. ✅ Refresh `docs/guides/internal-skill-governance.md` so the CLI and `/skills` verification stories match.
5. ☐ Document the `/api/v1/skills/*` endpoints and their DTOs in `docs/api/openapi.yaml` and `docs/api/openapi.json` (task 3.2).

Rollback is straightforward: revert the richer `evaluate()` pass while leaving inventory/detail/sync handler wiring intact.

## Open Questions

- Should the Node verifier scripts gain an optional JSON output mode for external automation reuse? This change does not require it to reach parity; it can remain out of scope unless implementation proves the extra format is low-cost and beneficial.
