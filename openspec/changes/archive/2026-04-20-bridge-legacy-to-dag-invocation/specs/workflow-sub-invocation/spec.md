## ADDED Requirements

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
