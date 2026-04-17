# sprint-control-plane-interfaces Specification

## Purpose
Define the canonical project-scoped sprint control-plane interfaces for filtered sprint queries, current sprint resolution, activation invariants, and scoped sprint detail reads consumed by web and IM surfaces.

## Requirements
### Requirement: Project-scoped sprint queries support explicit filters and current sprint lookup
The system SHALL expose canonical project-scoped sprint read interfaces that honor supported list filters and resolve the current sprint without depending on list ordering. The canonical current sprint for a project SHALL be the single sprint in `active` status for that project.

#### Scenario: Filtered sprint list returns only matching statuses
- **WHEN** an authenticated operator requests `GET /api/v1/projects/:pid/sprints?status=active`
- **THEN** the system returns only sprint records in `active` status for project `:pid`
- **AND** planning or closed sprints for that project are omitted from the response

#### Scenario: Current sprint lookup returns the active sprint
- **WHEN** an authenticated operator requests `GET /api/v1/projects/:pid/sprints/current` and project `:pid` has exactly one active sprint
- **THEN** the system returns that active sprint as the canonical current sprint
- **AND** the response does not depend on the ordering of the broader sprint list

#### Scenario: Current sprint lookup has no active sprint
- **WHEN** an authenticated operator requests `GET /api/v1/projects/:pid/sprints/current` and project `:pid` has no active sprint
- **THEN** the system returns a no-current-sprint outcome instead of guessing from planning or closed sprints
- **AND** the response remains machine-readable so web and IM clients can show a truthful empty-current state

### Requirement: Sprint activation preserves a single active sprint per project
The system SHALL prevent sprint lifecycle mutations from creating multiple active sprints in the same project. A sprint transition to `active` MUST succeed only when no different sprint is already active in that project.

#### Scenario: Planning sprint becomes the current sprint
- **WHEN** an operator updates a planning sprint to `active` and no other sprint in the project is active
- **THEN** the system persists the transition successfully
- **AND** subsequent current-sprint lookup returns that sprint

#### Scenario: Activation conflicts with an existing active sprint
- **WHEN** an operator updates sprint `A` to `active` while different sprint `B` is already active in the same project
- **THEN** the system rejects the mutation with a conflict outcome
- **AND** the response identifies the already-active sprint instead of silently choosing one active sprint arbitrarily

### Requirement: Sprint detail routes are project-scoped and access-checked
The system SHALL expose sprint metrics, burndown, and budget detail through project-scoped routes that verify the requested sprint belongs to the active project scope before returning sprint data.

#### Scenario: Project-scoped sprint detail succeeds for in-scope sprint
- **WHEN** an authenticated operator requests sprint metrics, burndown, or budget detail for sprint `:sid` under `GET /api/v1/projects/:pid/sprints/:sid/...` and the sprint belongs to project `:pid`
- **THEN** the system returns the requested sprint detail data
- **AND** all sprint detail surfaces for that sprint resolve through the same project-scoped contract

#### Scenario: Project-scoped sprint detail rejects out-of-scope sprint ids
- **WHEN** an authenticated operator requests sprint metrics, burndown, or budget detail for sprint `:sid` under project `:pid` but the sprint belongs to a different project or no longer exists
- **THEN** the system returns an out-of-scope or not-found outcome
- **AND** no sprint detail from another project is disclosed
