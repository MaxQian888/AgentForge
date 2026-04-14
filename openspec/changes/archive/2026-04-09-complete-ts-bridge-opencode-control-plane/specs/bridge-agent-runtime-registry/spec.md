## ADDED Requirements

### Requirement: OpenCode catalog discovery failures are explicit and non-silent
The runtime registry SHALL treat OpenCode catalog enrichment as truth-bearing data rather than best-effort decoration. When OpenCode health and base execution readiness succeed but agents, skills, or provider catalog discovery fails, the registry MUST keep the OpenCode runtime entry present and MUST publish machine-readable degraded diagnostics for the failed discovery surfaces instead of silently omitting those fields.

#### Scenario: OpenCode provider catalog lookup fails after readiness succeeds
- **WHEN** the Bridge can reach the OpenCode server and validate base execution readiness but provider catalog discovery fails
- **THEN** the OpenCode runtime entry remains present in `/bridge/runtimes`
- **THEN** the entry includes degraded diagnostics that identify provider catalog discovery failure instead of returning a seemingly complete entry with no provider metadata

#### Scenario: OpenCode agent or skill discovery fails independently
- **WHEN** agent discovery or skill discovery fails while the OpenCode server is otherwise healthy
- **THEN** the OpenCode runtime entry marks the affected discovery surface as degraded
- **THEN** upstream consumers can distinguish missing discovery data from an actual empty agent or skill inventory

### Requirement: OpenCode provider-auth state is published separately from execution reachability
The runtime registry SHALL publish OpenCode provider-auth readiness as its own catalog concern. A provider that is discoverable but disconnected and auth-capable MUST be represented as auth-required rather than collapsed into a generic unavailable runtime state, and the related interaction capability metadata MUST explain whether auth can be started from the Bridge control plane.

#### Scenario: Discoverable provider requires auth before execution
- **WHEN** the OpenCode provider catalog reports a provider that is available, disconnected, and exposes auth methods
- **THEN** the provider entry in the runtime catalog includes `connected=false`, `auth_required=true`, and the published auth methods
- **THEN** the OpenCode interaction capability metadata reports provider auth as degraded with a reason that tells callers authentication is required before execution
