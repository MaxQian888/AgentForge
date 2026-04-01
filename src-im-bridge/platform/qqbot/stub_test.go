package qqbot

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/core"
	"net/http"
	"strings"
	"testing"
)

func TestStub_MetadataAndReplyContextDeclareQQBotBehavior(t *testing.T) {
	stub := NewStub("0")

	if stub.Name() != "qqbot-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}

	metadata := stub.Metadata()
	if metadata.Source != "qqbot" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if !metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected callback requirement")
	}

	replyCtx := stub.ReplyContextFromTarget(&core.ReplyTarget{ChatID: "group-openid", MessageID: "evt-1", UserID: "user-openid"})
	ctx, ok := replyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyCtx)
	}
	if ctx.ChatID != "group-openid" || ctx.MessageID != "evt-1" {
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
	if got.Platform != "qqbot-stub" {
		t.Fatalf("Platform = %q", got.Platform)
	}
	if got.UserID != "qqbot-user" || got.ChatID != "group-openid" {
		t.Fatalf("message = %+v", got)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChatID != "group-openid" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
}

func TestStub_ReplyAndSendStoreReplies(t *testing.T) {
	stub := NewStub("0")

	if err := stub.Reply(context.Background(), replyContext{ChatID: "group-openid", MessageID: "evt-1"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := stub.Send(context.Background(), "user:user-openid", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_LogsNativeReplies(t *testing.T) {
	stub := NewStub("0")

	message, err := core.NewQQBotMarkdownMessage(
		"## Review Ready",
		nil,
	)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}

	if err := stub.SendNative(context.Background(), "group:group-openid", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if len(stub.replies) != 1 || stub.replies[0].NativeSurface != core.NativeSurfaceQQBotMarkdown {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_DeliverEnvelopeSupportsNativeAndStructured(t *testing.T) {
	stub := NewStub("0")

	native, err := core.NewQQBotMarkdownMessage("## Review Ready", nil)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}
	receipt, err := core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "group:group-openid", &core.DeliveryEnvelope{Native: native})
	if err != nil {
		t.Fatalf("DeliverEnvelope native error: %v", err)
	}
	if receipt.Type != "native" {
		t.Fatalf("native receipt = %+v", receipt)
	}

	receipt, err = core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "group:group-openid", &core.DeliveryEnvelope{
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
	if receipt.Type != "text" {
		t.Fatalf("structured receipt = %+v", receipt)
	}
	if len(stub.replies) != 2 || stub.replies[0].NativeSurface != core.NativeSurfaceQQBotMarkdown {
		t.Fatalf("replies = %+v", stub.replies)
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

func TestStub_HTTPHandlersExposeAndClearReplies(t *testing.T) {
	stub := NewStub("0")
	stub.replies = append(stub.replies, stubReply{ChatID: "group-openid", Content: "hello"})

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

func TestStub_FormattedTextSendAndReplyStoreReplies(t *testing.T) {
	stub := NewStub("0")

	if err := stub.SendFormattedText(context.Background(), "group:group-openid", &core.FormattedText{
		Content: "## Formatted Send",
		Format:  core.TextFormatQQBotMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}
	if err := stub.ReplyFormattedText(context.Background(), replyContext{ChatID: "group-openid", MessageID: "evt-1", IsGroup: true}, &core.FormattedText{
		Content: "## Formatted Reply",
		Format:  core.TextFormatQQBotMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if err := stub.UpdateFormattedText(context.Background(), replyContext{ChatID: "group-openid", IsGroup: true}, &core.FormattedText{
		Content: "## Formatted Update",
		Format:  core.TextFormatQQBotMD,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}

	if len(stub.replies) != 3 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].Content != "## Formatted Send" || stub.replies[0].NativeSurface != string(core.TextFormatQQBotMD) {
		t.Fatalf("first reply = %+v", stub.replies[0])
	}
	if stub.replies[1].Content != "## Formatted Reply" {
		t.Fatalf("second reply = %+v", stub.replies[1])
	}

	if err := stub.SendFormattedText(context.Background(), "group:group-openid", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil message error = %v", err)
	}
	if err := stub.ReplyFormattedText(context.Background(), replyContext{ChatID: "group-openid"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("nil reply message error = %v", err)
	}
}

func TestStub_SendStructuredRendersAsMarkdown(t *testing.T) {
	stub := NewStub("0")

	if err := stub.SendStructured(context.Background(), "group:group-openid", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Fields: []core.StructuredField{
			{Label: "Status", Value: "Open"},
		},
	}); err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if len(stub.replies) != 1 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if !strings.Contains(stub.replies[0].Content, "Review Ready") || !strings.Contains(stub.replies[0].Content, "Status") {
		t.Fatalf("reply content = %q", stub.replies[0].Content)
	}
	if stub.replies[0].NativeSurface != string(core.TextFormatQQBotMD) {
		t.Fatalf("reply format = %q", stub.replies[0].NativeSurface)
	}
}

func TestStub_HelperConversionsAndReplyNative(t *testing.T) {
	stub := NewStub("0")

	message, err := core.NewQQBotMarkdownMessage("## Review Ready", nil)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}
	if err := stub.ReplyNative(context.Background(), replyContext{UserID: "user-openid", MessageID: "evt-1"}, message); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if err := stub.SendStructured(context.Background(), "group:group-openid", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
	}); err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if len(stub.replies) != 2 || stub.replies[0].NativeSurface != core.NativeSurfaceQQBotMarkdown || !strings.Contains(stub.replies[1].Content, "Review Ready") {
		t.Fatalf("replies = %+v", stub.replies)
	}

	raw := replyContext{ChatID: "group-openid", UserID: "user-openid", MessageID: "evt-1", IsGroup: true}
	if got := toReplyContext(raw); got != raw {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ChatID: "group-openid", UserID: "user-openid", MessageID: "evt-2", IsGroup: true}); got.MessageID != "evt-2" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	msg := &core.Message{
		ChatID:      "group-openid",
		UserID:      "user-openid",
		IsGroup:     true,
		ReplyTarget: &core.ReplyTarget{MessageID: "evt-3"},
	}
	if got := toReplyContext(msg); got.ChatID != "group-openid" || got.MessageID != "evt-3" || !got.IsGroup {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}
	target := &core.ReplyTarget{ConversationID: "group-openid", UserID: "user-openid", MessageID: "evt-4"}
	if got := toReplyContext(target); got.ChatID != "group-openid" || got.MessageID != "evt-4" || !got.IsGroup {
		t.Fatalf("toReplyContext(target) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}
	if got := messageIDFromTarget(nil); got != "" {
		t.Fatalf("messageIDFromTarget(nil) = %q", got)
	}
	if got := messageIDFromTarget(&core.ReplyTarget{MessageID: " evt-5 "}); got != "evt-5" {
		t.Fatalf("messageIDFromTarget(target) = %q", got)
	}
}
