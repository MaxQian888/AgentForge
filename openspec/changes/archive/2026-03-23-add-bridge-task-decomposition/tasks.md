## 1. Bridge decomposition contract

- [x] 1.1 Add decomposition request/response schemas, prompt input shaping, and structured output validation in `src-bridge`.
- [x] 1.2 Implement `POST /bridge/decompose` with a lightweight AI execution path that is separate from the Agent runtime pool.
- [x] 1.3 Add bridge-level tests for validation failures, successful structured decomposition, and upstream AI failure handling.

## 2. Go task decomposition orchestration

- [x] 2.1 Extend the Go bridge client and task repository abstractions to support decomposition calls, child-task existence checks, and all-or-nothing child task creation.
- [x] 2.2 Implement task decomposition service logic that loads the parent task, rejects duplicate decomposition, normalizes Bridge output, and creates child tasks under the parent.
- [x] 2.3 Expose `POST /api/v1/tasks/:id/decompose` from the Go API handler/router with stable success and error responses.
- [x] 2.4 Add backend tests covering successful decomposition, missing task, duplicate-child conflict, invalid Bridge output, and no-partial-write failure behavior.

## 3. IM command integration

- [x] 3.1 Extend `src-im-bridge` API client/types to call the Go task decomposition endpoint and parse the structured response.
- [x] 3.2 Add `/task decompose <id>` command handling, including immediate progress feedback and final result/failure messaging.
- [x] 3.3 Add IM command tests or command-level verification for success and backend-error scenarios.

## 4. Verification and rollout readiness

- [x] 4.1 Run the targeted Bridge, Go backend, and IM bridge test suites that cover the new decomposition path.
- [x] 4.2 Verify the change artifacts and implementation notes stay aligned with the new `/bridge/decompose` and `/api/v1/tasks/:id/decompose` contracts.
