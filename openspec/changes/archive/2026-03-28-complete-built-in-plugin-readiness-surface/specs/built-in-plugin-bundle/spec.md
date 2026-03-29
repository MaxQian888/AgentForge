## ADDED Requirements

### Requirement: Bundle entries declare structured readiness contracts
Each official built-in bundle entry SHALL declare structured readiness metadata that the control plane can evaluate without inferring behavior from free-form text alone. The readiness contract MUST declare the verification profile, maintained docs reference, prerequisite or configuration categories when applicable, and operator-facing next-step guidance needed to explain why a built-in is or is not ready.

#### Scenario: Bundle entry declares missing-configuration guidance
- **WHEN** an official built-in plugin requires secrets or persisted config before activation can succeed
- **THEN** its bundle entry declares that configuration requirement in structured readiness metadata
- **THEN** the readiness contract can be surfaced without hardcoding plugin-specific UI rules

#### Scenario: Bundle verification rejects incomplete readiness metadata
- **WHEN** an official built-in bundle entry omits required readiness contract fields for a non-ready built-in
- **THEN** repository verification fails before that entry is treated as a valid official built-in
- **THEN** the failure identifies which bundle entry and readiness field drifted
