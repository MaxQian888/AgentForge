## ADDED Requirements

### Requirement: Repository ships official built-in control tool starters
The repository SHALL maintain official built-in ToolPlugin starters named `task-control`, `review-control`, and `workflow-control` as part of the repository-owned built-in plugin catalog. Each starter MUST resolve to a maintained ToolPlugin manifest under `plugins/tools/<id>/manifest.yaml`, remain installable through the official built-in install flow, and declare a platform-native control scope instead of generic external helper semantics.

#### Scenario: Control tool starters are bundled as official built-ins
- **WHEN** the current checkout exposes the official built-in plugin catalog
- **THEN** `task-control`, `review-control`, and `workflow-control` are listed as built-in ToolPlugin starters with valid manifest-backed entries
- **THEN** the platform treats them as official repo-owned starters instead of inferred local examples

#### Scenario: Missing control tool manifest blocks official starter status
- **WHEN** one of the declared control tool starters does not resolve to a valid ToolPlugin manifest
- **THEN** repository verification fails for that starter
- **THEN** the platform MUST NOT treat the drifted starter as an official built-in control tool

### Requirement: Task control tool proxies through existing task control-plane seams
The `task-control` built-in tool SHALL expose typed MCP operations for task lookup, task decomposition, dispatch or assignment helpers, and task status or progress inspection by proxying through the existing task control-plane contracts. The tool MUST reject requests that omit required task identity or request operations outside the manifest-declared task control scope.

#### Scenario: Task control tool returns task or decomposition data
- **WHEN** an operator or agent invokes `task-control` with a valid task identifier and a supported read or decomposition action
- **THEN** the tool returns the task summary, decomposition result, or dispatch-related summary produced by the existing task control-plane seam

#### Scenario: Unsupported task operation is rejected before execution
- **WHEN** the caller omits required task identity or requests a task mutation not declared by the tool's manifest
- **THEN** the tool rejects the request with a validation error before invoking the underlying task seam

### Requirement: Review control tool exposes bounded review trigger and inspection actions
The `review-control` built-in tool SHALL expose typed MCP operations for triggering supported review runs, retrieving review summaries, and inspecting review findings or current decision state through the existing review control-plane seams. The tool MUST preserve review identifiers, risk or status metadata, and next-step guidance in its structured responses.

#### Scenario: Review control tool triggers a supported review run
- **WHEN** a caller invokes `review-control` with a valid payload for a review trigger supported by the current review APIs
- **THEN** the tool routes the request through the existing review seam and returns the created or matched review metadata

#### Scenario: Review control tool returns current review status
- **WHEN** a caller requests the status or details of an existing review through `review-control`
- **THEN** the tool returns the structured review summary, findings metadata, and current decision state without requiring raw control-plane JSON inspection

### Requirement: Workflow control tool exposes workflow run start and inspection actions
The `workflow-control` built-in tool SHALL expose typed MCP operations for starting supported workflow runs, retrieving workflow run detail, and listing recent run history for official or installed workflow starters through the existing workflow control-plane seams. The tool MUST preserve workflow identifiers, current state, process mode, and latest step metadata in its responses.

#### Scenario: Workflow control tool starts a workflow run
- **WHEN** a caller invokes `workflow-control` with a valid workflow starter identifier and supported trigger payload
- **THEN** the tool routes the start request through the workflow control-plane seam and returns the created workflow run metadata

#### Scenario: Workflow control tool returns recent run history
- **WHEN** a caller requests recent workflow runs for a valid workflow starter through `workflow-control`
- **THEN** the tool returns recent run summaries including current state, terminal outcome when available, and latest step metadata
