package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLive_StartNormalizesInboundMessageFromLongConnection(t *testing.T) {
	runner := &fakeEventRunner{}
	sender := &fakeMessageClient{}

	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var gotPlatform core.Platform
	var got *core.Message
	if err := live.Start(func(p core.Platform, msg *core.Message) {
		gotPlatform = p
		got = msg
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	if !runner.started {
		t.Fatal("expected event runner to start")
	}

	err = runner.dispatch(context.Background(), newMessageReceiveEvent(
		"msg-1",
		"chat-1",
		"group",
		`{"text":"@_user_1 hello"}`,
		"1700000000123",
		"ou_user_1",
		[]*larkim.MentionEvent{
			newMentionEvent("@_user_1", "AgentForge"),
		},
	))
	if err != nil {
		t.Fatalf("dispatch error: %v", err)
	}

	if gotPlatform != live {
		t.Fatalf("platform = %#v, want live platform", gotPlatform)
	}
	if got == nil {
		t.Fatal("expected normalized message")
	}
	if got.Platform != "feishu" {
		t.Fatalf("Platform = %q, want feishu", got.Platform)
	}
	if got.SessionKey != "feishu:chat-1:ou_user_1" {
		t.Fatalf("SessionKey = %q", got.SessionKey)
	}
	if got.Content != "@AgentForge hello" {
		t.Fatalf("Content = %q, want @AgentForge hello", got.Content)
	}
	replyCtx, ok := got.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T, want replyContext", got.ReplyCtx)
	}
	if replyCtx.MessageID != "msg-1" || replyCtx.ChatID != "chat-1" {
		t.Fatalf("ReplyCtx = %+v", replyCtx)
	}
	if got.ReplyTarget == nil || got.ReplyTarget.ChatID != "chat-1" || got.ReplyTarget.MessageID != "msg-1" {
		t.Fatalf("ReplyTarget = %+v", got.ReplyTarget)
	}
	if !got.IsGroup {
		t.Fatal("expected group message")
	}
}

func TestLive_ReplySendAndCardFallbackUseFeishuMessageAPI(t *testing.T) {
	runner := &fakeEventRunner{}
	sender := &fakeMessageClient{}

	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{MessageID: "msg-1", ChatID: "chat-1"}
	if err := live.Reply(context.Background(), replyCtx, "hello"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if err := live.Send(context.Background(), "chat-2", "broadcast"); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	card := core.NewCard().
		SetTitle("Task Update").
		AddField("Status", "Done").
		AddPrimaryButton("Open", "link:https://example.test/task/1")
	if err := live.SendCard(context.Background(), "chat-3", card); err != nil {
		t.Fatalf("SendCard error: %v", err)
	}
	if err := live.ReplyCard(context.Background(), replyCtx, core.NewCard().SetTitle("Reply Card")); err != nil {
		t.Fatalf("ReplyCard error: %v", err)
	}

	if len(sender.replyCalls) != 2 {
		t.Fatalf("replyCalls = %+v", sender.replyCalls)
	}
	if sender.replyCalls[0].MessageID != "msg-1" || sender.replyCalls[0].MsgType != larkim.MsgTypeText {
		t.Fatalf("first reply call = %+v", sender.replyCalls[0])
	}

	replyText := decodeJSONMap(t, sender.replyCalls[0].Content)
	if replyText["text"] != "hello" {
		t.Fatalf("reply text payload = %+v", replyText)
	}

	if len(sender.sendCalls) != 2 {
		t.Fatalf("sendCalls = %+v", sender.sendCalls)
	}
	if sender.sendCalls[0].ReceiveID != "chat-2" || sender.sendCalls[0].ReceiveIDType != larkim.ReceiveIdTypeChatId {
		t.Fatalf("first send call = %+v", sender.sendCalls[0])
	}
	sendText := decodeJSONMap(t, sender.sendCalls[0].Content)
	if sendText["text"] != "broadcast" {
		t.Fatalf("send text payload = %+v", sendText)
	}

	if sender.sendCalls[1].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("interactive send call = %+v", sender.sendCalls[1])
	}
	if sender.replyCalls[1].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("interactive reply call = %+v", sender.replyCalls[1])
	}

	sendCardPayload := decodeJSONMap(t, sender.sendCalls[1].Content)
	if sendCardPayload["config"] == nil || sendCardPayload["header"] == nil {
		t.Fatalf("interactive payload = %+v", sendCardPayload)
	}
}

func TestLive_HTTPCallbackHandlerRequiresExplicitRegistration(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	if live.HTTPCallbackHandler() != nil {
		t.Fatal("expected no HTTP callback handler by default")
	}

	live, err = NewLive(
		"app-id",
		"app-secret",
		WithEventRunner(&fakeEventRunner{}),
		WithMessageClient(&fakeMessageClient{}),
		WithLegacyCardCallbackHandler("verification-token", "encrypt-key", func(ctx context.Context, event *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error) {
			return &larkcallback.CardActionTriggerResponse{}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewLive with callback handler error: %v", err)
	}
	if live.HTTPCallbackHandler() == nil {
		t.Fatal("expected explicit HTTP callback handler")
	}
}

func TestLive_CardCallbackWebhookExposesHandlerAndPath(t *testing.T) {
	live, err := NewLive(
		"app-id",
		"app-secret",
		WithEventRunner(&fakeEventRunner{}),
		WithMessageClient(&fakeMessageClient{}),
		WithCardCallbackWebhook("verification-token", "encrypt-key", "/feishu/callback"),
	)
	if err != nil {
		t.Fatalf("NewLive with webhook callback error: %v", err)
	}
	if live.HTTPCallbackHandler() == nil {
		t.Fatal("expected webhook callback handler")
	}
	if !reflect.DeepEqual(live.CallbackPaths(), []string{"/feishu/callback"}) {
		t.Fatalf("CallbackPaths = %+v", live.CallbackPaths())
	}
	if !live.Metadata().Capabilities.RequiresPublicCallback {
		t.Fatal("expected webhook-enabled Feishu live transport to require public callback")
	}
}

func TestLive_MetadataUsesSocketPayloadCallbackModeWithoutWebhook(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Capabilities.ActionCallbackMode != core.ActionCallbackSocketPayload {
		t.Fatalf("ActionCallbackMode = %q, want %q", metadata.Capabilities.ActionCallbackMode, core.ActionCallbackSocketPayload)
	}
	if metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected long-connection callback mode to avoid requiring a public callback")
	}
}

func TestLive_StartRoutesCardActionCallbackToActionHandler(t *testing.T) {
	runner := &fakeEventRunner{}
	sender := &fakeMessageClient{}
	actions := &fakeFeishuActionHandler{}
	updater := &fakeCardUpdater{}

	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(sender), WithCardUpdater(updater))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.SetActionHandler(actions)

	if err := live.Start(func(p core.Platform, msg *core.Message) {
		t.Fatalf("message handler should not receive card action callbacks: %+v", msg)
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer live.Stop()

	resp, err := runner.dispatchCardAction(context.Background(), &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Token: "card-token-1",
			Operator: &larkcallback.Operator{
				OpenID: "ou_123",
			},
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"action": "act:approve:review-1",
				},
			},
			Context: &larkcallback.Context{
				OpenMessageID: "om_123",
				OpenChatID:    "oc_456",
			},
		},
	})
	if err != nil {
		t.Fatalf("dispatchCardAction error: %v", err)
	}

	if len(actions.requests) != 1 {
		t.Fatalf("requests = %+v, want 1 request", actions.requests)
	}
	req := actions.requests[0]
	if req.Platform != "feishu" {
		t.Fatalf("Platform = %q, want feishu", req.Platform)
	}
	if req.Action != "approve" || req.EntityID != "review-1" {
		t.Fatalf("action request = %+v", req)
	}
	if req.ChatID != "oc_456" || req.UserID != "ou_123" {
		t.Fatalf("chat/user = %+v", req)
	}
	if req.ReplyTarget == nil {
		t.Fatal("expected reply target")
	}
	if req.ReplyTarget.MessageID != "om_123" || req.ReplyTarget.ChatID != "oc_456" {
		t.Fatalf("ReplyTarget = %+v", req.ReplyTarget)
	}
	if req.ReplyTarget.CallbackToken != "card-token-1" {
		t.Fatalf("CallbackToken = %q", req.ReplyTarget.CallbackToken)
	}
	if req.ReplyTarget.PreferredRenderer != "cards" || req.ReplyTarget.ProgressMode != "deferred_card_update" {
		t.Fatalf("ReplyTarget = %+v", req.ReplyTarget)
	}
	if req.Metadata["source"] != "card.action.trigger" {
		t.Fatalf("Metadata = %+v", req.Metadata)
	}
	if resp == nil || resp.Toast == nil || resp.Toast.Content != "Approved" {
		t.Fatalf("callback response = %+v", resp)
	}
	if updater.callbackToken != "" || updater.message != nil {
		t.Fatalf("delayed update should not run during synchronous callback ack, updater = %+v", updater)
	}
}

func TestLive_HandleCardActionReturnsTemplateCardResponseWhenActionHandlerProvidesNativeCard(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.SetActionHandler(&replyingFeishuActionHandler{
		response: &notify.ActionResponse{
			Result: "Updated",
			Native: &core.NativeMessage{
				Platform: "feishu",
				FeishuCard: &core.FeishuCardPayload{
					Mode:                core.FeishuCardModeTemplate,
					TemplateID:          "ctp_123",
					TemplateVersionName: "1.0.0",
					TemplateVariable: map[string]any{
						"title": "Done",
					},
				},
			},
		},
	})

	resp, err := live.handleCardAction(context.Background(), &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Token: "cb-token-1",
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"action": "act:approve:review-1",
				},
			},
			Context: &larkcallback.Context{
				OpenMessageID: "om_123",
				OpenChatID:    "oc_456",
			},
			Operator: &larkcallback.Operator{OpenID: "ou_123"},
		},
	})
	if err != nil {
		t.Fatalf("handleCardAction error: %v", err)
	}
	if resp == nil || resp.Card == nil {
		t.Fatalf("response = %+v, want template card response", resp)
	}
	if resp.Card.Type != "template" {
		t.Fatalf("card type = %q, want template", resp.Card.Type)
	}
	data, ok := resp.Card.Data.(map[string]any)
	if !ok {
		t.Fatalf("card data = %#v", resp.Card.Data)
	}
	if data["template_id"] != "ctp_123" {
		t.Fatalf("card data = %+v", data)
	}
	if resp.Toast == nil || resp.Toast.Content != "Updated" {
		t.Fatalf("toast = %+v", resp.Toast)
	}
}

func TestLive_HandleCardActionReturnsRawCardResponseWhenActionHandlerProvidesStructuredMessage(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.SetActionHandler(&replyingFeishuActionHandler{
		response: &notify.ActionResponse{
			Result: "Updated",
			Structured: &core.StructuredMessage{
				Title: "AgentForge IM 助手",
				Sections: []core.StructuredSection{
					{Type: core.StructuredSectionTypeText, TextSection: &core.TextSection{Body: "同步更新完成"}},
				},
			},
		},
	})

	resp, err := live.handleCardAction(context.Background(), &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Token: "cb-token-1",
			Action: &larkcallback.CallBackAction{
				Value: map[string]interface{}{
					"action": "act:approve:review-1",
				},
			},
			Context: &larkcallback.Context{
				OpenMessageID: "om_123",
				OpenChatID:    "oc_456",
			},
			Operator: &larkcallback.Operator{OpenID: "ou_123"},
		},
	})
	if err != nil {
		t.Fatalf("handleCardAction error: %v", err)
	}
	if resp == nil || resp.Card == nil {
		t.Fatalf("response = %+v, want raw card response", resp)
	}
	if resp.Card.Type != "raw" {
		t.Fatalf("card type = %q, want raw", resp.Card.Type)
	}
	data, ok := resp.Card.Data.(map[string]any)
	if !ok {
		t.Fatalf("card data = %#v", resp.Card.Data)
	}
	if data["header"] == nil || data["elements"] == nil {
		t.Fatalf("card data = %+v, want raw card payload", data)
	}
}

func TestLive_StopReturnsRunnerError(t *testing.T) {
	stopErr := errors.New("stop failed")
	runner := &fakeEventRunner{stopErr: stopErr}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(&fakeMessageClient{}))
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

func TestLive_MetadataDeclaresFeishuCapabilities(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "feishu" {
		t.Fatalf("Source = %q, want feishu", metadata.Source)
	}
	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected rich-message capability")
	}
	if metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected long-connection mode to avoid requiring a public callback by default")
	}
}

func TestLive_SendReplyAndUpdateNativePayloads(t *testing.T) {
	runner := &fakeEventRunner{}
	sender := &fakeMessageClient{}
	updater := &fakeCardUpdater{}

	live, err := NewLive(
		"app-id",
		"app-secret",
		WithEventRunner(runner),
		WithMessageClient(sender),
		WithCardUpdater(updater),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	jsonMessage := &core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode: core.FeishuCardModeJSON,
			JSON: json.RawMessage(`{"header":{"title":{"tag":"plain_text","content":"Native"}}}`),
		},
	}
	if err := live.SendNative(context.Background(), "chat-1", jsonMessage); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}

	templateMessage := &core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:                core.FeishuCardModeTemplate,
			TemplateID:          "ctp_123",
			TemplateVersionName: "1.0.0",
			TemplateVariable: map[string]any{
				"title": "Hello",
			},
		},
	}
	if err := live.ReplyNative(context.Background(), replyContext{MessageID: "msg-1", ChatID: "chat-1"}, templateMessage); err != nil {
		t.Fatalf("ReplyNative error: %v", err)
	}
	if err := live.UpdateNative(context.Background(), replyContext{ChatID: "chat-1", CallbackToken: "cb-token-1"}, templateMessage); err != nil {
		t.Fatalf("UpdateNative error: %v", err)
	}

	if len(sender.sendCalls) != 1 || sender.sendCalls[0].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("sendCalls = %+v", sender.sendCalls)
	}
	sendPayload := decodeJSONMap(t, sender.sendCalls[0].Content)
	if sendPayload["header"] == nil {
		t.Fatalf("send payload = %+v", sendPayload)
	}

	if len(sender.replyCalls) != 1 || sender.replyCalls[0].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("replyCalls = %+v", sender.replyCalls)
	}
	replyPayload := decodeJSONMap(t, sender.replyCalls[0].Content)
	if replyPayload["type"] != "template" {
		t.Fatalf("reply payload = %+v", replyPayload)
	}
	if updater.callbackToken != "cb-token-1" || updater.message == nil || updater.message.FeishuCard == nil || updater.message.FeishuCard.TemplateID != "ctp_123" {
		t.Fatalf("updater = %+v", updater)
	}
}

func TestLive_BuildNativeTextMessageReturnsMarkdownCard(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	message, err := live.BuildNativeTextMessage("AgentForge Update", "hello **world**")
	if err != nil {
		t.Fatalf("BuildNativeTextMessage error: %v", err)
	}
	if message == nil || message.FeishuCard == nil || message.FeishuCard.Mode != core.FeishuCardModeJSON {
		t.Fatalf("message = %+v", message)
	}
}

type fakeEventRunner struct {
	started                bool
	stopped                bool
	handler                func(context.Context, *larkim.P2MessageReceiveV1) error
	cardActionHandler      func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error)
	reactionDeletedHandler func(context.Context, *larkim.P2MessageReactionDeletedV1) error
	stopErr                error
}

func (r *fakeEventRunner) Start(ctx context.Context, handler func(context.Context, *larkim.P2MessageReceiveV1) error) error {
	r.started = true
	r.handler = handler
	return nil
}

func (r *fakeEventRunner) Stop(context.Context) error {
	r.stopped = true
	return r.stopErr
}

func (r *fakeEventRunner) dispatch(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if r.handler == nil {
		return errors.New("handler not registered")
	}
	return r.handler(ctx, event)
}

func (r *fakeEventRunner) StartWithCardActions(ctx context.Context, handler func(context.Context, *larkim.P2MessageReceiveV1) error, cardActionHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error)) error {
	r.started = true
	r.handler = handler
	r.cardActionHandler = cardActionHandler
	return nil
}

func (r *fakeEventRunner) StartFull(
	ctx context.Context,
	handler func(context.Context, *larkim.P2MessageReceiveV1) error,
	cardActionHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error),
	botAddedHandler func(context.Context, *larkim.P2ChatMemberBotAddedV1) error,
	botRemovedHandler func(context.Context, *larkim.P2ChatMemberBotDeletedV1) error,
	reactionHandler func(context.Context, *larkim.P2MessageReactionCreatedV1) error,
	reactionDeletedHandler func(context.Context, *larkim.P2MessageReactionDeletedV1) error,
) error {
	r.started = true
	r.handler = handler
	r.cardActionHandler = cardActionHandler
	r.reactionDeletedHandler = reactionDeletedHandler
	return nil
}

func (r *fakeEventRunner) dispatchCardAction(ctx context.Context, event *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error) {
	if r.cardActionHandler == nil {
		return nil, errors.New("card action handler not registered")
	}
	return r.cardActionHandler(ctx, event)
}

func (r *fakeEventRunner) dispatchReactionDeleted(ctx context.Context, event *larkim.P2MessageReactionDeletedV1) error {
	if r.reactionDeletedHandler == nil {
		return errors.New("reaction deleted handler not registered")
	}
	return r.reactionDeletedHandler(ctx, event)
}

type fakeMessageClient struct {
	sendCalls  []fakeSendCall
	replyCalls []fakeReplyCall
	patchCalls []fakePatchCall
	patchErr   error
}

type fakePatchCall struct {
	MessageID string
	Content   string
}

type fakeCardUpdater struct {
	callbackToken string
	message       *core.NativeMessage
}

type fakeSendCall struct {
	ReceiveIDType string
	ReceiveID     string
	MsgType       string
	Content       string
}

type fakeReplyCall struct {
	MessageID string
	MsgType   string
	Content   string
}

func (c *fakeMessageClient) Send(ctx context.Context, receiveIDType, receiveID, msgType, content string) error {
	c.sendCalls = append(c.sendCalls, fakeSendCall{
		ReceiveIDType: receiveIDType,
		ReceiveID:     receiveID,
		MsgType:       msgType,
		Content:       content,
	})
	return nil
}

func (c *fakeMessageClient) Reply(ctx context.Context, messageID, msgType, content string) error {
	c.replyCalls = append(c.replyCalls, fakeReplyCall{
		MessageID: messageID,
		MsgType:   msgType,
		Content:   content,
	})
	return nil
}

func (c *fakeMessageClient) Patch(ctx context.Context, messageID, content string) error {
	c.patchCalls = append(c.patchCalls, fakePatchCall{
		MessageID: messageID,
		Content:   content,
	})
	return c.patchErr
}

func (u *fakeCardUpdater) Update(ctx context.Context, callbackToken string, message *core.NativeMessage) error {
	u.callbackToken = callbackToken
	u.message = message
	return nil
}

func newMessageReceiveEvent(messageID, chatID, chatType, content, createTime, senderOpenID string, mentions []*larkim.MentionEvent) *larkim.P2MessageReceiveV1 {
	return newMessageReceiveEventWithType(messageID, chatID, chatType, larkim.MsgTypeText, content, createTime, senderOpenID, mentions)
}

func newMessageReceiveEventWithType(messageID, chatID, chatType, msgType, content, createTime, senderOpenID string, mentions []*larkim.MentionEvent) *larkim.P2MessageReceiveV1 {
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: stringPtr(senderOpenID),
				},
			},
			Message: &larkim.EventMessage{
				MessageId:   stringPtr(messageID),
				ChatId:      stringPtr(chatID),
				ChatType:    stringPtr(chatType),
				MessageType: stringPtr(msgType),
				Content:     stringPtr(content),
				CreateTime:  stringPtr(createTime),
				Mentions:    mentions,
			},
		},
	}
}

func newMentionEvent(key, name string) *larkim.MentionEvent {
	return &larkim.MentionEvent{
		Key:  stringPtr(key),
		Name: stringPtr(name),
	}
}

func decodeJSONMap(t *testing.T, raw string) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal JSON payload %q: %v", raw, err)
	}
	return decoded
}

func stringPtr(value string) *string { return &value }

var _ http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

type fakeFeishuActionHandler struct {
	requests []*notify.ActionRequest
}

func (h *fakeFeishuActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	h.requests = append(h.requests, req)
	return &notify.ActionResponse{Result: "Approved"}, nil
}

type replyingFeishuActionHandler struct {
	response *notify.ActionResponse
}

func (h *replyingFeishuActionHandler) HandleAction(context.Context, *notify.ActionRequest) (*notify.ActionResponse, error) {
	return h.response, nil
}

func TestFeishuLive_NameReplyContextAndHelperFunctions(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "feishu-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		MessageID:     " msg-1 ",
		ChannelID:     "chat-1",
		CallbackToken: " card-token ",
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.MessageID != "msg-1" || reply.ChatID != "chat-1" || reply.CallbackToken != "card-token" {
		t.Fatalf("reply = %+v", reply)
	}

	rawCtx := replyContext{MessageID: "msg-2", ChatID: "chat-2", CallbackToken: "cb-token-2"}
	if got := toReplyContext(rawCtx); got != rawCtx {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ChatID: "chat-3"}); got.ChatID != "chat-3" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	if got := toReplyContext(&core.Message{ChatID: "chat-4"}); got.ChatID != "chat-4" {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}
	if got := toReplyContext(&core.ReplyTarget{MessageID: "msg-5", ChatID: "chat-5", CallbackToken: "cb-token-5"}); got.MessageID != "msg-5" || got.ChatID != "chat-5" || got.CallbackToken != "cb-token-5" {
		t.Fatalf("toReplyContext(target) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}

	openID := "ou_123"
	userID := "user_123"
	unionID := "union_123"
	if got := senderID(&larkim.EventSender{SenderId: &larkim.UserId{OpenId: &openID}}); got != "ou_123" {
		t.Fatalf("senderID(open) = %q", got)
	}
	if got := senderID(&larkim.EventSender{SenderId: &larkim.UserId{UserId: &userID}}); got != "user_123" {
		t.Fatalf("senderID(user) = %q", got)
	}
	if got := senderID(&larkim.EventSender{SenderId: &larkim.UserId{UnionId: &unionID}}); got != "union_123" {
		t.Fatalf("senderID(union) = %q", got)
	}
	if got := senderID(nil); got != "" {
		t.Fatalf("senderID(nil) = %q", got)
	}

	if got := parseUnixMillis("1700000000123"); !got.Equal(time.Unix(0, 0).Add(1700000000123 * time.Millisecond)) {
		t.Fatalf("parseUnixMillis(valid) = %v", got)
	}
	before := time.Now()
	got := parseUnixMillis("bad")
	after := time.Now()
	if got.Before(before) || got.After(after.Add(time.Second)) {
		t.Fatalf("parseUnixMillis(invalid) = %v", got)
	}

	if got := normalizeButtonStyle(" primary "); got != "primary" {
		t.Fatalf("normalizeButtonStyle(primary) = %q", got)
	}
	if got := normalizeButtonStyle("other"); got != "default" {
		t.Fatalf("normalizeButtonStyle(other) = %q", got)
	}
	if got := feishuActionReference(map[string]interface{}{"action": " act:approve:review-1 "}); got != "act:approve:review-1" {
		t.Fatalf("feishuActionReference(action) = %q", got)
	}
	if got := feishuActionReference(map[string]interface{}{"action_id": " act:reject:review-1 "}); got != "act:reject:review-1" {
		t.Fatalf("feishuActionReference(action_id) = %q", got)
	}
	if got := feishuActionReference(nil); got != "" {
		t.Fatalf("feishuActionReference(nil) = %q", got)
	}
	operatorUserID := "user_456"
	if got := feishuOperatorID(&larkcallback.Operator{OpenID: "ou_456"}); got != "ou_456" {
		t.Fatalf("feishuOperatorID(open) = %q", got)
	}
	if got := feishuOperatorID(&larkcallback.Operator{UserID: &operatorUserID}); got != "user_456" {
		t.Fatalf("feishuOperatorID(user) = %q", got)
	}
	if got := feishuOperatorID(nil); got != "" {
		t.Fatalf("feishuOperatorID(nil) = %q", got)
	}
	if got := value(nil); got != "" {
		t.Fatalf("value(nil) = %q", got)
	}
	if got := value(&openID); got != "ou_123" {
		t.Fatalf("value(ptr) = %q", got)
	}
	if got := firstNonEmpty(" ", "chat-6", "chat-7"); got != "chat-6" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
	if got := compactMetadata(map[string]string{" source ": " card.action.trigger ", "empty": " "}); got["source"] != "card.action.trigger" || len(got) != 1 {
		t.Fatalf("compactMetadata = %+v", got)
	}
}

func TestLive_SendFormattedTextLarkMDUsesNativePath(t *testing.T) {
	sender := &fakeMessageClient{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendFormattedText(context.Background(), "chat-1", &core.FormattedText{
		Content: "hello **world**",
		Format:  core.TextFormatLarkMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}

	if len(sender.sendCalls) != 1 {
		t.Fatalf("sendCalls = %d, want 1", len(sender.sendCalls))
	}
	if sender.sendCalls[0].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("MsgType = %q, want interactive", sender.sendCalls[0].MsgType)
	}
}

func TestLive_SendFormattedTextPlainFallsBackToSend(t *testing.T) {
	sender := &fakeMessageClient{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendFormattedText(context.Background(), "chat-1", &core.FormattedText{
		Content: "plain text",
		Format:  core.TextFormatPlainText,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}

	if len(sender.sendCalls) != 1 {
		t.Fatalf("sendCalls = %d, want 1", len(sender.sendCalls))
	}
	if sender.sendCalls[0].MsgType != larkim.MsgTypeText {
		t.Fatalf("MsgType = %q, want text", sender.sendCalls[0].MsgType)
	}
}

func TestLive_SendFormattedTextNilMessageReturnsError(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.SendFormattedText(context.Background(), "chat-1", nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("SendFormattedText(nil) err = %v", err)
	}
}

func TestLive_ReplyFormattedTextLarkMDUsesNativePath(t *testing.T) {
	sender := &fakeMessageClient{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	rc := replyContext{MessageID: "msg-1", ChatID: "chat-1"}
	if err := live.ReplyFormattedText(context.Background(), rc, &core.FormattedText{
		Content: "hello **world**",
		Format:  core.TextFormatLarkMD,
	}); err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}

	if len(sender.replyCalls) != 1 {
		t.Fatalf("replyCalls = %d, want 1", len(sender.replyCalls))
	}
	if sender.replyCalls[0].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("MsgType = %q, want interactive", sender.replyCalls[0].MsgType)
	}
}

func TestLive_ReplyFormattedTextNilMessageReturnsError(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.ReplyFormattedText(context.Background(), replyContext{MessageID: "msg-1"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("ReplyFormattedText(nil) err = %v", err)
	}
}

func TestLive_UpdateFormattedTextNilMessageReturnsError(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateFormattedText(context.Background(), replyContext{MessageID: "msg-1"}, nil); err == nil || !strings.Contains(err.Error(), "formatted text is required") {
		t.Fatalf("UpdateFormattedText(nil) err = %v", err)
	}
}

func TestLive_UpdateFormattedTextFallsBackToReplyWhenNoCallbackToken(t *testing.T) {
	sender := &fakeMessageClient{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(sender), WithCardUpdater(&fakeCardUpdater{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	rc := replyContext{MessageID: "msg-1", ChatID: "chat-1"}
	if err := live.UpdateFormattedText(context.Background(), rc, &core.FormattedText{
		Content: "updated **content**",
		Format:  core.TextFormatLarkMD,
	}); err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}

	// Without callback token, UpdateNative fails, falls back to ReplyFormattedText -> ReplyNative
	if len(sender.replyCalls) != 1 {
		t.Fatalf("replyCalls = %d, want 1", len(sender.replyCalls))
	}
	if sender.replyCalls[0].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("MsgType = %q, want interactive", sender.replyCalls[0].MsgType)
	}
}

func TestLive_UpdateMessagePatchesExistingMessage(t *testing.T) {
	sender := &fakeMessageClient{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	rc := replyContext{MessageID: "msg-1", ChatID: "chat-1"}
	if err := live.UpdateMessage(context.Background(), rc, "updated text"); err != nil {
		t.Fatalf("UpdateMessage error: %v", err)
	}

	if len(sender.patchCalls) != 1 {
		t.Fatalf("patchCalls = %d, want 1", len(sender.patchCalls))
	}
	if sender.patchCalls[0].MessageID != "msg-1" {
		t.Fatalf("patch MessageID = %q, want msg-1", sender.patchCalls[0].MessageID)
	}
	patchPayload := decodeJSONMap(t, sender.patchCalls[0].Content)
	if patchPayload["text"] != "updated text" {
		t.Fatalf("patch content = %+v", patchPayload)
	}
}

func TestLive_UpdateMessageRequiresMessageID(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.UpdateMessage(context.Background(), replyContext{ChatID: "chat-1"}, "text"); err == nil || !strings.Contains(err.Error(), "requires message id") {
		t.Fatalf("UpdateMessage(no message id) err = %v", err)
	}
}

func TestLive_MetadataDeclaresAsyncUpdateEdit(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if !metadata.Capabilities.HasAsyncUpdateMode(core.AsyncUpdateEdit) {
		t.Fatal("expected AsyncUpdateEdit in capabilities")
	}
}

func TestFeishuLive_DecodeTextMessageAndReplyCardErrors(t *testing.T) {
	if _, err := decodeTextMessage(nil, nil); err == nil || !strings.Contains(err.Error(), "missing feishu text message content") {
		t.Fatalf("decodeTextMessage(nil) err = %v", err)
	}
	invalid := "{"
	if _, err := decodeTextMessage(&invalid, nil); err == nil || !strings.Contains(err.Error(), "decode feishu text payload") {
		t.Fatalf("decodeTextMessage(invalid) err = %v", err)
	}

	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	if err := live.ReplyCard(context.Background(), replyContext{}, core.NewCard().SetTitle("missing target")); err == nil || !strings.Contains(err.Error(), "requires message id or chat id") {
		t.Fatalf("ReplyCard missing target err = %v", err)
	}
}

// --- Post message tests ---

func TestLive_NormalizePostMessage(t *testing.T) {
	runner := &fakeEventRunner{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(runner), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	var got *core.Message
	live.Start(func(_ core.Platform, msg *core.Message) { got = msg })
	defer live.Stop()

	postContent := `{"title":"Update","content":[[{"tag":"text","text":"Hello "},{"tag":"a","text":"link","href":"https://example.com"}],[{"tag":"text","text":"second line"}]]}`
	runner.dispatch(context.Background(), newMessageReceiveEventWithType(
		"msg-post", "chat-1", "group", "post", postContent, "1700000000000", "ou_user_1", nil,
	))

	if got == nil {
		t.Fatal("expected normalized message")
	}
	if got.MessageType != "post" {
		t.Fatalf("MessageType = %q, want post", got.MessageType)
	}
	if !strings.Contains(got.Content, "Hello") {
		t.Fatalf("Content = %q, want to contain Hello", got.Content)
	}
	if !strings.Contains(got.Content, "https://example.com") {
		t.Fatalf("Content = %q, want to contain URL", got.Content)
	}
}

func TestDecodePostMessage_TitleAndParagraphs(t *testing.T) {
	raw := `{"title":"Update","content":[[{"tag":"text","text":"Hello "},{"tag":"a","text":"link","href":"https://example.com"}],[{"tag":"text","text":"second"}]]}`
	content, err := decodePostMessage(&raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "Update") {
		t.Errorf("missing title in content: %q", content)
	}
	if !strings.Contains(content, "link (https://example.com)") {
		t.Errorf("missing link in content: %q", content)
	}
	if !strings.Contains(content, "second") {
		t.Errorf("missing second paragraph: %q", content)
	}
}

func TestDecodePostMessage_NilReturnsError(t *testing.T) {
	if _, err := decodePostMessage(nil); err == nil {
		t.Fatal("expected error for nil")
	}
}

// --- Image message tests ---

func TestDecodeImageMessage(t *testing.T) {
	raw := `{"image_key":"img_v2_abc123"}`
	content, metadata, attachments, err := decodeImageMessage(&raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "img_v2_abc123") {
		t.Errorf("content = %q, want image key", content)
	}
	if metadata["image_key"] != "img_v2_abc123" {
		t.Errorf("metadata = %v", metadata)
	}
	if len(attachments) != 1 || attachments[0].Kind != core.AttachmentKindImage {
		t.Errorf("attachments = %v", attachments)
	}
}

func TestDecodeImageMessage_MissingKeyReturnsError(t *testing.T) {
	raw := `{}`
	if _, _, _, err := decodeImageMessage(&raw); err == nil {
		t.Fatal("expected error for missing image key")
	}
}

// --- File message tests ---

func TestDecodeFileMessage(t *testing.T) {
	raw := `{"file_key":"file_v2_xyz","file_name":"report.pdf"}`
	content, metadata, attachments, err := decodeFileMessage(&raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "report.pdf") {
		t.Errorf("content = %q, want file name", content)
	}
	if metadata["file_key"] != "file_v2_xyz" {
		t.Errorf("metadata = %v", metadata)
	}
	if metadata["file_name"] != "report.pdf" {
		t.Errorf("metadata = %v", metadata)
	}
	if len(attachments) != 1 || attachments[0].Filename != "report.pdf" {
		t.Errorf("attachments = %v", attachments)
	}
}

func TestDecodeFileMessage_MissingKeyReturnsError(t *testing.T) {
	raw := `{}`
	if _, _, _, err := decodeFileMessage(&raw); err == nil {
		t.Fatal("expected error for missing file key")
	}
}

// --- StructuredSender tests ---

func TestLive_SendStructured(t *testing.T) {
	sender := &fakeMessageClient{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	msg := &core.StructuredMessage{
		Title: "Test",
		Body:  "Hello",
	}
	if err := live.SendStructured(context.Background(), "chat-1", msg); err != nil {
		t.Fatalf("SendStructured error: %v", err)
	}
	if len(sender.sendCalls) != 1 {
		t.Fatalf("sendCalls = %d, want 1", len(sender.sendCalls))
	}
	if sender.sendCalls[0].MsgType != larkim.MsgTypeInteractive {
		t.Fatalf("MsgType = %q, want interactive", sender.sendCalls[0].MsgType)
	}
}

func TestLive_ReplyStructured(t *testing.T) {
	sender := &fakeMessageClient{}
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	msg := &core.StructuredMessage{
		Title: "Reply",
		Sections: []core.StructuredSection{
			{Type: "text", TextSection: &core.TextSection{Body: "content"}},
		},
	}
	if err := live.ReplyStructured(context.Background(), replyContext{MessageID: "msg-1"}, msg); err != nil {
		t.Fatalf("ReplyStructured error: %v", err)
	}
	if len(sender.replyCalls) != 1 {
		t.Fatalf("replyCalls = %d, want 1", len(sender.replyCalls))
	}
}

// --- Expanded action callback tests ---

func TestNormalizeCardActionRequest_SelectStatic(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:    "select_static",
				Option: "option_1",
				Value:  map[string]interface{}{"action": "act:choose:task-1"},
			},
			Token: "token-1",
			Context: &larkcallback.Context{
				OpenChatID:    "chat-1",
				OpenMessageID: "msg-1",
			},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != "choose" {
		t.Errorf("Action = %q, want choose", req.Action)
	}
	if req.EntityID != "task-1" {
		t.Errorf("EntityID = %q, want task-1", req.EntityID)
	}
	if req.Metadata["action_tag"] != "select_static" {
		t.Errorf("missing action_tag in metadata")
	}
	if req.Metadata["selected_option"] != "option_1" {
		t.Errorf("missing selected_option in metadata")
	}
}

func TestNormalizeCardActionRequest_SelectFallback(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:    "select_static",
				Option: "opt_deploy",
				Value:  map[string]interface{}{},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != "select" {
		t.Errorf("Action = %q, want select", req.Action)
	}
	if req.EntityID != "opt_deploy" {
		t.Errorf("EntityID = %q, want opt_deploy", req.EntityID)
	}
}

func TestNormalizeCardActionRequest_DatePicker(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:   "date_picker",
				Value: map[string]interface{}{"date": "2026-04-15"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != "date_pick" {
		t.Errorf("Action = %q, want date_pick", req.Action)
	}
	if req.EntityID != "2026-04-15" {
		t.Errorf("EntityID = %q, want 2026-04-15", req.EntityID)
	}
}

func TestNormalizeCardActionRequest_FormSubmit(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:       "button",
				Value:     map[string]interface{}{},
				FormValue: map[string]interface{}{"action": "create-task", "title": "New task"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != "form_submit" {
		t.Errorf("Action = %q, want form_submit", req.Action)
	}
	if req.EntityID != "create-task" {
		t.Errorf("EntityID = %q, want create-task", req.EntityID)
	}
	if req.Metadata["form_title"] != "New task" {
		t.Errorf("missing form_title in metadata: %v", req.Metadata)
	}
}

// --- Lifecycle event tests ---

type fakeLifecycleHandler struct {
	addedChats   []string
	removedChats []string
}

func (h *fakeLifecycleHandler) OnBotAdded(_ context.Context, _ core.Platform, chatID string) error {
	h.addedChats = append(h.addedChats, chatID)
	return nil
}

func (h *fakeLifecycleHandler) OnBotRemoved(_ context.Context, _ core.Platform, chatID string) error {
	h.removedChats = append(h.removedChats, chatID)
	return nil
}

func TestLive_HandleBotAdded(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	handler := &fakeLifecycleHandler{}
	live.SetLifecycleHandler(handler)

	chatID := "chat-added"
	live.handleBotAdded(context.Background(), &larkim.P2ChatMemberBotAddedV1{
		Event: &larkim.P2ChatMemberBotAddedV1Data{
			ChatId: &chatID,
		},
	})

	if len(handler.addedChats) != 1 || handler.addedChats[0] != "chat-added" {
		t.Fatalf("addedChats = %v", handler.addedChats)
	}
}

func TestLive_HandleBotRemoved(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	handler := &fakeLifecycleHandler{}
	live.SetLifecycleHandler(handler)

	chatID := "chat-removed"
	live.handleBotRemoved(context.Background(), &larkim.P2ChatMemberBotDeletedV1{
		Event: &larkim.P2ChatMemberBotDeletedV1Data{
			ChatId: &chatID,
		},
	})

	if len(handler.removedChats) != 1 || handler.removedChats[0] != "chat-removed" {
		t.Fatalf("removedChats = %v", handler.removedChats)
	}
}

func TestLive_HandleBotAddedNilEvent(t *testing.T) {
	live, _ := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	// Should not panic on nil event.
	if err := live.handleBotAdded(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLive_HandleReactionNoError(t *testing.T) {
	live, _ := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	// Reactions are logged only; should not error.
	if err := live.handleReaction(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeCardActionRequest_CheckerChecked(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "checker",
				Checked: true,
				Value:   map[string]interface{}{"action": "act:toggle:task-xyz"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1", OpenMessageID: "msg-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameToggle {
		t.Errorf("Action = %q, want toggle", req.Action)
	}
	if req.EntityID != "task-xyz" {
		t.Errorf("EntityID = %q, want task-xyz", req.EntityID)
	}
	if req.Metadata["checker_state"] != "true" {
		t.Errorf("checker_state = %q, want true", req.Metadata["checker_state"])
	}
	if req.Metadata["action_tag"] != "checker" {
		t.Errorf("action_tag = %q, want checker", req.Metadata["action_tag"])
	}
}

func TestNormalizeCardActionRequest_CheckerUncheckedWithoutActionRef(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "checker",
				Checked: false,
				Value:   map[string]interface{}{},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameToggle {
		t.Errorf("Action = %q, want toggle", req.Action)
	}
	if req.EntityID != "" {
		t.Errorf("EntityID = %q, want empty", req.EntityID)
	}
	if req.Metadata["checker_state"] != "false" {
		t.Errorf("checker_state = %q, want false", req.Metadata["checker_state"])
	}
}

func TestNormalizeCardActionRequest_MultiSelectWithActionRef(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "multi_select_static",
				Options: []string{"opt_a", "opt_b", "opt_c"},
				Value:   map[string]interface{}{"action": "act:assign-agent:task-xyz"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameMultiSelect {
		t.Errorf("Action = %q, want multi_select", req.Action)
	}
	if req.EntityID != "task-xyz" {
		t.Errorf("EntityID = %q, want task-xyz", req.EntityID)
	}
	if req.Metadata["selected_options"] != "opt_a,opt_b,opt_c" {
		t.Errorf("selected_options = %q, want opt_a,opt_b,opt_c", req.Metadata["selected_options"])
	}
}

func TestNormalizeCardActionRequest_MultiSelectPersonFallback(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:     "multi_select_person",
				Options: []string{"ou_user_a", "ou_user_b"},
				Value:   map[string]interface{}{},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameMultiSelect {
		t.Errorf("Action = %q, want multi_select", req.Action)
	}
	if req.Metadata["selected_options"] != "ou_user_a,ou_user_b" {
		t.Errorf("selected_options = %q", req.Metadata["selected_options"])
	}
}

func TestNormalizeCardActionRequest_InputWithActionRef(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:        "input",
				InputValue: "please reconsider",
				Value:      map[string]interface{}{"action": "act:input_submit:task-xyz"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != core.ActionNameInputSubmit {
		t.Errorf("Action = %q, want input_submit", req.Action)
	}
	if req.EntityID != "task-xyz" {
		t.Errorf("EntityID = %q, want task-xyz", req.EntityID)
	}
	if req.Metadata["input_value"] != "please reconsider" {
		t.Errorf("input_value = %q", req.Metadata["input_value"])
	}
}

func TestNormalizeCardActionRequest_InputWithoutActionRefIsIgnored(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:        "input",
				InputValue: "typed text",
				Value:      map[string]interface{}{},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	_, err := normalizeCardActionRequest(event)
	if !errors.Is(err, errIgnoreCardAction) {
		t.Fatalf("err = %v, want errIgnoreCardAction", err)
	}
}

func TestNormalizeCardActionRequest_PassesElementNameAndTimezone(t *testing.T) {
	event := &larkcallback.CardActionTriggerEvent{
		Event: &larkcallback.CardActionTriggerRequest{
			Action: &larkcallback.CallBackAction{
				Tag:      "date_picker",
				Name:     "due_date_picker",
				Timezone: "Asia/Shanghai",
				Value:    map[string]interface{}{"date": "2026-04-20"},
			},
			Token:    "token-1",
			Context:  &larkcallback.Context{OpenChatID: "chat-1"},
			Operator: &larkcallback.Operator{OpenID: "ou_user_1"},
		},
	}
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Metadata["element_name"] != "due_date_picker" {
		t.Errorf("element_name = %q", req.Metadata["element_name"])
	}
	if req.Metadata["timezone"] != "Asia/Shanghai" {
		t.Errorf("timezone = %q", req.Metadata["timezone"])
	}
}

type recordingActionHandler struct {
	reqs []*notify.ActionRequest
}

func (h *recordingActionHandler) HandleAction(_ context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	h.reqs = append(h.reqs, req)
	return &notify.ActionResponse{}, nil
}

func TestLive_HandleReactionCreatedForwardsToActionHandler(t *testing.T) {
	live, err := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	rec := &recordingActionHandler{}
	live.SetActionHandler(rec)

	msgID := "om_reaction_msg"
	emojiType := "THUMBSUP"
	openID := "ou_reactor"
	event := &larkim.P2MessageReactionCreatedV1{
		Event: &larkim.P2MessageReactionCreatedV1Data{
			MessageId:    &msgID,
			ReactionType: &larkim.Emoji{EmojiType: &emojiType},
			UserId:       &larkim.UserId{OpenId: &openID},
		},
	}
	if err := live.handleReaction(context.Background(), event); err != nil {
		t.Fatalf("handleReaction error: %v", err)
	}
	if len(rec.reqs) != 1 {
		t.Fatalf("expected 1 forwarded request, got %d", len(rec.reqs))
	}
	got := rec.reqs[0]
	if got.Action != core.ActionNameReact {
		t.Errorf("Action = %q, want react", got.Action)
	}
	if got.EntityID != msgID {
		t.Errorf("EntityID = %q, want %s", got.EntityID, msgID)
	}
	if got.UserID != openID {
		t.Errorf("UserID = %q, want %s", got.UserID, openID)
	}
	if got.Metadata["emoji"] != emojiType {
		t.Errorf("emoji = %q, want %s", got.Metadata["emoji"], emojiType)
	}
	if got.Metadata["event_type"] != "created" {
		t.Errorf("event_type = %q, want created", got.Metadata["event_type"])
	}
}

func TestLive_HandleReactionDeletedForwardsWithDeletedEventType(t *testing.T) {
	live, _ := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	rec := &recordingActionHandler{}
	live.SetActionHandler(rec)

	msgID := "om_reaction_msg"
	emojiType := "THUMBSUP"
	openID := "ou_reactor"
	event := &larkim.P2MessageReactionDeletedV1{
		Event: &larkim.P2MessageReactionDeletedV1Data{
			MessageId:    &msgID,
			ReactionType: &larkim.Emoji{EmojiType: &emojiType},
			UserId:       &larkim.UserId{OpenId: &openID},
		},
	}
	if err := live.handleReactionDeleted(context.Background(), event); err != nil {
		t.Fatalf("handleReactionDeleted error: %v", err)
	}
	if len(rec.reqs) != 1 {
		t.Fatalf("expected 1 forwarded request")
	}
	if rec.reqs[0].Metadata["event_type"] != "deleted" {
		t.Errorf("event_type = %q, want deleted", rec.reqs[0].Metadata["event_type"])
	}
}

func TestLive_HandleReactionWithNoHandlerIsNoOp(t *testing.T) {
	live, _ := NewLive("app-id", "app-secret", WithEventRunner(&fakeEventRunner{}), WithMessageClient(&fakeMessageClient{}))
	msgID := "om_x"
	emojiType := "OK"
	openID := "ou_x"
	event := &larkim.P2MessageReactionCreatedV1{
		Event: &larkim.P2MessageReactionCreatedV1Data{
			MessageId:    &msgID,
			ReactionType: &larkim.Emoji{EmojiType: &emojiType},
			UserId:       &larkim.UserId{OpenId: &openID},
		},
	}
	if err := live.handleReaction(context.Background(), event); err != nil {
		t.Fatalf("expected no error when handler missing, got %v", err)
	}
}
