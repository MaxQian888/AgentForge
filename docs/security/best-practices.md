# Security Best Practices / 安全编码清单

This checklist is grounded in the current AgentForge stack: Next.js frontend,
Go API, Redis-backed auth state, plugin control plane, and Tauri shell.

## Secrets And Config

- keep secrets in local env files such as `.env.local` or `src-go/.env`
- never expose secrets through `NEXT_PUBLIC_*`
- require a real `JWT_SECRET` outside development
- keep plugin registry and IM bridge secrets scoped per environment

## API And Validation

- validate every request body with the model validators already wired through Echo
- reject malformed UUIDs and invalid enum values early
- fail closed when auth/cache dependencies are unavailable
- use project-scoped middleware before mutating project resources

## Database And Query Safety

- keep PostgreSQL schema changes in versioned SQL migrations
- avoid raw string-concatenated SQL
- prefer repository-layer abstractions already used in `src-go/internal/repository`
- review JSONB and array indexes when adding new filtered query paths

## Frontend Safety

- treat the backend as the authority for auth/session state
- avoid trusting local booleans for protected-route access
- sanitize or constrain rich-content rendering paths
- keep desktop-only capabilities behind the platform facade

## Plugin Safety

- treat non-builtin or external plugins as untrusted until trust metadata or approval permits activation
- keep MCP tools/resources/prompts scoped and explicit
- keep WASM plugin capabilities minimal and documented in the manifest
- record lifecycle and runtime errors in plugin events for operator auditability

## Tauri Safety

- only add native permissions that are required for the concrete feature
- prefer named sidecars over broad shell permissions
- keep updater signing material out of source control
- verify desktop and web fallback behavior together when native surfaces change

## Operational Hygiene

- update docs when runtime commands, auth behavior, or permissions change
- run the test/build surface that matches the code you touched
- include doc updates in PR review when behavior or operator flows changed
