## 1. Internal skill inventory and registry foundation

- [x] 1.1 Add a repo-level internal skill registry that records the current built-in runtime skills, repo-assistant skills, and workflow mirror skills with family/profile, canonical root, provenance, and mirror metadata.
- [x] 1.2 Add shared registry-loading helpers and wire package scripts for repo-wide internal skill verification (plus any needed mirror sync entrypoint).
- [x] 1.3 Backfill the current repo-managed skills into that registry, including lockfile-backed upstream skills and the existing OpenSpec workflow skill mirrors.

## 2. Profile-aware validation and provenance checks

- [x] 2.1 Implement a shared internal skill validator that enforces profile-specific frontmatter, package layout, and agent-config checks for repo-authored skills.
- [x] 2.2 Extend the validator to enforce provenance rules for `upstream-sync` skills via `skills-lock.json` and to report mirror drift for generated workflow skill targets.
- [x] 2.3 Refactor or wrap `pnpm skill:verify:builtins` so the built-in runtime subset is validated through the shared governance rules instead of a parallel hidden contract.
- [x] 2.4 Add focused tests/fixtures covering repo-authored skills, upstream-synced skills, and workflow mirror drift detection.

## 3. Workflow mirror and package normalization

- [x] 3.1 Choose and codify the canonical OpenSpec workflow skill source root, then add a deterministic sync path for its `.claude` and `.github` mirrors.
- [x] 3.2 Normalize the current repo-authored skill packages to their declared profiles, including any required metadata, agent config naming, and allowed optional parts.
- [x] 3.3 Record explicit controlled exceptions for upstream-imported skills so non-repo-authored layout differences remain deliberate and verifiable.

## 4. Documentation and maintainer handoff

- [x] 4.1 Write a maintainer-facing internal skill authoring guide covering profile selection, required metadata, optional package parts, provenance rules, and verification/sync commands.
- [x] 4.2 Update the repository docs or README so the new internal skill governance model and commands are discoverable from the existing contributor workflow.
