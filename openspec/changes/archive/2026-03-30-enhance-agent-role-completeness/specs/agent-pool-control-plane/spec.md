## MODIFIED Requirements

### Requirement: AgentPool lifecycle is visible to operator-facing APIs and realtime consumers

The agents dashboard SHALL integrate the dispatch history panel to surface dispatch attempt history for operators. The dispatch preflight dialog SHALL be accessible from the agents page so operators can run preflight checks without entering the spawn flow. These components SHALL be presented in a "Dispatch" tab alongside the existing pool metrics and agents table views.

#### Scenario: Operator views dispatch history on agents page
- **WHEN** operator navigates to the agents dashboard and selects the Dispatch tab
- **THEN** the dispatch history panel displays recent dispatch attempts with task identity, member, runtime, outcome, and timestamp

#### Scenario: Operator runs preflight check from agents page
- **WHEN** operator clicks a preflight check action on the Dispatch tab
- **THEN** the dispatch preflight dialog opens showing admission likelihood, budget status, and pool snapshot
- **AND** the dialog uses the same preflight data as the spawn flow

#### Scenario: Dispatch tab shows event count badge
- **WHEN** the agents page loads and there are recent dispatch events
- **THEN** the Dispatch tab label displays a badge with the count of recent events
