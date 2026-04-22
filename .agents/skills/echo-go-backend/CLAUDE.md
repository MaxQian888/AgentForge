# echo-go-backend/CLAUDE.md

Echo Go backend skill for AgentForge.

## Purpose

Guides Go backend development with LabStack Echo in the `src-go/` workspace: route registration, handlers, middleware, binding, validation, auth, testing, migrations, and Tauri sidecar integration.

## Key References

| File | Content |
|------|---------|
| `SKILL.md` | Skill definition, task map, workflow |
| `references/common-task-recipes.md` | Common backend task recipes |
| `references/auth-data-lifecycle.md` | JWT, refresh tokens, Redis, PostgreSQL auth flow |
| `references/version-translation.md` | Echo version-specific translations |
| `references/frontend-desktop-integration.md` | Backend URL discovery, desktop handoff |
| `references/official-sources.md` | Official Echo documentation pointers |
| `references/project-template.md` | Repository template conventions |

## Key Workflows

1. **New endpoint**: inspect `src-go/internal/server/routes.go` + `references/common-task-recipes.md`
2. **Binding/validation**: inspect `src-go/internal/handler`, `src-go/internal/model`
3. **Middleware**: inspect `src-go/internal/server/server.go`
4. **JWT/auth**: inspect `src-go/internal/middleware/jwt.go`, `references/auth-data-lifecycle.md`
5. **Startup/sidecar**: inspect `src-go/cmd/server/main.go`, `src-tauri/src/lib.rs`
