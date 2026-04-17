## MODIFIED Requirements

### Requirement: Repository exposes shared internal skill verification
The repository SHALL expose a shared verification entrypoint for internal skills that validates registry coverage, profile rules, family-root placement, provenance/lock, built-in runtime bundle rules, agent-config contract, package existence and frontmatter, and mirror synchronization across the maintained internal skill families. The operator-facing verification API and maintainer CLI verification flows MUST derive from the same governed rule set and issue taxonomy. Shared issue codes MUST be stable string identifiers (e.g., `duplicate_skill_id`, `built_in_bundle_missing_category`, `unregistered_skill_package`) that callers can assert without coupling to message text.

#### Scenario: Shared internal verification detects governance drift
- **WHEN** a maintainer runs the repository's internal skill verification command
- **THEN** the command validates registry-declared skills across `skills/*`, `.agents/skills/*`, `.codex/skills/*`, and declared workflow mirror targets
- **THEN** it fails explicitly on duplicate registry ids/roots, invalid family or sourceType values, missing verification profile, family-root placement violations, missing lockKey or lock entries for upstream-sync skills, missing built-in bundle entries or mismatched bundle metadata, missing canonical root paths, missing or invalid SKILL.md frontmatter (name/description), missing or invalid agent configs for built-in-runtime skills, unregistered governed skill packages, or mirror drift

#### Scenario: Built-in runtime verification remains a governed subset
- **WHEN** a maintainer runs the built-in skill verification flow (i.e., `Verify(Families: ["built-in-runtime"])`)
- **THEN** that flow verifies the built-in runtime subset under `skills/*`
- **THEN** it reports bundle metadata failures (`built_in_bundle_missing_category`, `built_in_bundle_missing_tags`, `built_in_bundle_root_mismatch`) and agent-config failures (`built_in_agent_config_missing`, `invalid_agent_yaml`) that do not appear in broader inventory health summaries
- **THEN** missing built-in bundle entries (`missing_registry_entry`) are reported for bundle IDs that have no corresponding registry declaration

### Requirement: Governed skill inventory and actions are exposed through stable operator APIs
The repository SHALL expose stable operator-facing APIs for governed internal skills using the same registry, provenance, bundle, and mirror truth that powers internal skill verification. These APIs MUST provide machine-readable inventory, detail, verification, and sync responses. Operator verification responses MUST report the same classes of governance failures that the maintainer verification flows detect for the requested skill set. Per-skill results MUST include `skillId`, `family`, `status`, and `issues[]` with stable `code`, `message`, `targetPath`, `family`, and `sourceType` fields.

#### Scenario: Inventory API reflects registry-declared skill governance truth
- **WHEN** the operator-facing skills inventory API (`GET /api/v1/skills`) is requested
- **THEN** it returns every registry-declared skill with its family, verification profile, canonical root, source type, docs reference, lock key/provenance, mirror targets, bundle alignment (member, category, tags, featured), per-family supported and blocked actions, and resolved consumer surface handoffs
- **THEN** health status and issues are derived from the same `evaluate()` pass as verification — not separately inferred from frontend heuristics

#### Scenario: Verification API reports per-skill governance diagnostics
- **WHEN** the operator-facing verification API (`POST /api/v1/skills/verify`) is triggered for all skills or a supported family subset
- **THEN** it evaluates governed skills against the same registry, profile, provenance, bundle, and mirror rules used by internal skill verification
- **THEN** the response reports `{ok: bool, results: [{skillId, family, status, issues[]}]}` where each issue carries a stable `code`, human-readable `message`, and optional `targetPath`
- **THEN** `ok` is false if any result has status `blocked` or `drifted`

#### Scenario: Built-in verification API preserves built-in-only governance failures
- **WHEN** the operator-facing verification API is triggered with `{"families": ["built-in-runtime"]}`
- **THEN** it reports built-in bundle metadata failures and runtime package-contract violations using the same governed subset rules as the maintainer built-in verification flow
- **THEN** the operator can distinguish bundle-specific failures from broader registry or mirror drift issues by inspecting the per-issue `code` field

#### Scenario: Mirror sync API remains bounded to declared workflow mirrors
- **WHEN** the operator-facing mirror sync API (`POST /api/v1/skills/sync-mirrors`) is triggered
- **THEN** it only updates registry-declared workflow-mirror targets from their canonical `SKILL.md` source
- **THEN** the response identifies which targets changed (`updatedTargets[]`) and includes post-sync per-skill verification results
- **THEN** the sync is unavailable for skills outside the `workflow-mirror` family, which is communicated via `blockedActions` in the skill detail response rather than a sync API error
