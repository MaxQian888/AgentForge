## MODIFIED Requirements

### Requirement: Shell interaction result events are exposed to the frontend
The desktop event bridge SHALL emit normalized shell interaction result events for tray actions, native menu actions, window control actions, notification activations, and frameless window-state projections that must be observed by the shared desktop frame. These events MUST preserve `source`, `actionId`, status, timestamp, and any available route or entity context needed by frontend coordinators and tests. When the main window changes between restored, maximized, minimized, visible, or focused states through shell actions or native desktop gestures, the shared desktop subscription surface MUST expose a normalized state projection without requiring page-specific native listeners.

#### Scenario: Native shell action emits a result event
- **WHEN** the desktop shell processes a supported tray, menu, or window action
- **THEN** it emits a normalized shell action event that includes the action identifier, source, and result status
- **AND** frontend subscribers can observe that outcome without polling native logs

#### Scenario: Frameless chrome receives a window-state projection
- **WHEN** the main window changes state through the custom titlebar or a native desktop gesture such as maximizing or restoring from the drag region
- **THEN** the shared desktop subscription surface emits or refreshes a normalized window-state projection
- **AND** the shared frameless chrome can update its controls without wiring page-specific native listeners

#### Scenario: Notification activation emits a shell interaction result
- **WHEN** an operator activates a supported desktop notification
- **THEN** the desktop event bridge emits the corresponding shell action event with the related notification identifier and target context
- **AND** the event remains additive to the existing business notification truth
