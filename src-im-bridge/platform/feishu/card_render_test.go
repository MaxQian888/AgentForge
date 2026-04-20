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
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "interactive" {
		t.Fatalf("content type %s", out.ContentType)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out.Body), &payload); err != nil {
		t.Fatal(err)
	}
	header := payload["header"].(map[string]any)
	if header["template"] != "green" {
		t.Fatalf("status=success should map to green header, got %v", header["template"])
	}
	if !strings.Contains(out.Body, `"Echo"`) {
		t.Fatal("missing title")
	}
	if !strings.Contains(out.Body, `hello world`) {
		t.Fatal("missing summary")
	}
	if !strings.Contains(out.Body, `查看详情`) {
		t.Fatal("missing button")
	}
}

func TestFeishuRenderCard_FailedHeaderRed(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title: "x", Status: core.CardStatusFailed,
	}, &core.ReplyTarget{Platform: "feishu"})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(out.Body), &p); err != nil {
		t.Fatal(err)
	}
	if p["header"].(map[string]any)["template"] != "red" {
		t.Fatalf("failed → red header")
	}
}

func TestFeishuRenderCard_CallbackButtonCarriesToken(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title: "Approve?",
		Actions: []core.CardAction{{
			ID: "approve", Label: "Approve",
			Type: core.CardActionTypeCallback, CorrelationToken: "tok-xyz",
			Payload: map[string]any{"node_id": "wait-1"},
		}},
	}, &core.ReplyTarget{Platform: "feishu"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Body, `tok-xyz`) {
		t.Fatalf("button value must carry correlation_token")
	}
	if !strings.Contains(out.Body, `wait-1`) {
		t.Fatalf("button value must carry payload")
	}
}
