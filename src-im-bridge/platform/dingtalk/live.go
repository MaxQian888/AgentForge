package dingtalk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dingtalkai "github.com/alibabacloud-go/dingtalk/ai_interaction_1_0"
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
		SupportsSlashCommands: true,
		SupportsMentions:      true,
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
}

type directMessenger interface {
	SendText(ctx context.Context, target directSendTarget, content string) error
}

type accessTokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

type LiveOption func(*Live) error

// Live is the DingTalk production adapter backed by Stream mode for inbound
// traffic and direct-send OpenAPI/webhook flows for outbound text delivery.
type Live struct {
	appKey    string
	appSecret string

	runner    streamRunner
	webhook   webhookReplier
	messenger directMessenger

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

func (l *Live) Name() string { return "dingtalk-live" }

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

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
				log.Printf("[dingtalk-live] Ignoring inbound message: %v", err)
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
			log.Printf("[dingtalk-live] Ignoring inbound message: %v", err)
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
	return l.Send(ctx, chatID, renderStructuredFallback(message))
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
