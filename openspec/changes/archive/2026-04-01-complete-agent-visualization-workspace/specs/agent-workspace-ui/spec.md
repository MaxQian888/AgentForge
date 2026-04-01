## MODIFIED Requirements

### Requirement: Agent workspace exposes one consistent operator workspace shell
The system SHALL present the operator-facing agent experience as one consistent workspace shell that supports monitor, visualization, and dispatch views, URL-driven agent selection, and shared sidebar/detail framing instead of scattering those affordances across unrelated page-level layouts.

#### Scenario: Operator switches between monitor, visualization, and dispatch views
- **WHEN** the operator is on `/agents` and switches between the monitor, visualization, and dispatch views
- **THEN** the workspace presents those views through one first-class navigation control within the same shell
- **THEN** the sidebar, toolbar, and page framing remain consistent while only the active view content changes

#### Scenario: Operator deep-links to a specific agent
- **WHEN** the operator opens `/agent?id=<agent-id>` or `/agents?agent=<agent-id>`
- **THEN** the experience resolves into the same agent workspace shell
- **THEN** the selected agent detail is shown without creating a separate, divergent page layout for agent detail

#### Scenario: Operator selects an agent from the visualization
- **WHEN** the operator chooses an agent from the visualization view
- **THEN** the workspace updates the same URL-driven selection state used by the rest of the agent workspace
- **THEN** the operator reaches the existing agent detail experience with the same controls and output surfaces
