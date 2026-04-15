## MODIFIED Requirements

### Requirement: Supported action types
The system SHALL support the following action types: update_field, assign_user, send_notification, move_to_column, create_subtask, send_im_message, invoke_plugin, and start_workflow. The `start_workflow` action MUST start workflow execution through the canonical workflow runtime surface, and `invoke_plugin` MUST remain a generic plugin invocation path instead of the required execution path for workflow plugins.

#### Scenario: Update field action
- **WHEN** a rule's action is update_field with field="priority" and value="P0"
- **THEN** the system updates the task's priority field to P0

#### Scenario: Send IM message action
- **WHEN** a rule's action is send_im_message with a channel and template
- **THEN** the system sends the rendered message to the configured IM channel

#### Scenario: Invoke plugin action
- **WHEN** a rule's action is invoke_plugin with an integration plugin ID and input
- **THEN** the system triggers the declared plugin operation with the specified input

#### Scenario: Start workflow action
- **WHEN** a rule's action is start_workflow with a workflow plugin ID and optional trigger payload
- **THEN** the system starts workflow execution through the canonical workflow runtime surface instead of generic plugin invocation
- **THEN** the resulting workflow run remains visible through the normal workflow run query surfaces

### Requirement: Automation activity log
The system SHALL log every automation rule evaluation with trigger details, result, and any errors. When an action starts or attempts to start a workflow, the automation log detail MUST include machine-readable action-level outcome data, including the action type, workflow plugin identity, verdict, reason metadata when present, and the started workflow run reference when a run is created.

#### Scenario: Successful execution logged
- **WHEN** an automation rule evaluates and all actions succeed
- **THEN** the system creates an automation_log entry with status=success, the rule ID, task ID, and event details

#### Scenario: Failed execution logged with error
- **WHEN** an automation action fails (e.g., IM send fails)
- **THEN** the system creates an automation_log entry with status=failed and the error detail

#### Scenario: Workflow-start verdict is logged structurally
- **WHEN** an automation rule executes a start_workflow action
- **THEN** the automation_log detail includes a structured action outcome that identifies whether workflow execution was started, blocked, or failed
- **THEN** downstream consumers do not need to infer workflow lineage from free-form text alone

#### Scenario: View automation log
- **WHEN** admin opens the automation activity log
- **THEN** the system displays a searchable, filterable list of all automation evaluations with timestamps, rule names, statuses, and affected tasks

## ADDED Requirements

### Requirement: Automation-triggered workflow starts use canonical workflow runtime truth
The system SHALL allow automation rules to start workflow runs by declaring a `start_workflow` action that targets a workflow plugin. A successful automation-triggered workflow start MUST carry canonical automation lineage into the workflow trigger payload, including the automation event type, rule identity, project identity, and available task identity, and it MUST reuse the same persistence, dependency validation, and run visibility contract as other workflow runtime starts.

#### Scenario: Due-date automation starts a canonical workflow run
- **WHEN** a `task.due_date_approaching` rule matches and its action declares `start_workflow` for an enabled workflow plugin
- **THEN** the platform creates a canonical workflow run for that plugin
- **THEN** the created workflow run trigger payload includes the originating automation event, rule identity, project identity, and task identity

#### Scenario: Invalid workflow target fails explicitly
- **WHEN** a start_workflow action omits the required workflow plugin reference or targets an unavailable or non-workflow plugin
- **THEN** the action fails before any workflow step begins
- **THEN** the automation log records a structured failure reason instead of reporting a generic success

### Requirement: Automation-triggered workflow starts preserve duplicate truth
The system SHALL detect equivalent active workflow runs before creating a second automation-triggered workflow run for the same workflow plugin and automation event scope. If an equivalent active run already exists, the platform MUST return a structured blocked verdict and MUST NOT create a second active workflow run for that same automation-triggered scope.

#### Scenario: Duplicate task-scoped workflow start is blocked
- **WHEN** a start_workflow action is evaluated for a task-scoped automation event and an active workflow run already exists for the same workflow plugin, rule, task, and event scope
- **THEN** the platform returns a blocked verdict before creating a new run
- **THEN** the automation log records a machine-readable duplicate reason

#### Scenario: Non-duplicate workflow start proceeds
- **WHEN** no equivalent active workflow run exists for the evaluated automation event scope
- **THEN** the platform proceeds to start a new workflow run through the canonical workflow runtime
- **THEN** the structured action outcome records the started run identity

### Requirement: Workflow-start automation rules are validated before persistence
The system SHALL reject persisted automation rules whose start_workflow action config is missing required fields or is malformed for the canonical workflow runtime contract. At minimum, the persisted config MUST identify the target workflow plugin, and the API MUST return a validation error instead of storing a rule that can only fail later as a silent no-op.

#### Scenario: Missing workflow plugin ID is rejected at save time
- **WHEN** a client submits an automation rule whose start_workflow action omits the required workflow plugin identifier
- **THEN** the automation rule API rejects the request with a validation error
- **THEN** the invalid rule is not persisted

#### Scenario: Valid workflow-start rule is persisted
- **WHEN** a client submits an automation rule whose start_workflow action includes the required workflow plugin identifier and valid config shape
- **THEN** the automation rule is persisted successfully
- **THEN** later rule evaluation can use that config without relying on implied defaults
