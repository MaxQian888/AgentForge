package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

// rawCardSenderStub records the last raw card payload written through
// core.RawCardSender, so the receiver_card test can assert the bridge
// dispatched the ProviderNeutralCard via the new path instead of the legacy
// envelope.
type rawCardSenderStub struct {
	textOnlyPlatform
	chatID      string
	contentType string
	body        string
	target      *core.ReplyTarget
}

func (p *rawCardSenderStub) SendRawCard(ctx context.Context, chatID, contentType, body string, target *core.ReplyTarget) error {
	p.chatID = chatID
	p.contentType = contentType
	p.body = body
	p.target = target
	return nil
}

func TestReceiver_HandleSend_DispatchesProviderNeutralCard(t *testing.T) {
	p := &rawCardSenderStub{textOnlyPlatform: textOnlyPlatform{name: "feishu-stub"}}
	r := NewReceiverWithMetadata(p, core.PlatformMetadata{Source: "feishu"}, "0")

	// Register a stub feishu renderer so the dispatch maps to a known body.
	core.RegisterCardRenderer("feishu", func(c core.ProviderNeutralCard) (core.RenderedPayload, error) {
		return core.RenderedPayload{ContentType: "interactive", Body: `{"header":{"template":"green"},"title":"` + c.Title + `"}`}, nil
	})
	defer core.UnregisterCardRenderer("feishu")

	body, err := json.Marshal(SendRequest{
		Platform: "feishu",
		ChatID:   "c1",
		Card: &core.ProviderNeutralCard{
			Title: "T", Status: core.CardStatusSuccess, Summary: "ok",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if p.chatID != "c1" {
		t.Fatalf("chat_id = %q", p.chatID)
	}
	if p.contentType != "interactive" {
		t.Fatalf("content type = %q", p.contentType)
	}
	if !strings.Contains(p.body, `"green"`) {
		t.Fatalf("body missing rendered card payload: %q", p.body)
	}
}

func TestReceiver_HandleSend_RejectsCardCombinedWithContent(t *testing.T) {
	p := &rawCardSenderStub{textOnlyPlatform: textOnlyPlatform{name: "feishu-stub"}}
	r := NewReceiverWithMetadata(p, core.PlatformMetadata{Source: "feishu"}, "0")

	body, err := json.Marshal(SendRequest{
		Platform: "feishu",
		ChatID:   "c1",
		Content:  "should not coexist",
		Card:     &core.ProviderNeutralCard{Title: "T"},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
