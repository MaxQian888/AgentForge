## 1. Cross-Page Team Context Contract

- [x] 1.1 Define the supported `project` / `member` / `focus` query-param contract for team-management drill-down routes and document which destination pages must consume each parameter.
- [x] 1.2 Update team-management and dashboard source links so they only emit supported route parameters for `/team`, `/teams`, `/project`, and `/agents`.
- [x] 1.3 Implement query-param consumption in the relevant destination pages so member investigation links preserve project/member context instead of being ignored.

## 2. Project Team Roster Data Flow

- [x] 2.1 Refactor `/team` page loading so project-scoped member data is sourced from the members contract instead of depending on dashboard summary as the primary read path.
- [x] 2.2 Keep roster workload enrichment consistent with existing task, agent, and activity data while preserving project scope and post-write refresh behavior.
- [x] 2.3 Add explicit loading, error, empty, and project-switch states to the team roster flow so stale members are not shown under the wrong project.
- [x] 2.4 Surface member status and recent collaboration cues in the roster alongside workload summaries and drill-down actions.

## 3. Agent Team Collection And Detail Flow

- [x] 3.1 Make `team-store` and `/teams` always operate with an explicit project scope when listing team runs.
- [x] 3.2 Add project selection, empty state, error state, and retry behavior to `/teams` so list failures are not rendered as a successful empty collection.
- [x] 3.3 Split `/teams/detail` into loading, loaded-not-found, and loaded-success states so async fetches do not flash a false “Team not found”.
- [x] 3.4 Ensure team detail actions and summary rendering continue to work when opening a team directly from the collection or a deep link.

## 4. Team Strategy And Runtime Contract Alignment

- [x] 4.1 Canonicalize frontend team startup requests to use the backend-supported strategy identifiers and preserve resolved `runtime`, `provider`, and `model` values.
- [x] 4.2 Add legacy alias normalization for persisted `planner_coder_reviewer` team records so existing teams remain readable and actionable.
- [x] 4.3 Update team list/detail/pipeline surfaces to display the real strategy label, runtime identity, and relevant queue / retry / phase state instead of assuming a single hard-coded pipeline.
- [x] 4.4 Align store normalization and any backend DTO handling needed to keep new and legacy team summaries consistent across list and detail responses.

## 5. Verification

- [x] 5.1 Add or update route/page tests covering member drill-down context preservation across `/team`, `/project`, and `/agents`.
- [x] 5.2 Add or update component/store tests for `/team` roster loading/error/project-switch behavior and `/teams` list/detail loading semantics.
- [x] 5.3 Add or update startup/store normalization tests covering canonical strategy IDs, legacy alias compatibility, and resolved runtime/provider/model display.
- [x] 5.4 Run focused frontend and backend verification for the affected team-management surfaces and confirm the new change remains apply-ready.
