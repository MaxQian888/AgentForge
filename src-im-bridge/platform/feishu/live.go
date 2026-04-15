package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkcallback "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

var (
	_ core.FormattedTextSender   = (*Live)(nil)
	_ core.MessageUpdater        = (*Live)(nil)
	_ core.StructuredSender      = (*Live)(nil)
	_ core.ReplyStructuredSender = (*Live)(nil)
)

var liveMetadata = core.PlatformMetadata{
	Source: "feishu",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:     core.CommandSurfaceMixed,
		StructuredSurface:  core.StructuredSurfaceCards,
		AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateEdit, core.AsyncUpdateDeferredCardUpdate},
		ActionCallbackMode: core.ActionCallbackSocketPayload,
		MessageScopes:      []core.MessageScope{core.MessageScopeChat, core.MessageScopeThread},
		Mutability: core.MutabilitySemantics{
			CanEdit:        true,
			PrefersInPlace: true,
		},
		SupportsRichMessages:  true,
		SupportsSlashCommands: true,
		SupportsMentions:      true,
	},
}

type replyContext struct {
	MessageID     string
	ChatID        string
	CallbackToken string
}

// lifecycleEventRunner extends eventRunner with lifecycle event registration.
type lifecycleEventRunner interface {
	StartFull(
		ctx context.Context,
		handler func(context.Context, *larkim.P2MessageReceiveV1) error,
		cardActionHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error),
		botAddedHandler func(context.Context, *larkim.P2ChatMemberBotAddedV1) error,
		botRemovedHandler func(context.Context, *larkim.P2ChatMemberBotDeletedV1) error,
		reactionHandler func(context.Context, *larkim.P2MessageReactionCreatedV1) error,
	) error
}

type eventRunner interface {
	Start(ctx context.Context, handler func(context.Context, *larkim.P2MessageReceiveV1) error) error
	Stop(ctx context.Context) error
}

type cardActionEventRunner interface {
	StartWithCardActions(
		ctx context.Context,
		handler func(context.Context, *larkim.P2MessageReceiveV1) error,
		cardActionHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error),
	) error
}

type messageClient interface {
	Send(ctx context.Context, receiveIDType, receiveID, msgType, content string) error
	Reply(ctx context.Context, messageID, msgType, content string) error
	Patch(ctx context.Context, messageID, content string) error
}

type cardUpdater interface {
	Update(ctx context.Context, callbackToken string, message *core.NativeMessage) error
}

type LegacyCardCallbackHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error)

type LiveOption func(*Live) error

// Live is the production Feishu platform adapter backed by long connection and
// the Feishu IM APIs.
type Live struct {
	appID     string
	appSecret string

	runner            eventRunner
	messages          messageClient
	cardUpdater       cardUpdater
	callbackHTTP      http.Handler
	callbackPath      string
	verificationToken string
	eventEncryptKey   string
	actionHandler     notify.ActionHandler
	lifecycleHandler  core.LifecycleHandler
	startCancel       context.CancelFunc
	started           bool
	startedContext    context.Context
	mu                sync.Mutex
}

func NewLive(appID, appSecret string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" {
		return nil, errors.New("feishu live transport requires app id and app secret")
	}

	sdkClient := lark.NewClient(appID, appSecret)
	live := &Live{
		appID:     appID,
		appSecret: appSecret,
		runner:    &sdkEventRunner{appID: appID, appSecret: appSecret},
		messages:  &sdkMessageClient{client: sdkClient},
		cardUpdater: &sdkCardUpdater{
			client:     sdkClient,
			appID:      appID,
			appSecret:  appSecret,
			httpClient: http.DefaultClient,
		},
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	if live.runner == nil {
		return nil, errors.New("feishu live transport requires an event runner")
	}
	if live.messages == nil {
		return nil, errors.New("feishu live transport requires a message client")
	}

	return live, nil
}

func WithEventRunner(runner eventRunner) LiveOption {
	return func(live *Live) error {
		if runner == nil {
			return errors.New("event runner cannot be nil")
		}
		live.runner = runner
		return nil
	}
}

func WithMessageClient(client messageClient) LiveOption {
	return func(live *Live) error {
		if client == nil {
			return errors.New("message client cannot be nil")
		}
		live.messages = client
		return nil
	}
}

func WithCardUpdater(updater cardUpdater) LiveOption {
	return func(live *Live) error {
		if updater == nil {
			return errors.New("card updater cannot be nil")
		}
		live.cardUpdater = updater
		return nil
	}
}

func WithLegacyCardCallbackHandler(verificationToken, eventEncryptKey string, handler LegacyCardCallbackHandler) LiveOption {
	return func(live *Live) error {
		if strings.TrimSpace(verificationToken) == "" {
			return errors.New("legacy card callback requires verification token")
		}
		if handler == nil {
			return errors.New("legacy card callback handler cannot be nil")
		}

		dispatcher := larkdispatcher.NewEventDispatcher(verificationToken, eventEncryptKey).
			OnP2CardActionTrigger(handler)
		if strings.TrimSpace(live.callbackPath) == "" {
			live.callbackPath = "/feishu/callback"
		}
		live.callbackHTTP = newHTTPCallbackHandler(dispatcher)
		return nil
	}
}

func WithCardCallbackWebhook(verificationToken, eventEncryptKey, callbackPath string) LiveOption {
	return func(live *Live) error {
		if strings.TrimSpace(verificationToken) == "" {
			return errors.New("feishu card callback webhook requires verification token")
		}
		path := strings.TrimSpace(callbackPath)
		if path == "" {
			path = "/feishu/callback"
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		live.verificationToken = strings.TrimSpace(verificationToken)
		live.eventEncryptKey = strings.TrimSpace(eventEncryptKey)
		live.callbackPath = path
		dispatcher := larkdispatcher.NewEventDispatcher(live.verificationToken, live.eventEncryptKey).
			OnP2CardActionTrigger(live.handleCardAction)
		live.callbackHTTP = newHTTPCallbackHandler(dispatcher)
		return nil
	}
}

func (l *Live) Name() string { return "feishu-live" }

func (l *Live) Metadata() core.PlatformMetadata {
	metadata := liveMetadata
	if l != nil && l.callbackHTTP != nil {
		metadata.Capabilities.ActionCallbackMode = core.ActionCallbackWebhook
		metadata.Capabilities.RequiresPublicCallback = true
	} else {
		metadata.Capabilities.ActionCallbackMode = core.ActionCallbackSocketPayload
		metadata.Capabilities.RequiresPublicCallback = false
	}
	return core.NormalizeMetadata(metadata, metadata.Source)
}

func (l *Live) SetActionHandler(handler notify.ActionHandler) {
	l.actionHandler = handler
}

func (l *Live) SetLifecycleHandler(handler core.LifecycleHandler) {
	l.lifecycleHandler = handler
}

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	return replyContext{
		MessageID:     strings.TrimSpace(target.MessageID),
		ChatID:        firstNonEmpty(target.ChatID, target.ChannelID),
		CallbackToken: strings.TrimSpace(target.CallbackToken),
	}
}

func (l *Live) HTTPCallbackHandler() http.Handler { return l.callbackHTTP }

func (l *Live) CallbackPaths() []string {
	if l == nil || l.callbackHTTP == nil || strings.TrimSpace(l.callbackPath) == "" {
		return nil
	}
	return []string{l.callbackPath}
}

func (l *Live) BuildNativeTextMessage(title, content string) (*core.NativeMessage, error) {
	return core.NewFeishuMarkdownCardMessage(title, content)
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
	l.started = true
	l.startCancel = cancel
	l.startedContext = ctx

	messageHandler := func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		msg, err := normalizeIncomingMessage(event)
		if err != nil {
			log.WithField("component", "feishu-live").WithError(err).Warn("Ignoring inbound event")
			return nil
		}
		handler(l, msg)
		return nil
	}

	// Try full lifecycle runner first (supports bot added/removed + reactions).
	if runner, ok := l.runner.(lifecycleEventRunner); ok {
		var cardHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error)
		if l.actionHandler != nil {
			cardHandler = l.handleCardAction
		}
		return runner.StartFull(ctx, messageHandler, cardHandler,
			l.handleBotAdded, l.handleBotRemoved, l.handleReaction)
	}

	// Fall back to card action runner.
	if runner, ok := l.runner.(cardActionEventRunner); ok && l.actionHandler != nil {
		return runner.StartWithCardActions(ctx, messageHandler, l.handleCardAction)
	}

	return l.runner.Start(ctx, messageHandler)
}

func (l *Live) handleBotAdded(ctx context.Context, event *larkim.P2ChatMemberBotAddedV1) error {
	if event == nil || event.Event == nil {
		return nil
	}
	chatID := ""
	if event.Event.ChatId != nil {
		chatID = strings.TrimSpace(*event.Event.ChatId)
	}
	log.WithFields(log.Fields{"component": "feishu-live", "chat_id": chatID}).Info("Bot added to group")
	if l.lifecycleHandler != nil && chatID != "" {
		return l.lifecycleHandler.OnBotAdded(ctx, l, chatID)
	}
	return nil
}

func (l *Live) handleBotRemoved(ctx context.Context, event *larkim.P2ChatMemberBotDeletedV1) error {
	if event == nil || event.Event == nil {
		return nil
	}
	chatID := ""
	if event.Event.ChatId != nil {
		chatID = strings.TrimSpace(*event.Event.ChatId)
	}
	log.WithFields(log.Fields{"component": "feishu-live", "chat_id": chatID}).Info("Bot removed from group")
	if l.lifecycleHandler != nil && chatID != "" {
		return l.lifecycleHandler.OnBotRemoved(ctx, l, chatID)
	}
	return nil
}

func (l *Live) handleReaction(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
	if event == nil || event.Event == nil {
		return nil
	}
	log.WithField("component", "feishu-live").Debug("Reaction event received")
	// Reactions are logged but not forwarded as action requests yet; the
	// action handler interface does not have a reaction-specific path.
	return nil
}

func (l *Live) Reply(ctx context.Context, replyCtx any, content string) error {
	payload, err := renderTextPayload(content)
	if err != nil {
		return err
	}

	replyTarget := toReplyContext(replyCtx)
	if replyTarget.MessageID != "" {
		return l.messages.Reply(ctx, replyTarget.MessageID, larkim.MsgTypeText, payload)
	}
	if replyTarget.ChatID == "" {
		return errors.New("feishu reply requires message id or chat id")
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, replyTarget.ChatID, larkim.MsgTypeText, payload)
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("feishu send requires chat id")
	}

	payload, err := renderTextPayload(content)
	if err != nil {
		return err
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, chatID, larkim.MsgTypeText, payload)
}

func (l *Live) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("feishu card send requires chat id")
	}
	payload, err := renderInteractiveCard(card)
	if err != nil {
		return err
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, chatID, larkim.MsgTypeInteractive, payload)
}

func (l *Live) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	payload, err := renderInteractiveCard(card)
	if err != nil {
		return err
	}

	replyTarget := toReplyContext(replyCtx)
	if replyTarget.MessageID != "" {
		return l.messages.Reply(ctx, replyTarget.MessageID, larkim.MsgTypeInteractive, payload)
	}
	if replyTarget.ChatID == "" {
		return errors.New("feishu reply card requires message id or chat id")
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, replyTarget.ChatID, larkim.MsgTypeInteractive, payload)
}

// SendStructured implements core.StructuredSender.
func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("feishu structured send requires chat id")
	}
	payload, err := renderStructuredMessage(message)
	if err != nil {
		return err
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, chatID, larkim.MsgTypeInteractive, payload)
}

// ReplyStructured implements core.ReplyStructuredSender.
func (l *Live) ReplyStructured(ctx context.Context, replyCtx any, message *core.StructuredMessage) error {
	payload, err := renderStructuredMessage(message)
	if err != nil {
		return err
	}
	replyTarget := toReplyContext(replyCtx)
	if replyTarget.MessageID != "" {
		return l.messages.Reply(ctx, replyTarget.MessageID, larkim.MsgTypeInteractive, payload)
	}
	if replyTarget.ChatID == "" {
		return errors.New("feishu structured reply requires message id or chat id")
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, replyTarget.ChatID, larkim.MsgTypeInteractive, payload)
}

func (l *Live) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("feishu native send requires chat id")
	}
	payload, err := renderFeishuNativeContent(message)
	if err != nil {
		return err
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, chatID, larkim.MsgTypeInteractive, payload)
}

func (l *Live) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	payload, err := renderFeishuNativeContent(message)
	if err != nil {
		return err
	}
	replyTarget := toReplyContext(replyCtx)
	if replyTarget.MessageID != "" {
		return l.messages.Reply(ctx, replyTarget.MessageID, larkim.MsgTypeInteractive, payload)
	}
	if replyTarget.ChatID == "" {
		return errors.New("feishu native reply requires message id or chat id")
	}
	return l.messages.Send(ctx, larkim.ReceiveIdTypeChatId, replyTarget.ChatID, larkim.MsgTypeInteractive, payload)
}

func (l *Live) UpdateNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	if l.cardUpdater == nil {
		return errors.New("feishu native update requires a card updater")
	}
	replyTarget := toReplyContext(replyCtx)
	if strings.TrimSpace(replyTarget.CallbackToken) == "" {
		return errors.New("feishu native update requires callback token")
	}
	return l.cardUpdater.Update(ctx, replyTarget.CallbackToken, message)
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	if message == nil {
		return errors.New("formatted text is required")
	}
	if message.Format == core.TextFormatLarkMD {
		nativeMsg, err := l.BuildNativeTextMessage("", message.Content)
		if err == nil {
			return l.SendNative(ctx, chatID, nativeMsg)
		}
	}
	return l.Send(ctx, chatID, message.Content)
}

func (l *Live) ReplyFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	if message == nil {
		return errors.New("formatted text is required")
	}
	if message.Format == core.TextFormatLarkMD {
		nativeMsg, err := l.BuildNativeTextMessage("", message.Content)
		if err == nil {
			return l.ReplyNative(ctx, rawReplyCtx, nativeMsg)
		}
	}
	return l.Reply(ctx, rawReplyCtx, message.Content)
}

func (l *Live) UpdateFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	if message == nil {
		return errors.New("formatted text is required")
	}
	if message.Format == core.TextFormatLarkMD {
		nativeMsg, err := l.BuildNativeTextMessage("", message.Content)
		if err == nil {
			if updateErr := l.UpdateNative(ctx, rawReplyCtx, nativeMsg); updateErr == nil {
				return nil
			}
		}
	}
	return l.ReplyFormattedText(ctx, rawReplyCtx, message)
}

func (l *Live) UpdateMessage(ctx context.Context, rawReplyCtx any, content string) error {
	rc := toReplyContext(rawReplyCtx)
	if strings.TrimSpace(rc.MessageID) == "" {
		return errors.New("feishu update message requires message id")
	}
	payload, err := renderTextPayload(content)
	if err != nil {
		return err
	}
	return l.messages.Patch(ctx, rc.MessageID, payload)
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
	return l.runner.Stop(l.startedContext)
}

type sdkEventRunner struct {
	appID     string
	appSecret string
}

func (r *sdkEventRunner) Start(ctx context.Context, handler func(context.Context, *larkim.P2MessageReceiveV1) error) error {
	return r.StartFull(ctx, handler, nil, nil, nil, nil)
}

func (r *sdkEventRunner) StartWithCardActions(ctx context.Context, handler func(context.Context, *larkim.P2MessageReceiveV1) error, cardActionHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error)) error {
	return r.StartFull(ctx, handler, cardActionHandler, nil, nil, nil)
}

func (r *sdkEventRunner) StartFull(
	ctx context.Context,
	handler func(context.Context, *larkim.P2MessageReceiveV1) error,
	cardActionHandler func(context.Context, *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error),
	botAddedHandler func(context.Context, *larkim.P2ChatMemberBotAddedV1) error,
	botRemovedHandler func(context.Context, *larkim.P2ChatMemberBotDeletedV1) error,
	reactionHandler func(context.Context, *larkim.P2MessageReactionCreatedV1) error,
) error {
	dispatcher := larkdispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(handler)
	if cardActionHandler != nil {
		dispatcher = dispatcher.OnP2CardActionTrigger(cardActionHandler)
	}
	if botAddedHandler != nil {
		dispatcher = dispatcher.OnP2ChatMemberBotAddedV1(botAddedHandler)
	}
	if botRemovedHandler != nil {
		dispatcher = dispatcher.OnP2ChatMemberBotDeletedV1(botRemovedHandler)
	}
	if reactionHandler != nil {
		dispatcher = dispatcher.OnP2MessageReactionCreatedV1(reactionHandler)
	}
	client := larkws.NewClient(r.appID, r.appSecret, larkws.WithEventHandler(dispatcher))

	go func() {
		if err := client.Start(ctx); err != nil && ctx.Err() == nil {
			log.WithField("component", "feishu-live").WithError(err).Error("Long connection stopped with error")
		}
	}()
	return nil
}

func (r *sdkEventRunner) Stop(context.Context) error {
	// The upstream ws.Client does not expose a graceful Close/Stop API in the
	// current SDK, so process shutdown is still driven by parent context
	// cancellation and process exit.
	return nil
}

type sdkMessageClient struct {
	client *lark.Client
}

func (c *sdkMessageClient) Send(ctx context.Context, receiveIDType, receiveID, msgType, content string) error {
	resp, err := c.client.Im.Message.Create(
		ctx,
		larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(receiveIDType).
			Body(
				larkim.NewCreateMessageReqBodyBuilder().
					ReceiveId(receiveID).
					MsgType(msgType).
					Content(content).
					Build(),
			).
			Build(),
	)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu send failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *sdkMessageClient) Reply(ctx context.Context, messageID, msgType, content string) error {
	resp, err := c.client.Im.Message.Reply(
		ctx,
		larkim.NewReplyMessageReqBuilder().
			MessageId(messageID).
			Body(
				larkim.NewReplyMessageReqBodyBuilder().
					MsgType(msgType).
					Content(content).
					Build(),
			).
			Build(),
	)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu reply failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *sdkMessageClient) Patch(ctx context.Context, messageID, content string) error {
	resp, err := c.client.Im.Message.Patch(
		ctx,
		larkim.NewPatchMessageReqBuilder().
			MessageId(messageID).
			Body(
				larkim.NewPatchMessageReqBodyBuilder().
					Content(content).
					Build(),
			).
			Build(),
	)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu patch failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func normalizeIncomingMessage(event *larkim.P2MessageReceiveV1) (*core.Message, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil, errors.New("missing feishu message payload")
	}

	message := event.Event.Message
	msgType := value(message.MessageType)

	chatID := value(message.ChatId)
	if chatID == "" {
		return nil, errors.New("missing feishu chat id")
	}
	userID := senderID(event.Event.Sender)
	if userID == "" {
		return nil, errors.New("missing feishu sender id")
	}

	var content string
	var metadata map[string]string
	var attachments []core.Attachment
	var err error

	switch msgType {
	case larkim.MsgTypeText:
		content, err = decodeTextMessage(message.Content, message.Mentions)
		if err != nil {
			return nil, err
		}
	case "post":
		content, err = decodePostMessage(message.Content)
		if err != nil {
			return nil, err
		}
	case "image":
		content, metadata, attachments, err = decodeImageMessage(message.Content)
		if err != nil {
			return nil, err
		}
	case "file":
		content, metadata, attachments, err = decodeFileMessage(message.Content)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported feishu message type %q", msgType)
	}

	threadID := value(message.ThreadId)
	rootID := value(message.RootId)
	// For topic groups, RootId identifies the thread the message belongs to.
	if threadID == "" && rootID != "" {
		threadID = rootID
	}

	return &core.Message{
		Platform:   liveMetadata.Source,
		SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, chatID, userID),
		UserID:     userID,
		ChatID:     chatID,
		Content:    content,
		ReplyCtx: replyContext{
			MessageID:     value(message.MessageId),
			ChatID:        chatID,
			CallbackToken: "",
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:  liveMetadata.Source,
			ChatID:    chatID,
			ChannelID: chatID,
			MessageID: value(message.MessageId),
			ThreadID:  threadID,
			UseReply:  true,
		},
		Timestamp:   parseUnixMillis(value(message.CreateTime)),
		IsGroup:     value(message.ChatType) != "p2p",
		ThreadID:    threadID,
		MessageType: msgType,
		Metadata:    metadata,
		Attachments: attachments,
	}, nil
}

func (l *Live) handleCardAction(ctx context.Context, event *larkcallback.CardActionTriggerEvent) (*larkcallback.CardActionTriggerResponse, error) {
	req, err := normalizeCardActionRequest(event)
	if err != nil {
		if errors.Is(err, errIgnoreCardAction) {
			return &larkcallback.CardActionTriggerResponse{}, nil
		}
		return nil, err
	}
	if l.actionHandler == nil || req == nil {
		return &larkcallback.CardActionTriggerResponse{}, nil
	}

	result, err := l.actionHandler.HandleAction(ctx, req)
	if err != nil {
		return nil, err
	}
	response, err := renderCardActionTriggerResponse(result)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return &larkcallback.CardActionTriggerResponse{}, nil
	}
	return response, nil
}

var errIgnoreCardAction = errors.New("ignore feishu card action")

func normalizeCardActionRequest(event *larkcallback.CardActionTriggerEvent) (*notify.ActionRequest, error) {
	if event == nil || event.Event == nil || event.Event.Action == nil {
		return nil, errors.New("missing feishu card action payload")
	}

	act := event.Event.Action

	// Determine the action tag type for richer interactive elements.
	actionTag := strings.TrimSpace(act.Tag)

	// Extract action reference — buttons use .Value, selects use .Option,
	// date/time pickers use .Value (as date string), form submits use .FormValue.
	actionValue := feishuActionReference(act.Value)
	selectedOption := strings.TrimSpace(act.Option)
	selectedTime := feishuSelectedTimeFromValue(act.Value)
	formValues := feishuFormValues(act.FormValue)

	// Try to parse as structured action reference.
	action, entityID, actionMetadata, ok := core.ParseActionReferenceWithMetadata(actionValue)
	if !ok {
		// For select/picker/form elements, use the tag as action type and the
		// selected value as entity ID.
		switch actionTag {
		case "select_static", "select_person", "multi_select_static", "multi_select_person":
			action = "select"
			entityID = selectedOption
			if entityID == "" {
				entityID = actionValue
			}
		case "date_picker", "time_picker", "datetime_picker":
			action = "date_pick"
			entityID = selectedTime
		case "overflow":
			action = "overflow"
			entityID = selectedOption
		default:
			// Check if form values carry an action reference.
			if formAction, formOk := formValues["action"]; formOk {
				action = "form_submit"
				entityID = formAction
			} else {
				return nil, errIgnoreCardAction
			}
		}
		actionMetadata = nil
	}

	chatID := ""
	messageID := ""
	if event.Event.Context != nil {
		chatID = strings.TrimSpace(event.Event.Context.OpenChatID)
		messageID = strings.TrimSpace(event.Event.Context.OpenMessageID)
	}
	replyTarget := &core.ReplyTarget{
		Platform:          liveMetadata.Source,
		ChatID:            chatID,
		ChannelID:         chatID,
		MessageID:         messageID,
		CallbackToken:     strings.TrimSpace(event.Event.Token),
		UseReply:          true,
		PreferredRenderer: string(liveMetadata.Capabilities.StructuredSurface),
		ProgressMode:      string(core.AsyncUpdateDeferredCardUpdate),
	}
	metadata := map[string]string{
		"source":        "card.action.trigger",
		"host":          strings.TrimSpace(event.Event.Host),
		"delivery_type": strings.TrimSpace(event.Event.DeliveryType),
	}
	if actionTag != "" {
		metadata["action_tag"] = actionTag
	}
	if selectedOption != "" {
		metadata["selected_option"] = selectedOption
	}
	if selectedTime != "" {
		metadata["selected_time"] = selectedTime
	}
	for k, v := range formValues {
		metadata["form_"+k] = v
	}
	for key, value := range actionMetadata {
		metadata[key] = value
	}
	return &notify.ActionRequest{
		Platform:    liveMetadata.Source,
		Action:      action,
		EntityID:    entityID,
		ChatID:      chatID,
		UserID:      feishuOperatorID(event.Event.Operator),
		ReplyTarget: replyTarget,
		Metadata:    compactMetadata(metadata),
	}, nil
}

func renderCardActionTriggerResponse(result *notify.ActionResponse) (*larkcallback.CardActionTriggerResponse, error) {
	if result == nil {
		return &larkcallback.CardActionTriggerResponse{}, nil
	}

	response := &larkcallback.CardActionTriggerResponse{}
	if message := strings.TrimSpace(result.Result); message != "" {
		response.Toast = &larkcallback.Toast{
			Type:    "info",
			Content: message,
		}
	}

	switch {
	case result.Native != nil:
		card, err := renderCallbackResponseCardFromNativeMessage(result.Native)
		if err != nil {
			return nil, err
		}
		response.Card = card
	case result.Structured != nil:
		card, err := renderCallbackResponseCardFromStructuredMessage(result.Structured)
		if err != nil {
			return nil, err
		}
		response.Card = card
	}

	if response.Toast == nil && response.Card == nil {
		return &larkcallback.CardActionTriggerResponse{}, nil
	}
	return response, nil
}

func renderCallbackResponseCardFromNativeMessage(message *core.NativeMessage) (*larkcallback.Card, error) {
	if message == nil {
		return nil, nil
	}
	if strings.TrimSpace(message.Platform) == "" {
		message.Platform = liveMetadata.Source
	}
	if err := message.Validate(); err != nil {
		return nil, err
	}
	if message.NormalizedPlatform() != liveMetadata.Source || message.FeishuCard == nil {
		return nil, fmt.Errorf("feishu callback response requires feishu native card payload")
	}

	switch core.FeishuCardMode(strings.ToLower(strings.TrimSpace(string(message.FeishuCard.Mode)))) {
	case core.FeishuCardModeJSON:
		var decoded map[string]any
		if err := json.Unmarshal(message.FeishuCard.JSON, &decoded); err != nil {
			return nil, fmt.Errorf("decode feishu callback raw card payload: %w", err)
		}
		return &larkcallback.Card{
			Type: "raw",
			Data: decoded,
		}, nil
	case core.FeishuCardModeTemplate:
		payload, err := renderFeishuNativePayload(message)
		if err != nil {
			return nil, err
		}
		data, ok := payload["data"]
		if !ok {
			return nil, fmt.Errorf("feishu callback template response requires data payload")
		}
		return &larkcallback.Card{
			Type: "template",
			Data: data,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported feishu callback card mode %q", message.FeishuCard.Mode)
	}
}

func renderCallbackResponseCardFromStructuredMessage(message *core.StructuredMessage) (*larkcallback.Card, error) {
	if message == nil {
		return nil, nil
	}
	payload, err := renderStructuredMessage(message)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return nil, fmt.Errorf("decode feishu structured callback payload: %w", err)
	}
	return &larkcallback.Card{
		Type: "raw",
		Data: decoded,
	}, nil
}

func feishuSelectedTimeFromValue(values map[string]interface{}) string {
	if len(values) == 0 {
		return ""
	}
	// Date/time pickers put the value in the value map under common keys.
	for _, key := range []string{"date", "time", "datetime", "value"} {
		if v, ok := values[key]; ok {
			if s, isStr := v.(string); isStr && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func feishuFormValues(formValue map[string]interface{}) map[string]string {
	if len(formValue) == 0 {
		return nil
	}
	result := make(map[string]string, len(formValue))
	for k, v := range formValue {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			result[strings.TrimSpace(k)] = strings.TrimSpace(s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func decodeTextMessage(raw *string, mentions []*larkim.MentionEvent) (string, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", errors.New("missing feishu text message content")
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(*raw), &payload); err != nil {
		return "", fmt.Errorf("decode feishu text payload: %w", err)
	}

	text := payload.Text
	for _, mention := range mentions {
		key := value(mention.Key)
		name := value(mention.Name)
		if key == "" || name == "" {
			continue
		}
		text = strings.ReplaceAll(text, key, "@"+strings.TrimPrefix(name, "@"))
	}
	return strings.TrimSpace(text), nil
}

// decodePostMessage parses a Feishu rich text (post) message and extracts
// plain text content from the nested structure.
func decodePostMessage(raw *string) (string, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", errors.New("missing feishu post message content")
	}

	var payload struct {
		Title   string `json:"title"`
		Content [][]struct {
			Tag  string `json:"tag"`
			Text string `json:"text,omitempty"`
			Href string `json:"href,omitempty"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(*raw), &payload); err != nil {
		return "", fmt.Errorf("decode feishu post payload: %w", err)
	}

	var sb strings.Builder
	if title := strings.TrimSpace(payload.Title); title != "" {
		sb.WriteString(title)
		sb.WriteString("\n")
	}
	for _, paragraph := range payload.Content {
		for _, element := range paragraph {
			switch element.Tag {
			case "text":
				sb.WriteString(element.Text)
			case "a":
				sb.WriteString(element.Text)
				if element.Href != "" {
					sb.WriteString(" (")
					sb.WriteString(element.Href)
					sb.WriteString(")")
				}
			case "at":
				sb.WriteString(element.Text)
			}
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String()), nil
}

// decodeImageMessage parses a Feishu image message and extracts the image key.
func decodeImageMessage(raw *string) (string, map[string]string, []core.Attachment, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", nil, nil, errors.New("missing feishu image message content")
	}

	var payload struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal([]byte(*raw), &payload); err != nil {
		return "", nil, nil, fmt.Errorf("decode feishu image payload: %w", err)
	}

	imageKey := strings.TrimSpace(payload.ImageKey)
	if imageKey == "" {
		return "", nil, nil, errors.New("missing feishu image key")
	}

	metadata := map[string]string{
		"image_key": imageKey,
	}
	attachments := []core.Attachment{
		{Type: "image", Key: imageKey},
	}
	return "[image:" + imageKey + "]", metadata, attachments, nil
}

// decodeFileMessage parses a Feishu file message and extracts file metadata.
func decodeFileMessage(raw *string) (string, map[string]string, []core.Attachment, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", nil, nil, errors.New("missing feishu file message content")
	}

	var payload struct {
		FileKey  string `json:"file_key"`
		FileName string `json:"file_name"`
	}
	if err := json.Unmarshal([]byte(*raw), &payload); err != nil {
		return "", nil, nil, fmt.Errorf("decode feishu file payload: %w", err)
	}

	fileKey := strings.TrimSpace(payload.FileKey)
	if fileKey == "" {
		return "", nil, nil, errors.New("missing feishu file key")
	}
	fileName := strings.TrimSpace(payload.FileName)

	metadata := map[string]string{
		"file_key": fileKey,
	}
	if fileName != "" {
		metadata["file_name"] = fileName
	}
	attachments := []core.Attachment{
		{Type: "file", Key: fileKey, Name: fileName},
	}
	content := "[file:" + fileKey + "]"
	if fileName != "" {
		content = "[file:" + fileName + "]"
	}
	return content, metadata, attachments, nil
}

func renderTextPayload(content string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"text": content,
	})
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func renderInteractiveCard(card *core.Card) (string, error) {
	if card == nil {
		return "", errors.New("card is required")
	}

	elements := make([]map[string]any, 0, len(card.Fields)+1)
	for _, field := range card.Fields {
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**%s**\n%s", field.Label, field.Value),
			},
		})
	}
	if len(card.Buttons) > 0 {
		actions := make([]map[string]any, 0, len(card.Buttons))
		for _, button := range card.Buttons {
			action := map[string]any{
				"tag":  "button",
				"text": map[string]any{"tag": "plain_text", "content": button.Text},
				"type": normalizeButtonStyle(button.Style),
			}
			if strings.HasPrefix(button.Action, "link:") {
				action["url"] = strings.TrimPrefix(button.Action, "link:")
			} else if button.Action != "" {
				action["value"] = map[string]any{"action": button.Action}
			}
			actions = append(actions, action)
		}
		elements = append(elements, map[string]any{
			"tag":     "action",
			"actions": actions,
		})
	}

	payload := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": card.Title,
			},
		},
		"elements": elements,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func newHTTPCallbackHandler(dispatcher *larkdispatcher.EventDispatcher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := dispatcher.Handle(r.Context(), &larkevent.EventReq{
			Header:     r.Header.Clone(),
			Body:       body,
			RequestURI: r.RequestURI,
		})
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(resp.Body)
	})
}

func toReplyContext(replyCtx any) replyContext {
	switch value := replyCtx.(type) {
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
		return replyContext{ChatID: value.ChatID}
	case *core.ReplyTarget:
		if value == nil {
			return replyContext{}
		}
		return replyContext{
			MessageID:     strings.TrimSpace(value.MessageID),
			ChatID:        firstNonEmpty(value.ChatID, value.ChannelID),
			CallbackToken: strings.TrimSpace(value.CallbackToken),
		}
	default:
		return replyContext{}
	}
}

func senderID(sender *larkim.EventSender) string {
	if sender == nil || sender.SenderId == nil {
		return ""
	}
	if sender.SenderId.OpenId != nil && *sender.SenderId.OpenId != "" {
		return *sender.SenderId.OpenId
	}
	if sender.SenderId.UserId != nil && *sender.SenderId.UserId != "" {
		return *sender.SenderId.UserId
	}
	if sender.SenderId.UnionId != nil && *sender.SenderId.UnionId != "" {
		return *sender.SenderId.UnionId
	}
	return ""
}

func parseUnixMillis(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}
	millis, err := time.ParseDuration(raw + "ms")
	if err != nil {
		return time.Now()
	}
	return time.Unix(0, 0).Add(millis)
}

func normalizeButtonStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "primary", "danger", "default":
		return strings.ToLower(strings.TrimSpace(style))
	default:
		return "default"
	}
}

func feishuActionReference(values map[string]interface{}) string {
	if len(values) == 0 {
		return ""
	}
	if action, ok := values["action"].(string); ok {
		return strings.TrimSpace(action)
	}
	if action, ok := values["action_id"].(string); ok {
		return strings.TrimSpace(action)
	}
	return ""
}

func feishuOperatorID(operator *larkcallback.Operator) string {
	if operator == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(operator.OpenID); trimmed != "" {
		return trimmed
	}
	if operator.UserID != nil {
		if trimmed := strings.TrimSpace(*operator.UserID); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func value(raw *string) string {
	if raw == nil {
		return ""
	}
	return *raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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
