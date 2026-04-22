# guarded-project-i18n-support/CLAUDE.md

i18n support skill for AgentForge.

## Purpose

Guards and guides internationalization workflows in AgentForge: message extraction, key auditing, locale management, and translation safety.

## Key References

| File | Content |
|------|---------|
| `SKILL.md` | Skill definition, i18n rules |
| `references/` | Supporting documentation |

## Directories

| Directory | Content |
|-----------|---------|
| `scripts/` | i18n automation scripts |
| `tests/` | Validation tests |
| `agents/` | Agent/task definitions |
| `evals/` | Evaluation datasets |

## Project Context

- Frontend i18n uses `next-intl` (`lib/i18n/`, `messages/`)
- Audit script: `pnpm i18n:audit`
