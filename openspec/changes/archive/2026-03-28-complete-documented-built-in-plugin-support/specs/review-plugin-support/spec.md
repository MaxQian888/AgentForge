## ADDED Requirements

### Requirement: Official built-in review plugins are shipped as manifest-backed bundle entries
The system SHALL allow repository-owned built-in ReviewPlugin assets to be shipped as official built-in bundle entries using the same manifest contract required of custom review plugins. A built-in review plugin MUST declare a valid review manifest, remain installable through the built-in install flow, and preserve built-in source metadata instead of masquerading as an external marketplace plugin.

#### Scenario: Built-in review plugin is discovered from the official bundle
- **WHEN** the repository ships a built-in ReviewPlugin whose bundle entry and manifest are both valid
- **THEN** the control plane exposes that plugin through built-in discovery and catalog flows as an installable review extension candidate

#### Scenario: Built-in review plugin keeps built-in provenance after installation
- **WHEN** an operator installs a built-in ReviewPlugin from the official built-in bundle
- **THEN** the installed review plugin record preserves built-in source metadata while remaining eligible for normal enablement and trigger-based selection

### Requirement: Built-in review plugin executions are distinguished from built-in dimensions
The system SHALL preserve built-in review plugin identity separately from the built-in deep-review dimensions such as logic, security, performance, or compliance. Findings and execution metadata produced by an installed built-in ReviewPlugin MUST record the plugin identifier as plugin provenance rather than collapsing it into a built-in dimension label.

#### Scenario: Built-in review plugin contributes separate provenance
- **WHEN** an installed built-in ReviewPlugin returns findings for a Layer 2 review run
- **THEN** the aggregated review result records that plugin under plugin provenance metadata distinct from the built-in dimension results

#### Scenario: Built-in review plugin failure does not masquerade as a dimension failure
- **WHEN** an installed built-in ReviewPlugin errors or times out during a review run
- **THEN** the execution metadata marks the plugin execution as a plugin failure for that specific plugin id instead of attributing the failure to a built-in dimension name
