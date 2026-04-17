## 1. Schema and model

- [ ] 1.1 Migration: create `project_templates` table (`id`, `source`, `owner_user_id` nullable for system source, `name`, `description`, `snapshot_json JSONB`, `snapshot_version INT`, `created_at`, `updated_at`).
- [ ] 1.2 Indexes: `(source, owner_user_id)`, `(source, name)`; partial index `WHERE source='user'` for user-listing performance.
- [ ] 1.3 `internal/model/project_template.go`: typed struct, `Source` enum (`system|user|marketplace`), `SnapshotVersion` constant.
- [ ] 1.4 Snapshot struct definitions: `internal/model/project_template_snapshot.go` with versioned top-level fields (`Settings`, `CustomFields`, `SavedViews`, `Dashboards`, `Automations`, `WorkflowDefinitions`, `TaskStatuses`, `MemberRolePlaceholders`).

## 2. Snapshot build and apply service

- [ ] 2.1 `internal/service/project_template_service.go`: `BuildSnapshot(ctx, projectID) (Snapshot, error)` reads each subresource through existing services, strips identity/timestamp fields, runs sanitizer for secrets.
- [ ] 2.2 Sanitizer for settings: reuse audit denylist + extra explicit whitelist of settings fields considered template-safe; reject build if whitelist encounters unknown fields (fail closed).
- [ ] 2.3 `ApplySnapshot(ctx, projectID, snapshot)` in topological order: settings → customFields → savedViews → taskStatuses → workflowDefinitions → dashboards → automations (automations inserted as inactive pending re-activation by initiator).
- [ ] 2.4 Version upgrade path: if snapshot `version` < current, migrate through registered upgrade functions before apply.
- [ ] 2.5 Repository `internal/repository/project_template_repo.go` implementing CRUD + `ListVisible(userID)` returning system + user(userID) + marketplace(installed by userID).

## 3. Clone integration

- [ ] 3.1 Extend `internal/service/project_lifecycle_service.go` with `CreateFromTemplate(ctx, req, templateID, initiatorUserID)` wrapping: transactional project creation (reuse RBAC change's atomic owner registration) + `ApplySnapshot` on the new projectID.
- [ ] 3.2 `POST /projects` handler extension: when `templateSource` and `templateId` present, route to `CreateFromTemplate`; when absent, existing blank creation path unchanged.
- [ ] 3.3 Audit event `project_created_from_template` with templateId and version recorded; automation activations deferred to explicit user action afterwards.

## 4. Handlers and routes

- [ ] 4.1 `internal/handler/project_template_handler.go`: `List`, `Get`, `Create` (user source only, from an existing project), `Update` (user source, owner), `Delete` (user source, owner).
- [ ] 4.2 Route mounting: top-level protected routes `/project-templates`, `/project-templates/:id`; `POST /projects/:pid/save-as-template` on `projectGroup` with RBAC `project.save_as_template` (`admin+`).
- [ ] 4.3 Marketplace install seam: update main Go backend handler for `/api/v1/marketplace/install` to recognize `item_type=project_template` and persist as `source=marketplace` with `owner_user_id=installer`; marketplace service itself not changed in this change.

## 5. Built-in system templates

- [ ] 5.1 New `internal/service/builtin_project_template_bundle.go` (paralleling `builtin_bundle.go`): register at least one system template `"Starter Agile Project"` with sensible default customFields (priority, sprint, status), one default dashboard layout, one saved view, two baseline automation rule templates (marked inactive on clone), one default workflow template reference.
- [ ] 5.2 Bundle registration on server start; ensure system templates are idempotent (same ID / upsert).

## 6. Frontend

- [ ] 6.1 `lib/stores/project-template-store.ts`: fetch list / detail; save-as-template mutation; update/delete own user templates.
- [ ] 6.2 `components/project/new-project-dialog.tsx`: extend with a step "Start from" offering "Blank project" vs "Template"; template step lists system/user/marketplace templates grouped by source with preview.
- [ ] 6.3 `components/project/save-as-template-dialog.tsx` (new): admin+ affordance in project settings; form collects name, description; submit calls save-as-template; show the resulting template entry.
- [ ] 6.4 `app/(dashboard)/project/templates/page.tsx` (new, or a tab inside settings): list + manage user-owned templates with edit/delete; show system templates as read-only reference entries.
- [ ] 6.5 Localization: `messages/en/project-templates.json`, `messages/zh-CN/project-templates.json`.

## 7. Tests

- [ ] 7.1 Backend: `BuildSnapshot` round-trip test — create project with known config, build, apply to fresh project, compare config equivalence (modulo identities and timestamps).
- [ ] 7.2 Backend: sanitizer rejects secrets and unknown settings fields; size guard truncates or errors on oversized snapshots.
- [ ] 7.3 Backend: `CreateFromTemplate` transactionality — inject failure during apply, verify new project fully rolled back (no orphan records).
- [ ] 7.4 Backend: RBAC on save-as-template (admin+) and on user template update/delete (owner only).
- [ ] 7.5 Backend: marketplace install seam converts `item_type=project_template` into a user-visible template.
- [ ] 7.6 Backend: built-in template bundle registration yields at least one system template visible via List.
- [ ] 7.7 Frontend: new-project-dialog template selection flow; save-as-template dialog happy path; templates page listing and delete confirmation.
- [ ] 7.8 `pnpm exec tsc --noEmit`, `pnpm test`, `cd src-go && go test ./...`.

## 8. Docs

- [ ] 8.1 API reference for project-template endpoints and `POST /projects` template params.
- [ ] 8.2 Author guide: what counts as "project configuration" vs "project content" — educate contributors on what should go into snapshot vs stay out.
- [ ] 8.3 Marketplace publisher note (short, pointing to follow-up spec for full publish flow).
