package qq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/agentforge/im-bridge/core"
)

var liveMetadata = core.NormalizeMetadata(core.PlatformMetadata{
	Source: "qq",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:        core.CommandSurfaceMixed,
		AsyncUpdateModes:      []core.AsyncUpdateMode{core.AsyncUpdateReply},
		ActionCallbackMode:    core.ActionCallbackNone,
		MessageScopes:         []core.MessageScope{core.MessageScopeChat},
		SupportsMentions:      true,
		SupportsSlashCommands: true,
	},
}, "qq")

type senderInfo struct {
	Nickname string `json:"nickname,omitempty"`
	Card     string `json:"card,omitempty"`
}

type incomingEvent struct {
	PostType    string          `json:"post_type,omitempty"`
	MessageType string          `json:"message_type,omitempty"`
	MessageID   int64           `json:"message_id,omitempty"`
	UserID      int64           `json:"user_id,omitempty"`
	GroupID     int64           `json:"group_id,omitempty"`
	RawMessage  string          `json:"raw_message,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`
	Time        int64           `json:"time,omitempty"`
	Sender      senderInfo      `json:"sender,omitempty"`
}

type transport interface {
	Start(ctx context.Context, handler func(context.Context, incomingEvent) error) error
	Stop(ctx context.Context) error
	SendAction(ctx context.Context, action string, params map[string]any) error
}

type LiveOption func(*Live) error

type Live struct {
	wsURL       string
	accessToken string
	transport   transport
}

func NewLive(wsURL, accessToken string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(wsURL) == "" {
		return nil, errors.New("qq live transport requires onebot websocket url")
	}

	live := &Live{
		wsURL:       strings.TrimSpace(wsURL),
		accessToken: strings.TrimSpace(accessToken),
	}
	live.transport = newWSTransport(live.wsURL, live.accessToken)

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	if live.transport == nil {
		return nil, errors.New("qq live transport requires a transport")
	}
	return live, nil
}

func WithTransport(transport transport) LiveOption {
	return func(live *Live) error {
		if transport == nil {
			return errors.New("transport cannot be nil")
		}
		live.transport = transport
		return nil
	}
}

func (l *Live) Name() string { return "qq-live" }

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

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
	return l.transport.Start(context.Background(), func(ctx context.Context, event incomingEvent) error {
		msg, err := normalizeIncomingEvent(event)
		if err != nil {
			log.WithField("component", "qq-live").WithError(err).Warn("Ignoring inbound event")
			return nil
		}
		handler(l, msg)
		return nil
	})
}

func (l *Live) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	target := messageTargetFromReply(reply)
	if target.action == "" {
		return errors.New("qq reply requires group or user target")
	}
	if reply.MessageID != "" {
		content = "[CQ:reply,id=" + reply.MessageID + "]" + content
	}
	return l.transport.SendAction(ctx, target.action, map[string]any{
		target.paramName: target.id,
		"message":        content,
	})
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	target := parseTarget(chatID)
	if target.action == "" {
		return errors.New("qq send requires group or user target")
	}
	return l.transport.SendAction(ctx, target.action, map[string]any{
		target.paramName: target.id,
		"message":        content,
	})
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

var _ core.FormattedTextSender = (*Live)(nil)

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	return l.Send(ctx, chatID, strings.TrimSpace(message.FallbackText()))
}

func (l *Live) Stop() error {
	return l.transport.Stop(context.Background())
}

func normalizeIncomingEvent(event incomingEvent) (*core.Message, error) {
	if strings.TrimSpace(event.PostType) != "message" {
		return nil, fmt.Errorf("unsupported qq post type %q", event.PostType)
	}
	content := strings.TrimSpace(event.RawMessage)
	if content == "" && len(event.Message) > 0 {
		content = strings.TrimSpace(renderMessagePayload(event.Message))
	}
	if content == "" {
		return nil, errors.New("qq message missing text content")
	}
	if event.UserID == 0 {
		return nil, errors.New("qq message missing sender id")
	}

	isGroup := strings.TrimSpace(event.MessageType) != "private"
	chatID := fmt.Sprintf("%d", event.GroupID)
	if !isGroup {
		chatID = fmt.Sprintf("%d", event.UserID)
	}
	if strings.TrimSpace(chatID) == "" || strings.TrimSpace(chatID) == "0" {
		return nil, errors.New("qq message missing chat id")
	}

	userID := fmt.Sprintf("%d", event.UserID)
	reply := &core.ReplyTarget{
		Platform:       liveMetadata.Source,
		ChatID:         chatID,
		ChannelID:      chatID,
		ConversationID: chatID,
		MessageID:      fmt.Sprintf("%d", event.MessageID),
		UserID:         userID,
		UseReply:       true,
		ProgressMode:   string(core.AsyncUpdateReply),
		Metadata: map[string]string{
			"message_type": strings.TrimSpace(event.MessageType),
		},
	}

	return &core.Message{
		Platform:   liveMetadata.Source,
		SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, chatID, userID),
		UserID:     userID,
		UserName:   firstNonEmpty(event.Sender.Card, event.Sender.Nickname),
		ChatID:     chatID,
		Content:    content,
		ReplyCtx: replyContext{
			ChatID:    chatID,
			UserID:    userID,
			MessageID: fmt.Sprintf("%d", event.MessageID),
			IsGroup:   isGroup,
		},
		ReplyTarget: reply,
		Timestamp:   parseEventTime(event.Time),
		IsGroup:     isGroup,
	}, nil
}

type actionTarget struct {
	action    string
	paramName string
	id        string
}

func parseTarget(raw string) actionTarget {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return actionTarget{}
	case strings.HasPrefix(trimmed, "user:"):
		return actionTarget{action: "send_private_msg", paramName: "user_id", id: strings.TrimSpace(strings.TrimPrefix(trimmed, "user:"))}
	case strings.HasPrefix(trimmed, "group:"):
		return actionTarget{action: "send_group_msg", paramName: "group_id", id: strings.TrimSpace(strings.TrimPrefix(trimmed, "group:"))}
	default:
		return actionTarget{action: "send_group_msg", paramName: "group_id", id: trimmed}
	}
}

func messageTargetFromReply(reply replyContext) actionTarget {
	switch {
	case reply.IsGroup && reply.ChatID != "":
		return actionTarget{action: "send_group_msg", paramName: "group_id", id: reply.ChatID}
	case reply.UserID != "":
		return actionTarget{action: "send_private_msg", paramName: "user_id", id: reply.UserID}
	case reply.ChatID != "":
		return actionTarget{action: "send_group_msg", paramName: "group_id", id: reply.ChatID}
	default:
		return actionTarget{}
	}
}

func renderMessagePayload(raw json.RawMessage) string {
	var direct string
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct
	}
	var segments []struct {
		Type string                 `json:"type"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(raw, &segments); err != nil {
		return ""
	}
	var builder strings.Builder
	for _, segment := range segments {
		if strings.TrimSpace(segment.Type) != "text" {
			continue
		}
		if text, ok := segment.Data["text"].(string); ok {
			builder.WriteString(text)
		}
	}
	return builder.String()
}

func parseEventTime(raw int64) time.Time {
	if raw <= 0 {
		return time.Now()
	}
	return time.Unix(raw, 0)
}

type wsTransport struct {
	wsURL       string
	accessToken string

	mu     sync.Mutex
	conn   *websocket.Conn
	cancel context.CancelFunc
	done   chan struct{}
}

func newWSTransport(wsURL, accessToken string) *wsTransport {
	return &wsTransport{wsURL: wsURL, accessToken: accessToken}
}

func (t *wsTransport) Start(ctx context.Context, handler func(context.Context, incomingEvent) error) error {
	headers := map[string][]string{}
	if strings.TrimSpace(t.accessToken) != "" {
		headers["Authorization"] = []string{"Bearer " + strings.TrimSpace(t.accessToken)}
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, t.wsURL, headers)
	if err != nil {
		return fmt.Errorf("dial qq websocket: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	t.conn = conn
	t.cancel = cancel
	t.done = make(chan struct{})
	done := t.done
	t.mu.Unlock()

	go func() {
		defer close(done)
		for {
			select {
			case <-runCtx.Done():
				return
			default:
			}

			_, payload, err := conn.ReadMessage()
			if err != nil {
				if runCtx.Err() == nil {
					log.WithField("component", "qq-live").WithError(err).Warn("QQ websocket stopped")
				}
				return
			}

			var probe struct {
				PostType string `json:"post_type,omitempty"`
			}
			if err := json.Unmarshal(payload, &probe); err != nil || strings.TrimSpace(probe.PostType) == "" {
				continue
			}

			var event incomingEvent
			if err := json.Unmarshal(payload, &event); err != nil {
				log.WithField("component", "qq-live").WithError(err).Warn("Invalid QQ event payload")
				continue
			}
			if err := handler(runCtx, event); err != nil {
				log.WithField("component", "qq-live").WithError(err).Warn("QQ event handler failed")
			}
		}
	}()

	return nil
}

func (t *wsTransport) Stop(ctx context.Context) error {
	t.mu.Lock()
	cancel := t.cancel
	done := t.done
	conn := t.conn
	t.cancel = nil
	t.done = nil
	t.conn = nil
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if done != nil {
		<-done
	}
	return nil
}

func (t *wsTransport) SendAction(ctx context.Context, action string, params map[string]any) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil {
		return errors.New("qq transport is not connected")
	}
	return t.conn.WriteJSON(map[string]any{
		"action": action,
		"params": params,
		"echo":   time.Now().UnixNano(),
	})
}
