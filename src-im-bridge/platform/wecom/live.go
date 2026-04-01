package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/im-bridge/core"
)

var liveMetadata = core.NormalizeMetadata(core.PlatformMetadata{
	Source: "wecom",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:         core.CommandSurfaceMixed,
		StructuredSurface:      core.StructuredSurfaceCards,
		AsyncUpdateModes:       []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateSessionWebhook},
		ActionCallbackMode:     core.ActionCallbackWebhook,
		MessageScopes:          []core.MessageScope{core.MessageScopeChat},
		NativeSurfaces:         []string{core.NativeSurfaceWeComCard},
		RequiresPublicCallback: true,
		SupportsRichMessages:   true,
		SupportsSlashCommands:  true,
		SupportsMentions:       true,
	},
	Rendering: core.RenderingProfile{
		DefaultTextFormat:         core.TextFormatPlainText,
		SupportedFormats:          []core.TextFormatMode{core.TextFormatPlainText, core.TextFormatWeComMD},
		NativeSurfaces:            []string{core.NativeSurfaceWeComCard},
		MaxTextLength:             4096,
		SupportsSegments:          true,
		StructuredSurface:         core.StructuredSurfaceCards,
		UsesProviderOwnedBuilders: true,
	},
}, "wecom")

type callbackMessage struct {
	MsgID       string         `json:"msgid"`
	ChatID      string         `json:"chatid"`
	ChatType    string         `json:"chattype"`
	ResponseURL string         `json:"response_url"`
	MsgType     string         `json:"msgtype"`
	Text        callbackText   `json:"text"`
	From        callbackSender `json:"from"`
	CreatedAt   int64          `json:"create_time,omitempty"`
}

type callbackText struct {
	Content string `json:"content"`
}

type callbackSender struct {
	UserID string `json:"userid"`
}

type replyContext struct {
	ResponseURL string
	ChatID      string
	UserID      string
}

type directSendTarget struct {
	ChatID string
	UserID string
}

type accessTokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

type responseReplier interface {
	ReplyText(ctx context.Context, responseURL string, content string) error
	ReplyMessage(ctx context.Context, responseURL string, payload map[string]any) error
}

type directSender interface {
	SendText(ctx context.Context, target directSendTarget, content string) error
	SendMessage(ctx context.Context, target directSendTarget, payload map[string]any) error
}

type LiveOption func(*Live) error

type Live struct {
	corpID          string
	agentID         string
	agentSecret     string
	callbackToken   string
	callbackPort    string
	callbackPath    string
	tokenProvider   accessTokenProvider
	responseReplier responseReplier
	sender          directSender

	server      *http.Server
	startCtx    context.Context
	startCancel context.CancelFunc
	started     bool
	mu          sync.Mutex
}

func NewLive(corpID, agentID, agentSecret, callbackToken, callbackPort, callbackPath string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(corpID) == "" {
		return nil, errors.New("wecom live transport requires corp id")
	}
	if strings.TrimSpace(agentID) == "" {
		return nil, errors.New("wecom live transport requires agent id")
	}
	if strings.TrimSpace(agentSecret) == "" {
		return nil, errors.New("wecom live transport requires agent secret")
	}
	if strings.TrimSpace(callbackToken) == "" {
		return nil, errors.New("wecom live transport requires callback token")
	}
	if strings.TrimSpace(callbackPort) == "" {
		return nil, errors.New("wecom live transport requires callback port")
	}
	if strings.TrimSpace(callbackPath) == "" {
		callbackPath = "/wecom/callback"
	}
	if !strings.HasPrefix(callbackPath, "/") {
		callbackPath = "/" + callbackPath
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	tokenProvider := &cachedAccessTokenProvider{
		corpID:      strings.TrimSpace(corpID),
		agentSecret: strings.TrimSpace(agentSecret),
		client:      httpClient,
	}

	live := &Live{
		corpID:          strings.TrimSpace(corpID),
		agentID:         strings.TrimSpace(agentID),
		agentSecret:     strings.TrimSpace(agentSecret),
		callbackToken:   strings.TrimSpace(callbackToken),
		callbackPort:    strings.TrimSpace(callbackPort),
		callbackPath:    callbackPath,
		tokenProvider:   tokenProvider,
		responseReplier: &httpResponseReplier{client: httpClient},
		sender: &apiDirectSender{
			agentID:       strings.TrimSpace(agentID),
			tokenProvider: tokenProvider,
			client:        httpClient,
		},
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	if live.tokenProvider == nil {
		return nil, errors.New("wecom live transport requires an access token provider")
	}
	if live.responseReplier == nil {
		return nil, errors.New("wecom live transport requires a response replier")
	}
	if live.sender == nil {
		return nil, errors.New("wecom live transport requires a direct sender")
	}

	return live, nil
}

func WithAccessTokenProvider(provider accessTokenProvider) LiveOption {
	return func(live *Live) error {
		if provider == nil {
			return errors.New("access token provider cannot be nil")
		}
		live.tokenProvider = provider
		return nil
	}
}

func WithResponseReplier(replier responseReplier) LiveOption {
	return func(live *Live) error {
		if replier == nil {
			return errors.New("response replier cannot be nil")
		}
		live.responseReplier = replier
		return nil
	}
}

func WithDirectSender(sender directSender) LiveOption {
	return func(live *Live) error {
		if sender == nil {
			return errors.New("direct sender cannot be nil")
		}
		live.sender = sender
		return nil
	}
}

func (l *Live) Name() string { return "wecom-live" }

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

func (l *Live) CallbackPaths() []string { return []string{l.callbackPath} }

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	return replyContext{
		ResponseURL: strings.TrimSpace(target.SessionWebhook),
		ChatID:      firstNonEmpty(target.ChatID, target.ChannelID, target.ConversationID),
		UserID:      strings.TrimSpace(target.UserID),
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

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+l.callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if token := strings.TrimSpace(r.Header.Get("X-WeCom-Token")); token != "" && token != l.callbackToken {
			http.Error(w, "invalid callback token", http.StatusUnauthorized)
			return
		}

		var incoming callbackMessage
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		msg, err := normalizeInboundMessage(incoming)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		handler(l, msg)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	ctx, cancel := context.WithCancel(context.Background())
	l.startCtx = ctx
	l.startCancel = cancel
	l.server = &http.Server{Addr: ":" + l.callbackPort, Handler: mux}
	l.started = true

	go func() {
		if err := l.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "wecom-live").WithError(err).Error("Callback server stopped")
		}
	}()
	return nil
}

func (l *Live) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	if reply.ResponseURL != "" {
		return l.responseReplier.ReplyText(ctx, reply.ResponseURL, content)
	}
	target := directSendTarget{
		ChatID: reply.ChatID,
		UserID: reply.UserID,
	}
	if target.ChatID == "" && target.UserID == "" {
		return errors.New("wecom reply requires response url, chat id, or user id")
	}
	return l.sender.SendText(ctx, target, content)
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	target := parseDirectSendTarget(chatID)
	if target.ChatID == "" && target.UserID == "" {
		return errors.New("wecom send requires chat id or user target")
	}
	return l.sender.SendText(ctx, target, content)
}

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	if message != nil && len(message.Sections) > 0 {
		native := &core.NativeMessage{Platform: "wecom", WeComCard: renderStructuredSections(message.Sections)}
		if err := native.Validate(); err == nil {
			return l.SendNative(ctx, chatID, native)
		}
	}
	return l.Send(ctx, chatID, renderStructuredFallback(message))
}

func (l *Live) ReplyStructured(ctx context.Context, rawReplyCtx any, message *core.StructuredMessage) error {
	if message != nil && len(message.Sections) > 0 {
		native := &core.NativeMessage{Platform: "wecom", WeComCard: renderStructuredSections(message.Sections)}
		if err := native.Validate(); err == nil {
			return l.ReplyNative(ctx, rawReplyCtx, native)
		}
	}
	return l.Reply(ctx, rawReplyCtx, renderStructuredFallback(message))
}

func (l *Live) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	if message.NormalizedPlatform() != "wecom" || message.WeComCard == nil {
		return errors.New("native message is not a wecom card payload")
	}
	target := parseDirectSendTarget(chatID)
	if target.ChatID == "" && target.UserID == "" {
		return errors.New("wecom send requires chat id or user target")
	}
	return l.sender.SendMessage(ctx, target, renderWeComNativePayload(message.WeComCard))
}

func (l *Live) ReplyNative(ctx context.Context, rawReplyCtx any, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	if message.NormalizedPlatform() != "wecom" || message.WeComCard == nil {
		return errors.New("native message is not a wecom card payload")
	}
	reply := toReplyContext(rawReplyCtx)
	payload := renderWeComNativePayload(message.WeComCard)
	if reply.ResponseURL != "" {
		return l.responseReplier.ReplyMessage(ctx, reply.ResponseURL, payload)
	}
	target := directSendTarget{ChatID: reply.ChatID, UserID: reply.UserID}
	if target.ChatID == "" && target.UserID == "" {
		return errors.New("wecom reply requires response url, chat id, or user id")
	}
	return l.sender.SendMessage(ctx, target, payload)
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	if message == nil {
		return errors.New("formatted text is required")
	}
	if message.Format == core.TextFormatWeComMD {
		target := parseDirectSendTarget(chatID)
		if target.ChatID == "" && target.UserID == "" {
			return errors.New("wecom send requires chat id or user target")
		}
		return l.sender.SendMessage(ctx, target, map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": message.Content,
			},
		})
	}
	return l.Send(ctx, chatID, message.Content)
}

func (l *Live) ReplyFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	if message == nil {
		return errors.New("formatted text is required")
	}
	if message.Format == core.TextFormatWeComMD {
		reply := toReplyContext(rawReplyCtx)
		payload := map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": message.Content,
			},
		}
		if reply.ResponseURL != "" {
			return l.responseReplier.ReplyMessage(ctx, reply.ResponseURL, payload)
		}
		target := directSendTarget{ChatID: reply.ChatID, UserID: reply.UserID}
		if target.ChatID == "" && target.UserID == "" {
			return errors.New("wecom reply requires response url, chat id, or user id")
		}
		return l.sender.SendMessage(ctx, target, payload)
	}
	return l.Reply(ctx, rawReplyCtx, message.Content)
}

func (l *Live) UpdateFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	return l.ReplyFormattedText(ctx, rawReplyCtx, message)
}

func (l *Live) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	if card == nil {
		return errors.New("card is required")
	}
	target := parseDirectSendTarget(chatID)
	if target.ChatID == "" && target.UserID == "" {
		return errors.New("wecom send requires chat id or user target")
	}
	return l.sender.SendMessage(ctx, target, renderCardToWeComTextCard(card))
}

func (l *Live) ReplyCard(ctx context.Context, rawReplyCtx any, card *core.Card) error {
	if card == nil {
		return errors.New("card is required")
	}
	reply := toReplyContext(rawReplyCtx)
	payload := renderCardToWeComTextCard(card)
	if reply.ResponseURL != "" {
		return l.responseReplier.ReplyMessage(ctx, reply.ResponseURL, payload)
	}
	target := directSendTarget{ChatID: reply.ChatID, UserID: reply.UserID}
	if target.ChatID == "" && target.UserID == "" {
		return errors.New("wecom reply requires response url, chat id, or user id")
	}
	return l.sender.SendMessage(ctx, target, payload)
}

var _ core.FormattedTextSender = (*Live)(nil)
var _ core.CardSender = (*Live)(nil)

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
	if l.server != nil {
		return l.server.Shutdown(context.Background())
	}
	return nil
}

func normalizeInboundMessage(incoming callbackMessage) (*core.Message, error) {
	if strings.TrimSpace(incoming.MsgType) != "" && strings.TrimSpace(incoming.MsgType) != "text" {
		return nil, fmt.Errorf("unsupported wecom message type %q", incoming.MsgType)
	}
	content := strings.TrimSpace(incoming.Text.Content)
	if content == "" {
		return nil, errors.New("wecom callback missing text content")
	}
	chatID := strings.TrimSpace(incoming.ChatID)
	if chatID == "" {
		return nil, errors.New("wecom callback missing chat id")
	}
	userID := strings.TrimSpace(incoming.From.UserID)
	if userID == "" {
		return nil, errors.New("wecom callback missing sender user id")
	}

	reply := &core.ReplyTarget{
		Platform:       "wecom",
		ChatID:         chatID,
		ChannelID:      chatID,
		ConversationID: chatID,
		SessionWebhook: strings.TrimSpace(incoming.ResponseURL),
		UserID:         userID,
		UseReply:       true,
		Metadata: map[string]string{
			"chat_type": strings.TrimSpace(incoming.ChatType),
			"msgid":     strings.TrimSpace(incoming.MsgID),
		},
	}

	return &core.Message{
		Platform:   "wecom",
		SessionKey: fmt.Sprintf("wecom:%s:%s", chatID, userID),
		UserID:     userID,
		ChatID:     chatID,
		Content:    content,
		ReplyCtx: replyContext{
			ResponseURL: strings.TrimSpace(incoming.ResponseURL),
			ChatID:      chatID,
			UserID:      userID,
		},
		ReplyTarget: reply,
		Timestamp:   parseEventTime(incoming.CreatedAt),
		IsGroup:     strings.TrimSpace(incoming.ChatType) != "single",
	}, nil
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
		content += "\n\nWeCom richer card update is unavailable; sent as text fallback."
	}
	return content
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
			ChatID: firstNonEmpty(value.ChatID, value.ThreadID),
			UserID: value.UserID,
		}
	case *core.ReplyTarget:
		if value == nil {
			return replyContext{}
		}
		return replyContext{
			ResponseURL: strings.TrimSpace(value.SessionWebhook),
			ChatID:      firstNonEmpty(value.ChatID, value.ChannelID, value.ConversationID),
			UserID:      strings.TrimSpace(value.UserID),
		}
	default:
		return replyContext{}
	}
}

func parseDirectSendTarget(raw string) directSendTarget {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return directSendTarget{}
	case strings.HasPrefix(trimmed, "user:"):
		return directSendTarget{UserID: strings.TrimSpace(strings.TrimPrefix(trimmed, "user:"))}
	default:
		return directSendTarget{ChatID: trimmed}
	}
}

func parseEventTime(raw int64) time.Time {
	if raw <= 0 {
		return time.Now()
	}
	return time.UnixMilli(raw)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func baseMetadata() core.PlatformMetadata {
	return liveMetadata
}

type cachedAccessTokenProvider struct {
	corpID      string
	agentSecret string
	client      *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func (p *cachedAccessTokenProvider) AccessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.expiresAt.Add(-time.Minute)) {
		return p.token, nil
	}

	endpoint := "https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=" + url.QueryEscape(p.corpID) + "&corpsecret=" + url.QueryEscape(p.agentSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var payload struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.ErrCode != 0 || strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("wecom gettoken failed: %s", strings.TrimSpace(payload.ErrMsg))
	}
	p.token = strings.TrimSpace(payload.AccessToken)
	expireIn := payload.ExpiresIn
	if expireIn <= 0 {
		expireIn = 7200
	}
	p.expiresAt = time.Now().Add(time.Duration(expireIn) * time.Second)
	return p.token, nil
}

type httpResponseReplier struct {
	client *http.Client
}

func (r *httpResponseReplier) ReplyText(ctx context.Context, responseURL string, content string) error {
	return r.ReplyMessage(ctx, responseURL, map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	})
}

func (r *httpResponseReplier) ReplyMessage(ctx context.Context, responseURL string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("wecom response_url reply failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

type apiDirectSender struct {
	agentID       string
	tokenProvider accessTokenProvider
	client        *http.Client
}

func (s *apiDirectSender) SendText(ctx context.Context, target directSendTarget, content string) error {
	return s.SendMessage(ctx, target, map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	})
}

func (s *apiDirectSender) SendMessage(ctx context.Context, target directSendTarget, payload map[string]any) error {
	token, err := s.tokenProvider.AccessToken(ctx)
	if err != nil {
		return err
	}
	if payload == nil {
		return errors.New("wecom direct send payload is required")
	}
	bodyPayload := make(map[string]any, len(payload)+2)
	for key, value := range payload {
		bodyPayload[key] = value
	}
	bodyPayload["agentid"] = s.agentID
	bodyPayload["safe"] = 0
	switch {
	case strings.TrimSpace(target.ChatID) != "":
		bodyPayload["chatid"] = strings.TrimSpace(target.ChatID)
	case strings.TrimSpace(target.UserID) != "":
		bodyPayload["touser"] = strings.TrimSpace(target.UserID)
	default:
		return errors.New("wecom direct send requires chat id or user id")
	}
	body, err := json.Marshal(bodyPayload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token="+url.QueryEscape(token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return err
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom direct send failed: %s", strings.TrimSpace(result.ErrMsg))
	}
	return nil
}

func renderWeComNativePayload(payload *core.WeComCardPayload) map[string]any {
	switch payload.CardType {
	case core.WeComCardTypeNews:
		articles := make([]map[string]string, 0, len(payload.Articles))
		for _, article := range payload.Articles {
			articles = append(articles, map[string]string{
				"title":       strings.TrimSpace(article.Title),
				"description": strings.TrimSpace(article.Description),
				"url":         strings.TrimSpace(article.URL),
				"picurl":      strings.TrimSpace(article.PicURL),
			})
		}
		return map[string]any{
			"msgtype": "news",
			"news": map[string]any{
				"articles": articles,
			},
		}
	default:
		cardItems := make([]map[string]string, 0, len(payload.TemplateFields))
		for _, field := range payload.TemplateFields {
			cardItems = append(cardItems, map[string]string{
				"key":   strings.TrimSpace(field.Key),
				"value": strings.TrimSpace(field.Value),
			})
		}
		return map[string]any{
			"msgtype": "template_card",
			"template_card": map[string]any{
				"card_type": "text_notice",
				"source": map[string]string{
					"desc": strings.TrimSpace(payload.Title),
				},
				"main_title": map[string]string{
					"title": strings.TrimSpace(payload.Title),
					"desc":  strings.TrimSpace(payload.Description),
				},
				"card_action": map[string]any{
					"type": 1,
					"url":  strings.TrimSpace(payload.URL),
				},
				"horizontal_content_list": cardItems,
			},
		}
	}
}
