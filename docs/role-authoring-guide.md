# Role Authoring Guide

This guide is for operators who create or refine AgentForge roles through the dashboard or YAML files.

## Recommended Flow

1. Start from an existing template role when possible.
2. Set `extends` if the new role is truly a child role instead of a forked copy.
3. Fill identity and advanced identity fields first.
4. Add packages, allowed tools, shared knowledge, and governance cues.
5. Use preview to inspect the effective manifest and execution profile.
6. Use sandbox to run one bounded prompt probe before saving.

## Workspace Layout

- `Role Library`: compare existing roles, open a draft for editing, or start a new role.
- `Setup`: choose template reuse, inheritance, and confirm metadata before deeper edits.
- `Identity`: align role title, goal, prompt, persona, language, and response style.
- `Capabilities`: define packages, tools, skills, and execution limits.
- `Knowledge`: bind repository, document, pattern, and shared-source context.
- `Governance`: review security, collaboration, and trigger expectations.
- `Review`: inspect execution summary, YAML preview, preview output, and sandbox output before saving.

On wider layouts these surfaces may appear side by side. On narrower layouts the role library and review surfaces can move behind dedicated entry points, but the create -> edit -> preview flow should remain intact.

## What Each Section Is For

- `Identity`: the core job, goal, backstory, and primary prompt intent.
- `Advanced Identity`: persona, language, personality, and response style.
- `Capabilities`: packages, allowed tools, external tools, skills, and execution limits.
- `Knowledge`: repo/doc/pattern references plus shared knowledge sources.
- `Security`: profile, permission mode, path rules, output filters, and review requirements.
- `Collaboration`: who this role can delegate to and who can delegate to it.
- `Triggers`: lightweight activation cues such as `pr_created -> auto_review`.

## Preview Vs Sandbox

- Preview:
  - resolves inheritance
  - shows effective manifest
  - shows execution profile
  - does not call a model

- Sandbox:
  - does everything preview does
  - checks runtime readiness
  - optionally runs one bounded prompt probe
  - does not create a task, worktree, or persisted agent run

## Practical Tips

- Keep `goal` and `system_prompt` aligned. If they disagree, sandbox output becomes harder to trust.
- Use `packages` for reusable capability groups and `skills` for explicit opt-in knowledge references.
- Prefer `output_filters` and path restrictions over long negative instructions in the prompt.
- Add only triggers you can explain clearly to another operator.
- If preview shows surprising inherited values, revisit `extends` before saving.
- If the workspace is in a compact layout, reopen the review panel before saving so YAML and preview cues stay visible.
