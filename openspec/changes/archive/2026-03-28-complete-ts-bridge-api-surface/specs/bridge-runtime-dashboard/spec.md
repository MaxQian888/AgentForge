## ADDED Requirements

### Requirement: Frontend displays Bridge runtime catalog on Agents page
Agents page SHALL display available runtimes from `GET /api/v1/bridge/runtimes` including runtime key, display name, provider, availability status, and diagnostics.

#### Scenario: Runtimes loaded successfully
- **WHEN** Agents page mounts and bridge is ready
- **THEN** page displays runtime catalog cards showing each runtime's name, default provider/model, and availability badge

#### Scenario: Bridge is degraded
- **WHEN** Agents page mounts and bridge health is `degraded`
- **THEN** page displays degraded banner and runtime catalog shows all runtimes as unavailable

### Requirement: Frontend displays Bridge pool summary on Agents page
Agents page SHALL display pool summary (active slots, available slots, warm slots, queued count) fetched from existing agent store data enriched with `GET /api/v1/bridge/health` pool info.

#### Scenario: Pool summary displayed
- **WHEN** Agents page loads with active agents
- **THEN** pool summary cards show active/available/warm/queued counts matching bridge pool state

### Requirement: Frontend provides SpawnAgentDialog for single agent execution
Task detail view SHALL provide a dialog for single agent spawn with runtime, provider, model, and budget selection. The dialog SHALL reuse the shared `RuntimeSelector` component.

#### Scenario: User spawns single agent with custom config
- **WHEN** user clicks "Start Agent" on a task and selects runtime=claude_code, provider=anthropic, model=claude-sonnet-4-20250514, budget=5.00
- **THEN** system calls `POST /api/v1/agents/spawn` with selected configuration and agent appears in running state

#### Scenario: User spawns single agent with defaults
- **WHEN** user clicks "Start Agent" and immediately clicks "Start" without changing defaults
- **THEN** system uses default runtime/provider/model from catalog and default budget from project settings

### Requirement: RuntimeSelector is a shared component used by both single and team spawn
A `RuntimeSelector` component SHALL be extracted from `StartTeamDialog` and used by both `StartTeamDialog` and `SpawnAgentDialog`. The component SHALL accept runtime catalog data and emit selected runtime/provider/model.

#### Scenario: RuntimeSelector shows available options
- **WHEN** RuntimeSelector renders with catalog data containing 3 runtimes
- **THEN** runtime dropdown shows 3 options, provider dropdown filters to compatible providers, model dropdown shows default

#### Scenario: RuntimeSelector disables unavailable runtimes
- **WHEN** catalog includes a runtime with `available: false`
- **THEN** that runtime option is disabled with diagnostic tooltip

### Requirement: Frontend displays paused agents with resume action
Agents page SHALL show a section for paused agents (filtered from agent roster where `state === 'paused'`). Each paused agent row SHALL provide a "Resume" button.

#### Scenario: User resumes a paused agent
- **WHEN** user clicks "Resume" on a paused agent
- **THEN** system calls `POST /api/v1/agents/:id/resume` and agent transitions to `running` state in the UI

#### Scenario: No paused agents
- **WHEN** no agents have `paused` state
- **THEN** paused agents section is hidden
