# Feishu Card Phase 1 — Callback Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the Feishu card callback loop so every click of every supported element (button, select, multi-select, date/time picker, overflow, checker, input, form, reaction) reaches a backend handler that returns a deterministic response — never "Unknown action".

**Architecture:** Two surfaces change. In the bridge (`src-im-bridge/platform/feishu/`), `normalizeCardActionRequest` reads the remaining SDK `CallBackAction` fields (Checked, Options, InputValue, Name, Timezone) and `handleReaction` forwards to the action handler. In the backend (`src-go/internal/service/`), `BackendIMActionExecutor.Execute` gains eight new cases (react, select, multi_select, date_pick, overflow, toggle, input_submit, form_submit), a hard-coded `target_action` whitelist, and a new `im_reaction_events` repository.

**Tech Stack:** Go 1.22, `larksuite/oapi-sdk-go/v3 v3.5.3`, GORM, PostgreSQL, `testify` for assertions, Jest (not used in this plan — bridge and backend are Go-only).

**Reference spec:** `docs/superpowers/specs/2026-04-16-feishu-card-phase1-callback-closure-design.md`

---

## File Structure

### Modified
- `src-im-bridge/core/action_reference.go` — add 8 synthetic action name constants
- `src-im-bridge/platform/feishu/live.go` — expand `normalizeCardActionRequest`, wire `handleReaction`, register `OnP2MessageReactionDeletedV1`
- `src-im-bridge/platform/feishu/live_test.go` — expand test coverage
- `src-im-bridge/platform/feishu/stub.go` — add `/test/cardclick` endpoint
- `src-im-bridge/platform/feishu/stub_test.go` — test for card-click endpoint
- `src-go/internal/service/im_action_execution.go` — 8 new action cases + whitelist + recursion guard + new dependency injection
- `src-go/internal/service/im_action_execution_test.go` — coverage for the new cases

### Created
- `src-go/migrations/054_create_im_reaction_events.up.sql`
- `src-go/migrations/054_create_im_reaction_events.down.sql`
- `src-go/internal/model/im_reaction_event.go` — domain model
- `src-go/internal/repository/im_reaction_event_repo.go` — repository
- `src-go/internal/repository/im_reaction_event_repo_test.go` — tests

---

## Task 1 — Bridge: synthetic action name constants

**Files:**
- Modify: `src-im-bridge/core/action_reference.go` (append constants)
- Create: `src-im-bridge/core/action_reference_synthetic_test.go`

- [ ] **Step 1.1: Write the failing test**

Create `src-im-bridge/core/action_reference_synthetic_test.go`:

```go
package core

import "testing"

func TestSyntheticActionNames_AreClosedEnum(t *testing.T) {
	want := []string{
		"react",
		"select",
		"multi_select",
		"date_pick",
		"overflow",
		"toggle",
		"input_submit",
		"form_submit",
	}
	got := []string{
		ActionNameReact,
		ActionNameSelect,
		ActionNameMultiSelect,
		ActionNameDatePick,
		ActionNameOverflow,
		ActionNameToggle,
		ActionNameInputSubmit,
		ActionNameFormSubmit,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, len(want)=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 1.2: Run to confirm compile failure**

Run: `cd src-im-bridge && go test ./core/ -run TestSyntheticActionNames -count=1`
Expected: FAIL with `undefined: ActionNameReact` (and the other 7).

- [ ] **Step 1.3: Add the constants**

Append to `src-im-bridge/core/action_reference.go`:

```go
// Synthetic framework-level action names used when a card element click is
// not represented by an `act:<verb>:<entity>` reference. These are generated
// by the bridge-side normalizer and dispatched by the backend executor.
const (
	ActionNameReact       = "react"        // emoji reaction on a message
	ActionNameSelect      = "select"       // single-select click
	ActionNameMultiSelect = "multi_select" // multi-select click
	ActionNameDatePick    = "date_pick"    // date/time/datetime picker
	ActionNameOverflow    = "overflow"     // overflow ("…") menu click
	ActionNameToggle      = "toggle"       // checker element click
	ActionNameInputSubmit = "input_submit" // input element commit
	ActionNameFormSubmit  = "form_submit"  // form container submit
)
```

- [ ] **Step 1.4: Run the test to confirm pass**

Run: `cd src-im-bridge && go test ./core/ -run TestSyntheticActionNames -count=1`
Expected: PASS.

- [ ] **Step 1.5: Commit**

```bash
rtk git add src-im-bridge/core/action_reference.go src-im-bridge/core/action_reference_synthetic_test.go
rtk git commit -m "feat(im-bridge): add synthetic action name enum for Feishu card callbacks

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 2 — Bridge: Checker → toggle

**Files:**
- Modify: `src-im-bridge/platform/feishu/live.go` (inside `normalizeCardActionRequest`)
- Modify: `src-im-bridge/platform/feishu/live_test.go`

- [ ] **Step 2.1: Write the failing test**

Append to `src-im-bridge/platform/feishu/live_test.go`:

```go
func TestNormalizeCardActionRequest_CheckerChecked(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "checker",
				Checked: true,
				Value:   map[string]interface{}{"action": "act:toggle:task-xyz"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1", OpenMessageID: "msg-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameToggle {
		t.Errorf("Action = %q, want toggle", req.Action)
	}
	if req.EntityID != "task-xyz" {
		t.Errorf("EntityID = %q, want task-xyz", req.EntityID)
	}
	if req.Metadata["checker_state"] != "true" {
		t.Errorf("checker_state = %q, want true", req.Metadata["checker_state"])
	}
	if req.Metadata["action_tag"] != "checker" {
		t.Errorf("action_tag = %q, want checker", req.Metadata["action_tag"])
	}
}

func TestNormalizeCardActionRequest_CheckerUncheckedWithoutActionRef(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "checker",
				Checked: false,
				Value:   map[string]interface{}{},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameToggle {
		t.Errorf("Action = %q, want toggle", req.Action)
	}
	if req.EntityID != "" {
		t.Errorf("EntityID = %q, want empty", req.EntityID)
	}
	if req.Metadata["checker_state"] != "false" {
		t.Errorf("checker_state = %q, want false", req.Metadata["checker_state"])
	}
}
```

- [ ] **Step 2.2: Run to confirm failure**

Run: `cd src-im-bridge && go test ./platform/feishu/ -run 'TestNormalizeCardActionRequest_Checker' -count=1`
Expected: FAIL — test returns `errIgnoreCardAction` because `checker` is not in the switch.

- [ ] **Step 2.3: Implement**

In `src-im-bridge/platform/feishu/live.go` inside `normalizeCardActionRequest`, locate the switch on `actionTag` (around line 793). Add a `case "checker":` branch **before** the `default` branch:

```go
case "checker":
	action = core.ActionNameToggle
	// entityID comes from act:<verb>:<entity> if present, otherwise empty
	if parsedAction, parsedEntity, parsedMeta, parsedOK := core.ParseActionReferenceWithMetadata(actionValue); parsedOK {
		_ = parsedAction // we override action name to "toggle" but keep entity+metadata
		entityID = parsedEntity
		actionMetadata = parsedMeta
	} else {
		entityID = ""
	}
```

Then, below the switch (where metadata is assembled), add:

```go
if actionTag == "checker" {
	metadata["checker_state"] = strconv.FormatBool(act.Checked)
}
```

Add `"strconv"` to the imports of `live.go` if not already present.

- [ ] **Step 2.4: Run to confirm pass**

Run: `cd src-im-bridge && go test ./platform/feishu/ -run 'TestNormalizeCardActionRequest_Checker' -count=1`
Expected: PASS.

Also run the full feishu package: `go test ./platform/feishu/ -count=1`
Expected: PASS (no regressions).

- [ ] **Step 2.5: Commit**

```bash
rtk git add src-im-bridge/platform/feishu/live.go src-im-bridge/platform/feishu/live_test.go
rtk git commit -m "feat(im-bridge/feishu): map checker tag to toggle action with checker_state metadata

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 3 — Bridge: MultiSelect Options → multi_select

**Files:**
- Modify: `src-im-bridge/platform/feishu/live.go`
- Modify: `src-im-bridge/platform/feishu/live_test.go`

- [ ] **Step 3.1: Write the failing test**

Append to `live_test.go`:

```go
func TestNormalizeCardActionRequest_MultiSelectWithActionRef(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "multi_select_static",
				Options: []string{"opt_a", "opt_b", "opt_c"},
				Value:   map[string]interface{}{"action": "act:assign-agent:task-xyz"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameMultiSelect {
		t.Errorf("Action = %q, want multi_select", req.Action)
	}
	if req.EntityID != "task-xyz" {
		t.Errorf("EntityID = %q, want task-xyz", req.EntityID)
	}
	if req.Metadata["selected_options"] != "opt_a,opt_b,opt_c" {
		t.Errorf("selected_options = %q, want opt_a,opt_b,opt_c", req.Metadata["selected_options"])
	}
}

func TestNormalizeCardActionRequest_MultiSelectPersonFallback(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "multi_select_person",
				Options: []string{"ou_user_a", "ou_user_b"},
				Value:   map[string]interface{}{},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameMultiSelect {
		t.Errorf("Action = %q, want multi_select", req.Action)
	}
	// With no action ref, EntityID is empty; options go in metadata
	if req.Metadata["selected_options"] != "ou_user_a,ou_user_b" {
		t.Errorf("selected_options = %q", req.Metadata["selected_options"])
	}
}
```

- [ ] **Step 3.2: Run to confirm failure**

Run: `cd src-im-bridge && go test ./platform/feishu/ -run 'TestNormalizeCardActionRequest_MultiSelect' -count=1`
Expected: FAIL — current code treats `multi_select_*` as `select` with empty `entity_id`.

- [ ] **Step 3.3: Implement**

In `live.go`, inside `normalizeCardActionRequest`, replace the existing case:

```go
case "select_static", "select_person", "multi_select_static", "multi_select_person":
	action = "select"
	entityID = selectedOption
	if entityID == "" {
		entityID = actionValue
	}
```

with two distinct cases:

```go
case "select_static", "select_person":
	action = core.ActionNameSelect
	if parsedAction, parsedEntity, parsedMeta, parsedOK := core.ParseActionReferenceWithMetadata(actionValue); parsedOK {
		_ = parsedAction
		entityID = parsedEntity
		actionMetadata = parsedMeta
	} else {
		entityID = selectedOption
	}
case "multi_select_static", "multi_select_person":
	action = core.ActionNameMultiSelect
	if parsedAction, parsedEntity, parsedMeta, parsedOK := core.ParseActionReferenceWithMetadata(actionValue); parsedOK {
		_ = parsedAction
		entityID = parsedEntity
		actionMetadata = parsedMeta
	} else {
		entityID = ""
	}
```

Then below the switch, add:

```go
if strings.HasPrefix(actionTag, "multi_select") && len(act.Options) > 0 {
	metadata["selected_options"] = strings.Join(act.Options, ",")
}
```

Also update the existing `ActionName*` usage that currently sets `action = "select"` / `"date_pick"` / `"overflow"` to use the constants. Change:

- `action = "select"` → `action = core.ActionNameSelect`
- `action = "date_pick"` → `action = core.ActionNameDatePick`
- `action = "overflow"` → `action = core.ActionNameOverflow`
- `action = "form_submit"` → `action = core.ActionNameFormSubmit`

Add `core` import if `live.go` does not already reference a constant from the core package (it does — `core.ReplyTarget`).

- [ ] **Step 3.4: Run to confirm pass**

Run: `cd src-im-bridge && go test ./platform/feishu/ -count=1`
Expected: PASS. Pre-existing tests `TestNormalizeCardActionRequest_SelectStatic` and `TestNormalizeCardActionRequest_SelectFallback` should still pass because `core.ActionNameSelect == "select"`.

- [ ] **Step 3.5: Commit**

```bash
rtk git add src-im-bridge/platform/feishu/live.go src-im-bridge/platform/feishu/live_test.go
rtk git commit -m "feat(im-bridge/feishu): distinguish multi_select from select with selected_options metadata

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 4 — Bridge: InputValue → input_submit

**Files:**
- Modify: `src-im-bridge/platform/feishu/live.go`
- Modify: `src-im-bridge/platform/feishu/live_test.go`

- [ ] **Step 4.1: Write the failing test**

Append to `live_test.go`:

```go
func TestNormalizeCardActionRequest_InputWithActionRef(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:        "input",
				InputValue: "please reconsider",
				Value:      map[string]interface{}{"action": "act:input_submit:task-xyz"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameInputSubmit {
		t.Errorf("Action = %q, want input_submit", req.Action)
	}
	if req.EntityID != "task-xyz" {
		t.Errorf("EntityID = %q, want task-xyz", req.EntityID)
	}
	if req.Metadata["input_value"] != "please reconsider" {
		t.Errorf("input_value = %q", req.Metadata["input_value"])
	}
}

func TestNormalizeCardActionRequest_InputWithoutActionRefIsIgnored(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:        "input",
				InputValue: "typed text",
				Value:      map[string]interface{}{},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	_, err := normalizeCardActionRequest(event)
	if !errors.Is(err, errIgnoreCardAction) {
		t.Fatalf("err = %v, want errIgnoreCardAction", err)
	}
}
```

- [ ] **Step 4.2: Run to confirm failure**

Run: `cd src-im-bridge && go test ./platform/feishu/ -run 'TestNormalizeCardActionRequest_Input' -count=1`
Expected: FAIL — unknown tag falls to default `errIgnoreCardAction`, so first test fails on `input_submit` not being returned.

- [ ] **Step 4.3: Implement**

In `live.go` add a new case:

```go
case "input":
	action = core.ActionNameInputSubmit
	if parsedAction, parsedEntity, parsedMeta, parsedOK := core.ParseActionReferenceWithMetadata(actionValue); parsedOK {
		_ = parsedAction
		entityID = parsedEntity
		actionMetadata = parsedMeta
	} else {
		return nil, errIgnoreCardAction
	}
```

Below the switch, add:

```go
if strings.TrimSpace(act.InputValue) != "" {
	metadata["input_value"] = strings.TrimSpace(act.InputValue)
}
```

- [ ] **Step 4.4: Run to confirm pass**

Run: `cd src-im-bridge && go test ./platform/feishu/ -count=1`
Expected: PASS.

- [ ] **Step 4.5: Commit**

```bash
rtk git add src-im-bridge/platform/feishu/live.go src-im-bridge/platform/feishu/live_test.go
rtk git commit -m "feat(im-bridge/feishu): capture input element submissions as input_submit action

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 5 — Bridge: Name / Timezone pass-through

**Files:**
- Modify: `src-im-bridge/platform/feishu/live.go`
- Modify: `src-im-bridge/platform/feishu/live_test.go`

- [ ] **Step 5.1: Write the failing test**

Append:

```go
func TestNormalizeCardActionRequest_PassesElementNameAndTimezone(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:      "date_picker",
				Name:     "due_date_picker",
				Timezone: "Asia/Shanghai",
				Value:    map[string]interface{}{"date": "2026-04-20"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Metadata["element_name"] != "due_date_picker" {
		t.Errorf("element_name = %q", req.Metadata["element_name"])
	}
	if req.Metadata["timezone"] != "Asia/Shanghai" {
		t.Errorf("timezone = %q", req.Metadata["timezone"])
	}
}
```

- [ ] **Step 5.2: Run to confirm failure**

Run: `cd src-im-bridge && go test ./platform/feishu/ -run 'TestNormalizeCardActionRequest_PassesElementNameAndTimezone' -count=1`
Expected: FAIL — metadata lacks element_name / timezone.

- [ ] **Step 5.3: Implement**

In `live.go`, within `normalizeCardActionRequest` where metadata is assembled (alongside `action_tag`, `selected_option`, etc.), add:

```go
if name := strings.TrimSpace(act.Name); name != "" {
	metadata["element_name"] = name
}
if tz := strings.TrimSpace(act.Timezone); tz != "" {
	metadata["timezone"] = tz
}
```

- [ ] **Step 5.4: Run to confirm pass**

Run: `cd src-im-bridge && go test ./platform/feishu/ -count=1`
Expected: PASS.

- [ ] **Step 5.5: Commit**

```bash
rtk git add src-im-bridge/platform/feishu/live.go src-im-bridge/platform/feishu/live_test.go
rtk git commit -m "feat(im-bridge/feishu): surface element name and timezone in action metadata

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 6 — Bridge: Reaction Created + Deleted forwarding

> **Scope note:** the spec said Phase 1 scaffolds `MessageReactionDeletedV1`; the plan implements it fully because doing so is the same LOC as scaffolding and avoids a stub in the interface. The backend executor handles both `event_type=created` and `event_type=deleted` via the same path in Task 10.

**Files:**
- Modify: `src-im-bridge/platform/feishu/live.go`
- Modify: `src-im-bridge/platform/feishu/live_test.go`

- [ ] **Step 6.1: Write the failing tests**

Append:

```go
type recordingActionHandler struct {
	reqs []*notify.ActionRequest
}

func (h *recordingActionHandler) HandleAction(_ context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	h.reqs = append(h.reqs, req)
	return &notify.ActionResponse{}, nil
}

func TestLive_HandleReactionCreatedForwardsToActionHandler(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	rec := &recordingActionHandler{}
	live.SetActionHandler(rec)

	msgID := "om_reaction_msg"
	emojiType := "THUMBSUP"
	openID := "ou_reactor"
	event := &larkim.P2MessageReactionCreatedV1{
		Event: &larkim.P2MessageReactionCreatedV1Data{
			MessageId:    &msgID,
			ReactionType: &larkim.Emoji{EmojiType: &emojiType},
			UserId:       &larkim.UserId{OpenId: &openID},
		},
	}
	if err := live.handleReaction(context.Background(), event); err != nil {
		t.Fatalf("handleReaction error: %v", err)
	}
	if len(rec.reqs) != 1 {
		t.Fatalf("expected 1 forwarded request, got %d", len(rec.reqs))
	}
	got := rec.reqs[0]
	if got.Action != core.ActionNameReact {
		t.Errorf("Action = %q, want react", got.Action)
	}
	if got.EntityID != msgID {
		t.Errorf("EntityID = %q, want %s", got.EntityID, msgID)
	}
	if got.UserID != openID {
		t.Errorf("UserID = %q, want %s", got.UserID, openID)
	}
	if got.Metadata["emoji"] != emojiType {
		t.Errorf("emoji = %q, want %s", got.Metadata["emoji"], emojiType)
	}
	if got.Metadata["event_type"] != "created" {
		t.Errorf("event_type = %q, want created", got.Metadata["event_type"])
	}
}

func TestLive_HandleReactionDeletedForwardsWithDeletedEventType(t *testing.T) {
	live, _ := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	rec := &recordingActionHandler{}
	live.SetActionHandler(rec)

	msgID := "om_reaction_msg"
	emojiType := "THUMBSUP"
	openID := "ou_reactor"
	event := &larkim.P2MessageReactionDeletedV1{
		Event: &larkim.P2MessageReactionDeletedV1Data{
			MessageId:    &msgID,
			ReactionType: &larkim.Emoji{EmojiType: &emojiType},
			UserId:       &larkim.UserId{OpenId: &openID},
		},
	}
	if err := live.handleReactionDeleted(context.Background(), event); err != nil {
		t.Fatalf("handleReactionDeleted error: %v", err)
	}
	if len(rec.reqs) != 1 {
		t.Fatalf("expected 1 forwarded request")
	}
	if rec.reqs[0].Metadata["event_type"] != "deleted" {
		t.Errorf("event_type = %q, want deleted", rec.reqs[0].Metadata["event_type"])
	}
}

func TestLive_HandleReactionWithNoHandlerIsNoOp(t *testing.T) {
	live, _ := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	// no action handler set
	msgID := "om_x"
	emojiType := "OK"
	openID := "ou_x"
	event := &larkim.P2MessageReactionCreatedV1{
		Event: &larkim.P2MessageReactionCreatedV1Data{
			MessageId:    &msgID,
			ReactionType: &larkim.Emoji{EmojiType: &emojiType},
			UserId:       &larkim.UserId{OpenId: &openID},
		},
	}
	if err := live.handleReaction(context.Background(), event); err != nil {
		t.Fatalf("expected no error when handler missing, got %v", err)
	}
}
```

- [ ] **Step 6.2: Run to confirm failure**

Run: `cd src-im-bridge && go test ./platform/feishu/ -run 'TestLive_HandleReaction' -count=1`
Expected: FAIL — `handleReactionDeleted` not defined; `handleReaction` currently ignores the event.

- [ ] **Step 6.3: Implement**

In `live.go`, replace the existing `handleReaction` body (currently only logs) with:

```go
func (l *Live) handleReaction(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
	req := buildReactionRequest(event.GetEvent(), "created")
	if req == nil {
		return nil
	}
	if l.actionHandler == nil {
		return nil
	}
	_, err := l.actionHandler.HandleAction(ctx, req)
	return err
}

func (l *Live) handleReactionDeleted(ctx context.Context, event *larkim.P2MessageReactionDeletedV1) error {
	if event == nil {
		return nil
	}
	req := buildReactionRequestFromDeleted(event.Event)
	if req == nil {
		return nil
	}
	if l.actionHandler == nil {
		return nil
	}
	_, err := l.actionHandler.HandleAction(ctx, req)
	return err
}
```

Note: the current `P2MessageReactionCreatedV1` type embeds `Event *P2MessageReactionCreatedV1Data`. There is no `GetEvent()` helper — call `event.Event` directly:

```go
func (l *Live) handleReaction(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
	if event == nil {
		return nil
	}
	req := buildReactionRequestFromCreated(event.Event)
	// …
}
```

Add the two builders:

```go
func buildReactionRequestFromCreated(data *larkim.P2MessageReactionCreatedV1Data) *notify.ActionRequest {
	if data == nil {
		return nil
	}
	return assembleReactionRequest(
		value(data.MessageId),
		reactionEmoji(data.ReactionType),
		reactionUserID(data.UserId),
		"created",
	)
}

func buildReactionRequestFromDeleted(data *larkim.P2MessageReactionDeletedV1Data) *notify.ActionRequest {
	if data == nil {
		return nil
	}
	return assembleReactionRequest(
		value(data.MessageId),
		reactionEmoji(data.ReactionType),
		reactionUserID(data.UserId),
		"deleted",
	)
}

func assembleReactionRequest(messageID, emoji, userID, eventType string) *notify.ActionRequest {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil
	}
	metadata := map[string]string{
		"emoji":      emoji,
		"event_type": eventType,
	}
	return &notify.ActionRequest{
		Platform: liveMetadata.Source,
		Action:   core.ActionNameReact,
		EntityID: messageID,
		UserID:   userID,
		Metadata: compactMetadata(metadata),
		ReplyTarget: &core.ReplyTarget{
			Platform:  liveMetadata.Source,
			MessageID: messageID,
		},
	}
}

func reactionEmoji(emoji *larkim.Emoji) string {
	if emoji == nil || emoji.EmojiType == nil {
		return ""
	}
	return strings.TrimSpace(*emoji.EmojiType)
}

func reactionUserID(user *larkim.UserId) string {
	if user == nil {
		return ""
	}
	if user.OpenId != nil && strings.TrimSpace(*user.OpenId) != "" {
		return strings.TrimSpace(*user.OpenId)
	}
	if user.UserId != nil && strings.TrimSpace(*user.UserId) != "" {
		return strings.TrimSpace(*user.UserId)
	}
	return ""
}
```

Register the deleted handler. In the `lifecycleEventRunner` interface definition (line ~59), add the new parameter:

```go
type lifecycleEventRunner interface {
	StartFull(
		ctx context.Context,
		handler func(context.Context, *larkim.P2MessageReceiveV1) error,
		cardActionHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error),
		botAddedHandler func(context.Context, *larkim.P2ChatMemberBotAddedV1) error,
		botRemovedHandler func(context.Context, *larkim.P2ChatMemberBotDeletedV1) error,
		reactionCreatedHandler func(context.Context, *larkim.P2MessageReactionCreatedV1) error,
		reactionDeletedHandler func(context.Context, *larkim.P2MessageReactionDeletedV1) error,
	) error
}
```

Propagate the new parameter through **every call site**. The full cascade (grep for "StartFull" in `src-im-bridge/platform/feishu/`):

1. `lifecycleEventRunner` interface declaration — add the parameter as shown above.
2. `sdkEventRunner.Start` (~line 546) — add a trailing `nil` when delegating to `StartFull`.
3. `sdkEventRunner.StartWithCardActions` (~line 550) — add a trailing `nil`.
4. `sdkEventRunner.StartFull` (~line 553) — accept the new parameter and wire:
   ```go
   if reactionDeletedHandler != nil {
       dispatcher = dispatcher.OnP2MessageReactionDeletedV1(reactionDeletedHandler)
   }
   ```
5. `Live.Start` (~line 301) — when the runner is a `lifecycleEventRunner`, pass `l.handleReactionDeleted` as the new argument.
6. `fakeEventRunner` in `live_test.go` — add a `reactionDeletedHandler` field and accept the new arg in its `StartFull` method; add a `dispatchReactionDeleted` helper if any test needs to exercise it.

If you miss any of these, `go build ./...` will fail with "wrong number of arguments" — fix before running tests.

- [ ] **Step 6.4: Run to confirm pass**

Run: `cd src-im-bridge && go test ./platform/feishu/ -count=1`
Expected: PASS. Also run `go build ./...` to confirm the interface change propagates cleanly.

- [ ] **Step 6.5: Commit**

```bash
rtk git add src-im-bridge/platform/feishu/live.go src-im-bridge/platform/feishu/live_test.go
rtk git commit -m "feat(im-bridge/feishu): forward message reactions to action handler as react action

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 7 — Bridge: Stub `/test/cardclick` endpoint

**Files:**
- Modify: `src-im-bridge/platform/feishu/stub.go`
- Modify: `src-im-bridge/platform/feishu/stub_test.go`

- [ ] **Step 7.1: Write the failing test**

Append to `stub_test.go`:

```go
func TestStub_CardClickEndpointInvokesActionHandler(t *testing.T) {
	stub := NewStub("0")
	var captured *notify.ActionRequest
	stub.SetActionHandler(func(_ context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
		captured = req
		return &notify.ActionResponse{Result: "ok"}, nil
	})

	payload := map[string]any{
		"action":    "act:approve:review-1",
		"chat_id":   "chat-click",
		"user_id":   "ou_user",
		"message_id": "om_1",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/test/cardclick", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	stub.handleTestCardClick(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if captured == nil {
		t.Fatalf("action handler not invoked")
	}
	if captured.Action != "approve" || captured.EntityID != "review-1" {
		t.Errorf("action = %q / entity = %q", captured.Action, captured.EntityID)
	}
	if captured.ChatID != "chat-click" {
		t.Errorf("chat_id = %q", captured.ChatID)
	}
}
```

Add imports `bytes`, `encoding/json`, `net/http`, `net/http/httptest`, `notify` to `stub_test.go` if missing.

- [ ] **Step 7.2: Run to confirm failure**

Run: `cd src-im-bridge && go test ./platform/feishu/ -run 'TestStub_CardClick' -count=1`
Expected: FAIL — `SetActionHandler` and `handleTestCardClick` are not defined on `Stub`.

- [ ] **Step 7.3: Implement**

In `stub.go`:

Add field + setter:

```go
type Stub struct {
	// … existing fields
	actionHandler func(context.Context, *notify.ActionRequest) (*notify.ActionResponse, error)
}

func (s *Stub) SetActionHandler(h func(context.Context, *notify.ActionRequest) (*notify.ActionResponse, error)) {
	s.actionHandler = h
}
```

Register the route in `Start`:

```go
mux.HandleFunc("POST /test/cardclick", s.handleTestCardClick)
```

Add handler:

```go
type stubCardClickRequest struct {
	Action    string            `json:"action"`
	EntityID  string            `json:"entity_id,omitempty"`
	ChatID    string            `json:"chat_id"`
	UserID    string            `json:"user_id"`
	MessageID string            `json:"message_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func (s *Stub) handleTestCardClick(w http.ResponseWriter, r *http.Request) {
	var req stubCardClickRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if s.actionHandler == nil {
		http.Error(w, "action handler not configured", http.StatusServiceUnavailable)
		return
	}

	// Accept either raw act:<verb>:<entity> or already-parsed action/entity pair.
	actionName := strings.TrimSpace(req.Action)
	entityID := strings.TrimSpace(req.EntityID)
	if strings.HasPrefix(actionName, "act:") {
		if parsedAction, parsedEntity, _, ok := core.ParseActionReferenceWithMetadata(actionName); ok {
			actionName = parsedAction
			if entityID == "" {
				entityID = parsedEntity
			}
		}
	}

	ar := &notify.ActionRequest{
		Platform: "feishu",
		Action:   actionName,
		EntityID: entityID,
		ChatID:   req.ChatID,
		UserID:   req.UserID,
		Metadata: req.Metadata,
		ReplyTarget: &core.ReplyTarget{
			Platform:  "feishu",
			ChatID:    req.ChatID,
			ChannelID: req.ChatID,
			MessageID: req.MessageID,
			UseReply:  true,
		},
	}
	resp, err := s.actionHandler(r.Context(), ar)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if resp == nil {
		resp = &notify.ActionResponse{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"action":   actionName,
		"entityId": entityID,
		"result":   resp.Result,
		"status":   resp.Metadata["action_status"],
	})
}
```

- [ ] **Step 7.4: Run to confirm pass**

Run: `cd src-im-bridge && go test ./platform/feishu/ -count=1`
Expected: PASS.

- [ ] **Step 7.5: Commit**

```bash
rtk git add src-im-bridge/platform/feishu/stub.go src-im-bridge/platform/feishu/stub_test.go
rtk git commit -m "feat(im-bridge/feishu): add /test/cardclick endpoint to stub for end-to-end simulation

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 8 — Backend: migration for im_reaction_events

**Files:**
- Create: `src-go/migrations/054_create_im_reaction_events.up.sql`
- Create: `src-go/migrations/054_create_im_reaction_events.down.sql`

- [ ] **Step 8.1: Write the up migration**

Create `src-go/migrations/054_create_im_reaction_events.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS im_reaction_events (
    id          UUID PRIMARY KEY,
    platform    TEXT NOT NULL,
    chat_id     TEXT NOT NULL DEFAULT '',
    message_id  TEXT NOT NULL,
    user_id     TEXT NOT NULL DEFAULT '',
    emoji       TEXT NOT NULL DEFAULT '',
    event_type  TEXT NOT NULL CHECK (event_type IN ('created', 'deleted')),
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS im_reaction_events_message_created_idx
    ON im_reaction_events (message_id, created_at DESC);

CREATE INDEX IF NOT EXISTS im_reaction_events_platform_created_idx
    ON im_reaction_events (platform, created_at DESC);
```

- [ ] **Step 8.2: Write the down migration**

Create `src-go/migrations/054_create_im_reaction_events.down.sql`:

```sql
DROP INDEX IF EXISTS im_reaction_events_platform_created_idx;
DROP INDEX IF EXISTS im_reaction_events_message_created_idx;
DROP TABLE IF EXISTS im_reaction_events;
```

- [ ] **Step 8.3: Verify migration files are embedded**

Run: `cd src-go && go build ./migrations/...`
Expected: success (the `//go:embed *.sql` in `embed.go` automatically picks up new files).

- [ ] **Step 8.4: Confirm by running migration test** (if one exists)

Run: `cd src-go && go test ./migrations/ -count=1`
Expected: PASS.

- [ ] **Step 8.5: Commit**

```bash
rtk git add src-go/migrations/054_create_im_reaction_events.up.sql src-go/migrations/054_create_im_reaction_events.down.sql
rtk git commit -m "feat(backend): add im_reaction_events migration for Feishu reaction audit trail

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 9 — Backend: IMReactionEvent model + repository

**Files:**
- Create: `src-go/internal/model/im_reaction_event.go`
- Create: `src-go/internal/repository/im_reaction_event_repo.go`
- Create: `src-go/internal/repository/im_reaction_event_repo_test.go`

- [ ] **Step 9.1: Write the model**

Create `src-go/internal/model/im_reaction_event.go`:

```go
package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	IMReactionEventTypeCreated = "created"
	IMReactionEventTypeDeleted = "deleted"
)

type IMReactionEvent struct {
	ID         uuid.UUID `json:"id"`
	Platform   string    `json:"platform"`
	ChatID     string    `json:"chatId"`
	MessageID  string    `json:"messageId"`
	UserID     string    `json:"userId"`
	Emoji      string    `json:"emoji"`
	EventType  string    `json:"eventType"`
	RawPayload []byte    `json:"-"`
	CreatedAt  time.Time `json:"createdAt"`
}
```

- [ ] **Step 9.2: Write the repository test first**

Create `src-go/internal/repository/im_reaction_event_repo_test.go`:

```go
package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/agentforge/server/internal/model"
)

func TestIMReactionEventRepository_RecordRoundTrip(t *testing.T) {
	db := newTestDB(t) // helper used elsewhere in this package
	repo := NewIMReactionEventRepository(db)

	event := &model.IMReactionEvent{
		Platform:   "feishu",
		ChatID:     "chat-1",
		MessageID:  "om_abc",
		UserID:     "ou_reactor",
		Emoji:      "THUMBSUP",
		EventType:  model.IMReactionEventTypeCreated,
		RawPayload: []byte(`{"a":1}`),
	}
	if err := repo.Record(context.Background(), event); err != nil {
		t.Fatalf("Record error: %v", err)
	}
	if event.ID == uuid.Nil {
		t.Fatal("expected Record to set ID")
	}
	if event.CreatedAt.IsZero() {
		t.Fatal("expected Record to set CreatedAt")
	}

	stored, err := repo.ListByMessage(context.Background(), "om_abc", 10)
	if err != nil {
		t.Fatalf("ListByMessage error: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("len(stored)=%d", len(stored))
	}
	if stored[0].Emoji != "THUMBSUP" {
		t.Errorf("Emoji = %q", stored[0].Emoji)
	}
	var raw map[string]int
	if err := json.Unmarshal(stored[0].RawPayload, &raw); err != nil || raw["a"] != 1 {
		t.Errorf("raw payload round-trip failed: raw=%v err=%v", raw, err)
	}
	if time.Since(stored[0].CreatedAt) > time.Minute {
		t.Errorf("CreatedAt unexpected: %v", stored[0].CreatedAt)
	}
}

func TestIMReactionEventRepository_ValidatesEventType(t *testing.T) {
	db := newTestDB(t)
	repo := NewIMReactionEventRepository(db)

	err := repo.Record(context.Background(), &model.IMReactionEvent{
		Platform:  "feishu",
		MessageID: "om_abc",
		EventType: "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid event_type")
	}
}
```

Note: `newTestDB(t)` already exists in the repository package (used by `task_comment_repo_test.go` etc.). Inspect `task_comment_repo_test.go` for the helper name if it differs.

- [ ] **Step 9.3: Run to confirm failure**

Run: `cd src-go && go test ./internal/repository/ -run 'TestIMReactionEventRepository' -count=1`
Expected: FAIL — `NewIMReactionEventRepository` not defined.

- [ ] **Step 9.4: Implement the repository**

Create `src-go/internal/repository/im_reaction_event_repo.go`:

```go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/agentforge/server/internal/model"
	"gorm.io/gorm"
)

type IMReactionEventRepository struct {
	db *gorm.DB
}

func NewIMReactionEventRepository(db *gorm.DB) *IMReactionEventRepository {
	return &IMReactionEventRepository{db: db}
}

type imReactionEventRecord struct {
	ID         uuid.UUID `gorm:"column:id;primaryKey"`
	Platform   string    `gorm:"column:platform"`
	ChatID     string    `gorm:"column:chat_id"`
	MessageID  string    `gorm:"column:message_id"`
	UserID     string    `gorm:"column:user_id"`
	Emoji      string    `gorm:"column:emoji"`
	EventType  string    `gorm:"column:event_type"`
	RawPayload []byte    `gorm:"column:raw_payload;type:jsonb"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (imReactionEventRecord) TableName() string { return "im_reaction_events" }

func (r *IMReactionEventRepository) Record(ctx context.Context, event *model.IMReactionEvent) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if event == nil {
		return fmt.Errorf("im_reaction_event is required")
	}
	if event.EventType != model.IMReactionEventTypeCreated && event.EventType != model.IMReactionEventTypeDeleted {
		return fmt.Errorf("invalid event_type %q", event.EventType)
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	raw := event.RawPayload
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	record := imReactionEventRecord{
		ID:         event.ID,
		Platform:   event.Platform,
		ChatID:     event.ChatID,
		MessageID:  event.MessageID,
		UserID:     event.UserID,
		Emoji:      event.Emoji,
		EventType:  event.EventType,
		RawPayload: raw,
		CreatedAt:  event.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("record im reaction event: %w", err)
	}
	return nil
}

func (r *IMReactionEventRepository) ListByMessage(ctx context.Context, messageID string, limit int) ([]model.IMReactionEvent, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 100
	}
	var records []imReactionEventRecord
	if err := r.db.WithContext(ctx).
		Where("message_id = ?", messageID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list im reaction events by message: %w", err)
	}
	out := make([]model.IMReactionEvent, 0, len(records))
	for _, rec := range records {
		out = append(out, model.IMReactionEvent{
			ID:         rec.ID,
			Platform:   rec.Platform,
			ChatID:     rec.ChatID,
			MessageID:  rec.MessageID,
			UserID:     rec.UserID,
			Emoji:      rec.Emoji,
			EventType:  rec.EventType,
			RawPayload: rec.RawPayload,
			CreatedAt:  rec.CreatedAt,
		})
	}
	return out, nil
}
```

- [ ] **Step 9.5: Run to confirm pass**

Run: `cd src-go && go test ./internal/repository/ -run 'TestIMReactionEventRepository' -count=1`
Expected: PASS.

- [ ] **Step 9.6: Commit**

```bash
rtk git add src-go/internal/model/im_reaction_event.go src-go/internal/repository/im_reaction_event_repo.go src-go/internal/repository/im_reaction_event_repo_test.go
rtk git commit -m "feat(backend): add IMReactionEvent model and repository

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 10 — Backend: executeReact action case

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

- [ ] **Step 10.1: Write the failing test**

Append to `im_action_execution_test.go`:

```go
type fakeReactionRecorder struct {
	recorded []*model.IMReactionEvent
	err      error
}

func (r *fakeReactionRecorder) Record(ctx context.Context, event *model.IMReactionEvent) error {
	if r.err != nil {
		return r.err
	}
	r.recorded = append(r.recorded, event)
	return nil
}

func TestExecuteReact_RecordsEventAndReturnsCompleted(t *testing.T) {
	recorder := &fakeReactionRecorder{}
	exec := NewBackendIMActionExecutor(nil, nil, nil, recorder)

	req := &model.IMActionRequest{
		Platform:  "feishu",
		Action:    "react",
		EntityID:  "om_msg_1",
		ChannelID: "chat-1",
		UserID:    "ou_user",
		Metadata: map[string]string{
			"emoji":      "THUMBSUP",
			"event_type": "created",
		},
	}
	resp, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if resp.Status != model.IMActionStatusCompleted {
		t.Errorf("Status = %q, want completed", resp.Status)
	}
	if resp.Result != "" {
		t.Errorf("expected empty Result (so bridge does not post a reply); got %q", resp.Result)
	}
	if len(recorder.recorded) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(recorder.recorded))
	}
	rec := recorder.recorded[0]
	if rec.MessageID != "om_msg_1" || rec.Emoji != "THUMBSUP" || rec.EventType != "created" {
		t.Errorf("recorded event = %+v", rec)
	}
}

func TestExecuteReact_WithoutRecorderReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "react",
		EntityID: "om_msg_1",
		Metadata: map[string]string{"emoji": "OK", "event_type": "created"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}
```

- [ ] **Step 10.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteReact' -count=1`
Expected: FAIL — Execute returns "Unknown action".

- [ ] **Step 10.3: Implement**

In `im_action_execution.go`:

Add interface near top (after other IMAction* interfaces):

```go
type IMReactionRecorder interface {
	Record(ctx context.Context, event *model.IMReactionEvent) error
}
```

Add field to `BackendIMActionExecutor`:

```go
type BackendIMActionExecutor struct {
	// … existing fields
	reactions IMReactionRecorder
}
```

Extend the `NewBackendIMActionExecutor` extras type-switch:

```go
case IMReactionRecorder:
	executor.reactions = value
```

Add `"react"` case in `Execute`:

```go
case "react":
	return e.executeReact(ctx, req), nil
```

Add the method:

```go
func (e *BackendIMActionExecutor) executeReact(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	if e.reactions == nil {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Reaction recording is not configured.", false)
	}
	messageID := strings.TrimSpace(req.EntityID)
	if messageID == "" {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Reaction event missing message id.", false)
	}
	eventType := strings.TrimSpace(req.Metadata["event_type"])
	if eventType == "" {
		eventType = model.IMReactionEventTypeCreated
	}
	if eventType != model.IMReactionEventTypeCreated && eventType != model.IMReactionEventTypeDeleted {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Unknown reaction event type %q.", eventType), false)
	}
	event := &model.IMReactionEvent{
		Platform:  strings.TrimSpace(req.Platform),
		ChatID:    strings.TrimSpace(req.ChannelID),
		MessageID: messageID,
		UserID:    strings.TrimSpace(req.UserID),
		Emoji:     strings.TrimSpace(req.Metadata["emoji"]),
		EventType: eventType,
	}
	if err := e.reactions.Record(ctx, event); err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Record reaction failed: %s", err.Error()), false)
	}
	resp := newIMActionResponse(req, model.IMActionStatusCompleted, "", true)
	// Empty Result avoids posting a visible reply back to the chat.
	return resp
}
```

- [ ] **Step 10.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteReact' -count=1`
Expected: PASS.

- [ ] **Step 10.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): dispatch react action to IMReactionRecorder

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 11 — Backend: `executeSelect` with target_action whitelist

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

- [ ] **Step 11.1: Write the failing tests**

Append:

```go
func TestExecuteSelect_WithoutTargetActionReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "select",
		EntityID: "task-xyz",
		Metadata: map[string]string{"selected_option": "agent-alpha"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
	if !strings.Contains(resp.Result, "target_action") {
		t.Errorf("expected mention of target_action in %q", resp.Result)
	}
}

func TestExecuteSelect_WithInvalidTargetActionReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "select",
		EntityID: "task-xyz",
		Metadata: map[string]string{"target_action": "dangerous-internal-op", "selected_option": "x"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
	if !strings.Contains(resp.Result, "not allowed") {
		t.Errorf("Result = %q", resp.Result)
	}
}

func TestExecuteSelect_WithWhitelistedTargetActionDispatches(t *testing.T) {
	dispatcher := &fakeIMActionDispatcher{
		assignResp: &model.TaskDispatchResponse{
			Dispatch: model.DispatchOutcome{Status: model.DispatchStatusStarted},
			Task:     model.TaskDTO{ID: uuid.New().String()},
		},
	}
	exec := NewBackendIMActionExecutor(dispatcher, nil, nil)
	taskID := uuid.New()
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "select",
		EntityID: taskID.String(),
		Metadata: map[string]string{
			"target_action":   "assign-agent",
			"selected_option": "agent-alpha",
		},
	}
	resp, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if resp.Status == model.IMActionStatusBlocked || resp.Status == model.IMActionStatusFailed {
		t.Errorf("unexpected status %q; result=%q", resp.Status, resp.Result)
	}
	// The selected_option should have been promoted into assigneeId
	if dispatcher.lastAssign == nil || dispatcher.lastAssign.AssigneeID != "agent-alpha" {
		t.Errorf("assigneeId not propagated: %+v", dispatcher.lastAssign)
	}
}
```

**Existing fakes to reuse** (defined in `im_action_execution_test.go`): `fakeIMActionDispatcher` (field: `lastAssign`), `fakeIMActionDecomposer` (fields: `last`, `resp`, `err`), `fakeIMActionTaskCreator` (field: `created`), `fakeIMActionTaskTransitioner` (fields: `getTask`, `lastTaskID`, `lastTargetState`, `transitionErr`). Use these exact names throughout the Phase 1 tests rather than inventing new ones.

- [ ] **Step 11.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteSelect' -count=1`
Expected: FAIL.

- [ ] **Step 11.3: Implement**

Add the whitelist constant and helper near the top of `im_action_execution.go`:

```go
var allowedSelectTargetActions = map[string]struct{}{
	"assign-agent":    {},
	"decompose":       {},
	"transition-task": {},
	"move-task":       {},
	"save-as-doc":     {},
	"create-task":     {},
	"approve":         {},
	"request-changes": {},
}

func isAllowedTargetAction(action string) bool {
	_, ok := allowedSelectTargetActions[strings.TrimSpace(action)]
	return ok
}
```

Add `"select"` case in `Execute`:

```go
case "select":
	return e.executeSelect(ctx, req), nil
```

Add method:

```go
func (e *BackendIMActionExecutor) executeSelect(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	target := strings.TrimSpace(req.Metadata["target_action"])
	if target == "" {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Select action requires target_action metadata.", false)
	}
	if !isAllowedTargetAction(target) {
		return newIMActionResponse(req, model.IMActionStatusBlocked, fmt.Sprintf("target_action %q is not allowed.", target), false)
	}

	// Rewrite req and dispatch to the real action.
	delegated := *req
	delegated.Action = target
	delegated.Metadata = cloneStringMap(req.Metadata)
	// Promote selected_option into the common metadata keys each target expects.
	if opt := strings.TrimSpace(delegated.Metadata["selected_option"]); opt != "" {
		switch target {
		case "assign-agent":
			if delegated.Metadata["assigneeId"] == "" {
				delegated.Metadata["assigneeId"] = opt
			}
		case "transition-task", "move-task":
			if delegated.Metadata["targetStatus"] == "" {
				delegated.Metadata["targetStatus"] = opt
			}
		}
	}
	return e.dispatchAllowedAction(ctx, &delegated)
}

// dispatchAllowedAction invokes the whitelisted action. It exists so executeSelect,
// executeMultiSelect, and executeOverflow share the same dispatch path without
// touching the top-level Execute switch.
func (e *BackendIMActionExecutor) dispatchAllowedAction(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	resp, _ := e.Execute(ctx, req)
	return resp
}
```

**Important**: `dispatchAllowedAction` calls `Execute` again. We prevent recursion by only calling for **whitelisted** (real) actions, so select→select is impossible (select is not in the whitelist).

- [ ] **Step 11.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteSelect' -count=1`
Expected: PASS.

- [ ] **Step 11.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): dispatch select action via whitelisted target_action

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 12 — Backend: `executeMultiSelect`

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

- [ ] **Step 12.1: Write the failing test**

```go
func TestExecuteMultiSelect_PromotesSelectedOptionsAsCSV(t *testing.T) {
	dispatcher := &fakeIMActionDispatcher{
		assignResp: &model.TaskDispatchResponse{
			Dispatch: model.DispatchOutcome{Status: model.DispatchStatusStarted},
			Task:     model.TaskDTO{ID: uuid.New().String()},
		},
	}
	exec := NewBackendIMActionExecutor(dispatcher, nil, nil)
	taskID := uuid.New()
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "multi_select",
		EntityID: taskID.String(),
		Metadata: map[string]string{
			"target_action":    "assign-agent",
			"selected_options": "agent-a,agent-b",
		},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status == model.IMActionStatusFailed {
		t.Fatalf("unexpected failure: %q", resp.Result)
	}
	// First option becomes assigneeId; full list stays in metadata.
	if dispatcher.lastAssign == nil || dispatcher.lastAssign.AssigneeID != "agent-a" {
		t.Errorf("AssigneeID = %+v", dispatcher.lastAssign)
	}
}

func TestExecuteMultiSelect_WithoutTargetActionReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "multi_select",
		Metadata: map[string]string{"selected_options": "a,b"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}
```

- [ ] **Step 12.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteMultiSelect' -count=1`
Expected: FAIL.

- [ ] **Step 12.3: Implement**

Add case:

```go
case "multi_select":
	return e.executeMultiSelect(ctx, req), nil
```

Method:

```go
func (e *BackendIMActionExecutor) executeMultiSelect(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	target := strings.TrimSpace(req.Metadata["target_action"])
	if target == "" {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Multi-select action requires target_action metadata.", false)
	}
	if !isAllowedTargetAction(target) {
		return newIMActionResponse(req, model.IMActionStatusBlocked, fmt.Sprintf("target_action %q is not allowed.", target), false)
	}

	delegated := *req
	delegated.Action = target
	delegated.Metadata = cloneStringMap(req.Metadata)
	options := splitCSV(delegated.Metadata["selected_options"])
	if len(options) > 0 {
		switch target {
		case "assign-agent":
			if delegated.Metadata["assigneeId"] == "" {
				delegated.Metadata["assigneeId"] = options[0]
			}
		}
	}
	return e.dispatchAllowedAction(ctx, &delegated)
}

func splitCSV(s string) []string {
	raw := strings.Split(strings.TrimSpace(s), ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
```

- [ ] **Step 12.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteMultiSelect' -count=1`
Expected: PASS.

- [ ] **Step 12.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): dispatch multi_select action and promote first option to target params

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 13 — Backend: `executeToggle` transitions task

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

- [ ] **Step 13.1: Write the failing tests**

```go
func TestExecuteToggle_TrueTransitionsTaskToDone(t *testing.T) {
	mover := &fakeIMActionTaskTransitioner{
		getTask: &model.Task{ID: uuid.New(), Title: "t", Status: model.TaskStatusInProgress},
	}
	exec := NewBackendIMActionExecutor(nil, nil, nil, mover)
	taskID := uuid.New()
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "toggle",
		EntityID: taskID.String(),
		Metadata: map[string]string{"checker_state": "true"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusCompleted {
		t.Errorf("Status = %q, want completed", resp.Status)
	}
	if mover.lastTargetState != model.TaskStatusDone {
		t.Errorf("TargetState = %q, want done", mover.lastTargetState)
	}
}

func TestExecuteToggle_FalseReopensTask(t *testing.T) {
	mover := &fakeIMActionTaskTransitioner{
		getTask: &model.Task{ID: uuid.New(), Title: "t", Status: model.TaskStatusDone},
	}
	exec := NewBackendIMActionExecutor(nil, nil, nil, mover)
	taskID := uuid.New()
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "toggle",
		EntityID: taskID.String(),
		Metadata: map[string]string{"checker_state": "false"},
	}
	_, _ = exec.Execute(context.Background(), req)
	if mover.lastTargetState != model.TaskStatusInProgress {
		t.Errorf("TargetState = %q, want in_progress", mover.lastTargetState)
	}
}

func TestExecuteToggle_WithoutMoverReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "toggle",
		EntityID: uuid.New().String(),
		Metadata: map[string]string{"checker_state": "true"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}
```

- [ ] **Step 13.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteToggle' -count=1`
Expected: FAIL.

- [ ] **Step 13.3: Implement**

Add case:

```go
case "toggle":
	return e.executeToggle(ctx, req), nil
```

Method:

```go
func (e *BackendIMActionExecutor) executeToggle(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	if e.taskMover == nil {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Task transition workflow is unavailable.", false)
	}
	taskID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid task identifier.", false)
	}
	checkerState := strings.ToLower(strings.TrimSpace(req.Metadata["checker_state"]))
	targetStatus := model.TaskStatusInProgress
	if checkerState == "true" {
		targetStatus = model.TaskStatusDone
	}
	updated, err := e.taskMover.Transition(ctx, taskID, &model.TransitionRequest{Status: targetStatus})
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Toggle failed: %s", err.Error()), false)
	}
	resp := newIMActionResponse(req, model.IMActionStatusCompleted, fmt.Sprintf("Task %s set to %s.", updated.Title, targetStatus), true)
	dto := updated.ToDTO()
	resp.Task = &dto
	return resp
}
```

- [ ] **Step 13.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteToggle' -count=1`
Expected: PASS.

- [ ] **Step 13.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): dispatch toggle action to task transition

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 14 — Backend: `executeInputSubmit` appends task comment

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

- [ ] **Step 14.1: Write the failing tests**

```go
type fakeTaskCommenter struct {
	captured *model.TaskComment
	err      error
}

func (f *fakeTaskCommenter) Create(_ context.Context, comment *model.TaskComment) error {
	if f.err != nil {
		return f.err
	}
	f.captured = comment
	return nil
}

func TestExecuteInputSubmit_AppendsComment(t *testing.T) {
	commenter := &fakeTaskCommenter{}
	exec := NewBackendIMActionExecutor(nil, nil, nil, commenter)
	taskID := uuid.New()
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "input_submit",
		EntityID: taskID.String(),
		Metadata: map[string]string{"input_value": "please reconsider"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusCompleted {
		t.Errorf("Status = %q, want completed", resp.Status)
	}
	if commenter.captured == nil || commenter.captured.Body != "please reconsider" {
		t.Errorf("captured comment = %+v", commenter.captured)
	}
	if commenter.captured.TaskID != taskID {
		t.Errorf("TaskID = %s, want %s", commenter.captured.TaskID, taskID)
	}
}

func TestExecuteInputSubmit_WithoutValueReturnsFailed(t *testing.T) {
	commenter := &fakeTaskCommenter{}
	exec := NewBackendIMActionExecutor(nil, nil, nil, commenter)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "input_submit",
		EntityID: uuid.New().String(),
		Metadata: map[string]string{},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusFailed {
		t.Errorf("Status = %q, want failed", resp.Status)
	}
}

func TestExecuteInputSubmit_WithoutCommenterReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "input_submit",
		EntityID: uuid.New().String(),
		Metadata: map[string]string{"input_value": "hi"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}
```

- [ ] **Step 14.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteInputSubmit' -count=1`
Expected: FAIL.

- [ ] **Step 14.3: Implement**

Add interface:

```go
type IMActionTaskCommenter interface {
	Create(ctx context.Context, comment *model.TaskComment) error
}
```

Add field, extras case, and method:

```go
type BackendIMActionExecutor struct {
	// …
	commenter IMActionTaskCommenter
}

// in NewBackendIMActionExecutor:
case IMActionTaskCommenter:
	executor.commenter = value
```

Dispatch case:

```go
case "input_submit":
	return e.executeInputSubmit(ctx, req), nil
```

Method:

```go
func (e *BackendIMActionExecutor) executeInputSubmit(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	if e.commenter == nil {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Task comment workflow is unavailable.", false)
	}
	body := strings.TrimSpace(req.Metadata["input_value"])
	if body == "" {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Input submission missing value.", false)
	}
	taskID, err := parseIMEntityUUID(req.EntityID)
	if err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, "Invalid task identifier.", false)
	}
	comment := &model.TaskComment{
		TaskID: taskID,
		Body:   body,
	}
	// Best-effort author: if req.UserID is a parseable UUID, record it; otherwise
	// leave zero-UUID. (Feishu open_id is not a UUID, so this usually stays zero
	// in Phase 1. A later phase will map IM identities to internal user UUIDs.)
	if authorID, err := uuid.Parse(strings.TrimSpace(req.UserID)); err == nil {
		comment.CreatedBy = authorID
	}
	if err := e.commenter.Create(ctx, comment); err != nil {
		return newIMActionResponse(req, model.IMActionStatusFailed, fmt.Sprintf("Failed to append comment: %s", err.Error()), false)
	}
	return newIMActionResponse(req, model.IMActionStatusCompleted, "Comment appended to task.", true)
}
```

- [ ] **Step 14.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteInputSubmit' -count=1`
Expected: PASS.

- [ ] **Step 14.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): dispatch input_submit action to task comment repository

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 15 — Backend: `executeDatePick` returns Blocked with clear reason

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

Since the task model has no `due_date` field yet (confirmed during planning), Phase 1 returns Blocked with an explicit message. A later phase introduces the storage.

- [ ] **Step 15.1: Write the failing test**

```go
func TestExecuteDatePick_ReturnsBlockedWithReason(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "date_pick",
		EntityID: uuid.New().String(),
		Metadata: map[string]string{"selected_time": "2026-04-20"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
	if !strings.Contains(resp.Result, "Due-date") {
		t.Errorf("Result = %q", resp.Result)
	}
}
```

- [ ] **Step 15.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteDatePick' -count=1`
Expected: FAIL — returns "Unknown action".

- [ ] **Step 15.3: Implement**

Add case + method:

```go
case "date_pick":
	return e.executeDatePick(ctx, req), nil

func (e *BackendIMActionExecutor) executeDatePick(_ context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	selected := strings.TrimSpace(req.Metadata["selected_time"])
	reason := "Due-date workflow is not configured; date picker click received."
	if selected != "" {
		reason = fmt.Sprintf("Due-date workflow is not configured; received %s but could not record it.", selected)
	}
	return newIMActionResponse(req, model.IMActionStatusBlocked, reason, false)
}
```

- [ ] **Step 15.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteDatePick' -count=1`
Expected: PASS.

- [ ] **Step 15.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): acknowledge date_pick with Blocked status until due-date storage lands

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 16 — Backend: `executeOverflow` with recursion guard

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

- [ ] **Step 16.1: Write the failing tests**

```go
func TestExecuteOverflow_ParsesInnerActionReferenceAndDispatches(t *testing.T) {
	decomposer := &fakeIMActionDecomposer{
		resp: &model.TaskDecompositionResponse{
			ParentTask: model.TaskDTO{ID: uuid.New().String()},
		},
	}
	exec := NewBackendIMActionExecutor(nil, decomposer, nil)
	taskID := uuid.New()
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "overflow",
		Metadata: map[string]string{"selected_option": fmt.Sprintf("act:decompose:%s", taskID)},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status == model.IMActionStatusBlocked || resp.Status == model.IMActionStatusFailed {
		t.Errorf("unexpected %q: %q", resp.Status, resp.Result)
	}
	if decomposer.last != taskID {
		t.Errorf("decomposer called with %s, want %s", decomposer.last, taskID)
	}
}

func TestExecuteOverflow_RefusesNonAllowedTargetAction(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "overflow",
		Metadata: map[string]string{"selected_option": "act:select:x?target_action=dangerous"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}

func TestExecuteOverflow_InvalidSelectedOption(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "overflow",
		Metadata: map[string]string{"selected_option": "not-an-action-ref"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}
```

Use the same imports (`github.com/agentforge/server/internal/core` or whatever the bridge package alias is — the backend should not import the bridge; use an inline `parseActionRef` helper if needed, or copy the logic from `src-im-bridge/core/action_reference.go` into a backend utility).

**Note:** The backend cannot import the bridge package. Create a small helper in `im_action_execution.go` or a sibling file to parse `act:<verb>:<entity>?k=v&...`.

- [ ] **Step 16.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteOverflow' -count=1`
Expected: FAIL.

- [ ] **Step 16.3: Implement**

Add action-reference parsing helper in `im_action_execution.go`:

```go
func parseBackendActionReference(raw string) (action, entityID string, metadata map[string]string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "act:") {
		return "", "", nil, false
	}
	body := strings.TrimPrefix(trimmed, "act:")
	queryString := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		queryString = body[idx+1:]
		body = body[:idx]
	}
	parts := strings.SplitN(body, ":", 2)
	if len(parts) != 2 {
		return "", "", nil, false
	}
	action = strings.TrimSpace(parts[0])
	entityID = strings.TrimSpace(parts[1])
	if action == "" || entityID == "" {
		return "", "", nil, false
	}
	if queryString != "" {
		values, err := url.ParseQuery(queryString)
		if err == nil && len(values) > 0 {
			metadata = make(map[string]string, len(values))
			for key, entries := range values {
				if len(entries) == 0 {
					continue
				}
				value := strings.TrimSpace(entries[len(entries)-1])
				if strings.TrimSpace(key) == "" || value == "" {
					continue
				}
				metadata[strings.TrimSpace(key)] = value
			}
		}
	}
	return action, entityID, metadata, true
}
```

Import `net/url`.

Add case and method:

```go
case "overflow":
	return e.executeOverflow(ctx, req), nil

func (e *BackendIMActionExecutor) executeOverflow(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	opt := strings.TrimSpace(req.Metadata["selected_option"])
	innerAction, innerEntity, innerMeta, ok := parseBackendActionReference(opt)
	if !ok {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Overflow selection is not a valid action reference.", false)
	}
	if !isAllowedTargetAction(innerAction) {
		return newIMActionResponse(req, model.IMActionStatusBlocked, fmt.Sprintf("Overflow target action %q is not allowed.", innerAction), false)
	}
	delegated := *req
	delegated.Action = innerAction
	delegated.EntityID = innerEntity
	delegated.Metadata = cloneStringMap(req.Metadata)
	for k, v := range innerMeta {
		delegated.Metadata[k] = v
	}
	return e.dispatchAllowedAction(ctx, &delegated)
}
```

Because `isAllowedTargetAction` is the same whitelist used by `executeSelect`, the overflow cannot recurse into another `select`/`multi_select`/`overflow` — enforcing the "depth-1" constraint.

- [ ] **Step 16.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteOverflow' -count=1`
Expected: PASS.

- [ ] **Step 16.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): dispatch overflow action to inner action reference with recursion guard

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 17 — Backend: `executeFormSubmit` dispatches by element_name

**Files:**
- Modify: `src-go/internal/service/im_action_execution.go`
- Modify: `src-go/internal/service/im_action_execution_test.go`

- [ ] **Step 17.1: Write the failing tests**

```go
func TestExecuteFormSubmit_CreateTaskForm(t *testing.T) {
	taskMaker := &fakeIMActionTaskCreator{}
	exec := NewBackendIMActionExecutor(nil, nil, nil, taskMaker)
	projectID := uuid.New()
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "form_submit",
		EntityID: projectID.String(),
		Metadata: map[string]string{
			"element_name":  "create-task-form",
			"form_title":    "fix login",
			"form_body":     "users cannot log in with SSO",
			"form_priority": "high",
		},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusCompleted {
		t.Errorf("Status = %q: %q", resp.Status, resp.Result)
	}
	if taskMaker.created == nil || taskMaker.created.Title != "fix login" {
		t.Errorf("task = %+v", taskMaker.created)
	}
}

func TestExecuteFormSubmit_UnknownFormReturnsBlocked(t *testing.T) {
	exec := NewBackendIMActionExecutor(nil, nil, nil)
	req := &model.IMActionRequest{
		Platform: "feishu",
		Action:   "form_submit",
		Metadata: map[string]string{"element_name": "not-a-real-form"},
	}
	resp, _ := exec.Execute(context.Background(), req)
	if resp.Status != model.IMActionStatusBlocked {
		t.Errorf("Status = %q, want blocked", resp.Status)
	}
}
```

- [ ] **Step 17.2: Run to confirm failure**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteFormSubmit' -count=1`
Expected: FAIL.

- [ ] **Step 17.3: Implement**

Add case and method:

```go
case "form_submit":
	return e.executeFormSubmit(ctx, req), nil

func (e *BackendIMActionExecutor) executeFormSubmit(ctx context.Context, req *model.IMActionRequest) *model.IMActionResponse {
	formID := strings.TrimSpace(req.Metadata["element_name"])
	if formID == "" {
		return newIMActionResponse(req, model.IMActionStatusBlocked, "Form submit missing element_name.", false)
	}
	switch formID {
	case "create-task-form":
		delegated := *req
		delegated.Action = "create-task"
		delegated.Metadata = cloneStringMap(req.Metadata)
		// The form_* prefix is what the bridge uses; strip it for the target action.
		delegated.Metadata["title"] = firstNonEmptyMetadata(delegated.Metadata, "", "form_title", "title")
		delegated.Metadata["body"] = firstNonEmptyMetadata(delegated.Metadata, "", "form_body", "body")
		if p := firstNonEmptyMetadata(delegated.Metadata, "", "form_priority", "priority"); p != "" {
			delegated.Metadata["priority"] = p
		}
		return e.dispatchAllowedAction(ctx, &delegated)
	default:
		return newIMActionResponse(req, model.IMActionStatusBlocked, fmt.Sprintf("Unknown form %q; no handler configured.", formID), false)
	}
}
```

- [ ] **Step 17.4: Run to confirm pass**

Run: `cd src-go && go test ./internal/service/ -run 'TestExecuteFormSubmit' -count=1`
Expected: PASS.

Also run the entire service package: `go test ./internal/service/ -count=1`
Expected: PASS. No regressions on existing 7 actions.

- [ ] **Step 17.5: Commit**

```bash
rtk git add src-go/internal/service/im_action_execution.go src-go/internal/service/im_action_execution_test.go
rtk git commit -m "feat(backend): dispatch form_submit action via element_name registry

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 18 — Wire repository dependencies at composition root

**Files:**
- Modify: `src-go/internal/server/routes.go` or wherever `NewBackendIMActionExecutor` is constructed (grep the repo)

- [ ] **Step 18.1: Locate the composition site**

Run: `cd src-go && grep -n "NewBackendIMActionExecutor" --include="*.go" -r .`
Note the files where the executor is built.

- [ ] **Step 18.2: Inject new dependencies**

At each construction site, add the two new repositories to the variadic `extras`:

```go
executor := service.NewBackendIMActionExecutor(
    dispatcher,
    decomposer,
    reviewer,
    taskCreator,
    taskTransitioner,
    bindingWriter,
    progressRecorder,
    workflowEvaluator,
    wikiCreator,
    // new for Phase 1:
    reactionEventRepo,
    taskCommentRepo,
)
```

Where `reactionEventRepo := repository.NewIMReactionEventRepository(db)` and `taskCommentRepo := repository.NewTaskCommentRepository(db)` (it likely already exists if `executeCreateTask` uses it; if so, reuse the same instance).

- [ ] **Step 18.3: Build & run full test suite**

Run: `cd src-go && go build ./... && go test ./... -count=1`
Expected: full suite green. No regressions.

- [ ] **Step 18.4: Commit**

```bash
rtk git add src-go/internal/server/routes.go   # or the files touched
rtk git commit -m "feat(backend): wire IM reaction recorder and comment repo into action executor

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 19 — Smoke test documentation update

**Files:**
- Modify: `src-im-bridge/docs/platform-runbook.md` (or add a new `phase1-feishu-callback.md`)

- [ ] **Step 19.1: Draft the manual-test checklist**

Add a dedicated section at the end of `src-im-bridge/docs/platform-runbook.md`:

```markdown
## Phase 1 Feishu Callback Closure Smoke

Requires:
- Feishu live bridge running with `FEISHU_APP_ID`, `FEISHU_APP_SECRET`, and (optional) callback webhook config
- Go backend running with migration 054 applied
- A card published in a Feishu test chat that contains one instance of each element type

Steps:

1. **Button (existing behavior)** — click approve button on a card with action ref `act:approve:<reviewID>`. Expect review transitions to approved, toast "Review … was approved".
2. **Select** — click a `select_static` whose value is `{"action": "act:transition-task:<taskID>"}` and options `["inbox", "triaged", …]`. Pick "done". Expect task transitions, toast success.
3. **Multi-select** — pick two agents on a multi_select with value `{"action": "act:assign-agent:<taskID>"}`. Expect task assigned to the first agent, `selected_options` in backend logs.
4. **Date picker** — pick a date. Expect Blocked toast "Due-date workflow is not configured; received YYYY-MM-DD …".
5. **Overflow** — pick an option whose value is `act:decompose:<taskID>`. Expect task decomposed.
6. **Checker** — toggle on a card whose value is `act:toggle:<taskID>`. Expect task moves to done; toggle back → task moves to in_progress.
7. **Input** — type "please reconsider" and submit on a card whose value is `act:input_submit:<taskID>`. Expect comment appended.
8. **Form** — submit a form with `name="create-task-form"` and fields `title`, `body`, `priority`. Expect task created.
9. **Reaction** — react with 👍 on a task notification message. Expect row in `im_reaction_events` with `emoji="THUMBSUP"` and `event_type="created"`. Remove the reaction — expect row with `event_type="deleted"`.

Each step must produce a deterministic toast or status — none should show "Unknown action".
```

- [ ] **Step 19.2: Commit**

```bash
rtk git add src-im-bridge/docs/platform-runbook.md
rtk git commit -m "docs(im-bridge): document Phase 1 Feishu callback closure smoke test

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Final verification (after all tasks complete)

- [ ] **Full Go test suite passes**
  - `cd src-im-bridge && go test ./... -count=1`
  - `cd src-go && go test ./... -count=1`

- [ ] **Lint clean** (if linters configured)
  - `cd src-im-bridge && go vet ./...`
  - `cd src-go && go vet ./...`

- [ ] **Manual smoke test** per Task 19 checklist passes in a live Feishu environment.

- [ ] **Reference skill:** use superpowers:verification-before-completion before claiming the work is done.
