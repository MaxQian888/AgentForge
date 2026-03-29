## 1. Built-in Bundle Truth Source

- [x] 1.1 Add the repo-owned built-in plugin bundle metadata source and seed it with the current official built-ins plus the new docs-aligned built-in targets.
- [x] 1.2 Normalize official built-in plugin asset layout under plugins so each bundle entry points at a real maintained manifest and runtime entrypoint.

## 2. Discovery And Catalog Wiring

- [x] 2.1 Update Go built-in discovery and catalog assembly to read bundle membership, filter out unlisted manifests, and emit official built-in provenance plus availability metadata.
- [x] 2.2 Extend handler, DTO, store, and plugin management tests or rendering seams as needed so operator-facing built-in views preserve the new availability and provenance contract without regressing installed or marketplace flows.

## 3. Official Built-in Plugin Coverage

- [x] 3.1 Add the maintained built-in ReviewPlugin assets and wire their install plus review-execution provenance through the existing review pipeline.
- [x] 3.2 Add the maintained built-in WorkflowPlugin starter, validate it against the current role registry, and ensure it executes through the standard sequential workflow runtime.
- [x] 3.3 Add the additional docs-aligned built-in ToolPlugin entries and ensure each one declares truthful prerequisite or installability metadata.

## 4. Family-Based Verification Workflows

- [x] 4.1 Extend plugin build, debug, and verify scripts so maintained tool, review, workflow, and existing integration built-ins each have a supported family-specific verification path.
- [x] 4.2 Add targeted tests or fixtures for bundle drift detection, catalog filtering, review-plugin provenance, workflow starter validation, and prerequisite-aware live smoke boundaries.

## 5. Documentation And Acceptance

- [x] 5.1 Update PRD.md, PLUGIN_SYSTEM_DESIGN.md, and related built-in plugin examples so the documented official bundle matches the shipped repo assets and paths.
- [x] 5.2 Run focused verification for built-in bundle discovery, catalog truthfulness, review and workflow built-ins, and family-based plugin scripts, then capture any opt-in live smoke prerequisites in the final notes.
