## ADDED Requirements

### Requirement: Official tool and workflow starters declare core-flow catalog metadata
The official built-in plugin bundle SHALL record structured starter-catalog metadata for official ToolPlugin and WorkflowPlugin starters in addition to the existing docs, verification, and readiness fields. Each official starter entry MUST declare at least one `coreFlow`, a `starterFamily` or equivalent starter classification, the `dependencyRefs` needed to validate role or service dependencies, and the `workspaceRefs` or handoff metadata needed to guide operators to the appropriate surface.

#### Scenario: Starter entry maps to a platform core flow
- **WHEN** an official built-in ToolPlugin or WorkflowPlugin starter is listed in the built-in bundle
- **THEN** its bundle entry declares which platform core flow it belongs to, such as task delivery, review automation, or workflow operations
- **THEN** the entry also exposes the dependency and workspace metadata needed to explain how that starter should be used

#### Scenario: Verification rejects incomplete starter catalog metadata
- **WHEN** an official ToolPlugin or WorkflowPlugin starter omits required starter-catalog metadata such as `coreFlow`, `dependencyRefs`, or `workspaceRefs`
- **THEN** repository verification fails for that bundle entry before it is treated as a valid official starter
- **THEN** the failure identifies which starter entry and metadata field drifted

### Requirement: Core starters remain distinguishable from generic helper built-ins
The official built-in plugin bundle SHALL distinguish platform-native core starters from generic helper built-ins so discovery surfaces can recommend the correct entry point without hiding helper tools. Generic helpers MAY remain official built-ins, but they MUST NOT be presented as equivalent to platform-native core starters when starter-family metadata marks the difference.

#### Scenario: Core starter is surfaced as a recommended platform entry point
- **WHEN** a discovery surface evaluates a built-in entry whose starter family marks it as a platform-native core starter
- **THEN** the entry is eligible for recommended or featured starter treatment for its declared core flow

#### Scenario: Helper built-in remains installable without masquerading as a core starter
- **WHEN** a generic helper built-in such as a search or repository utility remains listed in the official bundle
- **THEN** it stays installable and documented as an official helper
- **THEN** discovery surfaces do not label it as the primary starter for a platform core flow unless the starter metadata explicitly says so
