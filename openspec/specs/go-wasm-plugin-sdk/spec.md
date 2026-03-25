# go-wasm-plugin-sdk Specification

## Purpose
Define AgentForge's Go SDK contract for authoring and validating Go-hosted WASM plugins, including required exports, safe host bindings, and the repository sample plugin workflow.
## Requirements
### Requirement: Go SDK defines the required WASM plugin contract
The system SHALL provide a Go SDK for authoring Go-hosted WASM plugins. The SDK MUST define the exported entrypoints, ABI version marker, and request or response envelope types required for plugin description, initialization, health checks, and kind-specific invocations supported by the plugin. The SDK MUST remain valid for current integration plugins and for future Go-hosted plugin extensions that declare `runtime: wasm` under the platform's manifest rules.

#### Scenario: SDK-authored integration plugin exposes the required entrypoints
- **WHEN** a plugin is built using the Go WASM SDK for an integration plugin
- **THEN** the resulting module exposes the entrypoints and ABI metadata required by the Go host runtime

#### Scenario: SDK-authored extension preserves manifest-aligned operations
- **WHEN** a Go-hosted plugin template declares manifest-aligned operations through the supported SDK contract
- **THEN** the generated module preserves the operation metadata needed by the registry and host runtime to validate and invoke that plugin

#### Scenario: Unsupported ABI version is rejected
- **WHEN** the Go host loads a WASM plugin built against an unsupported SDK ABI version
- **THEN** activation fails before the plugin is marked active

### Requirement: Go SDK exposes safe host bindings
The system SHALL provide SDK-level host bindings for structured logging, configuration lookup, structured success or error emission, and the bounded execution context needed by Go-hosted plugins without exposing arbitrary host internals directly to the plugin.

#### Scenario: Plugin reads host-provided configuration through the SDK
- **WHEN** a WASM plugin requests configuration values exposed by the host through the SDK
- **THEN** the host returns only the values permitted by the plugin manifest and runtime policy

#### Scenario: Go-hosted plugin receives bounded execution context
- **WHEN** the runtime invokes a Go-hosted plugin through the supported SDK contract
- **THEN** the SDK exposes only the bounded execution metadata needed for that invocation instead of unrestricted host internals

#### Scenario: Plugin returns a structured runtime error
- **WHEN** a WASM plugin operation fails while handling an invocation
- **THEN** the SDK returns a structured error envelope that the Go host can record in plugin runtime state

### Requirement: Repository includes a verifiable SDK sample plugin
The system SHALL include a sample Go WASM plugin and repository-supported build and debug workflows that produce a module and manifest the Go runtime can register, activate, and health-check during verification. The supported workflows MUST allow developers to target a maintained sample plugin without editing script source, MUST preserve the manifest-resolved artifact path expected by the Go host, and MUST replay the same `AGENTFORGE_*` operation envelope contract used by the runtime for local debugging.

#### Scenario: Sample plugin builds into a valid WASM artifact
- **WHEN** the repository build workflow runs for the maintained sample plugin through the supported script interface
- **THEN** it produces a WASM module artifact at the manifest-resolved path and reports an artifact location that passes plugin validation

#### Scenario: Sample plugin can be debugged through the SDK contract
- **WHEN** a developer invokes the supported debug workflow for the maintained sample plugin with `describe`, `init`, `health`, or another supported operation
- **THEN** the workflow passes the operation, config, and payload through the SDK contract and returns the structured result without requiring manual script edits

#### Scenario: Sample plugin can be activated by the Go runtime
- **WHEN** the sample plugin is registered and activated through the Go plugin APIs after the supported build or debug workflow has prepared the artifact
- **THEN** the Go runtime loads the module successfully and reports the plugin as active

