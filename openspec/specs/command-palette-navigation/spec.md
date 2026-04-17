# command-palette-navigation Specification

## Purpose
Global Cmd+K command palette for quick navigation and actions across the application. Supports fuzzy search over pages, actions, and settings with keyboard-first navigation, recent items, categories, and context-aware commands.

## Requirements

### Requirement: Command palette provides global quick access

The system SHALL provide a command palette accessible via ⌘K (Ctrl+K on Windows) keyboard shortcut for quick navigation and actions.

#### Scenario: User opens command palette
- **WHEN** user presses ⌘K (or Ctrl+K)
- **THEN** system opens command palette modal centered on screen
- **AND** input field is automatically focused

#### Scenario: User closes command palette
- **WHEN** user presses Escape while palette is open
- **THEN** system closes the palette
- **AND** focus returns to previous element

#### Scenario: User clicks outside palette
- **WHEN** user clicks outside the command palette modal
- **THEN** system closes the palette
- **AND** no action is performed

### Requirement: Command palette supports navigation commands

The system SHALL allow users to quickly navigate to any page in the application via the command palette.

#### Scenario: User searches for page
- **WHEN** user types "agent" in the command palette
- **THEN** system displays matching pages (Agents, Agent Settings, etc.)
- **AND** results update as user types

#### Scenario: User navigates to page
- **WHEN** user presses Enter on a navigation result
- **THEN** system navigates to the selected page
- **AND** command palette closes

#### Scenario: User navigates with arrow keys
- **WHEN** user presses arrow down/up in command palette
- **THEN** selection moves through results
- **AND** current selection is highlighted

### Requirement: Command palette supports action commands

The system SHALL allow users to perform common actions directly from the command palette.

#### Scenario: User creates new task
- **WHEN** user types "new task" and presses Enter
- **THEN** system opens task creation dialog
- **AND** command palette closes

#### Scenario: User spawns new agent
- **WHEN** user types "spawn agent" and presses Enter
- **THEN** system opens agent spawn dialog
- **AND** command palette closes

#### Scenario: User searches globally
- **WHEN** user types "search <query>" and presses Enter
- **THEN** system performs global search for the query
- **AND** navigates to search results page

### Requirement: Command palette displays recent items

The system SHALL show recently visited pages and recent actions in the command palette when empty.

#### Scenario: User opens empty palette
- **WHEN** user opens command palette without typing
- **THEN** system displays list of recently visited pages
- **AND** shows up to 5 recent items

#### Scenario: User selects recent item
- **WHEN** user clicks on a recent page in the palette
- **THEN** system navigates to that page
- **AND** item moves to top of recent list

### Requirement: Command palette groups commands by category

The system SHALL organize commands into categories (Navigation, Actions, Settings, etc.) for easier discovery.

#### Scenario: User views command categories
- **WHEN** command palette displays results
- **THEN** results are grouped under category headers (Navigation, Actions, etc.)
- **AND** each category shows relevant commands

#### Scenario: User filters by category
- **WHEN** user types category prefix (e.g., ">" for commands, "@" for users)
- **THEN** system filters to only show commands in that category
- **AND** category indicator is shown in input

### Requirement: Command palette shows keyboard shortcut hints

The system SHALL display keyboard shortcuts for commands that have them.

#### Scenario: User views commands with shortcuts
- **WHEN** command palette displays results
- **THEN** each command shows its keyboard shortcut on the right side if available
- **AND** shortcuts are displayed in platform-appropriate format (⌘ vs Ctrl)

#### Scenario: User executes via shortcut from palette
- **WHEN** command is selected and user presses displayed shortcut
- **THEN** command is executed immediately
- **AND** palette closes

### Requirement: Command palette supports fuzzy search

The system SHALL match commands even when search query contains typos or partial matches.

#### Scenario: User searches with typo
- **WHEN** user types "agnets" (typo of "agents")
- **THEN** system still shows "Agents" page in results
- **AND** ranks exact matches higher than fuzzy matches

#### Scenario: User searches with partial word
- **WHEN** user types "set"
- **THEN** system shows Settings, Setup, and any commands containing "set"
- **AND** results are ranked by relevance

### Requirement: Command palette enables quick settings access

The system SHALL provide shortcuts to common settings pages.

#### Scenario: User accesses theme settings
- **WHEN** user types "theme" in command palette
- **THEN** system shows "Toggle Theme" action and "Theme Settings" page
- **AND** selecting toggle immediately switches theme

#### Scenario: User accesses project settings
- **WHEN** user types "project settings"
- **THEN** system shows "Project Settings" navigation result
- **AND** selecting navigates to settings with current project context

### Requirement: Command palette shows contextual commands

The system SHALL display context-aware commands based on current page and user permissions.

#### Scenario: User on task page
- **WHEN** user opens palette while viewing task detail
- **THEN** palette includes task-specific actions (Edit Task, Add Comment, etc.)
- **AND** actions are relevant to current context

#### Scenario: User lacks permission
- **WHEN** user searches for admin-only command
- **THEN** command is not shown in results
- **AND** no error is displayed (graceful hiding)

### Requirement: Command palette supports command history

The system SHALL remember and suggest previously executed commands.

#### Scenario: User views command history
- **WHEN** user opens palette and scrolls past recent items
- **THEN** system shows "Recent Commands" section
- **AND** commands are listed in reverse chronological order

#### Scenario: User reruns command
- **WHEN** user selects a command from history
- **THEN** system executes the command
- **AND** command moves to top of history
