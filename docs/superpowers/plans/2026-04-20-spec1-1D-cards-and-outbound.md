# Spec 1D — IM Bridge Card Schema + Outbound Dispatcher

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 spec1 §5 outbound_dispatcher + §8 provider-neutral card schema + §12 老 hardcode 一次性删除。让任意 Feishu 触发的 workflow 完成/失败时，自动有结构化卡片回到原线程。

**Architecture:** IM Bridge 引入 `core/card_schema` + 每平台 renderer + 文本 fallback；backend 新增 outbound_dispatcher 订阅 workflow 完成/失败事件，按 `system_metadata.im_dispatched / reply_target` 决定是否发送默认卡片，3 次指数退避后失败 emit `EventOutboundDeliveryFailed`；feishu/live.go 内 renderInteractiveCard/renderStructuredMessage 一次性删除，所有调用点切到 core dispatch。

**Tech Stack:** Go (IM Bridge core/platform renderers, eventbus Mod, HTTP client; the bridge is a Go service — JSON 字面量与 spec §8 ProviderNeutralCard schema 一致), Postgres jsonb (`system_metadata` 已由 1A 提供), Next.js (badge 微调).

**Depends on:** 1A (needs `workflow_executions.system_metadata` column + trigger_handler 把 `reply_target` 写入 system_metadata)

**Parallel with:** 1C (Trigger CRUD) — completely independent after 1A

**Unblocks:** 1E (im_send 节点写 system_metadata.im_dispatched 来抑制此 dispatcher)

---

## Coordination notes (read before starting)

- **Bridge runtime is Go, not TypeScript.** Spec §8 names files `card_schema.ts` / `card_renderer.ts` for illustration; the actual bridge under `src-im-bridge/` is a Go module (`core/`, `platform/{feishu,slack,dingtalk,...}/`). This plan uses `card_schema.go` / `card_renderer.go` etc. The wire shape (the `ProviderNeutralCard` JSON the backend POSTs) is unchanged from the spec.
- **`system_metadata` is owned by 1A.** This plan ASSUMES `WorkflowExecution.SystemMetadata json.RawMessage` exists and the trigger pipeline writes `reply_target` into it on IM-triggered runs. If 1A hasn't merged when this plan starts, stub the column with the same name/JSON keys but coordinate before T6.
- **No new "Failed" event type.** `EventWorkflowExecutionCompleted` already carries `status` ∈ {completed, failed, cancelled}; the dispatcher branches on the payload's `status`. Do NOT introduce `EventWorkflowExecutionFailed`.
- **Old code deletion (§12)**: `feishu/live.go::renderInteractiveCard` + `renderStructuredMessage` and `platform/feishu/renderer.go::renderStructuredMessage` are deleted in T9 in the SAME PR that re-routes every caller to the new dispatch entry. No feature flag, no parallel period.
- **`workflow-execution-view.tsx` was touched recently** (the linter / user landed local edits per `git status`). Only ADD the badge logic; do NOT revert their changes or refactor surrounding code.

---

## Task 1 — Provider-neutral card schema (`core/card_schema.go`)

- [x] Step 1.1 — write failing test: schema marshals to spec §8 wire shape
  - File: `src-im-bridge/core/card_schema_test.go` (new)
    ```go
    package core

    import (
        "encoding/json"
        "testing"
    )

    func TestProviderNeutralCard_JSONShape(t *testing.T) {
        c := ProviderNeutralCard{
            Title:   "Echo",
            Status:  CardStatusSuccess,
            Summary: "hello world",
            Fields:  []CardField{{Label: "Run", Value: "abc"}},
            Actions: []CardAction{
                {ID: "view", Label: "查看", Type: CardActionTypeURL, URL: "https://x/runs/abc"},
                {ID: "approve", Label: "Approve", Type: CardActionTypeCallback,
                    Style: CardStyleDanger, CorrelationToken: "tok-1",
                    Payload: map[string]any{"who": "qa"}},
            },
            Footer: "2026-04-20T10:00:00Z",
        }
        raw, err := json.Marshal(c)
        if err != nil { t.Fatal(err) }
        var back map[string]any
        if err := json.Unmarshal(raw, &back); err != nil { t.Fatal(err) }
        if back["title"] != "Echo" || back["status"] != "success" {
            t.Fatalf("title/status not preserved: %v", back)
        }
        actions := back["actions"].([]any)
        if actions[0].(map[string]any)["type"] != "url" { t.Fatal("url action") }
        if actions[1].(map[string]any)["correlation_token"] != "tok-1" { t.Fatal("token") }
    }

    func TestCardAction_ValidateExclusiveFields(t *testing.T) {
        bad := CardAction{ID: "x", Label: "x", Type: CardActionTypeURL, URL: "", CorrelationToken: "t"}
        if err := bad.Validate(); err == nil { t.Fatal("expected url action without URL to fail") }

        good := CardAction{ID: "x", Label: "x", Type: CardActionTypeCallback, CorrelationToken: "t"}
        if err := good.Validate(); err != nil { t.Fatalf("good callback failed: %v", err) }
    }
    ```

- [x] Step 1.2 — implement `card_schema.go`
  - File: `src-im-bridge/core/card_schema.go` (new)
    ```go
    package core

    import "errors"

    // CardStatus mirrors spec §8.
    type CardStatus string

    const (
        CardStatusSuccess CardStatus = "success"
        CardStatusFailed  CardStatus = "failed"
        CardStatusRunning CardStatus = "running"
        CardStatusPending CardStatus = "pending"
        CardStatusInfo    CardStatus = "info"
    )

    type CardStyle string

    const (
        CardStylePrimary CardStyle = "primary"
        CardStyleDanger  CardStyle = "danger"
        CardStyleDefault CardStyle = "default"
    )

    type CardActionType string

    const (
        CardActionTypeURL      CardActionType = "url"
        CardActionTypeCallback CardActionType = "callback"
    )

    type CardField struct {
        Label  string `json:"label"`
        Value  string `json:"value"`
        Inline bool   `json:"inline,omitempty"`
    }

    // CardAction is a flat-discriminated union: Type chooses URL vs Callback;
    // the unused fields are omitempty so the wire shape matches spec §8.
    type CardAction struct {
        ID               string                 `json:"id"`
        Label            string                 `json:"label"`
        Style            CardStyle              `json:"style,omitempty"`
        Type             CardActionType         `json:"type"`
        URL              string                 `json:"url,omitempty"`
        CorrelationToken string                 `json:"correlation_token,omitempty"`
        Payload          map[string]any         `json:"payload,omitempty"`
    }

    func (a CardAction) Validate() error {
        if a.ID == "" || a.Label == "" {
            return errors.New("card action: id and label are required")
        }
        switch a.Type {
        case CardActionTypeURL:
            if a.URL == "" {
                return errors.New("card action: url type requires url")
            }
        case CardActionTypeCallback:
            if a.CorrelationToken == "" {
                return errors.New("card action: callback type requires correlation_token")
            }
        default:
            return errors.New("card action: unknown type")
        }
        return nil
    }

    type ProviderNeutralCard struct {
        Title   string       `json:"title"`
        Status  CardStatus   `json:"status,omitempty"`
        Summary string       `json:"summary,omitempty"`
        Fields  []CardField  `json:"fields,omitempty"`
        Actions []CardAction `json:"actions,omitempty"`
        Footer  string       `json:"footer,omitempty"`
    }

    func (c ProviderNeutralCard) Validate() error {
        if c.Title == "" {
            return errors.New("card: title required")
        }
        for i, a := range c.Actions {
            if err := a.Validate(); err != nil {
                return errFmt("card.actions[%d]: %w", i, err)
            }
        }
        return nil
    }
    ```
  - Add tiny `errFmt` helper (or inline `fmt.Errorf`) — pick one and import accordingly; prefer `fmt.Errorf` to avoid a new helper.

- [x] Step 1.3 — verify
  - `rtk go test ./core/...` (within `src-im-bridge/`) — both new tests pass.

---

## Task 2 — Card renderer dispatch (`core/card_renderer.go`)

- [x] Step 2.1 — failing test: dispatch routes by ReplyTarget.Platform
  - File: `src-im-bridge/core/card_renderer_test.go` (new)
    ```go
    package core_test

    import (
        "testing"

        "github.com/agentforge/im-bridge/core"
    )

    func TestDispatch_RoutesByPlatform(t *testing.T) {
        registered := map[string]bool{}
        core.RegisterCardRenderer("feishu", func(c core.ProviderNeutralCard) (core.RenderedPayload, error) {
            registered["feishu"] = true
            return core.RenderedPayload{ContentType: "interactive", Body: `{"feishu":1}`}, nil
        })
        defer core.UnregisterCardRenderer("feishu")

        out, err := core.DispatchCard(core.ProviderNeutralCard{Title: "t"}, &core.ReplyTarget{Platform: "feishu"})
        if err != nil { t.Fatal(err) }
        if !registered["feishu"] || out.Body != `{"feishu":1}` { t.Fatalf("expected feishu route, got %+v", out) }
    }

    func TestDispatch_FallbackToText(t *testing.T) {
        out, err := core.DispatchCard(
            core.ProviderNeutralCard{Title: "T", Status: core.CardStatusSuccess, Summary: "ok"},
            &core.ReplyTarget{Platform: "qq"},
        )
        if err != nil { t.Fatal(err) }
        if out.ContentType != "text" { t.Fatalf("expected text fallback, got %s", out.ContentType) }
        if out.Body == "" { t.Fatal("text fallback empty") }
    }

    func TestDispatch_RejectsInvalidCard(t *testing.T) {
        if _, err := core.DispatchCard(core.ProviderNeutralCard{}, &core.ReplyTarget{Platform: "feishu"}); err == nil {
            t.Fatal("expected validation error for empty title")
        }
    }
    ```

- [x] Step 2.2 — implement `core/card_renderer.go`
  - File: `src-im-bridge/core/card_renderer.go` (new)
    ```go
    package core

    import (
        "errors"
        "fmt"
        "strings"
        "sync"
    )

    // RenderedPayload is the platform-specific JSON or text the bridge will
    // hand to its existing send path. ContentType is platform-meaningful
    // ("interactive" for feishu cards, "blocks" for slack, "actioncard" for
    // dingtalk, "text" for fallback).
    type RenderedPayload struct {
        ContentType string
        Body        string
    }

    // CardRenderer renders a ProviderNeutralCard into a platform payload.
    type CardRenderer func(ProviderNeutralCard) (RenderedPayload, error)

    var (
        rendererMu sync.RWMutex
        renderers  = map[string]CardRenderer{}
    )

    // RegisterCardRenderer is called from each platform package's init().
    func RegisterCardRenderer(platform string, fn CardRenderer) {
        rendererMu.Lock()
        defer rendererMu.Unlock()
        renderers[strings.ToLower(strings.TrimSpace(platform))] = fn
    }

    // UnregisterCardRenderer is test-only.
    func UnregisterCardRenderer(platform string) {
        rendererMu.Lock()
        defer rendererMu.Unlock()
        delete(renderers, strings.ToLower(strings.TrimSpace(platform)))
    }

    // DispatchCard validates the card, then renders it via the platform
    // renderer (when registered) or the text fallback.
    func DispatchCard(card ProviderNeutralCard, target *ReplyTarget) (RenderedPayload, error) {
        if err := card.Validate(); err != nil {
            return RenderedPayload{}, fmt.Errorf("dispatch card: %w", err)
        }
        platform := ""
        if target != nil {
            platform = strings.ToLower(strings.TrimSpace(target.Platform))
        }
        if platform == "" {
            return RenderedPayload{}, errors.New("dispatch card: reply target platform is required")
        }
        rendererMu.RLock()
        fn, ok := renderers[platform]
        rendererMu.RUnlock()
        if ok {
            return fn(card)
        }
        return RenderTextFallback(card), nil
    }
    ```

- [x] Step 2.3 — verify
  - `rtk go test ./core/...` — three new tests pass; existing tests still green.

---

## Task 3 — Text fallback renderer

- [x] Step 3.1 — failing snapshot test
  - File: `src-im-bridge/core/card_renderer_text_test.go` (new)
    ```go
    package core_test

    import (
        "testing"

        "github.com/agentforge/im-bridge/core"
    )

    func TestRenderTextFallback_SuccessCard(t *testing.T) {
        got := core.RenderTextFallback(core.ProviderNeutralCard{
            Title: "Echo workflow",
            Status: core.CardStatusSuccess,
            Summary: "hello",
            Fields: []core.CardField{{Label: "Run", Value: "abc-123"}},
            Footer: "2026-04-20T10:00:00Z",
            Actions: []core.CardAction{
                {ID: "view", Label: "查看详情", Type: core.CardActionTypeURL, URL: "https://x/runs/abc-123"},
            },
        }).Body
        want := "Echo workflow\n[SUCCESS] hello\n[Run] abc-123\n2026-04-20T10:00:00Z\n[查看详情] https://x/runs/abc-123\n"
        if got != want { t.Fatalf("text fallback mismatch:\nwant=%q\ngot =%q", want, got) }
    }

    func TestRenderTextFallback_CallbackButtonOmittedNoURL(t *testing.T) {
        got := core.RenderTextFallback(core.ProviderNeutralCard{
            Title: "Wait",
            Actions: []core.CardAction{
                {ID: "approve", Label: "Approve", Type: core.CardActionTypeCallback, CorrelationToken: "t"},
            },
        }).Body
        want := "Wait\n[Approve] (interactive button — open AgentForge to respond)\n"
        if got != want { t.Fatalf("got %q", got) }
    }
    ```

- [x] Step 3.2 — implement
  - In `src-im-bridge/core/card_renderer.go` append:
    ```go
    func RenderTextFallback(c ProviderNeutralCard) RenderedPayload {
        var b strings.Builder
        b.WriteString(c.Title); b.WriteByte('\n')
        if c.Status != "" || c.Summary != "" {
            if c.Status != "" {
                b.WriteString("["); b.WriteString(strings.ToUpper(string(c.Status))); b.WriteString("]")
                if c.Summary != "" { b.WriteByte(' ') }
            }
            if c.Summary != "" { b.WriteString(c.Summary) }
            b.WriteByte('\n')
        }
        for _, f := range c.Fields {
            b.WriteString("["); b.WriteString(f.Label); b.WriteString("] ")
            b.WriteString(f.Value); b.WriteByte('\n')
        }
        if c.Footer != "" { b.WriteString(c.Footer); b.WriteByte('\n') }
        for _, a := range c.Actions {
            switch a.Type {
            case CardActionTypeURL:
                b.WriteString("["); b.WriteString(a.Label); b.WriteString("] ")
                b.WriteString(a.URL); b.WriteByte('\n')
            case CardActionTypeCallback:
                b.WriteString("["); b.WriteString(a.Label); b.WriteString("] (interactive button — open AgentForge to respond)\n")
            }
        }
        return RenderedPayload{ContentType: "text", Body: b.String()}
    }
    ```

- [x] Step 3.3 — verify
  - `rtk go test ./core/...` — both snapshot tests pass.

---

## Task 4 — Per-platform renderers (feishu / slack / dingtalk)

- [x] Step 4.1 — failing snapshot tests
  - File: `src-im-bridge/platform/feishu/card_render_test.go` (new)
    ```go
    package feishu_test

    import (
        "encoding/json"
        "strings"
        "testing"

        "github.com/agentforge/im-bridge/core"
        _ "github.com/agentforge/im-bridge/platform/feishu" // init() registers
    )

    func TestFeishuRenderCard_Success(t *testing.T) {
        out, err := core.DispatchCard(core.ProviderNeutralCard{
            Title:   "Echo",
            Status:  core.CardStatusSuccess,
            Summary: "hello world",
            Fields:  []core.CardField{{Label: "Run", Value: "abc"}},
            Actions: []core.CardAction{
                {ID: "view", Label: "查看详情", Type: core.CardActionTypeURL, URL: "https://x/runs/abc"},
            },
            Footer: "2026-04-20",
        }, &core.ReplyTarget{Platform: "feishu"})
        if err != nil { t.Fatal(err) }
        if out.ContentType != "interactive" { t.Fatalf("content type %s", out.ContentType) }

        var payload map[string]any
        if err := json.Unmarshal([]byte(out.Body), &payload); err != nil { t.Fatal(err) }
        header := payload["header"].(map[string]any)
        if header["template"] != "green" {
            t.Fatalf("status=success should map to green header, got %v", header["template"])
        }
        // Title presence
        if !strings.Contains(out.Body, `"Echo"`) { t.Fatal("missing title") }
        // Footer + summary present as div elements
        if !strings.Contains(out.Body, `hello world`) { t.Fatal("missing summary") }
        if !strings.Contains(out.Body, `查看详情`) { t.Fatal("missing button") }
    }

    func TestFeishuRenderCard_FailedHeaderRed(t *testing.T) {
        out, _ := core.DispatchCard(core.ProviderNeutralCard{
            Title: "x", Status: core.CardStatusFailed,
        }, &core.ReplyTarget{Platform: "feishu"})
        var p map[string]any
        _ = json.Unmarshal([]byte(out.Body), &p)
        if p["header"].(map[string]any)["template"] != "red" {
            t.Fatalf("failed → red header")
        }
    }

    func TestFeishuRenderCard_CallbackButtonCarriesToken(t *testing.T) {
        out, _ := core.DispatchCard(core.ProviderNeutralCard{
            Title: "Approve?",
            Actions: []core.CardAction{{
                ID: "approve", Label: "Approve",
                Type: core.CardActionTypeCallback, CorrelationToken: "tok-xyz",
                Payload: map[string]any{"node_id": "wait-1"},
            }},
        }, &core.ReplyTarget{Platform: "feishu"})
        if !strings.Contains(out.Body, `tok-xyz`) {
            t.Fatalf("button value must carry correlation_token")
        }
        if !strings.Contains(out.Body, `wait-1`) {
            t.Fatalf("button value must carry payload")
        }
    }
    ```
  - Mirror analogous tests in `src-im-bridge/platform/slack/card_render_test.go` and `src-im-bridge/platform/dingtalk/card_render_test.go` (success + failed + callback). For slack, assert payload starts with `{"blocks":[`; for dingtalk, assert `card_type=ActionCard` and buttons array present.

- [x] Step 4.2 — implement Feishu renderer
  - File: `src-im-bridge/platform/feishu/card_render.go` (new)
    ```go
    package feishu

    import (
        "encoding/json"
        "fmt"
        "strings"

        "github.com/agentforge/im-bridge/core"
    )

    func init() {
        core.RegisterCardRenderer("feishu", renderProviderNeutralCard)
    }

    func renderProviderNeutralCard(card core.ProviderNeutralCard) (core.RenderedPayload, error) {
        elements := make([]map[string]any, 0, len(card.Fields)+len(card.Actions)+2)

        if s := strings.TrimSpace(card.Summary); s != "" {
            elements = append(elements, map[string]any{
                "tag":  "div",
                "text": map[string]any{"tag": "lark_md", "content": s},
            })
        }

        for _, f := range card.Fields {
            elements = append(elements, map[string]any{
                "tag": "div",
                "text": map[string]any{
                    "tag":     "lark_md",
                    "content": fmt.Sprintf("**%s**\n%s", f.Label, f.Value),
                },
            })
        }

        if footer := strings.TrimSpace(card.Footer); footer != "" {
            elements = append(elements, map[string]any{
                "tag":      "note",
                "elements": []map[string]any{{"tag": "plain_text", "content": footer}},
            })
        }

        if len(card.Actions) > 0 {
            actions := make([]map[string]any, 0, len(card.Actions))
            for _, a := range card.Actions {
                btn := map[string]any{
                    "tag":  "button",
                    "text": map[string]any{"tag": "plain_text", "content": a.Label},
                    "type": normalizeFeishuButtonStyle(string(a.Style)),
                }
                switch a.Type {
                case core.CardActionTypeURL:
                    btn["url"] = a.URL
                case core.CardActionTypeCallback:
                    value := map[string]any{
                        "action":            a.ID,
                        "correlation_token": a.CorrelationToken,
                    }
                    if len(a.Payload) > 0 {
                        value["payload"] = a.Payload
                    }
                    btn["value"] = value
                }
                actions = append(actions, btn)
            }
            elements = append(elements, map[string]any{"tag": "action", "actions": actions})
        }

        payload := map[string]any{
            "config":   map[string]any{"wide_screen_mode": true},
            "header":   feishuHeaderForStatus(card.Title, card.Status),
            "elements": elements,
        }
        body, err := json.Marshal(payload)
        if err != nil {
            return core.RenderedPayload{}, err
        }
        return core.RenderedPayload{ContentType: "interactive", Body: string(body)}, nil
    }

    func feishuHeaderForStatus(title string, status core.CardStatus) map[string]any {
        h := map[string]any{
            "title": map[string]any{"tag": "plain_text", "content": title},
        }
        switch status {
        case core.CardStatusSuccess:
            h["template"] = "green"
        case core.CardStatusFailed:
            h["template"] = "red"
        case core.CardStatusRunning:
            h["template"] = "blue"
        case core.CardStatusPending:
            h["template"] = "grey"
        case core.CardStatusInfo:
            h["template"] = "blue"
        }
        return h
    }

    func normalizeFeishuButtonStyle(style string) string {
        switch strings.ToLower(strings.TrimSpace(style)) {
        case "primary", "danger", "default":
            return strings.ToLower(strings.TrimSpace(style))
        default:
            return "default"
        }
    }
    ```

- [x] Step 4.3 — implement Slack renderer
  - File: `src-im-bridge/platform/slack/card_render.go` (new) — uses `slack-go/slack` blocks. Header → `header` block; Summary → markdown section; Fields → 2-col section with `*label*\nvalue`; Footer → context block; Actions → action block with button elements (URL action sets `URL`, callback action sets `Value` to `correlation_token` JSON-encoded with `payload` and `action_id`).
    ```go
    package slack

    import (
        "encoding/json"

        "github.com/agentforge/im-bridge/core"
        gs "github.com/slack-go/slack"
    )

    func init() {
        core.RegisterCardRenderer("slack", renderProviderNeutralCard)
    }

    func renderProviderNeutralCard(card core.ProviderNeutralCard) (core.RenderedPayload, error) {
        blocks := []gs.Block{
            gs.NewHeaderBlock(gs.NewTextBlockObject(gs.PlainTextType, card.Title, false, false)),
        }
        if card.Summary != "" {
            blocks = append(blocks, gs.NewSectionBlock(
                gs.NewTextBlockObject(gs.MarkdownType, card.Summary, false, false), nil, nil))
        }
        if len(card.Fields) > 0 {
            fields := make([]*gs.TextBlockObject, 0, len(card.Fields))
            for _, f := range card.Fields {
                fields = append(fields, gs.NewTextBlockObject(gs.MarkdownType,
                    "*"+f.Label+"*\n"+f.Value, false, false))
            }
            blocks = append(blocks, gs.NewSectionBlock(nil, fields, nil))
        }
        if len(card.Actions) > 0 {
            elems := make([]gs.BlockElement, 0, len(card.Actions))
            for _, a := range card.Actions {
                btn := gs.NewButtonBlockElement("act-"+a.ID, a.ID,
                    gs.NewTextBlockObject(gs.PlainTextType, a.Label, false, false))
                switch a.Type {
                case core.CardActionTypeURL:
                    btn.URL = a.URL
                case core.CardActionTypeCallback:
                    valueJSON, _ := json.Marshal(map[string]any{
                        "action_id":         a.ID,
                        "correlation_token": a.CorrelationToken,
                        "payload":           a.Payload,
                    })
                    btn.Value = string(valueJSON)
                }
                btn.Style = gs.Style(slackButtonStyle(string(a.Style)))
                elems = append(elems, btn)
            }
            blocks = append(blocks, gs.NewActionBlock("af-actions", elems...))
        }
        if card.Footer != "" {
            blocks = append(blocks, gs.NewContextBlock("af-footer",
                gs.NewTextBlockObject(gs.MarkdownType, card.Footer, false, false)))
        }
        body, err := json.Marshal(map[string]any{"blocks": blocks})
        if err != nil { return core.RenderedPayload{}, err }
        return core.RenderedPayload{ContentType: "blocks", Body: string(body)}, nil
    }

    func slackButtonStyle(s string) string {
        switch s { case "primary", "danger": return s }
        return ""
    }
    ```

- [x] Step 4.4 — implement DingTalk renderer
  - File: `src-im-bridge/platform/dingtalk/card_render.go` (new)
    ```go
    package dingtalk

    import (
        "encoding/json"
        "strings"

        "github.com/agentforge/im-bridge/core"
    )

    func init() {
        core.RegisterCardRenderer("dingtalk", renderProviderNeutralCard)
    }

    func renderProviderNeutralCard(card core.ProviderNeutralCard) (core.RenderedPayload, error) {
        var md strings.Builder
        if card.Status != "" {
            md.WriteString("**[")
            md.WriteString(strings.ToUpper(string(card.Status)))
            md.WriteString("]** ")
        }
        if card.Summary != "" { md.WriteString(card.Summary); md.WriteString("\n\n") }
        for _, f := range card.Fields {
            md.WriteString("**" + f.Label + "**: " + f.Value + "  \n")
        }
        if card.Footer != "" { md.WriteString("\n_" + card.Footer + "_") }

        // DingTalk ActionCard supports URL buttons only — callback actions
        // degrade to a markdown line so the user sees the labels even if
        // they can't interact directly.
        buttons := make([]map[string]string, 0, len(card.Actions))
        for _, a := range card.Actions {
            if a.Type == core.CardActionTypeURL && a.URL != "" {
                buttons = append(buttons, map[string]string{"title": a.Label, "actionURL": a.URL})
            } else if a.Type == core.CardActionTypeCallback {
                md.WriteString("\n• [" + a.Label + "] (使用 AgentForge 客户端响应)")
            }
        }
        payload := map[string]any{
            "card_type": "ActionCard",
            "title":     card.Title,
            "markdown":  md.String(),
            "buttons":   buttons,
        }
        body, err := json.Marshal(payload)
        if err != nil { return core.RenderedPayload{}, err }
        return core.RenderedPayload{ContentType: "actioncard", Body: string(body)}, nil
    }
    ```

- [x] Step 4.5 — verify
  - `rtk go test ./core/... ./platform/feishu/... ./platform/slack/... ./platform/dingtalk/...` — all snapshot tests pass.

---

## Task 5 — Extend `/im/send` to accept `ProviderNeutralCard`

- [ ] Step 5.1 — failing test for new request shape
  - File: `src-im-bridge/notify/receiver_card_test.go` (new) — boots `Receiver` with a stub platform that records the last `Send` payload. Posts:
    ```json
    {"platform":"feishu","chat_id":"c1",
     "card":{"title":"T","status":"success","summary":"ok"}}
    ```
    Assert: stub got `MsgTypeInteractive` with body containing `"green"`. Then post the same body but with both `text` and `card` set — expect HTTP 400 ("send requires exactly one of text or card").

- [ ] Step 5.2 — extend `SendRequest`
  - File: `src-im-bridge/notify/receiver.go`, in the `SendRequest` struct (around line 387) add:
    ```go
    Card *core.ProviderNeutralCard `json:"card,omitempty"`
    ```
    Update the `// SendRequest is the payload for the /im/send endpoint.` doc comment to note: "Exactly one of `content`, `structured`, `native`, or `card` may be set; mixing returns 400."

- [ ] Step 5.3 — wire `card` through dispatch in `handleSend`
  - In `handleSend` (line 416), after `s.ChatID` validation and before the existing `core.DeliverEnvelope`:
    ```go
    if s.Card != nil {
        if s.Content != "" || s.Structured != nil || s.Native != nil || len(s.Attachments) > 0 {
            http.Error(w, "card cannot be combined with content/structured/native/attachments", http.StatusBadRequest)
            r.emitFailed("/im/send", deliveryID, "send", chatID, "", "exclusive_card", start)
            return
        }
        rendered, err := core.DispatchCard(*s.Card, s.ReplyTarget)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            r.emitFailed("/im/send", deliveryID, "send", chatID, "", err.Error(), start)
            return
        }
        // Bridge to existing path: convert RenderedPayload → core.DeliveryEnvelope.Content.
        // Platform-specific bodies (feishu interactive JSON, slack blocks, etc.) are
        // already in the form the platform Send/Reply path expects; the platform
        // adapter recognises the content-type via metadata.
        envMeta := cloneMetadata(s.Metadata)
        envMeta["card_content_type"] = rendered.ContentType
        receipt, err := core.DeliverEnvelope(ctx, r.platform, r.metadata, chatID, &core.DeliveryEnvelope{
            Content:     rendered.Body,
            ReplyTarget: s.ReplyTarget,
            Metadata:    envMeta,
        })
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            r.emitFailed("/im/send", deliveryID, "send", chatID, "", err.Error(), start)
            return
        }
        writeDeliveryReceipt(w, receipt)
        r.emitDelivered("/im/send", deliveryID, "send", chatID, "", receipt, start)
        return
    }
    ```
  - Move the existing `attachments`/`DeliverEnvelope` block into an `else` branch so the two paths are mutually exclusive.

- [ ] Step 5.4 — make platform adapters honour `card_content_type` metadata
  - File: `src-im-bridge/core/delivery.go` (or `delivery_attachments_test.go` neighbouring file — find the existing dispatch). Add a routing branch: when `Metadata["card_content_type"]` is `"interactive"`, send via the platform's `CardSender` raw-payload entrypoint; for `"blocks"` route to slack's blocks send; for `"actioncard"` route to dingtalk's ActionCard send. Provide a new minimal interface `core.RawCardSender { SendRawCard(ctx, chatID, contentType, body string, target *ReplyTarget) error }` that each platform's `live.go` implements (3-line wrapper around the existing `messages.Send` / `webhook.Post` calls).

- [ ] Step 5.5 — verify
  - `rtk go test ./notify/... ./core/... ./platform/feishu/...` — new test passes; existing pass.

---

## Task 6 — Backend: register `EventOutboundDeliveryFailed` event type

- [ ] Step 6.1 — failing assertion test
  - File: `src-go/internal/eventbus/types_test.go` (extend or add)
    ```go
    func TestEventOutboundDeliveryFailedConstantStable(t *testing.T) {
        if EventOutboundDeliveryFailed != "workflow.outbound_delivery.failed" {
            t.Fatal("name must be stable for FE WS subscribers")
        }
    }
    ```
  - Mirror in `src-go/internal/ws/events.go` if a similar test file exists.

- [ ] Step 6.2 — add the constant
  - File: `src-go/internal/eventbus/types.go`, in the `// Workflow DAG execution events` block (line 74-89), add:
    ```go
    // Emitted by outbound_dispatcher after retry exhaustion when it could
    // not deliver the default reply card to the IM Bridge. Payload shape:
    //   {executionId: string, lastError: string, attempts: int}
    EventOutboundDeliveryFailed = "workflow.outbound_delivery.failed"
    ```
  - File: `src-go/internal/ws/events.go`, mirror the same constant in the workflow block (line 92-100). Use the same string value so the FE only subscribes to one type.

- [ ] Step 6.3 — verify
  - `rtk go test ./internal/eventbus/... ./internal/ws/...` — passes.

---

## Task 7 — Outbound dispatcher core (eventbus Mod)

- [ ] Step 7.1 — failing 4-cell matrix test
  - File: `src-go/internal/service/outbound_dispatcher_test.go` (new)
    ```go
    package service_test

    import (
        "context"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "sync/atomic"
        "testing"
        "time"

        "github.com/google/uuid"
        eb "github.com/react-go-quick-starter/server/internal/eventbus"
        "github.com/react-go-quick-starter/server/internal/model"
        "github.com/react-go-quick-starter/server/internal/service"
        "github.com/react-go-quick-starter/server/internal/ws"
    )

    type fakeExecRepo struct {
        exec *model.WorkflowExecution
    }
    func (f *fakeExecRepo) GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
        return f.exec, nil
    }

    func mkExec(t *testing.T, status string, sysMeta map[string]any) *model.WorkflowExecution {
        raw, _ := json.Marshal(sysMeta)
        return &model.WorkflowExecution{
            ID:             uuid.New(),
            WorkflowID:     uuid.New(),
            ProjectID:      uuid.New(),
            Status:         status,
            SystemMetadata: raw,
        }
    }

    func TestDispatcher_Matrix(t *testing.T) {
        cases := []struct{
            name        string
            replyTarget map[string]any
            imDispatched bool
            wantPosts   int32
        }{
            {"no reply target", nil, false, 0},
            {"reply target + im_dispatched", map[string]any{"platform":"feishu","chat_id":"c"}, true, 0},
            {"reply target + not dispatched (success)", map[string]any{"platform":"feishu","chat_id":"c"}, false, 1},
            // failed branch tested by status payload — see Step 7.4 separate test
        }
        for _, c := range cases {
            t.Run(c.name, func(t *testing.T) {
                var posts int32
                srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                    atomic.AddInt32(&posts, 1)
                    w.WriteHeader(200); w.Write([]byte(`{"status":"sent"}`))
                }))
                defer srv.Close()
                meta := map[string]any{}
                if c.replyTarget != nil { meta["reply_target"] = c.replyTarget }
                if c.imDispatched { meta["im_dispatched"] = true }
                exec := mkExec(t, model.WorkflowExecStatusCompleted, meta)
                d := service.NewOutboundDispatcher(&fakeExecRepo{exec: exec}, srv.URL, "https://fe.example", nil)
                d.SetRetryDelays(0, 0, 0) // fast tests
                payload, _ := json.Marshal(map[string]any{
                    "executionId": exec.ID.String(),
                    "workflowId":  exec.WorkflowID.String(),
                    "status":      model.WorkflowExecStatusCompleted,
                })
                ev := eb.NewEvent(ws.EventWorkflowExecutionCompleted, "core", "project:"+exec.ProjectID.String())
                ev.Payload = payload
                d.Observe(context.Background(), ev, &eb.PipelineCtx{})
                time.Sleep(50 * time.Millisecond)
                if got := atomic.LoadInt32(&posts); got != c.wantPosts {
                    t.Fatalf("posts: want %d got %d", c.wantPosts, got)
                }
            })
        }
    }
    ```

- [ ] Step 7.2 — failing retry+failure test
  - Append to the file:
    ```go
    func TestDispatcher_RetriesThenEmitsFailureEvent(t *testing.T) {
        var posts int32
        srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            atomic.AddInt32(&posts, 1)
            http.Error(w, "boom", 500)
        }))
        defer srv.Close()

        exec := mkExec(t, model.WorkflowExecStatusFailed, map[string]any{
            "reply_target": map[string]any{"platform":"feishu","chat_id":"c"},
        })
        var emitted []eb.Event
        bus := &captureBus{onPublish: func(e *eb.Event) { emitted = append(emitted, *e) }}
        d := service.NewOutboundDispatcher(&fakeExecRepo{exec: exec}, srv.URL, "https://fe.example", bus)
        d.SetRetryDelays(0, 0, 0)

        payload, _ := json.Marshal(map[string]any{
            "executionId": exec.ID.String(),
            "status":      model.WorkflowExecStatusFailed,
        })
        ev := eb.NewEvent(eb.EventWorkflowExecutionCompleted, "core", "project:"+exec.ProjectID.String())
        ev.Payload = payload
        d.Observe(context.Background(), ev, &eb.PipelineCtx{})
        time.Sleep(100 * time.Millisecond)

        if posts != 3 { t.Fatalf("expected 3 attempts, got %d", posts) }
        if len(emitted) != 1 || emitted[0].Type != eb.EventOutboundDeliveryFailed {
            t.Fatalf("expected one EventOutboundDeliveryFailed, got %+v", emitted)
        }
    }
    ```

- [ ] Step 7.3 — implement the dispatcher
  - File: `src-go/internal/service/outbound_dispatcher.go` (new)
    ```go
    package service

    import (
        "bytes"
        "context"
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "time"

        "github.com/google/uuid"
        log "github.com/sirupsen/logrus"

        eb "github.com/react-go-quick-starter/server/internal/eventbus"
        "github.com/react-go-quick-starter/server/internal/model"
    )

    // ExecutionLoader is the repository surface the dispatcher needs.
    type ExecutionLoader interface {
        GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
    }

    // OutboundDispatcher subscribes to terminal workflow execution events and
    // delivers the default reply card to the IM Bridge unless the workflow
    // explicitly took over delivery via system_metadata.im_dispatched.
    type OutboundDispatcher struct {
        execRepo  ExecutionLoader
        bridgeURL string
        feBaseURL string
        bus       eb.Publisher
        client    *http.Client
        delays    [3]time.Duration
    }

    func NewOutboundDispatcher(repo ExecutionLoader, bridgeURL, feBaseURL string, bus eb.Publisher) *OutboundDispatcher {
        return &OutboundDispatcher{
            execRepo:  repo,
            bridgeURL: bridgeURL,
            feBaseURL: feBaseURL,
            bus:       bus,
            client:    &http.Client{Timeout: 10 * time.Second},
            delays:    [3]time.Duration{1 * time.Second, 4 * time.Second, 16 * time.Second},
        }
    }

    // SetRetryDelays is for tests; production keeps 1s/4s/16s.
    func (d *OutboundDispatcher) SetRetryDelays(d1, d2, d3 time.Duration) {
        d.delays = [3]time.Duration{d1, d2, d3}
    }

    // --- eventbus.Mod interface ---

    func (d *OutboundDispatcher) Name() string         { return "service.outbound-dispatcher" }
    func (d *OutboundDispatcher) Intercepts() []string { return []string{eb.EventWorkflowExecutionCompleted} }
    func (d *OutboundDispatcher) Priority() int        { return 80 }
    func (d *OutboundDispatcher) Mode() eb.Mode        { return eb.ModeObserve }

    type completedPayload struct {
        ExecutionID string `json:"executionId"`
        WorkflowID  string `json:"workflowId"`
        Status      string `json:"status"`
    }

    func (d *OutboundDispatcher) Observe(ctx context.Context, e *eb.Event, _ *eb.PipelineCtx) {
        if e == nil { return }
        var p completedPayload
        if err := json.Unmarshal(e.Payload, &p); err != nil {
            log.WithError(err).Warn("outbound_dispatcher: payload decode")
            return
        }
        // Only dispatch on completed/failed; cancelled is silent.
        if p.Status != model.WorkflowExecStatusCompleted && p.Status != model.WorkflowExecStatusFailed {
            return
        }
        execID, err := uuid.Parse(p.ExecutionID)
        if err != nil { return }

        // Spawn so we don't block the event pipeline; retries live here.
        go d.dispatch(context.Background(), execID, p.Status)
    }

    func (d *OutboundDispatcher) dispatch(ctx context.Context, execID uuid.UUID, status string) {
        exec, err := d.execRepo.GetExecution(ctx, execID)
        if err != nil || exec == nil {
            log.WithError(err).WithField("executionId", execID).Warn("outbound_dispatcher: load exec")
            return
        }
        sm := decodeSystemMetadata(exec.SystemMetadata)
        if v, _ := sm["im_dispatched"].(bool); v {
            log.WithField("executionId", execID).Info("outbound_dispatcher: skipped (explicit im_send)")
            return
        }
        target := decodeReplyTarget(sm["reply_target"])
        if target == nil {
            log.WithField("executionId", execID).Info("outbound_dispatcher: skipped (no reply target)")
            return
        }

        card := d.buildDefaultCard(exec, status, sm)
        body, _ := json.Marshal(map[string]any{
            "platform":    target.Platform,
            "chat_id":     target.ChatID,
            "replyTarget": target,
            "card":        card,
        })

        var lastErr error
        for attempt := 0; attempt < 3; attempt++ {
            if attempt > 0 {
                time.Sleep(d.delays[attempt-1])
            }
            req, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.bridgeURL+"/im/send", bytes.NewReader(body))
            req.Header.Set("Content-Type", "application/json")
            resp, err := d.client.Do(req)
            if err == nil && resp.StatusCode < 400 {
                resp.Body.Close()
                return
            }
            if resp != nil {
                b, _ := io.ReadAll(resp.Body); resp.Body.Close()
                lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
            } else {
                lastErr = err
            }
            log.WithError(lastErr).WithField("attempt", attempt+1).Warn("outbound_dispatcher: send failed")
        }
        d.emitFailure(ctx, exec, lastErr)
    }

    func (d *OutboundDispatcher) emitFailure(ctx context.Context, exec *model.WorkflowExecution, lastErr error) {
        if d.bus == nil { return }
        msg := ""
        if lastErr != nil { msg = lastErr.Error() }
        _ = eb.PublishLegacy(ctx, d.bus, eb.EventOutboundDeliveryFailed, exec.ProjectID.String(), map[string]any{
            "executionId": exec.ID.String(),
            "lastError":   msg,
            "attempts":    3,
        })
    }
    ```
  - The helpers `decodeSystemMetadata`, `decodeReplyTarget`, `buildDefaultCard` go in the same file or `outbound_dispatcher_helpers.go`. `buildDefaultCard` reads `final_output` (preferred) or last node output (`exec.DataStore` last key) for `summary`.

- [ ] Step 7.4 — failing test for default card content (success + failed)
  - File: `src-go/internal/service/outbound_dispatcher_card_test.go` (new) — table-test that asserts card title, status, fields[`Run`], and `view` action URL = `<feBaseURL>/runs/<exec_id>` for completed; for failed asserts title `+ ' 执行失败'`, fields contain `失败节点` + `Run`, summary contains `error_message`. Use a fake `WorkflowDefinitionLoader` to provide the workflow name (or pass workflow name through completion event payload — extend the payload shape in step 7.3).

- [ ] Step 7.5 — verify
  - `rtk go test ./internal/service/...` — all dispatcher tests pass.

---

## Task 8 — Wire dispatcher into server startup

- [ ] Step 8.1 — failing wiring test
  - File: `src-go/cmd/server/main_test.go` (extend) — assert `bus.Mods` after wiring contains the dispatcher (use a small `Mods()` accessor on `*Bus` if not present; add it as a debug helper used only in tests).

- [ ] Step 8.2 — register the Mod in `main.go`
  - File: `src-go/cmd/server/main.go`, after `bus.Register(ebmods.NewMetrics())` (line 153), add:
    ```go
    outboundDispatcher := service.NewOutboundDispatcher(
        repository.NewWorkflowExecutionRepositoryAdapter(workflowExecRepo), // thin adapter exposing GetExecution
        cfg.IMBridgeURL,        // e.g. http://localhost:7779 — reuse cfg.NotifyURL if that's the canonical name
        cfg.FrontendBaseURL,    // e.g. https://app.example.com or http://localhost:3000
        bus,
    )
    bus.Register(outboundDispatcher)
    ```
  - Confirm the canonical config keys exist; if `cfg.IMBridgeURL` doesn't exist, reuse the same field `im_service.go` already uses (`s.notifyURL`) — search the config struct and pick the existing one. `cfg.FrontendBaseURL` must exist or be added (one-line addition to `internal/config/config.go` with env `FRONTEND_BASE_URL`, default `http://localhost:3000`).

- [ ] Step 8.3 — verify
  - `rtk go build ./...` — builds clean.
  - `rtk go test ./cmd/server/...` — wiring assertion passes.

---

## Task 9 — Delete old Feishu card builders + re-route every caller (§12)

- [ ] Step 9.1 — enumerate every caller
  - Run: `rtk grep "renderInteractiveCard\|renderStructuredMessage" src-im-bridge/`. Confirmed callers (from initial survey):
    - `src-im-bridge/platform/feishu/live.go:473` — `SendCard(ctx, chatID, *core.Card)`
    - `src-im-bridge/platform/feishu/live.go:481` — `ReplyCard(ctx, replyCtx, *core.Card)`
    - `src-im-bridge/platform/feishu/live.go:501` — `SendStructured`
    - `src-im-bridge/platform/feishu/live.go:510` — `ReplyStructured`
    - `src-im-bridge/platform/feishu/live.go:1094` — payload encode for native fallback path
    - `src-im-bridge/platform/feishu/renderer.go` — entire file (renderStructuredMessage + helpers)
    - `src-im-bridge/platform/feishu/renderer_test.go` — the snapshot tests
  - Re-grep at the start of this task to catch any new callers added in parallel branches; if found, list them as "additional re-route in this PR".

- [ ] Step 9.2 — replace `SendCard` / `ReplyCard` / `SendStructured` / `ReplyStructured` to go through `core.DispatchCard`
  - File: `src-im-bridge/platform/feishu/live.go`
  - The legacy `*core.Card` is a smaller subset (Title + Fields + Buttons) than `ProviderNeutralCard`. Add a private adapter `legacyCardToNeutral(*core.Card) core.ProviderNeutralCard` near the top of `live.go`:
    ```go
    func legacyCardToNeutral(c *core.Card) core.ProviderNeutralCard {
        out := core.ProviderNeutralCard{Title: c.Title}
        for _, f := range c.Fields {
            out.Fields = append(out.Fields, core.CardField{Label: f.Label, Value: f.Value})
        }
        for _, b := range c.Buttons {
            a := core.CardAction{ID: b.Action, Label: b.Text, Style: core.CardStyle(b.Style)}
            if strings.HasPrefix(b.Action, "link:") {
                a.Type = core.CardActionTypeURL
                a.URL = strings.TrimPrefix(b.Action, "link:")
            } else {
                a.Type = core.CardActionTypeCallback
                a.CorrelationToken = b.Action  // legacy: the action string IS the correlation key
            }
            out.Actions = append(out.Actions, a)
        }
        return out
    }
    ```
  - Rewrite `SendCard`:
    ```go
    func (l *Live) SendCard(ctx context.Context, chatID string, card *core.Card) error {
        if strings.TrimSpace(chatID) == "" { return errors.New("feishu card send requires chat id") }
        rendered, err := core.DispatchCard(legacyCardToNeutral(card), &core.ReplyTarget{Platform: "feishu", ChatID: chatID})
        if err != nil { return err }
        return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, chatID, larkim.MsgTypeInteractive, rendered.Body)
    }
    ```
  - Same shape for `ReplyCard`. For `SendStructured` / `ReplyStructured` — the `StructuredMessage` is richer than `ProviderNeutralCard`. Two options:
    1. Convert StructuredMessage → ProviderNeutralCard then dispatch (lossy for image/divider/context sections).
    2. Keep StructuredMessage rendering but move the renderer body OUT of the deleted helpers into a NEW file `card_render_structured.go` exposing `renderStructured(message) (string, error)`.
  - Take option 2 — preserve existing functionality. Create `src-im-bridge/platform/feishu/card_render_structured.go` containing the existing `renderStructuredMessage`, `renderStructuredSections`, `renderLegacySections`, `renderFieldsAsColumns`, `fieldToColumn`, `renderButtons`, `feishuCardHeader` bodies (verbatim copy from the existing `renderer.go`), and rename them to `renderStructured*` to avoid collision. Update the four `SendStructured`/`ReplyStructured` call-sites to call `renderStructured(message)`.

- [ ] Step 9.3 — delete the old code
  - File: `src-im-bridge/platform/feishu/live.go`, delete the function `renderInteractiveCard` (lines 1281-1335).
  - File: `src-im-bridge/platform/feishu/renderer.go` — DELETE the file entirely (its content has been split between `card_render_structured.go` for structured messages and `card_render.go` for the new neutral path).
  - File: `src-im-bridge/platform/feishu/renderer_test.go` — DELETE (replaced by `card_render_test.go` from T4 + a new `card_render_structured_test.go` mirroring the old assertions but pointing at the renamed functions).

- [ ] Step 9.4 — port the structured snapshot tests
  - File: `src-im-bridge/platform/feishu/card_render_structured_test.go` (new) — copy each test from the deleted `renderer_test.go`, change `renderStructuredMessage(...)` → `renderStructured(...)` (or whatever name was chosen in 9.2), keep all fixtures.

- [ ] Step 9.5 — verify cold compile + all tests
  - `rtk go build ./...` from `src-im-bridge/` — must compile with zero unresolved references to `renderInteractiveCard` / `renderStructuredMessage`. If grep still finds either name, fix before proceeding.
  - `rtk go test ./...` from `src-im-bridge/` — every existing test must still pass; the structured-message snapshots assert byte-for-byte the same JSON they did before, since we only renamed the function and split files.

---

## Task 10 — FE: "回帖失败" badge in execution detail

- [ ] Step 10.1 — failing test for badge rendering
  - File: `components/workflow/workflow-execution-view.test.tsx` (new — there isn't one for this component yet)
    ```tsx
    import { render, screen } from "@testing-library/react";
    import { WorkflowExecutionView } from "./workflow-execution-view";
    // Mock useAuthStore + fetch + ws-client to seed a completed exec.
    // Then dispatch a synthetic "workflow.outbound_delivery.failed" frame on
    // the mock WSClient and assert a badge with text "回帖失败" appears.
    ```
    Use existing test fixtures in adjacent component tests (e.g. `workflow-runs-tab.test.tsx`) for the boilerplate WS mock.

- [ ] Step 10.2 — extend the component (ADD ONLY)
  - File: `components/workflow/workflow-execution-view.tsx`
  - Inside the `ExecutionView` component (the parent that owns `useEffect` at line 387), add:
    ```tsx
    const [outboundDeliveryFailed, setOutboundDeliveryFailed] = useState(false);
    useEffect(() => {
      const token = useAuthStore.getState().accessToken;
      if (!token || !executionId) return;
      const ws = new WSClient(createApiClient(API_URL).wsUrl("/ws"), token);
      ws.connect();
      ws.subscribe(`project:${execution?.projectId ?? ""}`);
      const handler = (msg: unknown) => {
        const m = msg as { event?: { payload?: { executionId?: string } } };
        if (m.event?.payload?.executionId === executionId) {
          setOutboundDeliveryFailed(true);
        }
      };
      ws.on("workflow.outbound_delivery.failed", handler);
      return () => {
        ws.off("workflow.outbound_delivery.failed", handler);
        ws.close();
      };
    }, [executionId, execution?.projectId]);
    ```
    (Use the existing `WSClient` import from `@/lib/ws-client`; the WS URL builder may live elsewhere — match the style used by `workflow-runs-tab.tsx`.)
  - In the JSX header block where `<ExecutionStatusBadge status={execution.status} />` renders (around line 453), append:
    ```tsx
    {outboundDeliveryFailed && (
      <Badge variant="destructive" className="text-[10px]" title="后端尝试 3 次仍未把结果回帖到 IM 线程">
        回帖失败
      </Badge>
    )}
    ```
  - Do NOT remove or refactor any other code in this file (the user's recent edits stay intact).

- [ ] Step 10.3 — verify
  - `rtk pnpm test components/workflow/workflow-execution-view.test.tsx` — passes.
  - `rtk lint` — clean (no new warnings in the touched file).

---

## Task 11 — Integration test: dispatcher → mock IM Bridge

- [ ] Step 11.1 — write the integration test
  - File: `src-go/internal/service/outbound_dispatcher_integration_test.go` (new, build tag `//go:build integration`)
    ```go
    //go:build integration
    package service_test
    // Boots a real test PG via the existing integration harness (see
    // dag_workflow_service_integration_test.go for the pattern); inserts
    // a workflow_executions row with system_metadata.reply_target and status=completed,
    // boots a mock IM Bridge HTTP server, registers the dispatcher with a real
    // bus, publishes EventWorkflowExecutionCompleted, asserts:
    //   - mock got POST /im/send within 200ms
    //   - posted body has card.title == workflow.name
    //   - posted body has card.actions[0].url contains exec id
    //   - dispatcher does NOT post when system_metadata.im_dispatched=true
    ```

- [ ] Step 11.2 — verify
  - `rtk go test -tags=integration ./internal/service/...` — passes against local PG.

---

## Task 12 — E2E smoke fixtures (Trace A + Trace C)

- [ ] Step 12.1 — Trace A fixture
  - File: `src-im-bridge/scripts/smoke/fixtures/feishu-workflow-with-card.json` (new)
    ```json
    {
      "content": "/workflow echo-with-card",
      "user_id": "feishu-user-1",
      "user_name": "Feishu Smoke",
      "chat_id": "feishu-chat-1",
      "is_group": true,
      "expect_card": {
        "platform": "feishu",
        "title_contains": "echo",
        "status": "success",
        "actions_min": 1
      }
    }
    ```
    The smoke harness should poll the mock IM Bridge for an inbound `/im/send` whose `card` matches `expect_card` within 10s.

- [ ] Step 12.2 — Trace C fixture
  - File: `src-im-bridge/scripts/smoke/fixtures/feishu-workflow-http-fail.json` (new)
    ```json
    {
      "content": "/workflow http-fail-demo",
      "user_id": "feishu-user-1",
      "chat_id": "feishu-chat-1",
      "is_group": true,
      "stub_http_node_response": { "status": 401, "body": "unauthorized" },
      "expect_card": {
        "platform": "feishu",
        "title_contains": "执行失败",
        "status": "failed",
        "fields_contains": ["失败节点", "Run"]
      }
    }
    ```
  - For now this fixture is consumed by a future smoke runner extension; document the wire shape so the runner work in 1B (HTTP node) can pick it up. A TODO comment in the fixture's sibling README is fine.

- [ ] Step 12.3 — extend Invoke-StubSmoke.ps1 minimally
  - Add a comment block at the top of `src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1` listing the new fixtures and their `expect_card` field. Implementation of the assertion runner can be a follow-up; the fixture files alone unblock manual smoke until then.

- [ ] Step 12.4 — verify
  - `rtk pnpm exec tsc --noEmit` — fixtures are pure JSON; no TS impact.
  - Manual: `rtk pwsh src-im-bridge/scripts/smoke/Invoke-StubSmoke.ps1 -Fixture feishu-workflow-with-card` — manual eyeball that the request reaches the bridge.

---

## Task 13 — Self-review pass

- [ ] Step 13.1 — spec coverage check
  - Walk spec §5 (architecture box: outbound_dispatcher), §8 (card schema), §9 Trace A + Trace C, §10 row "outbound_dispatcher 发送失败", §12 (deletion checklist) and tick each off against this plan's tasks. If any row has no task line, add one.

- [ ] Step 13.2 — discriminated-union consistency
  - Open `src-im-bridge/core/card_schema.go` and EVERY platform `card_render.go`. Confirm each renderer switches on `a.Type` with both `CardActionTypeURL` and `CardActionTypeCallback` cases. The text fallback's `card_renderer.go` must do the same. `RenderTextFallback` and the four renderers must agree on field names: `correlation_token`, `payload`, `url`, `style`, `id`, `label`. Grep `correlation_token` across `core/` + `platform/` and confirm 1 producer + 4 consumers.

- [ ] Step 13.3 — placeholder sweep
  - `rtk grep -nE "TODO|FIXME|XXX" src-im-bridge/core/card_schema.go src-im-bridge/core/card_renderer.go src-im-bridge/platform/{feishu,slack,dingtalk}/card_render.go src-go/internal/service/outbound_dispatcher.go` — must return zero matches.

- [ ] Step 13.4 — old-code dangling check
  - `rtk grep -n "renderInteractiveCard\|renderStructuredMessage" src-im-bridge/` — must return zero matches outside the renamed `renderStructured*` functions inside `card_render_structured.go`. If anything matches, route it through `core.DispatchCard` before this PR ships (per spec §12 deletion-in-same-PR rule).

- [ ] Step 13.5 — verify everything together
  - `rtk go build ./...` from both `src-go/` and `src-im-bridge/`.
  - `rtk go test ./...` from both.
  - `rtk pnpm test components/workflow/workflow-execution-view.test.tsx`.
  - `rtk lint` from repo root.

---

## Done criteria

1. A Feishu user types `/workflow echo` in a group chat; within 5s of the workflow completing they see a green-headed Feishu card in the same thread with title, summary, `Run` field, and "查看详情" link.
2. Repeating with a workflow whose final node uses `im_send` (1E feature) produces NO duplicate card — only the explicit one.
3. Killing the IM Bridge before completion → backend retries 3 times, then the FE execution detail page shows a "回帖失败" badge.
4. `rtk grep -n renderInteractiveCard src-im-bridge/` returns zero hits.
