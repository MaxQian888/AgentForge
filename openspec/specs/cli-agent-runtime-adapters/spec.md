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

### Requirement: CLI runtime launches follow documented headless contracts
The TypeScript bridge SHALL define a documented headless launch contract for each CLI-backed runtime (`cursor`, `gemini`, `qoder`, `iflow`) covering how prompt input is supplied, how machine-readable output is requested, how model and workspace options are passed, and which per-run approval controls are officially supported. The bridge MUST NOT treat undocumented stdin payloads, alias flags, or guessed option names as valid substitutes for an official headless launch path.

#### Scenario: Cursor launch uses documented headless prompt transport
- **WHEN** an execute request targets `cursor`
- **THEN** the bridge SHALL launch Cursor through its documented headless prompt entrypoint instead of depending on stdin-only prompt delivery
- **THEN** any additional model or approval flags SHALL be limited to parameters Cursor currently documents for that headless path

#### Scenario: Qoder launch rejects undocumented output flag assumptions
- **WHEN** an execute request targets `qoder` and the bridge needs machine-readable output
- **THEN** the bridge SHALL use only the documented Qoder print-mode and output-format parameters
- **THEN** it SHALL NOT launch Qoder with undocumented output flag aliases or guessed permission flags

### Requirement: CLI runtime readiness reflects official auth and config prerequisites
The bridge SHALL evaluate each CLI-backed runtime against the authentication, login, configuration, and profile prerequisites that its current official documentation declares for headless execution. A runtime MUST NOT be marked ready based only on binary discovery or repo-local defaults when its documented auth or configuration state is missing.

#### Scenario: Gemini readiness accepts current official auth modes
- **WHEN** `gemini` is configured through a currently supported official auth path such as login, API key, or provider-backed configuration
- **THEN** the bridge SHALL treat that official auth state as satisfying readiness
- **THEN** it SHALL NOT require a narrower bridge-only env-var assumption when the runtime is otherwise ready

#### Scenario: Cursor binary exists but official auth state is missing
- **WHEN** the bridge can discover the `cursor` executable but Cursor's documented headless auth or login prerequisite is not satisfied
- **THEN** the `cursor` runtime SHALL be reported unavailable
- **THEN** diagnostics SHALL identify the missing official auth prerequisite before execution is attempted

### Requirement: CLI runtime output normalization is gated by documented machine-readable modes
The bridge SHALL normalize Cursor, Gemini, Qoder, and iFlow output only through machine-readable modes that the selected runtime officially documents for the invoked headless path. When a runtime only documents plain-text output or an unstable subset of event types for the requested path, the bridge MUST either publish a reduced text-only capability surface or reject advanced event expectations before launch.

#### Scenario: Gemini publishes stream JSON output
- **WHEN** the bridge launches `gemini` through its documented streaming JSON headless mode
- **THEN** the adapter SHALL parse only the event shapes published for that mode
- **THEN** the bridge SHALL derive reasoning, tool, or progress events from that documented stream rather than from shell-text heuristics

#### Scenario: iFlow headless path lacks documented structured stream semantics
- **WHEN** the selected `iflow` execution path does not publish a stable documented machine-readable event stream for a requested advanced signal
- **THEN** the bridge SHALL degrade or reject that advanced signal before execution
- **THEN** catalog metadata and execute preflight SHALL report the same limitation

### Requirement: Deprecated CLI runtimes surface lifecycle truth
The bridge SHALL treat vendor-published deprecation or sunset notices as runtime readiness facts. When a CLI-backed runtime has a published sunset date or documented replacement path, the runtime catalog and execute preflight MUST surface that lifecycle state together with migration guidance. Before the sunset date, the runtime MAY remain launchable only with degraded diagnostics; after the sunset date, new launches MUST be rejected unless the runtime contract has been explicitly revalidated.

#### Scenario: iFlow is inside the published shutdown window
- **WHEN** the bridge evaluates `iflow` before the published shutdown date of 2026-04-17 Beijing Time
- **THEN** the runtime catalog SHALL mark `iflow` as degraded with sunset diagnostics and migration guidance to Qoder
- **THEN** operator-facing consumers SHALL be able to distinguish lifecycle risk from install/auth failures

#### Scenario: iFlow sunset date has passed
- **WHEN** the current time is after 2026-04-17 Beijing Time and no replacement contract override has been configured
- **THEN** execute preflight SHALL reject new `iflow` launches with a sunset or deprecation error
- **THEN** the runtime catalog SHALL NOT present `iflow` as a normal available backend

