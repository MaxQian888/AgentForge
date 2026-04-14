## ADDED Requirements

### Requirement: IM Bridge SHALL route capabilities by backend responsibility
IM Bridge command routing SHALL distinguish Bridge-proxied capabilities, backend-native workflow capabilities, and backend-mediated delivery capabilities. Commands that require TS Bridge AI or runtime diagnostics MUST go through Go proxy endpoints; commands that operate on backend workflows directly MUST stay on canonical Go workflow endpoints.

#### Scenario: Bridge-backed command uses Go proxy
- **WHEN** a user invokes an IM command that needs Bridge runtime or AI capability, such as runtime listing or intent classification
- **THEN** the IM Bridge calls the corresponding Go backend proxy endpoint
- **THEN** the Go backend invokes TS Bridge on behalf of that request
- **THEN** the IM Bridge does not replace the proxy flow with a direct TS Bridge call

#### Scenario: Backend-native command bypasses Bridge
- **WHEN** a user invokes an IM command whose canonical implementation is a backend workflow, such as task creation
- **THEN** the IM Bridge calls the backend workflow endpoint directly
- **THEN** the command does not proxy through TS Bridge unless the workflow explicitly requests Bridge AI assistance

### Requirement: IM-facing Bridge failures SHALL remain source-aware and truthful
When a Bridge-backed IM command cannot complete because the Bridge upstream is unavailable, the runtime is not ready, or only a backend fallback is available, the user-visible response and logs SHALL preserve that failure source instead of returning an ambiguous command failure.

#### Scenario: Runtime-backed command fails because the runtime is not ready
- **WHEN** a user invokes an IM command that depends on a runtime whose readiness diagnostics report a missing executable or authentication prerequisite
- **THEN** the IM-facing response identifies that runtime readiness failure
- **THEN** the command result does not claim that the Bridge capability succeeded

#### Scenario: Decompose command falls back to backend-native decomposition
- **WHEN** a user invokes `/task decompose` and the Bridge upstream is unavailable but a documented backend fallback exists
- **THEN** the IM Bridge records and reports that the result came from fallback behavior
- **THEN** operators can distinguish fallback success from normal Bridge-backed execution
