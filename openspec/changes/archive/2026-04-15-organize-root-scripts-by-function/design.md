## Context

AgentForge's root `scripts/` directory has grown into a flat collection of unrelated automation entrypoints. Current scripts cover backend and bridge builds, full-stack and backend development orchestration, Go WASM plugin authoring and verification, internal skill governance, updater release utilities, i18n auditing, and stub smoke helpers. Those files are not isolated implementation details: they are referenced directly by root `package.json`, plugin-local `package.json` files, GitHub Actions workflows, repository docs, Jest tests, shell wrappers, and cross-script `require()` calls.

That means the risk is not "moving files"; the risk is leaving behind one or more stale callers after the move. The design therefore needs a canonical target layout plus a migration rule that treats every repo-owned caller as part of the same change.

## Goals / Non-Goals

**Goals:**
- Group root automation scripts by functional domain so maintainers can find and extend them without navigating an ever-growing flat folder.
- Move shared helpers, fixtures, and wrappers with the script family that owns them instead of leaving disconnected support files behind.
- Rewrite all supported callers to the canonical paths in the same change, including package commands, workflows, docs, tests, and intra-script imports.
- Preserve the current supported command semantics so the repository does not gain new behavioral churn while only cleaning up layout.
- Make partial migration detectable through deterministic verification rather than relying on manual spot checks.

**Non-Goals:**
- Replacing the existing Node-based script surface with a new CLI framework, TypeScript runtime, or monolithic command router.
- Changing supported user-facing package command names unless a migrated caller cannot remain correct without it.
- Reorganizing non-root script trees such as `src-im-bridge/scripts/**`; this change is scoped to the root repository `scripts/` directory and its callers.
- Broadly refactoring the internal behavior of script families beyond what is required to support the new layout.

## Decisions

### 1. Use function-oriented subdirectories under the root `scripts/` tree

The repository will replace the current flat folder with canonical function-based groupings for the existing script families. Exact folder names can be finalized during implementation, but the grouping must make ownership obvious for at least the currently visible families: build/runtime packaging, local development orchestration, plugin authoring/verification, internal skill governance, updater/release tooling, and i18n or audit utilities. Shared helpers and fixtures move with the family that owns them.

Why this approach:
- It reduces lookup cost without changing the underlying execution model.
- It preserves the current "plain Node script invoked from pnpm/workflows" pattern already used by the repo.
- It avoids inventing a higher-level orchestration layer just to solve a directory-structure problem.

Alternatives considered:
- Keep a flat folder and rely on longer file-name prefixes. Rejected because naming noise continues to grow and ownership remains implicit.
- Introduce a single router CLI that dispatches subcommands. Rejected because it adds a second refactor axis and is not required to satisfy the requested cleanup.

### 2. Treat the reorganization as a full caller migration, not a file-only move

Every repo-supported caller of a moved script must be updated in the same change. That includes root `package.json`, plugin-local `package.json` files that use relative `../../../scripts/...` paths, GitHub workflows, repository docs, Jest tests, fixtures that embed canonical paths, and script-to-script imports.

Why this approach:
- The current repo has multiple direct `node scripts/...` entrypoints outside `package.json`, so leaving migration to follow-up work would knowingly create drift.
- The user explicitly asked for no omissions or missed call sites.

Alternatives considered:
- Leave compatibility wrappers at the old flat paths. Rejected as the default because it preserves duplicate sources of truth and makes it harder to tell whether the migration is actually complete.
- Update only `package.json` and let docs or plugin packages catch up later. Rejected because those callers are already part of the supported workflow surface.

### 3. Preserve supported command semantics while changing physical paths

The migration will prefer keeping existing command names and workflows stable, with only their backing script paths updated. Directly documented `node ...` invocations will be rewritten to the new canonical locations rather than replaced with unrelated new commands.

Why this approach:
- The requested work is repository organization, not command redesign.
- Stable command semantics keep the migration focused and reduce downstream surprises.

Alternatives considered:
- Rename commands to match the new directory layout. Rejected because it multiplies the migration surface and mixes layout cleanup with API churn.

### 4. Verification must prove that no repo-owned caller still points at removed paths

Implementation will include a deterministic migration audit that combines focused validation of moved script families with a repo-wide search for stale legacy paths. The verification surface should be strong enough to catch outdated package commands, workflow steps, docs, or relative imports before the change is considered complete.

Why this approach:
- The highest-risk failure mode is a missed path, not a compile error in the moved file itself.
- Search plus targeted execution is the fastest truthful way to validate a path migration in this repo.

Alternatives considered:
- Rely only on manual review or selected happy-path commands. Rejected because the call surface is too distributed.

## Risks / Trade-offs

- [A stale path remains in a low-traffic caller such as a plugin sub-package or markdown doc] → Mitigation: build a complete caller inventory up front and finish with a repo-wide stale-path search against moved script names.
- [Moving helpers or fixtures breaks relative imports and targeted Jest coverage] → Mitigation: migrate each family with its owned tests and support files together, and keep exported script APIs stable where tests already rely on them.
- [A helper could reasonably belong to more than one domain] → Mitigation: choose a single owning domain based on primary callers and avoid creating a generic "misc" bucket that recreates the current ambiguity.
- [The cleanup turns into behavioral refactoring] → Mitigation: keep command semantics stable and defer non-essential internal rewrites unless required to unblock the move.

## Migration Plan

1. Inventory the current root `scripts/` files and classify them into canonical function-based families.
2. Move one family at a time together with its owned helpers, fixtures, wrappers, and tests.
3. Rewrite every repo-owned caller for that family immediately, including package commands, workflows, docs, tests, and intra-script imports.
4. Run focused verification for the moved family plus a repo-wide search for stale references to removed paths.
5. Repeat until the old flat layout is fully retired.

Rollback strategy: because this is a repository-internal path migration, rollback is a single-change revert that restores the previous paths and caller references if a critical missed dependency is discovered during validation.

## Open Questions

- Non-blocking: exact subdirectory names can be finalized during implementation as long as they remain function-oriented and do not split one tightly coupled script family across multiple unrelated folders.
