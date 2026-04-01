## ADDED Requirements

### Requirement: Role readiness evaluates plugin-scoped tool dependencies
The system SHALL evaluate plugin-scoped tool dependencies declared by a role through `capabilities.toolConfig.external` and `capabilities.toolConfig.mcpServers` against the current plugin registry whenever it builds preview, sandbox, or execution-facing readiness. Missing, disabled, or otherwise unusable plugin dependencies that would prevent the current checkout from honoring the role's declared tool set MUST appear as readiness diagnostics and MUST block execution startup instead of being silently dropped from runtime projection.

#### Scenario: Preview reports a missing role-scoped plugin dependency
- **WHEN** a preview or sandbox request resolves a role whose declared plugin-scoped tool dependency is not currently usable
- **THEN** the response includes a readiness diagnostic for that missing dependency and marks it as blocking for execution-facing readiness
- **THEN** the execution profile does not pretend that the missing dependency is satisfied

#### Scenario: Agent startup rejects a role with a blocking plugin dependency gap
- **WHEN** an agent spawn or workflow step tries to execute with a role whose declared plugin-scoped tool dependency is currently missing or unusable
- **THEN** the startup path rejects execution before bridge runtime begins
- **THEN** the returned error explains which role dependency is blocking readiness

### Requirement: Role deletion guards against installed plugin consumers
The role API SHALL refuse destructive deletion when installed plugins still declare bindings to the target role. Any installed plugin type that declares role references, including WorkflowPlugin manifests, MUST be included in this impact evaluation, and the error response MUST identify the blocking consumers so operators can repair them first.

#### Scenario: Delete role referenced by an installed workflow plugin is blocked
- **WHEN** an operator deletes a role that is still referenced by an installed plugin with declared role bindings
- **THEN** the API does not delete the role
- **THEN** the response identifies the blocking plugin consumer and the referenced role binding instead of returning a generic failure

#### Scenario: Delete unused role keeps existing behavior
- **WHEN** an operator deletes a role that has no remaining installed plugin consumers
- **THEN** the system deletes the role using the existing success path
- **THEN** no dependency-specific blocker is returned

### Requirement: Role manifests preserve plugin function bindings
The system SHALL allow a role manifest to preserve plugin-scoped function selections alongside plugin identifiers. When a role author binds an installed tool plugin to a role and narrows that plugin to a selected subset of declared functions, the canonical role manifest, normalized API response, preview output, and bridge-facing execution profile MUST preserve that binding instead of flattening it into a plugin id string alone.

#### Scenario: Role API round-trips plugin function bindings
- **WHEN** an operator saves a role whose tool configuration binds `repo-search` with selected functions such as `search_code` and `open_file`
- **THEN** subsequent get, list, preview, or sandbox responses preserve the same plugin id and selected function list
- **THEN** the canonical role manifest does not silently drop the function selection metadata

#### Scenario: Execution profile carries plugin function binding metadata
- **WHEN** the system derives a bridge-facing execution profile from a role that contains plugin function bindings
- **THEN** the execution profile includes that plugin binding metadata alongside the plugin identifiers already used for runtime selection
- **THEN** downstream runtimes can inspect the selected function list without reparsing the role YAML
