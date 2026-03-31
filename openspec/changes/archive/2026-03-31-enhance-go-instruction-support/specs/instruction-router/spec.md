# Instruction Router Specification

## ADDED Requirements

### Requirement: Router executes registered instruction definitions across local, bridge, and plugin targets
The system SHALL execute instructions through definitions registered in the in-process router. Each definition MUST declare a target and handler, and completed or failed results MUST report the resolved target.

#### Scenario: Execute local instruction through registered definition
- **WHEN** a `read` instruction is registered with the `local` target and executed
- **THEN** the router invokes the registered handler locally
- **THEN** the returned result reports target `local`

#### Scenario: Execute bridge instruction through registered definition
- **WHEN** a `think` instruction is registered with the `bridge` target and executed
- **THEN** the router invokes the registered handler for bridge execution
- **THEN** the returned result reports target `bridge`

#### Scenario: Execute plugin instruction through registered definition
- **WHEN** a `plugin.search` instruction is registered with the `plugin` target and executed
- **THEN** the router invokes the registered plugin handler
- **THEN** the returned result reports target `plugin`

### Requirement: Router normalizes and validates requests before execution
The router SHALL normalize requests before enqueueing or executing them by trimming the instruction type, generating an ID when missing, applying the definition default priority when the request priority is unset, cloning payload and metadata maps, and invoking any registered validator.

#### Scenario: Auto-generate request identity and default priority
- **WHEN** an instruction request omits `id` and `priority`
- **THEN** the router generates a stable instruction ID using the normalized type
- **THEN** the router applies the definition's default priority before queueing or execution

#### Scenario: Reject request that fails registered validator
- **WHEN** an instruction definition includes a validator and the request payload is invalid
- **THEN** the router rejects the request before handler execution
- **THEN** the recorded result is marked failed with the validation error

### Requirement: Router enforces timeouts and cancellation during execution
The router SHALL support definition-level default timeouts, request-level timeout overrides, queued-instruction cancellation, and in-flight cancellation through the execution context.

#### Scenario: Definition timeout fails slow handler
- **WHEN** an instruction exceeds the definition's default timeout
- **THEN** the router cancels the handler context
- **THEN** the recorded result is marked failed with the timeout error

#### Scenario: Request timeout overrides definition timeout
- **WHEN** an instruction request specifies its own timeout
- **THEN** the router uses the request timeout instead of the definition default

#### Scenario: Cancel queued instruction
- **WHEN** cancellation is requested for an instruction that is still pending
- **THEN** the router removes it from the queue
- **THEN** the recorded result is marked cancelled without executing the handler

#### Scenario: Cancel running instruction
- **WHEN** cancellation is requested for an instruction that is already running
- **THEN** the router cancels the handler context
- **THEN** the recorded result is marked cancelled

### Requirement: Router processes queued instructions by priority and dependency state
The router SHALL execute the highest-priority runnable instruction first, preserve FIFO order for instructions with the same priority, keep dependency-blocked items queued until they become runnable, and fail dependent instructions when a completed dependency did not succeed.

#### Scenario: Highest-priority runnable instruction executes first
- **WHEN** the queue contains runnable instructions with different priorities
- **THEN** `ProcessNext` executes the runnable instruction with the highest priority first

#### Scenario: Same-priority instructions keep enqueue order
- **WHEN** two runnable instructions share the same priority
- **THEN** the router executes them in enqueue order

#### Scenario: Blocked instruction stays queued while another runnable item exists
- **WHEN** the highest-priority queued instruction depends on another instruction that is still pending or running
- **THEN** `ProcessNext` leaves the blocked instruction queued
- **THEN** the router executes the next runnable instruction instead

#### Scenario: All queued instructions are blocked on dependencies
- **WHEN** every pending instruction depends on unfinished work
- **THEN** `ProcessNext` returns no runnable instruction
- **THEN** the queued instructions remain pending for later processing

#### Scenario: Failed dependency propagates to dependent instruction
- **WHEN** a queued instruction depends on another instruction that has already completed with a non-success status
- **THEN** the dependent instruction is recorded as failed without invoking its handler
- **THEN** the router returns a dependency failure error

### Requirement: Router records in-process introspection and execution metrics
The router SHALL expose in-process introspection for pending instructions, bounded execution history, and per-instruction-type metrics derived from recorded results.

#### Scenario: Inspect pending queue
- **WHEN** the caller requests pending instructions
- **THEN** the router returns queued items with their ID, type, target, priority, queued status, and dependency list
- **THEN** the returned list is sorted by execution priority

#### Scenario: Inspect bounded execution history
- **WHEN** the caller requests recent history with a limit
- **THEN** the router returns cloned completed, failed, and cancelled results up to that limit

#### Scenario: Inspect aggregated metrics
- **WHEN** the caller requests instruction metrics
- **THEN** the router returns per-type counts for total, success, failure, and cancelled executions
- **THEN** the router also returns last status, last error, and accumulated duration fields for each instruction type

### Requirement: Router isolates handler failures from the process
The router SHALL recover handler panics and convert them into failed instruction results instead of crashing the process.

#### Scenario: Handler panic becomes failed result
- **WHEN** a registered handler panics during execution
- **THEN** the router recovers the panic
- **THEN** the recorded result is marked failed with a panic-derived error message
