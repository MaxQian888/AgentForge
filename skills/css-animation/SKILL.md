---
name: CSS Animation
description: Use when adding or refining motion, transitions, or interaction polish for AgentForge UI without sacrificing clarity, performance, accessibility, or shared shell behavior.
requires:
  - skills/react
tools:
  - code_editor
  - browser_preview
---

# CSS Animation

Use motion to reinforce hierarchy and state change, not to decorate otherwise weak UI.

## Guardrails

- Animate state change, hierarchy, and orientation. Do not add movement that does not explain anything to the operator.
- Prefer opacity and transform-based transitions over layout-thrashing animation.
- Keep durations short, consistent, and compatible with dense dashboard workflows.
- Respect reduced-motion expectations by keeping every animated state understandable without the animation.
- Avoid motion that interferes with inputs, drag regions, panel resizing, or desktop window chrome interactions.

## Implementation

- Attach motion to an existing UI state transition such as loading to ready, dialog enter and exit, panel expansion, selection change, or list reveal.
- Reuse Tailwind utilities, existing transition tokens, and current component patterns before adding custom keyframes.
- Keep idle surfaces calm. Avoid continuous looping effects unless the motion communicates live status and remains subtle.
- Pair any richer animation with a clear static fallback state so the UI still reads correctly when motion is disabled or skipped.

## Verification

- Check desktop and browser layouts for clipped content, focus loss, accidental hover traps, and jank during rerenders.
- Verify that the interaction still feels responsive on shared dashboard shells, sidebars, sheets, and dialogs.

## References

- Read [references/agentforge-motion-guidelines.md](references/agentforge-motion-guidelines.md) when the motion work spans shared shells, drag regions, or reusable interaction patterns.
