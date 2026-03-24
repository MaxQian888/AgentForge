## ADDED Requirements

### Requirement: Bridge execute requests accept normalized role execution profiles from Go
The TypeScript bridge SHALL treat `role_config` in execute requests as a normalized execution profile produced by the Go role-loading pipeline rather than as a raw Role YAML document. The bridge MUST apply the projected role persona and tool constraints without needing to read YAML files, resolve inheritance, or interpret PRD-only role metadata locally.

#### Scenario: Normalized role execution profile is honored
- **WHEN** the Go orchestrator submits an execute request with a valid normalized `role_config`
- **THEN** the bridge uses that projected role configuration when composing the effective system prompt and tool constraints for the runtime

#### Scenario: Bridge does not need direct YAML access
- **WHEN** the bridge receives a valid execute request whose role was loaded from disk by Go
- **THEN** the bridge executes the task without reading the roles directory or parsing the source YAML itself

### Requirement: Bridge rejects non-normalized role payloads
The bridge SHALL validate `role_config` against the normalized execution-profile contract and MUST reject payloads that omit required execution fields or attempt to send raw nested Role YAML structures that belong to the Go-side role model.

#### Scenario: Execute request with incomplete role execution data is rejected
- **WHEN** the Go orchestrator submits an execute request whose `role_config` omits required normalized execution fields such as the projected role name or system prompt inputs
- **THEN** the bridge returns a validation error and does not start execution

#### Scenario: Raw YAML-shaped role payload is rejected
- **WHEN** an execute request includes nested PRD role sections such as raw `metadata`, `knowledge`, or `security` objects where a normalized execution profile is expected
- **THEN** the bridge rejects the payload instead of trying to interpret it as runtime configuration
