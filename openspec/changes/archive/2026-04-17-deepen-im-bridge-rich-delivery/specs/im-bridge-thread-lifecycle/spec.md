## ADDED Requirements

### Requirement: Bridge SHALL expose thread policy as a first-class reply-target field

IM Bridge SHALL extend `ReplyTarget` with `ThreadPolicy` (`reuse | open | isolate`) and `ThreadParentID`. The delivery router MUST honor policy at reply-plan resolution time: `reuse` keeps the current thread context, `open` requests a new thread, `isolate` produces a standalone message with a `[session: <short-id>]` prefix derived from the thread parent id.

#### Scenario: Open policy routes to the open-thread method
- **WHEN** a reply target carries `ThreadPolicy=open` and the active provider declares `ThreadPolicySupport` including `open`
- **THEN** `ResolveReplyPlan` selects `DeliveryMethodOpenThread`
- **AND** the bridge calls `ThreadOpener.OpenThread` before replying into the newly opened thread

#### Scenario: Reuse policy keeps existing thread context
- **WHEN** a reply target carries `ThreadPolicy=reuse` and `ThreadID` is set
- **THEN** the delivery uses `DeliveryMethodThreadReply` against the existing thread

#### Scenario: Isolate policy prefixes the message and uses standalone send
- **WHEN** a reply target carries `ThreadPolicy=isolate` and a `ThreadParentID`
- **THEN** the delivery uses `DeliveryMethodSend` with the content prefixed by `[session: <first-12-chars-of-parent-id>]`

### Requirement: Unsupported thread policies SHALL degrade with truthful fallback reasons

When a requested thread policy cannot be honored by the active provider, Bridge SHALL degrade to the closest supported method and report `fallback_reason=thread_<policy>_unsupported` in both the reply plan and the delivery receipt. Bridge MUST NOT pretend a provider supports threads when it does not.

#### Scenario: Provider without thread support degrades open-policy to reply
- **WHEN** a WeCom-targeted envelope requests `ThreadPolicy=open`
- **THEN** the reply plan falls back to a non-thread method
- **AND** `fallback_reason` is `thread_open_unsupported`

#### Scenario: Isolate prefix is applied idempotently
- **WHEN** Bridge repeats isolate delivery for the same parent id
- **THEN** the prefix is applied only once per message (no nested `[session: ...]` markers)
