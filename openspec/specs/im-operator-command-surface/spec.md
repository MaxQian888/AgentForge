# im-operator-command-surface Specification

## Purpose
Define the canonical AgentForge IM operator command catalog so slash commands, aliases, help output, and natural-language guidance stay aligned with existing task, agent, queue, team, and memory collaboration surfaces.
## Requirements
### Requirement: Canonical operator command catalog stays discoverable and backward-compatible
The IM Bridge SHALL define one canonical operator command catalog that enumerates supported command families, subcommands, aliases, usage strings, and short descriptions. Slash-command registration, help output, usage errors, and natural-language guidance MUST derive from this catalog. When a legacy command spelling remains supported for compatibility, the catalog MUST mark one canonical name for docs and help while continuing to accept the legacy alias until a future explicit removal change.

#### Scenario: Help output is generated from the canonical catalog
- **WHEN** an IM user sends /help
- **THEN** the Bridge replies with a command list generated from the canonical operator catalog
- **AND** the list includes the approved task, agent, review, sprint, queue, team, memory, cost, and fallback guidance entries for the current build

#### Scenario: Legacy alias stays accepted while docs point at the canonical name
- **WHEN** an IM user sends /agent list
- **THEN** the Bridge handles the request as the canonical agent status command
- **AND** the reply or usage guidance references the canonical name without rejecting the legacy alias

### Requirement: Agent commands expose runtime status and lifecycle control
The /agent command family SHALL expose the existing AgentForge runtime control APIs through IM. The canonical surface MUST support pool status, single-run status, logs, pause, resume, and kill in addition to the existing spawn and run flows, and it MUST translate queued, paused, running, completed, and blocked states into IM-readable summaries.

#### Scenario: Agent status without an id returns pool summary
- **WHEN** an IM user sends /agent status
- **THEN** the Bridge queries the canonical agent pool status API
- **AND** the reply summarizes active, available, queued, and resumable capacity in IM-readable form

#### Scenario: Agent status with an id returns run detail
- **WHEN** an IM user sends /agent status run-123
- **THEN** the Bridge queries the canonical agent run detail API for run-123
- **AND** the reply includes the run status, task identity, runtime identity, and the next supported control actions when applicable

#### Scenario: Agent pause or resume reuses backend runtime control
- **WHEN** an IM user sends /agent pause run-123 or /agent resume run-123
- **THEN** the Bridge calls the corresponding backend pause or resume endpoint for that run id
- **AND** the IM reply reflects the resulting paused, resumed, blocked, or not-found state without inventing a separate IM-only lifecycle

#### Scenario: Agent kill terminates a run through the canonical endpoint
- **WHEN** an IM user sends /agent kill run-123
- **THEN** the Bridge calls the canonical agent kill endpoint for run-123
- **AND** the IM reply confirms the run is cancelling or cancelled, or returns the backend failure reason verbatim in readable form

### Requirement: Task and queue commands expose lightweight orchestration control
The IM command surface SHALL let users perform lightweight task workflow control and queue management through existing backend APIs. The /task family MUST keep create, list, status, assign, and decompose, and it MUST add one canonical status-transition subcommand. The /queue family MUST support project-scoped queue listing and cancellation with readable results and preserved error semantics for non-cancellable entries.

#### Scenario: Task move transitions workflow status
- **WHEN** an IM user sends /task move task-123 done
- **THEN** the Bridge calls the canonical task status-transition endpoint for task-123 with target status done
- **AND** the reply confirms the updated workflow state and includes the task identity needed for follow-up actions

#### Scenario: Queue list returns project-scoped admission summaries
- **WHEN** an IM user sends /queue list queued
- **THEN** the Bridge calls the canonical project queue list endpoint with the requested filter
- **AND** the reply summarizes matching queued entries with task, member or runtime identity, priority, and reason in a compact IM format

#### Scenario: Queue cancel preserves backend conflict semantics
- **WHEN** an IM user sends /queue cancel entry-123
- **THEN** the Bridge calls the canonical project queue cancel endpoint for entry-123
- **AND** the IM reply distinguishes successful cancellation from already-completed or invalid-entry conflicts instead of flattening all outcomes into generic success text

### Requirement: Team and memory commands expose project collaboration context
The IM command surface SHALL provide concise project-scoped collaboration context through team summary and memory commands. The canonical surface MUST support team summary listing backed by existing project team or member APIs, memory search, and one lightweight project-scoped memory note write path with deterministic defaults that do not require the caller to supply raw repository fields.

#### Scenario: Team list returns project collaboration summary
- **WHEN** an IM user sends /team list
- **THEN** the Bridge calls the canonical project team or member listing API for the configured project scope
- **AND** the reply summarizes the active human and agent collaborators with enough identity or readiness detail to guide assignment decisions

#### Scenario: Memory search returns top project-scoped matches
- **WHEN** an IM user sends /memory search release plan
- **THEN** the Bridge calls the canonical project memory search API with query text release plan
- **AND** the reply returns a compact list of the highest-relevance memory hits with truncated content and stable identifiers or links

#### Scenario: Memory note stores a lightweight operator record
- **WHEN** an IM user sends /memory note Remember to reuse the Codex runtime for release triage
- **THEN** the Bridge stores a project-scoped memory note through the canonical project memory write API using deterministic defaults for scope and category
- **AND** the reply confirms the note was stored with a stable identifier that can be referenced later

### Requirement: Usage errors and natural-language guidance resolve through the command catalog
When a user invokes an unknown subcommand or a mention-based natural-language request that maps to a supported operator flow, the Bridge SHALL use the canonical catalog to return the closest supported command or usage text. The fallback MUST NOT invent commands that are not present in the catalog.

#### Scenario: Unknown subcommand returns canonical usage guidance
- **WHEN** an IM user sends /task transition without the required arguments
- **THEN** the Bridge replies with the canonical usage for the supported task workflow control command
- **AND** the response points at the catalog-approved command shape rather than a stale or removed spelling

#### Scenario: Mention-based request suggests a supported command
- **WHEN** an IM user sends @AgentForge 暂停 run-123
- **THEN** the natural-language fallback maps the request to the supported agent pause flow or suggests /agent pause run-123 explicitly
- **AND** the reply does not reference a command that is absent from the canonical operator catalog

