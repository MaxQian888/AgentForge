package slack

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	goslack "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"io"
	"net/http"
	"strings"
	"testing"
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
	posts   []slackOutgoingMessage
	updates []fakeUpdateCall
}

type fakeUpdateCall struct {
	ChannelID string
	MessageTS string
	Text      string
}

func (c *fakeSlackMessageClient) PostMessage(ctx context.Context, message slackOutgoingMessage) error {
	c.posts = append(c.posts, message)
	return nil
}

func (c *fakeSlackMessageClient) UpdateMessage(ctx context.Context, channelID, messageTS, text string) error {
	c.updates = append(c.updates, fakeUpdateCall{ChannelID: channelID, MessageTS: messageTS, Text: text})
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

type slackRoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn slackRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestSlackLive_NameReplyContextAndReplyBranches(t *testing.T) {
	messages := &fakeSlackMessageClient{}
	responses := &fakeSlackResponseClient{}
	live, err := NewLive(
		"xoxb-bot",
		"xapp-app",
		WithSocketRunner(&fakeSocketRunner{}),
		WithMessageClient(messages),
		WithResponseClient(responses),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "slack-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		ChatID:      "C111",
		ThreadID:    "1700000000.100000",
		ResponseURL: "https://hooks.slack.com/actions/test",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.ChannelID != "C111" || reply.ThreadTS != "1700000000.100000" || reply.ResponseURL != "https://hooks.slack.com/actions/test" {
		t.Fatalf("reply = %+v", reply)
	}

	if err := live.ReplyStructured(context.Background(), replyContext{
		ChannelID:   "C111",
		ThreadTS:    "1700000000.100000",
		ResponseURL: "https://hooks.slack.com/actions/test",
	}, &core.StructuredMessage{
		Sections: []core.StructuredSection{{
			Type: core.StructuredSectionTypeText,
			TextSection: &core.TextSection{
				Body: "Build ready",
			},
		}},
	}); err != nil {
		t.Fatalf("ReplyStructured error: %v", err)
	}
	if len(responses.calls) != 1 || responses.calls[0].ResponseURL != "https://hooks.slack.com/actions/test" {
		t.Fatalf("responses = %+v", responses.calls)
	}

	if err := live.ReplyCard(context.Background(), replyContext{
		ChannelID: "C222",
		ThreadTS:  "1700000000.200000",
	}, core.NewCard().
		SetTitle("Review Ready").
		AddPrimaryButton("Open", "link:https://example.test/reviews/1")); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}
	if len(messages.posts) != 1 || messages.posts[0].ChannelID != "C222" || len(messages.posts[0].Blocks) == 0 {
		t.Fatalf("messages.posts = %+v", messages.posts)
	}

	if err := live.ReplyFormattedText(context.Background(), replyContext{
		ChannelID: "C333",
		ThreadTS:  "1700000000.300000",
	}, &core.FormattedText{
		Content: "*bold*",
		Format:  core.TextFormatSlackMrkdwn,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if len(messages.posts) != 2 || messages.posts[1].ThreadTS != "1700000000.300000" {
		t.Fatalf("messages.posts = %+v", messages.posts)
	}

	if err := live.UpdateFormattedText(context.Background(), replyContext{
		ChannelID:   "C444",
		ResponseURL: "https://hooks.slack.com/actions/update",
	}, &core.FormattedText{
		Content: "*literal*",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(responses.calls) != 2 || responses.calls[1].ResponseURL != "https://hooks.slack.com/actions/update" {
		t.Fatalf("responses = %+v", responses.calls)
	}

	if err := live.ReplyStructured(context.Background(), replyContext{}, &core.StructuredMessage{Title: "missing target"}); err == nil || !strings.Contains(err.Error(), "channel id") {
		t.Fatalf("missing target err = %v", err)
	}
}

func TestLive_UpdateMessageCallsClientUpdate(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	rc := &replyContext{ChannelID: "C111", MessageTS: "1700000000.123456"}
	if err := live.UpdateMessage(context.Background(), rc, "updated text"); err != nil {
		t.Fatalf("UpdateMessage error: %v", err)
	}

	if len(messages.updates) != 1 {
		t.Fatalf("updates = %+v, want 1 update", messages.updates)
	}
	if messages.updates[0].ChannelID != "C111" || messages.updates[0].MessageTS != "1700000000.123456" || messages.updates[0].Text != "updated text" {
		t.Fatalf("update = %+v", messages.updates[0])
	}
}

func TestLive_UpdateMessageRejectsNilReplyContext(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateMessage(context.Background(), nil, "text"); err == nil || !strings.Contains(err.Error(), "invalid reply context") {
		t.Fatalf("UpdateMessage nil context err = %v", err)
	}

	if err := live.UpdateMessage(context.Background(), "wrong-type", "text"); err == nil || !strings.Contains(err.Error(), "invalid reply context") {
		t.Fatalf("UpdateMessage wrong type err = %v", err)
	}
}

func TestLive_UpdateMessageRejectsMissingMessageTS(t *testing.T) {
	runner := &fakeSocketRunner{}
	messages := &fakeSlackMessageClient{}

	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(messages))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	rc := &replyContext{ChannelID: "C111"}
	if err := live.UpdateMessage(context.Background(), rc, "text"); err == nil || !strings.Contains(err.Error(), "message timestamp required") {
		t.Fatalf("UpdateMessage missing ts err = %v", err)
	}
}

func TestLive_ReplyContextFromTargetIncludesMessageTS(t *testing.T) {
	runner := &fakeSocketRunner{}
	live, err := NewLive("xoxb-bot", "xapp-app", WithSocketRunner(runner), WithMessageClient(&fakeSlackMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		ChatID:    "C111",
		ThreadID:  "1700000000.100000",
		MessageID: "1700000000.200000",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.MessageTS != "1700000000.200000" {
		t.Fatalf("MessageTS = %q, want 1700000000.200000", reply.MessageTS)
	}
}

func TestSlackHelpers_NormalizationAndHTTPResponseClient(t *testing.T) {
	if _, err := normalizeEnvelope(socketEnvelope{Type: socketEnvelopeSlashCommand}); err == nil || !strings.Contains(err.Error(), "missing slash command payload") {
		t.Fatalf("normalizeEnvelope slash err = %v", err)
	}
	if _, err := normalizeEnvelope(socketEnvelope{Type: socketEnvelopeEventsAPI}); err == nil || !strings.Contains(err.Error(), "missing events api payload") {
		t.Fatalf("normalizeEnvelope events err = %v", err)
	}
	if _, err := normalizeEnvelope(socketEnvelope{Type: socketEnvelopeInteractive}); err != errIgnoreEnvelope {
		t.Fatalf("normalizeEnvelope interactive err = %v", err)
	}

	msg, err := normalizeEventsAPI(&slackevents.EventsAPIEvent{
		Type: slackevents.CallbackEvent,
		InnerEvent: slackevents.EventsAPIInnerEvent{
			Type: "message",
			Data: &slackevents.MessageEvent{
				User:            "U123",
				Text:            "hello",
				Channel:         "D456",
				TimeStamp:       "1700000000.123456",
				ThreadTimeStamp: "1700000000.100000",
				ChannelType:     "im",
			},
		},
	})
	if err != nil {
		t.Fatalf("normalizeEventsAPI message error: %v", err)
	}
	if msg.ChatID != "D456" || msg.IsGroup {
		t.Fatalf("message = %+v", msg)
	}
	if _, err := normalizeEventsAPI(&slackevents.EventsAPIEvent{
		Type: "url_verification",
	}); err != errIgnoreEnvelope {
		t.Fatalf("normalizeEventsAPI non-callback err = %v", err)
	}

	rawCtx := replyContext{ChannelID: "C111", ThreadTS: "1700000000.100000"}
	if got := toReplyContext(rawCtx); got != rawCtx {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ChannelID: "C222", ResponseURL: "https://hooks.slack.com"}); got.ChannelID != "C222" || got.ResponseURL == "" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	coreMsg := &core.Message{ChatID: "C333"}
	if got := toReplyContext(coreMsg); got.ChannelID != "C333" {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}

	if got := normalizeMentionText(" <@U999> hello "); got != "@AgentForge hello" {
		t.Fatalf("normalizeMentionText = %q", got)
	}
	if got := parseSlackTimestamp("1700000000.5"); got.Unix() != 1700000000 {
		t.Fatalf("parseSlackTimestamp(valid) = %v", got)
	}
	if !isDirectSlackChannel("D123") || isDirectSlackChannel("C123") {
		t.Fatalf("isDirectSlackChannel unexpected result")
	}
	if got := compactMetadata(map[string]string{" source ": " slash_command ", "empty": " "}); got["source"] != "slash_command" || len(got) != 1 {
		t.Fatalf("compactMetadata = %+v", got)
	}
	if got := valueOrEmpty(&fakeSlackResponseCall{ResponseURL: "https://hooks.slack.com"}, func(v *fakeSlackResponseCall) string { return v.ResponseURL }); got != "https://hooks.slack.com" {
		t.Fatalf("valueOrEmpty = %q", got)
	}

	calls := make([]map[string]any, 0, 2)
	client := &httpResponseClient{client: &http.Client{Transport: slackRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode response body: %v", err)
		}
		calls = append(calls, body)
		status := http.StatusOK
		if strings.Contains(req.URL.String(), "bad") {
			status = http.StatusBadGateway
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})}}

	if err := client.PostResponse(context.Background(), "https://hooks.slack.com/actions/test", slackOutgoingMessage{
		Text:     "hello",
		ThreadTS: "1700000000.100000",
		Markdown: boolPtr(true),
	}); err != nil {
		t.Fatalf("PostResponse error: %v", err)
	}
	if len(calls) != 1 || calls[0]["text"] != "hello" || calls[0]["mrkdwn"] != true || calls[0]["thread_ts"] != "1700000000.100000" {
		t.Fatalf("calls = %+v", calls)
	}
	if err := client.PostResponse(context.Background(), "", slackOutgoingMessage{}); err == nil || !strings.Contains(err.Error(), "response url is required") {
		t.Fatalf("missing response url err = %v", err)
	}
	if err := client.PostResponse(context.Background(), "https://hooks.slack.com/actions/bad", slackOutgoingMessage{Text: "boom"}); err == nil || !strings.Contains(err.Error(), "returned 502") {
		t.Fatalf("upstream err = %v", err)
	}
}
