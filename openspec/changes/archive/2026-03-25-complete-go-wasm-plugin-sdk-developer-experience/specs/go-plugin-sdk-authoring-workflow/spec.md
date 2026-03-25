## ADDED Requirements

### Requirement: Repository defines a supported Go-hosted plugin authoring workflow
The system SHALL provide a repository-supported authoring workflow for Go-hosted WASM plugins that includes a maintained sample or template, a supported build helper, and manifest-aligned usage guidance so plugin authors do not need to reverse-engineer runtime expectations from internal implementation details.

#### Scenario: Supported workflow builds a manifest-aligned Go-hosted plugin artifact
- **WHEN** a developer follows the repository-supported Go plugin authoring workflow
- **THEN** the workflow produces a `.wasm` artifact and manifest combination that matches the current `runtime: wasm` host contract and passes plugin registration validation

#### Scenario: Supported workflow exposes the required runtime metadata
- **WHEN** a maintained sample or template declares plugin metadata and supported operations through the supported workflow
- **THEN** the resulting output preserves the ABI, capability, and module metadata needed by the registry and Go runtime to activate and invoke that plugin

### Requirement: Repository verification protects the Go plugin authoring workflow from drift
The system SHALL keep the maintained Go-hosted plugin sample or template, build helper, and usage documentation under repository verification so SDK drift is detected before release.

#### Scenario: Verification validates the maintained Go-hosted sample or template
- **WHEN** repository verification runs against the maintained Go plugin sample or template
- **THEN** it confirms that the artifact still builds, the manifest remains valid, and the Go runtime can validate the resulting plugin contract

#### Scenario: Documentation remains aligned with the supported authoring workflow
- **WHEN** the repository documents the supported Go-hosted plugin authoring workflow
- **THEN** the documentation describes the current runtime truth, supported boundaries, and verification entrypoints instead of outdated or aspirational SDK behavior
