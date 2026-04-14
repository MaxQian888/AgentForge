# auth-session-management Specification

## Purpose
Define the baseline contract for AgentForge frontend and backend authentication session management, including the canonical auth response payload, session bootstrap and refresh recovery, authoritative identity loading, protected-route auth resolution, and fail-closed revocation behavior when token-cache state is unavailable.

## Requirements
### Requirement: Frontend auth state SHALL use the backend auth response contract
The system SHALL treat the backend auth response as the canonical frontend session payload. Successful register, login, and refresh operations MUST return and store `accessToken`, `refreshToken`, and `user` together so that subsequent authenticated API calls use the stored `accessToken` as the Bearer token and the frontend can restore the same user identity across reloads.

#### Scenario: Login stores the canonical session payload
- **WHEN** a user successfully submits valid credentials to `POST /api/v1/auth/login`
- **THEN** the frontend stores the returned `accessToken`, `refreshToken`, and `user`
- **AND** subsequent authenticated API calls use the stored `accessToken` as the Bearer token

#### Scenario: Register stores the canonical session payload
- **WHEN** a user successfully submits a valid registration request to `POST /api/v1/auth/register`
- **THEN** the frontend stores the returned `accessToken`, `refreshToken`, and `user`
- **AND** the application transitions into an authenticated session without requiring a second login

### Requirement: Stored sessions SHALL be revalidated and recoverable on application bootstrap
When the application starts or enters a protected area with stored auth data, the system SHALL revalidate the session against the backend before treating the user as authenticated. If the access token is no longer valid and a refresh token is present, the system MUST attempt one refresh flow and retry identity validation before deciding whether to keep or clear the session.

#### Scenario: Valid stored access token restores the session
- **WHEN** the application starts with a stored access token and refresh token
- **AND** `GET /api/v1/users/me` succeeds for the current access token
- **THEN** the frontend marks the session as authenticated
- **AND** it updates the in-memory user state from the authoritative `/users/me` response

#### Scenario: Expired access token is recovered through refresh
- **WHEN** the application starts with a stored refresh token and an access token that no longer authorizes `GET /api/v1/users/me`
- **AND** `POST /api/v1/auth/refresh` succeeds
- **THEN** the frontend replaces the stored session with the newly returned `accessToken`, `refreshToken`, and `user`
- **AND** it retries identity validation before rendering protected content

#### Scenario: Failed refresh clears the stored session
- **WHEN** the application starts with stored auth data
- **AND** access-token validation fails
- **AND** the refresh request is rejected or the refreshed session still fails identity validation
- **THEN** the frontend clears the stored auth session
- **AND** the user is treated as unauthenticated

### Requirement: Protected application routes SHALL wait for auth resolution before rendering
The system SHALL not render protected dashboard content based only on a persisted boolean flag. Protected layouts MUST wait until auth bootstrap resolves to either an authenticated or unauthenticated state, then render protected content only for authenticated users and redirect unauthenticated users to the login flow.

#### Scenario: Protected layout blocks rendering while auth is being checked
- **WHEN** a user navigates to a protected dashboard route and auth bootstrap is still in progress
- **THEN** the protected layout does not render dashboard content as authenticated

#### Scenario: Unauthenticated user is redirected from a protected route
- **WHEN** auth bootstrap resolves without a valid session for a protected dashboard route
- **THEN** the application redirects the user to `/login`
- **AND** protected dashboard content is not rendered

#### Scenario: Authenticated user can enter a protected route after bootstrap
- **WHEN** auth bootstrap resolves with a valid authenticated session
- **THEN** the protected dashboard layout renders the requested content

### Requirement: The identity endpoint SHALL return authoritative user profile data
`GET /api/v1/users/me` SHALL use the authenticated subject to load the current user profile from the authoritative user store and return the full public user representation required by the frontend session. The endpoint MUST not rely only on JWT claims when constructing the user response.

#### Scenario: Authenticated request returns the stored user profile
- **WHEN** an authenticated user calls `GET /api/v1/users/me` with a valid access token
- **THEN** the backend loads the user record associated with the token subject
- **AND** it returns the current public user profile, including fields required by the frontend session model

#### Scenario: Missing or invalid identity cannot be restored
- **WHEN** `GET /api/v1/users/me` is called with a token whose subject cannot be resolved to a valid user
- **THEN** the backend rejects the request as unauthorized
- **AND** the frontend bootstrap flow clears the stored session instead of keeping stale user data

### Requirement: Expired access tokens SHALL be refreshed automatically during active sessions
When an authenticated API call receives a 401 response and a refresh token is available, the system SHALL attempt to refresh the access token and retry the original request transparently. If the refresh succeeds, the retried request MUST use the new access token and the user session MUST remain uninterrupted. If the refresh fails, the system SHALL clear the session and treat the user as unauthenticated.

#### Scenario: Automatic retry after 401 during active session
- **WHEN** an authenticated API call returns HTTP 401
- **AND** a refresh token is stored in the auth session
- **THEN** the system calls `POST /api/v1/auth/refresh` with the stored refresh token
- **AND** on success, retries the original API call with the new access token
- **AND** the user does not observe the refresh (transparent)

#### Scenario: Failed auto-refresh clears the session
- **WHEN** an authenticated API call returns HTTP 401
- **AND** the subsequent refresh request fails (rejected, network error, or retried call still returns 401)
- **THEN** the system clears the stored auth session
- **AND** the user is treated as unauthenticated and redirected to login

#### Scenario: Non-401 errors are not intercepted
- **WHEN** an authenticated API call returns a non-401 error (e.g., 403, 500)
- **THEN** the error propagates to the caller without triggering a refresh attempt

### Requirement: Concurrent refresh attempts SHALL be coalesced into a single request
When multiple API calls fail with 401 simultaneously, the system SHALL ensure only one refresh request is sent to the backend. All concurrent callers MUST await the same refresh result and retry with the same new token.

#### Scenario: Two concurrent 401s produce one refresh call
- **WHEN** two authenticated API calls both return 401 at nearly the same time
- **THEN** exactly one `POST /api/v1/auth/refresh` request is sent
- **AND** both original requests are retried with the new access token from that single refresh

#### Scenario: Second 401 during in-flight refresh awaits existing refresh
- **WHEN** a refresh request is already in progress due to a prior 401
- **AND** another API call returns 401
- **THEN** the second caller awaits the in-flight refresh result instead of issuing a new refresh request

### Requirement: Revocation-dependent auth flows SHALL fail closed when the token cache is unavailable
The system SHALL treat refresh-token validation, logout revocation, and access-token blacklist checks as security-critical operations. If the token cache required for those operations is unavailable, the backend MUST reject the auth operation explicitly and MUST NOT silently accept a protected request or mint new tokens as though revocation state were healthy.

#### Scenario: Refresh is rejected when the refresh-token cache is unavailable
- **WHEN** `POST /api/v1/auth/refresh` is called and the backend cannot read the stored refresh token because the token cache is unavailable
- **THEN** the backend rejects the refresh request
- **AND** it does not issue a new access token or refresh token

#### Scenario: Protected route is not accepted when blacklist verification is unavailable
- **WHEN** a request reaches a JWT-protected route
- **AND** the backend cannot verify blacklist state for the presented access token because the token cache is unavailable
- **THEN** the backend rejects the request
- **AND** the request is not forwarded to the protected handler

#### Scenario: Logout does not report success when revocation cannot be persisted
- **WHEN** `POST /api/v1/auth/logout` is called and the backend cannot persist the required token revocation state
- **THEN** the backend returns a failure response instead of reporting a successful logout
- **AND** the frontend clears its local session and requires a new login before future protected access
