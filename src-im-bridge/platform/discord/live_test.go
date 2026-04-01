package discord

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLive_StartAcknowledgesInteractionBeforeDispatchAndSyncsCommands(t *testing.T) {
	runner := &fakeInteractionRunner{}
	followups := &fakeFollowupClient{}
	channels := &fakeChannelClient{}
	originals := &fakeOriginalResponseClient{}
	registrar := &fakeCommandRegistrar{}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(followups),
		WithChannelClient(channels),
		WithOriginalResponseClient(originals),
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
			Data: &interactionData{
				Name: "agent",
				Options: []interactionDataOption{
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
	if replyCtx.InteractionToken != "interaction-token" || replyCtx.ChannelID != "channel-1" || replyCtx.OriginalResponseID != "@original" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if gotMessage.ReplyTarget == nil || gotMessage.ReplyTarget.ChannelID != "channel-1" || gotMessage.ReplyTarget.InteractionToken != "interaction-token" {
		t.Fatalf("ReplyTarget = %+v", gotMessage.ReplyTarget)
	}
	if gotMessage.ReplyTarget.OriginalResponseID != "@original" || !gotMessage.ReplyTarget.PreferEdit {
		t.Fatalf("ReplyTarget = %+v", gotMessage.ReplyTarget)
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
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{InteractionToken: "reply-token", ChannelID: "channel-1", OriginalResponseID: "@original"}
	if err := live.Reply(context.Background(), replyCtx, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "channel-2", "notify text"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(followups.calls) != 1 {
		t.Fatalf("followup calls = %+v", followups.calls)
	}
	if followups.calls[0].AppID != "app-123" || followups.calls[0].Token != "reply-token" || followups.calls[0].Message.Content != "reply text" {
		t.Fatalf("followup call = %+v", followups.calls[0])
	}
	if len(channels.calls) != 1 {
		t.Fatalf("channel calls = %+v", channels.calls)
	}
	if channels.calls[0].ChannelID != "channel-2" || channels.calls[0].Message.Content != "notify text" {
		t.Fatalf("channel call = %+v", channels.calls[0])
	}
}

func TestLive_StartRoutesMessageComponentToActionHandlerAndEditsOriginalResponse(t *testing.T) {
	runner := &fakeInteractionRunner{}
	followups := &fakeFollowupClient{}
	channels := &fakeChannelClient{}
	originals := &fakeOriginalResponseClient{}
	actions := &fakeDiscordActionHandler{}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(followups),
		WithChannelClient(channels),
		WithOriginalResponseClient(originals),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.SetActionHandler(actions)

	if err := live.Start(func(p core.Platform, msg *core.Message) {
		t.Fatalf("message handler should not receive component interactions: %+v", msg)
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), interactionEnvelope{
		Interaction: &interaction{
			Type:          interactionTypeMessageComponent,
			Token:         "component-token",
			ApplicationID: "app-123",
			ChannelID:     "channel-1",
			Data: &interactionData{
				ComponentType: componentTypeButton,
				CustomID:      "act:approve:review-1",
			},
			Message: &interactionMessage{
				ID: "message-1",
			},
			Member: &interactionMember{
				User: &interactionUser{
					ID:       "user-1",
					Username: "alice",
				},
			},
		},
		Ack: func(response interactionResponse) error {
			if response.Type != interactionCallbackTypeDeferredUpdateMessage {
				t.Fatalf("ack response type = %d", response.Type)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(actions.requests) != 1 {
		t.Fatalf("requests = %+v", actions.requests)
	}
	req := actions.requests[0]
	if req.Platform != "discord" || req.Action != "approve" || req.EntityID != "review-1" {
		t.Fatalf("request = %+v", req)
	}
	if req.ChatID != "channel-1" || req.UserID != "user-1" {
		t.Fatalf("request = %+v", req)
	}
	if req.ReplyTarget == nil || req.ReplyTarget.InteractionToken != "component-token" || req.ReplyTarget.OriginalResponseID != "@original" {
		t.Fatalf("ReplyTarget = %+v", req.ReplyTarget)
	}
	if !req.ReplyTarget.PreferEdit {
		t.Fatalf("expected PreferEdit reply target: %+v", req.ReplyTarget)
	}
	if req.Metadata["source"] != "message_component" || req.Metadata["custom_id"] != "act:approve:review-1" {
		t.Fatalf("Metadata = %+v", req.Metadata)
	}
	if len(originals.calls) != 1 {
		t.Fatalf("original response calls = %+v", originals.calls)
	}
	if originals.calls[0].AppID != "app-123" || originals.calls[0].Token != "component-token" || originals.calls[0].Message.Content != "Approved" {
		t.Fatalf("original response call = %+v", originals.calls[0])
	}
}

func TestLive_UpdateMessageUsesDiscordOriginalResponseEditor(t *testing.T) {
	runner := &fakeInteractionRunner{}
	originals := &fakeOriginalResponseClient{}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(&fakeChannelClient{}),
		WithOriginalResponseClient(originals),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateMessage(context.Background(), replyContext{InteractionToken: "reply-token", OriginalResponseID: "@original"}, "updated text"); err != nil {
		t.Fatalf("UpdateMessage error: %v", err)
	}

	if len(originals.calls) != 1 {
		t.Fatalf("original response calls = %+v", originals.calls)
	}
	if originals.calls[0].AppID != "app-123" || originals.calls[0].Token != "reply-token" || originals.calls[0].Message.Content != "updated text" {
		t.Fatalf("original response call = %+v", originals.calls[0])
	}
}

func TestLive_SendStructuredUsesDiscordComponents(t *testing.T) {
	runner := &fakeInteractionRunner{}
	channels := &fakeChannelClient{}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(channels),
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "channel-1", &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve", Style: core.ActionStylePrimary},
			{URL: "https://example.test/reviews/1", Label: "Open"},
		},
	})
	if err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}

	if len(channels.calls) != 1 {
		t.Fatalf("channel calls = %+v", channels.calls)
	}
	call := channels.calls[0]
	if call.ChannelID != "channel-1" {
		t.Fatalf("ChannelID = %q", call.ChannelID)
	}
	if call.Message.Content == "" {
		t.Fatalf("message = %+v", call.Message)
	}
	if len(call.Message.Components) != 2 {
		t.Fatalf("components = %+v", call.Message.Components)
	}
	if len(call.Message.Components[0].Components) != 1 || call.Message.Components[0].Components[0].CustomID != "act:approve:review-1" {
		t.Fatalf("first row = %+v", call.Message.Components[0])
	}
	if len(call.Message.Components[1].Components) != 1 || call.Message.Components[1].Components[0].URL != "https://example.test/reviews/1" {
		t.Fatalf("second row = %+v", call.Message.Components[1])
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
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
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
	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected discord live transport to advertise rich-message support")
	}
	if !coreHasTextFormat(metadata.Rendering.SupportedFormats, core.TextFormatDiscordMD) {
		t.Fatalf("SupportedFormats = %+v, want discord_md", metadata.Rendering.SupportedFormats)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != core.NativeSurfaceDiscordEmbed {
		t.Fatalf("NativeSurfaces = %+v", metadata.Rendering.NativeSurfaces)
	}
}

func TestLive_SendNativeAndReplyNativeUseDiscordEmbeds(t *testing.T) {
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
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := core.NewDiscordEmbedMessage(
		"Build Ready",
		"Agent finished the run.",
		[]core.DiscordEmbedField{{Name: "Status", Value: "success"}},
		0x00FF00,
		[]core.DiscordActionRow{{
			Buttons: []core.DiscordButton{{Label: "Open", URL: "https://example.test/builds/1", Style: "link"}},
		}},
	)
	if err != nil {
		t.Fatalf("NewDiscordEmbedMessage error: %v", err)
	}

	if err := live.SendNative(context.Background(), "channel-1", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if len(channels.calls) != 1 {
		t.Fatalf("channel calls = %+v", channels.calls)
	}
	if len(channels.calls[0].Message.Embeds) != 1 || len(channels.calls[0].Message.Components) != 1 {
		t.Fatalf("channel message = %+v", channels.calls[0].Message)
	}

	if err := live.ReplyNative(context.Background(), replyContext{
		InteractionToken: "reply-token",
		ChannelID:        "channel-1",
	}, message); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if len(followups.calls) != 1 || len(followups.calls[0].Message.Embeds) != 1 {
		t.Fatalf("followup calls = %+v", followups.calls)
	}
}

func TestLive_SendFormattedTextSupportsDiscordMarkdown(t *testing.T) {
	runner := &fakeInteractionRunner{}
	channels := &fakeChannelClient{}

	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(runner),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(channels),
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendFormattedText(context.Background(), "channel-1", &core.FormattedText{
		Content: "**literal**",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText plain error: %v", err)
	}
	if err := live.SendFormattedText(context.Background(), "channel-1", &core.FormattedText{
		Content: "**bold** and ~~strike~~",
		Format:  core.TextFormatDiscordMD,
	}); err != nil {
		t.Fatalf("SendFormattedText markdown error: %v", err)
	}

	if len(channels.calls) != 2 {
		t.Fatalf("channel calls = %+v", channels.calls)
	}
	if channels.calls[0].Message.Content == "**literal**" {
		t.Fatalf("expected plain-text send to escape markdown, got %+v", channels.calls[0].Message)
	}
	if channels.calls[1].Message.Content != "**bold** and ~~strike~~" {
		t.Fatalf("markdown message = %+v", channels.calls[1].Message)
	}
}

func TestRenderStructuredSectionsBuildsDiscordEmbedsAndComponents(t *testing.T) {
	embed, components := renderStructuredSections([]core.StructuredSection{
		{
			Type: core.StructuredSectionTypeText,
			TextSection: &core.TextSection{
				Body: "Build ready",
			},
		},
		{
			Type:           core.StructuredSectionTypeDivider,
			DividerSection: &core.DividerSection{},
		},
		{
			Type: core.StructuredSectionTypeFields,
			FieldsSection: &core.FieldsSection{
				Fields: []core.StructuredField{{Label: "Status", Value: "success"}},
			},
		},
		{
			Type: core.StructuredSectionTypeImage,
			ImageSection: &core.ImageSection{
				URL: "https://example.test/build.png",
			},
		},
		{
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{{Label: "Open", URL: "https://example.test/builds/1"}},
			},
		},
	})

	raw, err := json.Marshal(embed)
	if err != nil {
		t.Fatalf("marshal embed: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("expected serializable embed")
	}
	if embed.Description == "" || len(embed.Fields) != 1 || embed.Image.URL == "" {
		t.Fatalf("embed = %+v", embed)
	}
	if len(components) != 1 || len(components[0].Components) != 1 {
		t.Fatalf("components = %+v", components)
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
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
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

func TestLive_SendCardUsesChannelClient(t *testing.T) {
	channels := &fakeChannelClient{}
	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(&fakeInteractionRunner{}),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(channels),
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Build Ready").
		AddField("Status", "success").
		AddPrimaryButton("View", "act:view:build-1")

	if err := live.SendCard(context.Background(), "channel-1", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}
	if len(channels.calls) != 1 {
		t.Fatalf("channel calls = %+v", channels.calls)
	}
	call := channels.calls[0]
	if call.ChannelID != "channel-1" {
		t.Fatalf("ChannelID = %q", call.ChannelID)
	}
	if len(call.Message.Embeds) != 1 || call.Message.Embeds[0].Title != "Build Ready" {
		t.Fatalf("embeds = %+v", call.Message.Embeds)
	}
	if len(call.Message.Components) != 1 {
		t.Fatalf("components = %+v", call.Message.Components)
	}

	if err := live.SendCard(context.Background(), "", card); err == nil || !strings.Contains(err.Error(), "channel id") {
		t.Fatalf("expected channel id error, got: %v", err)
	}
}

func TestLive_ReplyCardUsesFollowupOrChannel(t *testing.T) {
	followups := &fakeFollowupClient{}
	channels := &fakeChannelClient{}
	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(&fakeInteractionRunner{}),
		WithFollowupClient(followups),
		WithChannelClient(channels),
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	card := core.NewCard().SetTitle("Review Ready")

	if err := live.ReplyCard(context.Background(), replyContext{InteractionToken: "reply-token", ChannelID: "channel-1"}, card); err != nil {
		t.Fatalf("ReplyCard via followup error: %v", err)
	}
	if len(followups.calls) != 1 || followups.calls[0].Message.Embeds[0].Title != "Review Ready" {
		t.Fatalf("followup calls = %+v", followups.calls)
	}

	if err := live.ReplyCard(context.Background(), replyContext{ChannelID: "channel-2"}, card); err != nil {
		t.Fatalf("ReplyCard via channel error: %v", err)
	}
	if len(channels.calls) != 1 || channels.calls[0].ChannelID != "channel-2" {
		t.Fatalf("channel calls = %+v", channels.calls)
	}

	if err := live.ReplyCard(context.Background(), replyContext{}, card); err == nil || !strings.Contains(err.Error(), "requires interaction token or channel id") {
		t.Fatalf("expected missing target error, got: %v", err)
	}
}

func TestLive_StartTypingCallsAPIAndStopTypingIsNoop(t *testing.T) {
	typing := &fakeTypingClient{}
	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(&fakeInteractionRunner{}),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(&fakeChannelClient{}),
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
		WithTypingClient(typing),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.StartTyping(context.Background(), "channel-1"); err != nil {
		t.Fatalf("StartTyping error: %v", err)
	}
	if len(typing.calls) != 1 || typing.calls[0] != "channel-1" {
		t.Fatalf("typing calls = %+v", typing.calls)
	}

	// Empty channel should be silently ignored
	if err := live.StartTyping(context.Background(), ""); err != nil {
		t.Fatalf("StartTyping empty error: %v", err)
	}
	if len(typing.calls) != 1 {
		t.Fatalf("typing calls after empty = %+v", typing.calls)
	}

	// StopTyping is always a no-op
	if err := live.StopTyping(context.Background(), "channel-1"); err != nil {
		t.Fatalf("StopTyping error: %v", err)
	}
}

type fakeTypingClient struct {
	calls []string
}

func (f *fakeTypingClient) TriggerTyping(ctx context.Context, channelID string) error {
	f.calls = append(f.calls, channelID)
	return nil
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
	Message discordOutgoingMessage
}

type fakeFollowupClient struct {
	calls []followupCall
}

func (f *fakeFollowupClient) SendFollowup(ctx context.Context, appID, token string, message discordOutgoingMessage) error {
	f.calls = append(f.calls, followupCall{
		AppID:   appID,
		Token:   token,
		Message: message,
	})
	return nil
}

type channelCall struct {
	ChannelID string
	Message   discordOutgoingMessage
}

type fakeChannelClient struct {
	calls []channelCall
}

func (f *fakeChannelClient) SendChannelMessage(ctx context.Context, channelID string, message discordOutgoingMessage) error {
	f.calls = append(f.calls, channelCall{
		ChannelID: channelID,
		Message:   message,
	})
	return nil
}

type originalResponseCall struct {
	AppID   string
	Token   string
	Message discordOutgoingMessage
}

type fakeOriginalResponseClient struct {
	calls []originalResponseCall
}

func (f *fakeOriginalResponseClient) EditOriginalResponse(ctx context.Context, appID, token string, message discordOutgoingMessage) error {
	f.calls = append(f.calls, originalResponseCall{
		AppID:   appID,
		Token:   token,
		Message: message,
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

type fakeDiscordActionHandler struct {
	requests []*notify.ActionRequest
}

func (h *fakeDiscordActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	h.requests = append(h.requests, req)
	return &notify.ActionResponse{Result: "Approved"}, nil
}

func coreHasTextFormat(formats []core.TextFormatMode, target core.TextFormatMode) bool {
	for _, format := range formats {
		if format == target {
			return true
		}
	}
	return false
}

type discordRoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn discordRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestDiscordLive_NameOptionsAndReplyContextHelpers(t *testing.T) {
	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(&fakeInteractionRunner{}),
		WithFollowupClient(&fakeFollowupClient{}),
		WithChannelClient(&fakeChannelClient{}),
		WithOriginalResponseClient(&fakeOriginalResponseClient{}),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
		WithCommandGuildID(" guild-1 "),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "discord-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.commandGuildID != "guild-1" {
		t.Fatalf("commandGuildID = %q", live.commandGuildID)
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		InteractionToken: " reply-token ",
		ChatID:           "channel-1",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.InteractionToken != "reply-token" || reply.ChannelID != "channel-1" || reply.OriginalResponseID != "@original" {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestDiscordLive_ReplyStructuredFormattedAndUpdateBranches(t *testing.T) {
	followups := &fakeFollowupClient{}
	channels := &fakeChannelClient{}
	originals := &fakeOriginalResponseClient{}
	live, err := NewLive(
		"app-123",
		"bot-token",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"9000",
		WithInteractionRunner(&fakeInteractionRunner{}),
		WithFollowupClient(followups),
		WithChannelClient(channels),
		WithOriginalResponseClient(originals),
		WithCommandRegistrar(&fakeCommandRegistrar{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.ReplyStructured(context.Background(), replyContext{
		InteractionToken: "reply-token",
		ChannelID:        "channel-1",
	}, &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve"},
		},
	}); err != nil {
		t.Fatalf("ReplyStructured via followup error: %v", err)
	}
	if len(followups.calls) != 1 || len(followups.calls[0].Message.Components) != 1 {
		t.Fatalf("followups = %+v", followups.calls)
	}

	followups.calls = nil
	if err := live.ReplyFormattedText(context.Background(), replyContext{
		ChannelID: "channel-2",
	}, &core.FormattedText{
		Content: "**bold**",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("ReplyFormattedText channel error: %v", err)
	}
	if len(channels.calls) != 1 || channels.calls[0].ChannelID != "channel-2" {
		t.Fatalf("channels = %+v", channels.calls)
	}

	if err := live.UpdateFormattedText(context.Background(), replyContext{
		InteractionToken: "update-token",
	}, &core.FormattedText{
		Content: "**bold**",
		Format:  core.TextFormatDiscordMD,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(originals.calls) != 1 || originals.calls[0].Token != "update-token" {
		t.Fatalf("originals = %+v", originals.calls)
	}

	if err := live.ReplyStructured(context.Background(), replyContext{}, &core.StructuredMessage{Title: "missing target"}); err == nil || !strings.Contains(err.Error(), "requires interaction token or channel id") {
		t.Fatalf("missing target error = %v", err)
	}
	if err := live.UpdateFormattedText(context.Background(), replyContext{}, &core.FormattedText{Content: "missing"}); err == nil || !strings.Contains(err.Error(), "requires interaction token") {
		t.Fatalf("missing update token error = %v", err)
	}
}

func TestDiscordAPIWrappersAndHelpers(t *testing.T) {
	requests := make([]struct {
		path   string
		auth   string
		method string
		body   map[string]any
	}, 0, 5)

	api := &discordAPIClient{
		baseURL:  "https://discord.example/api",
		botToken: "bot-token",
		client: &http.Client{Transport: discordRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var body map[string]any
			if req.Body != nil {
				_ = json.NewDecoder(req.Body).Decode(&body)
			}
			requests = append(requests, struct {
				path   string
				auth   string
				method string
				body   map[string]any
			}{
				path:   req.URL.Path,
				auth:   req.Header.Get("Authorization"),
				method: req.Method,
				body:   body,
			})
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		})},
	}

	followups := &discordFollowupClient{client: api}
	channels := &discordChannelClient{client: api}
	originals := &discordOriginalResponseClient{client: api}
	registrar := &discordCommandRegistrar{client: api}

	if err := followups.SendFollowup(context.Background(), "app-123", "reply-token", discordOutgoingMessage{Content: "reply text"}); err != nil {
		t.Fatalf("SendFollowup error: %v", err)
	}
	if err := channels.SendChannelMessage(context.Background(), "channel-1", discordOutgoingMessage{Content: "notify text"}); err != nil {
		t.Fatalf("SendChannelMessage error: %v", err)
	}
	if err := originals.EditOriginalResponse(context.Background(), "app-123", "reply-token", discordOutgoingMessage{Content: "updated"}); err != nil {
		t.Fatalf("EditOriginalResponse error: %v", err)
	}
	if err := registrar.SyncCommands(context.Background(), "app-123", "guild-1", defaultApplicationCommands()); err != nil {
		t.Fatalf("SyncCommands error: %v", err)
	}

	if len(requests) != 4 {
		t.Fatalf("requests = %+v", requests)
	}
	if requests[0].path != "/api/webhooks/app-123/reply-token" || requests[0].auth != "" {
		t.Fatalf("followup request = %+v", requests[0])
	}
	if requests[1].path != "/api/channels/channel-1/messages" || requests[1].auth != "Bot bot-token" {
		t.Fatalf("channel request = %+v", requests[1])
	}
	if requests[2].path != "/api/webhooks/app-123/reply-token/messages/@original" || requests[2].method != http.MethodPatch {
		t.Fatalf("original request = %+v", requests[2])
	}
	if requests[3].path != "/api/applications/app-123/guilds/guild-1/commands" || requests[3].method != http.MethodPut {
		t.Fatalf("command request = %+v", requests[3])
	}

	if err := followups.SendFollowup(context.Background(), "app-123", "", discordOutgoingMessage{}); err == nil || !strings.Contains(err.Error(), "interaction token") {
		t.Fatalf("followup missing token err = %v", err)
	}
	if err := channels.SendChannelMessage(context.Background(), "", discordOutgoingMessage{}); err == nil || !strings.Contains(err.Error(), "channel id") {
		t.Fatalf("channel missing id err = %v", err)
	}
	if err := originals.EditOriginalResponse(context.Background(), "app-123", "", discordOutgoingMessage{}); err == nil || !strings.Contains(err.Error(), "interaction token") {
		t.Fatalf("original missing token err = %v", err)
	}

	errorAPI := &discordAPIClient{
		baseURL:  "https://discord.example/api",
		botToken: "bot-token",
		client: &http.Client{Transport: discordRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("upstream boom")),
				Header:     make(http.Header),
			}, nil
		})},
	}
	if err := errorAPI.doJSON(context.Background(), http.MethodPost, "/bad", map[string]any{"x": 1}, true); err == nil || !strings.Contains(err.Error(), "discord api error 502") {
		t.Fatalf("doJSON error = %v", err)
	}

	if got := toReplyContext(replyContext{InteractionToken: "reply-token"}); got.InteractionToken != "reply-token" {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ChannelID: "channel-1"}); got.ChannelID != "channel-1" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	msg := &core.Message{
		ChatID:      "channel-2",
		ReplyTarget: &core.ReplyTarget{InteractionToken: "reply-token", OriginalResponseID: "@original"},
	}
	if got := toReplyContext(msg); got.ChannelID != "channel-2" || got.InteractionToken != "reply-token" || got.OriginalResponseID != "@original" {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}

	components := buildMessageComponents(&core.StructuredMessage{
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve", Style: core.ActionStylePrimary},
			{URL: "https://example.test/reviews/1", Label: "Open"},
		},
	})
	if len(components) != 2 || components[0].Components[0].CustomID != "act:approve:review-1" || components[1].Components[0].URL != "https://example.test/reviews/1" {
		t.Fatalf("components = %+v", components)
	}

	if got := compactMetadata(map[string]string{" source ": " interaction ", "empty": " "}); got["source"] != "interaction" || len(got) != 1 {
		t.Fatalf("compactMetadata = %+v", got)
	}
	if got := compactMetadata(map[string]string{" ": " "}); got != nil {
		t.Fatalf("compactMetadata(empty) = %+v", got)
	}
	if got := valueOrEmpty(&followupCall{AppID: "app-123"}, func(v *followupCall) string { return v.AppID }); got != "app-123" {
		t.Fatalf("valueOrEmpty = %q", got)
	}
	if got := valueOrEmpty[*followupCall](nil, func(v **followupCall) string { return "" }); got != "" {
		t.Fatalf("valueOrEmpty(nil) = %q", got)
	}
}
