## ADDED Requirements

### Requirement: Backend service callers SHALL reach TS Bridge only through Go-owned surfaces
All non-frontend backend service callers, including IM Bridge and operator-facing automation surfaces, SHALL access TS Bridge runtime inspection and lightweight AI capabilities only through Go backend-owned HTTP surfaces. They MUST NOT call TS Bridge service endpoints directly from IM Bridge or other backend helpers.

#### Scenario: IM Bridge queries runtime catalog through Go proxy
- **WHEN** an IM Bridge command needs the runtime catalog
- **THEN** it calls the Go backend `GET /api/v1/bridge/runtimes` endpoint
- **THEN** the Go backend proxies that request to the canonical TS Bridge `/bridge/runtimes` route
- **THEN** the returned payload preserves the upstream runtime catalog semantics without inventing a parallel contract

#### Scenario: IM Bridge requests AI classification through Go proxy
- **WHEN** an IM Bridge command or natural-language routing flow needs intent classification
- **THEN** it calls the Go backend `POST /api/v1/ai/classify-intent` endpoint
- **THEN** the Go backend forwards the request to the canonical TS Bridge `/bridge/classify-intent` route
- **THEN** the IM Bridge does not bypass the backend to call TS Bridge directly

### Requirement: Go proxy endpoints SHALL expose upstream connectivity failures truthfully
When a Go backend proxy endpoint for TS Bridge capabilities cannot reach its upstream or receives an upstream validation/runtime failure, the endpoint SHALL preserve the failure source and SHALL NOT report the issue as a successful local backend response.

#### Scenario: Runtime catalog request fails because Bridge is unavailable
- **WHEN** a caller invokes `GET /api/v1/bridge/runtimes` while the Go backend cannot reach TS Bridge
- **THEN** the response reports that the Bridge upstream is unavailable
- **THEN** operator or IM callers can distinguish that failure from an empty runtime catalog

#### Scenario: AI proxy request fails due to upstream validation
- **WHEN** a caller invokes `POST /api/v1/ai/decompose` with a payload rejected by TS Bridge validation
- **THEN** the Go backend returns the upstream validation failure as a rejected proxy request
- **THEN** the response does not claim that decomposition ran successfully in the backend
