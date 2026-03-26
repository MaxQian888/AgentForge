## ADDED Requirements

### Requirement: Form definition CRUD
The system SHALL allow project admins to create, update, and delete intake forms per project.

#### Scenario: Create a bug report form
- **WHEN** admin creates a form with name "Bug Report", fields mapped to title, description, priority (custom field), and severity (custom field), with target_status="todo"
- **THEN** the system creates the form definition with a unique slug

#### Scenario: Update form fields
- **WHEN** admin adds a new field mapping to an existing form
- **THEN** the form definition is updated and new submissions include the field

#### Scenario: Delete form
- **WHEN** admin deletes a form
- **THEN** the form is soft-deleted; existing submissions and their tasks are not affected

### Requirement: Form submission creates task
The system SHALL create a task when a form is submitted, with field values mapped to task properties and custom fields.

#### Scenario: Submit form creates task in backlog
- **WHEN** user submits a bug report form with title "Login broken" and priority "P0"
- **THEN** the system creates a task with title="Login broken", status=todo, custom field Priority="P0", and origin="form"

#### Scenario: Form pre-maps assignee and status
- **WHEN** a form is configured with target_assignee and target_status
- **THEN** the created task has the specified assignee and status

### Requirement: Public and private form links
The system SHALL support public form URLs (no auth required) and private form URLs (requires project membership).

#### Scenario: Public form accessible without login
- **WHEN** an unauthenticated user visits a public form URL `/forms/:slug`
- **THEN** the system renders the form and accepts submissions

#### Scenario: Private form requires authentication
- **WHEN** an unauthenticated user visits a private form URL
- **THEN** the system redirects to login

### Requirement: Form rate limiting
The system SHALL rate-limit form submissions to prevent abuse.

#### Scenario: Rate limit exceeded on public form
- **WHEN** more than 10 submissions are received from the same IP within 1 minute on a public form
- **THEN** the system returns 429 Too Many Requests

### Requirement: Form API
The system SHALL expose REST endpoints for form operations.

#### Scenario: List forms via API
- **WHEN** client sends `GET /api/v1/projects/:pid/forms`
- **THEN** the system returns all form definitions for the project

#### Scenario: Submit form via API
- **WHEN** client sends `POST /api/v1/forms/:slug/submit` with field values
- **THEN** the system creates a task and returns the task ID with 201
