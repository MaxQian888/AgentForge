## MODIFIED Requirements

### Requirement: Agent spawn starts a real execution runtime
The system SHALL turn an authenticated spawn request into an AgentPool admission flow that either starts a real runtime immediately or records a truthful queued admission outcome when capacity is temporarily unavailable. When the request is admitted immediately, the spawn flow MUST create a new agent run in `starting` state, resolve the canonical execution context for that run, provision an isolated worktree, call the configured Bridge execute endpoint with the task, member, runtime and provider tuple, expanded role execution profile, and any applicable team execution context, persist the resulting branch, worktree, and session identifiers on the task, and mark the run as `running` only after the Bridge accepts the execution request.

#### Scenario: Successful spawn provisions runtime state immediately
- **WHEN** an authenticated client submits a valid spawn request for a task that has no active agent run and AgentPool admission has an available slot
- **THEN** the system creates a new agent run in `starting` state
- **THEN** the system resolves the execution context for that run, including role-derived runtime fields and any team-aware context that applies to the spawn
- **THEN** the system provisions a worktree and deterministic agent branch for that task
- **THEN** the system invokes the configured Bridge execute API with the canonical execution context and managed workspace information
- **THEN** the system stores `agent_branch`, `agent_worktree`, and `agent_session_id` on the task
- **THEN** the system updates the agent run status to `running`

#### Scenario: Spawn request is queued by AgentPool admission
- **WHEN** an authenticated client submits a valid spawn request for a task that has no active agent run but AgentPool admission has no immediate slot available
- **THEN** the system records a queue entry for that spawn request that preserves the canonical runtime selection and role context needed for later admission
- **THEN** the synchronous result reports that the request is `queued`
- **THEN** the system MUST NOT create a real agent run until the queued request is later admitted
