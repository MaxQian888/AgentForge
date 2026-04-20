package slack_test

import (
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
	_ "github.com/agentforge/im-bridge/platform/slack" // init() registers
)

func TestSlackRenderCard_Success(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title:   "Echo",
		Status:  core.CardStatusSuccess,
		Summary: "hello world",
		Fields:  []core.CardField{{Label: "Run", Value: "abc"}},
		Actions: []core.CardAction{
			{ID: "view", Label: "View", Type: core.CardActionTypeURL, URL: "https://x/runs/abc"},
		},
		Footer: "2026-04-20",
	}, &core.ReplyTarget{Platform: "slack"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "blocks" {
		t.Fatalf("content type %s", out.ContentType)
	}
	if !strings.HasPrefix(out.Body, `{"blocks":[`) {
		t.Fatalf("expected blocks payload, got %s", out.Body)
	}
	if !strings.Contains(out.Body, `Echo`) {
		t.Fatal("missing title")
	}
	if !strings.Contains(out.Body, `hello world`) {
		t.Fatal("missing summary")
	}
	if !strings.Contains(out.Body, `https://x/runs/abc`) {
		t.Fatal("missing URL action")
	}
}

func TestSlackRenderCard_FailedHasButtons(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title:  "x",
		Status: core.CardStatusFailed,
	}, &core.ReplyTarget{Platform: "slack"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.Body, `{"blocks":[`) {
		t.Fatalf("expected blocks, got %s", out.Body)
	}
}

func TestSlackRenderCard_CallbackButtonCarriesToken(t *testing.T) {
	out, err := core.DispatchCard(core.ProviderNeutralCard{
		Title: "Approve?",
		Actions: []core.CardAction{{
			ID: "approve", Label: "Approve",
			Type: core.CardActionTypeCallback, CorrelationToken: "tok-xyz",
			Payload: map[string]any{"node_id": "wait-1"},
		}},
	}, &core.ReplyTarget{Platform: "slack"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Body, `tok-xyz`) {
		t.Fatal("expected correlation_token in slack button value")
	}
	if !strings.Contains(out.Body, `wait-1`) {
		t.Fatal("expected payload in slack button value")
	}
}
