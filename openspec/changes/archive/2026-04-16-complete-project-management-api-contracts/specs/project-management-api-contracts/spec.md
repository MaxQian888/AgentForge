## ADDED Requirements

### Requirement: Project entry APIs return truthful project management summary
The system SHALL return authoritative project-management summary fields from the project list and project detail APIs so project entry surfaces do not fabricate lifecycle state from client-side defaults. At minimum, the response contract MUST include the project identity fields already used by management workspaces together with truthful lifecycle status, task count, and agent count derived from persisted project state.

#### Scenario: Project list returns truthful summary fields
- **WHEN** the client requests the authenticated project list for the management workspace
- **THEN** each returned project includes authoritative `status`, `taskCount`, and `agentCount` values together with its identity and settings fields
- **AND** those values are derived from current backend state instead of being omitted and replaced by client defaults

#### Scenario: Project detail reuses the same summary contract
- **WHEN** the client requests a single project record for a project-management workspace
- **THEN** the returned project detail includes the same lifecycle summary fields required by project entry surfaces
- **AND** the detail response does not require the frontend to guess missing summary values from local fallback logic

### Requirement: Project-scoped management APIs require explicit project context
Any project-management API that reads or mutates project-owned workflow or template state SHALL resolve an explicit project context from the request contract and SHALL reject requests that omit that context or provide a context that cannot be validated. The system MUST NOT silently fall back to ambient browser selection, zero-value project IDs, or unscoped global behavior for project-owned resources.

#### Scenario: Request omits required project context
- **WHEN** a client calls a project-owned workflow or template management endpoint without the required explicit project context
- **THEN** the system rejects the request with a client-visible validation error
- **AND** the request does not continue with a zero-value or inferred project scope

#### Scenario: Request includes explicit project context
- **WHEN** a client calls a project-owned workflow or template management endpoint with a valid explicit project context
- **THEN** the system resolves that project as the authoritative scope for the request
- **AND** downstream handlers and services can validate ownership against that same resolved project scope

### Requirement: Project-owned workflow records enforce ownership boundaries
The system SHALL verify project ownership before reading, publishing, cloning, updating, deleting, or executing project-owned workflow definitions or user-owned workflow templates. Built-in and marketplace templates MAY be reused as global sources, but any resulting custom template, workflow definition, or execution MUST belong to the explicitly requested project.

#### Scenario: Workflow definition belongs to another project
- **WHEN** a client attempts to publish or mutate a workflow definition that belongs to a different project than the explicit request scope
- **THEN** the system rejects the request as an ownership mismatch
- **AND** no template, workflow definition, or execution is created or modified for that mismatched record

#### Scenario: Global template is reused into current project
- **WHEN** a client clones or executes a built-in or marketplace template within an explicit project scope
- **THEN** the source template remains immutable in its global source category
- **AND** the resulting custom workflow definition or execution is created in the explicitly requested project scope