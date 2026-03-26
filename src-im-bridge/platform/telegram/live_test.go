package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
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

type fakeSender struct {
	calls           []sendCall
	edits           []editCall
	callbackAnswers []callbackAnswerCall
	structuredCalls []structuredCall
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

type fakeTelegramActionHandler struct {
	requests []*notify.ActionRequest
}

func (h *fakeTelegramActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	h.requests = append(h.requests, req)
	return &notify.ActionResponse{Result: "Approved"}, nil
}
