## Why

The `/skills` workspace exposes Verify Internal Skills and Verify Built-in Skills, but the Go operator API previously derived verification results from inventory health alone — replaying only the preview/lock/bundle-root/mirror-drift state already computed by `List()`. That left a gap between maintainer truth (`pnpm skill:verify:internal`, `pnpm skill:verify:builtins`) and operator truth (`POST /api/v1/skills/verify`): missing registry entries, invalid family or sourceType values, absent bundle metadata, missing required frontmatter, missing required agent configs, duplicate registry ids/roots, and unregistered skill packages were all caught by the CLI but invisible to the workspace.

The existing specs and docs already describe `/skills` as a truthful, bounded operator surface for verification and sync. The mismatch undermined that contract precisely where maintainers expect the workspace to explain governance drift.

## What Changed

- Replaced the inventory-replay verification path in `src-go/internal/skills/service.go` with a shared `evaluate()` pass that covers 28 stable issue codes across 8 rule classes: registry validity, profile rules, family-root placement, provenance/lock, built-in bundle alignment, package existence and frontmatter, agent-config contract, and mirror drift / unregistered packages.
- `POST /api/v1/skills/verify` now returns `VerifyResult{OK bool, Results []VerificationResult}` where each result carries per-skill `{skillId, family, status, issues[]}` derived from the same evaluation path used by List/Get — not from a post-hoc health recomputation.
- The built-in verification path (`Families: ["built-in-runtime"]`) reports bundle-specific failures (`built_in_bundle_missing`, `built_in_bundle_root_mismatch`, `built_in_bundle_missing_category`, `built_in_bundle_missing_tags`, `built_in_agent_config_missing`) that a plain family filter over inventory health cannot catch.
- `lib/stores/skills-store.ts` persists `lastVerificationResult: SkillsVerifyResult` and the frontend workspace renders per-skill diagnostics — issue code, message, and target path — in a structured result card instead of collapsing the response to a generic success/failure toast.
- Per-skill `supportedActions` and `blockedActions` are resolved per family so the workspace can render blocked states (e.g., "only workflow-mirror skills can sync mirrors") rather than hiding unsupported actions.
- Backfilled Go service tests (4), Go handler tests (3), and Jest page tests (4) that lock verifier parity by failing on verifier-only conditions.
- Updated `docs/guides/internal-skill-governance.md` with a Skills Workspace section that aligns the operator narrative with the actual verification behavior.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- internal-skill-governance: operator verification APIs now derive from the same shared governance evaluation pass as `pnpm skill:verify:internal` and `pnpm skill:verify:builtins`, returning per-skill diagnostics across all 8 governed rule classes.
- skills-management-workspace: the `/skills` workspace renders verifier-grade per-skill results (status, issues with codes/messages/paths) from `lastVerificationResult` and exposes per-family blocked-action explanations instead of hiding or collapsing unsupported actions.

## Impact

- Modified backend seams: `src-go/internal/skills/service.go` (evaluate, validateRegistryEntry, validateMirrorTargets, supportedActions, blockedActions, addIssue, addStandaloneIssue), `src-go/internal/handler/skills_handler.go` (unchanged handler, richer service contract).
- Modified frontend/store seams: `lib/stores/skills-store.ts` (SkillIssue, SkillsVerifyResult, lastVerificationResult), `components/skills/skills-workspace.tsx` (per-skill result card, blockedActions rendering, issue detail with code/message/targetPath), `app/(dashboard)/skills/page.tsx`.
- Remaining: `docs/api/openapi.yaml` and `docs/api/openapi.json` do not yet document the `/api/v1/skills` endpoints or their DTOs (task 3.2).
