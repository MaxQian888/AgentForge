## MODIFIED Requirements

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
