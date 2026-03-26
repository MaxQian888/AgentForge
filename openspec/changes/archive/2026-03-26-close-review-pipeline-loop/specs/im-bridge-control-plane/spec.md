## ADDED Requirements

### Requirement: IM /review command supports deep, approve, and request-changes subcommands
The system SHALL extend the IM `/review` command handler to accept three additional subcommands: `deep <pr-url>`, `approve <review-id>`, and `request-changes <review-id> [comment]`. Each subcommand SHALL call the corresponding backend API and reply with a structured result card. The existing `/review <pr-url>` and `/review status <id>` commands SHALL remain unchanged.

#### Scenario: /review deep <pr-url> creates a standalone deep review
- **WHEN** an IM user sends `/review deep <pr-url>`
- **THEN** the bridge calls the standalone deep review creation API
- **THEN** the bridge replies with a card showing review ID, initial pending status, and a "View Review" link

#### Scenario: /review approve <review-id> approves a pending_human review
- **WHEN** an IM user sends `/review approve <review-id>`
- **THEN** the bridge calls `ApproveReview` for the specified review ID with the IM user's identity as the actor
- **THEN** the bridge replies with a confirmation card showing the updated review status

#### Scenario: /review approve on a non-pending_human review returns an error card
- **WHEN** an IM user sends `/review approve <review-id>` for a review not in `pending_human` state
- **THEN** the bridge receives a backend error and replies with an error card describing the invalid transition

#### Scenario: /review request-changes <review-id> <comment> records a changes request
- **WHEN** an IM user sends `/review request-changes <review-id> <comment>`
- **THEN** the bridge calls `RequestChangesReview` with the review ID and the supplied comment
- **THEN** the bridge replies with a confirmation card showing the review ID and new state

### Requirement: Review result cards include approve and request-changes action buttons
The system SHALL include inline action buttons on review result cards delivered to IM platforms when the review is in `pending_human` state. The card SHALL contain at minimum an "Approve" button and a "Request Changes" button that trigger the corresponding `/review approve` and `/review request-changes` flows via the existing IM action execution infrastructure.

#### Scenario: pending_human review card includes action buttons
- **WHEN** the bridge delivers a review card for a review in `pending_human` state
- **THEN** the card includes interactive "Approve" and "Request Changes" buttons
- **THEN** pressing "Approve" triggers the approve flow for the review ID embedded in the card
- **THEN** pressing "Request Changes" prompts the user for a comment and triggers the request-changes flow

#### Scenario: Completed review card does not include action buttons
- **WHEN** the bridge delivers a review card for a review in a terminal completed state
- **THEN** the card does not include Approve or Request Changes buttons
- **THEN** the card may include a "View Details" link only
