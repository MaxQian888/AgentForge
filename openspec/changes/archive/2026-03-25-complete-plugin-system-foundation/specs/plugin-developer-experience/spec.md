## ADDED Requirements

### Requirement: TypeScript plugin SDK bootstraps Tool and Review plugins
The system SHALL provide a TypeScript plugin SDK for authoring `ToolPlugin` and `ReviewPlugin` implementations, including manifest helpers, MCP bootstrap utilities, normalized result helpers, and a local test harness aligned with the platform's plugin contracts.

#### Scenario: SDK-authored tool plugin passes platform validation
- **WHEN** a developer creates a tool plugin using the TypeScript SDK helpers
- **THEN** the generated manifest and runtime bootstrap pass the platform's plugin validation rules without requiring handwritten protocol glue

#### Scenario: SDK-authored review plugin emits normalized findings
- **WHEN** a developer uses the TypeScript SDK to build a review plugin
- **THEN** the plugin can emit findings in the normalized review result shape expected by the deep-review pipeline

### Requirement: Go plugin SDK and build helpers align with Go-hosted plugin contracts
The system SHALL provide Go-side SDK and build helpers that align with the current Go-hosted plugin runtime contracts so Integration plugins and future Go-hosted plugin extensions can be built, packaged, and validated using one supported workflow.

#### Scenario: Go-hosted plugin template builds a valid artifact
- **WHEN** a developer generates a Go-hosted plugin template from the supported SDK workflow
- **THEN** the repository build helpers produce an artifact and manifest combination that passes plugin registration validation

#### Scenario: Go plugin template preserves runtime metadata
- **WHEN** a generated Go-hosted plugin declares ABI, runtime, and manifest metadata through the supported SDK helpers
- **THEN** the generated output preserves the metadata needed by the registry and host runtime to activate that plugin

### Requirement: `create-plugin` scaffolding generates type-specific starter projects
The system SHALL provide a `create-plugin` scaffolding flow that generates starter projects for supported plugin types with the correct manifest skeleton, source layout, build scripts, verification entrypoints, and example tests.

#### Scenario: Tool plugin scaffold includes MCP starter files
- **WHEN** a developer scaffolds a `ToolPlugin`
- **THEN** the generated project includes a manifest, MCP server entrypoint, package metadata, and at least one example test or verification script

#### Scenario: Workflow or review plugin scaffold includes type-specific templates
- **WHEN** a developer scaffolds a `WorkflowPlugin` or `ReviewPlugin`
- **THEN** the generated project includes the type-specific manifest fields, starter logic, and verification hooks needed for that plugin class

### Requirement: SDK examples and templates remain verifiable in the repository
The system SHALL keep the shipped SDK examples, templates, and scaffold outputs under repository verification so that SDK drift is detected before release.

#### Scenario: Repository verification validates shipped templates
- **WHEN** the repository validation flow runs against the maintained plugin templates and example projects
- **THEN** it confirms that the examples still build, lint, or validate against the current plugin contracts

