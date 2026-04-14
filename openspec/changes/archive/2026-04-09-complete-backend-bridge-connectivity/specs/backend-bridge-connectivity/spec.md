## ADDED Requirements

### Requirement: Go backend SHALL remain the canonical backend connectivity hub
The system SHALL treat the Go backend as the only backend orchestration hub between the TypeScript Bridge, IM Bridge instances, and external coding-agent runtimes. TS Bridge runtime execution, lightweight AI proxy calls, IM control-plane delivery, and backend workflow execution MUST all traverse canonical Go-owned seams, and project documentation MUST NOT describe TS Bridge as directly calling IM Bridge.

#### Scenario: IM-triggered agent execution follows the canonical hub topology
- **WHEN** a user triggers an agent-capable workflow from an IM conversation
- **THEN** the IM Bridge sends the request to the Go backend
- **THEN** the Go backend invokes the canonical TS Bridge execution surface for runtime work
- **THEN** any follow-up IM progress or terminal delivery is routed back through the Go backend control plane to the bound IM Bridge instance

#### Scenario: Source-of-truth docs describe the real topology
- **WHEN** a spec, proposal, or operator-facing document describes backend connectivity
- **THEN** it identifies the Go backend as the mediator between TS Bridge and IM Bridge
- **THEN** it does not describe TS Bridge as directly discovering, targeting, or invoking IM Bridge instances

### Requirement: Backend connectivity SHALL be defined and verified as end-to-end flows
The system SHALL define backend connectivity completeness by end-to-end flows rather than isolated modules. At minimum, the canonical flows MUST cover Go-to-Bridge runtime execution, IM-to-Go-to-Bridge AI routing, IM-to-Go workflow execution, Go-to-IM control-plane delivery, and operator diagnostics for broken seams.

#### Scenario: Runtime diagnostics identify the broken hop
- **WHEN** a backend-managed runtime request cannot execute successfully
- **THEN** the reported diagnostics identify whether the failing seam is Bridge upstream reachability, runtime readiness, backend request validation, or IM delivery reachability
- **THEN** the failure is not collapsed into a generic connectivity error

#### Scenario: Backend-native workflow bypasses Bridge intentionally
- **WHEN** an IM action targets a backend-native workflow such as task creation or review state transition
- **THEN** the IM Bridge invokes the Go backend canonical workflow endpoint directly
- **THEN** the flow does not proxy through TS Bridge unless the workflow explicitly requires Bridge capabilities
