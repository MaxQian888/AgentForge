## 1. Data Contracts And State

- [x] 1.1 Define the dashboard summary contract and shared frontend types for task, cost, review, activity, and team insight sections.
- [x] 1.2 Add a project member/team store that normalizes the documented `members` model for human and agent entries.
- [x] 1.3 Wire the dashboard summary loader and team store to the appropriate API endpoints with loading, empty, and error handling.

## 2. Dashboard Insights UI

- [x] 2.1 Replace the current static dashboard summary cards with structured insight sections for progress, agents, reviews, weekly cost, and team capacity.
- [x] 2.2 Implement recent activity and risk signal panels on the dashboard, including explicit empty states and inline retry handling for partial failures.
- [x] 2.3 Add drill-down navigation from dashboard insight cards and activity items into the relevant projects, agents, reviews, or team surfaces with scope preserved.

## 3. Team Management UI

- [x] 3.1 Add a dedicated dashboard `team` surface and navigation entry for project-level team management.
- [x] 3.2 Render a unified team roster that distinguishes human and agent members while showing role, status, and collaboration metadata.
- [x] 3.3 Implement member creation and update flows for editable team fields, with the roster refreshing in-place after successful changes.
- [x] 3.4 Surface member workload context and navigation into related task or agent detail views from the team page.

## 4. Integration And Verification

- [x] 4.1 Ensure dashboard team insights and the team management page reuse the same member identities and summary fields without duplicating transformation logic.
- [x] 4.2 Add or update tests covering dashboard insight rendering, empty/error states, and team management interactions.
- [x] 4.3 Run lint and targeted UI verification for the dashboard and team-management flows.
