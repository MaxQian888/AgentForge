package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLive_StartNormalizesSlashCommandUpdate(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var gotPlatform core.Platform
	var gotMessage *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		gotPlatform = p
		gotMessage = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), update{
		Message: &message{
			MessageID: 42,
			Date:      time.Unix(1_700_000_000, 0).Unix(),
			Text:      "/task@agentforge_bot list",
			From: &user{
				ID:        1001,
				Username:  "alice",
				FirstName: "Alice",
			},
			Chat: &chat{
				ID:    -2001,
				Title: "Ops",
				Type:  "group",
			},
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if gotPlatform != live {
		t.Fatalf("platform = %#v, want live platform", gotPlatform)
	}
	if gotMessage == nil {
		t.Fatal("expected normalized message")
	}
	if gotMessage.Platform != "telegram" {
		t.Fatalf("Platform = %q", gotMessage.Platform)
	}
	if gotMessage.SessionKey != "telegram:-2001:1001" {
		t.Fatalf("SessionKey = %q", gotMessage.SessionKey)
	}
	if gotMessage.Content != "/task list" {
		t.Fatalf("Content = %q", gotMessage.Content)
	}
	replyCtx, ok := gotMessage.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", gotMessage.ReplyCtx)
	}
	if replyCtx.ChatID != -2001 || replyCtx.MessageID != 42 {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if gotMessage.ReplyTarget == nil || gotMessage.ReplyTarget.ChatID != "-2001" || gotMessage.ReplyTarget.MessageID != "42" {
		t.Fatalf("ReplyTarget = %+v", gotMessage.ReplyTarget)
	}
	if !gotMessage.IsGroup {
		t.Fatal("expected group update to be marked as group")
	}
}

func TestLive_ReplyAndSendUseTelegramSender(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{ChatID: 12345, MessageID: 99}
	if err := live.Reply(context.Background(), replyCtx, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "-4001", "broadcast"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if len(sender.calls) != 2 {
		t.Fatalf("calls = %+v", sender.calls)
	}
	if sender.calls[0].ChatID != 12345 || sender.calls[0].ReplyToMessageID != 99 || sender.calls[0].Text != "reply text" {
		t.Fatalf("reply call = %+v", sender.calls[0])
	}
	if sender.calls[1].ChatID != -4001 || sender.calls[1].ReplyToMessageID != 0 || sender.calls[1].Text != "broadcast" {
		t.Fatalf("send call = %+v", sender.calls[1])
	}
}

func TestLive_StartRoutesCallbackQueryToActionHandlerAndEditsMessage(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}
	actions := &fakeTelegramActionHandler{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.SetActionHandler(actions)

	if err := live.Start(func(p core.Platform, msg *core.Message) {
		t.Fatalf("message handler should not receive callback query payloads: %+v", msg)
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), update{
		CallbackQuery: &callbackQuery{
			ID:   "callback-1",
			Data: "act:approve:review-1",
			From: &user{ID: 1001, Username: "alice"},
			Message: &message{
				MessageID:       42,
				MessageThreadID: 777,
				Date:            time.Unix(1_700_000_000, 0).Unix(),
				Text:            "Choose an action",
				Chat: &chat{
					ID:    -2001,
					Title: "Ops",
					Type:  "supergroup",
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
	if req.Platform != "telegram" || req.Action != "approve" || req.EntityID != "review-1" {
		t.Fatalf("action request = %+v", req)
	}
	if req.ChatID != "-2001" || req.UserID != "1001" {
		t.Fatalf("chat/user = %+v", req)
	}
	if req.ReplyTarget == nil {
		t.Fatal("expected reply target")
	}
	if req.ReplyTarget.MessageID != "42" || req.ReplyTarget.TopicID != "777" {
		t.Fatalf("ReplyTarget = %+v", req.ReplyTarget)
	}
	if !req.ReplyTarget.PreferEdit {
		t.Fatalf("expected callback query reply target to prefer edit: %+v", req.ReplyTarget)
	}
	if req.Metadata["source"] != "callback_query" || req.Metadata["callback_query_id"] != "callback-1" {
		t.Fatalf("Metadata = %+v", req.Metadata)
	}
	if len(sender.callbackAnswers) != 1 || sender.callbackAnswers[0].CallbackQueryID != "callback-1" {
		t.Fatalf("callback answers = %+v", sender.callbackAnswers)
	}
	if len(sender.edits) != 1 {
		t.Fatalf("edits = %+v", sender.edits)
	}
	if sender.edits[0].ChatID != -2001 || sender.edits[0].MessageID != 42 || sender.edits[0].Text != "Approved" {
		t.Fatalf("edit = %+v", sender.edits[0])
	}
}

func TestLive_UpdateMessageUsesTelegramEditAPI(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateMessage(context.Background(), replyContext{ChatID: -2001, MessageID: 42, TopicID: 777}, "updated text"); err != nil {
		t.Fatalf("UpdateMessage error: %v", err)
	}

	if len(sender.edits) != 1 {
		t.Fatalf("edits = %+v", sender.edits)
	}
	if sender.edits[0].ChatID != -2001 || sender.edits[0].MessageID != 42 || sender.edits[0].Text != "updated text" {
		t.Fatalf("edit = %+v", sender.edits[0])
	}
}

func TestLive_SendStructuredUsesInlineKeyboardMarkup(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "-2001", &core.StructuredMessage{
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

	if len(sender.structuredCalls) != 1 {
		t.Fatalf("structuredCalls = %+v", sender.structuredCalls)
	}
	call := sender.structuredCalls[0]
	if call.ChatID != -2001 {
		t.Fatalf("ChatID = %d", call.ChatID)
	}
	if call.Text == "" || call.Markup == nil {
		t.Fatalf("structured call = %+v", call)
	}
	if len(call.Markup.InlineKeyboard) != 2 {
		t.Fatalf("inline keyboard = %+v", call.Markup.InlineKeyboard)
	}
	if call.Markup.InlineKeyboard[0][0].CallbackData != "act:approve:review-1" {
		t.Fatalf("first button = %+v", call.Markup.InlineKeyboard[0][0])
	}
	if call.Markup.InlineKeyboard[1][0].URL != "https://example.test/reviews/1" {
		t.Fatalf("second button = %+v", call.Markup.InlineKeyboard[1][0])
	}
}

func TestLive_StartPreservesTopicReplyTargetFromMessage(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var gotMessage *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		gotMessage = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	err = runner.dispatch(context.Background(), update{
		Message: &message{
			MessageID:       42,
			MessageThreadID: 777,
			Date:            time.Unix(1_700_000_000, 0).Unix(),
			Text:            "/task list",
			From:            &user{ID: 1001, Username: "alice"},
			Chat:            &chat{ID: -2001, Title: "Ops", Type: "supergroup"},
		},
	})
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if gotMessage == nil || gotMessage.ReplyTarget == nil {
		t.Fatalf("message = %+v", gotMessage)
	}
	if gotMessage.ReplyTarget.TopicID != "777" {
		t.Fatalf("ReplyTarget = %+v", gotMessage.ReplyTarget)
	}
	replyCtx, ok := gotMessage.ReplyCtx.(replyContext)
	if !ok || replyCtx.TopicID != 777 {
		t.Fatalf("ReplyCtx = %#v", gotMessage.ReplyCtx)
	}
}

func TestLive_MetadataDeclaresTelegramCapabilities(t *testing.T) {
	live, err := NewLive("bot-token", WithUpdateRunner(&fakeUpdateRunner{}), WithSender(&fakeSender{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "telegram" {
		t.Fatalf("Source = %q", metadata.Source)
	}
	if metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected telegram live transport to use text fallback for rich notifications")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash command capability")
	}
	if !metadata.Capabilities.SupportsMentions {
		t.Fatal("expected mention capability")
	}
	foundMarkdown := false
	for _, format := range metadata.Rendering.SupportedFormats {
		if format == core.TextFormatMarkdownV2 {
			foundMarkdown = true
			break
		}
	}
	if !foundMarkdown {
		t.Fatalf("SupportedFormats = %+v, want markdown_v2", metadata.Rendering.SupportedFormats)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != core.NativeSurfaceTelegramRich {
		t.Fatalf("NativeSurfaces = %+v", metadata.Rendering.NativeSurfaces)
	}
}

func TestLive_SendNativeUsesTelegramRichPayload(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := core.NewTelegramRichMessage(
		"*Build* ready",
		"MarkdownV2",
		[][]core.TelegramInlineButton{{
			{Text: "Open", URL: "https://example.test/builds/1"},
		}},
	)
	if err != nil {
		t.Fatalf("NewTelegramRichMessage error: %v", err)
	}

	if err := live.SendNative(context.Background(), "-2001", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}

	if len(sender.structuredCalls) != 1 {
		t.Fatalf("structuredCalls = %+v", sender.structuredCalls)
	}
	call := sender.structuredCalls[0]
	if call.ChatID != -2001 || call.ParseMode != "MarkdownV2" {
		t.Fatalf("call = %+v", call)
	}
	if call.Markup == nil || len(call.Markup.InlineKeyboard) != 1 || call.Markup.InlineKeyboard[0][0].URL != "https://example.test/builds/1" {
		t.Fatalf("markup = %+v", call.Markup)
	}
}

func TestRenderStructuredSectionsBuildsTelegramTextAndKeyboard(t *testing.T) {
	message, markup := renderStructuredSections([]core.StructuredSection{
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
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{
					{ID: "act:approve:review-1", Label: "Approve"},
					{URL: "https://example.test/builds/1", Label: "Open"},
				},
				ButtonsPerRow: 2,
			},
		},
	})

	if message.Text == "" {
		t.Fatalf("message = %+v", message)
	}
	if markup == nil || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 2 {
		t.Fatalf("markup = %+v", markup)
	}
	if markup.InlineKeyboard[0][0].CallbackData != "act:approve:review-1" {
		t.Fatalf("first button = %+v", markup.InlineKeyboard[0][0])
	}
	if markup.InlineKeyboard[0][1].URL != "https://example.test/builds/1" {
		t.Fatalf("second button = %+v", markup.InlineKeyboard[0][1])
	}
}

func TestLive_DeliverEnvelopeUsesFormattedTextWhenRequested(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	receipt, err := core.DeliverEnvelope(context.Background(), live, live.Metadata(), "-2001", &core.DeliveryEnvelope{
		Content: "build *status*",
		Metadata: map[string]string{
			"text_format": "markdown_v2",
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Type != "text" || receipt.Method != core.DeliveryMethodSend {
		t.Fatalf("receipt = %+v", receipt)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("calls = %+v", sender.calls)
	}
	if sender.calls[0].ParseMode != "MarkdownV2" {
		t.Fatalf("call = %+v", sender.calls[0])
	}
	if sender.calls[0].Text != `build \*status\*` {
		t.Fatalf("text = %q", sender.calls[0].Text)
	}
}

func TestNormalizeIncomingUpdateRejectsUnsupportedPayload(t *testing.T) {
	_, err := normalizeIncomingUpdate(update{})
	if err == nil {
		t.Fatal("expected update without message payload to fail")
	}
}

func TestNormalizeUpdateModeRejectsWebhookWhenLongPollingChosen(t *testing.T) {
	if err := validateUpdateMode("longpoll", "https://example.test/webhook"); err == nil {
		t.Fatal("expected long polling with webhook url to fail")
	}
}

func TestLive_StopReturnsRunnerError(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := &fakeUpdateRunner{stopErr: stopErr}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(&fakeSender{}))
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

type fakeUpdateRunner struct {
	handler func(context.Context, update) error
	stopErr error
}

func (r *fakeUpdateRunner) Start(ctx context.Context, handler func(context.Context, update) error) error {
	r.handler = handler
	return nil
}

func (r *fakeUpdateRunner) Stop(context.Context) error {
	return r.stopErr
}

func (r *fakeUpdateRunner) dispatch(ctx context.Context, incoming update) error {
	if r.handler == nil {
		return errors.New("handler not registered")
	}
	return r.handler(ctx, incoming)
}

type sendCall struct {
	ChatID           int64
	TopicID          int64
	ReplyToMessageID int
	Text             string
	ParseMode        string
}

type editCall struct {
	ChatID    int64
	MessageID int
	Text      string
	ParseMode string
}

type callbackAnswerCall struct {
	CallbackQueryID string
	Text            string
}

type structuredCall struct {
	ChatID    int64
	TopicID   int64
	Text      string
	ParseMode string
	Markup    *inlineKeyboardMarkup
}

type chatActionCall struct {
	ChatID int64
	Action string
}

type fakeSender struct {
	calls           []sendCall
	edits           []editCall
	callbackAnswers []callbackAnswerCall
	structuredCalls []structuredCall
	chatActions     []chatActionCall
}

func (s *fakeSender) SendText(ctx context.Context, chatID int64, topicID int64, replyToMessageID int, message telegramTextMessage) error {
	s.calls = append(s.calls, sendCall{
		ChatID:           chatID,
		TopicID:          topicID,
		ReplyToMessageID: replyToMessageID,
		Text:             message.Text,
		ParseMode:        message.ParseMode,
	})
	return nil
}

func (s *fakeSender) EditText(ctx context.Context, chatID int64, messageID int, message telegramTextMessage) error {
	s.edits = append(s.edits, editCall{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      message.Text,
		ParseMode: message.ParseMode,
	})
	return nil
}

func (s *fakeSender) AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error {
	s.callbackAnswers = append(s.callbackAnswers, callbackAnswerCall{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	})
	return nil
}

func (s *fakeSender) SendStructured(ctx context.Context, chatID int64, topicID int64, message telegramTextMessage, markup *inlineKeyboardMarkup) error {
	s.structuredCalls = append(s.structuredCalls, structuredCall{
		ChatID:    chatID,
		TopicID:   topicID,
		Text:      message.Text,
		ParseMode: message.ParseMode,
		Markup:    markup,
	})
	return nil
}

func (s *fakeSender) SendChatAction(ctx context.Context, chatID int64, action string) error {
	s.chatActions = append(s.chatActions, chatActionCall{
		ChatID: chatID,
		Action: action,
	})
	return nil
}

type fakeTelegramActionHandler struct {
	requests []*notify.ActionRequest
}

func (h *fakeTelegramActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	h.requests = append(h.requests, req)
	return &notify.ActionResponse{Result: "Approved"}, nil
}

type telegramRoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn telegramRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestLive_NameReplyStructuredAndReplyFormattedText(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "telegram-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		ChatID:    "-2001",
		TopicID:   "777",
		MessageID: "42",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.ChatID != -2001 || reply.TopicID != 777 || reply.MessageID != 42 {
		t.Fatalf("reply = %+v", reply)
	}

	if err := live.ReplyStructured(context.Background(), replyContext{ChatID: -2001, TopicID: 777}, &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Actions: []core.StructuredAction{
			{ID: "act:approve:review-1", Label: "Approve"},
		},
	}); err != nil {
		t.Fatalf("ReplyStructured error: %v", err)
	}
	if len(sender.structuredCalls) != 1 || sender.structuredCalls[0].ChatID != -2001 || sender.structuredCalls[0].TopicID != 777 {
		t.Fatalf("structuredCalls = %+v", sender.structuredCalls)
	}

	if err := live.ReplyFormattedText(context.Background(), replyContext{ChatID: -2001, TopicID: 777, MessageID: 42}, &core.FormattedText{
		Content: "*Build* ready",
		Format:  core.TextFormatMarkdownV2,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if len(sender.calls) != 1 || sender.calls[0].ReplyToMessageID != 42 || sender.calls[0].ParseMode != "MarkdownV2" {
		t.Fatalf("calls = %+v", sender.calls)
	}

	if err := live.ReplyStructured(context.Background(), replyContext{}, &core.StructuredMessage{Title: "Missing target"}); err == nil || !strings.Contains(err.Error(), "requires chat id") {
		t.Fatalf("missing target error = %v", err)
	}
}

func TestTelegramHelpers_ParseContextValidationAndNativeRendering(t *testing.T) {
	raw := replyContext{ChatID: -2001, TopicID: 777, MessageID: 42}
	if got := toReplyContext(raw); got != raw {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ChatID: -3001, TopicID: 888, MessageID: 43}); got.ChatID != -3001 || got.TopicID != 888 || got.MessageID != 43 {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	msg := &core.Message{
		ChatID:      "-4001",
		ThreadID:    "999",
		ReplyTarget: &core.ReplyTarget{MessageID: "44"},
	}
	if got := toReplyContext(msg); got.ChatID != -4001 || got.TopicID != 999 || got.MessageID != 44 {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}

	if _, err := parseChatID("abc"); err == nil || !strings.Contains(err.Error(), "numeric chat id") {
		t.Fatalf("parseChatID(non-numeric) err = %v", err)
	}
	if _, err := parseChatID("0"); err == nil || !strings.Contains(err.Error(), "requires chat id") {
		t.Fatalf("parseChatID(zero) err = %v", err)
	}
	if got, err := parseChatID("-2001"); err != nil || got != -2001 {
		t.Fatalf("parseChatID(valid) = %d, %v", got, err)
	}

	if err := validateUpdateMode("", ""); err != nil {
		t.Fatalf("validateUpdateMode(default) error: %v", err)
	}
	if err := validateUpdateMode("webhook", ""); err == nil || !strings.Contains(err.Error(), "only longpoll update mode") {
		t.Fatalf("validateUpdateMode(webhook) = %v", err)
	}

	if got := normalizeCommandText("/task@AgentForge   list "); got != "/task list" {
		t.Fatalf("normalizeCommandText = %q", got)
	}
	if got := descriptionOrFallback("  ", "fallback"); got != "fallback" {
		t.Fatalf("descriptionOrFallback(blank) = %q", got)
	}
	if got := descriptionOrFallback(" detailed error ", "fallback"); got != "detailed error" {
		t.Fatalf("descriptionOrFallback(desc) = %q", got)
	}
	if got := telegramParseMode(core.TextFormatMarkdownV2); got != "MarkdownV2" {
		t.Fatalf("telegramParseMode(markdown) = %q", got)
	}
	if got := telegramParseMode(core.TextFormatPlainText); got != "" {
		t.Fatalf("telegramParseMode(plain) = %q", got)
	}

	message, err := core.NewTelegramRichMessage(
		"*Build* ready",
		"MarkdownV2",
		[][]core.TelegramInlineButton{{
			{Text: "Open", URL: "https://example.test/builds/1"},
		}},
	)
	if err != nil {
		t.Fatalf("NewTelegramRichMessage error: %v", err)
	}
	textMessage, markup, err := renderTelegramNativeMessage(message)
	if err != nil {
		t.Fatalf("renderTelegramNativeMessage error: %v", err)
	}
	if textMessage.Text != "*Build* ready" || textMessage.ParseMode != "MarkdownV2" {
		t.Fatalf("textMessage = %+v", textMessage)
	}
	if markup == nil || len(markup.InlineKeyboard) != 1 || markup.InlineKeyboard[0][0].URL != "https://example.test/builds/1" {
		t.Fatalf("markup = %+v", markup)
	}
	if _, _, err := renderTelegramNativeMessage(&core.NativeMessage{Platform: "slack"}); err == nil || !strings.Contains(err.Error(), "native message") {
		t.Fatalf("renderTelegramNativeMessage(non-telegram) err = %v", err)
	}
}

func TestLive_ReplyNativeUsesReplyTargetAndRejectsMissingChat(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}
	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := core.NewTelegramRichMessage(
		"*Build* ready",
		"MarkdownV2",
		[][]core.TelegramInlineButton{{{
			Text: "Open",
			URL:  "https://example.test/builds/1",
		}}},
	)
	if err != nil {
		t.Fatalf("NewTelegramRichMessage error: %v", err)
	}

	if err := live.ReplyNative(context.Background(), replyContext{ChatID: -2001, TopicID: 777}, message); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if len(sender.structuredCalls) != 1 || sender.structuredCalls[0].ChatID != -2001 || sender.structuredCalls[0].TopicID != 777 {
		t.Fatalf("structuredCalls = %+v", sender.structuredCalls)
	}
	if err := live.ReplyNative(context.Background(), replyContext{}, message); err == nil || !strings.Contains(err.Error(), "requires chat id") {
		t.Fatalf("missing chat error = %v", err)
	}
}

func TestBotAPISenderAndClientMethods(t *testing.T) {
	requests := make([]struct {
		path string
		body map[string]any
	}, 0, 5)
	client := &botAPIClient{
		baseURL: "https://api.telegram.example/bot-token",
		client: &http.Client{Transport: telegramRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			requests = append(requests, struct {
				path string
				body map[string]any
			}{path: req.URL.Path, body: body})

			responseBody := `{"ok":true,"result":{"message_id":1}}`
			if strings.HasSuffix(req.URL.Path, "/getUpdates") {
				responseBody = `{"ok":true,"result":[{"update_id":1}]}`
			} else if strings.HasSuffix(req.URL.Path, "/answerCallbackQuery") || strings.HasSuffix(req.URL.Path, "/deleteWebhook") {
				responseBody = `{"ok":true,"result":true}`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(responseBody)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	sender := &botAPISender{client: client}

	if err := sender.SendText(context.Background(), -2001, 777, 42, telegramTextMessage{Text: "hello", ParseMode: "MarkdownV2"}); err != nil {
		t.Fatalf("SendText error: %v", err)
	}
	if err := sender.EditText(context.Background(), -2001, 42, telegramTextMessage{Text: "updated", ParseMode: "MarkdownV2"}); err != nil {
		t.Fatalf("EditText error: %v", err)
	}
	if err := sender.AnswerCallbackQuery(context.Background(), "cb-1", "done"); err != nil {
		t.Fatalf("AnswerCallbackQuery error: %v", err)
	}
	if err := sender.SendStructured(context.Background(), -2001, 777, telegramTextMessage{Text: "structured"}, &inlineKeyboardMarkup{
		InlineKeyboard: [][]inlineKeyboardButton{{{Text: "Open", URL: "https://example.test"}}},
	}); err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if err := client.deleteWebhook(context.Background()); err != nil {
		t.Fatalf("deleteWebhook error: %v", err)
	}
	updates, err := client.getUpdates(context.Background(), 9)
	if err != nil {
		t.Fatalf("getUpdates error: %v", err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 1 {
		t.Fatalf("updates = %+v", updates)
	}

	if len(requests) != 6 {
		t.Fatalf("requests = %+v", requests)
	}
	if requests[0].path != "/bot-token/sendMessage" || requests[0].body["chat_id"] != float64(-2001) || requests[0].body["message_thread_id"] != float64(777) {
		t.Fatalf("send text request = %+v", requests[0])
	}
	replyParameters, ok := requests[0].body["reply_parameters"].(map[string]any)
	if !ok || replyParameters["message_id"] != float64(42) {
		t.Fatalf("reply parameters = %+v", requests[0].body["reply_parameters"])
	}
	if requests[1].path != "/bot-token/editMessageText" || requests[1].body["message_id"] != float64(42) {
		t.Fatalf("edit text request = %+v", requests[1])
	}
	if requests[2].path != "/bot-token/answerCallbackQuery" || requests[2].body["callback_query_id"] != "cb-1" {
		t.Fatalf("answer callback request = %+v", requests[2])
	}
	if requests[3].path != "/bot-token/sendMessage" || requests[3].body["reply_markup"] == nil {
		t.Fatalf("structured request = %+v", requests[3])
	}
	if requests[4].path != "/bot-token/deleteWebhook" {
		t.Fatalf("delete webhook request = %+v", requests[4])
	}
	if requests[5].path != "/bot-token/getUpdates" || requests[5].body["offset"] != float64(9) {
		t.Fatalf("get updates request = %+v", requests[5])
	}

	if err := client.sendMessage(context.Background(), sendMessageRequest{}); err == nil || !strings.Contains(err.Error(), "requires content") {
		t.Fatalf("sendMessage empty err = %v", err)
	}
	if err := client.editMessageText(context.Background(), editMessageTextRequest{}); err == nil || !strings.Contains(err.Error(), "requires content") {
		t.Fatalf("editMessageText empty err = %v", err)
	}
	if err := client.answerCallbackQuery(context.Background(), answerCallbackQueryRequest{}); err == nil || !strings.Contains(err.Error(), "callback query id") {
		t.Fatalf("answerCallbackQuery empty err = %v", err)
	}

	errorClient := &botAPIClient{
		baseURL: "https://api.telegram.example/bot-token",
		client: &http.Client{Transport: telegramRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("upstream boom")),
				Header:     make(http.Header),
			}, nil
		})},
	}
	if err := errorClient.deleteWebhook(context.Background()); err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("deleteWebhook upstream err = %v", err)
	}

	notOKClient := &botAPIClient{
		baseURL: "https://api.telegram.example/bot-token",
		client: &http.Client{Transport: telegramRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":false,"description":"denied"}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	if _, err := notOKClient.getUpdates(context.Background(), 0); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("getUpdates not-ok err = %v", err)
	}
}

func TestLive_SendCardRendersMarkdownV2WithInlineKeyboard(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Build #42").
		AddField("Status", "success").
		AddField("Branch", "main").
		AddPrimaryButton("Approve", "act:approve:build-42").
		AddButton("View", "link:https://example.test/builds/42")

	if err := live.SendCard(context.Background(), "-2001", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}

	if len(sender.structuredCalls) != 1 {
		t.Fatalf("structuredCalls = %+v", sender.structuredCalls)
	}
	call := sender.structuredCalls[0]
	if call.ChatID != -2001 {
		t.Fatalf("ChatID = %d", call.ChatID)
	}
	if call.ParseMode != "MarkdownV2" {
		t.Fatalf("ParseMode = %q", call.ParseMode)
	}
	if !strings.Contains(call.Text, "Build \\#42") {
		t.Fatalf("text = %q, expected escaped title", call.Text)
	}
	if !strings.Contains(call.Text, "*Status:*") {
		t.Fatalf("text = %q, expected bold label", call.Text)
	}
	if call.Markup == nil || len(call.Markup.InlineKeyboard) != 2 {
		t.Fatalf("markup = %+v", call.Markup)
	}
	if call.Markup.InlineKeyboard[0][0].CallbackData != "act:approve:build-42" {
		t.Fatalf("first button = %+v", call.Markup.InlineKeyboard[0][0])
	}
	if call.Markup.InlineKeyboard[1][0].URL != "https://example.test/builds/42" {
		t.Fatalf("second button = %+v", call.Markup.InlineKeyboard[1][0])
	}
}

func TestLive_ReplyCardUsesReplyContextAndRejectsMissingChat(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Task Assigned").
		AddField("Assignee", "alice").
		AddPrimaryButton("Accept", "act:accept:task-1")

	if err := live.ReplyCard(context.Background(), replyContext{ChatID: -2001, TopicID: 777}, card); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}

	if len(sender.structuredCalls) != 1 {
		t.Fatalf("structuredCalls = %+v", sender.structuredCalls)
	}
	call := sender.structuredCalls[0]
	if call.ChatID != -2001 || call.TopicID != 777 {
		t.Fatalf("call = %+v", call)
	}
	if call.ParseMode != "MarkdownV2" {
		t.Fatalf("ParseMode = %q", call.ParseMode)
	}
	if call.Markup == nil || len(call.Markup.InlineKeyboard) != 1 {
		t.Fatalf("markup = %+v", call.Markup)
	}

	if err := live.ReplyCard(context.Background(), replyContext{}, card); err == nil || !strings.Contains(err.Error(), "requires chat id") {
		t.Fatalf("missing chat error = %v", err)
	}
}

func TestLive_StartTypingSendsChatActionAndStopTypingIsNoop(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.StartTyping(context.Background(), "-2001"); err != nil {
		t.Fatalf("StartTyping error: %v", err)
	}

	if len(sender.chatActions) != 1 {
		t.Fatalf("chatActions = %+v", sender.chatActions)
	}
	if sender.chatActions[0].ChatID != -2001 || sender.chatActions[0].Action != "typing" {
		t.Fatalf("chatAction = %+v", sender.chatActions[0])
	}

	// Non-numeric chatID is silently ignored
	if err := live.StartTyping(context.Background(), "not-a-number"); err != nil {
		t.Fatalf("StartTyping non-numeric error: %v", err)
	}
	if len(sender.chatActions) != 1 {
		t.Fatalf("expected non-numeric chatID to be ignored, chatActions = %+v", sender.chatActions)
	}

	// Empty chatID is silently ignored
	if err := live.StartTyping(context.Background(), ""); err != nil {
		t.Fatalf("StartTyping empty error: %v", err)
	}

	// StopTyping is always a no-op
	if err := live.StopTyping(context.Background(), "-2001"); err != nil {
		t.Fatalf("StopTyping error: %v", err)
	}
}

func TestLive_SendCardWithNilCardSendsEmptyStructured(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendCard(context.Background(), "-2001", nil); err != nil {
		t.Fatalf("SendCard nil error: %v", err)
	}
}

func TestBotAPISender_SendChatActionCallsAPI(t *testing.T) {
	var requestPath string
	var requestBody map[string]any
	client := &botAPIClient{
		baseURL: "https://api.telegram.example/bot-token",
		client: &http.Client{Transport: telegramRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requestPath = req.URL.Path
			if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":true}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	sender := &botAPISender{client: client}

	if err := sender.SendChatAction(context.Background(), -2001, "typing"); err != nil {
		t.Fatalf("SendChatAction error: %v", err)
	}
	if requestPath != "/bot-token/sendChatAction" {
		t.Fatalf("path = %q", requestPath)
	}
	if requestBody["chat_id"] != float64(-2001) || requestBody["action"] != "typing" {
		t.Fatalf("body = %+v", requestBody)
	}

	if err := client.sendChatAction(context.Background(), sendChatActionRequest{}); err == nil || !strings.Contains(err.Error(), "requires action") {
		t.Fatalf("sendChatAction empty err = %v", err)
	}
}
