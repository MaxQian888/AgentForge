## ADDED Requirements

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

