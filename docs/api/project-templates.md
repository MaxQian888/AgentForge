# Project Templates

Reusable project-configuration snapshots. A project template captures a project's configuration — settings, custom fields, saved views, dashboards, automations, workflow definitions, task statuses, and advisory role placeholders — and lets users clone new projects from that snapshot in a single `POST /projects` call.

Templates do **not** include business data: members, tasks, knowledge assets, comments, runs, logs, memory entries, or invitations. See the Author Guide section below for how to decide what goes into a snapshot.

See also:
- `openspec/changes/2026-04-17-add-project-templates/proposal.md` — the source of truth for scope.
- `openspec/changes/2026-04-17-add-project-templates/design.md` — the trade-offs behind the design.
- `docs/api/projects.md` — the `POST /projects` base contract this extends.
- `docs/api/rbac.md` — the matrix entry for `project.save_as_template`.

## Storage

Migration `059_create_project_templates.up.sql` adds the `project_templates` table:

| Column             | Notes                                                                                              |
|--------------------|----------------------------------------------------------------------------------------------------|
| `id`               | UUID                                                                                               |
| `source`           | `system` (built-in), `user` (owner-private), `marketplace` (installed from market)                  |
| `owner_user_id`    | nullable for `system`; required otherwise                                                           |
| `name`             | 1–128 chars                                                                                         |
| `description`      | free text, ≤ 4096 chars                                                                            |
| `snapshot_json`    | JSONB envelope — see "Snapshot shape" below                                                        |
| `snapshot_version` | integer; the service runs a registered upgrade chain from this version to `CurrentProjectTemplateSnapshotVersion` on clone |
| `created_at` / `updated_at` | standard timestamps                                                                       |

Indexes: `(source, owner_user_id)`, `(source, name)`, and a partial `WHERE source='user'` on `(owner_user_id)` for fast per-user listing.

## Snapshot shape

The top-level keys are **fixed**. Adding a new sub-resource to a snapshot requires (1) a new field on `model.ProjectTemplateSnapshot`, (2) a version bump in `model.CurrentProjectTemplateSnapshotVersion`, and (3) an entry in `projectTemplateSnapshotUpgrades`.

```json
{
  "version": 1,
  "settings": {
    "reviewPolicy":       { "...": "whitelisted subset" },
    "defaultCodingAgent": { "runtime": "...", "provider": "...", "model": "..." },
    "budgetGovernance":   { "...": "..." }
  },
  "customFields":        [ { "key": "...", "label": "...", "type": "..." } ],
  "savedViews":          [ { "name": "...", "kind": "...", "config": { } } ],
  "dashboards":          [ { "name": "...", "widgets": [ { "type": "burndown", "...": "..." } ] } ],
  "automations":         [ { "name": "...", "trigger": { }, "actions": [] } ],
  "workflowDefinitions": [ { "name": "...", "templateRef": "...", "definitionJson": { } } ],
  "taskStatuses":        [ { "key": "...", "label": "...", "order": 0 } ],
  "memberRolePlaceholders": [ { "label": "Project Manager", "suggestedRole": "admin" } ]
}
```

### Fields that are explicitly **not** copied

- `webhook.url` / `webhook.secret` — integration credentials.
- `automations.configuredByUserID` — stripped. Clone initiator rebinds on activation.
- Any `*_id`, `created_at`, `updated_at` — stripped before serialization.
- Anything matching the audit sanitizer's secret-key denylist.

The sanitizer is fail-closed: any unknown top-level settings field causes `BuildSnapshot` to error rather than silently drop data.

## Endpoints

### `POST /projects` (extended)

Existing fields keep their previous behavior. Two new optional fields enable the template path:

| Field            | Type   | Notes                                                                                             |
|------------------|--------|---------------------------------------------------------------------------------------------------|
| `templateSource` | string | `system` \| `user` \| `marketplace`. Required when `templateId` is set.                            |
| `templateId`     | UUID   | When set, the backend clones this template's snapshot onto the new project inside the create call. |

Both empty = existing blank-project behavior. RBAC: unchanged (project creation itself has no role gate; the caller becomes owner).

Audit emission: after a successful clone the service records a `project.created_from_template` event with `{ templateId, templateSource, snapshotVersion }` as the payload.

### `GET /project-templates`

Lists templates visible to the caller:

- every `system` template
- every `user` template where `owner_user_id` = caller
- every `marketplace` template where `owner_user_id` = caller

Response body omits `snapshot` — callers fetch the full snapshot via `GET /project-templates/{id}`.

### `GET /project-templates/{id}`

Returns a single template with its `snapshot` payload included. Non-visible templates return 404.

### `PUT /project-templates/{id}`

Edits `name` / `description` on a user-source or marketplace-source template the caller owns. System templates return 403. The snapshot itself is immutable post-creation — callers who want a refreshed snapshot recreate the template.

### `DELETE /project-templates/{id}`

Removes a user-source or marketplace-source template the caller owns. System templates return 403. Projects already cloned from this template are **not** affected — clones copy the snapshot, they do not reference it.

### `POST /projects/{pid}/save-as-template`

Builds a snapshot of project `:pid` and persists it as a user-source template owned by the caller.

- RBAC: `project.save_as_template` — **admin+** on the project.
- Body: `{ "name": "...", "description": "..." }`.
- Response: the newly created `ProjectTemplateDTO`.

## Errors

| HTTP | i18n message id                      | Meaning                                                        |
|------|--------------------------------------|----------------------------------------------------------------|
| 403  | `ProjectTemplateImmutableSystem`     | attempted update/delete on a `system` template                  |
| 403  | `ProjectTemplateOwnerMismatch`       | caller is not the owner of this user/marketplace template       |
| 404  | `ProjectTemplateNotFound`            | id does not exist OR caller cannot see it                       |
| 413  | `ProjectTemplateSnapshotTooLarge`    | snapshot exceeds 1 MiB on build                                |
| 422  | `ProjectTemplateSnapshotInvalid`     | stored snapshot cannot be parsed / is malformed                 |

## Author guide — "what counts as project configuration?"

A sub-resource **belongs in a template** when:

- It is configured once per project and rarely changes during day-to-day work (custom field schemas, workflow definitions, dashboard layouts).
- It describes **structure**, not **events** (a custom field definition ✅; a task that happens to use it ❌).
- It does not bind to a specific human actor or integration credential (review policy ✅; webhook URL ❌).

A sub-resource **stays out of a template** when:

- It is generated as the project runs (tasks, runs, logs, comments, notifications, invitations).
- It captures member-level state (member availability, individual notification prefs).
- It references external identities / secrets (webhook URLs, OAuth client secrets, API keys).

The snapshot schema (`ProjectTemplateSnapshot`) declares the fixed set of sub-resource families. Adding a family requires a design note in the relevant OpenSpec change plus a snapshot version bump.

## Marketplace publisher note

The marketplace receiving seam is live: the main backend recognizes `item_type=project_template` at `POST /api/v1/marketplace/install` and materializes the payload as a `marketplace`-source template for the installing user. The **publish flow** (how a user turns a template into a marketplace-published item) is not yet implemented in `src-marketplace`; it will be specified in a follow-up change under `openspec/changes/` once the receiving end is stable.

Artifact layout expected by the install seam:

```
project-template.zip
└── project-template.json          # { name, description, snapshotVersion, snapshot }
```

Single subdirectory is also supported (`project-template.zip` → `name/project-template.json`).
