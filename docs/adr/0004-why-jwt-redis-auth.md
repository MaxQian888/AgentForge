# ADR-0004: Why JWT Plus Redis / 为什么使用 JWT + Redis 双层鉴权

- Status: Accepted
- Date: 2026-03-31
- Owners: AgentForge maintainers

## Context

The API needs stateless access-token validation for normal requests, but it also
needs refresh-token rotation and explicit revocation on logout. Purely stateless
JWT handling cannot revoke an already-issued token without another server-side
state store.

## Decision

Use JWT access tokens for request authentication and Redis for:

- refresh-token storage
- access-token blacklist state keyed by JTI

The auth flow fails closed when Redis-backed revocation state is unavailable.

## Consequences

- protected routes stay fast while logout and refresh remain revocable
- Redis becomes a required dependency for truthful auth semantics, not only a cache
- frontend bootstrap can safely probe `/users/me` and perform a single refresh attempt
