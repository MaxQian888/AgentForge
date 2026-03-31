# cli-agent-runtime-adapters Specification

## Purpose
Define the shared TypeScript bridge contract for CLI-backed coding-agent runtimes so additional backends such as Cursor Agent, Gemini CLI, Qoder CLI, and iFlow can be registered, diagnosed, and executed through one truthful adapter family without pretending full parity with the dedicated Claude Code, Codex, and OpenCode connectors.
## Requirements
### Requirement: Bridge registers additional CLI-backed coding-agent runtime profiles
The TypeScript bridge SHALL support `cursor`, `gemini`, `qoder`, and `iflow` as first-class coding-agent runtimes through one shared CLI-backed runtime profile family. Each runtime profile MUST declare its canonical runtime key, display label, executable discovery rule, provider compatibility metadata, default selection metadata, supported feature flags, and backend-specific readiness prerequisites before that runtime can appear in the runtime catalog.

#### Scenario: Runtime catalog includes additional CLI-backed backends
- **WHEN** the bridge publishes runtime catalog metadata after this change
- **THEN** the catalog includes dedicated entries for `cursor`, `gemini`, `qoder`, and `iflow` instead of collapsing them into a generic placeholder runtime
- **THEN** each entry exposes its own label, runtime key, default selection metadata, and supported feature flags

### Requirement: CLI-backed runtime profiles surface backend-specific readiness diagnostics
The bridge SHALL evaluate readiness for each CLI-backed runtime profile using the truthful prerequisites that backend actually needs, including executable discovery, login or API-key state, provider-profile setup, and supported model constraints.

#### Scenario: Cursor runtime command is missing
- **WHEN** the bridge evaluates readiness for `cursor` and cannot discover the configured Cursor Agent executable
- **THEN** the `cursor` runtime SHALL be marked unavailable
- **THEN** the diagnostics SHALL identify the missing executable or install prerequisite instead of returning a generic runtime failure

#### Scenario: Gemini runtime authentication is incomplete
- **WHEN** the bridge evaluates readiness for `gemini` and no supported Gemini login or API-key profile is configured
- **THEN** the `gemini` runtime SHALL be marked unavailable
- **THEN** the diagnostics SHALL identify the missing authentication prerequisite before any execute request is attempted

### Requirement: CLI-backed runtime adapters normalize native output truthfully
The bridge SHALL translate native Cursor Agent, Gemini CLI, Qoder CLI, and iFlow output into the canonical AgentForge runtime event model without inventing unsupported cost, tool, or continuity details. When a backend exposes only partial lifecycle or usage data, the adapter MUST emit the closest truthful canonical subset and still preserve consistent terminal behavior.

#### Scenario: CLI-backed runtime emits partial native metadata
- **WHEN** a CLI-backed runtime produces assistant output and progress data but does not expose a native equivalent for cost or detailed tool payloads
- **THEN** the bridge SHALL emit the assistant output and any truthful progress or reasoning events it can normalize
- **THEN** the bridge SHALL complete or fail the runtime with stable canonical status semantics instead of fabricating missing cost or tool details

### Requirement: CLI-backed runtime lifecycle controls are capability-gated
The bridge SHALL expose advanced lifecycle controls for a CLI-backed runtime only when that runtime profile explicitly supports them. Unsupported pause, resume, fork, rollback, revert, or set-model operations MUST return explicit unsupported feedback, and any persisted snapshot for that runtime MUST mark continuity as blocked instead of replaying the original task as if a truthful resume were available.

#### Scenario: Unsupported resume is rejected explicitly
- **WHEN** an operator or backend caller attempts to resume a paused CLI-backed runtime whose profile does not support truthful resume semantics
- **THEN** the bridge SHALL reject the resume with explicit unsupported or blocked-continuity feedback
- **THEN** the bridge SHALL NOT start a fresh execution from the original prompt while presenting it as a successful resume
