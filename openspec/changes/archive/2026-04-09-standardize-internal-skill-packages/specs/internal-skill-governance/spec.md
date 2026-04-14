## ADDED Requirements

### Requirement: Repository maintains an explicit internal skill registry
The repository SHALL maintain an explicit internal skill registry for repo-managed skills. Each registry entry MUST identify the skill id, its family/profile, its canonical root, its provenance (`repo-authored`, `upstream-sync`, or `generated-mirror`), and any declared mirror targets or lockfile references needed to validate that skill truthfully.

#### Scenario: Canonical roots are explicit for each internal skill family
- **WHEN** a maintainer inspects a repo-managed skill such as `skills/react`, `.agents/skills/echo-go-backend`, or `.codex/skills/openspec-propose`
- **THEN** the registry identifies which path is the canonical source for that skill
- **THEN** any mirrored copies are represented as mirrors of that canonical source instead of being treated as separate authoring truths

#### Scenario: Unregistered skill copies do not become canonical implicitly
- **WHEN** a duplicate or ad-hoc skill copy exists outside the registry's declared canonical root or mirror targets
- **THEN** the repository MUST NOT treat that copy as an authoritative maintained internal skill
- **THEN** verification reports the mismatch explicitly instead of silently accepting the extra copy

### Requirement: Internal skill packages follow profile-aware minimum contracts
The repository SHALL validate internal skill packages against profile-aware minimum contracts instead of a single one-size-fits-all template. Repo-authored skills MUST satisfy the required metadata and file layout for their declared profile, while `upstream-sync` skills MAY use controlled exceptions only when the registry and lockfile provenance make those exceptions explicit.

#### Scenario: Repo-authored skill fails when required metadata or files are missing
- **WHEN** a repo-authored built-in runtime skill or repo-assistant skill is missing required frontmatter, has unreadable required companion files, or violates its declared profile layout
- **THEN** internal skill verification fails for that skill
- **THEN** the failure explains which profile rule was violated

#### Scenario: Upstream-synced skill keeps approved structural exceptions
- **WHEN** an upstream-synced internal skill intentionally preserves an upstream-specific detail such as a locked source reference or accepted filename variance
- **THEN** the skill remains valid only if the registry marks it as `upstream-sync` and the corresponding provenance record exists in `skills-lock.json`
- **THEN** verification accepts the skill as a controlled exception instead of forcing it to masquerade as a repo-authored layout

### Requirement: Workflow mirror skills stay synchronized with their canonical source
The repository SHALL define workflow-oriented mirrored skills as generated or synchronized copies of one canonical package. Mirror targets MUST stay byte-for-byte or content-equivalent with the declared canonical source according to the repository's sync contract.

#### Scenario: Canonical workflow skill can refresh declared mirrors
- **WHEN** a maintainer updates a canonical workflow skill package that declares mirror targets
- **THEN** the repository provides a deterministic sync path to refresh the declared mirrors from that canonical source
- **THEN** maintainers do not need to hand-edit every mirror copy independently

#### Scenario: Mirror drift is reported explicitly
- **WHEN** a declared mirror target no longer matches the canonical workflow skill content
- **THEN** internal skill verification fails or reports drift explicitly for that mirror target
- **THEN** the mismatch is attributed to the canonical skill id rather than being hidden inside a generic file-diff failure

### Requirement: Repository exposes shared internal skill verification
The repository SHALL expose a shared verification entrypoint for internal skills that validates registry coverage, profile rules, provenance, and mirror synchronization across the maintained internal skill families. Existing narrow verifiers MAY remain, but they MUST derive their checks from the same internal governance truth where applicable.

#### Scenario: Shared internal verification detects governance drift
- **WHEN** a maintainer runs the repository's internal skill verification command
- **THEN** the command validates registry-declared skills across `skills/*`, `.agents/skills/*`, and any declared workflow mirrors
- **THEN** it fails explicitly on missing canonical roots, profile violations, broken provenance, or mirror drift

#### Scenario: Built-in runtime verification remains a governed subset
- **WHEN** a maintainer runs the built-in skill verification flow used by marketplace and runtime-facing assets
- **THEN** that flow still verifies the built-in runtime subset under `skills/*`
- **THEN** the underlying contract remains aligned with the shared internal skill governance rules instead of diverging into a separate hidden standard

### Requirement: Maintainers have a documented internal skill authoring workflow
The repository SHALL document how maintainers add, update, sync, and verify internal skills. The guide MUST describe profile selection, required metadata, allowed optional parts, provenance expectations, and the verification or sync commands that keep internal skills compliant.

#### Scenario: Maintainer can add a new internal skill using the documented workflow
- **WHEN** a maintainer creates a new repo-authored internal skill
- **THEN** the repository documentation explains which profile to choose, which files and metadata are required, and which verification commands to run before considering the skill complete
- **THEN** the maintainer does not need to infer the workflow from scattered examples alone

#### Scenario: Maintainer can refresh an upstream-synced skill safely
- **WHEN** a maintainer updates an internal skill whose source of truth is an upstream lockfile-backed import
- **THEN** the documentation explains how to update the provenance record, what structural exceptions remain allowed, and how to verify the refreshed skill package
- **THEN** upstream refreshes stay reproducible instead of becoming undocumented manual edits
