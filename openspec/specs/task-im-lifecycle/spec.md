# task-im-lifecycle Specification

## Purpose
Define the canonical task lifecycle contract for IM surfaces so task commands, task card actions, reply-target binding, and task or workflow follow-up stay truthful across callback-ready and text-first providers.
## Requirements
### Requirement: IM task responses SHALL expose canonical task state and truthful next-step affordances
The system SHALL expose task-oriented IM responses as a canonical task lifecycle surface instead of a loose collection of ad hoc replies. When an IM user creates, inspects, or advances a task, the response MUST identify the task, its current workflow state, and the next supported task actions. If the active provider can render structured or native cards truthfully, the response MAY include interactive affordances; otherwise it MUST fall back to readable manual guidance without losing task identity.

#### Scenario: Card-capable provider returns a task summary card with truthful follow-up actions
- **WHEN** an IM user requests task status or completes a task action on a provider that supports structured or native task cards
- **THEN** the Bridge returns a task summary payload that includes task identity, title, current status, and the supported next-step actions for that task
- **AND** the returned affordances only include actions that the current runtime can actually execute

#### Scenario: Text-first provider falls back to manual task guidance
- **WHEN** an IM user requests task status or completes a task action on a provider or reply target that cannot support the preferred richer task card output
- **THEN** the Bridge returns a readable plain-text or structured-text response with the same task identity and state
- **AND** the response includes manual command guidance for the supported next task steps instead of pretending interactive actions are available

### Requirement: IM-originated task lifecycle SHALL preserve reply-target binding for follow-up outcomes
When a task lifecycle action originates from IM, the system SHALL preserve task-scoped reply-target lineage so later task-triggered follow-up can return to the same conversation, thread, or callback context. The backend MUST prefer that preserved task binding over guessed routing derived from task metadata alone.

#### Scenario: IM task transition keeps the original conversation for follow-up delivery
- **WHEN** an IM user transitions a task through the canonical task lifecycle surface and that transition later produces a task or workflow follow-up result
- **THEN** the backend preserves task-scoped binding to the originating reply target
- **AND** the follow-up result is delivered back into the same conversation or thread instead of a new unrelated destination

#### Scenario: Missing task binding remains explicit
- **WHEN** a task follow-up result is ready but the system cannot resolve a preserved task-scoped reply target or live bound bridge instance
- **THEN** the system records that the task follow-up could not be delivered through bound IM routing
- **AND** it does not silently retarget the result to an unrelated default channel

### Requirement: Provider-aware task actions SHALL be gated by real callback readiness
Interactive task actions SHALL only be exposed when the active provider and runtime are actually ready to receive and execute them. If callback-backed or mutable task actions are unavailable for the current provider, the Bridge MUST fall back to non-interactive guidance that remains truthful to the same task lifecycle contract.

#### Scenario: Callback-ready Feishu task card exposes interactive task actions
- **WHEN** the active provider is Feishu and the current runtime has usable callback readiness for task card actions
- **THEN** the Bridge renders callback-backed task actions through the Feishu card surface
- **AND** those actions map to the canonical backend task lifecycle contract instead of provider-only behavior

#### Scenario: Callback-missing provider omits unavailable task actions
- **WHEN** the active provider or runtime does not have usable callback readiness for task card actions
- **THEN** the Bridge omits callback-backed task actions from the task response
- **AND** it provides manual command guidance for the same supported task lifecycle steps
