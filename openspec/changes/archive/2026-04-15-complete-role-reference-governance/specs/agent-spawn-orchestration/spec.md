## ADDED Requirements

### Requirement: Spawn preflight rejects stale effective role bindings before admission
The spawn and dispatch entrypoints SHALL validate the effective role binding before queue admission or runtime startup, regardless of whether that binding came from an explicit request `roleId` or from the selected member's saved agent profile. If the effective role binding no longer resolves from the authoritative role registry, the system MUST return actionable preflight feedback and MUST NOT enqueue or start the run.

#### Scenario: Explicit role selection references a deleted role
- **WHEN** an operator submits a spawn request with an explicit `roleId` that no longer exists in the current role registry
- **THEN** the system rejects the request before runtime startup
- **THEN** no queue entry or agent run is created for that request
- **THEN** the response explains that the selected role binding is stale or missing

#### Scenario: Member-derived role binding is stale
- **WHEN** a spawn or dispatch flow resolves its effective role from the target member's saved agent profile and that bound `roleId` no longer exists in the current role registry
- **THEN** the system rejects the request before queue admission or runtime startup
- **THEN** the response identifies the member role binding as the failing preflight dependency
- **THEN** the operator is directed to repair the member's bound role instead of receiving a late runtime failure
