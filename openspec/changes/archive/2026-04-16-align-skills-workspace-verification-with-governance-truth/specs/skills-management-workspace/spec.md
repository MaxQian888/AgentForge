## MODIFIED Requirements

### Requirement: Skills workspace actions remain truthful and explicitly bounded
The `/skills` workspace SHALL expose only the supported management actions for the selected skill or family and MUST report blocked or unsupported actions explicitly. Verification actions MUST render verifier-grade diagnostics derived from the shared governance rule set for the requested skill set instead of only reflecting precomputed inventory health. The workspace SHALL NOT collapse a structured per-skill verification result into a generic success or failure toast.

#### Scenario: Operator runs verification from the workspace
- **WHEN** the operator triggers internal skill verification from `/skills` (clicks "Verify Internal Skills")
- **THEN** the store calls `POST /api/v1/skills/verify` without a family filter and stores the response in `lastVerificationResult`
- **THEN** the workspace renders a "Latest verification" card with per-skill result rows, each showing skill id, family label, status badge, and any issues with their code, message, and target path
- **THEN** the UI updates each affected skill's health state in the inventory list by re-fetching after the verify call completes

#### Scenario: Operator runs built-in verification from the workspace
- **WHEN** the operator triggers built-in skill verification from `/skills` (clicks "Verify Built-in Skills", visible only when the selected skill's `supportedActions` includes `"verify-builtins"`)
- **THEN** the store calls `POST /api/v1/skills/verify` with `{"families": ["built-in-runtime"]}`
- **THEN** the workspace receives bundle-specific and runtime-package verification results for the built-in subset
- **THEN** the operator can see which built-in skills failed because of bundle metadata or package-contract issues (e.g., `built_in_bundle_missing_category`, `built_in_agent_config_missing`) instead of only seeing family-level success/failure

#### Scenario: Operator syncs workflow mirrors
- **WHEN** the operator triggers mirror sync from `/skills` (clicks "Sync Mirrors", visible only when the selected skill's `supportedActions` includes `"sync-mirrors"`)
- **THEN** the store calls `POST /api/v1/skills/sync-mirrors`
- **THEN** the workspace re-fetches inventory after the sync completes, reflecting updated mirror health states
- **THEN** the skill detail includes `mirrorTargets[]` listing declared target paths so the operator can correlate sync results with the specific paths updated

#### Scenario: Unsupported management actions stay blocked
- **WHEN** the selected skill does not support a requested action (e.g., sync-mirrors for a built-in-runtime skill, refresh-upstream for a repo-assistant skill)
- **THEN** the workspace shows the blocked actions from `skill.blockedActions[]` with the stable reason string in the detail pane
- **THEN** the Sync Mirrors and Verify Built-in Skills buttons are conditionally rendered based on `supportedActions` membership — blocked actions are never presented as clickable controls, and the reason for blocking is always visible in the detail card

#### Scenario: Workspace filter controls are truthful and bounded
- **WHEN** the operator filters by family or health status in the skill list
- **THEN** the filter applies client-side against the already-fetched inventory without triggering new API calls
- **THEN** the search input matches against skill id, canonical root, and family label
- **THEN** the workspace shows an explicit empty state when no skills match the active filters
