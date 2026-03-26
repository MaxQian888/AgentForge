## ADDED Requirements

### Requirement: Automation rule definition
The system SHALL allow project admins to define event-condition-action automation rules per project.

#### Scenario: Create an automation rule
- **WHEN** admin creates a rule: "When status changes to done AND assignee is agent, then send IM notification"
- **THEN** the system creates an automation_rule with event_type=task.status_changed, conditions=[{field:"status",op:"eq",value:"done"},{field:"assignee_type",op:"eq",value:"agent"}], actions=[{type:"send_im_message",config:{...}}]

#### Scenario: Enable/disable rule
- **WHEN** admin toggles a rule's enabled flag to false
- **THEN** the rule stops evaluating on future events

### Requirement: Supported event types
The system SHALL evaluate automation rules on the following event types: task.status_changed, task.assignee_changed, task.due_date_approaching, task.field_changed, review.completed, budget.threshold_reached.

#### Scenario: Rule triggers on status change
- **WHEN** a task's status changes from "in_progress" to "done" and a rule exists for task.status_changed with matching conditions
- **THEN** the system evaluates the rule and executes its actions

#### Scenario: Rule triggers on due date approaching
- **WHEN** a task's due date is within 24 hours and a rule exists for task.due_date_approaching
- **THEN** the system evaluates the rule and executes its actions

### Requirement: Supported action types
The system SHALL support the following action types: update_field, assign_user, send_notification, move_to_column, create_subtask, send_im_message, invoke_plugin.

#### Scenario: Update field action
- **WHEN** a rule's action is update_field with field="priority" and value="P0"
- **THEN** the system updates the task's priority field to P0

#### Scenario: Send IM message action
- **WHEN** a rule's action is send_im_message with a channel and template
- **THEN** the system sends the rendered message to the configured IM channel

#### Scenario: Invoke plugin action
- **WHEN** a rule's action is invoke_plugin with a plugin ID and input
- **THEN** the system triggers the plugin execution with the specified input

### Requirement: Cascade prevention
The system SHALL prevent infinite automation cascades by skipping rule evaluation on events triggered by other automations.

#### Scenario: Automation-triggered event skips evaluation
- **WHEN** an automation action updates a task field, generating a task.field_changed event
- **THEN** the system marks the event as automation-triggered and skips automation evaluation for that event

### Requirement: Automation rule API
The system SHALL expose REST endpoints for automation rule CRUD.

#### Scenario: List rules via API
- **WHEN** client sends `GET /api/v1/projects/:pid/automations`
- **THEN** the system returns all automation rules for the project

#### Scenario: Create rule via API
- **WHEN** client sends `POST /api/v1/projects/:pid/automations` with event_type, conditions, actions, and name
- **THEN** the system creates the rule and returns 201

### Requirement: Automation activity log
The system SHALL log every automation rule evaluation with trigger details, result, and any errors.

#### Scenario: Successful execution logged
- **WHEN** an automation rule evaluates and all actions succeed
- **THEN** the system creates an automation_log entry with status=success, the rule ID, task ID, and event details

#### Scenario: Failed execution logged with error
- **WHEN** an automation action fails (e.g., IM send fails)
- **THEN** the system creates an automation_log entry with status=failed and the error detail

#### Scenario: View automation log
- **WHEN** admin opens the automation activity log
- **THEN** the system displays a searchable, filterable list of all automation evaluations with timestamps, rule names, statuses, and affected tasks
