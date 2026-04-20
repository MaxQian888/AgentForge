package dingtalk_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
	_ "github.com/agentforge/im-bridge/platform/dingtalk" // init() registers
)

func TestDingTalkRenderCard_Success(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title:   "Echo",
		Status:  core.CardStatusSuccess,
		Summary: "hello world",
		Fields:  []core.CardField{{Label: "Run", Value: "abc"}},
		Actions: []core.CardAction{
			{ID: "view", Label: "View", Type: core.CardActionTypeURL, URL: "https://x/runs/abc"},
		},
		Footer: "2026-04-20",
	}, &core.ReplyTarget{Platform: "dingtalk"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "actioncard" {
		t.Fatalf("content type %s", out.ContentType)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(out.Body), &p); err != nil {
		t.Fatal(err)
	}
	if p["card_type"] != "ActionCard" {
		t.Fatalf("card_type=%v", p["card_type"])
	}
	if p["title"] != "Echo" {
		t.Fatalf("title=%v", p["title"])
	}
	buttons, ok := p["buttons"].([]any)
	if !ok || len(buttons) != 1 {
		t.Fatalf("buttons=%v", p["buttons"])
	}
	if !strings.Contains(p["markdown"].(string), "hello world") {
		t.Fatal("missing summary in markdown")
	}
	if !strings.Contains(p["markdown"].(string), "**[SUCCESS]**") {
		t.Fatal("missing status badge in markdown")
	}
}

func TestDingTalkRenderCard_FailedHasMarkdown(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title: "x", Status: core.CardStatusFailed,
	}, &core.ReplyTarget{Platform: "dingtalk"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Body, `"card_type":"ActionCard"`) {
		t.Fatalf("expected ActionCard, got %s", out.Body)
	}
	if !strings.Contains(out.Body, `**[FAILED]**`) {
		t.Fatalf("expected FAILED badge, got %s", out.Body)
	}
}

func TestDingTalkRenderCard_CallbackButtonDegradesToMarkdown(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title: "Approve?",
		Actions: []core.CardAction{{
			ID: "approve", Label: "Approve",
			Type: core.CardActionTypeCallback, CorrelationToken: "tok-xyz",
			Payload: map[string]any{"node_id": "wait-1"},
		}},
	}, &core.ReplyTarget{Platform: "dingtalk"})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(out.Body), &p); err != nil {
		t.Fatal(err)
	}
	buttons, _ := p["buttons"].([]any)
	if len(buttons) != 0 {
		t.Fatalf("expected callback to degrade — buttons should be empty, got %v", buttons)
	}
	if !strings.Contains(p["markdown"].(string), "Approve") {
		t.Fatal("expected Approve label preserved in markdown")
	}
}
