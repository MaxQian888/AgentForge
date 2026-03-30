package dingtalk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dingtalkai "github.com/alibabacloud-go/dingtalk/ai_interaction_1_0"
	dingtalkcardapi "github.com/alibabacloud-go/dingtalk/card_1_0"
	dingtalkim "github.com/alibabacloud-go/dingtalk/im_1_0"
	dingtalkoauth "github.com/alibabacloud-go/dingtalk/oauth2_1_0"
	teautil "github.com/alibabacloud-go/tea-utils/v2/service"
	dingtalkcard "github.com/open-dingtalk/dingtalk-stream-sdk-go/card"
	dingtalkchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dingtalkclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

var liveMetadata = core.PlatformMetadata{
	Source: "dingtalk",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:        core.CommandSurfaceMixed,
		StructuredSurface:     core.StructuredSurfaceActionCard,
		AsyncUpdateModes:      []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateSessionWebhook},
		ActionCallbackMode:    core.ActionCallbackWebhook,
		MessageScopes:         []core.MessageScope{core.MessageScopeChat},
		NativeSurfaces:        []string{core.NativeSurfaceDingTalkCard},
		SupportsRichMessages:  true,
		SupportsSlashCommands: true,
		SupportsMentions:      true,
	},
	Rendering: core.RenderingProfile{
		DefaultTextFormat: core.TextFormatPlainText,
		SupportedFormats:  []core.TextFormatMode{core.TextFormatPlainText, core.TextFormatDingTalkMD},
		NativeSurfaces:    []string{core.NativeSurfaceDingTalkCard},
		MaxTextLength:     20000,
		SupportsSegments:  true,
		StructuredSurface: core.StructuredSurfaceActionCard,
	},
}

type chatbotMessage struct {
	ConversationID    string
	ConversationType  string
	ConversationTitle string
	SenderID          string
	SenderStaffID     string
	SenderNick        string
	SessionWebhook    string
	Text              string
	CreatedAt         time.Time
}

type replyContext struct {
	SessionWebhook   string
	ConversationID   string
	ConversationType string
	UserID           string
}

type directSendTarget struct {
	OpenConversationID string
	UnionID            string
}

type streamRunner interface {
	Start(ctx context.Context, handler func(context.Context, chatbotMessage) error) error
	Stop(ctx context.Context) error
}

type cardActionStreamRunner interface {
	StartWithCardCallbacks(
		ctx context.Context,
		handler func(context.Context, chatbotMessage) error,
		cardHandler func(context.Context, *dingtalkcard.CardRequest) (*dingtalkcard.CardResponse, error),
	) error
}

type webhookReplier interface {
	ReplyText(ctx context.Context, sessionWebhook string, content string) error
	ReplyMessage(ctx context.Context, sessionWebhook string, requestBody map[string]any) error
}

type directMessenger interface {
	SendText(ctx context.Context, target directSendTarget, content string) error
}

type accessTokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

type robotCardSender interface {
	SendCard(ctx context.Context, target directSendTarget, card *core.Card) error
}

type advancedCardClient interface {
	CreateAndDeliverWithOptions(
		request *dingtalkcardapi.CreateAndDeliverRequest,
		headers *dingtalkcardapi.CreateAndDeliverHeaders,
		runtime *teautil.RuntimeOptions,
	) (*dingtalkcardapi.CreateAndDeliverResponse, error)
}

type LiveOption func(*Live) error

// Live is the DingTalk production adapter backed by Stream mode for inbound
// traffic and direct-send OpenAPI/webhook flows for outbound text delivery.
type Live struct {
	appKey    string
	appSecret string

	runner     streamRunner
	webhook    webhookReplier
	messenger  directMessenger
	cardSender robotCardSender

	actionHandler notify.ActionHandler

	startCtx    context.Context
	startCancel context.CancelFunc
	started     bool
	mu          sync.Mutex
}

func NewLive(appKey, appSecret string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(appKey) == "" || strings.TrimSpace(appSecret) == "" {
		return nil, errors.New("dingtalk live transport requires app key and app secret")
	}

	tokenProvider, err := newAccessTokenProvider(appKey, appSecret)
	if err != nil {
		return nil, err
	}
	messenger, err := newSDKDirectMessenger(tokenProvider)
	if err != nil {
		return nil, err
	}

	live := &Live{
		appKey:    appKey,
		appSecret: appSecret,
		runner:    newSDKStreamRunner(appKey, appSecret),
		webhook:   &sdkWebhookReplier{replier: dingtalkchatbot.NewChatbotReplier()},
		messenger: messenger,
		cardSender: &sdkRobotCardSender{
			robotCode:      strings.TrimSpace(appKey),
			tokenProvider:  tokenProvider,
			templateID:     "StandardCard",
			advancedClient: mustNewCardClient(),
			client:         mustNewIMClient(),
		},
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	if live.runner == nil {
		return nil, errors.New("dingtalk live transport requires a stream runner")
	}
	if live.webhook == nil {
		return nil, errors.New("dingtalk live transport requires a webhook replier")
	}
	if live.messenger == nil {
		return nil, errors.New("dingtalk live transport requires a direct messenger")
	}
	if live.cardSender == nil {
		return nil, errors.New("dingtalk live transport requires a robot card sender")
	}

	return live, nil
}

func WithStreamRunner(runner streamRunner) LiveOption {
	return func(live *Live) error {
		if runner == nil {
			return errors.New("stream runner cannot be nil")
		}
		live.runner = runner
		return nil
	}
}

func WithWebhookReplier(replier webhookReplier) LiveOption {
	return func(live *Live) error {
		if replier == nil {
			return errors.New("webhook replier cannot be nil")
		}
		live.webhook = replier
		return nil
	}
}

func WithDirectMessenger(messenger directMessenger) LiveOption {
	return func(live *Live) error {
		if messenger == nil {
			return errors.New("direct messenger cannot be nil")
		}
		live.messenger = messenger
		return nil
	}
}

func WithRobotCardSender(sender robotCardSender) LiveOption {
	return func(live *Live) error {
		if sender == nil {
			return errors.New("robot card sender cannot be nil")
		}
		live.cardSender = sender
		return nil
	}
}

func (l *Live) Name() string { return "dingtalk-live" }

func (l *Live) Metadata() core.PlatformMetadata {
	return core.NormalizeMetadata(liveMetadata, liveMetadata.Source)
}

func (l *Live) SetActionHandler(handler notify.ActionHandler) {
	l.actionHandler = handler
}

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	return replyContext{
		SessionWebhook:   strings.TrimSpace(target.SessionWebhook),
		ConversationID:   firstNonEmpty(target.ConversationID, target.ChatID, target.ChannelID),
		ConversationType: metadataValue(target.Metadata, "conversation_type"),
		UserID:           strings.TrimSpace(target.UserID),
	}
}

func (l *Live) Start(handler core.MessageHandler) error {
	if handler == nil {
		return errors.New("message handler is required")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.started {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	l.startCtx = ctx
	l.startCancel = cancel

	if runner, ok := l.runner.(cardActionStreamRunner); ok && l.actionHandler != nil {
		if err := runner.StartWithCardCallbacks(ctx, func(ctx context.Context, incoming chatbotMessage) error {
			msg, err := normalizeIncomingMessage(incoming)
			if err != nil {
				log.WithField("component", "dingtalk-live").WithError(err).Warn("Ignoring inbound message")
				return nil
			}
			handler(l, msg)
			return nil
		}, l.handleCardAction); err != nil {
			cancel()
			return err
		}
		l.started = true
		return nil
	}

	if err := l.runner.Start(ctx, func(ctx context.Context, incoming chatbotMessage) error {
		msg, err := normalizeIncomingMessage(incoming)
		if err != nil {
			log.WithField("component", "dingtalk-live").WithError(err).Warn("Ignoring inbound message")
			return nil
		}
		handler(l, msg)
		return nil
	}); err != nil {
		cancel()
		return err
	}

	l.started = true
	return nil
}

func (l *Live) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	if reply.SessionWebhook != "" {
		return l.webhook.ReplyText(ctx, reply.SessionWebhook, content)
	}

	target, err := resolveDirectSendTarget(reply.ConversationID)
	if err != nil {
		return errors.New("dingtalk reply requires session webhook or conversation target")
	}
	return l.messenger.SendText(ctx, target, content)
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	target := strings.TrimSpace(chatID)
	if target == "" {
		return errors.New("dingtalk send requires target")
	}
	if isWebhookTarget(target) {
		return l.webhook.ReplyText(ctx, target, content)
	}

	sendTarget, err := resolveDirectSendTarget(target)
	if err != nil {
		return err
	}
	return l.messenger.SendText(ctx, sendTarget, content)
}

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	if message != nil && len(message.Sections) > 0 {
		payload := renderStructuredSections(message.Sections)
		if title := strings.TrimSpace(message.Title); title != "" {
			payload.Title = title
		}
		native, err := core.NewDingTalkCardMessage(payload.CardType, payload.Title, payload.Markdown, payload.Buttons)
		if err == nil {
			return l.SendNative(ctx, chatID, native)
		}
	}
	return l.Send(ctx, chatID, renderStructuredFallback(message))
}

func (l *Live) ReplyStructured(ctx context.Context, rawReplyCtx any, message *core.StructuredMessage) error {
	if message != nil && len(message.Sections) > 0 {
		payload := renderStructuredSections(message.Sections)
		if title := strings.TrimSpace(message.Title); title != "" {
			payload.Title = title
		}
		native := &core.NativeMessage{Platform: "dingtalk", DingTalkCard: payload}
		if err := native.Validate(); err == nil {
			return l.ReplyNative(ctx, rawReplyCtx, native)
		}
	}
	return l.Reply(ctx, rawReplyCtx, renderStructuredFallback(message))
}

func (l *Live) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	if isWebhookTarget(chatID) {
		if err := l.sendActionCardViaWebhook(ctx, chatID, card); err != nil {
			if fallbackErr := l.deliverCardFallback(ctx, chatID, nil, card); fallbackErr != nil {
				return fallbackErr
			}
			return err
		}
		return nil
	}
	target, err := resolveDirectSendTarget(chatID)
	if err == nil && l.cardSender != nil {
		if err := l.cardSender.SendCard(ctx, target, card); err == nil {
			return nil
		}
	}
	return l.deliverCardFallback(ctx, chatID, nil, card)
}

func (l *Live) ReplyCard(ctx context.Context, rawReplyCtx any, card *core.Card) error {
	reply := toReplyContext(rawReplyCtx)
	if reply.SessionWebhook != "" {
		if err := l.sendActionCardViaWebhook(ctx, reply.SessionWebhook, card); err != nil {
			if fallbackErr := l.deliverCardFallback(ctx, reply.ConversationID, rawReplyCtx, card); fallbackErr != nil {
				return fallbackErr
			}
			return err
		}
		return nil
	}
	target, err := resolveDirectSendTarget(reply.ConversationID)
	if err == nil && l.cardSender != nil {
		if err := l.cardSender.SendCard(ctx, target, card); err == nil {
			return nil
		}
	}
	return l.deliverCardFallback(ctx, reply.ConversationID, rawReplyCtx, card)
}

func (l *Live) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	if message.NormalizedPlatform() != "dingtalk" || message.DingTalkCard == nil {
		return errors.New("native message is not a dingtalk card payload")
	}
	if isWebhookTarget(chatID) {
		return l.webhook.ReplyMessage(ctx, chatID, buildNativeActionCardPayload(message.DingTalkCard))
	}
	target, err := resolveDirectSendTarget(chatID)
	if err != nil {
		return err
	}
	if l.cardSender != nil {
		if err := l.cardSender.SendCard(ctx, target, dingTalkNativeToCard(message.DingTalkCard)); err == nil {
			return nil
		}
	}
	return l.Send(ctx, chatID, message.FallbackText())
}

func (l *Live) ReplyNative(ctx context.Context, rawReplyCtx any, message *core.NativeMessage) error {
	reply := toReplyContext(rawReplyCtx)
	if err := message.Validate(); err != nil {
		return err
	}
	if message.NormalizedPlatform() != "dingtalk" || message.DingTalkCard == nil {
		return errors.New("native message is not a dingtalk card payload")
	}
	if reply.SessionWebhook != "" {
		return l.webhook.ReplyMessage(ctx, reply.SessionWebhook, buildNativeActionCardPayload(message.DingTalkCard))
	}
	if reply.ConversationID != "" {
		return l.SendNative(ctx, reply.ConversationID, message)
	}
	return errors.New("dingtalk reply requires session webhook or conversation target")
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	outgoing, err := renderFormattedTextPayload(message)
	if err != nil {
		return err
	}
	if isWebhookTarget(chatID) && outgoing != nil {
		return l.webhook.ReplyMessage(ctx, chatID, outgoing)
	}
	return l.Send(ctx, chatID, strings.TrimSpace(message.Content))
}

func (l *Live) ReplyFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	reply := toReplyContext(rawReplyCtx)
	outgoing, err := renderFormattedTextPayload(message)
	if err != nil {
		return err
	}
	if reply.SessionWebhook != "" && outgoing != nil {
		return l.webhook.ReplyMessage(ctx, reply.SessionWebhook, outgoing)
	}
	return l.Reply(ctx, rawReplyCtx, strings.TrimSpace(message.Content))
}

func (l *Live) UpdateFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	return l.ReplyFormattedText(ctx, rawReplyCtx, message)
}

func (l *Live) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.started {
		return nil
	}
	if l.startCancel != nil {
		l.startCancel()
	}
	l.started = false
	return l.runner.Stop(l.startCtx)
}

type sdkStreamRunner struct {
	client *dingtalkclient.StreamClient
}

func newSDKStreamRunner(appKey, appSecret string) *sdkStreamRunner {
	return &sdkStreamRunner{
		client: dingtalkclient.NewStreamClient(
			dingtalkclient.WithAppCredential(
				dingtalkclient.NewAppCredentialConfig(appKey, appSecret),
			),
		),
	}
}

func (r *sdkStreamRunner) Start(ctx context.Context, handler func(context.Context, chatbotMessage) error) error {
	return r.StartWithCardCallbacks(ctx, handler, nil)
}

func (r *sdkStreamRunner) StartWithCardCallbacks(ctx context.Context, handler func(context.Context, chatbotMessage) error, cardHandler func(context.Context, *dingtalkcard.CardRequest) (*dingtalkcard.CardResponse, error)) error {
	r.client.RegisterChatBotCallbackRouter(func(ctx context.Context, data *dingtalkchatbot.BotCallbackDataModel) ([]byte, error) {
		return []byte(""), handler(ctx, chatbotMessage{
			ConversationID:    strings.TrimSpace(data.ConversationId),
			ConversationType:  strings.TrimSpace(data.ConversationType),
			ConversationTitle: strings.TrimSpace(data.ConversationTitle),
			SenderID:          strings.TrimSpace(data.SenderId),
			SenderStaffID:     strings.TrimSpace(data.SenderStaffId),
			SenderNick:        strings.TrimSpace(data.SenderNick),
			SessionWebhook:    strings.TrimSpace(data.SessionWebhook),
			Text:              strings.TrimSpace(data.Text.Content),
			CreatedAt:         parseUnixMillis(data.CreateAt),
		})
	})
	if cardHandler != nil {
		r.client.RegisterCardCallbackRouter(cardHandler)
	}
	return r.client.Start(ctx)
}

func (r *sdkStreamRunner) Stop(context.Context) error {
	r.client.Close()
	return nil
}

type sdkWebhookReplier struct {
	replier *dingtalkchatbot.ChatbotReplier
}

func (r *sdkWebhookReplier) ReplyText(ctx context.Context, sessionWebhook string, content string) error {
	if strings.TrimSpace(sessionWebhook) == "" {
		return errors.New("session webhook is required")
	}
	return r.replier.SimpleReplyText(ctx, sessionWebhook, []byte(content))
}

func (r *sdkWebhookReplier) ReplyMessage(ctx context.Context, sessionWebhook string, requestBody map[string]any) error {
	if strings.TrimSpace(sessionWebhook) == "" {
		return errors.New("session webhook is required")
	}
	return r.replier.ReplyMessage(ctx, sessionWebhook, requestBody)
}

type sdkDirectMessenger struct {
	client        *dingtalkai.Client
	tokenProvider accessTokenProvider
}

func newSDKDirectMessenger(tokenProvider accessTokenProvider) (*sdkDirectMessenger, error) {
	client, err := dingtalkai.NewClient(&openapi.Config{})
	if err != nil {
		return nil, fmt.Errorf("create dingtalk ai interaction client: %w", err)
	}
	return &sdkDirectMessenger{
		client:        client,
		tokenProvider: tokenProvider,
	}, nil
}

func (m *sdkDirectMessenger) SendText(ctx context.Context, target directSendTarget, content string) error {
	token, err := m.tokenProvider.AccessToken(ctx)
	if err != nil {
		return err
	}

	request := (&dingtalkai.SendRequest{}).
		SetContent(content).
		SetContentType("text")
	if target.OpenConversationID != "" {
		request.SetOpenConversationId(target.OpenConversationID)
	}
	if target.UnionID != "" {
		request.SetUnionId(target.UnionID)
	}
	if target.OpenConversationID == "" && target.UnionID == "" {
		return errors.New("dingtalk direct send requires conversation id or union id")
	}

	headers := (&dingtalkai.SendHeaders{}).SetXAcsDingtalkAccessToken(token)
	resp, err := m.client.SendWithOptions(request, headers, &teautil.RuntimeOptions{})
	if err != nil {
		return err
	}
	if resp == nil || resp.Body == nil || resp.Body.Success == nil || !*resp.Body.Success {
		return errors.New("dingtalk direct send failed")
	}
	return nil
}

type sdkRobotCardSender struct {
	client             *dingtalkim.Client
	advancedClient     advancedCardClient
	tokenProvider      accessTokenProvider
	robotCode          string
	templateID         string
	advancedTemplateID string
}

func mustNewIMClient() *dingtalkim.Client {
	client, err := dingtalkim.NewClient(&openapi.Config{})
	if err != nil {
		panic(fmt.Errorf("create dingtalk im client: %w", err))
	}
	return client
}

func mustNewCardClient() *dingtalkcardapi.Client {
	client, err := dingtalkcardapi.NewClient(&openapi.Config{})
	if err != nil {
		panic(fmt.Errorf("create dingtalk card client: %w", err))
	}
	return client
}

func WithAdvancedCardTemplate(templateID string) LiveOption {
	return func(live *Live) error {
		sender, ok := live.cardSender.(*sdkRobotCardSender)
		if !ok {
			return errors.New("advanced card template requires sdk robot card sender")
		}
		sender.advancedTemplateID = strings.TrimSpace(templateID)
		return nil
	}
}

func (s *sdkRobotCardSender) SendCard(ctx context.Context, target directSendTarget, card *core.Card) error {
	if strings.TrimSpace(s.advancedTemplateID) != "" {
		if err := s.sendAdvancedCard(ctx, target, card); err == nil {
			return nil
		}
	}
	token, err := s.tokenProvider.AccessToken(ctx)
	if err != nil {
		return err
	}
	cardData, err := buildStandardCardData(card)
	if err != nil {
		return err
	}
	request := (&dingtalkim.SendRobotInteractiveCardRequest{}).
		SetCardTemplateId(s.templateID).
		SetCardBizId(cardBizID(card)).
		SetCardData(cardData).
		SetRobotCode(strings.TrimSpace(s.robotCode)).
		SetPullStrategy(false)
	switch {
	case strings.TrimSpace(target.OpenConversationID) != "":
		request.SetOpenConversationId(strings.TrimSpace(target.OpenConversationID))
	case strings.TrimSpace(target.UnionID) != "":
		receiver, marshalErr := json.Marshal(map[string]string{"unionId": strings.TrimSpace(target.UnionID)})
		if marshalErr != nil {
			return marshalErr
		}
		request.SetSingleChatReceiver(string(receiver))
	default:
		return errors.New("dingtalk robot card send requires conversation id or union id")
	}

	headers := (&dingtalkim.SendRobotInteractiveCardHeaders{}).
		SetXAcsDingtalkAccessToken(token)
	resp, err := s.client.SendRobotInteractiveCardWithOptions(request, headers, &teautil.RuntimeOptions{})
	if err != nil {
		return err
	}
	if resp == nil || resp.StatusCode == nil || *resp.StatusCode >= httpStatusBadRequest {
		return errors.New("dingtalk interactive card send failed")
	}
	return nil
}

func (s *sdkRobotCardSender) sendAdvancedCard(ctx context.Context, target directSendTarget, card *core.Card) error {
	if s.advancedClient == nil {
		return errors.New("dingtalk advanced card client is unavailable")
	}
	token, err := s.tokenProvider.AccessToken(ctx)
	if err != nil {
		return err
	}
	cardData := &dingtalkcardapi.CreateAndDeliverRequestCardData{
		CardParamMap: make(map[string]*string),
	}
	for key, value := range buildAdvancedCardParams(card) {
		cloned := value
		cardData.CardParamMap[key] = &cloned
	}

	request := (&dingtalkcardapi.CreateAndDeliverRequest{}).
		SetCardTemplateId(strings.TrimSpace(s.advancedTemplateID)).
		SetOutTrackId(cardBizID(card)).
		SetCallbackType("STREAM").
		SetCardData(cardData)

	switch {
	case strings.TrimSpace(target.OpenConversationID) != "":
		request.SetOpenSpaceId("dtv1.card//IM_GROUP." + strings.TrimSpace(target.OpenConversationID))
		request.SetImGroupOpenSpaceModel((&dingtalkcardapi.CreateAndDeliverRequestImGroupOpenSpaceModel{}).SetSupportForward(true))
		request.SetImGroupOpenDeliverModel((&dingtalkcardapi.CreateAndDeliverRequestImGroupOpenDeliverModel{}).
			SetExtension(map[string]*string{}).
			SetRobotCode(strings.TrimSpace(s.robotCode)))
	case strings.TrimSpace(target.UnionID) != "":
		request.SetOpenSpaceId("dtv1.card//IM_SINGLE." + strings.TrimSpace(target.UnionID))
		request.SetUserId(strings.TrimSpace(target.UnionID))
		request.SetUserIdType(2)
		request.SetImSingleOpenSpaceModel((&dingtalkcardapi.CreateAndDeliverRequestImSingleOpenSpaceModel{}).SetSupportForward(false))
		request.SetImSingleOpenDeliverModel((&dingtalkcardapi.CreateAndDeliverRequestImSingleOpenDeliverModel{}).
			SetExtension(map[string]*string{}))
	default:
		return errors.New("dingtalk advanced card send requires conversation id or union id")
	}

	headers := (&dingtalkcardapi.CreateAndDeliverHeaders{}).
		SetXAcsDingtalkAccessToken(token)
	resp, err := s.advancedClient.CreateAndDeliverWithOptions(request, headers, &teautil.RuntimeOptions{})
	if err != nil {
		return err
	}
	if resp == nil || resp.StatusCode == nil || *resp.StatusCode >= httpStatusBadRequest {
		return errors.New("dingtalk advanced interactive card send failed")
	}
	return nil
}

type cachedAccessTokenProvider struct {
	client    *dingtalkoauth.Client
	appKey    string
	appSecret string

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func newAccessTokenProvider(appKey, appSecret string) (*cachedAccessTokenProvider, error) {
	client, err := dingtalkoauth.NewClient(&openapi.Config{})
	if err != nil {
		return nil, err
	}
	return &cachedAccessTokenProvider{
		client:    client,
		appKey:    appKey,
		appSecret: appSecret,
	}, nil
}

func (p *cachedAccessTokenProvider) AccessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token != "" && time.Now().Before(p.expiresAt.Add(-time.Minute)) {
		return p.token, nil
	}

	request := (&dingtalkoauth.GetAccessTokenRequest{}).
		SetAppKey(p.appKey).
		SetAppSecret(p.appSecret)
	resp, err := p.client.GetAccessTokenWithOptions(request, map[string]*string{}, &teautil.RuntimeOptions{})
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Body == nil || resp.Body.AccessToken == nil || strings.TrimSpace(*resp.Body.AccessToken) == "" {
		return "", errors.New("dingtalk access token response missing token")
	}

	p.token = *resp.Body.AccessToken
	expireIn := int64(7200)
	if resp.Body.ExpireIn != nil && *resp.Body.ExpireIn > 0 {
		expireIn = *resp.Body.ExpireIn
	}
	p.expiresAt = time.Now().Add(time.Duration(expireIn) * time.Second)
	return p.token, nil
}

func normalizeIncomingMessage(incoming chatbotMessage) (*core.Message, error) {
	conversationID := strings.TrimSpace(incoming.ConversationID)
	if conversationID == "" {
		return nil, errors.New("missing dingtalk conversation id")
	}
	userID := strings.TrimSpace(incoming.SenderStaffID)
	if userID == "" {
		userID = strings.TrimSpace(incoming.SenderID)
	}
	if userID == "" {
		return nil, errors.New("missing dingtalk sender id")
	}
	content := strings.TrimSpace(incoming.Text)
	if content == "" {
		return nil, errors.New("missing dingtalk text content")
	}

	return &core.Message{
		Platform:   liveMetadata.Source,
		SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, conversationID, userID),
		UserID:     userID,
		UserName:   strings.TrimSpace(incoming.SenderNick),
		ChatID:     conversationID,
		ChatName:   strings.TrimSpace(incoming.ConversationTitle),
		Content:    content,
		ReplyCtx: replyContext{
			SessionWebhook:   strings.TrimSpace(incoming.SessionWebhook),
			ConversationID:   conversationID,
			ConversationType: strings.TrimSpace(incoming.ConversationType),
			UserID:           userID,
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:       liveMetadata.Source,
			ChatID:         conversationID,
			ChannelID:      conversationID,
			ConversationID: conversationID,
			SessionWebhook: strings.TrimSpace(incoming.SessionWebhook),
			UserID:         userID,
			UseReply:       true,
			Metadata: map[string]string{
				"conversation_type": strings.TrimSpace(incoming.ConversationType),
			},
		},
		Timestamp: incoming.CreatedAt,
		IsGroup:   strings.TrimSpace(incoming.ConversationType) == "2",
	}, nil
}

func (l *Live) handleCardAction(ctx context.Context, request *dingtalkcard.CardRequest) (*dingtalkcard.CardResponse, error) {
	req, err := normalizeCardActionRequest(request)
	if err != nil {
		return &dingtalkcard.CardResponse{}, err
	}
	if req == nil || l.actionHandler == nil {
		return &dingtalkcard.CardResponse{}, nil
	}

	result, err := l.actionHandler.HandleAction(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil || strings.TrimSpace(result.Result) == "" {
		return &dingtalkcard.CardResponse{}, nil
	}

	target := req.ReplyTarget
	if result.ReplyTarget != nil {
		target = result.ReplyTarget
	}
	_, err = core.DeliverText(ctx, l, l.Metadata(), target, req.ChatID, result.Result)
	if err != nil {
		return nil, err
	}
	return &dingtalkcard.CardResponse{}, nil
}

func normalizeCardActionRequest(request *dingtalkcard.CardRequest) (*notify.ActionRequest, error) {
	if request == nil {
		return nil, errors.New("missing dingtalk card callback payload")
	}

	actionRef := firstNonEmpty(
		request.GetActionString("action"),
		request.GetActionString("action_id"),
		firstActionID(request.CardActionData.CardPrivateData.ActionIdList),
	)
	action, entityID, ok := core.ParseActionReference(actionRef)
	if !ok {
		return nil, errors.New("missing dingtalk action reference")
	}

	callbackCtx := parseCardCallbackContext(request)
	chatID := firstNonEmpty(callbackCtx.ConversationID, strings.TrimSpace(request.SpaceId))
	replyTarget := &core.ReplyTarget{
		Platform:          liveMetadata.Source,
		ChatID:            chatID,
		ChannelID:         chatID,
		ConversationID:    chatID,
		SessionWebhook:    callbackCtx.SessionWebhook,
		UserID:            firstNonEmpty(strings.TrimSpace(request.UserId), callbackCtx.UserID),
		UseReply:          true,
		PreferredRenderer: string(liveMetadata.Capabilities.StructuredSurface),
	}
	metadata := compactMetadata(map[string]string{
		"source":            "card_callback",
		"space_type":        firstNonEmpty(strings.TrimSpace(request.SpaceType), callbackCtx.ConversationType),
		"out_track_id":      strings.TrimSpace(request.OutTrackId),
		"conversation_type": callbackCtx.ConversationType,
	})
	if len(metadata) > 0 {
		replyTarget.Metadata = map[string]string{
			"conversation_type": metadata["conversation_type"],
		}
	}

	return &notify.ActionRequest{
		Platform:    liveMetadata.Source,
		Action:      action,
		EntityID:    entityID,
		ChatID:      chatID,
		UserID:      firstNonEmpty(strings.TrimSpace(request.UserId), callbackCtx.UserID),
		ReplyTarget: replyTarget,
		Metadata:    metadata,
	}, nil
}

func toReplyContext(raw any) replyContext {
	switch value := raw.(type) {
	case replyContext:
		return value
	case *replyContext:
		if value == nil {
			return replyContext{}
		}
		return *value
	case *core.Message:
		if value == nil {
			return replyContext{}
		}
		if ctx, ok := value.ReplyCtx.(replyContext); ok {
			return ctx
		}
		if ctx, ok := value.ReplyCtx.(*replyContext); ok && ctx != nil {
			return *ctx
		}
		return replyContext{
			ConversationID: value.ChatID,
			UserID:         value.UserID,
		}
	default:
		return replyContext{}
	}
}

func resolveDirectSendTarget(raw string) (directSendTarget, error) {
	target := strings.TrimSpace(raw)
	switch {
	case target == "":
		return directSendTarget{}, errors.New("dingtalk send requires target")
	case strings.HasPrefix(target, "open-conversation:"):
		id := strings.TrimSpace(strings.TrimPrefix(target, "open-conversation:"))
		if id == "" {
			return directSendTarget{}, errors.New("dingtalk open conversation id cannot be empty")
		}
		return directSendTarget{OpenConversationID: id}, nil
	case strings.HasPrefix(target, "union:"):
		id := strings.TrimSpace(strings.TrimPrefix(target, "union:"))
		if id == "" {
			return directSendTarget{}, errors.New("dingtalk union id cannot be empty")
		}
		return directSendTarget{UnionID: id}, nil
	case strings.HasPrefix(target, "cid"):
		return directSendTarget{OpenConversationID: target}, nil
	default:
		return directSendTarget{UnionID: target}, nil
	}
}

func isWebhookTarget(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && parsed != nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func metadataValue(metadata map[string]string, key string) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(metadata[key])
}

func parseUnixMillis(raw int64) time.Time {
	if raw <= 0 {
		return time.Now()
	}
	return time.UnixMilli(raw)
}

type cardCallbackContext struct {
	SessionWebhook   string `json:"sessionWebhook"`
	ConversationID   string `json:"conversationId"`
	ConversationType string `json:"conversationType"`
	UserID           string `json:"userId"`
}

func parseCardCallbackContext(request *dingtalkcard.CardRequest) cardCallbackContext {
	if request == nil {
		return cardCallbackContext{}
	}

	ctx := cardCallbackContext{
		ConversationID:   strings.TrimSpace(request.SpaceId),
		ConversationType: strings.TrimSpace(request.SpaceType),
		UserID:           strings.TrimSpace(request.UserId),
	}
	if extension := strings.TrimSpace(request.Extension); extension != "" {
		var decoded cardCallbackContext
		if err := json.Unmarshal([]byte(extension), &decoded); err == nil {
			ctx.SessionWebhook = firstNonEmpty(strings.TrimSpace(decoded.SessionWebhook), ctx.SessionWebhook)
			ctx.ConversationID = firstNonEmpty(strings.TrimSpace(decoded.ConversationID), ctx.ConversationID)
			ctx.ConversationType = firstNonEmpty(strings.TrimSpace(decoded.ConversationType), ctx.ConversationType)
			ctx.UserID = firstNonEmpty(strings.TrimSpace(decoded.UserID), ctx.UserID)
		}
	}
	if params := request.CardActionData.CardPrivateData.Params; len(params) > 0 {
		ctx.SessionWebhook = firstNonEmpty(stringParam(params, "session_webhook"), stringParam(params, "sessionWebhook"), ctx.SessionWebhook)
		ctx.ConversationID = firstNonEmpty(stringParam(params, "conversation_id"), stringParam(params, "conversationId"), ctx.ConversationID)
		ctx.ConversationType = firstNonEmpty(stringParam(params, "conversation_type"), stringParam(params, "conversationType"), ctx.ConversationType)
		ctx.UserID = firstNonEmpty(stringParam(params, "user_id"), stringParam(params, "userId"), ctx.UserID)
	}
	return ctx
}

func renderStructuredFallback(message *core.StructuredMessage) string {
	if message == nil {
		return ""
	}
	content := strings.TrimSpace(message.FallbackText())
	if content == "" {
		return ""
	}
	if len(message.Actions) > 0 {
		content += "\n\nDingTalk ActionCard 暂未启用，已降级为文本。"
	}
	return content
}

func buildNativeActionCardPayload(payload *core.DingTalkCardPayload) map[string]any {
	actionCard := map[string]any{
		"title":          strings.TrimSpace(payload.Title),
		"text":           strings.TrimSpace(payload.Markdown),
		"btnOrientation": "0",
	}
	if len(payload.Buttons) == 1 {
		actionCard["singleTitle"] = strings.TrimSpace(payload.Buttons[0].Title)
		actionCard["singleURL"] = strings.TrimSpace(payload.Buttons[0].ActionURL)
	} else if len(payload.Buttons) > 1 {
		buttons := make([]map[string]string, 0, len(payload.Buttons))
		for _, button := range payload.Buttons {
			buttons = append(buttons, map[string]string{
				"title":     strings.TrimSpace(button.Title),
				"actionURL": strings.TrimSpace(button.ActionURL),
			})
		}
		actionCard["btns"] = buttons
	}
	return map[string]any{
		"msgtype":    "actionCard",
		"actionCard": actionCard,
	}
}

func dingTalkNativeToCard(payload *core.DingTalkCardPayload) *core.Card {
	card := core.NewCard().SetTitle(strings.TrimSpace(payload.Title))
	if body := strings.TrimSpace(payload.Markdown); body != "" {
		card.AddField("Body", body)
	}
	for _, button := range payload.Buttons {
		card.AddButton(strings.TrimSpace(button.Title), "link:"+strings.TrimSpace(button.ActionURL))
	}
	return card
}

func renderFormattedTextPayload(message *core.FormattedText) (map[string]any, error) {
	if message == nil {
		return nil, errors.New("formatted text is required")
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return nil, errors.New("formatted text content is required")
	}
	switch message.Format {
	case core.TextFormatDingTalkMD:
		return map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]any{
				"title": "AgentForge Update",
				"text":  content,
			},
		}, nil
	default:
		return nil, nil
	}
}

type deliveredFallbackError struct {
	reason string
}

func (e deliveredFallbackError) Error() string {
	return e.reason
}

func (e deliveredFallbackError) FallbackReason() string {
	return e.reason
}

func (e deliveredFallbackError) FallbackDelivered() bool {
	return true
}

func (l *Live) sendActionCardViaWebhook(ctx context.Context, sessionWebhook string, card *core.Card) error {
	payload, err := buildActionCardPayload(card)
	if err != nil {
		return err
	}
	return l.webhook.ReplyMessage(ctx, sessionWebhook, payload)
}

func (l *Live) deliverCardFallback(ctx context.Context, chatID string, rawReplyCtx any, card *core.Card) error {
	fallbackText := cardFallbackText(card)
	if strings.TrimSpace(fallbackText) == "" {
		fallbackText = "DingTalk ActionCard send failed."
	}
	if rawReplyCtx != nil {
		if err := l.Reply(ctx, rawReplyCtx, fallbackText); err != nil {
			return err
		}
		return deliveredFallbackError{reason: "actioncard_send_failed"}
	}
	if err := l.Send(ctx, chatID, fallbackText); err != nil {
		return err
	}
	return deliveredFallbackError{reason: "actioncard_send_failed"}
}

func buildActionCardPayload(card *core.Card) (map[string]any, error) {
	if card == nil {
		return nil, errors.New("card is required")
	}
	if len(card.Buttons) == 0 {
		return nil, errors.New("action card requires at least one button")
	}

	markdownText := strings.TrimSpace(cardMarkdown(card))
	if markdownText == "" {
		return nil, errors.New("action card requires non-empty content")
	}

	actionCard := map[string]any{
		"title":          strings.TrimSpace(card.Title),
		"text":           markdownText,
		"btnOrientation": "0",
	}
	if len(card.Buttons) == 1 {
		url, ok := resolveCardButtonURL(card.Buttons[0])
		if !ok {
			return nil, errors.New("action card button requires a URL")
		}
		actionCard["singleTitle"] = strings.TrimSpace(card.Buttons[0].Text)
		actionCard["singleURL"] = url
	} else {
		buttons := make([]map[string]string, 0, len(card.Buttons))
		for _, button := range card.Buttons {
			url, ok := resolveCardButtonURL(button)
			if !ok {
				return nil, errors.New("action card buttons require URLs")
			}
			buttons = append(buttons, map[string]string{
				"title":     strings.TrimSpace(button.Text),
				"actionURL": url,
			})
		}
		actionCard["btns"] = buttons
	}

	return map[string]any{
		"msgtype":    "actionCard",
		"actionCard": actionCard,
	}, nil
}

const httpStatusBadRequest int32 = 400

func buildStandardCardData(card *core.Card) (string, error) {
	if card == nil {
		return "", errors.New("card is required")
	}
	payload := map[string]any{
		"config": map[string]any{
			"autoLayout":    true,
			"enableForward": true,
		},
		"header": map[string]any{
			"title": map[string]string{
				"type": "text",
				"text": strings.TrimSpace(card.Title),
			},
		},
		"contents": []map[string]any{
			{
				"type": "markdown",
				"text": cardStandardContent(card),
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func buildAdvancedCardParams(card *core.Card) map[string]string {
	params := map[string]string{
		"title": strings.TrimSpace(card.Title),
		"body":  cardStandardContent(card),
	}
	for index, button := range card.Buttons {
		if index >= 3 {
			break
		}
		slot := index + 1
		params[fmt.Sprintf("action_%d_label", slot)] = strings.TrimSpace(button.Text)
		params[fmt.Sprintf("action_%d_ref", slot)] = strings.TrimSpace(button.Action)
		if url, ok := resolveCardButtonURL(button); ok {
			params[fmt.Sprintf("action_%d_url", slot)] = url
		}
	}
	return params
}

func cardStandardContent(card *core.Card) string {
	lines := make([]string, 0, len(card.Fields)+len(card.Buttons)+1)
	for _, field := range card.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		switch {
		case label == "" && value == "":
			continue
		case label == "":
			lines = append(lines, value)
		default:
			lines = append(lines, fmt.Sprintf("**%s**: %s", label, value))
		}
	}
	for _, button := range card.Buttons {
		label := strings.TrimSpace(button.Text)
		if label == "" {
			continue
		}
		if url, ok := resolveCardButtonURL(button); ok {
			lines = append(lines, fmt.Sprintf("- [%s](%s)", label, url))
			continue
		}
		lines = append(lines, "- "+label)
	}
	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

func cardBizID(card *core.Card) string {
	title := strings.TrimSpace(card.Title)
	if title == "" {
		title = "agentforge"
	}
	sanitized := strings.NewReplacer(" ", "-", "/", "-", "\\", "-").Replace(strings.ToLower(title))
	return fmt.Sprintf("af-%s-%d", sanitized, time.Now().UnixNano())
}

func resolveCardButtonURL(button core.CardButton) (string, bool) {
	action := strings.TrimSpace(button.Action)
	switch {
	case strings.HasPrefix(action, "link:"):
		url := strings.TrimSpace(strings.TrimPrefix(action, "link:"))
		return url, url != ""
	case strings.HasPrefix(action, "http://"), strings.HasPrefix(action, "https://"):
		return action, true
	default:
		return "", false
	}
}

func cardMarkdown(card *core.Card) string {
	if card == nil {
		return ""
	}
	lines := make([]string, 0, 1+len(card.Fields))
	if title := strings.TrimSpace(card.Title); title != "" {
		lines = append(lines, "### "+title)
	}
	for _, field := range card.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		switch {
		case label == "" && value == "":
			continue
		case label == "":
			lines = append(lines, value)
		default:
			lines = append(lines, fmt.Sprintf("**%s**: %s", label, value))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n\n"))
}

func cardFallbackText(card *core.Card) string {
	if card == nil {
		return ""
	}
	lines := make([]string, 0, 1+len(card.Fields)+len(card.Buttons))
	if title := strings.TrimSpace(card.Title); title != "" {
		lines = append(lines, title)
	}
	for _, field := range card.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		switch {
		case label == "" && value == "":
			continue
		case label == "":
			lines = append(lines, value)
		default:
			lines = append(lines, label+": "+value)
		}
	}
	for _, button := range card.Buttons {
		label := strings.TrimSpace(button.Text)
		action := strings.TrimSpace(button.Action)
		switch {
		case label != "" && strings.HasPrefix(action, "link:"):
			lines = append(lines, label+": "+strings.TrimSpace(strings.TrimPrefix(action, "link:")))
		case label != "":
			lines = append(lines, label)
		}
	}
	lines = append(lines, "DingTalk ActionCard 暂未完全支持，已降级为文本。")
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func firstActionID(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringParam(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	raw, ok := values[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func compactMetadata(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	metadata := make(map[string]string, len(values))
	for key, value := range values {
		if trimmedKey := strings.TrimSpace(key); trimmedKey != "" {
			if trimmedValue := strings.TrimSpace(value); trimmedValue != "" {
				metadata[trimmedKey] = trimmedValue
			}
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}
