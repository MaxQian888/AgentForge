package dingtalk

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	dingtalkcardapi "github.com/alibabacloud-go/dingtalk/card_1_0"
	teautil "github.com/alibabacloud-go/tea-utils/v2/service"
	dingtalkcard "github.com/open-dingtalk/dingtalk-stream-sdk-go/card"
	"strings"
	"testing"
	"time"
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
	foundMarkdown := false
	for _, format := range metadata.Rendering.SupportedFormats {
		if format == core.TextFormatDingTalkMD {
			foundMarkdown = true
			break
		}
	}
	if !foundMarkdown {
		t.Fatalf("SupportedFormats = %+v, want dingtalk_md", metadata.Rendering.SupportedFormats)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != core.NativeSurfaceDingTalkCard {
		t.Fatalf("NativeSurfaces = %+v", metadata.Rendering.NativeSurfaces)
	}
}

func TestLive_SendNativeUsesWebhookActionCard(t *testing.T) {
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

	message, err := core.NewDingTalkCardMessage(
		core.DingTalkCardTypeActionCard,
		"Review Ready",
		"### Choose the next step",
		[]core.DingTalkCardButton{{Title: "Open", ActionURL: "https://example.test/reviews/1"}},
	)
	if err != nil {
		t.Fatalf("NewDingTalkCardMessage error: %v", err)
	}

	if err := live.SendNative(context.Background(), "https://session.example/reply", message); err != nil {
		t.Fatalf("SendNative error: %v", err)
	}

	if len(replier.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", replier.messageCalls)
	}
	if replier.messageCalls[0].Body["msgtype"] != "actionCard" {
		t.Fatalf("payload = %+v", replier.messageCalls[0].Body)
	}
}

func TestLive_SendFormattedTextUsesMarkdownWebhookPayload(t *testing.T) {
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

	if err := live.SendFormattedText(context.Background(), "https://session.example/reply", &core.FormattedText{
		Content: "### Review Ready",
		Format:  core.TextFormatDingTalkMD,
	}); err != nil {
		t.Fatalf("SendFormattedText error: %v", err)
	}

	if len(replier.messageCalls) != 1 {
		t.Fatalf("messageCalls = %+v", replier.messageCalls)
	}
	if replier.messageCalls[0].Body["msgtype"] != "markdown" {
		t.Fatalf("payload = %+v", replier.messageCalls[0].Body)
	}
}

func TestRenderStructuredSectionsBuildsDingTalkCardPayload(t *testing.T) {
	payload := renderStructuredSections([]core.StructuredSection{
		{
			Type: core.StructuredSectionTypeText,
			TextSection: &core.TextSection{
				Body: "Build ready",
			},
		},
		{
			Type: core.StructuredSectionTypeImage,
			ImageSection: &core.ImageSection{
				URL: "https://example.test/build.png",
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
				Actions: []core.StructuredAction{{Label: "Open", URL: "https://example.test/reviews/1"}},
			},
		},
	})

	if payload.Title == "" || payload.Markdown == "" || len(payload.Buttons) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if !containsAll(payload.Markdown, []string{"Build ready", "Status", "https://example.test/build.png"}) {
		t.Fatalf("markdown = %q", payload.Markdown)
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

func TestDingTalkLive_NameReplyContextAndLifecycle(t *testing.T) {
	runner := &fakeStreamRunner{}
	replier := &fakeWebhookReplier{}
	messenger := &fakeDirectMessenger{}

	live, err := NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(runner),
		WithWebhookReplier(replier),
		WithDirectMessenger(messenger),
	)
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	if live.Name() != "dingtalk-live" {
		t.Fatalf("Name = %q", live.Name())
	}
	if live.ReplyContextFromTarget(nil) != nil {
		t.Fatal("expected nil reply target to stay nil")
	}

	replyAny := live.ReplyContextFromTarget(&core.ReplyTarget{
		SessionWebhook: " https://session.example/reply ",
		ConversationID: "cid-group-1",
		UserID:         "staff-1",
		Metadata: map[string]string{
			"conversation_type": " 2 ",
		},
	})
	reply, ok := replyAny.(replyContext)
	if !ok {
		t.Fatalf("ReplyContextFromTarget type = %T", replyAny)
	}
	if reply.SessionWebhook != "https://session.example/reply" || reply.ConversationID != "cid-group-1" || reply.ConversationType != "2" || reply.UserID != "staff-1" {
		t.Fatalf("reply = %+v", reply)
	}

	if err := live.Stop(); err != nil {
		t.Fatalf("Stop before Start error: %v", err)
	}

	stopErr := context.Canceled
	live, err = NewLive(
		"app-key",
		"app-secret",
		WithStreamRunner(&fakeStreamRunner{stopErr: stopErr}),
		WithWebhookReplier(&fakeWebhookReplier{}),
		WithDirectMessenger(&fakeDirectMessenger{}),
	)
	if err != nil {
		t.Fatalf("NewLive second error: %v", err)
	}
	if err := live.Start(func(core.Platform, *core.Message) {}); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := live.Stop(); err != stopErr {
		t.Fatalf("Stop error = %v, want %v", err, stopErr)
	}
}

func TestDingTalkLive_ReplyStructuredNativeAndFormattedBranches(t *testing.T) {
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

	err = live.ReplyStructured(context.Background(), replyContext{
		SessionWebhook: "https://session.example/reply",
		ConversationID: "cid-group-1",
	}, &core.StructuredMessage{
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
					Actions: []core.StructuredAction{
						{Label: "Open", URL: "https://example.test/builds/1"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ReplyStructured error: %v", err)
	}
	if len(replier.messageCalls) != 1 || replier.messageCalls[0].Body["msgtype"] != "actionCard" {
		t.Fatalf("messageCalls = %+v", replier.messageCalls)
	}

	replier.messageCalls = nil
	err = live.ReplyFormattedText(context.Background(), replyContext{
		SessionWebhook: "https://session.example/reply",
		ConversationID: "cid-group-1",
	}, &core.FormattedText{
		Content: "### Review Ready",
		Format:  core.TextFormatDingTalkMD,
	})
	if err != nil {
		t.Fatalf("ReplyFormattedText error: %v", err)
	}
	if len(replier.messageCalls) != 1 || replier.messageCalls[0].Body["msgtype"] != "markdown" {
		t.Fatalf("formatted messageCalls = %+v", replier.messageCalls)
	}

	replier.calls = nil
	err = live.UpdateFormattedText(context.Background(), replyContext{
		SessionWebhook: "https://session.example/reply",
		ConversationID: "cid-group-1",
	}, &core.FormattedText{
		Content: "plain fallback",
		Format:  core.TextFormatPlainText,
	})
	if err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}
	if len(replier.calls) != 1 || replier.calls[0].Content != "plain fallback" {
		t.Fatalf("replier.calls = %+v", replier.calls)
	}

	native, err := core.NewDingTalkCardMessage(
		core.DingTalkCardTypeActionCard,
		"Review Ready",
		"### Choose the next step",
		[]core.DingTalkCardButton{{Title: "Open", ActionURL: "https://example.test/reviews/1"}},
	)
	if err != nil {
		t.Fatalf("NewDingTalkCardMessage error: %v", err)
	}
	replier.messageCalls = nil
	if err := live.ReplyNative(context.Background(), replyContext{
		ConversationID: "cid-group-2",
	}, native); err != nil {
		t.Fatalf("ReplyNative via conversation error: %v", err)
	}
	if len(messenger.calls) == 0 {
		t.Fatalf("messenger.calls = %+v", messenger.calls)
	}

	if err := live.ReplyNative(context.Background(), replyContext{}, native); err == nil || !strings.Contains(err.Error(), "requires session webhook or conversation target") {
		t.Fatalf("missing target error = %v", err)
	}
}

func TestDingTalkLive_HelperFunctionsAndPayloadBuilders(t *testing.T) {
	rawCtx := replyContext{SessionWebhook: "https://session.example/reply", ConversationID: "cid-1", UserID: "staff-1"}
	if got := toReplyContext(rawCtx); got != rawCtx {
		t.Fatalf("toReplyContext(raw) = %+v", got)
	}
	if got := toReplyContext(&replyContext{ConversationID: "cid-2", UserID: "staff-2"}); got.ConversationID != "cid-2" || got.UserID != "staff-2" {
		t.Fatalf("toReplyContext(pointer) = %+v", got)
	}
	msg := &core.Message{ChatID: "cid-3", UserID: "staff-3"}
	if got := toReplyContext(msg); got.ConversationID != "cid-3" || got.UserID != "staff-3" {
		t.Fatalf("toReplyContext(message) = %+v", got)
	}
	if got := toReplyContext("invalid"); got != (replyContext{}) {
		t.Fatalf("toReplyContext(invalid) = %+v", got)
	}

	if _, err := resolveDirectSendTarget(""); err == nil || !strings.Contains(err.Error(), "requires target") {
		t.Fatalf("resolveDirectSendTarget(empty) err = %v", err)
	}
	if _, err := resolveDirectSendTarget("open-conversation:"); err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("resolveDirectSendTarget(open empty) err = %v", err)
	}
	if _, err := resolveDirectSendTarget("union:"); err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("resolveDirectSendTarget(union empty) err = %v", err)
	}
	if got, err := resolveDirectSendTarget("open-conversation:cid-group-1"); err != nil || got.OpenConversationID != "cid-group-1" {
		t.Fatalf("resolveDirectSendTarget(open) = %+v, %v", got, err)
	}
	if got, err := resolveDirectSendTarget("union:user-1"); err != nil || got.UnionID != "user-1" {
		t.Fatalf("resolveDirectSendTarget(union) = %+v, %v", got, err)
	}
	if got, err := resolveDirectSendTarget("cid-group-2"); err != nil || got.OpenConversationID != "cid-group-2" {
		t.Fatalf("resolveDirectSendTarget(cid) = %+v, %v", got, err)
	}
	if got, err := resolveDirectSendTarget("user-2"); err != nil || got.UnionID != "user-2" {
		t.Fatalf("resolveDirectSendTarget(default) = %+v, %v", got, err)
	}

	if got := parseUnixMillis(1710000000000); !got.Equal(time.UnixMilli(1710000000000)) {
		t.Fatalf("parseUnixMillis(valid) = %v", got)
	}
	before := time.Now()
	gotNow := parseUnixMillis(0)
	after := time.Now()
	if gotNow.Before(before) || gotNow.After(after.Add(time.Second)) {
		t.Fatalf("parseUnixMillis(now) = %v", gotNow)
	}

	payload, err := buildStandardCardData(core.NewCard().
		SetTitle("Review Ready").
		AddField("Status", "pending").
		AddPrimaryButton("Approve", "act:approve:review-1"))
	if err != nil {
		t.Fatalf("buildStandardCardData error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded["header"] == nil || decoded["contents"] == nil {
		t.Fatalf("decoded = %+v", decoded)
	}
	if _, err := buildStandardCardData(nil); err == nil {
		t.Fatal("expected nil card to fail")
	}

	nativePayload := &core.DingTalkCardPayload{
		Title:    "Review Ready",
		Markdown: "### Choose the next step",
		Buttons: []core.DingTalkCardButton{
			{Title: "Open", ActionURL: "https://example.test/reviews/1"},
			{Title: "Approve", ActionURL: "https://example.test/reviews/1?approve=1"},
		},
	}
	card := dingTalkNativeToCard(nativePayload)
	if card.Title != "Review Ready" || len(card.Fields) != 1 || len(card.Buttons) != 2 {
		t.Fatalf("card = %+v", card)
	}
	actionPayload := buildNativeActionCardPayload(nativePayload)
	if actionPayload["msgtype"] != "actionCard" {
		t.Fatalf("actionPayload = %+v", actionPayload)
	}
	if _, err := renderFormattedTextPayload(nil); err == nil {
		t.Fatal("expected nil formatted text to fail")
	}
	if _, err := renderFormattedTextPayload(&core.FormattedText{}); err == nil {
		t.Fatal("expected empty formatted text to fail")
	}
	if payload, err := renderFormattedTextPayload(&core.FormattedText{Content: "### Review Ready", Format: core.TextFormatDingTalkMD}); err != nil || payload["msgtype"] != "markdown" {
		t.Fatalf("renderFormattedTextPayload(markdown) = %+v, %v", payload, err)
	}
	if payload, err := renderFormattedTextPayload(&core.FormattedText{Content: "plain", Format: core.TextFormatPlainText}); err != nil || payload != nil {
		t.Fatalf("renderFormattedTextPayload(plain) = %+v, %v", payload, err)
	}
}
