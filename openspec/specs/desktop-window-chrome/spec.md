# desktop-window-chrome Specification

## Purpose
Define the shared frameless desktop window chrome contract for the AgentForge main Tauri window, including custom titlebar rendering, drag-safe regions, and synchronized desktop control state.
## Requirements
### Requirement: Desktop main window supports a shared frameless chrome
The system SHALL support running the AgentForge main Tauri window without native decorations in desktop mode and SHALL render one shared custom window chrome surface for that main window. The shared chrome MUST remain available across authenticated and unauthenticated route shells in the main desktop window, and it MUST provide a stable title area, drag-safe region, and desktop-only control cluster without requiring each page to implement its own frame.

#### Scenario: Main desktop window renders the shared custom chrome
- **WHEN** a desktop session loads a route in the AgentForge main Tauri window after frameless support is enabled
- **THEN** the main window runs without native decorations
- **AND** the application renders one shared custom titlebar before page content
- **AND** both authenticated and unauthenticated route shells remain operable without page-specific frame patches

### Requirement: Drag regions and interactive zones stay predictable
The frameless window chrome SHALL define explicit drag regions and explicit non-drag interaction zones. Interactive controls such as buttons, popover triggers, dropdown triggers, inputs, search fields, and notification affordances MUST remain clickable and MUST NOT be interpreted as drag handles.

#### Scenario: Operator drags the frameless window from the title area
- **WHEN** the operator presses and drags inside the titlebar's documented drag region in desktop mode
- **THEN** the main window moves using the native drag behavior
- **AND** the gesture does not require page-specific JavaScript drag code

#### Scenario: Interactive titlebar controls do not become drag handles
- **WHEN** the operator clicks a documented non-drag control inside the custom titlebar
- **THEN** the intended button, menu, or input interaction executes
- **AND** the window does not start dragging instead of honoring that interaction

### Requirement: Frameless chrome keeps control state synchronized
The frameless window chrome SHALL keep its desktop control affordances synchronized with the current main-window state. Minimize, maximize or restore, and close affordances MUST reflect the latest known window state even when that state changes through shell actions, drag-region double click, or other native window gestures. Outside desktop mode, the application MUST NOT pretend that desktop-only frame controls are available.

#### Scenario: Maximized window updates the custom chrome state
- **WHEN** the main window becomes maximized through the custom titlebar or a native window gesture
- **THEN** the shared chrome updates its visible control state to the maximized variant
- **AND** a subsequent restore action is presented through the same shared chrome surface

#### Scenario: Web mode keeps the existing non-desktop frame behavior
- **WHEN** the same application route is rendered outside the Tauri desktop shell
- **THEN** the shared desktop-only frame controls are hidden or reported as not applicable
- **AND** the existing web layout remains usable without raw desktop APIs
