# State Management Guide / Zustand 状态管理指南

AgentForge uses one domain-focused Zustand store per major surface under
`lib/stores/`.

## Store Layout

Representative stores:

- Auth and session: `auth-store.ts`
- Appearance and locale: `appearance-store.ts`, `locale-store.ts`
- Task workspace: `task-store.ts`, `task-workspace-store.ts`, `task-comment-store.ts`
- Review: `review-store.ts`
- Plugins: `plugin-store.ts`
- Docs/wiki: `docs-store.ts`
- Scheduler: `scheduler-store.ts`
- Realtime fan-in: `ws-store.ts`

## Patterns Used In This Repo

### 1. Persistent auth and preferences

Persisted stores already in use:

- `useAuthStore`
- `useAppearanceStore`
- `useLocaleStore`

These use `zustand/middleware/persist` and only write the minimum state needed
for restoration.

### 2. Domain store plus action methods

Store actions are colocated with domain state:

- fetch/mutate through `createApiClient`
- keep async effects inside store actions when the store owns the domain
- normalize API payloads close to the store boundary

### 3. UI-only workspace state

`useTaskWorkspaceStore` is the pattern for per-page view state:

- `viewMode`
- filters
- selected task IDs
- context rail visibility
- density and display options

This keeps list/board/timeline/calendar switching out of the persistence layer.

### 4. Realtime projection

`useWSStore` is the single websocket fan-in surface. It:

- owns connection/subscription lifecycle
- projects incoming events into domain stores
- keeps the rest of the UI free of direct WebSocket client usage

## Example: Auth Bootstrap

`useAuthStore` persists:

- `accessToken`
- `refreshToken`
- `user`

and exposes:

- `login`
- `register`
- `logout`
- `bootstrapSession`
- `clearSession`

The bootstrap path first probes `/api/v1/users/me`, then performs one refresh
attempt if needed.

## Example: Plugin Store

`usePluginStore` mixes:

- installed plugin state
- marketplace/catalog results
- event streams
- MCP capability snapshots
- workflow run state

This is the repo's example for a store that owns both CRUD actions and
operator-facing diagnostics.

## Recommended Rules

- add a new store only when a domain truly owns its own API/realtime boundary
- keep persisted state minimal
- keep cross-store writes explicit through `getState()` rather than hidden global mutation
- centralize websocket event projection in `useWSStore`
- avoid duplicating the same backend resource in multiple stores with conflicting normalization rules
