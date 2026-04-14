## ADDED Requirements

### Requirement: IM action execution SHALL preserve bridge binding and reply-target lineage
When IM Bridge submits an executable action through the backend, the backend SHALL preserve the originating bridge binding and reply-target lineage needed for follow-up progress and terminal delivery. The backend MUST return enough canonical context for the IM Bridge or control plane to deliver the eventual outcome back to the originating conversation without inventing a new destination.

#### Scenario: Assign-agent action keeps the originating bridge binding
- **WHEN** an IM-originated `assign-agent` action starts a real backend dispatch workflow
- **THEN** the backend action result preserves the bridge binding or reply-target context associated with that originating IM conversation
- **THEN** later progress and terminal updates can be routed through the control plane to the same conversation

#### Scenario: Review action terminal result returns to the same conversation
- **WHEN** an IM-originated review action completes successfully or is blocked
- **THEN** the backend action result preserves the reply-target-aware completion context
- **THEN** the IM Bridge can render the final result in the originating conversation without resolving a new reply target

### Requirement: Workflow success and delivery settlement SHALL remain distinct
The backend SHALL distinguish between successful execution of a workflow and successful delivery of the follow-up IM message. If a workflow completes but the terminal IM delivery cannot be settled, the action result and diagnostics MUST preserve that distinction instead of reporting a fully successful end-to-end IM completion.

#### Scenario: Workflow succeeds but terminal delivery is blocked
- **WHEN** an IM action starts or completes the requested backend workflow but the bound bridge instance is unavailable for the terminal response
- **THEN** the backend records the workflow outcome and the delivery settlement failure separately
- **THEN** operators can see that the action logic succeeded even though the user-facing IM reply did not settle
