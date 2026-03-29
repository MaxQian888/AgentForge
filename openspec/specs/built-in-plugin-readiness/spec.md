# built-in-plugin-readiness Specification

## Purpose
Define the operator-facing readiness contract for official built-in plugins so discovery, catalog, and installed-plugin surfaces can distinguish installability from current activation readiness and show actionable setup guidance.
## Requirements
### Requirement: Official built-in plugins expose evaluated readiness state
The system SHALL evaluate each official built-in plugin against a repository-owned readiness contract before returning operator-facing built-in discovery or catalog data. The evaluated readiness state MUST distinguish at least `ready`, `requires_prerequisite`, `requires_configuration`, and `unsupported_host`, and MUST include stable machine-readable blocking reasons rather than only free-form prose.

#### Scenario: Built-in plugin is ready on the current host
- **WHEN** an official built-in plugin satisfies its declared prerequisite and configuration checks
- **THEN** the control plane returns that built-in with readiness state `ready`
- **THEN** the response includes no blocking reasons for that built-in

#### Scenario: Built-in plugin is blocked by a missing prerequisite
- **WHEN** an official built-in plugin depends on a local tool or runtime that is not available on the current host
- **THEN** the control plane returns readiness state `requires_prerequisite`
- **THEN** the response identifies the missing prerequisite with a stable blocking reason that the frontend can render

#### Scenario: Built-in plugin is unsupported on the current host
- **WHEN** an official built-in plugin cannot run on the current host family or runtime surface supported by the current checkout
- **THEN** the control plane returns readiness state `unsupported_host`
- **THEN** the response does not imply that activation can succeed on the current host

### Requirement: Built-in readiness includes actionable setup guidance
The system SHALL attach operator-facing setup guidance to evaluated built-in readiness so the plugin console can explain what to do next without inspecting raw manifests or bundle files. Setup guidance MUST include the documentation reference, a short next-step summary, and any declared configuration keys or prerequisite labels that block readiness.

#### Scenario: Built-in plugin requires configuration before activation
- **WHEN** an official built-in plugin is installable but still depends on missing configuration
- **THEN** the control plane returns readiness state `requires_configuration`
- **THEN** the response includes the configuration items or labels that must be satisfied before activation can succeed

#### Scenario: Built-in readiness guidance references maintained docs
- **WHEN** the control plane returns readiness guidance for an official built-in plugin
- **THEN** that guidance includes the maintained documentation reference declared for the built-in
- **THEN** the guidance remains tied to the official built-in bundle instead of ad hoc UI-only copy
