package dingtalk

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dingtalkcardapi "github.com/alibabacloud-go/dingtalk/card_1_0"
	teautil "github.com/alibabacloud-go/tea-utils/v2/service"
	dingtalkcard "github.com/open-dingtalk/dingtalk-stream-sdk-go/card"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

func TestNewLive_RequiresCredentials(t *testing.T) {
	if _, err := NewLive("", "secret"); err == nil {
		t.Fatal("expected missing app key to fail")
	}
	if _, err := NewLive("app-key", ""); err == nil {
		t.Fatal("expected missing app secret to fail")
	}
}

func TestLive_StartNormalizesChatbotMessage(t *testing.T) {
	runner := &fakeStreamRunner{
		messages: []chatbotMessage{
			{
				ConversationID:   "cid-group-1",
				ConversationType: "2",
				SenderStaffID:    "staff-1",
				SenderNick:       "Ding User",
				SessionWebhook:   "https://session.example/reply",
				Text:             "@AgentForge /task list",
				CreatedAt:        time.Unix(1710000000, 0),
			},
		},
	}

	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(runner),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
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

	if gotPlatform != live {
		t.Fatalf("platform = %#v, want live platform", gotPlatform)
	}
	if gotMessage == nil {
		t.Fatal("expected normalized message")
	}
	if gotMessage.Platform != "dingtalk" {
		t.Fatalf("Platform = %q", gotMessage.Platform)
	}
	if gotMessage.SessionKey != "dingtalk:cid-group-1:staff-1" {
		t.Fatalf("SessionKey = %q", gotMessage.SessionKey)
	}
	if gotMessage.UserID != "staff-1" {
		t.Fatalf("UserID = %q", gotMessage.UserID)
	}
	if gotMessage.UserName != "Ding User" {
		t.Fatalf("UserName = %q", gotMessage.UserName)
	}
	if gotMessage.ChatID != "cid-group-1" {
		t.Fatalf("ChatID = %q", gotMessage.ChatID)
	}
	if gotMessage.Content != "@AgentForge /task list" {
		t.Fatalf("Content = %q", gotMessage.Content)
	}
	if !gotMessage.IsGroup {
		t.Fatal("expected group conversation")
	}

	replyCtx, ok := gotMessage.ReplyCtx.(replyContext)
	if !ok {
		t.Fatalf("ReplyCtx type = %T", gotMessage.ReplyCtx)
	}
	if replyCtx.SessionWebhook != "https://session.example/reply" {
		t.Fatalf("SessionWebhook = %q", replyCtx.SessionWebhook)
	}
	if replyCtx.ConversationID != "cid-group-1" {
		t.Fatalf("ConversationID = %q", replyCtx.ConversationID)
	}
}

func TestLive_StartRoutesCardCallbackToActionHandlerAndRepliesViaSessionWebhook(t *testing.T) {
	runner := &fakeStreamRunner{
		cardRequests: []*dingtalkcard.CardRequest{
			{
				SpaceId:   "cid-group-1",
				SpaceType: "group",
				UserId:    "staff-1",
				Extension: `{"sessionWebhook":"https://session.example/reply","conversationId":"cid-group-1","conversationType":"2"}`,
				CardActionData: dingtalkcard.PrivateCardActionData{
					CardPrivateData: dingtalkcard.CardPrivateData{
						ActionIdList: []string{"approve"},
						Params: map[string]any{
							"action": "act:approve:review-1",
						},
					},
				},
			},
		},
	}
	replier := &fakeWebhookReplier{}
	actions := &fakeDingTalkActionHandler{}

	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(runner),
		WithWebhookReplier(replier),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}
	live.SetActionHandler(actions)

	if err := live.Start(func(p core.Platform, msg *core.Message) {
		t.Fatalf("message handler should not receive card callbacks: %+v", msg)
	}); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	if len(actions.requests) != 1 {
		t.Fatalf("requests = %+v", actions.requests)
	}
	req := actions.requests[0]
	if req.Platform != "dingtalk" || req.Action != "approve" || req.EntityID != "review-1" {
		t.Fatalf("request = %+v", req)
	}
	if req.ChatID != "cid-group-1" || req.UserID != "staff-1" {
		t.Fatalf("request = %+v", req)
	}
	if req.ReplyTarget == nil || req.ReplyTarget.SessionWebhook != "https://session.example/reply" {
		t.Fatalf("ReplyTarget = %+v", req.ReplyTarget)
	}
	if req.Metadata["source"] != "card_callback" || req.Metadata["space_type"] != "group" {
		t.Fatalf("Metadata = %+v", req.Metadata)
	}
	if len(replier.calls) != 1 {
		t.Fatalf("webhook calls = %+v", replier.calls)
	}
	if replier.calls[0].Webhook != "https://session.example/reply" || replier.calls[0].Content != "Approved" {
		t.Fatalf("webhook call = %+v", replier.calls[0])
	}
}

func TestLive_ReplyAndSendUseOutboundClients(t *testing.T) {
	replier := &fakeWebhookReplier{}
	messenger := &fakeDirectMessenger{}

	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(replier),
		WithDirectMessenger(messenger),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	replyCtx := replyContext{
		SessionWebhook: "https://session.example/reply",
		ConversationID: "cid-group-1",
	}
	if err := live.Reply(context.Background(), replyCtx, "reply text"); err != nil {
		t.Fatalf("Reply error: %v", err)
	}
	if len(replier.calls) != 1 {
		t.Fatalf("webhook calls = %+v", replier.calls)
	}
	if replier.calls[0].Webhook != "https://session.example/reply" {
		t.Fatalf("reply webhook = %q", replier.calls[0].Webhook)
	}
	if replier.calls[0].Content != "reply text" {
		t.Fatalf("reply content = %q", replier.calls[0].Content)
	}

	if err := live.Send(context.Background(), "cid-group-2", "group notification"); err != nil {
		t.Fatalf("Send group error: %v", err)
	}
	if err := live.Send(context.Background(), "union-user-2", "dm notification"); err != nil {
		t.Fatalf("Send dm error: %v", err)
	}
	if len(messenger.calls) != 2 {
		t.Fatalf("direct messenger calls = %+v", messenger.calls)
	}
	if messenger.calls[0].OpenConversationID != "cid-group-2" {
		t.Fatalf("group target = %+v", messenger.calls[0])
	}
	if messenger.calls[1].UnionID != "union-user-2" {
		t.Fatalf("dm target = %+v", messenger.calls[1])
	}
}

func TestLive_SendStructuredFallsBackToTextWithExplicitDowngrade(t *testing.T) {
	messenger := &fakeDirectMessenger{}

	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(messenger),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.SendStructured(context.Background(), "cid-group-2", &core.StructuredMessage{
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

	if len(messenger.calls) != 1 {
		t.Fatalf("direct messenger calls = %+v", messenger.calls)
	}
	if messenger.calls[0].OpenConversationID != "cid-group-2" {
		t.Fatalf("target = %+v", messenger.calls[0])
	}
	if messenger.calls[0].Content == "" {
		t.Fatalf("content = %q", messenger.calls[0].Content)
	}
	if !containsAll(messenger.calls[0].Content, []string{"Review Ready", "Approve", "Open", "已降级为文本"}) {
		t.Fatalf("content = %q", messenger.calls[0].Content)
	}
}

func TestLive_ReplyCardUsesSessionWebhookActionCardForLinkButtons(t *testing.T) {
	replier := &fakeWebhookReplier{}
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(replier),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	plan, err := core.DeliverCard(context.Background(), live, live.Metadata(), &core.ReplyTarget{
		Platform:       "dingtalk",
		SessionWebhook: "https://session.example/reply",
		ConversationID: "cid-group-1",
		UseReply:       true,
	}, "", core.NewCard().
		SetTitle("Review Ready").
		AddField("Status", "pending_human").
		AddButton("Open", "link:https://example.test/reviews/1"))
	if err != nil {
		t.Fatalf("DeliverCard error: %v", err)
	}

	if plan.FallbackReason != "" {
		t.Fatalf("FallbackReason = %q, want empty", plan.FallbackReason)
	}
	if len(replier.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", replier.messageCalls)
	}
	if replier.messageCalls[0].Webhook != "https://session.example/reply" {
		t.Fatalf("webhook = %q", replier.messageCalls[0].Webhook)
	}
	if replier.messageCalls[0].Body["msgtype"] != "actionCard" {
		t.Fatalf("payload = %+v", replier.messageCalls[0].Body)
	}
}

func TestLive_ReplyCardFallsBackToTextForInteractiveButtons(t *testing.T) {
	replier := &fakeWebhookReplier{}
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(replier),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	plan, err := core.DeliverCard(context.Background(), live, live.Metadata(), &core.ReplyTarget{
		Platform:       "dingtalk",
		SessionWebhook: "https://session.example/reply",
		ConversationID: "cid-group-1",
		UseReply:       true,
	}, "", core.NewCard().
		SetTitle("Review Ready").
		AddPrimaryButton("Approve", "act:approve:review-1"))
	if err != nil {
		t.Fatalf("DeliverCard error: %v", err)
	}

	if plan.FallbackReason != "actioncard_send_failed" {
		t.Fatalf("FallbackReason = %q, want actioncard_send_failed", plan.FallbackReason)
	}
	if len(replier.calls) != 1 {
		t.Fatalf("text fallback calls = %+v", replier.calls)
	}
	if !strings.Contains(replier.calls[0].Content, "已降级为文本") {
		t.Fatalf("fallback content = %q", replier.calls[0].Content)
	}
}

func TestLive_SendCardUsesRobotCardSenderForDirectConversation(t *testing.T) {
	cardSender := &fakeRobotCardSender{}
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(&fakeDirectMessenger{}),
		WithRobotCardSender(cardSender),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	plan, err := core.DeliverCard(context.Background(), live, live.Metadata(), nil, "cid-group-2", core.NewCard().
		SetTitle("Review Ready").
		AddField("Status", "pending_human").
		AddPrimaryButton("Approve", "act:approve:review-1"))
	if err != nil {
		t.Fatalf("DeliverCard error: %v", err)
	}

	if plan.Method != core.DeliveryMethodSend {
		t.Fatalf("Method = %q, want send", plan.Method)
	}
	if plan.FallbackReason != "" {
		t.Fatalf("FallbackReason = %q, want empty", plan.FallbackReason)
	}
	if len(cardSender.calls) != 1 {
		t.Fatalf("calls = %+v", cardSender.calls)
	}
	if cardSender.calls[0].Target.OpenConversationID != "cid-group-2" {
		t.Fatalf("target = %+v", cardSender.calls[0].Target)
	}
}

func TestLive_SendCardUsesAdvancedTemplateWhenConfigured(t *testing.T) {
	advancedClient := &fakeAdvancedCardClient{
		response: &dingtalkcardapi.CreateAndDeliverResponse{
			StatusCode: int32Ptr(200),
		},
	}
	sender := &sdkRobotCardSender{
		client:             mustNewIMClient(),
		advancedClient:     advancedClient,
		tokenProvider:      staticTokenProvider("token"),
		robotCode:          "app-key",
		templateID:         "StandardCard",
		advancedTemplateID: "template-1.schema",
	}
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(&fakeDirectMessenger{}),
		WithRobotCardSender(sender),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	_, err = core.DeliverCard(context.Background(), live, live.Metadata(), nil, "cid-group-2", core.NewCard().
		SetTitle("Review Ready").
		AddPrimaryButton("Approve", "act:approve:review-1").
		AddDangerButton("Reject", "act:reject:review-1"))
	if err != nil {
		t.Fatalf("DeliverCard error: %v", err)
	}

	if advancedClient.request == nil {
		t.Fatal("expected CreateAndDeliver request")
	}
	if advancedClient.request.CallbackType == nil || *advancedClient.request.CallbackType != "STREAM" {
		t.Fatalf("request = %+v", advancedClient.request)
	}
	if advancedClient.request.OpenSpaceId == nil || !strings.Contains(*advancedClient.request.OpenSpaceId, "IM_GROUP.cid-group-2") {
		t.Fatalf("openSpaceId = %+v", advancedClient.request.OpenSpaceId)
	}
	if advancedClient.request.CardData == nil || advancedClient.request.CardData.CardParamMap["action_1_ref"] == nil || *advancedClient.request.CardData.CardParamMap["action_1_ref"] != "act:approve:review-1" {
		t.Fatalf("cardData = %+v", advancedClient.request.CardData)
	}
}

func TestLive_SendCardFallsBackWhenRobotCardSenderFails(t *testing.T) {
	cardSender := &fakeRobotCardSender{err: errors.New("send card failed")}
	messenger := &fakeDirectMessenger{}
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(messenger),
		WithRobotCardSender(cardSender),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	plan, err := core.DeliverCard(context.Background(), live, live.Metadata(), nil, "cid-group-2", core.NewCard().
		SetTitle("Review Ready").
		AddPrimaryButton("Approve", "act:approve:review-1"))
	if err != nil {
		t.Fatalf("DeliverCard error: %v", err)
	}

	if plan.FallbackReason != "actioncard_send_failed" {
		t.Fatalf("FallbackReason = %q, want actioncard_send_failed", plan.FallbackReason)
	}
	if len(messenger.calls) != 1 {
		t.Fatalf("fallback messenger calls = %+v", messenger.calls)
	}
}

func TestLive_MetadataDeclaresActionCardCapabilities(t *testing.T) {
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	metadata := live.Metadata()
	if metadata.Source != "dingtalk" {
		t.Fatalf("source = %q", metadata.Source)
	}
	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected DingTalk live transport to advertise action card support")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected slash-like commands support")
	}
	if !metadata.Capabilities.SupportsMentions {
		t.Fatal("expected mention support")
	}
	if _, ok := any(live).(core.CardSender); !ok {
		t.Fatal("expected DingTalk live transport to implement CardSender")
	}
}

func TestLive_StartRequiresHandler(t *testing.T) {
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.Start(nil); err == nil {
		t.Fatal("expected nil handler to fail")
	}
}

func TestLive_PropagatesRunnerFailure(t *testing.T) {
	expected := errors.New("boom")
	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{startErr: expected}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if err := live.Start(func(core.Platform, *core.Message) {}); !errors.Is(err, expected) {
		t.Fatalf("Start error = %v, want %v", err, expected)
	}
}

type fakeStreamRunner struct {
	messages     []chatbotMessage
	cardRequests []*dingtalkcard.CardRequest
	startErr     error
	stopErr      error
}

func (f *fakeStreamRunner) Start(ctx context.Context, handler func(context.Context, chatbotMessage) error) error {
	return f.StartWithCardCallbacks(ctx, handler, nil)
}

func (f *fakeStreamRunner) StartWithCardCallbacks(ctx context.Context, handler func(context.Context, chatbotMessage) error, cardHandler func(context.Context, *dingtalkcard.CardRequest) (*dingtalkcard.CardResponse, error)) error {
	if f.startErr != nil {
		return f.startErr
	}
	for _, message := range f.messages {
		if err := handler(ctx, message); err != nil {
			return err
		}
	}
	for _, request := range f.cardRequests {
		if cardHandler == nil {
			continue
		}
		if _, err := cardHandler(ctx, request); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeStreamRunner) Stop(context.Context) error {
	return f.stopErr
}

type fakeWebhookReplier struct {
	calls        []webhookReplyCall
	messageCalls []webhookMessageCall
	err          error
}

type webhookReplyCall struct {
	Webhook string
	Content string
}

type webhookMessageCall struct {
	Webhook string
	Body    map[string]any
}

func (f *fakeWebhookReplier) ReplyText(ctx context.Context, sessionWebhook string, content string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, webhookReplyCall{
		Webhook: sessionWebhook,
		Content: content,
	})
	return nil
}

func (f *fakeWebhookReplier) ReplyMessage(ctx context.Context, sessionWebhook string, requestBody map[string]any) error {
	if f.err != nil {
		return f.err
	}
	f.messageCalls = append(f.messageCalls, webhookMessageCall{
		Webhook: sessionWebhook,
		Body:    requestBody,
	})
	return nil
}

type fakeDirectMessenger struct {
	calls []directSendCall
	err   error
}

type directSendCall struct {
	OpenConversationID string
	UnionID            string
	Content            string
}

func (f *fakeDirectMessenger) SendText(ctx context.Context, target directSendTarget, content string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, directSendCall{
		OpenConversationID: target.OpenConversationID,
		UnionID:            target.UnionID,
		Content:            content,
	})
	return nil
}

type fakeRobotCardSender struct {
	calls []robotCardCall
	err   error
}

type robotCardCall struct {
	Target directSendTarget
	Card   *core.Card
}

func (f *fakeRobotCardSender) SendCard(ctx context.Context, target directSendTarget, card *core.Card) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, robotCardCall{
		Target: target,
		Card:   card,
	})
	return nil
}

type fakeAdvancedCardClient struct {
	request  *dingtalkcardapi.CreateAndDeliverRequest
	headers  *dingtalkcardapi.CreateAndDeliverHeaders
	response *dingtalkcardapi.CreateAndDeliverResponse
	err      error
}

func (f *fakeAdvancedCardClient) CreateAndDeliverWithOptions(
	request *dingtalkcardapi.CreateAndDeliverRequest,
	headers *dingtalkcardapi.CreateAndDeliverHeaders,
	runtime *teautil.RuntimeOptions,
) (*dingtalkcardapi.CreateAndDeliverResponse, error) {
	f.request = request
	f.headers = headers
	_ = runtime
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func int32Ptr(value int32) *int32 {
	return &value
}

type staticTokenProvider string

func (s staticTokenProvider) AccessToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(string(s)) == "" {
		return "", errors.New("missing token")
	}
	return string(s), nil
}

type fakeDingTalkActionHandler struct {
	requests []*notify.ActionRequest
}

func (h *fakeDingTalkActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	h.requests = append(h.requests, req)
	return &notify.ActionResponse{Result: "Approved"}, nil
}

func containsAll(content string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(content, part) {
			return false
		}
	}
	return true
}
