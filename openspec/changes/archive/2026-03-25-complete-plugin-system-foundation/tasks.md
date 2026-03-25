## 1. Shared Plugin Contracts

- [x] 1.1 Expand shared manifest, schema, and model definitions to cover `WorkflowPlugin`, `ReviewPlugin`, multi-source install metadata, trust metadata, and the extended lifecycle operations.
- [x] 1.2 Update registry-facing persistence and API shapes so plugin records can store source provenance, digest or signature status, approval state, release metadata, and workflow or review plugin-specific runtime fields.

## 2. Workflow Plugin Runtime

- [x] 2.1 Implement workflow plugin validation and registration for sequential workflows, including role reference checks, step-transition validation, and explicit unsupported-mode errors for non-executable process modes.
- [x] 2.2 Build the sequential workflow execution service that resolves role bindings, materializes step inputs and outputs, persists workflow run state, and records retry or failure outcomes.
- [x] 2.3 Add manual or internal workflow execution entrypoints plus query surfaces for workflow run status so operators can inspect in-progress and terminal workflow outcomes.

## 3. Review Plugin Execution

- [x] 3.1 Implement review plugin manifest loading, trigger matching, and execution-plan selection so enabled plugins can be chosen per review run by event and file patterns.
- [x] 3.2 Refactor the Layer 2 deep-review execution path so the built-in logic, security, performance, and compliance dimensions run through the same plugin-aware aggregation path as custom `ReviewPlugin` contributions.
- [x] 3.3 Persist plugin provenance and partial-failure metadata in review results, and expose the enriched review state through existing review APIs and real-time events.

## 4. SDK And Scaffolding

- [x] 4.1 Create the TypeScript plugin SDK package for Tool and Review plugins with manifest helpers, MCP bootstrap utilities, normalized review finding helpers, and a local test harness.
- [x] 4.2 Extend the Go-hosted plugin SDK and build helpers so maintained samples or templates remain valid for current Go-hosted plugin contracts and repository verification.
- [x] 4.3 Implement the `create-plugin` scaffolding flow with type-specific templates, build scripts, validation hooks, and starter tests for supported plugin classes.

## 5. Distribution And Trust

- [x] 5.1 Implement normalized install, update, deactivate, disable, uninstall, and source-record flows for built-in, local, Git, npm, and configured catalog sources.
- [x] 5.2 Add digest, signature, and approval verification handling so external plugins can be marked verified or blocked before activation.
- [x] 5.3 Add catalog search and install surfaces that let operators inspect installable plugin entries separately from installed plugin records while preserving release-history metadata in the registry.

## 6. Verification And Documentation

- [x] 6.1 Add repository validation coverage for workflow manifests, review plugin selection, SDK templates, scaffold output, and multi-source plugin install or trust scenarios.
- [x] 6.2 Update plugin-related documentation (`PRD`, plugin design docs, SDK usage docs, and operator guidance) so the repo documents the new contracts and current runtime truth instead of outdated placeholder behavior.
