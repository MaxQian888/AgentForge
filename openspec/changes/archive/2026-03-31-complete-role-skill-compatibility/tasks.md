## 1. Shared Skill Compatibility Foundations

- [x] 1.1 Extend the shared skill package parsing path in `src-go/internal/role` so catalog entries and runtime bundles both expose normalized direct `requires` paths and declared `tools`.
- [x] 1.2 Implement canonical role tool-capability normalization, including legacy built-in alias handling for the current repo's role YAML values.
- [x] 1.3 Add focused Go unit tests for skill compatibility metadata parsing and role-tool normalization behavior.

## 2. Runtime Compatibility Diagnostics

- [x] 2.1 Extend execution-profile construction to evaluate role-skill compatibility for auto-load skill closure and on-demand inventory using the normalized tool-capability set.
- [x] 2.2 Update preview, sandbox, agent spawn, and workflow role execution paths to reuse the same blocking-versus-warning skill compatibility diagnostics.
- [x] 2.3 Add focused Go and runtime-contract tests for missing-skill, dependency, and tool-mismatch scenarios across blocking and warning cases.

## 3. Role Authoring And Review Surfaces

- [x] 3.1 Extend role store and frontend skill metadata types so the dashboard can consume catalog-level dependency and declared-tool compatibility data.
- [x] 3.2 Update the role workspace Skills section, draft summary, role library, and context rail to display dependency, declared-tool, transitive-skill, and compatibility cues without removing manual path fallback.
- [x] 3.3 Add focused frontend tests covering skill selection, compatibility status changes after role tool edits, and library or review-surface compatibility summaries.

## 4. Fixtures, Docs, And Verification

- [x] 4.1 Align sample skills and sample roles with the canonical compatibility vocabulary used by the new role-skill checks, including any required legacy alias coverage.
- [x] 4.2 Update `docs/role-authoring-guide.md`, `docs/role-yaml.md`, and any touched role-skill guidance so operators understand dependency visibility, declared-tool requirements, and blocking versus warning behavior.
- [x] 4.3 Run focused verification for Go role compatibility logic, role workspace compatibility cues, and any affected bridge or contract tests before marking the change ready for apply.
