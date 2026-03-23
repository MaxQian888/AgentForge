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
