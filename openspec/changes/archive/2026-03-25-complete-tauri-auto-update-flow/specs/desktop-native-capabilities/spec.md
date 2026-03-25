## MODIFIED Requirements

### Requirement: Tray, global shortcut, and update checks degrade predictably
The platform SHALL define tray, global shortcut, and updater lifecycle capabilities as desktop-enhanced features with explicit non-desktop semantics. Web mode MUST NOT silently pretend these capabilities succeeded. Desktop update handling MUST surface normalized update metadata, progress, install outcome, and post-install restart requirements through the shared facade instead of reducing the flow to a single success or failure flag.

#### Scenario: Tray state updates on desktop
- **WHEN** the desktop runtime is ready and the frontend requests a tray status update
- **THEN** Tauri updates the tray state and returns a success acknowledgement

#### Scenario: Global shortcut is unavailable on web
- **WHEN** a web session requests global shortcut registration
- **THEN** the shared facade returns an explicit unsupported result

#### Scenario: Update lifecycle is not applicable on web
- **WHEN** a web session requests an update check or update installation
- **THEN** the shared facade returns not-applicable without triggering desktop-only updater APIs

#### Scenario: Desktop update metadata is returned before install starts
- **WHEN** a desktop session checks configured updater endpoints and a newer signed release is available
- **THEN** the shared facade returns the available version, release notes or body, and release date without starting download or installation implicitly

## ADDED Requirements

### Requirement: Desktop update installation exposes progress and restart handoff
The desktop platform facade SHALL support an explicit install flow for a discovered update. The install flow MUST expose normalized download or install progress, MUST report failure without terminating the current session, and MUST provide a restart handoff once installation succeeds. Frontend pages MUST NOT need to import raw Tauri updater or process APIs directly to complete this flow.

#### Scenario: Download and install completes successfully
- **WHEN** an operator confirms installation for an available desktop update
- **THEN** the shared facade reports started, progress, and finished states for the install session
- **AND** the install result transitions to a stable state that indicates the app is ready to relaunch into the new version

#### Scenario: Download or install fails
- **WHEN** the updater download or installation step fails
- **THEN** the shared facade returns a stable failed result with an error summary
- **AND** the current app session remains usable without falsely reporting the update as installed

#### Scenario: Restart is triggered after a successful install
- **WHEN** an installed update is waiting to be activated and the operator chooses restart now
- **THEN** the desktop shell relaunches the application through the supported process capability
- **AND** the page does not need to call raw platform APIs outside the shared facade
