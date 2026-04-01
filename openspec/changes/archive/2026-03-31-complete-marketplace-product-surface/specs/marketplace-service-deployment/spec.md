## ADDED Requirements

### Requirement: The marketplace service exposes a standalone runtime contract
The `src-marketplace` service SHALL run as a standalone Go microservice with its own port, health endpoint, CORS configuration, and artifact storage contract. Its default runtime contract MUST NOT conflict with the ports already reserved for the Go orchestrator, TS bridge, or IM bridge.

#### Scenario: Marketplace runs alongside the rest of the local stack
- **WHEN** the local or separated deployment stack starts the Go orchestrator, TS bridge, IM bridge, and marketplace service together
- **THEN** the marketplace service binds to its own configured port and health endpoint without colliding with the other services
- **THEN** operators can reach both the marketplace service and the existing bridge services concurrently

#### Scenario: Marketplace service reports health independently
- **WHEN** an operator or script probes the marketplace health endpoint
- **THEN** the service responds with a stable success or failure signal independent of the main application services
- **THEN** deployment or readiness checks can distinguish marketplace failures from other stack failures

### Requirement: Main application surfaces integrate with marketplace through explicit URL configuration
The Next.js frontend and the Go orchestrator SHALL target the marketplace service through explicit runtime configuration rather than assuming co-location or a hardcoded port. When the marketplace service is unavailable or unconfigured, consuming surfaces MUST report that state explicitly instead of silently degrading to empty data.

#### Scenario: Web deployment targets a separately deployed marketplace service
- **WHEN** the frontend is deployed separately from the marketplace service
- **THEN** the marketplace workspace uses the configured marketplace URL contract to reach the service
- **THEN** the workspace continues to function without requiring the marketplace service to be reverse-proxied through the main Go API

#### Scenario: Main Go backend bridges marketplace installs through configured service URL
- **WHEN** the Go orchestrator is configured with a marketplace service URL
- **THEN** it exposes the supported marketplace install and consumption bridge routes
- **THEN** those routes fail with a stable service-unavailable or misconfigured state when the marketplace service cannot be reached

### Requirement: Marketplace deployment preserves artifact persistence and origin controls
The standalone marketplace runtime SHALL support persistent artifact storage and explicit origin controls suitable for local, web, and desktop usage. The service MUST allow the operator to configure artifact storage location and allowed origins without broad wildcard defaults that undermine deployment clarity.

#### Scenario: Artifact storage survives service restart
- **WHEN** a marketplace item version is uploaded and the service restarts
- **THEN** the artifact remains available through the configured artifact storage directory
- **THEN** operators can still download or install the previously uploaded version without re-publishing it

#### Scenario: Allowed origins support both web and desktop entrypoints
- **WHEN** the marketplace service is configured for local web and desktop development
- **THEN** the allowed origin list can admit the supported web and Tauri origins explicitly
- **THEN** unsupported origins remain blocked instead of being implicitly trusted by an overly broad default
