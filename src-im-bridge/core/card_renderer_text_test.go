package core_test

import (
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderTextFallback_SuccessCard(t *testing.T) {
	got := core.RenderTextFallback(core.ProviderNeutralCard{
		Title:   "Echo workflow",
		Status:  core.CardStatusSuccess,
		Summary: "hello",
		Fields:  []core.CardField{{Label: "Run", Value: "abc-123"}},
		Footer:  "2026-04-20T10:00:00Z",
		Actions: []core.CardAction{
			{ID: "view", Label: "查看详情", Type: core.CardActionTypeURL, URL: "https://x/runs/abc-123"},
		},
	}).Body
	want := "Echo workflow\n[SUCCESS] hello\n[Run] abc-123\n2026-04-20T10:00:00Z\n[查看详情] https://x/runs/abc-123\n"
	if got != want {
		t.Fatalf("text fallback mismatch:\nwant=%q\ngot =%q", want, got)
	}
}

func TestRenderTextFallback_CallbackButtonOmittedNoURL(t *testing.T) {
	got := core.RenderTextFallback(core.ProviderNeutralCard{
		Title: "Wait",
		Actions: []core.CardAction{
			{ID: "approve", Label: "Approve", Type: core.CardActionTypeCallback, CorrelationToken: "t"},
		},
	}).Body
	want := "Wait\n[Approve] (interactive button — open AgentForge to respond)\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
