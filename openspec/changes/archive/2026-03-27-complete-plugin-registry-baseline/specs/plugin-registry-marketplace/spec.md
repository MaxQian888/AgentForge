## ADDED Requirements

### Requirement: Remote registry marketplace entries are served through the Go control plane
The system SHALL expose remote AgentForge Plugin Registry entries through the Go plugin control plane instead of requiring the frontend to query registry infrastructure directly. The control plane MUST provide search or list results for configured remote registries using a stable operator-facing response that can be rendered alongside existing local catalog and installed plugin views.

#### Scenario: Configured remote registry returns browse results
- **WHEN** an operator opens or searches the remote registry marketplace and a configured registry is reachable
- **THEN** the Go control plane returns normalized remote marketplace entries for that registry
- **THEN** the frontend can render those entries without issuing direct browser requests to the registry service

#### Scenario: Unconfigured remote registry stays explicit
- **WHEN** no remote registry client or registry source is configured for the current environment
- **THEN** the control plane returns a stable unavailable state for the remote marketplace capability
- **THEN** the operator-facing surface does not fail with an opaque internal configuration error

### Requirement: Remote marketplace metadata is normalized for operator decisions
The system SHALL normalize remote marketplace entries into an operator-facing metadata shape that includes plugin identifier, name, version, kind, description, source or registry identity, installability, and trust or verification hints when available. The response MUST also preserve remote-source reachability or availability state so operators can distinguish an empty marketplace from a broken remote source.

#### Scenario: Reachable remote entry shows registry metadata
- **WHEN** a remote registry entry is returned successfully
- **THEN** the entry includes the registry source identity and version metadata needed for the operator to decide whether to install it
- **THEN** the entry identifies whether it is installable, already installed, or blocked by current trust or source policy

#### Scenario: Registry outage remains distinguishable from an empty result set
- **WHEN** the configured remote registry cannot be reached or returns an invalid response
- **THEN** the control plane reports a remote-source availability failure distinct from a valid zero-results search
- **THEN** operator surfaces can present the registry as unavailable without mislabeling it as simply empty

### Requirement: Remote marketplace installation is explicit and side-effect free until confirmation
The system SHALL treat remote marketplace browsing as a read-only operation. A remote marketplace entry MUST NOT create or update an installed plugin record until the operator explicitly requests remote installation through the control plane.

#### Scenario: Browsing remote entries does not mutate installed state
- **WHEN** an operator lists or searches remote marketplace entries without choosing install
- **THEN** the platform returns browse metadata only
- **THEN** no installed plugin record, lifecycle state, or instance state is created as a side effect

#### Scenario: Explicit remote install promotes the entry into the registry
- **WHEN** an operator chooses a remote marketplace entry to install
- **THEN** the control plane downloads the referenced artifact through the configured remote registry client
- **THEN** the plugin only appears in the installed registry view after the install and verification flow succeeds

### Requirement: Remote marketplace install failures are classified for operators
The system SHALL classify remote marketplace install failures into stable operator-facing categories that distinguish remote-source reachability, download failure, invalid artifact or manifest, trust verification failure, and approval or policy blocking. These failures MUST leave existing installed plugin state unchanged.

#### Scenario: Remote download fails before verification
- **WHEN** a remote marketplace installation cannot download the requested artifact from the registry
- **THEN** the control plane returns a remote download failure category and preserves the related source context
- **THEN** no partial installed plugin record is created from the failed download attempt

#### Scenario: Trust gate blocks a downloaded remote artifact
- **WHEN** the remote artifact downloads successfully but fails digest, signature, approval, or policy checks
- **THEN** the control plane returns the corresponding verification or approval failure category
- **THEN** the operator can distinguish trust blocking from transport failure without inspecting internal logs