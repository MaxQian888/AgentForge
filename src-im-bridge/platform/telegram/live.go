package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
)

const (
	updateModeLongPoll = "longpoll"
)

var liveMetadata = core.PlatformMetadata{
	Source: "telegram",
	Capabilities: core.PlatformCapabilities{
		SupportsSlashCommands: true,
		SupportsMentions:      true,
	},
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message,omitempty"`
}

type message struct {
	MessageID int64 `json:"message_id"`
	Date      int64 `json:"date"`
	Text      string
	From      *user `json:"from,omitempty"`
	Chat      *chat `json:"chat,omitempty"`
}

type user struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type,omitempty"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type replyContext struct {
	ChatID    int64
	MessageID int
}

type updateRunner interface {
	Start(ctx context.Context, handler func(context.Context, update) error) error
	Stop(ctx context.Context) error
}

type sender interface {
	SendText(ctx context.Context, chatID int64, replyToMessageID int, text string) error
}

type LiveOption func(*Live) error

type Live struct {
	botToken string

	runner updateRunner
	sender sender

	startCtx    context.Context
	startCancel context.CancelFunc
	started     bool
	mu          sync.Mutex
}

func NewLive(botToken string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(botToken) == "" {
		return nil, errors.New("telegram live transport requires bot token")
	}

	live := &Live{
		botToken: botToken,
		runner:   newLongPollingRunner(botToken),
		sender:   newBotAPISender(botToken),
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	if live.runner == nil {
		return nil, errors.New("telegram live transport requires an update runner")
	}
	if live.sender == nil {
		return nil, errors.New("telegram live transport requires a sender")
	}

	return live, nil
}

func WithUpdateRunner(runner updateRunner) LiveOption {
	return func(live *Live) error {
		if runner == nil {
			return errors.New("update runner cannot be nil")
		}
		live.runner = runner
		return nil
	}
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

func (l *Live) Name() string { return "telegram-live" }

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	chatID, _ := strconv.ParseInt(strings.TrimSpace(firstNonEmpty(target.ChatID, target.ChannelID)), 10, 64)
	messageID, _ := strconv.Atoi(strings.TrimSpace(target.MessageID))
	return replyContext{ChatID: chatID, MessageID: messageID}
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

	if err := l.runner.Start(ctx, func(ctx context.Context, incoming update) error {
		msg, err := normalizeIncomingUpdate(incoming)
		if err != nil {
			log.Printf("[telegram-live] Ignoring inbound update: %v", err)
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
	if reply.ChatID == 0 {
		return errors.New("telegram reply requires chat id")
	}
	return l.sender.SendText(ctx, reply.ChatID, reply.MessageID, content)
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	target, err := parseChatID(chatID)
	if err != nil {
		return err
	}
	return l.sender.SendText(ctx, target, 0, content)
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

type longPollingRunner struct {
	client *botAPIClient

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func newLongPollingRunner(botToken string) *longPollingRunner {
	return &longPollingRunner{
		client: newBotAPIClient(botToken),
	}
}

func (r *longPollingRunner) Start(ctx context.Context, handler func(context.Context, update) error) error {
	if handler == nil {
		return errors.New("telegram update handler is required")
	}
	if err := r.client.deleteWebhook(ctx); err != nil {
		return err
	}

	pollCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.cancel = cancel
	r.done = make(chan struct{})
	done := r.done
	r.mu.Unlock()

	go func() {
		defer close(done)

		var offset int64
		for {
			select {
			case <-pollCtx.Done():
				return
			default:
			}

			updates, err := r.client.getUpdates(pollCtx, offset)
			if err != nil {
				if pollCtx.Err() != nil {
					return
				}
				log.Printf("[telegram-live] Long polling error: %v", err)
				select {
				case <-time.After(time.Second):
				case <-pollCtx.Done():
					return
				}
				continue
			}

			for _, incoming := range updates {
				if incoming.UpdateID >= offset {
					offset = incoming.UpdateID + 1
				}
				if err := handler(pollCtx, incoming); err != nil {
					log.Printf("[telegram-live] Handler error: %v", err)
				}
			}
		}
	}()

	return nil
}

func (r *longPollingRunner) Stop(context.Context) error {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	return nil
}

type botAPISender struct {
	client *botAPIClient
}

func newBotAPISender(botToken string) *botAPISender {
	return &botAPISender{client: newBotAPIClient(botToken)}
}

func (s *botAPISender) SendText(ctx context.Context, chatID int64, replyToMessageID int, text string) error {
	request := sendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}
	if replyToMessageID > 0 {
		request.ReplyParameters = &replyParameters{
			MessageID: replyToMessageID,
		}
	}
	return s.client.sendMessage(ctx, request)
}

type botAPIClient struct {
	baseURL string
	client  *http.Client
}

func newBotAPIClient(botToken string) *botAPIClient {
	return &botAPIClient{
		baseURL: "https://api.telegram.org/bot" + strings.TrimSpace(botToken),
		client: &http.Client{
			Timeout: 70 * time.Second,
		},
	}
}

type botAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      T      `json:"result"`
}

type getUpdatesRequest struct {
	Offset         int64    `json:"offset,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

type deleteWebhookRequest struct {
	DropPendingUpdates bool `json:"drop_pending_updates"`
}

type sendMessageRequest struct {
	ChatID          int64            `json:"chat_id"`
	Text            string           `json:"text"`
	ReplyParameters *replyParameters `json:"reply_parameters,omitempty"`
}

type replyParameters struct {
	MessageID int `json:"message_id"`
}

func (c *botAPIClient) deleteWebhook(ctx context.Context) error {
	var response botAPIResponse[bool]
	return c.call(ctx, "deleteWebhook", deleteWebhookRequest{}, &response)
}

func (c *botAPIClient) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	request := getUpdatesRequest{
		Offset:         offset,
		Timeout:        50,
		AllowedUpdates: []string{"message"},
	}
	var response botAPIResponse[[]update]
	if err := c.call(ctx, "getUpdates", request, &response); err != nil {
		return nil, err
	}
	return response.Result, nil
}

func (c *botAPIClient) sendMessage(ctx context.Context, request sendMessageRequest) error {
	if strings.TrimSpace(request.Text) == "" {
		return errors.New("telegram send requires content")
	}
	var response botAPIResponse[message]
	return c.call(ctx, "sendMessage", request, &response)
}

func (c *botAPIClient) call(ctx context.Context, method string, request any, response any) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal telegram %s request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read telegram %s response: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram %s failed with status %d: %s", method, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if err := json.Unmarshal(respBody, response); err != nil {
		return fmt.Errorf("decode telegram %s response: %w", method, err)
	}

	switch parsed := response.(type) {
	case *botAPIResponse[bool]:
		if !parsed.OK {
			return errors.New(descriptionOrFallback(parsed.Description, "telegram api request failed"))
		}
	case *botAPIResponse[[]update]:
		if !parsed.OK {
			return errors.New(descriptionOrFallback(parsed.Description, "telegram getUpdates failed"))
		}
	case *botAPIResponse[message]:
		if !parsed.OK {
			return errors.New(descriptionOrFallback(parsed.Description, "telegram sendMessage failed"))
		}
	}
	return nil
}

func normalizeIncomingUpdate(incoming update) (*core.Message, error) {
	if incoming.Message == nil {
		return nil, errors.New("telegram update does not contain a message payload")
	}
	if incoming.Message.Chat == nil {
		return nil, errors.New("telegram message missing chat")
	}
	if incoming.Message.From == nil {
		return nil, errors.New("telegram message missing sender")
	}

	content := strings.TrimSpace(incoming.Message.Text)
	if content == "" {
		return nil, errors.New("telegram message missing text content")
	}

	content = normalizeCommandText(content)
	chatID := incoming.Message.Chat.ID
	userID := incoming.Message.From.ID
	if chatID == 0 || userID == 0 {
		return nil, errors.New("telegram message missing stable chat or user id")
	}

	return &core.Message{
		Platform:   liveMetadata.Source,
		SessionKey: fmt.Sprintf("%s:%d:%d", liveMetadata.Source, chatID, userID),
		UserID:     strconv.FormatInt(userID, 10),
		UserName:   firstNonEmpty(incoming.Message.From.Username, joinNames(incoming.Message.From.FirstName, incoming.Message.From.LastName)),
		ChatID:     strconv.FormatInt(chatID, 10),
		ChatName:   firstNonEmpty(incoming.Message.Chat.Title, incoming.Message.Chat.Username, joinNames(incoming.Message.Chat.FirstName, incoming.Message.Chat.LastName)),
		Content:    content,
		ReplyCtx: replyContext{
			ChatID:    chatID,
			MessageID: int(incoming.Message.MessageID),
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:  liveMetadata.Source,
			ChatID:    strconv.FormatInt(chatID, 10),
			ChannelID: strconv.FormatInt(chatID, 10),
			MessageID: strconv.FormatInt(incoming.Message.MessageID, 10),
			UseReply:  true,
		},
		Timestamp: time.Unix(incoming.Message.Date, 0),
		IsGroup:   strings.TrimSpace(incoming.Message.Chat.Type) != "private",
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
		chatID, _ := strconv.ParseInt(strings.TrimSpace(value.ChatID), 10, 64)
		return replyContext{ChatID: chatID}
	default:
		return replyContext{}
	}
}

func parseChatID(raw string) (int64, error) {
	target, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, errors.New("telegram send requires numeric chat id")
	}
	if target == 0 {
		return 0, errors.New("telegram send requires chat id")
	}
	return target, nil
}

func validateUpdateMode(mode, webhookURL string) error {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		normalized = updateModeLongPoll
	}
	if normalized != updateModeLongPoll {
		return fmt.Errorf("telegram live transport currently supports only %s update mode", updateModeLongPoll)
	}
	if strings.TrimSpace(webhookURL) != "" {
		return errors.New("telegram long polling cannot be combined with webhook configuration")
	}
	return nil
}

func normalizeCommandText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "/") {
		return trimmed
	}

	parts := strings.SplitN(trimmed, " ", 2)
	command := parts[0]
	if at := strings.Index(command, "@"); at > 0 {
		command = command[:at]
	}
	if len(parts) == 1 {
		return command
	}

	args := strings.TrimSpace(parts[1])
	if args == "" {
		return command
	}
	return command + " " + args
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func joinNames(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func descriptionOrFallback(description, fallback string) string {
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		return trimmed
	}
	return fallback
}
