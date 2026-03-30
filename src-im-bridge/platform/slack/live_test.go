package slack

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	goslack "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func TestLive_StartAcknowledgesSlashCommandBeforeDispatch(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var order []string
	var got *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		order = append(order, "handler")
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), socketEnvelope{
		Type: socketEnvelopeSlashCommand,
		Ack: func(payload any) error {
			order = append(order, "ack")
			return nil
		},
		SlashCommand: &goslack.SlashCommand{
			Command:     "/task",
			Text:        "list",
			UserID:      "U123",
			UserName:    "alice",
			ChannelID:   "C456",
			ChannelName: "ops",
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(order) != 2 || order[0] != "ack" || order[1] != "handler" {
		t.Fatalf("order = %v, want ack before handler", order)
	}
	if got == nil {
		t.Fatal("expected normalized slash command message")
	}
	if got.Platform != "slack" {
		t.Fatalf("Platform = %q, want slack", got.Platform)
	}
	if got.SessionKey != "slack:C456:U123" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
	}
	if got.Content != "/task list" {
		t.Fatalf("Content = %q", got.Content)
	}
	replyCtx, ok := got.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", got.ReplyCtx)
	}
	if replyCtx.ChannelID != "C456" || replyCtx.ResponseURL != "" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChannelID != "C456" || !got.ReplyTarget.UseReply {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
}

func TestLive_StartNormalizesAppMentionEvent(t *testing.T) {
	runner := &fakeSocketRunner{}
	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(&fakeSlackMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var got *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), socketEnvelope{
		Type: socketEnvelopeEventsAPI,
		Ack:  func(payload any) error { return nil },
		EventsAPI: &slackevents.EventsAPIEvent{
			Type: slackevents.CallbackEvent,
			InnerEvent: slackevents.EventsAPIInnerEvent{
				Type: "app_mention",
				Data: &slackevents.AppMentionEvent{
					Type:            "app_mention",
					User:            "U123",
					Text:            "<@U999> status",
					Channel:         "C456",
					TimeStamp:       "1700000000.123456",
					ThreadTimeStamp: "1700000000.100000",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if got == nil {
		t.Fatal("expected normalized app mention")
	}
	if got.Content != "@AgentForge status" {
		t.Fatalf("Content = %q, want @AgentForge status", got.Content)
	}
	replyCtx, ok := got.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", got.ReplyCtx)
	}
	if replyCtx.ThreadTS != "1700000000.100000" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChannelID != "C456" || got.ReplyTarget.ThreadID != "1700000000.100000" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
}

func TestLive_StartRoutesBlockActionToActionHandler(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}
	actions := &fakeSlackActionHandler{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	var order []string
	live.SetActionHandler(actions)
	actions.onHandle = func() {
		order = append(order, "action")
	}

	if err := live.Start(func(p core.Platform, msg *core.Message) {
		t.Fatalf("message handler should not receive interactive action payloads: %+v", msg)
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), socketEnvelope{
		Type: socketEnvelopeInteractive,
		Ack: func(payload any) error {
			order = append(order, "ack")
			return nil
		},
		Interaction: &goslack.InteractionCallback{
			Type:        goslack.InteractionTypeBlockActions,
			ResponseURL: "https://hooks.slack.com/actions/test",
			TriggerID:   "trigger-1",
			Channel: goslack.Channel{
				GroupConversation: goslack.GroupConversation{
					Conversation: goslack.Conversation{ID: "C456"},
				},
			},
			User: goslack.User{
				ID:   "U123",
				Name: "alice",
			},
			Container: goslack.Container{
				ChannelID: "C456",
				ThreadTs:  "1700000000.100000",
			},
			ActionCallback: goslack.ActionCallbacks{
				BlockActions: []*goslack.BlockAction{
					{
						ActionID: "approve_button",
						BlockID:  "review_actions",
						Value:    "act:approve:review-1",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(order) != 2 || order[0] != "ack" || order[1] != "action" {
		t.Fatalf("order = %v, want ack before action", order)
	}
	if len(actions.requests) != 1 {
		t.Fatalf("requests = %+v, want 1 request", actions.requests)
	}

	req := actions.requests[0]
	if req.Platform != "slack" {
		t.Fatalf("Platform = %q, want slack", req.Platform)
	}
	if req.Action != "approve" || req.EntityID != "review-1" {
		t.Fatalf("action request = %+v", req)
	}
	if req.ChatID != "C456" || req.UserID != "U123" {
		t.Fatalf("action request chat/user = %+v", req)
	}
	if req.ReplyTarget == nil {
		t.Fatal("expected reply target")
	}
	if req.ReplyTarget.ChannelID != "C456" || req.ReplyTarget.ThreadID != "1700000000.100000" {
		t.Fatalf("ReplyTarget = %+v", req.ReplyTarget)
	}
	if req.ReplyTarget.ResponseURL != "https://hooks.slack.com/actions/test" {
		t.Fatalf("ResponseURL = %q", req.ReplyTarget.ResponseURL)
	}
	if req.ReplyTarget.PreferredRenderer != "blocks" || !req.ReplyTarget.UseReply {
		t.Fatalf("ReplyTarget renderer/useReply = %+v", req.ReplyTarget)
	}
	if req.Metadata["source"] != "block_actions" || req.Metadata["action_id"] != "approve_button" || req.Metadata["block_id"] != "review_actions" {
		t.Fatalf("Metadata = %+v", req.Metadata)
	}
}

func TestLive_StartRoutesViewSubmissionToActionHandler(t *testing.T) {
	runner := &fakeSocketRunner{}
	actions := &fakeSlackActionHandler{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(&fakeSlackMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.SetActionHandler(actions)

	if err := live.Start(func(p core.Platform, msg *core.Message) {
		t.Fatalf("message handler should not receive modal submission payloads: %+v", msg)
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), socketEnvelope{
		Type: socketEnvelopeInteractive,
		Ack:  func(payload any) error { return nil },
		Interaction: &goslack.InteractionCallback{
			Type:      goslack.InteractionTypeViewSubmission,
			TriggerID: "trigger-2",
			User: goslack.User{
				ID:   "U999",
				Name: "bob",
			},
			View: goslack.View{
				ID:              "V123",
				PrivateMetadata: "act:request-changes:review-9",
				Hash:            "hash-1",
			},
			ViewSubmissionCallback: goslack.ViewSubmissionCallback{
				ResponseURLs: []goslack.ViewSubmissionCallbackResponseURL{
					{
						ChannelID:   "C999",
						ResponseURL: "https://hooks.slack.com/actions/view",
						BlockID:     "review_modal",
						ActionID:    "submit_review",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if len(actions.requests) != 1 {
		t.Fatalf("requests = %+v, want 1 request", actions.requests)
	}

	req := actions.requests[0]
	if req.Action != "request-changes" || req.EntityID != "review-9" {
		t.Fatalf("action request = %+v", req)
	}
	if req.ChatID != "C999" || req.UserID != "U999" {
		t.Fatalf("action request chat/user = %+v", req)
	}
	if req.ReplyTarget == nil || req.ReplyTarget.ResponseURL != "https://hooks.slack.com/actions/view" || req.ReplyTarget.ChannelID != "C999" {
		t.Fatalf("ReplyTarget = %+v", req.ReplyTarget)
	}
	if req.Metadata["source"] != "view_submission" || req.Metadata["view_id"] != "V123" || req.Metadata["view_hash"] != "hash-1" {
		t.Fatalf("Metadata = %+v", req.Metadata)
	}
}

func TestLive_ReplySendAndSendCardUseSlackMessageClient(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{ChannelID: "C111", ThreadTS: "1700000000.100000"}
	if err := live.Reply(context.Background(), replyCtx, "hello"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "C222", "broadcast"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Task Update").
		AddField("Status", "Done").
		AddPrimaryButton("Open", "link:https://example.test/task/1")
	if err := live.SendCard(context.Background(), "C333", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}

	if len(messages.posts) != 3 {
		t.Fatalf("posts = %+v", messages.posts)
	}
	if messages.posts[0].ChannelID != "C111" || messages.posts[0].ThreadTS != "1700000000.100000" || messages.posts[0].Text != "hello" {
		t.Fatalf("reply post = %+v", messages.posts[0])
	}
	if messages.posts[1].ChannelID != "C222" || messages.posts[1].Text != "broadcast" {
		t.Fatalf("send post = %+v", messages.posts[1])
	}
	if len(messages.posts[2].Blocks) == 0 {
		t.Fatalf("card post = %+v", messages.posts[2])
	}
}

func TestLive_ReplyUsesResponseURLWhenAvailable(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}
	responses := &fakeSlackResponseClient{}

	live, err := NewLive(
		"xoxb-bot",
		"xapp-app",
		WithSocketRunner(runner),
		WithMessageClient(messages),
		WithResponseClient(responses),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{
		ChannelID:   "C111",
		ThreadTS:    "1700000000.100000",
		ResponseURL: "https://hooks.slack.com/actions/test",
	}
	if err := live.Reply(context.Background(), replyCtx, "hello"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}

	if len(messages.posts) != 0 {
		t.Fatalf("expected response_url reply to avoid chat.postMessage, got posts=%+v", messages.posts)
	}
	if len(responses.calls) != 1 {
		t.Fatalf("response calls = %+v", responses.calls)
	}
	if responses.calls[0].ResponseURL != "https://hooks.slack.com/actions/test" || responses.calls[0].Text != "hello" {
		t.Fatalf("response call = %+v", responses.calls[0])
	}
}

func TestLive_StopReturnsRunnerError(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := &fakeSocketRunner{stopErr: stopErr}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(&fakeSlackMessageClient{}))
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

func TestLive_MetadataDeclaresDeferredRichSlackCapabilities(t *testing.T) {
	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(&fakeSocketRunner{}), WithMessageClient(&fakeSlackMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "slack" {
		t.Fatalf("Source = %q, want slack", metadata.Source)
	}
	if !metadata.Capabilities.SupportsDeferredReply {
		t.Fatal("expected deferred reply capability")
	}
	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected rich-message capability")
	}
	if !coreHasTextFormat(metadata.Rendering.SupportedFormats, core.TextFormatSlackMrkdwn) {
		t.Fatalf("SupportedFormats = %+v, want slack_mrkdwn", metadata.Rendering.SupportedFormats)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != core.NativeSurfaceSlackBlockKit {
		t.Fatalf("NativeSurfaces = %+v", metadata.Rendering.NativeSurfaces)
	}
}

func TestLive_SendNativeAndReplyNativeUseSlackBlockKit(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}
	responses := &fakeSlackResponseClient{}

	live, err := NewLive(
		"xoxb-bot",
		"xapp-app",
		WithSocketRunner(runner),
		WithMessageClient(messages),
		WithResponseClient(responses),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := core.NewSlackBlockKitMessage([]map[string]any{
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": "*Build* ready",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewSlackBlockKitMessage error: %v", err)
	}

	if err := live.SendNative(context.Background(), "C123", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}
	if len(messages.posts) != 1 {
		t.Fatalf("posts = %+v", messages.posts)
	}
	if len(messages.posts[0].Blocks) != 1 || messages.posts[0].Text == "" {
		t.Fatalf("native post = %+v", messages.posts[0])
	}

	if err := live.ReplyNative(context.Background(), replyContext{
		ChannelID:   "C123",
		ResponseURL: "https://hooks.slack.com/actions/native",
	}, message); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if len(responses.calls) != 1 || responses.calls[0].Blocks != 1 {
		t.Fatalf("response calls = %+v", responses.calls)
	}
}

func TestLive_SendFormattedTextControlsSlackMarkdownParsing(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendFormattedText(context.Background(), "C123", &core.FormattedText{
		Content: "*literal*",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText plain error: %v", err)
	}
	if err := live.SendFormattedText(context.Background(), "C123", &core.FormattedText{
		Content: "*bold* and ~strike~",
		Format:  core.TextFormatSlackMrkdwn,
	}); err != nil {
		t.Fatalf("SendFormattedText mrkdwn error: %v", err)
	}

	if len(messages.posts) != 2 {
		t.Fatalf("posts = %+v", messages.posts)
	}
	if messages.posts[0].Markdown == nil || *messages.posts[0].Markdown {
		t.Fatalf("plain formatted post = %+v", messages.posts[0])
	}
	if messages.posts[1].Markdown == nil || !*messages.posts[1].Markdown {
		t.Fatalf("mrkdwn formatted post = %+v", messages.posts[1])
	}
}

func TestLive_SendStructuredUsesSlackBlocks(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "C123", &core.StructuredMessage{
		Sections: []core.StructuredSection{
			{
				Type: core.StructuredSectionTypeText,
				TextSection: &core.TextSection{
					Body: "Build ready",
				},
			},
			{
				Type: core.StructuredSectionTypeActions,
				ActionsSection: &core.ActionsSection{
					Actions: []core.StructuredAction{{Label: "Open", URL: "https://example.test/builds/1"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}

	if len(messages.posts) != 1 {
		t.Fatalf("posts = %+v", messages.posts)
	}
	if len(messages.posts[0].Blocks) != 2 {
		t.Fatalf("structured post = %+v", messages.posts[0])
	}
}

func TestRenderStructuredSectionsBuildsSlackBlocks(t *testing.T) {
	blocks := renderStructuredSections([]core.StructuredSection{
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
			Type: core.StructuredSectionTypeContext,
			ContextSection: &core.ContextSection{
				Elements: []string{"alice", "2m ago"},
			},
		},
		{
			Type: core.StructuredSectionTypeFields,
			FieldsSection: &core.FieldsSection{
				Fields: []core.StructuredField{{Label: "Status", Value: "success"}},
			},
		},
		{
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{{Label: "Open", URL: "https://example.test/builds/1"}},
			},
		},
	})

	if len(blocks) != 5 {
		t.Fatalf("blocks = %+v", blocks)
	}
	raw, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("marshal blocks: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("expected serializable block payload")
	}
}

type fakeSocketRunner struct {
	handler func(context.Context, socketEnvelope) error
	stopErr error
}

func (r *fakeSocketRunner) Start(ctx context.Context, handler func(context.Context, socketEnvelope) error) error {
	r.handler = handler
	return nil
}

func (r *fakeSocketRunner) Stop(context.Context) error {
	return r.stopErr
}

func (r *fakeSocketRunner) dispatch(ctx context.Context, envelope socketEnvelope) error {
	if r.handler == nil {
		return errors.New("handler not registered")
	}
	return r.handler(ctx, envelope)
}

type fakeSlackMessageClient struct {
	posts []slackOutgoingMessage
}

func (c *fakeSlackMessageClient) PostMessage(ctx context.Context, message slackOutgoingMessage) error {
	c.posts = append(c.posts, message)
	return nil
}

type fakeSlackResponseClient struct {
	calls []fakeSlackResponseCall
}

type fakeSlackResponseCall struct {
	ResponseURL string
	Text        string
	ThreadTS    string
	Blocks      int
	Markdown    *bool
}

func (c *fakeSlackResponseClient) PostResponse(ctx context.Context, responseURL string, message slackOutgoingMessage) error {
	c.calls = append(c.calls, fakeSlackResponseCall{
		ResponseURL: responseURL,
		Text:        message.Text,
		ThreadTS:    message.ThreadTS,
		Blocks:      len(message.Blocks),
		Markdown:    message.Markdown,
	})
	return nil
}

type fakeSlackActionHandler struct {
	requests []*notify.ActionRequest
	onHandle func()
}

func (h *fakeSlackActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	if h.onHandle != nil {
		h.onHandle()
	}
	h.requests = append(h.requests, req)
	return &notify.ActionResponse{}, nil
}

func coreHasTextFormat(formats []core.TextFormatMode, target core.TextFormatMode) bool {
	for _, format := range formats {
		if format == target {
			return true
		}
	}
	return false
}
