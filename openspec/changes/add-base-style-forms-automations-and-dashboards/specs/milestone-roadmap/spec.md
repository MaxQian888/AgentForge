## ADDED Requirements

### Requirement: Milestone CRUD
The system SHALL support creating, updating, and deleting milestones within a project.

#### Scenario: Create a milestone
- **WHEN** user creates a milestone with name "v2.0 Release" and target_date "2026-06-01"
- **THEN** the system creates the milestone with status=planned

#### Scenario: Update milestone status
- **WHEN** user changes a milestone's status from planned to in_progress
- **THEN** the system updates the status and reflects it in all views

#### Scenario: Delete milestone
- **WHEN** user deletes a milestone
- **THEN** the milestone is soft-deleted; tasks and sprints linked to it retain the reference but display "Milestone deleted"

### Requirement: Milestone-task and milestone-sprint association
The system SHALL allow associating tasks and sprints with milestones.

#### Scenario: Assign task to milestone
- **WHEN** user sets a task's milestone to "v2.0 Release"
- **THEN** the task appears under that milestone in the roadmap view

#### Scenario: Assign sprint to milestone
- **WHEN** user associates a sprint with a milestone
- **THEN** the sprint's tasks aggregate into the milestone's completion metrics

### Requirement: Roadmap view
The system SHALL provide a roadmap view showing milestones as horizontal lanes with their associated sprints and release markers on a timeline.

#### Scenario: View roadmap
- **WHEN** user opens the roadmap view
- **THEN** the system displays milestones as horizontal lanes on a timeline axis, with sprints shown as blocks within their milestone lanes and release markers at milestone target dates

#### Scenario: Milestone completion percentage
- **WHEN** a milestone has 10 associated tasks and 7 are completed
- **THEN** the roadmap shows 70% completion for that milestone

### Requirement: Milestone API
The system SHALL expose REST endpoints for milestone operations.

#### Scenario: List milestones via API
- **WHEN** client sends `GET /api/v1/projects/:pid/milestones`
- **THEN** the system returns all milestones with status, target_date, and completion metrics

#### Scenario: Create milestone via API
- **WHEN** client sends `POST /api/v1/projects/:pid/milestones` with name and target_date
- **THEN** the system creates the milestone and returns 201
