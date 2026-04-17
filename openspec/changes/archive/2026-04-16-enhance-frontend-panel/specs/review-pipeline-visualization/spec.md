## ADDED Requirements

### Requirement: Review pipeline displays visual queue

The system SHALL display reviews in a kanban-style pipeline with columns for each review status (pending, in_review, approved, rejected, blocked).

#### Scenario: User views review pipeline
- **WHEN** user navigates to the review pipeline page
- **THEN** system displays review cards organized by status in columns
- **AND** each column shows count of reviews in that status

#### Scenario: Pipeline is empty
- **WHEN** no reviews exist in the system
- **THEN** system displays empty state with "Create your first review" call-to-action

### Requirement: Review cards show review metadata

The system SHALL display review title, risk level, assignee, target branch, and age on each review card.

#### Scenario: User views review card
- **WHEN** review card is displayed in the pipeline
- **THEN** card shows review title, risk badge (critical/high/medium/low), assignee avatar, target branch, and time since creation
- **AND** risk badge uses appropriate color coding (red for critical, yellow for high, etc.)

#### Scenario: Review has no assignee
- **WHEN** review is unassigned
- **THEN** card shows "Unassigned" indicator
- **AND** card is visually distinct to attract attention

### Requirement: Review pipeline supports status transitions

The system SHALL allow users to transition reviews between statuses with appropriate permissions and logging.

#### Scenario: User approves review
- **WHEN** user clicks "Approve" action on a review in "in_review" status
- **THEN** system transitions review to "approved" status
- **AND** review card moves to approved column with animation

#### Scenario: User blocks review
- **WHEN** user clicks "Block" action and provides reason
- **THEN** system transitions review to "blocked" status
- **AND** blocking reason is displayed on the review card

#### Scenario: Invalid status transition
- **WHEN** user attempts transition not allowed by workflow
- **THEN** system displays error message explaining allowed transitions
- **AND** review status remains unchanged

### Requirement: Review pipeline enables bulk actions

The system SHALL allow users to select multiple reviews and perform bulk assign, approve, or reject operations.

#### Scenario: User bulk assigns reviews
- **WHEN** user selects multiple reviews and chooses "Assign To" action
- **THEN** system opens assignee selection dialog
- **AND** confirming assigns all selected reviews to chosen assignee

#### Scenario: User bulk approves reviews
- **WHEN** user selects multiple reviews and clicks "Approve All"
- **THEN** system displays confirmation dialog with review count
- **AND** confirming approves all selected reviews that are in valid status

### Requirement: Review pipeline provides filtering and search

The system SHALL allow users to filter reviews by assignee, risk level, target branch, and age, and search by title.

#### Scenario: User filters by high risk
- **WHEN** user selects "High Risk" from risk filter
- **THEN** system displays only reviews with high or critical risk level
- **AND** filter badge shows active filter count

#### Scenario: User searches reviews
- **WHEN** user types search query in the search box
- **THEN** system filters reviews to those matching query in title or description
- **AND** search updates in real-time as user types

### Requirement: Review pipeline shows review details panel

The system SHALL display a slide-out panel with full review details when a review card is clicked.

#### Scenario: User views review details
- **WHEN** user clicks on a review card
- **THEN** system opens panel showing review description, code changes, comments, and status history
- **AND** panel includes action buttons for available transitions

#### Scenario: User views review history
- **WHEN** user clicks "History" tab in review details
- **THEN** system displays timeline of all status changes with timestamps and actors

### Requirement: Review pipeline displays metrics summary

The system SHALL show summary metrics at the top of the pipeline (total reviews, by status, average age).

#### Scenario: User views pipeline metrics
- **WHEN** review pipeline loads
- **THEN** system displays metric cards showing total reviews, breakdown by status, and average review age
- **AND** metrics update when filters are applied

### Requirement: Review pipeline supports assignment workflow

The system SHALL allow users to claim unassigned reviews or assign reviews to specific users.

#### Scenario: User claims review
- **WHEN** user clicks "Claim" action on an unassigned review
- **THEN** system assigns review to current user
- **AND** review card updates to show user as assignee

#### Scenario: User assigns to team member
- **WHEN** user clicks "Assign" and selects a team member
- **THEN** system assigns review to selected team member
- **AND** assignee receives notification
