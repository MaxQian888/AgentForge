## ADDED Requirements

### Requirement: Repository verifies official built-in readiness contracts
The repository SHALL provide a bounded verification workflow that validates official built-in readiness metadata and deterministic readiness preflight behavior without requiring secret-dependent or network-dependent live execution. Verification MUST identify which built-in entry and which readiness stage failed.

#### Scenario: Readiness verification catches bundle metadata drift
- **WHEN** an official built-in bundle entry declares malformed or incomplete readiness metadata
- **THEN** the readiness verification workflow exits non-zero
- **THEN** the failure identifies the affected built-in entry and readiness field

#### Scenario: Readiness verification reports missing deterministic prerequisite
- **WHEN** a readiness preflight evaluates a built-in plugin that depends on a missing local tool or host capability
- **THEN** the verification output reports that deterministic prerequisite failure for the affected built-in
- **THEN** the workflow does not replace that result with an unrelated generic plugin failure
