## ADDED Requirements

### Requirement: Task decomposition runs through a real Bridge text-generation provider
The system MUST execute task decomposition through a Bridge provider that supports `text_generation`, instead of returning a simulated decomposition payload.

#### Scenario: Decomposition succeeds through the resolved provider
- **WHEN** the backend requests decomposition for an eligible task and the Bridge resolves a configured text-generation provider
- **THEN** the Bridge SHALL call a real provider runtime for that request
- **THEN** the backend SHALL receive a structured decomposition result that can be validated and persisted as child tasks

#### Scenario: Decomposition provider cannot serve the request
- **WHEN** the Bridge resolves task decomposition to a provider that is unknown, misconfigured, or does not support `text_generation`
- **THEN** the decomposition request MUST fail
- **THEN** the backend MUST NOT create any child tasks for that parent task

### Requirement: Task decomposition honors provider and model resolution rules
The system MUST apply the Bridge provider registry defaults and validation rules consistently for task decomposition requests, regardless of whether the caller supplied explicit provider metadata.

#### Scenario: Decomposition request uses Bridge defaults
- **WHEN** the backend submits a decomposition request without explicit provider or model values
- **THEN** the Bridge SHALL use the default `text_generation` provider and model configured for task decomposition
- **THEN** the returned result SHALL identify a truthful success or failure from that resolved provider path

#### Scenario: Provider output is structurally invalid
- **WHEN** the resolved provider returns output that does not satisfy the decomposition schema
- **THEN** the Bridge MUST reject the result as invalid instead of fabricating a substitute response
- **THEN** the backend MUST preserve the existing all-or-nothing decomposition persistence behavior
