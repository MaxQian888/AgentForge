# Role Authoring Guide

This guide is for operators who create or refine AgentForge roles through the dashboard or YAML files.

## Recommended Flow

1. Start from an existing template role when possible.
2. Set `extends` if the new role is truly a child role instead of a forked copy.
3. Fill identity and advanced identity fields first.
4. Add packages, allowed tools, shared or private knowledge, memory settings, and governance cues.
5. Use preview to inspect the effective manifest and execution profile.
6. Use sandbox to run one bounded prompt probe before saving.

## Workspace Layout

- `Role Library`: compare existing roles, open a draft for editing, or start a new role.
- `Setup`: choose template reuse, inheritance, and confirm metadata before deeper edits.
- `Identity`: align role title, goal, prompt, persona, language, and response style.
- `Capabilities`: define packages, tools, skills, custom settings, MCP servers, and execution limits.
- `Knowledge`: bind repository, document, pattern, shared-source, private-source, and memory context.
- `Governance`: review security, collaboration, and trigger expectations.
- `Review`: inspect execution summary, YAML preview, preview output, sandbox output, and override rules before saving.

On wider layouts these surfaces may appear side by side. On narrower layouts the role library and review surfaces can move behind dedicated entry points, but the create -> edit -> preview flow should remain intact.

## What Each Section Is For

- `Identity`: the core job, goal, backstory, and primary prompt intent.
- `Advanced Identity`: persona, language, personality, and response style.
- `Capabilities`: packages, allowed tools, external tools, MCP servers, custom settings, skills, and execution limits.
- `Knowledge`: repo/doc/pattern references plus shared knowledge sources, private knowledge sources, and memory settings.
- `Security`: profile, permission mode, path rules, output filters, and review requirements.
- `Collaboration`: who this role can delegate to and who can delegate to it.
- `Triggers`: lightweight activation cues such as `pr_created -> auto_review`.
- `Overrides`: open-ended patch rules entered through the controlled JSON editor in the Review stage.

## Preview Vs Sandbox

- Preview:
  - resolves inheritance
  - shows effective manifest
  - shows execution profile
  - keeps stored-only advanced fields visible so you can confirm they were preserved
  - does not call a model

- Sandbox:
  - does everything preview does
  - checks runtime readiness
  - optionally runs one bounded prompt probe
  - does not create a task, worktree, or persisted agent run

## Practical Tips

- Keep `goal` and `system_prompt` aligned. If they disagree, sandbox output becomes harder to trust.
- Use `packages` for reusable capability groups and `skills` for explicit opt-in knowledge references.
- Prefer catalog-backed repo-local skills when the workspace offers them. Manual skill paths remain valid for staged or future references, but the review context will flag them as unresolved until the current repository catalog can resolve them.
- When a catalog-backed skill is selected, review its direct dependencies and declared tools instead of only the top-level path. The authoring flow now surfaces both so you can see what the role is implicitly pulling in.
- Use `auto_load` only for skills that must become runtime prompt context immediately. Non-auto-load skills stay visible as on-demand inventory and are not injected into the current execution prompt by default.
- Use `custom_settings` for role-specific execution hints that should stay structured, not buried in prompt text.
- Use MCP server rows for named hosts that belong in the role definition, not for temporary per-run experiments.
- Use private knowledge rows for operator-only context and shared rows for reusable sources another operator should also understand.
- Prefer `output_filters` and path restrictions over long negative instructions in the prompt.
- Add only triggers you can explain clearly to another operator.
- Treat `overrides` as a surgical tool. If you cannot explain the patch path and save impact, go back and simplify the role instead of stacking more overrides.
- If preview shows surprising inherited values, revisit `extends` before saving.
- If preview or sandbox shows a blocking skill diagnostic, check whether an `auto_load` skill or one of its dependencies is missing from `skills/**/SKILL.md`, or whether the role does not currently cover the skill's declared tool requirements. Auto-load failures now block execution-facing projection and launch.
- If the Skills section marks a path as an unresolved manual reference, that means the role will still preserve the skill path in canonical YAML, but the current repository catalog cannot explain that skill yet. If that skill is not auto-loaded, it remains warning-only inventory context.
- Current sample roles in this repository still use legacy runtime tool names such as `Read`, `Edit`, `Write`, and `Bash`. The compatibility checker normalizes those values, plus package and framework hints, into authoring capabilities such as `code_editor`, `terminal`, and `browser_preview`; you do not need to rewrite existing role YAML just to satisfy the new skill diagnostics.
- If the Review rail shows transitive loaded skills, those came from dependency closure rather than direct skill rows. Treat them as part of the effective role-skill tree when deciding whether the role is safe to launch.
- If preview or sandbox still shows a field under stored-only advanced sections, that means the field was preserved in canonical YAML but is not part of the current execution profile.
- If the workspace is in a compact layout, reopen the review panel before saving so YAML and preview cues stay visible.
