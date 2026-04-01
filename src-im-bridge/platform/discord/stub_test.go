package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/core"
	"net/http"
	"testing"
)

func TestStub_MetadataAndReplyContextDeclareDiscordBehavior(t *testing.T) {
	stub := NewStub("0")

	if stub.Name() != "discord-stub" {
		t.Fatalf("Name = %q", stub.Name())
	}

	metadata := stub.Metadata()
	if metadata.Source != "discord" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if !metadata.Capabilities.SupportsDeferredReply {
		t.Fatal("expected deferred reply capability")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash command capability")
	}

	replyCtx := stub.ReplyContextFromTarget(&core.ReplyTarget{ChannelID: "channel-1"})
	msg, ok := replyCtx.(*core.Message)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyCtx)
	}
	if msg.ChatID != "channel-1" {
		t.Fatalf("ReplyContextFromTarget chatID = %q", msg.ChatID)
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

	req, err := http.NewRequest(http.MethodPost, "/test/message", bytes.NewBufferString(`{"content":"/agent list"}`))
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	rr := testRecorder{}
	stub.handleTestMessage(&rr, req)

	if got == nil {
		t.Fatal("expected handler to receive message")
	}
	if got.Platform != "discord-stub" {
		t.Fatalf("Platform = %q", got.Platform)
	}
	if got.UserID != "discord-user" || got.ChatID != "discord-channel" {
		t.Fatalf("message = %+v", got)
	}
	if got.SessionKey != "discord-stub:discord-channel:discord-user" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.InteractionToken != "stub-interaction-token" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
	if !got.ReplyTarget.UseReply {
		t.Fatalf("expected reply target to prefer reply: %+v", got.ReplyTarget)
	}
}

func TestStub_ReplyAndSendStoreReplies(t *testing.T) {
	stub := NewStub("0")

	if err := stub.Reply(context.Background(), &core.Message{ChatID: "channel-1"}, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := stub.Send(context.Background(), "channel-2", "send text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].ChatID != "channel-1" || stub.replies[0].Content != "reply text" {
		t.Fatalf("first reply = %+v", stub.replies[0])
	}
	if stub.replies[1].ChatID != "channel-2" || stub.replies[1].Content != "send text" {
		t.Fatalf("second reply = %+v", stub.replies[1])
	}
}

func TestStub_LogsNativeAndFormattedReplies(t *testing.T) {
	stub := NewStub("0")

	message, err := core.NewDiscordEmbedMessage(
		"Build Ready",
		"Agent finished the run.",
		nil,
		0,
		nil,
	)
	if err != nil {
		t.Fatalf("NewDiscordEmbedMessage error: %v", err)
	}

	if err := stub.SendNative(context.Background(), "channel-1", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if err := stub.SendFormattedText(context.Background(), "channel-1", &core.FormattedText{
		Content: "**bold**",
		Format:  core.TextFormatDiscordMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].NativeSurface != core.NativeSurfaceDiscordEmbed {
		t.Fatalf("native reply = %+v", stub.replies[0])
	}
	if stub.replies[1].Format != string(core.TextFormatDiscordMD) {
		t.Fatalf("formatted reply = %+v", stub.replies[1])
	}
}

func TestStub_DeliverEnvelopeSupportsNativeStructuredAndFormatted(t *testing.T) {
	stub := NewStub("0")

	native, err := core.NewDiscordEmbedMessage("Build Ready", "Agent finished the run.", nil, 0, nil)
	if err != nil {
		t.Fatalf("NewDiscordEmbedMessage error: %v", err)
	}
	receipt, err := core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "channel-1", &core.DeliveryEnvelope{Native: native})
	if err != nil {
		t.Fatalf("DeliverEnvelope native error: %v", err)
	}
	if receipt.Type != "native" {
		t.Fatalf("native receipt = %+v", receipt)
	}

	receipt, err = core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "channel-1", &core.DeliveryEnvelope{
		Structured: &core.StructuredMessage{
			Sections: []core.StructuredSection{{
				Type: core.StructuredSectionTypeText,
				TextSection: &core.TextSection{
					Body: "Build ready",
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

	receipt, err = core.DeliverEnvelope(context.Background(), stub, stub.Metadata(), "channel-1", &core.DeliveryEnvelope{
		Content: "**bold**",
		Metadata: map[string]string{
			"text_format": string(core.TextFormatDiscordMD),
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope formatted error: %v", err)
	}
	if receipt.Type != "text" {
		t.Fatalf("formatted receipt = %+v", receipt)
	}
	if len(stub.replies) != 3 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].NativeSurface != core.NativeSurfaceDiscordEmbed || stub.replies[2].Format != string(core.TextFormatDiscordMD) {
		t.Fatalf("replies = %+v", stub.replies)
	}
}

func TestStub_HTTPHandlersExposeAndClearReplies(t *testing.T) {
	stub := NewStub("0")
	stub.replies = append(stub.replies, stubReply{ChatID: "channel-1", Content: "hello"})

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

func TestStub_SendCardAndReplyCardRecordReplies(t *testing.T) {
	stub := NewStub("0")

	card := core.NewCard().
		SetTitle("Build Ready").
		AddField("Status", "success").
		AddPrimaryButton("View", "act:view:build-1")

	if err := stub.SendCard(context.Background(), "channel-1", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}
	if err := stub.ReplyCard(context.Background(), &core.Message{ChatID: "channel-2"}, card); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}

	if len(stub.replies) != 2 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].ChatID != "channel-1" || stub.replies[0].Content != "Build Ready" || stub.replies[0].NativeSurface != "discord_card" {
		t.Fatalf("first reply = %+v", stub.replies[0])
	}
	if stub.replies[1].ChatID != "channel-2" || stub.replies[1].Content != "Build Ready" || stub.replies[1].NativeSurface != "discord_card" {
		t.Fatalf("second reply = %+v", stub.replies[1])
	}
}

func TestStub_TypingIndicatorIsNoop(t *testing.T) {
	stub := NewStub("0")

	if err := stub.StartTyping(context.Background(), "channel-1"); err != nil {
		t.Fatalf("StartTyping error: %v", err)
	}
	if err := stub.StopTyping(context.Background(), "channel-1"); err != nil {
		t.Fatalf("StopTyping error: %v", err)
	}
}

func TestDiscordStub_HelperBranches(t *testing.T) {
	stub := NewStub("0")

	if stub.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}
	replyAny := stub.ReplyContextFromTarget(&core.ReplyTarget{ChatID: "channel-1"})
	msg, ok := replyAny.(*core.Message)
	if !ok || msg.ChatID != "channel-1" {
		t.Fatalf("ReplyContextFromTarget = %#v", replyAny)
	}

	native, err := core.NewDiscordEmbedMessage("Build Ready", "Agent finished the run.", nil, 0, nil)
	if err != nil {
		t.Fatalf("NewDiscordEmbedMessage error: %v", err)
	}
	if err := stub.ReplyNative(context.Background(), &core.ReplyTarget{ChannelID: "channel-2"}, native); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if err := stub.ReplyFormattedText(context.Background(), &core.ReplyTarget{ChannelID: "channel-3"}, &core.FormattedText{
		Content: "formatted",
		Format:  core.TextFormatDiscordMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if err := stub.UpdateFormattedText(context.Background(), &core.ReplyTarget{ChannelID: "channel-4"}, &core.FormattedText{
		Content: "updated",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(stub.replies) != 3 {
		t.Fatalf("replies = %+v", stub.replies)
	}
	if stub.replies[0].NativeSurface != core.NativeSurfaceDiscordEmbed {
		t.Fatalf("native reply = %+v", stub.replies[0])
	}
	if stub.replies[1].Format != string(core.TextFormatDiscordMD) {
		t.Fatalf("formatted reply = %+v", stub.replies[1])
	}
	if stub.replies[2].Format != string(core.TextFormatPlainText) {
		t.Fatalf("updated reply = %+v", stub.replies[2])
	}

	if got := chatIDFromReplyContext(&core.ReplyTarget{ChannelID: "channel-5"}); got != "channel-5" {
		t.Fatalf("chatIDFromReplyContext(replyTarget) = %q", got)
	}
	if got := chatIDFromReplyContext("invalid"); got != "" {
		t.Fatalf("chatIDFromReplyContext(invalid) = %q", got)
	}
}
