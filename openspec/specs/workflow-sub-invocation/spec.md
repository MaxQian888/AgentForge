# workflow-sub-invocation Specification

## Purpose

Defines how the DAG workflow engine invokes child workflow runs from a `sub_workflow` node, including target resolution, input mapping, parent parking/resumption, parent↔child linkage persistence, recursion/cycle safety, and project-scope enforcement. This capability bridges the DAG engine with both DAG and legacy plugin child targets via the unified target-engine registry used by trigger dispatch.

## Requirements

### Requirement: DAG sub_workflow node invokes a child workflow run

The DAG workflow engine SHALL execute a `sub_workflow` node by resolving a target workflow from the node's configuration and starting a child workflow run. The target kind MUST be explicitly declared (`dag` or `plugin`) and MUST be resolved through the same target-engine registry used by the unified trigger dispatch router. A `sub_workflow` node MUST NOT silently fail or produce a no-op when the invocation cannot be performed.

#### Scenario: Sub_workflow node invokes a DAG child
- **WHEN** a `sub_workflow` node with `target_kind = 'dag'` and a valid `target_workflow_id` executes
- **THEN** the DAG engine starts a new `WorkflowExecution` for the referenced workflow
- **THEN** the parent node transitions to `awaiting_sub_workflow` status until the child reaches a terminal state

#### Scenario: Sub_workflow node invokes a legacy plugin child
- **WHEN** a `sub_workflow` node with `target_kind = 'plugin'` and a valid plugin reference executes
- **THEN** the plugin runtime starts a new workflow plugin run for the referenced plugin
- **THEN** the parent node transitions to `awaiting_sub_workflow` status until the child reaches a terminal state

#### Scenario: Sub_workflow node with unknown target kind fails explicitly
- **WHEN** a `sub_workflow` node declares a `target_kind` the target-engine registry does not support
- **THEN** the parent node fails with a structured error identifying the unknown target kind
- **THEN** no child run is started

### Requirement: Sub_workflow invocation maps inputs from parent context

A `sub_workflow` node SHALL accept an `input_mapping` configuration that templates the child run's initial seed against the parent run's data store, execution context, and any explicitly declared node inputs. The templating syntax MUST match the `{{$event.*}}` convention already used by triggers, extended to support `{{$parent.dataStore.*}}` and `{{$parent.context.*}}` references.

#### Scenario: Input mapping resolves parent data store values
- **WHEN** a `sub_workflow` node declares `input_mapping: {"task_id": "{{$parent.dataStore.previous_node.taskId}}"}` and the parent's data store contains that value
- **THEN** the child run is started with `seed.task_id` equal to the resolved value

#### Scenario: Input mapping with unresolvable path produces structured rejection
- **WHEN** a `sub_workflow` node declares an input mapping referencing a parent path that resolves to null
- **THEN** the engine emits a structured non-success outcome identifying the unresolved path
- **THEN** no child run is started

### Requirement: Parent run parks while child is executing and resumes on child terminal state

The DAG engine SHALL park the parent node while its child run is executing, and it SHALL resume the parent node when the child reaches a terminal state. Resumption MUST materialize the child's final outputs into the parent's `dataStore` keyed by the parent `sub_workflow` node's identifier.

#### Scenario: Parent resumes when child completes successfully
- **WHEN** a child run reaches the `completed` terminal state
- **THEN** the parent `sub_workflow` node is marked `completed`
- **THEN** the parent's `dataStore[<parent_node_id>]` contains the child's outputs keyed under a `subWorkflow.outputs` envelope
- **THEN** the parent execution advances to downstream edges

#### Scenario: Parent fails when child fails
- **WHEN** a child run reaches the `failed` terminal state
- **THEN** the parent `sub_workflow` node is marked `failed` with a structured error identifying the child run and its failure reason
- **THEN** the parent execution follows its declared failure handling (e.g. downstream failure edge or overall run failure)

#### Scenario: Parent fails when child is cancelled
- **WHEN** a child run reaches the `cancelled` terminal state (including external cancellation)
- **THEN** the parent `sub_workflow` node is marked `failed` with a structured error identifying the cancelled child run

#### Scenario: Parent remains parked when child is awaiting approval
- **WHEN** a child run is in the `paused` state awaiting approval or external event
- **THEN** the parent `sub_workflow` node remains in `awaiting_sub_workflow` status
- **THEN** the parent does not advance until the child resumes and reaches a terminal state

### Requirement: Parent↔child linkage is persisted and queryable

The system SHALL persist a parent↔child linkage record for every `sub_workflow` invocation that includes the parent execution identifier, parent node identifier, child engine kind, and child run identifier. Run read APIs (DAG execution read, plugin run read) MUST expose this linkage so callers can walk the invocation tree from either direction without inferring it from logs.

#### Scenario: Parent linkage is exposed on parent run read
- **WHEN** a caller reads a DAG execution containing at least one `sub_workflow` node that has invoked a child
- **THEN** the response DTO lists each parent↔child linkage: parent node id, child engine kind, child run id, linkage status

#### Scenario: Child exposes its parent linkage on child run read
- **WHEN** a caller reads a run (DAG or plugin) that was started as a child via `sub_workflow` invocation
- **THEN** the response DTO includes a back-reference to the parent execution id and parent node id

### Requirement: Recursion cycles are rejected at invocation time

The system SHALL reject a `sub_workflow` invocation whose target workflow is any ancestor of the current parent execution in the sub-workflow invocation chain. Detection MUST happen before the child run is started. A bounded maximum ancestor depth SHALL apply so cycle detection terminates in a predictable number of steps.

#### Scenario: Direct self-recursion is rejected
- **WHEN** a `sub_workflow` node's target workflow id equals the parent execution's workflow id
- **THEN** the engine emits a structured non-success outcome identifying the self-recursion
- **THEN** no child run is started

#### Scenario: Transitive recursion is rejected
- **WHEN** the invocation chain A → B → C → A would form at the current dispatch
- **THEN** the engine emits a structured non-success outcome identifying the transitive cycle A → B → C → A
- **THEN** no child run is started

#### Scenario: Depth-limited chain rejects at the configured maximum
- **WHEN** the invocation chain depth would exceed the configured maximum sub-workflow depth
- **THEN** the engine emits a structured non-success outcome identifying the depth-limit violation
- **THEN** no child run is started

### Requirement: Sub_workflow targets are same-project only

A `sub_workflow` node MUST reference a target workflow that belongs to the same project as the parent workflow. Cross-project references MUST be rejected at both node-config save time and at invocation time with a structured non-success outcome.

#### Scenario: Cross-project DAG target is rejected at save time
- **WHEN** a workflow save stores a `sub_workflow` node referencing a DAG workflow from a different project
- **THEN** the save endpoint returns a structured rejection identifying the cross-project mismatch

#### Scenario: Cross-project plugin target is rejected at dispatch time
- **WHEN** a `sub_workflow` node executes with a plugin target whose project scope does not match the parent workflow's project
- **THEN** the engine emits a structured non-success outcome identifying the cross-project mismatch
- **THEN** no child run is started

### Requirement: Parent kind discriminator on sub-workflow linkage supports plugin parents

The parent↔child linkage persistence SHALL identify the parent kind on every linkage record. Supported parent kinds are `dag_execution` (a DAG workflow execution invoking a sub-workflow through a `sub_workflow` node) and `plugin_run` (a legacy workflow plugin run invoking a child through its `workflow` step action). Parent-kind MUST be persisted so resume and cancellation hooks can route terminal-state notifications to the correct parent engine.

#### Scenario: Plugin parent invoking DAG child persists plugin_run parent kind
- **WHEN** a legacy plugin run starts a DAG workflow as a child through its `workflow` step
- **THEN** the parent↔child linkage row persists `parent_kind='plugin_run'` and identifies the plugin run as the parent

#### Scenario: DAG parent invoking any child persists dag_execution parent kind
- **WHEN** a DAG `sub_workflow` node starts a child run of either engine
- **THEN** the parent↔child linkage row persists `parent_kind='dag_execution'` and identifies the DAG execution as the parent

### Requirement: Recursion guard covers cross-engine invocation chains

The sub-workflow recursion guard SHALL walk across engine kinds when computing an invocation chain. A plugin run whose ancestor chain contains a DAG execution referencing the same plugin MUST be rejected. A DAG execution whose ancestor chain contains a plugin run referencing the same DAG workflow MUST be rejected. The bounded maximum depth applies uniformly across engines.

#### Scenario: Cross-engine cycle DAG → plugin → DAG is rejected
- **WHEN** a DAG workflow would be invoked as a child of a plugin run whose ancestor chain contains the same DAG workflow
- **THEN** the invocation is rejected with a structured non-success outcome identifying the cross-engine cycle
- **THEN** no child run is started

#### Scenario: Cross-engine cycle plugin → DAG → plugin is rejected
- **WHEN** a plugin workflow would be invoked as a child of a DAG execution whose ancestor chain contains the same plugin
- **THEN** the invocation is rejected with a structured non-success outcome identifying the cross-engine cycle
- **THEN** no child run is started
