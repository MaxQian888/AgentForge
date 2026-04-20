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
	if err != nil {
		t.Fatal(err)
	}
	if !registered["feishu"] || out.Body != `{"feishu":1}` {
		t.Fatalf("expected feishu route, got %+v", out)
	}
}

func TestDispatch_FallbackToText(t *testing.T) {
	out, err := core.DispatchCard(
		core.ProviderNeutralCard{Title: "T", Status: core.CardStatusSuccess, Summary: "ok"},
		&core.ReplyTarget{Platform: "qq"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.ContentType != "text" {
		t.Fatalf("expected text fallback, got %s", out.ContentType)
	}
	if out.Body == "" {
		t.Fatal("text fallback empty")
	}
}

func TestDispatch_RejectsInvalidCard(t *testing.T) {
	if _, err := core.DispatchCard(core.ProviderNeutralCard{}, &core.ReplyTarget{Platform: "feishu"}); err == nil {
		t.Fatal("expected validation error for empty title")
	}
}
