# Feishu Card Support — Phase 1: Callback Closure

**Date:** 2026-04-16
**Status:** Draft (pending user + spec-document-reviewer approval)
**Scope:** First of a 5-phase program to complete Feishu card support in AgentForge.

---

## Context

The current Feishu adapter (`src-im-bridge/platform/feishu/`) routes inbound `P2CardActionTrigger` events through `handleCardAction → normalizeCardActionRequest → notify.ActionHandler`. The bridge side partially recognizes non-button elements (select, date_picker, overflow, form submit) and tags them with synthetic action names (`select` / `date_pick` / `overflow` / `form_submit`), but three defects make these callbacks fail in production:

1. The Go backend executor (`src-go/internal/service/im_action_execution.go`) has a 7-case switch and returns `"Unknown action"` for every synthetic name, so any non-button card element click shows a failure toast to the user.
2. `normalizeCardActionRequest` does not read `Checked`, `Options`, `InputValue`, or `Name` from the SDK's `CallBackAction` struct, so multi-select/checker/input values are lost even when the bridge recognizes the tag.
3. `handleReaction` is a no-op with a TODO comment; emoji reactions never reach `ActionHandler`.

Phase 1 closes these three gaps without changing card rendering, the NativeMessage schema, or introducing cardkit 2.0. After Phase 1, clicking any currently-supported Feishu card element reaches a backend handler that either performs real work or returns a typed "unsupported in this context" response — never a generic "Unknown action".

## Goals

- Every callback element the bridge already recognizes (button / select_static / select_person / multi_select_static / multi_select_person / date_picker / time_picker / datetime_picker / overflow / checker / input / form submit) delivers a **complete** payload to `IMActionExecutor.Execute` and gets a deterministic response.
- The backend executor routes the new synthetic action names to real business operations or to typed "not-applicable" outcomes with actionable error text.
- Emoji reactions are surfaced as an `ActionRequest` with `action="react"` and are persisted as IM events; no business side-effect by default.
- All changes covered by unit tests; bridge-side Feishu smoke test extended to cover a button click end-to-end.

## Non-goals

- No new card elements on the sending side (Phase 3).
- No cardkit 2.0 / schema 2.0 (Phase 4).
- No new inbound message types beyond text/post/image/file (Phase 2).
- No change to the delayed-update (`card/update`) wire format (Phase 4).
- No change to webhook signature behavior (Phase 5).

## Architecture

Data flow after Phase 1:

```
Feishu server
  │  (long connection or webhook)
  ▼
larkdispatcher.EventDispatcher
  ├── OnP2CardActionTrigger ──▶ handleCardAction ─▶ normalizeCardActionRequest ─┐
  └── OnP2MessageReactionCreatedV1 ──▶ handleReaction ─▶ buildReactionRequest ──┤
                                                                                 ▼
                                                              notify.ActionHandler (bridge client)
                                                                                 │
                                                                                 ▼
                                                                        Go backend  /im/action
                                                                                 │
                                                                                 ▼
                                                              BackendIMActionExecutor.Execute
                                                                                 │
                                           ┌─────────────────────────────────────┼──────────────┐
                                           ▼                                     ▼              ▼
                                existing actions                    new select/date/form/     react
                                (assign-agent, etc.)                 overflow routers         event recorder
```

Four surfaces change:

1. **`CallBackAction` normalization** (`live.go`): read the remaining SDK fields and expose them via `ActionRequest.Metadata` under stable keys.
2. **Action name taxonomy** (`live.go`): the synthetic names become a closed enum documented in `core/action_reference.go`.
3. **Reaction → ActionRequest** (`live.go`): `handleReaction` builds a request whose `action="react"`, `entity_id=<message_id>`, and metadata carries the emoji key, operator, and chat.
4. **Backend executor** (`im_action_execution.go`): add dispatch cases for every new synthetic action. Where no business binding exists yet, return a typed `IMActionStatusBlocked` response with a human-readable explanation — never `"Unknown action"`.

## Component changes

### 1. CallBackAction field coverage

**File:** `src-im-bridge/platform/feishu/live.go::normalizeCardActionRequest`

Read these SDK fields in addition to the current `Value`/`Option`/`FormValue`:

| SDK field | Semantics | Metadata key |
|---|---|---|
| `Checked` (bool) | checker element state after click | `checker_state` = `"true"` / `"false"` |
| `Options` ([]string) | values of multi-select after click | `selected_options` = CSV |
| `InputValue` (string) | value of input element | `input_value` |
| `Name` (string) | author-assigned element name | `element_name` |
| `Timezone` (string) | picker timezone | `timezone` |

Additional rules:

- For `checker` tag: set `action="toggle"`, `entity_id=<action-ref entity>`, include `checker_state` in metadata.
- For `multi_select_*` tag: set `action="multi_select"`, `entity_id=<action-ref entity>`, include `selected_options`.
- For `input` tag: set `action="input_submit"`, include `input_value`.
- `form_submit` path already exists; extend it to copy every `form_value` key verbatim with prefix `form_` (current behavior) **and** retain `element_name` / `input_value` if present.
- Unknown tags still return `errIgnoreCardAction` (current behavior) rather than a fabricated action — this preserves forward compatibility.

### 2. Synthetic action name taxonomy

Document the closed enum in `core/action_reference.go` (new comment block) and add constants:

```go
const (
    ActionNameReact         = "react"          // emoji reaction on message
    ActionNameSelect        = "select"         // single-select click
    ActionNameMultiSelect   = "multi_select"   // multi-select click
    ActionNameDatePick      = "date_pick"      // date/time/datetime picker
    ActionNameOverflow      = "overflow"       // "..." menu click
    ActionNameToggle        = "toggle"         // checker click
    ActionNameInputSubmit   = "input_submit"   // input element commit
    ActionNameFormSubmit    = "form_submit"    // form container submit
)
```

These are **framework-level** action names — distinct from user-defined `act:<verb>:<entity>` references. When a bridge-side element does not carry an explicit `act:...` value, the synthetic name above applies and `entity_id` is set to the selected option / message id / (empty).

### 3. Reaction forwarding

**File:** `src-im-bridge/platform/feishu/live.go::handleReaction`

Current code logs and returns. New behavior:

```go
func (l *Live) handleReaction(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
    req, err := buildReactionRequest(event)
    if err != nil || req == nil {
        return err
    }
    if l.actionHandler == nil {
        return nil  // drop quietly when no handler configured
    }
    _, err = l.actionHandler.HandleAction(ctx, req)
    return err
}
```

`buildReactionRequest` extracts `MessageID`, `ChatID`, operator, and emoji type from `event.Event.Reaction` and populates `ActionRequest`:

- `Platform = "feishu"`
- `Action = "react"`
- `EntityID = <message_id>`
- `UserID = <operator open_id/user_id>`
- `Metadata = {"emoji": <type>, "event_type": "created"}`
- `ReplyTarget = {Platform, ChatID, MessageID}` — no callback token (reactions have no interactive ack window)

Same pattern for `MessageReactionDeletedV1` (added in Phase 2 but scaffolded here so the handler is in place). For Phase 1 we only implement `Created`.

### 4. Backend executor dispatch

**File:** `src-go/internal/service/im_action_execution.go`

Extend the switch with the new names. Proposed business bindings (override during spec review if you want different semantics):

| Synthetic action | Backend behavior (proposed) | Required metadata | Outcome |
|---|---|---|---|
| `select` | Parameterize an existing action. If metadata has `target_action`, dispatch to it with `selected_option` as the chosen param. Otherwise return `IMActionStatusBlocked` with explanation. | `target_action` (e.g. `assign-agent`), plus action-specific keys | reuses existing executor path |
| `multi_select` | Same as `select` but `selected_options` CSV is split and passed as a list in metadata | `target_action` | reuses existing executor path |
| `date_pick` | New `setTaskDueDate(taskID, date)` — requires a repository method; if the project has no due-date model yet, record as `IMActionStatusBlocked` with "Due-date workflow not configured" | `entity_id=<taskID>`, `date` (from bridge) | `IMActionStatusCompleted` on success |
| `overflow` | Router: treat `selected_option` as an `act:<verb>:<entity>` reference and recurse into Execute with that parsed reference | `selected_option` must be a parseable action ref | whatever the inner action returns |
| `toggle` | If `entity_id` parses as `taskID`, use `checker_state` to set `completed` vs `open` via existing transition path. Otherwise return blocked. | `entity_id=<taskID>`, `checker_state` | `IMActionStatusCompleted` |
| `input_submit` | Append `input_value` as a task comment if `entity_id` is a taskID. Requires a comment repository method. If comment not configured, return blocked. | `entity_id=<taskID>`, `input_value` | `IMActionStatusCompleted` |
| `form_submit` | Inspect `element_name` (form id) and dispatch to a specific sub-handler keyed by name (`create-task-form` / `review-form` / etc.). Unknown form id → blocked. | form fields in metadata | varies |
| `react` | Record-only: write an entry to a new `im_reaction_events` table and return `IMActionStatusCompleted` with empty Result so the bridge does not post a reply. | `emoji` | `IMActionStatusCompleted`, no user-visible side-effect |

**Key design decision:** no action silently returns success if the required business binding is missing. The executor **must** pick one of:
- `IMActionStatusCompleted` + meaningful `Result` text
- `IMActionStatusBlocked` + explicit reason
- `IMActionStatusFailed` + actionable error

`IMActionStatusFailed` is reserved for internal errors (DB down, bad input). `IMActionStatusBlocked` is the correct status for "this workflow is not configured yet" so the user sees a specific message instead of a generic failure.

### 5. Storage additions

Phase 1 adds one new repository-level feature:

- **`im_reaction_events` table** — append-only log of reactions. Columns: `id`, `platform`, `chat_id`, `message_id`, `user_id`, `emoji`, `event_type` (created/deleted), `raw_payload` (jsonb for debugging), `created_at`. Indexed by `(message_id, created_at)`.

Reactions are the only new side-effect; the other new actions reuse existing repositories (Task, Review, Dispatch).

### 6. Test coverage

**Bridge side** (`src-im-bridge/platform/feishu/live_test.go`):

- `TestNormalizeCardActionRequest_Checker` — checker tag with `Checked=true` → `action=toggle`, `checker_state=true`
- `TestNormalizeCardActionRequest_MultiSelect` — multi_select with 3 options → `action=multi_select`, metadata has comma-separated `selected_options`
- `TestNormalizeCardActionRequest_Input` — input tag → `action=input_submit`, `input_value` in metadata
- `TestNormalizeCardActionRequest_OverflowWithActionRef` — overflow option is `act:decompose:task-x` → `action=overflow`, `selected_option=act:decompose:task-x`
- `TestHandleReaction_ForwardsToActionHandler` — `MessageReactionCreatedV1` → ActionHandler receives `action=react`
- `TestHandleReaction_NoHandlerIsQuiet` — without action handler, no error
- `TestNormalizeCardActionRequest_ElementName` — `Name` field copied to `element_name` metadata

**Backend side** (new `im_action_execution_phase1_test.go` or extend existing):

- `TestExecute_Select_WithoutTargetAction_ReturnsBlocked`
- `TestExecute_Select_WithTargetAction_ReusesPath` (delegates to assign-agent mock)
- `TestExecute_Overflow_ParsesInnerActionRef`
- `TestExecute_Toggle_TransitionsTask`
- `TestExecute_InputSubmit_AppendsComment` (or Blocked if comment not wired)
- `TestExecute_DatePick_WithoutDueDateRepo_ReturnsBlocked`
- `TestExecute_Reaction_RecordsEvent_NoReply`
- `TestExecute_FormSubmit_UnknownFormID_ReturnsBlocked`
- `TestExecute_FormSubmit_KnownFormID_Dispatches` (at least one wired form like `create-task-form`)

**Smoke test** (`src-im-bridge/scripts/smoke/`):

- Extend `scripts/smoke/feishu-live.ps1` (or equivalent) with a `-CardClick` flag that sends a synthetic `P2CardActionTrigger` event through the bridge's HTTP callback and asserts `/im/action` was invoked with the expected synthetic action and metadata.

## Migration / rollout

Phase 1 is purely additive:
- Existing 7 actions keep working identically.
- New synthetic actions had no prior handler, so there is no deprecation.
- DB migration for `im_reaction_events` is a new `CREATE TABLE`.

No feature flag is required. Rollout order: merge → bridge redeploy → backend redeploy (reaction events table must exist before bridge forwards them; enforce via migration-first deployment).

## Risks & open questions

1. **Date-pick repository gap** — task model currently has no due-date field. Either (a) add a nullable `due_date` field in Phase 1, or (b) return Blocked and defer due-date storage to a later change. The spec currently proposes (b); implementation may discover (a) is cheap enough to bundle. Decide during planning (writing-plans).

2. **Comment repository gap** — same situation for input_submit; current task model has comments via a separate path. If the existing path is reusable, wire it; otherwise return Blocked and add in a later phase.

3. **Overflow recursion depth** — `overflow → select → overflow …` is theoretically possible. Limit recursion to 1 level (overflow calls an inner executor once) to prevent abuse.

4. **Reaction spam** — bots or spammy channels may generate many reactions. Phase 1 writes every reaction to `im_reaction_events` unconditionally. Consider rate limits in Phase 5; for Phase 1, rely on Feishu's own rate limits.

5. **target_action whitelist** — a bridge-provided `target_action` could attempt to call internal-only actions. Phase 1 restricts `target_action` to the 8 existing action strings (`assign-agent`, `decompose`, `transition-task`, `move-task`, `save-as-doc`, `create-task`, `approve`, `request-changes`) via a hard-coded whitelist. `select`, `multi_select`, and `overflow` are explicitly excluded from the whitelist to prevent recursion.

6. **`selected_options` CSV encoding** — Phase 1 joins multi-select values with `,`. This is safe because Feishu option values are developer-controlled identifiers, not free text. If a later phase needs arbitrary strings, switch to JSON-array encoding.

## Success criteria

When Phase 1 is merged and deployed, the following **manual test** passes:

1. Send a Feishu card that contains one button (`act:approve:<reviewID>`), one select_static with `selected_option=deploy`, one date_picker, one overflow with `[act:decompose:<taskID>, act:transition-task:<taskID>?targetStatus=done]`, one checker, one input, and an inline form.
2. Click each element.
3. Observe:
   - Button: approved review (existing behavior unchanged).
   - Select: either runs the `target_action` or shows "Action workflow not configured" blocked toast — never "Unknown action".
   - Date picker: task due date updated OR "Due-date workflow not configured" blocked toast.
   - Overflow first item: task decomposed.
   - Overflow second item: task transitioned to done.
   - Checker: task completed/reopened per state.
   - Input: comment appended OR blocked toast.
   - Form: dispatched to named form handler OR blocked toast.
4. React with 👍 on a task notification. Verify row in `im_reaction_events`.

Every interaction returns a deterministic, actionable response within the Feishu callback ack window (~3 s).
