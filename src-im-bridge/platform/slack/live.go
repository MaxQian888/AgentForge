package slack

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
	goslack "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

var liveMetadata = core.PlatformMetadata{
	Source: "slack",
	Capabilities: core.PlatformCapabilities{
		SupportsRichMessages:  true,
		SupportsDeferredReply: true,
		SupportsSlashCommands: true,
		SupportsMentions:      true,
	},
}

var errIgnoreEnvelope = errors.New("ignore slack envelope")

type replyContext struct {
	ChannelID   string
	ThreadTS    string
	ResponseURL string
}

type socketEnvelopeType string

const (
	socketEnvelopeEventsAPI    socketEnvelopeType = "events_api"
	socketEnvelopeSlashCommand socketEnvelopeType = "slash_command"
	socketEnvelopeInteractive  socketEnvelopeType = "interactive"
)

type socketEnvelope struct {
	Type         socketEnvelopeType
	Ack          func(payload any) error
	SlashCommand *goslack.SlashCommand
	EventsAPI    *slackevents.EventsAPIEvent
	Interaction  *goslack.InteractionCallback
}

type socketRunner interface {
	Start(ctx context.Context, handler func(context.Context, socketEnvelope) error) error
	Stop(ctx context.Context) error
}

type slackOutgoingMessage struct {
	ChannelID string
	ThreadTS  string
	Text      string
	Blocks    []goslack.Block
}

type messageClient interface {
	PostMessage(ctx context.Context, message slackOutgoingMessage) error
}

type LiveOption func(*Live) error

// Live is the Slack production adapter backed by Socket Mode and chat.postMessage.
type Live struct {
	botToken string
	appToken string

	runner   socketRunner
	messages messageClient

	startCancel context.CancelFunc
	startCtx    context.Context
	started     bool
	mu          sync.Mutex
}

func NewLive(botToken, appToken string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(botToken) == "" || strings.TrimSpace(appToken) == "" {
		return nil, errors.New("slack live transport requires bot token and app token")
	}

	api := goslack.New(botToken, goslack.OptionAppLevelToken(appToken))
	live := &Live{
		botToken: botToken,
		appToken: appToken,
		runner:   &managedSocketRunner{client: socketmode.New(api)},
		messages: &slackAPIMessageClient{client: api},
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	if live.runner == nil {
		return nil, errors.New("slack live transport requires a socket runner")
	}
	if live.messages == nil {
		return nil, errors.New("slack live transport requires a message client")
	}

	return live, nil
}

func WithSocketRunner(runner socketRunner) LiveOption {
	return func(live *Live) error {
		if runner == nil {
			return errors.New("socket runner cannot be nil")
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

func (l *Live) Name() string { return "slack-live" }

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

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
	l.started = true

	return l.runner.Start(ctx, func(ctx context.Context, envelope socketEnvelope) error {
		if envelope.Ack != nil {
			if err := envelope.Ack(nil); err != nil {
				return err
			}
		}

		msg, err := normalizeEnvelope(envelope)
		if err != nil {
			if errors.Is(err, errIgnoreEnvelope) {
				return nil
			}
			log.Printf("[slack-live] Ignoring inbound envelope: %v", err)
			return nil
		}

		handler(l, msg)
		return nil
	})
}

func (l *Live) Reply(ctx context.Context, replyCtx any, content string) error {
	target := toReplyContext(replyCtx)
	if target.ChannelID == "" {
		return errors.New("slack reply requires channel id")
	}
	return l.messages.PostMessage(ctx, slackOutgoingMessage{
		ChannelID: target.ChannelID,
		ThreadTS:  target.ThreadTS,
		Text:      content,
	})
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("slack send requires channel id")
	}
	return l.messages.PostMessage(ctx, slackOutgoingMessage{
		ChannelID: chatID,
		Text:      content,
	})
}

func (l *Live) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("slack card send requires channel id")
	}
	text, blocks, err := renderCardMessage(card)
	if err != nil {
		return err
	}
	return l.messages.PostMessage(ctx, slackOutgoingMessage{
		ChannelID: chatID,
		Text:      text,
		Blocks:    blocks,
	})
}

func (l *Live) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	target := toReplyContext(replyCtx)
	if target.ChannelID == "" {
		return errors.New("slack reply card requires channel id")
	}
	text, blocks, err := renderCardMessage(card)
	if err != nil {
		return err
	}
	return l.messages.PostMessage(ctx, slackOutgoingMessage{
		ChannelID: target.ChannelID,
		ThreadTS:  target.ThreadTS,
		Text:      text,
		Blocks:    blocks,
	})
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

type managedSocketRunner struct {
	client *socketmode.Client

	cancel context.CancelFunc
}

func (r *managedSocketRunner) Start(ctx context.Context, handler func(context.Context, socketEnvelope) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	go r.consumeEvents(runCtx, handler)
	go func() {
		if err := r.client.RunContext(runCtx); err != nil && runCtx.Err() == nil {
			log.Printf("[slack-live] socket mode stopped with error: %v", err)
		}
	}()

	return nil
}

func (r *managedSocketRunner) Stop(context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}

func (r *managedSocketRunner) consumeEvents(ctx context.Context, handler func(context.Context, socketEnvelope) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-r.client.Events:
			if !ok {
				return
			}

			switch evt.Type {
			case socketmode.EventTypeConnecting:
				log.Printf("[slack-live] Connecting to Slack Socket Mode")
			case socketmode.EventTypeConnected:
				log.Printf("[slack-live] Connected to Slack Socket Mode")
			case socketmode.EventTypeConnectionError:
				log.Printf("[slack-live] Slack Socket Mode connection error: %+v", evt.Data)
			case socketmode.EventTypeDisconnect:
				if evt.Request != nil {
					log.Printf("[slack-live] Slack requested disconnect: reason=%s retry_reason=%s retry_attempt=%d", evt.Request.Reason, evt.Request.RetryReason, evt.Request.RetryAttempt)
				} else {
					log.Printf("[slack-live] Slack requested disconnect")
				}
			case socketmode.EventTypeSlashCommand:
				command, ok := evt.Data.(goslack.SlashCommand)
				if !ok || evt.Request == nil {
					continue
				}
				if err := handler(ctx, socketEnvelope{
					Type:         socketEnvelopeSlashCommand,
					Ack:          r.ackFn(*evt.Request),
					SlashCommand: &command,
				}); err != nil {
					log.Printf("[slack-live] slash command handling failed: %v", err)
				}
			case socketmode.EventTypeEventsAPI:
				eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok || evt.Request == nil {
					continue
				}
				if err := handler(ctx, socketEnvelope{
					Type:      socketEnvelopeEventsAPI,
					Ack:       r.ackFn(*evt.Request),
					EventsAPI: &eventsAPI,
				}); err != nil {
					log.Printf("[slack-live] events api handling failed: %v", err)
				}
			case socketmode.EventTypeInteractive:
				interaction, ok := evt.Data.(goslack.InteractionCallback)
				if !ok || evt.Request == nil {
					continue
				}
				if err := handler(ctx, socketEnvelope{
					Type:        socketEnvelopeInteractive,
					Ack:         r.ackFn(*evt.Request),
					Interaction: &interaction,
				}); err != nil {
					log.Printf("[slack-live] interactive handling failed: %v", err)
				}
			}
		}
	}
}

func (r *managedSocketRunner) ackFn(req socketmode.Request) func(payload any) error {
	return func(payload any) error {
		if payload == nil {
			r.client.Ack(req)
			return nil
		}
		r.client.Ack(req, payload)
		return nil
	}
}

type slackAPIMessageClient struct {
	client *goslack.Client
}

func (c *slackAPIMessageClient) PostMessage(ctx context.Context, message slackOutgoingMessage) error {
	if strings.TrimSpace(message.ChannelID) == "" {
		return errors.New("channel id is required")
	}

	options := []goslack.MsgOption{
		goslack.MsgOptionText(message.Text, false),
	}
	if len(message.Blocks) > 0 {
		options = append(options, goslack.MsgOptionBlocks(message.Blocks...))
	}
	if message.ThreadTS != "" {
		options = append(options, goslack.MsgOptionTS(message.ThreadTS))
	}

	_, _, err := c.client.PostMessageContext(ctx, message.ChannelID, options...)
	return err
}

func normalizeEnvelope(envelope socketEnvelope) (*core.Message, error) {
	switch envelope.Type {
	case socketEnvelopeSlashCommand:
		if envelope.SlashCommand == nil {
			return nil, errors.New("missing slash command payload")
		}
		return normalizeSlashCommand(envelope.SlashCommand), nil
	case socketEnvelopeEventsAPI:
		if envelope.EventsAPI == nil {
			return nil, errors.New("missing events api payload")
		}
		return normalizeEventsAPI(envelope.EventsAPI)
	case socketEnvelopeInteractive:
		return nil, errIgnoreEnvelope
	default:
		return nil, errIgnoreEnvelope
	}
}

func normalizeSlashCommand(command *goslack.SlashCommand) *core.Message {
	content := strings.TrimSpace(strings.TrimSpace(command.Command) + " " + strings.TrimSpace(command.Text))
	return &core.Message{
		Platform:   liveMetadata.Source,
		SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, command.ChannelID, command.UserID),
		UserID:     command.UserID,
		UserName:   command.UserName,
		ChatID:     command.ChannelID,
		ChatName:   command.ChannelName,
		Content:    strings.TrimSpace(content),
		ReplyCtx: replyContext{
			ChannelID:   command.ChannelID,
			ResponseURL: command.ResponseURL,
		},
		Timestamp: time.Now(),
		IsGroup:   !isDirectSlackChannel(command.ChannelID),
	}
}

func normalizeEventsAPI(eventsAPI *slackevents.EventsAPIEvent) (*core.Message, error) {
	if eventsAPI.Type != slackevents.CallbackEvent {
		return nil, errIgnoreEnvelope
	}

	switch event := eventsAPI.InnerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		return &core.Message{
			Platform:   liveMetadata.Source,
			SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, event.Channel, event.User),
			UserID:     event.User,
			ChatID:     event.Channel,
			Content:    normalizeMentionText(event.Text),
			ReplyCtx: replyContext{
				ChannelID: event.Channel,
				ThreadTS:  event.ThreadTimeStamp,
			},
			Timestamp: parseSlackTimestamp(event.TimeStamp),
			IsGroup:   true,
		}, nil
	case *slackevents.MessageEvent:
		if event.BotID != "" || event.SubType != "" {
			return nil, errIgnoreEnvelope
		}
		return &core.Message{
			Platform:   liveMetadata.Source,
			SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, event.Channel, event.User),
			UserID:     event.User,
			ChatID:     event.Channel,
			Content:    strings.TrimSpace(event.Text),
			ReplyCtx: replyContext{
				ChannelID: event.Channel,
				ThreadTS:  event.ThreadTimeStamp,
			},
			Timestamp: parseSlackTimestamp(event.TimeStamp),
			IsGroup:   event.ChannelType != "im",
		}, nil
	default:
		return nil, errIgnoreEnvelope
	}
}

func renderCardMessage(card *core.Card) (string, []goslack.Block, error) {
	if card == nil {
		return "", nil, errors.New("card is required")
	}

	blocks := make([]goslack.Block, 0, len(card.Fields)+2)
	if strings.TrimSpace(card.Title) != "" {
		blocks = append(blocks, goslack.NewSectionBlock(
			goslack.NewTextBlockObject(goslack.MarkdownType, "*"+card.Title+"*", false, false),
			nil,
			nil,
		))
	}
	if len(card.Fields) > 0 {
		fields := make([]*goslack.TextBlockObject, 0, len(card.Fields))
		for _, field := range card.Fields {
			fields = append(fields, goslack.NewTextBlockObject(
				goslack.MarkdownType,
				fmt.Sprintf("*%s*\n%s", field.Label, field.Value),
				false,
				false,
			))
		}
		blocks = append(blocks, goslack.NewSectionBlock(nil, fields, nil))
	}
	if len(card.Buttons) > 0 {
		elements := make([]goslack.BlockElement, 0, len(card.Buttons))
		for index, button := range card.Buttons {
			element := goslack.NewButtonBlockElement(
				fmt.Sprintf("card-action-%d", index),
				button.Action,
				goslack.NewTextBlockObject(goslack.PlainTextType, button.Text, false, false),
			)
			if strings.HasPrefix(button.Action, "link:") {
				element.URL = strings.TrimPrefix(button.Action, "link:")
			}
			element.Style = normalizeButtonStyle(button.Style)
			elements = append(elements, element)
		}
		blocks = append(blocks, goslack.NewActionBlock("card-actions", elements...))
	}

	fallback := strings.TrimSpace(card.Title)
	if fallback == "" && len(card.Fields) > 0 {
		fallback = fmt.Sprintf("%s: %s", card.Fields[0].Label, card.Fields[0].Value)
	}
	if fallback == "" {
		fallback = "AgentForge notification"
	}

	return fallback, blocks, nil
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
		return replyContext{ChannelID: value.ChatID}
	default:
		return replyContext{}
	}
}

func normalizeMentionText(text string) string {
	text = strings.TrimSpace(text)
	for {
		start := strings.Index(text, "<@")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], ">")
		if end == -1 {
			break
		}
		end += start
		text = text[:start] + "@AgentForge" + text[end+1:]
	}
	return strings.TrimSpace(text)
}

func parseSlackTimestamp(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return time.Now()
	}
	whole := int64(seconds)
	nanos := int64((seconds - float64(whole)) * float64(time.Second))
	return time.Unix(whole, nanos)
}

func isDirectSlackChannel(channelID string) bool {
	return strings.HasPrefix(channelID, "D")
}

func normalizeButtonStyle(style string) goslack.Style {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "primary":
		return goslack.Style("primary")
	case "danger":
		return goslack.Style("danger")
	default:
		return goslack.Style("")
	}
}
