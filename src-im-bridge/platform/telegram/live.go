package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

const (
	updateModeLongPoll = "longpoll"
)

var liveMetadata = core.PlatformMetadata{
	Source: "telegram",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:     core.CommandSurfaceMixed,
		StructuredSurface:  core.StructuredSurfaceInlineKeyboard,
		AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateEdit},
		ActionCallbackMode: core.ActionCallbackQuery,
		MessageScopes:      []core.MessageScope{core.MessageScopeChat, core.MessageScopeTopic},
		Mutability: core.MutabilitySemantics{
			CanEdit:        true,
			PrefersInPlace: true,
		},
		SupportsSlashCommands: true,
		SupportsMentions:      true,
	},
}

type update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *message       `json:"message,omitempty"`
	CallbackQuery *callbackQuery `json:"callback_query,omitempty"`
}

type callbackQuery struct {
	ID      string   `json:"id"`
	From    *user    `json:"from,omitempty"`
	Message *message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

type message struct {
	MessageID       int64 `json:"message_id"`
	MessageThreadID int64 `json:"message_thread_id,omitempty"`
	Date            int64 `json:"date"`
	Text            string
	From            *user `json:"from,omitempty"`
	Chat            *chat `json:"chat,omitempty"`
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
	TopicID   int64
	MessageID int
}

type updateRunner interface {
	Start(ctx context.Context, handler func(context.Context, update) error) error
	Stop(ctx context.Context) error
}

type sender interface {
	SendText(ctx context.Context, chatID int64, topicID int64, replyToMessageID int, message telegramTextMessage) error
	EditText(ctx context.Context, chatID int64, messageID int, message telegramTextMessage) error
	AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error
	SendStructured(ctx context.Context, chatID int64, topicID int64, message telegramTextMessage, markup *inlineKeyboardMarkup) error
}

type LiveOption func(*Live) error

type Live struct {
	botToken string

	runner updateRunner
	sender sender

	actionHandler notify.ActionHandler

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
	chatID, _ := strconv.ParseInt(strings.TrimSpace(firstNonEmpty(target.ChatID, target.ChannelID)), 10, 64)
	topicID, _ := strconv.ParseInt(strings.TrimSpace(target.TopicID), 10, 64)
	messageID, _ := strconv.Atoi(strings.TrimSpace(target.MessageID))
	return replyContext{
		ChatID:    chatID,
		TopicID:   topicID,
		MessageID: messageID,
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

	if err := l.runner.Start(ctx, func(ctx context.Context, incoming update) error {
		handled, err := l.handleActionUpdate(ctx, incoming)
		if err != nil {
			log.WithField("component", "telegram-live").WithError(err).Warn("Ignoring callback query")
			return nil
		}
		if handled {
			return nil
		}

		msg, err := normalizeIncomingUpdate(incoming)
		if err != nil {
			log.WithField("component", "telegram-live").WithError(err).Warn("Ignoring inbound update")
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
	return l.sender.SendText(ctx, reply.ChatID, reply.TopicID, reply.MessageID, telegramTextMessage{Text: content})
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	target, err := parseChatID(chatID)
	if err != nil {
		return err
	}
	return l.sender.SendText(ctx, target, 0, 0, telegramTextMessage{Text: content})
}

func (l *Live) UpdateMessage(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	if reply.ChatID == 0 || reply.MessageID == 0 {
		return errors.New("telegram update requires chat id and message id")
	}
	return l.sender.EditText(ctx, reply.ChatID, reply.MessageID, telegramTextMessage{Text: content})
}

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	target, err := parseChatID(chatID)
	if err != nil {
		return err
	}
	return l.sender.SendStructured(ctx, target, 0, telegramTextMessage{Text: structuredFallbackText(message)}, buildInlineKeyboardMarkup(message))
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	target, err := parseChatID(chatID)
	if err != nil {
		return err
	}
	return l.sendFormattedSegments(ctx, target, 0, 0, renderTelegramText(message))
}

func (l *Live) ReplyFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	reply := toReplyContext(rawReplyCtx)
	if reply.ChatID == 0 {
		return errors.New("telegram reply requires chat id")
	}
	return l.sendFormattedSegments(ctx, reply.ChatID, reply.TopicID, reply.MessageID, renderTelegramText(message))
}

func (l *Live) UpdateFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	reply := toReplyContext(rawReplyCtx)
	if reply.ChatID == 0 || reply.MessageID == 0 {
		return errors.New("telegram update requires chat id and message id")
	}
	segments := renderTelegramText(message)
	if len(segments) == 0 {
		return errors.New("telegram formatted update requires content")
	}
	if len(segments) == 1 {
		return l.sender.EditText(ctx, reply.ChatID, reply.MessageID, segments[0])
	}
	return l.sendFormattedSegments(ctx, reply.ChatID, reply.TopicID, reply.MessageID, segments)
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

func (l *Live) handleActionUpdate(ctx context.Context, incoming update) (bool, error) {
	if incoming.CallbackQuery == nil {
		return false, nil
	}

	answerText := ""
	if l.actionHandler == nil {
		return true, l.sender.AnswerCallbackQuery(ctx, strings.TrimSpace(incoming.CallbackQuery.ID), answerText)
	}

	req, err := normalizeCallbackQueryAction(incoming.CallbackQuery)
	if err != nil {
		return true, err
	}
	result, err := l.actionHandler.HandleAction(ctx, req)
	if err != nil {
		return true, err
	}
	if result != nil {
		answerText = strings.TrimSpace(result.Result)
	}
	if err := l.sender.AnswerCallbackQuery(ctx, strings.TrimSpace(incoming.CallbackQuery.ID), answerText); err != nil {
		return true, err
	}
	if result == nil || strings.TrimSpace(result.Result) == "" {
		return true, nil
	}

	target := req.ReplyTarget
	if result.ReplyTarget != nil {
		target = result.ReplyTarget
	}
	_, err = core.DeliverText(ctx, l, l.Metadata(), target, req.ChatID, result.Result)
	return true, err
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
				log.WithField("component", "telegram-live").WithError(err).Error("Long polling error")
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
					log.WithField("component", "telegram-live").WithError(err).Error("Handler error")
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

func (s *botAPISender) SendText(ctx context.Context, chatID int64, topicID int64, replyToMessageID int, message telegramTextMessage) error {
	request := sendMessageRequest{
		ChatID:    chatID,
		Text:      message.Text,
		ParseMode: message.ParseMode,
	}
	if topicID > 0 {
		request.MessageThreadID = topicID
	}
	if replyToMessageID > 0 {
		request.ReplyParameters = &replyParameters{
			MessageID: replyToMessageID,
		}
	}
	return s.client.sendMessage(ctx, request)
}

func (s *botAPISender) EditText(ctx context.Context, chatID int64, messageID int, message telegramTextMessage) error {
	return s.client.editMessageText(ctx, editMessageTextRequest{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      message.Text,
		ParseMode: message.ParseMode,
	})
}

func (s *botAPISender) AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error {
	return s.client.answerCallbackQuery(ctx, answerCallbackQueryRequest{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	})
}

func (s *botAPISender) SendStructured(ctx context.Context, chatID int64, topicID int64, message telegramTextMessage, markup *inlineKeyboardMarkup) error {
	request := sendMessageRequest{
		ChatID:      chatID,
		Text:        message.Text,
		ParseMode:   message.ParseMode,
		ReplyMarkup: markup,
	}
	if topicID > 0 {
		request.MessageThreadID = topicID
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

type telegramTextMessage struct {
	Text      string
	ParseMode string
}

type sendMessageRequest struct {
	ChatID          int64                 `json:"chat_id"`
	MessageThreadID int64                 `json:"message_thread_id,omitempty"`
	Text            string                `json:"text"`
	ParseMode       string                `json:"parse_mode,omitempty"`
	ReplyParameters *replyParameters      `json:"reply_parameters,omitempty"`
	ReplyMarkup     *inlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type editMessageTextRequest struct {
	ChatID    int64  `json:"chat_id"`
	MessageID int    `json:"message_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type answerCallbackQueryRequest struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
}

type replyParameters struct {
	MessageID int `json:"message_id"`
}

type inlineKeyboardMarkup struct {
	InlineKeyboard [][]inlineKeyboardButton `json:"inline_keyboard,omitempty"`
}

type inlineKeyboardButton struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

func (c *botAPIClient) deleteWebhook(ctx context.Context) error {
	var response botAPIResponse[bool]
	return c.call(ctx, "deleteWebhook", deleteWebhookRequest{}, &response)
}

func (c *botAPIClient) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	request := getUpdatesRequest{
		Offset:         offset,
		Timeout:        50,
		AllowedUpdates: []string{"message", "callback_query"},
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

func (c *botAPIClient) editMessageText(ctx context.Context, request editMessageTextRequest) error {
	if strings.TrimSpace(request.Text) == "" {
		return errors.New("telegram edit requires content")
	}
	var response botAPIResponse[message]
	return c.call(ctx, "editMessageText", request, &response)
}

func (c *botAPIClient) answerCallbackQuery(ctx context.Context, request answerCallbackQueryRequest) error {
	if strings.TrimSpace(request.CallbackQueryID) == "" {
		return errors.New("telegram callback answer requires callback query id")
	}
	var response botAPIResponse[bool]
	return c.call(ctx, "answerCallbackQuery", request, &response)
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
			return errors.New(descriptionOrFallback(parsed.Description, "telegram api request failed"))
		}
	}
	return nil
}

func normalizeIncomingUpdate(incoming update) (*core.Message, error) {
	if incoming.Message == nil {
		return nil, errors.New("telegram update does not contain a message payload")
	}
	return normalizeIncomingMessage(incoming.Message)
}

func normalizeIncomingMessage(incoming *message) (*core.Message, error) {
	if incoming == nil {
		return nil, errors.New("telegram update does not contain a message payload")
	}
	if incoming.Chat == nil {
		return nil, errors.New("telegram message missing chat")
	}
	if incoming.From == nil {
		return nil, errors.New("telegram message missing sender")
	}

	content := strings.TrimSpace(incoming.Text)
	if content == "" {
		return nil, errors.New("telegram message missing text content")
	}

	content = normalizeCommandText(content)
	chatID := incoming.Chat.ID
	userID := incoming.From.ID
	if chatID == 0 || userID == 0 {
		return nil, errors.New("telegram message missing stable chat or user id")
	}

	replyCtx := replyContext{
		ChatID:    chatID,
		TopicID:   incoming.MessageThreadID,
		MessageID: int(incoming.MessageID),
	}
	replyTarget := &core.ReplyTarget{
		Platform:  liveMetadata.Source,
		ChatID:    strconv.FormatInt(chatID, 10),
		ChannelID: strconv.FormatInt(chatID, 10),
		MessageID: strconv.FormatInt(incoming.MessageID, 10),
		UseReply:  true,
	}
	if incoming.MessageThreadID > 0 {
		replyTarget.TopicID = strconv.FormatInt(incoming.MessageThreadID, 10)
	}

	threadID := ""
	if incoming.MessageThreadID > 0 {
		threadID = strconv.FormatInt(incoming.MessageThreadID, 10)
	}

	return &core.Message{
		Platform:    liveMetadata.Source,
		SessionKey:  fmt.Sprintf("%s:%d:%d", liveMetadata.Source, chatID, userID),
		UserID:      strconv.FormatInt(userID, 10),
		UserName:    firstNonEmpty(incoming.From.Username, joinNames(incoming.From.FirstName, incoming.From.LastName)),
		ChatID:      strconv.FormatInt(chatID, 10),
		ChatName:    firstNonEmpty(incoming.Chat.Title, incoming.Chat.Username, joinNames(incoming.Chat.FirstName, incoming.Chat.LastName)),
		Content:     content,
		ReplyCtx:    replyCtx,
		ReplyTarget: replyTarget,
		Timestamp:   time.Unix(incoming.Date, 0),
		IsGroup:     strings.TrimSpace(incoming.Chat.Type) != "private",
		ThreadID:    threadID,
	}, nil
}

func normalizeCallbackQueryAction(query *callbackQuery) (*notify.ActionRequest, error) {
	if query == nil {
		return nil, errors.New("telegram callback query missing payload")
	}
	if query.Message == nil {
		return nil, errors.New("telegram callback query missing message")
	}
	if query.Message.Chat == nil {
		return nil, errors.New("telegram callback query missing chat")
	}
	if query.From == nil {
		return nil, errors.New("telegram callback query missing sender")
	}

	action, entityID, ok := core.ParseActionReference(query.Data)
	if !ok {
		return nil, errors.New("telegram callback query missing action reference")
	}

	chatID := strconv.FormatInt(query.Message.Chat.ID, 10)
	replyTarget := &core.ReplyTarget{
		Platform:          liveMetadata.Source,
		ChatID:            chatID,
		ChannelID:         chatID,
		MessageID:         strconv.FormatInt(query.Message.MessageID, 10),
		UserID:            strconv.FormatInt(query.From.ID, 10),
		UseReply:          true,
		PreferEdit:        true,
		PreferredRenderer: string(liveMetadata.Capabilities.StructuredSurface),
	}
	if query.Message.MessageThreadID > 0 {
		replyTarget.TopicID = strconv.FormatInt(query.Message.MessageThreadID, 10)
	}

	return &notify.ActionRequest{
		Platform:    liveMetadata.Source,
		Action:      action,
		EntityID:    entityID,
		ChatID:      chatID,
		UserID:      strconv.FormatInt(query.From.ID, 10),
		ReplyTarget: replyTarget,
		Metadata: compactMetadata(map[string]string{
			"source":            "callback_query",
			"callback_query_id": strings.TrimSpace(query.ID),
			"callback_data":     strings.TrimSpace(query.Data),
		}),
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
		topicID, _ := strconv.ParseInt(strings.TrimSpace(value.ThreadID), 10, 64)
		messageID := 0
		if value.ReplyTarget != nil {
			messageID, _ = strconv.Atoi(strings.TrimSpace(value.ReplyTarget.MessageID))
			if topicID == 0 {
				topicID, _ = strconv.ParseInt(strings.TrimSpace(value.ReplyTarget.TopicID), 10, 64)
			}
		}
		return replyContext{ChatID: chatID, TopicID: topicID, MessageID: messageID}
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

func buildInlineKeyboardMarkup(message *core.StructuredMessage) *inlineKeyboardMarkup {
	if message == nil || len(message.Actions) == 0 {
		return nil
	}

	rows := make([][]inlineKeyboardButton, 0, len(message.Actions))
	for _, action := range message.Actions {
		label := strings.TrimSpace(action.Label)
		if label == "" {
			continue
		}
		button := inlineKeyboardButton{Text: label}
		switch {
		case strings.TrimSpace(action.URL) != "":
			button.URL = strings.TrimSpace(action.URL)
		case strings.TrimSpace(action.ID) != "":
			button.CallbackData = strings.TrimSpace(action.ID)
		default:
			continue
		}
		rows = append(rows, []inlineKeyboardButton{button})
	}
	if len(rows) == 0 {
		return nil
	}
	return &inlineKeyboardMarkup{InlineKeyboard: rows}
}

func structuredFallbackText(message *core.StructuredMessage) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(message.FallbackText())
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

func (l *Live) sendFormattedSegments(ctx context.Context, chatID int64, topicID int64, replyToMessageID int, messages []telegramTextMessage) error {
	for index, message := range messages {
		currentReplyTo := 0
		if index == 0 {
			currentReplyTo = replyToMessageID
		}
		if err := l.sender.SendText(ctx, chatID, topicID, currentReplyTo, message); err != nil {
			return err
		}
	}
	return nil
}
