## ADDED Requirements

### Requirement: Settings page includes a non-project-scoped Appearance section
The system SHALL render an Appearance card at the top of the settings page that is always visible regardless of whether a project is selected. The Appearance section SHALL contain a theme mode selector and a language selector, and SHALL NOT require a project context to function.

#### Scenario: User opens settings with no project selected
- **WHEN** a user navigates to the settings page without a project selected
- **THEN** the Appearance section is visible and fully functional
- **THEN** the project-specific settings sections are not rendered or show the no-project placeholder

#### Scenario: User opens settings with a project selected
- **WHEN** a user navigates to the settings page with a project selected
- **THEN** the Appearance section appears above all project-scoped setting cards
- **THEN** all existing project-scoped cards (General, Repository, Coding Agent, Budget, Review Policy, Webhook, Diagnostics, Custom Fields, Forms, Automations) remain present and unmodified
