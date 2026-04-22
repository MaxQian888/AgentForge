# .agents/CLAUDE.md

AgentForge skill catalog for Claude Code.

## Overview

This directory contains reusable skills that extend Claude Code capabilities for AgentForge-specific workflows. Each skill is a self-contained bundle of knowledge, rules, and task definitions.

## Structure

```
.agents/
  skills/
    <skill-name>/
      SKILL.md              # Skill definition (name, description, rules)
      references/           # Supporting docs, recipes, and deep-dives
      agents/               # Agent/task definitions
      rules/                # Enforced rule files (Incorrect/Correct pairs)
      scripts/              # Automation scripts
      tests/                # Validation tests
      assets/               # Static assets
      evals/                # Evaluation datasets
```

## Skills

| Skill | Purpose | Guide |
|-------|---------|-------|
| `shadcn` | shadcn/ui component usage, styling rules, CLI workflows | [CLAUDE.md](skills/shadcn/CLAUDE.md) |
| `next-best-practices` | Next.js App Router patterns, RSC boundaries, async APIs | [CLAUDE.md](skills/next-best-practices/CLAUDE.md) |
| `echo-go-backend` | Echo framework backend patterns, auth lifecycle, Go idioms | [CLAUDE.md](skills/echo-go-backend/CLAUDE.md) |
| `tauri-v2` | Tauri v2 desktop integration, sidecar patterns, Rust bridge | [CLAUDE.md](skills/tauri-v2/CLAUDE.md) |
| `fumadocs-ui-css-design` | Fumadocs-based documentation site design | [CLAUDE.md](skills/fumadocs-ui-css-design/CLAUDE.md) |
| `deploying-and-running-react-go-quick-starter` | React+Go quick-starter deployment surfaces | [CLAUDE.md](skills/deploying-and-running-react-go-quick-starter/CLAUDE.md) |
| `guarded-project-i18n-support` | i18n workflows, message extraction, translation safety | [CLAUDE.md](skills/guarded-project-i18n-support/CLAUDE.md) |

## Contributing

- Skills are auto-discovered by Claude Code when placed under `.agents/skills/`.
- Keep `SKILL.md` frontmatter accurate: `name`, `description`, `user-invocable`.
- Reference files should use relative paths from the skill root.
- Use `agents/` for sub-task definitions that skills can delegate to.
