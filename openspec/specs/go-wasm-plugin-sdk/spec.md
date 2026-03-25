# go-wasm-plugin-sdk Specification

## Purpose
Define AgentForge's Go SDK contract for authoring and validating Go-hosted WASM plugins, including required exports, safe host bindings, and the repository sample plugin workflow.
## Requirements
### Requirement: Go SDK defines the required WASM plugin contract
The system SHALL provide a Go SDK for authoring Go-hosted WASM plugins. The SDK MUST define manifest-aligned typed descriptor and capability metadata, the exported entrypoints, ABI version marker, and the request or response envelope helpers required for plugin description, initialization, health checks, and kind-specific invocations supported by the plugin. The SDK MUST remain valid for current integration plugins and for future Go-hosted plugin extensions that declare `runtime: wasm` under the platform's manifest rules.

#### Scenario: SDK-authored integration plugin exposes the required entrypoints
- **WHEN** a plugin is built using the Go WASM SDK for an integration plugin
- **THEN** the resulting module exposes the entrypoints and ABI metadata required by the Go host runtime

#### Scenario: SDK-authored plugin preserves manifest-aligned operations
- **WHEN** a Go-hosted plugin declares its supported operations and metadata through the supported SDK contract
- **THEN** the generated module preserves the capability and descriptor data needed by the registry and host runtime to validate and invoke that plugin

#### Scenario: Unsupported ABI version is rejected
- **WHEN** the Go host loads a WASM plugin built against an unsupported SDK ABI version
- **THEN** activation fails before the plugin is marked active

### Requirement: Go SDK exposes safe host bindings
The system SHALL provide SDK-level host bindings for structured logging, configuration lookup, bounded execution context access, and structured success or error emission without exposing arbitrary host internals directly to the plugin.

#### Scenario: Plugin reads host-provided configuration through the SDK
- **WHEN** a WASM plugin requests configuration values exposed by the host through the SDK
- **THEN** the host returns only the values permitted by the plugin manifest and runtime policy

#### Scenario: Plugin receives bounded execution context through the SDK
- **WHEN** the Go runtime invokes a Go-hosted plugin through the supported SDK contract
- **THEN** the SDK exposes only the current operation and bounded runtime metadata needed for that invocation instead of unrestricted host internals

#### Scenario: Plugin returns a structured runtime error
- **WHEN** a WASM plugin operation fails while handling an invocation
- **THEN** the SDK returns a structured error envelope that the Go host can record in plugin runtime state

### Requirement: Repository includes a verifiable SDK sample plugin
The system SHALL include repository-maintained Go-hosted plugin samples or templates and a build workflow that produce artifacts and manifests the platform can register, activate, and validate during verification.

#### Scenario: Sample Go-hosted plugin builds into a valid artifact
- **WHEN** the repository build workflow runs for a maintained Go-hosted plugin sample or template
- **THEN** it produces an artifact referenced by a manifest that passes plugin validation

#### Scenario: Maintained Go plugin sample or template stays verifiable
- **WHEN** repository verification runs against the shipped Go plugin sample or template
- **THEN** it confirms that the generated or maintained plugin still builds and satisfies the current SDK contract

