package qqbot

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
	Source: "qqbot",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:         core.CommandSurfaceMixed,
		AsyncUpdateModes:       []core.AsyncUpdateMode{core.AsyncUpdateReply},
		ActionCallbackMode:     core.ActionCallbackWebhook,
		MessageScopes:          []core.MessageScope{core.MessageScopeChat},
		RequiresPublicCallback: true,
		SupportsMentions:       true,
		SupportsSlashCommands:  true,
	},
}, "qqbot")

type webhookPayload struct {
	Type      string         `json:"t,omitempty"`
	EventType string         `json:"event_type,omitempty"`
	Data      inboundMessage `json:"d"`
}

type inboundMessage struct {
	ID          string        `json:"id,omitempty"`
	Content     string        `json:"content,omitempty"`
	GroupOpenID string        `json:"group_openid,omitempty"`
	ChannelID   string        `json:"channel_id,omitempty"`
	GuildID     string        `json:"guild_id,omitempty"`
	Timestamp   string        `json:"timestamp,omitempty"`
	Author      inboundAuthor `json:"author"`
}

type inboundAuthor struct {
	UserOpenID string `json:"user_openid,omitempty"`
	ID         string `json:"id,omitempty"`
	Username   string `json:"username,omitempty"`
}

type messageTarget struct {
	GroupOpenID string
	UserOpenID  string
	MessageID   string
}

type accessTokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

type sender interface {
	SendText(ctx context.Context, target messageTarget, content string) error
}

type LiveOption func(*Live) error

type Live struct {
	appID         string
	appSecret     string
	callbackPort  string
	callbackPath  string
	apiBase       string
	tokenBase     string
	tokenProvider accessTokenProvider
	sender        sender

	server      *http.Server
	startCtx    context.Context
	startCancel context.CancelFunc
	started     bool
	mu          sync.Mutex
}

func NewLive(appID, appSecret, callbackPort, callbackPath string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, errors.New("qqbot live transport requires app id")
	}
	if strings.TrimSpace(appSecret) == "" {
		return nil, errors.New("qqbot live transport requires app secret")
	}
	if strings.TrimSpace(callbackPort) == "" {
		return nil, errors.New("qqbot live transport requires callback port")
	}
	if strings.TrimSpace(callbackPath) == "" {
		callbackPath = "/qqbot/callback"
	}
	if !strings.HasPrefix(callbackPath, "/") {
		callbackPath = "/" + callbackPath
	}

	live := &Live{
		appID:        strings.TrimSpace(appID),
		appSecret:    strings.TrimSpace(appSecret),
		callbackPort: strings.TrimSpace(callbackPort),
		callbackPath: callbackPath,
		apiBase:      "https://api.sgroup.qq.com",
		tokenBase:    "https://bots.qq.com",
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	if live.tokenProvider == nil {
		live.tokenProvider = &cachedAccessTokenProvider{
			appID:     live.appID,
			appSecret: live.appSecret,
			tokenBase: live.tokenBase,
			client:    httpClient,
		}
	}
	if live.sender == nil {
		live.sender = &apiSender{
			apiBase:       live.apiBase,
			tokenProvider: live.tokenProvider,
			client:        httpClient,
		}
	}

	return live, nil
}

func WithSender(sender sender) LiveOption {
	return func(live *Live) error {
		if sender == nil {
			return errors.New("sender cannot be nil")
		}
		live.sender = sender
		return nil
	}
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

func WithAPIBase(apiBase string) LiveOption {
	return func(live *Live) error {
		if strings.TrimSpace(apiBase) != "" {
			live.apiBase = strings.TrimRight(strings.TrimSpace(apiBase), "/")
		}
		return nil
	}
}

func WithTokenBase(tokenBase string) LiveOption {
	return func(live *Live) error {
		if strings.TrimSpace(tokenBase) != "" {
			live.tokenBase = strings.TrimRight(strings.TrimSpace(tokenBase), "/")
		}
		return nil
	}
}

func (l *Live) Name() string { return "qqbot-live" }

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

func (l *Live) CallbackPaths() []string { return []string{l.callbackPath} }

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	chatID := firstNonEmpty(target.ChatID, target.ChannelID, target.ConversationID)
	return replyContext{
		ChatID:    chatID,
		UserID:    strings.TrimSpace(target.UserID),
		MessageID: strings.TrimSpace(target.MessageID),
		IsGroup:   chatID != "" && chatID != strings.TrimSpace(target.UserID),
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
		var payload webhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		msg, err := normalizeInboundPayload(payload)
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
			log.WithField("component", "qqbot-live").WithError(err).Error("Callback server stopped")
		}
	}()
	return nil
}

func (l *Live) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	target := targetFromReply(reply)
	if target.GroupOpenID == "" && target.UserOpenID == "" {
		return errors.New("qqbot reply requires group or user target")
	}
	return l.sender.SendText(ctx, target, content)
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	target := parseTarget(chatID)
	if target.GroupOpenID == "" && target.UserOpenID == "" {
		return errors.New("qqbot send requires group or user target")
	}
	return l.sender.SendText(ctx, target, content)
}

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	return l.Send(ctx, chatID, strings.TrimSpace(message.FallbackText()))
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
	if l.server != nil {
		return l.server.Shutdown(context.Background())
	}
	return nil
}

func normalizeInboundPayload(payload webhookPayload) (*core.Message, error) {
	eventType := firstNonEmpty(payload.Type, payload.EventType)
	content := strings.TrimSpace(payload.Data.Content)
	if content == "" {
		return nil, errors.New("qqbot message missing text content")
	}

	userID := firstNonEmpty(payload.Data.Author.UserOpenID, payload.Data.Author.ID)
	if userID == "" {
		return nil, errors.New("qqbot message missing author id")
	}

	chatID := firstNonEmpty(payload.Data.GroupOpenID, payload.Data.ChannelID, userID)
	if chatID == "" {
		return nil, errors.New("qqbot message missing target conversation id")
	}
	isGroup := payload.Data.GroupOpenID != "" || strings.Contains(strings.ToUpper(eventType), "GROUP")

	reply := &core.ReplyTarget{
		Platform:  liveMetadata.Source,
		ChatID:    chatID,
		ChannelID: chatID,
		MessageID: strings.TrimSpace(payload.Data.ID),
		UserID:    userID,
		UseReply:  true,
		Metadata: map[string]string{
			"event_type": eventType,
			"scope":      map[bool]string{true: "group", false: "user"}[isGroup],
		},
	}

	return &core.Message{
		Platform:   liveMetadata.Source,
		SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, chatID, userID),
		UserID:     userID,
		UserName:   strings.TrimSpace(payload.Data.Author.Username),
		ChatID:     chatID,
		Content:    content,
		ReplyCtx: replyContext{
			ChatID:    chatID,
			UserID:    userID,
			MessageID: strings.TrimSpace(payload.Data.ID),
			IsGroup:   isGroup,
		},
		ReplyTarget: reply,
		Timestamp:   parseEventTime(payload.Data.Timestamp),
		IsGroup:     isGroup,
	}, nil
}

func parseTarget(raw string) messageTarget {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return messageTarget{}
	case strings.HasPrefix(trimmed, "user:"):
		return messageTarget{UserOpenID: strings.TrimSpace(strings.TrimPrefix(trimmed, "user:"))}
	case strings.HasPrefix(trimmed, "group:"):
		return messageTarget{GroupOpenID: strings.TrimSpace(strings.TrimPrefix(trimmed, "group:"))}
	default:
		return messageTarget{GroupOpenID: trimmed}
	}
}

func targetFromReply(reply replyContext) messageTarget {
	switch {
	case reply.IsGroup && reply.ChatID != "":
		return messageTarget{GroupOpenID: reply.ChatID, MessageID: reply.MessageID}
	case reply.UserID != "":
		return messageTarget{UserOpenID: reply.UserID, MessageID: reply.MessageID}
	case reply.ChatID != "":
		return messageTarget{GroupOpenID: reply.ChatID, MessageID: reply.MessageID}
	default:
		return messageTarget{}
	}
}

func parseEventTime(raw string) time.Time {
	if strings.TrimSpace(raw) == "" {
		return time.Now()
	}
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw)); err == nil {
		return parsed
	}
	return time.Now()
}

type cachedAccessTokenProvider struct {
	appID     string
	appSecret string
	tokenBase string
	client    *http.Client

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

	body, err := json.Marshal(map[string]string{
		"appId":        p.appID,
		"clientSecret": p.appSecret,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.tokenBase, "/")+"/app/getAppAccessToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("qqbot access token request failed: %s", strings.TrimSpace(string(respBody)))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", errors.New("qqbot access token response missing access_token")
	}

	expireIn := payload.ExpiresIn
	if expireIn <= 0 {
		expireIn = 3600
	}
	p.token = strings.TrimSpace(payload.AccessToken)
	p.expiresAt = time.Now().Add(time.Duration(expireIn) * time.Second)
	return p.token, nil
}

type apiSender struct {
	apiBase       string
	tokenProvider accessTokenProvider
	client        *http.Client
}

func (s *apiSender) SendText(ctx context.Context, target messageTarget, content string) error {
	token, err := s.tokenProvider.AccessToken(ctx)
	if err != nil {
		return err
	}

	endpoint := ""
	switch {
	case strings.TrimSpace(target.GroupOpenID) != "":
		endpoint = strings.TrimRight(s.apiBase, "/") + "/v2/groups/" + url.PathEscape(strings.TrimSpace(target.GroupOpenID)) + "/messages"
	case strings.TrimSpace(target.UserOpenID) != "":
		endpoint = strings.TrimRight(s.apiBase, "/") + "/v2/users/" + url.PathEscape(strings.TrimSpace(target.UserOpenID)) + "/messages"
	default:
		return errors.New("qqbot send requires group_openid or user_openid")
	}

	payload := map[string]any{
		"content":  strings.TrimSpace(content),
		"msg_type": 0,
	}
	if strings.TrimSpace(target.MessageID) != "" {
		payload["msg_id"] = strings.TrimSpace(target.MessageID)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "QQBot "+strings.TrimSpace(token))
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qqbot send failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}
