# AgentForge Motion Guidelines

Use this reference when motion touches reusable product shells or desktop-adjacent UI.

## Favor

- Short opacity and transform transitions
- Motion that clarifies selection, reveal, expand, or loading state
- Reuse of existing transition tokens from `app/globals.css`

## Avoid

- Constant looping animation on dense dashboard surfaces
- Motion that interferes with drag regions, titlebar controls, or resize handles
- Layout-thrashing animation for sidebars, rails, and sheets

## Check Shared Shells

- `components/layout/desktop-window-frame.tsx`
- `components/ui/sidebar.tsx`
- `components/ui/sheet.tsx`
- `components/roles/role-workspace.tsx`
