## ADDED Requirements

### Requirement: Lightweight AI operations use canonical bridge endpoints
The TypeScript Bridge SHALL expose provider-aware lightweight AI operations through the canonical `/bridge/decompose`, `/bridge/classify-intent`, and `/bridge/generate` endpoints, and Go-side callers plus live project documentation MUST use those endpoints as the primary contract. Compatibility aliases MAY remain for migration, but they SHALL NOT become the preferred entrypoints for new integration work.

#### Scenario: Go task decomposition uses the canonical bridge route
- **WHEN** the Go bridge client requests task decomposition
- **THEN** it SHALL call `/bridge/decompose`
- **THEN** the bridge SHALL apply the same provider-resolution and validation rules defined for lightweight AI requests

#### Scenario: Intent classification documentation avoids legacy primary routes
- **WHEN** project documentation describes IM intent classification through the TS Bridge
- **THEN** it SHALL identify `/bridge/classify-intent` as the primary live endpoint
- **THEN** it SHALL not describe `/api/classify-intent` or similar historical paths as the current canonical contract

#### Scenario: Compatibility alias keeps provider semantics aligned
- **WHEN** a legacy alias such as `/ai/decompose` or `/ai/generate` is invoked for a supported lightweight AI operation
- **THEN** the bridge SHALL enforce the same provider, model, and credential validation behavior as the corresponding canonical `/bridge/*` endpoint
- **THEN** the response semantics SHALL remain equivalent to the canonical route
