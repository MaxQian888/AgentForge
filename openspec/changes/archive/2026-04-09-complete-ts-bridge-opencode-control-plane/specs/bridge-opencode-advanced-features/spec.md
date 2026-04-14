## ADDED Requirements

### Requirement: OpenCode permission requests bind canonical Bridge request IDs to upstream permission identifiers
The Bridge SHALL normalize each OpenCode permission request into a Bridge-owned pending interaction record that stores the originating upstream session binding plus the upstream permission identifier. When OpenCode asks for approval, the Bridge SHALL emit a canonical `permission_request` event containing a Bridge-generated `request_id` and SHALL resolve `POST /bridge/permission-response/:request_id` by forwarding the decision to the matching upstream OpenCode permission endpoint instead of treating the request as a Claude-only callback.

#### Scenario: OpenCode permission request round-trip resolves against the correct session permission
- **WHEN** an active OpenCode session emits a permission request with upstream `permissionID` `perm-42`
- **THEN** the Bridge emits a canonical `permission_request` event that includes a new Bridge `request_id`
- **THEN** `POST /bridge/permission-response/{request_id}` forwards the caller's allow or deny decision to `POST /session/{upstream_session_id}/permissions/perm-42`

#### Scenario: Permission response is rejected when the Bridge no longer has a live pending mapping
- **WHEN** a caller posts to `/bridge/permission-response/{request_id}` after the pending OpenCode permission mapping expired or was already resolved
- **THEN** the Bridge returns an explicit pending-request-not-found error
- **THEN** it SHALL NOT claim the permission decision succeeded locally

### Requirement: OpenCode provider auth handshake is exposed through canonical Bridge control routes
The Bridge SHALL expose additive canonical routes for OpenCode provider authentication under the `/bridge/*` family so callers can initiate and complete provider OAuth or equivalent upstream auth without bypassing the Bridge runtime contract. The Bridge SHALL use the upstream OpenCode provider authorize and callback surfaces behind those routes and SHALL publish the resulting provider-auth state back through the runtime catalog.

#### Scenario: Start provider auth for a disconnected OpenCode provider
- **WHEN** a caller posts to `POST /bridge/opencode/provider-auth/{provider}/start` for an OpenCode provider whose catalog entry reports `auth_required=true`
- **THEN** the Bridge requests the upstream provider authorize surface and returns a Bridge-owned `request_id` plus the authorization URL or equivalent auth payload
- **THEN** the pending auth interaction remains bound to that provider until completion or expiry

#### Scenario: Complete provider auth and refresh catalog readiness
- **WHEN** a caller posts the callback payload to `POST /bridge/opencode/provider-auth/{request_id}/complete` for a pending OpenCode provider-auth interaction
- **THEN** the Bridge forwards the opaque callback payload to the matching upstream provider callback surface
- **THEN** subsequent OpenCode runtime catalog reads reflect the updated provider connectivity or remaining auth failure truthfully
