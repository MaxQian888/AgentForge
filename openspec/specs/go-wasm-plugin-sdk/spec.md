# go-wasm-plugin-sdk Specification

## Purpose
Define AgentForge's Go SDK contract for authoring and validating Go-hosted WASM plugins, including required exports, safe host bindings, and the repository sample plugin workflow.

## Requirements
### Requirement: Go SDK defines the required WASM plugin contract
The system SHALL provide a Go SDK for authoring AgentForge WASM plugins. The SDK MUST define the exported entrypoints, ABI version marker, and request/response envelope types required for plugin description, initialization, health checks, and kind-specific invocations supported by the plugin.

#### Scenario: SDK-authored plugin exposes the required entrypoints
- **WHEN** a plugin is built using the Go WASM SDK
- **THEN** the resulting module exposes the entrypoints and ABI metadata required by the Go host runtime

#### Scenario: Unsupported ABI version is rejected
- **WHEN** the Go host loads a WASM plugin built against an unsupported SDK ABI version
- **THEN** activation fails before the plugin is marked active

### Requirement: Go SDK exposes safe host bindings
The system SHALL provide SDK-level host bindings for structured logging, configuration lookup, and structured success/error emission without exposing arbitrary host internals directly to the plugin.

#### Scenario: Plugin reads host-provided configuration through the SDK
- **WHEN** a WASM plugin requests configuration values exposed by the host through the SDK
- **THEN** the host returns only the values permitted by the plugin manifest and runtime policy

#### Scenario: Plugin returns a structured runtime error
- **WHEN** a WASM plugin operation fails while handling an invocation
- **THEN** the SDK returns a structured error envelope that the Go host can record in plugin runtime state

### Requirement: Repository includes a verifiable SDK sample plugin
The system SHALL include a sample Go WASM plugin and build workflow that produce a module and manifest the Go runtime can register, activate, and health-check during verification.

#### Scenario: Sample plugin builds into a valid WASM artifact
- **WHEN** the repository build workflow runs for the sample plugin
- **THEN** it produces a WASM module artifact referenced by a manifest that passes plugin validation

#### Scenario: Sample plugin can be activated by the Go runtime
- **WHEN** the sample plugin is registered and activated through the Go plugin APIs
- **THEN** the Go runtime loads the module successfully and reports the plugin as active
