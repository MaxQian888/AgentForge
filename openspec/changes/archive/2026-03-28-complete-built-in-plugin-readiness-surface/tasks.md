## 1. Bundle Readiness Contract

- [x] 1.1 Extend `plugins/builtin-bundle.yaml` parsing and validation so official built-in entries can declare structured readiness metadata for prerequisites, configuration needs, docs references, and next-step guidance.
- [x] 1.2 Update shared plugin models and bundle helpers so built-in discovery and catalog DTOs can carry evaluated readiness state, blocking reasons, and setup guidance without collapsing them into a single availability string.

## 2. Control-Plane Readiness Evaluation

- [x] 2.1 Implement deterministic built-in readiness evaluation in the Go plugin service using bundle metadata plus local host or config preflight checks, while keeping installability separate from activation readiness.
- [x] 2.2 Wire the evaluated readiness data through built-in discovery, catalog assembly, and installed-record hydration so official built-ins preserve the same readiness semantics before and after install.
- [x] 2.3 Add backend tests covering `ready`, `requires_prerequisite`, `requires_configuration`, and `unsupported_host` cases, including cases where a built-in remains installable but not activation-ready.

## 3. Plugin Management Surface

- [x] 3.1 Update frontend store types and API handling to consume the structured readiness payload for official built-ins and installed built-in records.
- [x] 3.2 Extend the plugins page and related detail surfaces to render readiness badges, blocking reasons, docs links, setup guidance, and truthful action gating for built-in and installed plugin flows.
- [x] 3.3 Add UI tests for built-in readiness rendering, installed-but-blocked built-ins, and host-unsupported or browse-only states so the panel does not regress into static messaging.

## 4. Verification And Acceptance

- [x] 4.1 Extend `scripts/verify-built-in-plugin-bundle` and related fixtures or tests to validate readiness metadata drift and deterministic readiness preflight behavior for official built-ins.
- [x] 4.2 Run focused verification for built-in discovery or catalog readiness behavior, plugin panel rendering, and readiness-script coverage, then document any intentionally opt-in live prerequisites in the final implementation notes.
