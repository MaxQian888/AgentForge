# .agents/AGENTS.md

## Skill Development Guidelines

### Structure

Each skill lives in `.agents/skills/<name>/` and contains:
- `SKILL.md` — skill definition with YAML frontmatter (`name`, `description`, `user-invocable`)
- `references/` — deep-dive docs, recipes, official sources
- `agents/` — agent/task definitions for delegation
- `rules/` — enforced Incorrect/Correct rule files
- `scripts/`, `tests/`, `assets/`, `evals/` — optional supporting files

### Frontmatter

```yaml
---
name: skill-name
description: When to use this skill and what it covers.
user-invocable: false  # or true
---
```

### Writing Rules

- Keep `description` actionable: mention triggers ("Use when...") and scope
- Reference files use relative paths from the skill root
- Incorrect/Correct pairs in `rules/` must be copy-paste ready
- Prefer linking over inlining large code blocks

### Validation

```bash
pnpm skill:verify:internal   # verify internal skills
pnpm skill:verify:builtins   # verify built-in skill bundle
pnpm skill:sync:mirrors      # sync internal skill mirrors
```

### Adding a New Skill

1. Create directory `.agents/skills/<name>/`
2. Write `SKILL.md` with frontmatter
3. Add `references/` and any rules/scripts/tests
4. Run verification before committing

### Commit Style

- Conventional Commits: `feat(skill):`, `fix(skill):`, `docs(skill):`
- Example: `feat(shadcn): add data-table rule for empty states`
