## ADDED Requirements

### Requirement: Backend connectivity completeness SHALL include a zero-credential smoke proof
The system SHALL maintain a repo-supported smoke proof path for backend connectivity that runs without live third-party IM credentials. At minimum, that proof MUST validate Go backend health, TS Bridge health, IM Bridge health, and one canonical IM-originated Bridge-backed command that traverses IM Bridge -> Go backend -> TS Bridge before returning a reply through the IM stub surface.

#### Scenario: Smoke proof validates the canonical IM to Go to TS Bridge path
- **WHEN** a developer runs the supported backend smoke workflow against the local backend stack
- **THEN** the proof validates that the Go backend and TS Bridge are both reachable on their canonical health surfaces
- **THEN** it exercises at least one IM-originated Bridge-backed command and only reports success if the command reply is captured through the stub flow

#### Scenario: Bridge hop failure is reported truthfully
- **WHEN** the canonical smoke proof cannot complete because TS Bridge is unavailable, Go proxy routing fails, or IM Bridge cannot return the command reply
- **THEN** the reported result identifies the broken hop instead of collapsing the failure into a generic backend error
- **THEN** the smoke proof does not claim backend connectivity is complete
