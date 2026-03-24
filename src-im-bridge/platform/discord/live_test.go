package discord

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestLive_StartAcknowledgesInteractionBeforeDispatchAndSyncsCommands(t *testing.T) {
	runner := &fakeInteractionRunner{}
	followups := &fakeFollowupClient{}
	channels := &fakeChannelClient{}
	registrar := &fakeCommandRegistrar{}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(followups),
		WithChannelClient(channels),
		WithCommandRegistrar(registrar),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var order []string
	var gotPlatform core.Platform
	var gotMessage *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		order = append(order, "handler")
		gotPlatform = p
		gotMessage = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	if len(registrar.calls) != 1 {
		t.Fatalf("registrar calls = %+v", registrar.calls)
	}
	if len(registrar.calls[0].Commands) != 4 {
		t.Fatalf("commands = %+v", registrar.calls[0].Commands)
	}

	err = runner.dispatch(context.Background(), interactionEnvelope{
		Interaction: &interaction{
			Type:          interactionTypeApplicationCommand,
			Token:         "interaction-token",
			ApplicationID: "app-123",
			ChannelID:     "channel-1",
			Data: &applicationCommandData{
				Name: "agent",
				Options: []applicationCommandOption{
					{Name: "args", Type: commandOptionTypeString, Value: "spawn task-123"},
				},
			},
			Member: &interactionMember{
				User: &interactionUser{
					ID:       "user-1",
					Username: "alice",
				},
			},
		},
		Ack: func(response interactionResponse) error {
			order = append(order, "ack")
			if response.Type != interactionCallbackTypeDeferredChannelMessageWithSource {
				t.Fatalf("ack response type = %d", response.Type)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(order) != 2 || order[0] != "ack" || order[1] != "handler" {
		t.Fatalf("order = %v, want ack before handler", order)
	}
	if gotPlatform != live {
		t.Fatalf("platform = %#v, want live platform", gotPlatform)
	}
	if gotMessage == nil {
		t.Fatal("expected normalized interaction message")
	}
	if gotMessage.Platform != "discord" {
		t.Fatalf("Platform = %q", gotMessage.Platform)
	}
	if gotMessage.Content != "/agent spawn task-123" {
		t.Fatalf("Content = %q", gotMessage.Content)
	}
	replyCtx, ok := gotMessage.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", gotMessage.ReplyCtx)
	}
	if replyCtx.InteractionToken != "interaction-token" || replyCtx.ChannelID != "channel-1" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
}

func TestLive_ReplyAndSendUseDiscordClients(t *testing.T) {
	runner := &fakeInteractionRunner{}
	followups := &fakeFollowupClient{}
	channels := &fakeChannelClient{}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(followups),
		WithChannelClient(channels),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{InteractionToken: "reply-token", ChannelID: "channel-1"}
	if err := live.Reply(context.Background(), replyCtx, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "channel-2", "notify text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(followups.calls) != 1 {
		t.Fatalf("followup calls = %+v", followups.calls)
	}
	if followups.calls[0].AppID != "app-123" || followups.calls[0].Token != "reply-token" || followups.calls[0].Content != "reply text" {
		t.Fatalf("followup call = %+v", followups.calls[0])
	}
	if len(channels.calls) != 1 {
		t.Fatalf("channel calls = %+v", channels.calls)
	}
	if channels.calls[0].ChannelID != "channel-2" || channels.calls[0].Content != "notify text" {
		t.Fatalf("channel call = %+v", channels.calls[0])
	}
}

func TestLive_MetadataDeclaresDeferredDiscordCapabilities(t *testing.T) {
	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(&fakeInteractionRunner{}),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(&fakeChannelClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "discord" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if !metadata.Capabilities.SupportsDeferredReply {
		t.Fatal("expected deferred reply capability")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash command capability")
	}
	if metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected discord live transport to rely on text fallback for notifications")
	}
}

func TestValidateRequestSignature_AcceptsSignedPayload(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}

	timestamp := "1700000000"
	body := []byte(`{"type":1}`)
	signature := ed25519.Sign(privateKey, append([]byte(timestamp), body...))

	err = validateRequestSignature(
		hex.EncodeToString(publicKey),
		timestamp,
		hex.EncodeToString(signature),
		body,
	)
	if err != nil {
		t.Fatalf("validateRequestSignature error: %v", err)
	}
}

func TestValidateRequestSignature_RejectsInvalidPayload(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}

	timestamp := "1700000000"
	signature := ed25519.Sign(privateKey, append([]byte(timestamp), []byte(`{"type":1}`)...))

	err = validateRequestSignature(
		hex.EncodeToString(publicKey),
		timestamp,
		hex.EncodeToString(signature),
		[]byte(`{"type":2}`),
	)
	if err == nil {
		t.Fatal("expected signature validation to fail for mismatched body")
	}
}

func TestLive_StopReturnsRunnerError(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := &fakeInteractionRunner{stopErr: stopErr}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(&fakeChannelClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	if err := live.Start(func(p core.Platform, msg *core.Message) {}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := live.Stop(); !errors.Is(err, stopErr) {
		t.Fatalf("Stop error = %v, want %v", err, stopErr)
	}
}

type fakeInteractionRunner struct {
	handler func(context.Context, interactionEnvelope) error
	stopErr error
}

func (r *fakeInteractionRunner) Start(ctx context.Context, handler func(context.Context, interactionEnvelope) error) error {
	r.handler = handler
	return nil
}

func (r *fakeInteractionRunner) Stop(context.Context) error {
	return r.stopErr
}

func (r *fakeInteractionRunner) dispatch(ctx context.Context, envelope interactionEnvelope) error {
	if r.handler == nil {
		return errors.New("handler not registered")
	}
	return r.handler(ctx, envelope)
}

type followupCall struct {
	AppID   string
	Token   string
	Content string
}

type fakeFollowupClient struct {
	calls []followupCall
}

func (f *fakeFollowupClient) SendFollowup(ctx context.Context, appID, token, content string) error {
	f.calls = append(f.calls, followupCall{
		AppID:   appID,
		Token:   token,
		Content: content,
	})
	return nil
}

type channelCall struct {
	ChannelID string
	Content   string
}

type fakeChannelClient struct {
	calls []channelCall
}

func (f *fakeChannelClient) SendChannelMessage(ctx context.Context, channelID, content string) error {
	f.calls = append(f.calls, channelCall{
		ChannelID: channelID,
		Content:   content,
	})
	return nil
}

type commandSyncCall struct {
	AppID    string
	GuildID  string
	Commands []applicationCommand
}

type fakeCommandRegistrar struct {
	calls []commandSyncCall
}

func (f *fakeCommandRegistrar) SyncCommands(ctx context.Context, appID, guildID string, commands []applicationCommand) error {
	cloned := make([]applicationCommand, len(commands))
	copy(cloned, commands)
	f.calls = append(f.calls, commandSyncCall{
		AppID:    appID,
		GuildID:  guildID,
		Commands: cloned,
	})
	return nil
}
