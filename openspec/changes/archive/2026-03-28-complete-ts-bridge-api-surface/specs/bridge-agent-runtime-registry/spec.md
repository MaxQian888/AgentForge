## ADDED Requirements

### Requirement: Runtime catalog is queryable from Go API layer
Go backend SHALL expose `GET /api/v1/bridge/runtimes` endpoint that returns the Bridge runtime catalog. The endpoint SHALL cache the catalog for 60 seconds to avoid excessive Bridge calls.

#### Scenario: Cached catalog returned
- **WHEN** catalog was fetched 30 seconds ago and client requests again
- **THEN** cached catalog is returned without calling Bridge

#### Scenario: Cache expired
- **WHEN** catalog cache is older than 60 seconds
- **THEN** Go backend calls Bridge `/bridge/runtimes` and refreshes cache

### Requirement: Frontend uses runtime catalog for agent spawn configuration
Frontend agent store SHALL fetch and cache the runtime catalog from `GET /api/v1/bridge/runtimes`. The `RuntimeSelector` component SHALL use this data to populate runtime, provider, and model dropdowns.

#### Scenario: Agent store loads catalog on first access
- **WHEN** `RuntimeSelector` renders and catalog is not yet loaded
- **THEN** agent store fetches catalog from API and populates the selector options

#### Scenario: Catalog shows runtime diagnostics
- **WHEN** a runtime has `available: false` with diagnostics `["API key not configured"]`
- **THEN** RuntimeSelector shows runtime as disabled with tooltip showing the diagnostic messages
