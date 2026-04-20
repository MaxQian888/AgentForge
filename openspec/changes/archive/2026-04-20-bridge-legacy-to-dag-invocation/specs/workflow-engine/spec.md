## ADDED Requirements

### Requirement: Legacy workflow action can target a DAG child workflow

The current workflow step router SHALL accept an optional `targetKind` discriminator on the trigger payload (or step config) of the `workflow` action, with supported values `plugin` (default, existing behavior) and `dag`. When `targetKind='dag'` is declared, the router MUST resolve the child through the target-engine registry and start a DAG workflow execution as the child run. Omitting `targetKind` MUST preserve the existing `pluginId` / `plugin_id` resolution path unchanged.

#### Scenario: Workflow action defaults to plugin child when discriminator omitted
- **WHEN** a legacy `workflow` step provides `trigger.pluginId` and omits `targetKind`
- **THEN** the step router starts a child workflow plugin run exactly as it does today
- **THEN** the step output envelope includes `child_run_id`, `child_plugin`, and `status`

#### Scenario: Workflow action starts a DAG child when targetKind='dag'
- **WHEN** a legacy `workflow` step provides `targetKind='dag'` and a valid `targetWorkflowId`
- **THEN** the step router starts a DAG workflow execution as the child run
- **THEN** the step output envelope includes `child_run_id`, `child_engine='dag'`, `child_workflow_id`, and `status`

#### Scenario: Workflow action rejects unknown targetKind
- **WHEN** a legacy `workflow` step provides `targetKind` with an unsupported value
- **THEN** the step router returns a structured error identifying the unknown target kind
- **THEN** no child run is started

### Requirement: Legacy workflow step parks when DAG child is long-running

When the child is a DAG workflow, the legacy `workflow` step SHALL park the parent plugin run's step in an `awaiting_sub_workflow` status until the DAG child reaches a terminal state. The parent plugin step MUST resume when the DAG child completes, fails, or is cancelled. When the child is a plugin, the existing synchronous behavior of the `workflow` action is preserved.

#### Scenario: DAG child in progress parks the parent plugin step
- **WHEN** the `workflow` step starts a DAG child that has not yet reached a terminal state
- **THEN** the parent plugin run's step transitions to `awaiting_sub_workflow` status
- **THEN** the plugin run's overall status reflects the parked step and does not falsely report completion

#### Scenario: DAG child completion resumes the parent plugin step
- **WHEN** the DAG child reaches the `completed` terminal state
- **THEN** the parent plugin step transitions to `completed`
- **THEN** subsequent plugin steps execute normally

#### Scenario: DAG child failure fails the parent plugin step
- **WHEN** the DAG child reaches the `failed` terminal state
- **THEN** the parent plugin step transitions to `failed` with a structured reason identifying the child run

#### Scenario: Parent plugin cancellation cascades to DAG child
- **WHEN** the parent plugin run is cancelled while its `workflow` step is parked with a DAG child in progress
- **THEN** the DAG child is cancelled or detached from the linkage
- **THEN** the parent plugin step reports a cancellation terminal state
