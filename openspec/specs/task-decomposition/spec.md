# task-decomposition Specification

## Purpose
Define the baseline contract for decomposing an existing task into structured child tasks through the TypeScript Bridge, the Go task API, and the IM command surface, while preserving project ownership, preventing duplicate decomposition, and avoiding partial child-task creation.
## Requirements
### Requirement: API can decompose an existing task via Bridge
The system MUST provide a `POST /api/v1/tasks/:id/decompose` operation that loads an existing task, sends a structured decomposition request through the TypeScript Bridge, and returns a structured result for the same parent task.

#### Scenario: Successful decomposition request
- **WHEN** an authenticated caller requests decomposition for an existing task and the Bridge returns a valid decomposition result
- **THEN** the system returns success for the same parent task
- **AND** the response includes a readable summary and the created subtasks

#### Scenario: Parent task does not exist
- **WHEN** a caller requests decomposition for a task id that is not present
- **THEN** the system MUST return a not-found error
- **AND** the system MUST NOT create any subtasks

### Requirement: Decomposition results are persisted as child tasks
The system MUST persist each accepted decomposition item as a child task of the requested parent task, using the existing task model and project ownership boundaries.

#### Scenario: Child tasks inherit parent linkage
- **WHEN** the system creates subtasks from a decomposition result
- **THEN** every created task MUST reference the requested task as its `parent_id`
- **AND** every created task MUST belong to the same project as the parent task
- **AND** every created task MUST start in an inbox-equivalent status that can be triaged later

#### Scenario: Invalid decomposition fields are normalized before persistence
- **WHEN** the Bridge returns optional or non-canonical fields such as an unsupported priority value
- **THEN** the system MUST normalize those fields to repository-safe values before creating child tasks
- **AND** the stored child tasks MUST NOT contain values that violate the task persistence contract

### Requirement: Decomposition avoids duplicate or partial child creation
The system MUST guard against duplicate decomposition and MUST treat decomposition persistence as an all-or-nothing operation.

#### Scenario: Parent task already has child tasks
- **WHEN** a caller requests decomposition for a parent task that already has one or more child tasks
- **THEN** the system MUST reject the request as a conflict
- **AND** the system MUST NOT append another batch of subtasks

#### Scenario: Bridge or persistence fails mid-request
- **WHEN** the Bridge request fails, returns invalid structured output, or any child task creation step fails
- **THEN** the system MUST fail the decomposition request
- **AND** the system MUST leave the parent task without a partially created decomposition batch

### Requirement: IM command can trigger task decomposition
The system MUST expose task decomposition through the IM command surface as `/task decompose <id>`, using the Go API as the single business entrypoint.

#### Scenario: IM command succeeds
- **WHEN** a user invokes `/task decompose <id>` for a decomposable task
- **THEN** the IM bridge MUST call the Go task decomposition API rather than calling the TypeScript Bridge directly
- **AND** the user MUST receive a result message that includes the decomposition summary and created subtasks

#### Scenario: IM command fails
- **WHEN** the decomposition request is rejected or the backend returns an error
- **THEN** the IM bridge MUST return a user-facing failure message
- **AND** the message MUST explain that no new subtasks were created

### Requirement: Task decomposition runs through a real Bridge text-generation provider
The system MUST execute task decomposition through a Bridge provider that supports `text_generation`, instead of returning a simulated decomposition payload.

#### Scenario: Decomposition succeeds through the resolved provider
- **WHEN** the backend requests decomposition for an eligible task and the Bridge resolves a configured text-generation provider
- **THEN** the Bridge SHALL call a real provider runtime for that request
- **THEN** the backend SHALL receive a structured decomposition result that can be validated and persisted as child tasks

#### Scenario: Decomposition provider cannot serve the request
- **WHEN** the Bridge resolves task decomposition to a provider that is unknown, misconfigured, or does not support `text_generation`
- **THEN** the decomposition request MUST fail
- **THEN** the backend MUST NOT create any child tasks for that parent task

### Requirement: Task decomposition honors provider and model resolution rules
The system MUST apply the Bridge provider registry defaults and validation rules consistently for task decomposition requests, regardless of whether the caller supplied explicit provider metadata.

#### Scenario: Decomposition request uses Bridge defaults
- **WHEN** the backend submits a decomposition request without explicit provider or model values
- **THEN** the Bridge SHALL use the default `text_generation` provider and model configured for task decomposition
- **THEN** the returned result SHALL identify a truthful success or failure from that resolved provider path

#### Scenario: Provider output is structurally invalid
- **WHEN** the resolved provider returns output that does not satisfy the decomposition schema
- **THEN** the Bridge MUST reject the result as invalid instead of fabricating a substitute response
- **THEN** the backend MUST preserve the existing all-or-nothing decomposition persistence behavior

