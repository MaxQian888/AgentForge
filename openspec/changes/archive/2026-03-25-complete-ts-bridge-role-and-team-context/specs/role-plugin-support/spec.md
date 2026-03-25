## MODIFIED Requirements

### Requirement: Role manifests can be projected into execution profiles
The system SHALL derive a normalized execution profile from a resolved role manifest for downstream agent execution. The execution profile MUST include the runtime-facing role data needed by the current Bridge contract, including the effective role identifier and name, system prompt, tool allowlist, bridge tool or plugin identifiers, injected knowledge context, output filters, budget or turn limits, and permission mode, while preserving richer PRD-only fields in the stored role model.

#### Scenario: Execution profile is derived from a resolved role
- **WHEN** the system resolves a valid role manifest for execution use
- **THEN** it emits a normalized execution profile containing the runtime-facing prompt, tool allowlist, plugin identifiers, knowledge context, output filters, budget, turn, and permission settings derived from that role

#### Scenario: Execution profile is built from the fully resolved role
- **WHEN** a child role inherits settings from a parent role
- **THEN** the derived execution profile reflects the post-merge effective values for prompt, tools, knowledge context, output filters, and guardrails instead of only the child YAML fragment

#### Scenario: Non-runtime role metadata remains available without leaking into execution config
- **WHEN** a role manifest contains collaboration, memory, or trigger metadata that the current bridge runtime path does not yet execute
- **THEN** the system preserves that metadata in the normalized role record
- **THEN** the execution profile excludes those unsupported sections rather than silently dropping the stored data or sending raw YAML-shaped payloads to the Bridge
