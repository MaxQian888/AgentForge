package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/core"
	"net/http"
	"strings"
	"testing"
)

func TestStub_MetadataAndReplyContextDeclareWeComBehavior(t *testing.T) {
	stub := NewStub("0")

	if stub.Name() != "wecom-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}

	metadata := stub.Metadata()
	if metadata.Source != "wecom" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != core.NativeSurfaceWeComCard {
		t.Fatalf("NativeSurfaces = %+v", metadata.Rendering.NativeSurfaces)
	}

	replyCtx := stub.ReplyContextFromTarget(&core.ReplyTarget{ChatID: "chat-1", UserID: "zhangsan"})
	ctx, ok := replyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyCtx)
	}
	if ctx.ChatID != "chat-1" || ctx.UserID != "zhangsan" {
		t.Fatalf("reply context = %+v", ctx)
	}
}

func TestStub_MapsInboundMessageAndAppliesDefaults(t *testing.T) {
	stub := NewStub("0")

	var got *core.Message
	if err := stub.Start(func(p core.Platform, msg *core.Message) {
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer stub.Stop()

	req, err := http.NewRequest(http.MethodPost, "/test/message", bytes.NewBufferString(`{"content":"/task list"}`))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	rr := testRecorder{}
	stub.handleTestMessage(&rr, req)

	if got == nil {
		t.Fatal("expected handler to receive message")
	}
	if got.Platform != "wecom" {
		t.Fatalf("Platform = %q", got.Platform)
	}
	if got.ChatID != "wecom-chat" || got.UserID != "wecom-user" {
		t.Fatalf("message = %+v", got)
	}
}

func TestStub_LogsNativeReplies(t *testing.T) {
	stub := NewStub("0")

	message, err := core.NewWeComCardMessage(
		core.WeComCardTypeNews,
		"Review Ready",
		"Choose the next step.",
		"https://example.test/reviews/1",
		[]core.WeComArticle{{
			Title:       "Review Ready",
			Description: "Choose the next step.",
			URL:         "https://example.test/reviews/1",
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewWeComCardMessage error: %v", err)
	}

	if err := stub.SendNative(context.Background(), "chat-1", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].NativeSurface != core.NativeSurfaceWeComCard {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_DeliverEnvelopeSupportsNativeAndStructured(t *testing.T) {
	stub := NewStub("0")

	native, err := core.NewWeComCardMessage(
		core.WeComCardTypeNews,
		"Review Ready",
		"Choose the next step.",
		"https://example.test/reviews/1",
		[]core.WeComArticle{{
			Title:       "Review Ready",
			Description: "Choose the next step.",
			URL:         "https://example.test/reviews/1",
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewWeComCardMessage error: %v", err)
	}
	receipt, err := core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "chat-1", &core.DeliveryEnvelope{Native: native})
	if err != nil {
		t.Fatalf("DeliverEnvelope native error: %v", err)
	}
	if receipt.Type != "native" {
		t.Fatalf("native receipt = %+v", receipt)
	}

	receipt, err = core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "chat-1", &core.DeliveryEnvelope{
		Structured: &core.StructuredMessage{
			Sections: []core.StructuredSection{{
				Type: core.StructuredSectionTypeText,
				TextSection: &core.TextSection{
					Body: "Review Ready",
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope structured error: %v", err)
	}
	if receipt.Type != "structured" {
		t.Fatalf("structured receipt = %+v", receipt)
	}
	if len(stub.replies) != 2 || stub.replies[0].NativeSurface != core.NativeSurfaceWeComCard {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_HTTPHandlersExposeAndClearReplies(t *testing.T) {
	stub := NewStub("0")
	stub.replies = append(stub.replies, stubReply{ChatID: "chat-1", Content: "hello"})

	getReq, err := http.NewRequest(http.MethodGet, "/test/replies", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	getRec := testRecorder{}
	stub.handleGetReplies(&getRec, getReq)

	var replies []stubReply
	if err := json.Unmarshal(getRec.buf.Bytes(), &replies); err != nil {
		t.Fatalf("unmarshal replies: %v", err)
	}
	if len(replies) != 1 || replies[0].Content != "hello" {
		t.Fatalf("replies = %+v", replies)
	}

	clearReq, err := http.NewRequest(http.MethodDelete, "/test/replies", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	clearRec := testRecorder{}
	stub.handleClearReplies(&clearRec, clearReq)

	if len(stub.replies) != 0 {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_InvalidJSONReturnsBadRequest(t *testing.T) {
	stub := NewStub("0")

	req, err := http.NewRequest(http.MethodPost, "/test/message", bytes.NewBufferString("{"))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	rr := testRecorder{}
	stub.handleTestMessage(&rr, req)

	if rr.code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.code, http.StatusBadRequest)
	}
}

type testRecorder struct {
	header http.Header
	code   int
	buf    bytes.Buffer
}

func (r *testRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *testRecorder) Write(data []byte) (int, error) { return r.buf.Write(data) }
func (r *testRecorder) WriteHeader(statusCode int)     { r.code = statusCode }

func TestStub_ReplyAndReplyNativeUseReplyTargets(t *testing.T) {
	stub := NewStub("0")

	if err := stub.Reply(context.Background(), replyContext{ChatID: "chat-1"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}

	message, err := core.NewWeComCardMessage(
		core.WeComCardTypeNews,
		"Review Ready",
		"Choose the next step.",
		"https://example.test/reviews/1",
		[]core.WeComArticle{{
			Title:       "Review Ready",
			Description: "Choose the next step.",
			URL:         "https://example.test/reviews/1",
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewWeComCardMessage error: %v", err)
	}
	if err := stub.ReplyNative(context.Background(), replyContext{UserID: "zhangsan"}, message); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].ChatID != "chat-1" {
		t.Fatalf("first reply = %+v", stub.replies[0])
	}
	if stub.replies[1].ChatID != "zhangsan" || stub.replies[1].NativeSurface != core.NativeSurfaceWeComCard {
		t.Fatalf("second reply = %+v", stub.replies[1])
	}
}

func TestStub_SendStructuredUsesFallbackText(t *testing.T) {
	stub := NewStub("0")
	if err := stub.SendStructured(context.Background(), "chat-2", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{Label: "Open", URL: "https://example.test/reviews/1"},
		},
	}); err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if len(stub.replies) != 1 || !strings.Contains(stub.replies[0].Content, "WeCom richer card update is unavailable") {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_FormattedTextSenderRecordsFormatAndContent(t *testing.T) {
	stub := NewStub("0")

	if err := stub.SendFormattedText(context.Background(), "chat-1", &core.FormattedText{
		Content: "**bold**",
		Format:  core.TextFormatWeComMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].Format != string(core.TextFormatWeComMD) || stub.replies[0].Content != "**bold**" {
		t.Fatalf("replies = %+v", stub.replies)
	}

	if err := stub.ReplyFormattedText(context.Background(), replyContext{ChatID: "chat-2"}, &core.FormattedText{
		Content: "reply markdown",
		Format:  core.TextFormatWeComMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if len(stub.replies) != 2 || stub.replies[1].ChatID != "chat-2" || stub.replies[1].Format != string(core.TextFormatWeComMD) {
		t.Fatalf("replies = %+v", stub.replies)
	}

	if err := stub.UpdateFormattedText(context.Background(), &core.ReplyTarget{ChannelID: "chat-3"}, &core.FormattedText{
		Content: "updated",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(stub.replies) != 3 || stub.replies[2].ChatID != "chat-3" {
		t.Fatalf("replies = %+v", stub.replies)
	}

	// nil message returns error
	if err := stub.SendFormattedText(context.Background(), "chat-1", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil SendFormattedText error = %v", err)
	}
	if err := stub.ReplyFormattedText(context.Background(), replyContext{ChatID: "chat-1"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil ReplyFormattedText error = %v", err)
	}
}

func TestStub_CardSenderRecordsCardFallbackText(t *testing.T) {
	stub := NewStub("0")

	card := core.NewCard().
		SetTitle("Review Ready").
		AddField("Status", "pending").
		AddPrimaryButton("Open", "link:https://example.test/reviews/1")

	if err := stub.SendCard(context.Background(), "chat-1", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].NativeSurface != "wecom_card" {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if !strings.Contains(stub.replies[0].Content, "Review Ready") || !strings.Contains(stub.replies[0].Content, "Status: pending") {
		t.Fatalf("content = %q", stub.replies[0].Content)
	}

	if err := stub.ReplyCard(context.Background(), replyContext{ChatID: "chat-2"}, card); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}
	if len(stub.replies) != 2 || stub.replies[1].ChatID != "chat-2" || stub.replies[1].NativeSurface != "wecom_card" {
		t.Fatalf("replies = %+v", stub.replies)
	}

	// nil card returns error
	if err := stub.SendCard(context.Background(), "chat-1", nil); err == nil || !strings.Contains(err.Error(), "card is required") {
		t.Fatalf("nil SendCard error = %v", err)
	}
	if err := stub.ReplyCard(context.Background(), replyContext{ChatID: "chat-1"}, nil); err == nil || !strings.Contains(err.Error(), "card is required") {
		t.Fatalf("nil ReplyCard error = %v", err)
	}
}

func TestChatIDFromReplyContext_ExtractsTargetFromVariousTypes(t *testing.T) {
	if got := chatIDFromReplyContext(replyContext{ChatID: "chat-1", UserID: "user-1"}); got != "chat-1" {
		t.Fatalf("replyContext = %q", got)
	}
	if got := chatIDFromReplyContext(&replyContext{UserID: "user-2"}); got != "user-2" {
		t.Fatalf("*replyContext = %q", got)
	}
	if got := chatIDFromReplyContext(&core.Message{ChatID: "chat-3"}); got != "chat-3" {
		t.Fatalf("*Message = %q", got)
	}
	if got := chatIDFromReplyContext(&core.ReplyTarget{ChannelID: "channel-4"}); got != "channel-4" {
		t.Fatalf("*ReplyTarget = %q", got)
	}
	if got := chatIDFromReplyContext("invalid"); got != "" {
		t.Fatalf("invalid = %q", got)
	}
	if got := chatIDFromReplyContext((*replyContext)(nil)); got != "" {
		t.Fatalf("nil *replyContext = %q", got)
	}
}
