## 1. Go Skill Runtime Resolution

- [x] 1.1 Add a repo-local skill bundle resolver in `src-go/internal/role` that parses `skills/**/SKILL.md` metadata and body, normalizes canonical paths, and resolves dependency closure for runtime use.
- [x] 1.2 Extend the normalized role execution profile and Go Bridge client contract with runtime-facing skill projection for loaded auto-load skills, available on-demand skills, and blocking diagnostics.
- [x] 1.3 Update role spawn and execution-profile construction so unresolved auto-load skills fail fast while unresolved on-demand skills remain preserved warning-level references.

## 2. Bridge Runtime Consumption

- [x] 2.1 Extend Bridge schemas, types, and request validation to accept the expanded normalized role skill projection from Go.
- [x] 2.2 Update Bridge prompt injection and execute-path handling so loaded skill context is applied to runtime prompt composition and on-demand skill inventory remains visible without Bridge-side file reads.

## 3. Preview And Review Diagnostics

- [x] 3.1 Extend preview and sandbox payloads or helpers so operators can inspect loaded-versus-available skill projection plus blocking versus warning diagnostics before save or launch.
- [x] 3.2 Update role authoring review mappings or UI summary seams that currently explain execution profile behavior so projected runtime skill behavior is visible and consistent.

## 4. Fixtures, Docs, And Verification

- [x] 4.1 Expand repo-local sample skills and any aligned sample role fixtures with runtime-meaningful `SKILL.md` content and dependency examples that match the documented Skill-Tree model.
- [x] 4.2 Update role or skill docs to explain runtime skill projection, auto-load versus on-demand semantics, and blocking rules for unresolved auto-load skills.
- [x] 4.3 Add focused verification for Go skill resolution, execution-profile projection, Bridge prompt injection, and preview or sandbox readiness diagnostics.
