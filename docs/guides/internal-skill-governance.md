# Internal Skill Governance

AgentForge now treats repository-managed skills as governed assets instead of loose `SKILL.md` copies.

The source of truth lives in:

```text
internal-skills.yaml
```

This registry declares every maintained internal skill, its ownership model, and how it is verified.

## Skill Families

### 1. `built-in-runtime`

Use this family for repo-owned product/runtime skills under:

```text
skills/<skill-id>/
```

These skills are consumed by the role catalog, runtime projection, and built-in marketplace surface.

Current examples:

- `skills/react`
- `skills/typescript`
- `skills/testing`
- `skills/css-animation`

Minimum contract:

- canonical root must stay under `skills/`
- `SKILL.md` must exist with `name` and `description` frontmatter
- at least one readable agent config must exist under `agents/*.yaml`
- the registry entry must align with `skills/builtin-bundle.yaml`

### 2. `repo-assistant`

Use this family for repo-local assistant skills under:

```text
.agents/skills/<skill-id>/
```

These skills help Codex/Claude-style repository workflows, but they are **not** runtime role skills.

Current repo-authored examples:

- `.agents/skills/echo-go-backend`
- `.agents/skills/tauri-v2`
- `.agents/skills/fumadocs-ui-css-design`

Minimum contract:

- canonical root must stay under `.agents/skills/`
- `SKILL.md` must exist with `name` and `description`
- readable `agents/*.yaml|*.yml` files are validated when present
- references/scripts/tests are optional and should only exist when they materially help the workflow

### 3. `workflow-mirror`

Use this family for canonical workflow skills that are authored once and mirrored into multiple consumer directories.

Current canonical root:

```text
.codex/skills/<skill-id>/
```

Current mirror targets:

- `.claude/skills/<skill-id>/SKILL.md`
- `.github/skills/<skill-id>/SKILL.md`

Rules:

- author the canonical content in `.codex/skills/*`
- treat `.claude` and `.github` copies as mirrors, not independent sources
- update mirrors with the sync command instead of hand-editing them

## Provenance Types

Each registry entry declares one provenance mode:

- `repo-authored`: the repository owns and edits the canonical package directly
- `upstream-sync`: the repository tracks an upstream source through `skills-lock.json`
- `generated-mirror`: reserved for mirror-only assets when needed later

If a skill uses `upstream-sync`, it must also declare a `lockKey` that exists in `skills-lock.json`.

Current upstream-synced repo-assistant skills:

- `next-best-practices`
- `shadcn`

## Allowed Exceptions

Exceptions must be explicit in `internal-skills.yaml`.

Current supported exception keys:

- `noncanonical-agent-config-extension`: allows an upstream-synced skill to keep `.yml` agent config filenames instead of repo-preferred `.yaml`

Do not add new exception types casually. Prefer normalizing repo-authored skills first.

## Authoring Workflow

### Add a new repo-authored skill

1. Choose the correct family:
   - runtime/product skill → `built-in-runtime`
   - repo workflow skill → `repo-assistant`
   - mirrored workflow skill → `workflow-mirror`
2. Create the canonical package under the expected root.
3. Add the registry entry to `internal-skills.yaml`.
4. If the skill is runtime-facing and official, also update `skills/builtin-bundle.yaml`.
5. Run verification:

```bash
pnpm skill:verify:internal
```

6. If the skill is a workflow mirror, refresh mirrors:

```bash
pnpm skill:sync:mirrors
pnpm skill:verify:internal
```

### Refresh an upstream-synced skill

1. Update the skill files under `.agents/skills/<id>/`.
2. Update the corresponding `skills-lock.json` entry.
3. Keep any structural exception explicit in `internal-skills.yaml`.
4. Re-run:

```bash
pnpm skill:verify:internal
```

## Verification Commands

```bash
pnpm skill:verify:internal
pnpm skill:verify:builtins
pnpm skill:sync:mirrors
```

- `skill:verify:internal`: verifies registry coverage, profile rules, lockfile provenance, and workflow mirror drift
- `skill:verify:builtins`: verifies the built-in runtime subset plus bundle-specific marketplace metadata
- `skill:sync:mirrors`: refreshes `.claude` and `.github` workflow mirrors from `.codex`

## Maintainer Notes

- `skills/*` remains the only product/runtime skill source root.
- `.agents/skills/*` is for repository workflow assistance, not role runtime loading.
- `.codex/.claude/.github` workflow skills should stay content-equivalent where declared as mirrors.
- Unregistered `SKILL.md` copies are treated as drift, not as implicit new managed skills.
