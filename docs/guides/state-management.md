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

## Primitive → Store Pairings

The shared page primitives documented in `docs/guides/frontend-components.md`
assume specific store patterns. When you reach for a primitive, wire it to the
matching store seam below — don't invent a parallel path.

| Primitive | Canonical hook | Pattern |
| --- | --- | --- |
| `PageHeader` breadcrumbs | `useBreadcrumbs` from `hooks/use-breadcrumbs.ts` | call once in the page `useEffect`; also pass to `PageHeader.breadcrumbs` prop |
| `PageHeader` actions | domain store action or `useLayoutStore` | primary actions dispatch through the owning domain store; global actions (command palette open) come from `useLayoutStore` |
| `FilterBar` search + filters | domain filter slice (e.g., `useSchedulerStore.listFilters`, `useMarketplaceStore.filters`) | set via `set*Filters`, reset via `reset*Filters`; keep the URL sync in the page, not the store |
| `FilterBar` reset | same filter slice | expose a single `reset*Filters()` action so the primitive can call it through `onReset` |
| `MetricCard` (value) | derived selector on the domain store | memoize on the store; never recompute in render |
| `MetricCard` (sparkline) | `lib/dashboard/metric-sparkline.ts` helpers | pass normalized `{timestamp, amount}` rows from the store |
| `MetricCard` (loading) | domain store `loading` flag | pair with the skeleton layout while the store resolves |
| `SectionCard` | section-local presentational state | no store dependency required; if a section needs persistence, expose it through the owning domain store |
| `ResponsiveTabs` | URL param + store `selected*` action | read `value` from `useSearchParams`, dispatch `onValueChange` to both the URL and the store's `select*` action |
| `EmptyState` | domain store `items` length | render when the list is empty and no filters are active |
| `ErrorBanner` | domain store `error` field | show above the affected section; `onRetry` calls the same fetch action that produced the error |
| `skeleton-layouts/*` | domain store `loading` flag | match the footprint of the loaded primitive so layout doesn't shift |

### When adding a new page

1. Pick a layout template (`OverviewLayout` / `ListLayout` / `SettingsLayout` /
   `WorkspaceLayout`).
2. Identify the domain store that owns the page data. Reuse an existing store
   where possible (a new sibling slice beats a whole new store).
3. Wire the primitives above to that store's actions — not to ad-hoc local
   state.
4. Update both `docs/guides/frontend-components.md` (audit row) and this guide
   if you introduce a new primitive.
