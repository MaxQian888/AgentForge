package wechat

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/im-bridge/core"
)

var liveMetadata = core.NormalizeMetadata(core.PlatformMetadata{
	Source: "wechat",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:           core.CommandSurfaceMixed,
		StructuredSurface:        core.StructuredSurfaceNone,
		AsyncUpdateModes:         []core.AsyncUpdateMode{core.AsyncUpdateReply},
		PreferredAsyncUpdateMode: core.AsyncUpdateReply,
		ActionCallbackMode:       core.ActionCallbackWebhook,
		MessageScopes:            []core.MessageScope{core.MessageScopeChat},
		ReadinessTier:            core.ReadinessTierTextFirst,
		SupportsMentions:         true,
	},
	Rendering: core.RenderingProfile{
		DefaultTextFormat: core.TextFormatPlainText,
		SupportedFormats:  []core.TextFormatMode{core.TextFormatPlainText},
		MaxTextLength:     2048,
	},
}, "wechat")

// callbackXMLMessage represents an incoming WeChat XML callback message.
type callbackXMLMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
}

// replyContext holds context for replying to a WeChat message.
type replyContext struct {
	OpenID string
	ChatID string
}

// LiveOption configures optional Live parameters.
type LiveOption func(*Live) error

// Live is the production adapter for WeChat Official Account (公众号) API.
type Live struct {
	appID         string
	appSecret     string
	callbackToken string
	callbackPort  string
	callbackPath  string

	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time

	server      *http.Server
	handler     core.MessageHandler
	startCtx    context.Context
	startCancel context.CancelFunc
	started     bool
}

// NewLive creates a new production WeChat adapter.
func NewLive(appID, appSecret, callbackToken string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(appID) == "" {
		return nil, errors.New("wechat live transport requires app id")
	}
	if strings.TrimSpace(appSecret) == "" {
		return nil, errors.New("wechat live transport requires app secret")
	}
	if strings.TrimSpace(callbackToken) == "" {
		return nil, errors.New("wechat live transport requires callback token")
	}

	live := &Live{
		appID:         strings.TrimSpace(appID),
		appSecret:     strings.TrimSpace(appSecret),
		callbackToken: strings.TrimSpace(callbackToken),
		callbackPort:  "8080",
		callbackPath:  "/wechat/callback",
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}

	return live, nil
}

// WithCallbackPort sets the HTTP server listen port.
func WithCallbackPort(port string) LiveOption {
	return func(live *Live) error {
		if strings.TrimSpace(port) != "" {
			live.callbackPort = strings.TrimSpace(port)
		}
		return nil
	}
}

// WithCallbackPath sets the callback endpoint path.
func WithCallbackPath(path string) LiveOption {
	return func(live *Live) error {
		if strings.TrimSpace(path) != "" {
			p := strings.TrimSpace(path)
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			live.callbackPath = p
		}
		return nil
	}
}

func (l *Live) Name() string { return "wechat-live" }

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

func (l *Live) CallbackPaths() []string { return []string{l.callbackPath} }

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	return replyContext{
		OpenID: strings.TrimSpace(target.UserID),
		ChatID: firstNonEmpty(target.ChatID, target.ChannelID, target.ConversationID),
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

	l.handler = handler

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+l.callbackPath, l.handleVerification)
	mux.HandleFunc("POST "+l.callbackPath, l.handleCallback)

	ctx, cancel := context.WithCancel(context.Background())
	l.startCtx = ctx
	l.startCancel = cancel
	l.server = &http.Server{Addr: ":" + l.callbackPort, Handler: mux}
	l.started = true

	go func() {
		if err := l.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "wechat-live").WithError(err).Error("Callback server stopped")
		}
	}()
	return nil
}

// handleVerification handles WeChat callback URL verification (GET).
func (l *Live) handleVerification(w http.ResponseWriter, r *http.Request) {
	signature := r.URL.Query().Get("signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	echostr := r.URL.Query().Get("echostr")

	if !verifySignature(l.callbackToken, timestamp, nonce, signature) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(echostr))
}

// handleCallback handles incoming WeChat messages (POST).
func (l *Live) handleCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var incoming callbackXMLMessage
	if err := xml.Unmarshal(body, &incoming); err != nil {
		http.Error(w, fmt.Sprintf("invalid XML: %v", err), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(incoming.MsgType) != "" && strings.TrimSpace(incoming.MsgType) != "text" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("success"))
		return
	}

	content := strings.TrimSpace(incoming.Content)
	if content == "" {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("success"))
		return
	}

	openID := strings.TrimSpace(incoming.FromUserName)
	chatID := strings.TrimSpace(incoming.ToUserName)

	msg := &core.Message{
		Platform:   "wechat",
		SessionKey: fmt.Sprintf("wechat:%s:%s", chatID, openID),
		UserID:     openID,
		ChatID:     chatID,
		Content:    content,
		Timestamp:  parseEventTime(incoming.CreateTime),
		ReplyCtx: replyContext{
			OpenID: openID,
			ChatID: chatID,
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:       "wechat",
			ChatID:         chatID,
			ChannelID:      chatID,
			ConversationID: chatID,
			UserID:         openID,
			UseReply:       true,
			ProgressMode:   string(core.AsyncUpdateReply),
			Metadata: map[string]string{
				"msg_id": fmt.Sprintf("%d", incoming.MsgId),
			},
		},
	}

	l.handler(l, msg)

	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("success"))
}

func (l *Live) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	if reply.OpenID == "" {
		return errors.New("wechat reply requires openid")
	}
	return l.sendCustomMessage(ctx, reply.OpenID, renderTextReply(reply.OpenID, content))
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	openID := strings.TrimSpace(chatID)
	if openID == "" {
		return errors.New("wechat send requires target openid")
	}
	return l.sendCustomMessage(ctx, openID, renderTextReply(openID, content))
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	if message == nil {
		return errors.New("formatted text is required")
	}
	return l.Send(ctx, chatID, message.Content)
}

func (l *Live) ReplyFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	if message == nil {
		return errors.New("formatted text is required")
	}
	return l.Reply(ctx, rawReplyCtx, message.Content)
}

func (l *Live) UpdateFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	return l.ReplyFormattedText(ctx, rawReplyCtx, message)
}

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	return l.Send(ctx, chatID, renderStructuredFallback(message))
}

func (l *Live) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	return l.Send(ctx, chatID, message.FallbackText())
}

func (l *Live) ReplyNative(ctx context.Context, rawReplyCtx any, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	return l.Reply(ctx, rawReplyCtx, message.FallbackText())
}

var _ core.FormattedTextSender = (*Live)(nil)
var _ core.NativeMessageSender = (*Live)(nil)

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

// sendCustomMessage sends a customer service message via the WeChat API.
func (l *Live) sendCustomMessage(ctx context.Context, openID string, payload []byte) error {
	token, err := l.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("wechat access token error: %w", err)
	}

	endpoint := "https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.httpClient.Do(req)
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
		return fmt.Errorf("wechat customer message send failed: %s", strings.TrimSpace(result.ErrMsg))
	}
	return nil
}

// getAccessToken returns a cached access token or fetches a new one.
func (l *Live) getAccessToken(ctx context.Context) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.accessToken != "" && time.Now().Before(l.tokenExpiry.Add(-time.Minute)) {
		return l.accessToken, nil
	}

	endpoint := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" +
		url.QueryEscape(l.appID) + "&secret=" + url.QueryEscape(l.appSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := l.httpClient.Do(req)
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
		return "", fmt.Errorf("wechat gettoken failed: %s", strings.TrimSpace(payload.ErrMsg))
	}

	l.accessToken = strings.TrimSpace(payload.AccessToken)
	expireIn := payload.ExpiresIn
	if expireIn <= 0 {
		expireIn = 7200
	}
	l.tokenExpiry = time.Now().Add(time.Duration(expireIn) * time.Second)
	return l.accessToken, nil
}

// verifySignature implements WeChat callback verification:
// sort(token, timestamp, nonce) -> SHA1 -> compare with signature.
func verifySignature(token, timestamp, nonce, signature string) bool {
	strs := []string{token, timestamp, nonce}
	sort.Strings(strs)
	combined := strings.Join(strs, "")
	hash := sha1.New()
	hash.Write([]byte(combined))
	computed := fmt.Sprintf("%x", hash.Sum(nil))
	return computed == signature
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
			OpenID: value.UserID,
			ChatID: firstNonEmpty(value.ChatID, value.ThreadID),
		}
	case *core.ReplyTarget:
		if value == nil {
			return replyContext{}
		}
		return replyContext{
			OpenID: strings.TrimSpace(value.UserID),
			ChatID: firstNonEmpty(value.ChatID, value.ChannelID, value.ConversationID),
		}
	default:
		return replyContext{}
	}
}

func renderStructuredFallback(message *core.StructuredMessage) string {
	if message == nil {
		return ""
	}
	content := strings.TrimSpace(message.FallbackText())
	if content == "" {
		return ""
	}
	return content
}

func parseEventTime(raw int64) time.Time {
	if raw <= 0 {
		return time.Now()
	}
	return time.Unix(raw, 0)
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
