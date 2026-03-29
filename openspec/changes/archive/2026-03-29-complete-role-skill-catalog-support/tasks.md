## 1. Build Repo-Truthful Skill Catalog Foundations

- [x] 1.1 Add canonical repo-local sample skill fixtures under `skills/` and align built-in/sample role manifests so documented skill paths resolve inside the repository.
- [x] 1.2 Implement authoritative role-skill catalog discovery for the supported repo-local skill roots, including normalized role-compatible paths, source metadata, and path-based fallback labels.
- [x] 1.3 Add focused backend or helper tests for catalog discovery, empty-root behavior, and missing-optional-metadata fallback.

## 2. Extend Role Authoring Contracts With Skill Resolution Context

- [x] 2.1 Extend role authoring store and API mapping layers to load catalog entries plus per-skill resolution state needed by the dashboard.
- [x] 2.2 Extend preview or sandbox related role-authoring responses or interpretation helpers so effective role skills carry provenance and unresolved-warning context without becoming runtime readiness blockers.
- [x] 2.3 Add focused tests for skill-resolution mapping across direct draft edits, template-derived skills, inherited skills, and unresolved manual references.

## 3. Upgrade The Role Skills Workspace

- [x] 3.1 Enhance the role workspace Skills section so operators can search or select catalog-backed skills while preserving manual path entry and the existing `auto_load` toggle behavior.
- [x] 3.2 Surface resolved versus unresolved, source, and provenance cues in the role library, live draft summary, and review context for roles with configured skills.
- [x] 3.3 Ensure the upgraded skill-authoring flow remains usable across existing desktop, medium, narrow, and empty-catalog states.

## 4. Sync Docs And Focused Verification

- [x] 4.1 Update `docs/role-authoring-guide.md`, `docs/role-yaml.md`, and any touched sample-role guidance so the documented role-skills flow matches the repo-local catalog and manual fallback behavior.
- [x] 4.2 Run focused verification for sample role-to-skill consistency, skill catalog discovery, role workspace skill selection, and preview/sandbox skill-resolution feedback before marking the change ready.
