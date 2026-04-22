## Description

<!--
Provide a comprehensive description of the changes.
- What problem does this PR solve?
- What approach did you take and why?
- Link to any relevant design docs, discussions, or precedents.
-->

### Summary

<!-- One or two sentences summarizing the change for reviewers scanning quickly -->

### Motivation & Context

<!-- Why is this change needed? Reference issues, Slack threads, or architectural decisions -->

## Related Issue

<!--
Link to the issue this PR addresses.
Use GitHub keywords to auto-close: Closes #123, Fixes #456, Resolves #789
For partial fixes, use: Related to #123, Addresses #456
-->

Closes #

## Scope & Subproject

<!-- Mark all areas affected by this change -->

- [ ] Frontend (`app/`, `components/`, `lib/`, `hooks/`)
- [ ] Go Orchestrator (`src-go/`)
- [ ] Bridge (`src-bridge/`)
- [ ] IM Bridge (`src-im-bridge/`)
- [ ] Marketplace (`src-marketplace/`)
- [ ] Tauri Desktop (`src-tauri/`)
- [ ] Shared / Cross-cutting (schemas, contracts, tooling)
- [ ] Documentation only (`docs/`, `*.md`)

### Runtime Surface

<!-- If Frontend or Tauri is checked above, mark which runtimes were verified -->

- [ ] Web (`pnpm dev` → <http://localhost:3000>)
- [ ] Desktop (`pnpm tauri dev`)
- [ ] Both
- [ ] N/A (backend/tooling change only)

## Type of Change

<!-- Mark ALL relevant options with an "x" -->

- [ ] 🐛 Bug fix (non-breaking change that fixes an issue)
- [ ] ✨ New feature (non-breaking change that adds functionality)
- [ ] 💥 Breaking change (fix or feature that causes existing functionality to break)
- [ ] 📝 Documentation update
- [ ] 🎨 Style/UI update
- [ ] ♻️ Code refactor (no functional changes)
- [ ] ⚡ Performance improvement
- [ ] ✅ Test update
- [ ] 🔧 Build/CI configuration
- [ ] 🧹 Chore (dependency updates, formatting, etc.)
- [ ] 🛡️ Security fix

## Changes Made

<!--
List changes organized by subproject or layer.
Be specific: file names, function names, API route changes, DB migrations.
-->

### Frontend
- 
- 

### Backend / API
- 
- 

### Desktop (Tauri)
- 
- 

### Bridge / IM Bridge / Marketplace
- 
- 

### Tooling / CI / Infra
- 
- 

## Screenshots / Recordings

<!--
If the PR includes UI changes, add screenshots or screen recordings.
For desktop changes, include both web and native window shots if applicable.
Use `<details>` blocks to keep the PR readable when many images are needed.
-->

<details>
<summary>Click to expand screenshots</summary>

<!-- Drag and drop images here -->

</details>

## Testing

<!-- Describe what you tested and how. Be specific about commands run and environments. -->

### Local Verification

- [ ] I have tested this locally on the relevant runtime(s) marked above
- [ ] I have tested the happy path end-to-end
- [ ] I have tested edge cases and error paths

### Automated Tests

<!-- Check all that apply. Provide commands used if non-standard. -->

- [ ] Frontend unit tests pass (`pnpm test`)
- [ ] Frontend type check passes (`pnpm exec tsc --noEmit`)
- [ ] Go tests pass (`cd src-go && go test ./...`)
- [ ] Bridge tests pass (`cd src-bridge && bun test`)
- [ ] IM Bridge tests pass (`cd src-im-bridge && go test ./...`)
- [ ] Marketplace tests pass (`cd src-marketplace && go test ./...`)
- [ ] E2E tests pass (`pnpm test:e2e`)
- [ ] Tauri Rust tests pass (`pnpm test:tauri`)
- [ ] New tests added/updated to cover the change
- [ ] N/A — change is documentation, config, or non-testable plumbing

### Test Instructions for Reviewers

<!--
Provide concrete, copy-pastable steps a reviewer can follow.
Include setup (env vars, DB state, fixtures) and expected outcomes.
-->

1. 
2. 
3. 

**Expected result:**

## Checklist

### Code Quality

- [ ] My code follows the project's style guidelines (ESLint, Prettier, `go fmt`, `go vet`)
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] Complex logic includes inline comments or references to design docs
- [ ] My changes generate no new warnings (lint, type check, compiler)

### AgentForge Specific

- [ ] Conditional classes use `cn()` utility (`import { cn } from "@/lib/utils"`)
- [ ] UI changes respect dark mode (`dark:` variants or `.dark` context)
- [ ] New user-facing strings are wrapped in i18n (`pnpm i18n:audit` passes)
- [ ] Zustand stores follow existing patterns (separate file per domain, typed selectors)
- [ ] API changes are reflected in frontend types / fetchers / stores
- [ ] Go handlers use structured logging (`internal/log`) rather than `log.Printf`
- [ ] Database migrations are backwards-compatible or accompanied by a migration plan
- [ ] New dependencies are justified and added to the correct `package.json` / `go.mod`

### Documentation & Communication

- [ ] I have made corresponding changes to the documentation (`docs/`, README, inline docs)
- [ ] Breaking changes are documented below with migration steps
- [ ] API changes are reflected in OpenAPI / API docs
- [ ] ADRs or design docs updated if architectural decisions changed

### Dependencies & External Impact

- [ ] Any dependent changes have been merged and published
- [ ] Lockfiles updated (`pnpm-lock.yaml`, `bun.lockb`, `go.sum`) via the standard package manager
- [ ] No unintended changes in `package.json` or `go.mod` (no version drift, no duplicate deps)

## Breaking Changes

<!--
If this is a breaking change, describe:
- What breaks
- Why it was necessary
- How consumers should migrate
Delete this section if not applicable.
-->

### What breaks

### Migration guide

1. 
2. 

## Performance Impact

<!--
If the change affects performance (latency, throughput, memory, bundle size),
include before/after numbers or a brief assessment.
Delete this section if not applicable.
-->

- **Bundle size impact:**
- **Runtime performance impact:**
- **Database query impact:**

## Security Considerations

<!--
If the change touches auth, secrets, input validation, file system, network, or WASM execution,
describe the security implications and mitigations.
Delete this section if not applicable.
-->

- [ ] No new attack surface introduced
- [ ] Input validated at system boundaries (API handlers, file uploads, user input)
- [ ] Secrets are not logged or exposed in error messages
- [ ] Auth/authorization checks are fail-closed

## Additional Notes

<!--
Add any additional context for reviewers:
- Known limitations or follow-up work
- Rollback considerations
- Deployment order dependencies
- Anything non-obvious about the implementation
-->

---

**Reviewer Focus:**
<!-- Optional: call out specific files, logic, or decisions you want extra eyes on -->

