package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	goslack "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

var liveMetadata = core.PlatformMetadata{
	Source: "slack",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:        core.CommandSurfaceMixed,
		StructuredSurface:     core.StructuredSurfaceBlocks,
		AsyncUpdateModes:      []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateThreadReply, core.AsyncUpdateFollowUp, core.AsyncUpdateEdit},
		ActionCallbackMode:    core.ActionCallbackSocketPayload,
		MessageScopes:         []core.MessageScope{core.MessageScopeChat, core.MessageScopeThread},
		NativeSurfaces:        []string{core.NativeSurfaceSlackBlockKit},
		SupportsRichMessages:  true,
		Mutability: core.MutabilitySemantics{
			CanEdit:        true,
			PrefersInPlace: true,
		},
		SupportsDeferredReply: true,
		SupportsSlashCommands: true,
		SupportsMentions:      true,
	},
	Rendering: core.RenderingProfile{
		DefaultTextFormat: core.TextFormatPlainText,
		SupportedFormats:  []core.TextFormatMode{core.TextFormatPlainText, core.TextFormatSlackMrkdwn},
		NativeSurfaces:    []string{core.NativeSurfaceSlackBlockKit},
		MaxTextLength:     4000,
		SupportsSegments:  true,
		StructuredSurface: core.StructuredSurfaceBlocks,
	},
}

var _ core.MessageUpdater = (*Live)(nil)

var errIgnoreEnvelope = errors.New("ignore slack envelope")

type replyContext struct {
	ChannelID   string
	ThreadTS    string
	ResponseURL string
	MessageTS   string
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
	Markdown  *bool
}

type messageClient interface {
	PostMessage(ctx context.Context, message slackOutgoingMessage) error
	UpdateMessage(ctx context.Context, channelID, messageTS, text string) error
}

type responseClient interface {
	PostResponse(ctx context.Context, responseURL string, message slackOutgoingMessage) error
}

type LiveOption func(*Live) error

// Live is the Slack production adapter backed by Socket Mode and chat.postMessage.
type Live struct {
	botToken string
	appToken string

	runner    socketRunner
	messages  messageClient
	responses responseClient

	actionHandler notify.ActionHandler

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
		responses: &httpResponseClient{
			client: &http.Client{Timeout: 15 * time.Second},
		},
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
	if live.responses == nil {
		return nil, errors.New("slack live transport requires a response client")
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

func WithResponseClient(client responseClient) LiveOption {
	return func(live *Live) error {
		if client == nil {
			return errors.New("response client cannot be nil")
		}
		live.responses = client
		return nil
	}
}

func (l *Live) Name() string { return "slack-live" }

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
		ChannelID:   firstNonEmpty(target.ChannelID, target.ChatID),
		ThreadTS:    target.ThreadID,
		ResponseURL: target.ResponseURL,
		MessageTS:   target.MessageID,
	}
}

func (l *Live) UpdateMessage(ctx context.Context, rawReplyCtx any, content string) error {
	rc, ok := rawReplyCtx.(*replyContext)
	if !ok || rc == nil {
		return errors.New("invalid reply context for Slack UpdateMessage")
	}
	if rc.MessageTS == "" {
		return errors.New("message timestamp required for Slack UpdateMessage")
	}
	return l.messages.UpdateMessage(ctx, rc.ChannelID, rc.MessageTS, content)
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
	l.started = true

	return l.runner.Start(ctx, func(ctx context.Context, envelope socketEnvelope) error {
		if envelope.Ack != nil {
			if err := envelope.Ack(nil); err != nil {
				return err
			}
		}

		if err := l.handleActionEnvelope(ctx, envelope); err != nil {
			if errors.Is(err, errIgnoreEnvelope) {
				// fall through to normal message handling
			} else {
				log.WithField("component", "slack-live").WithError(err).Warn("Ignoring interactive action payload")
				return nil
			}
		} else if envelope.Type == socketEnvelopeInteractive {
			return nil
		}

		msg, err := normalizeEnvelope(envelope)
		if err != nil {
			if errors.Is(err, errIgnoreEnvelope) {
				return nil
			}
			log.WithField("component", "slack-live").WithError(err).Warn("Ignoring inbound envelope")
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
	message := slackOutgoingMessage{
		ChannelID: target.ChannelID,
		ThreadTS:  target.ThreadTS,
		Text:      content,
	}
	if target.ResponseURL != "" {
		return l.responses.PostResponse(ctx, target.ResponseURL, message)
	}
	return l.messages.PostMessage(ctx, message)
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

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("slack structured send requires channel id")
	}
	outgoing, err := renderSlackStructuredMessage(message)
	if err != nil {
		return err
	}
	outgoing.ChannelID = chatID
	return l.messages.PostMessage(ctx, outgoing)
}

func (l *Live) ReplyStructured(ctx context.Context, replyCtx any, message *core.StructuredMessage) error {
	target := toReplyContext(replyCtx)
	if target.ChannelID == "" {
		return errors.New("slack structured reply requires channel id")
	}
	outgoing, err := renderSlackStructuredMessage(message)
	if err != nil {
		return err
	}
	outgoing.ChannelID = target.ChannelID
	outgoing.ThreadTS = target.ThreadTS
	if target.ResponseURL != "" {
		return l.responses.PostResponse(ctx, target.ResponseURL, outgoing)
	}
	return l.messages.PostMessage(ctx, outgoing)
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
	if target.ResponseURL != "" {
		return l.responses.PostResponse(ctx, target.ResponseURL, slackOutgoingMessage{
			ChannelID: target.ChannelID,
			ThreadTS:  target.ThreadTS,
			Text:      text,
			Blocks:    blocks,
		})
	}
	return l.messages.PostMessage(ctx, slackOutgoingMessage{
		ChannelID: target.ChannelID,
		ThreadTS:  target.ThreadTS,
		Text:      text,
		Blocks:    blocks,
	})
}

func (l *Live) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("slack native send requires channel id")
	}
	outgoing, err := renderSlackNativeMessage(message)
	if err != nil {
		return err
	}
	outgoing.ChannelID = chatID
	return l.messages.PostMessage(ctx, outgoing)
}

func (l *Live) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	target := toReplyContext(replyCtx)
	if target.ChannelID == "" {
		return errors.New("slack native reply requires channel id")
	}
	outgoing, err := renderSlackNativeMessage(message)
	if err != nil {
		return err
	}
	outgoing.ChannelID = target.ChannelID
	outgoing.ThreadTS = target.ThreadTS
	if target.ResponseURL != "" {
		return l.responses.PostResponse(ctx, target.ResponseURL, outgoing)
	}
	return l.messages.PostMessage(ctx, outgoing)
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	if strings.TrimSpace(chatID) == "" {
		return errors.New("slack formatted send requires channel id")
	}
	outgoing, err := renderSlackFormattedTextMessage(message)
	if err != nil {
		return err
	}
	outgoing.ChannelID = chatID
	return l.messages.PostMessage(ctx, outgoing)
}

func (l *Live) ReplyFormattedText(ctx context.Context, replyCtx any, message *core.FormattedText) error {
	target := toReplyContext(replyCtx)
	if target.ChannelID == "" {
		return errors.New("slack formatted reply requires channel id")
	}
	outgoing, err := renderSlackFormattedTextMessage(message)
	if err != nil {
		return err
	}
	outgoing.ChannelID = target.ChannelID
	outgoing.ThreadTS = target.ThreadTS
	if target.ResponseURL != "" {
		return l.responses.PostResponse(ctx, target.ResponseURL, outgoing)
	}
	return l.messages.PostMessage(ctx, outgoing)
}

func (l *Live) UpdateFormattedText(ctx context.Context, replyCtx any, message *core.FormattedText) error {
	return l.ReplyFormattedText(ctx, replyCtx, message)
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
			log.WithField("component", "slack-live").WithError(err).Error("Socket mode stopped with error")
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
				log.WithField("component", "slack-live").Info("Connecting to Slack Socket Mode")
			case socketmode.EventTypeConnected:
				log.WithField("component", "slack-live").Info("Connected to Slack Socket Mode")
			case socketmode.EventTypeConnectionError:
				log.WithField("component", "slack-live").WithField("data", evt.Data).Error("Slack Socket Mode connection error")
			case socketmode.EventTypeDisconnect:
				if evt.Request != nil {
					log.WithFields(log.Fields{"component": "slack-live", "reason": evt.Request.Reason, "retry_reason": evt.Request.RetryReason, "retry_attempt": evt.Request.RetryAttempt}).Warn("Slack requested disconnect")
				} else {
					log.WithField("component", "slack-live").Warn("Slack requested disconnect")
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
					log.WithField("component", "slack-live").WithError(err).Error("Slash command handling failed")
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
					log.WithField("component", "slack-live").WithError(err).Error("Events API handling failed")
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
					log.WithField("component", "slack-live").WithError(err).Error("Interactive handling failed")
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

type httpResponseClient struct {
	client *http.Client
}

func (c *slackAPIMessageClient) UpdateMessage(ctx context.Context, channelID, messageTS, text string) error {
	_, _, _, err := c.client.UpdateMessageContext(ctx, channelID, messageTS, goslack.MsgOptionText(text, false))
	return err
}

func (c *slackAPIMessageClient) PostMessage(ctx context.Context, message slackOutgoingMessage) error {
	if strings.TrimSpace(message.ChannelID) == "" {
		return errors.New("channel id is required")
	}

	options := []goslack.MsgOption{
		goslack.MsgOptionText(message.Text, false),
	}
	if message.Markdown != nil {
		params := goslack.NewPostMessageParameters()
		params.Markdown = *message.Markdown
		options = append(options, goslack.MsgOptionPostMessageParameters(params))
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

func (c *httpResponseClient) PostResponse(ctx context.Context, responseURL string, message slackOutgoingMessage) error {
	if strings.TrimSpace(responseURL) == "" {
		return errors.New("response url is required")
	}

	payload := map[string]any{
		"text": message.Text,
	}
	if message.Markdown != nil {
		payload["mrkdwn"] = *message.Markdown
	}
	if message.ThreadTS != "" {
		payload["thread_ts"] = message.ThreadTS
	}
	if len(message.Blocks) > 0 {
		payload["blocks"] = message.Blocks
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack response payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create slack response request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack response url payload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack response url returned %d", resp.StatusCode)
	}
	return nil
}

func renderSlackNativeMessage(message *core.NativeMessage) (slackOutgoingMessage, error) {
	if err := message.Validate(); err != nil {
		return slackOutgoingMessage{}, err
	}
	if message.NormalizedPlatform() != "slack" || message.SlackBlockKit == nil {
		return slackOutgoingMessage{}, errors.New("native message is not a slack block kit payload")
	}

	var blocks goslack.Blocks
	if err := json.Unmarshal(message.SlackBlockKit.Blocks, &blocks); err != nil {
		return slackOutgoingMessage{}, fmt.Errorf("decode slack block kit payload: %w", err)
	}
	if len(blocks.BlockSet) == 0 {
		return slackOutgoingMessage{}, errors.New("slack block kit payload requires at least one block")
	}

	return slackOutgoingMessage{
		Text:   strings.TrimSpace(message.FallbackText()),
		Blocks: blocks.BlockSet,
	}, nil
}

func renderSlackStructuredMessage(message *core.StructuredMessage) (slackOutgoingMessage, error) {
	if message == nil {
		return slackOutgoingMessage{}, errors.New("structured message is required")
	}
	outgoing := slackOutgoingMessage{
		Text: strings.TrimSpace(message.FallbackText()),
	}
	if len(message.Sections) > 0 {
		outgoing.Blocks = renderStructuredSections(message.Sections)
		return outgoing, nil
	}

	card := message.LegacyCard()
	if card == nil {
		return outgoing, nil
	}
	_, blocks, err := renderCardMessage(card)
	if err != nil {
		return slackOutgoingMessage{}, err
	}
	outgoing.Blocks = blocks
	return outgoing, nil
}

func renderSlackFormattedTextMessage(message *core.FormattedText) (slackOutgoingMessage, error) {
	if message == nil {
		return slackOutgoingMessage{}, errors.New("formatted text is required")
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return slackOutgoingMessage{}, errors.New("formatted text content is required")
	}

	markdown := false
	switch message.Format {
	case "", core.TextFormatPlainText:
		markdown = false
	case core.TextFormatSlackMrkdwn:
		markdown = true
	default:
		markdown = false
	}

	return slackOutgoingMessage{
		Text:     content,
		Markdown: boolPtr(markdown),
	}, nil
}

func boolPtr(value bool) *bool {
	return &value
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

func (l *Live) handleActionEnvelope(ctx context.Context, envelope socketEnvelope) error {
	req, err := normalizeActionEnvelope(envelope)
	if err != nil {
		return err
	}
	if req == nil || l.actionHandler == nil {
		return nil
	}

	result, err := l.actionHandler.HandleAction(ctx, req)
	if err != nil {
		return err
	}
	if result == nil || strings.TrimSpace(result.Result) == "" || strings.TrimSpace(req.ChatID) == "" {
		return nil
	}

	target := req.ReplyTarget
	if result.ReplyTarget != nil {
		target = result.ReplyTarget
	}
	_, err = core.DeliverText(ctx, l, l.Metadata(), target, req.ChatID, result.Result)
	return err
}

func normalizeActionEnvelope(envelope socketEnvelope) (*notify.ActionRequest, error) {
	if envelope.Type != socketEnvelopeInteractive {
		return nil, errIgnoreEnvelope
	}
	if envelope.Interaction == nil {
		return nil, errors.New("missing interaction payload")
	}
	return normalizeInteractionAction(envelope.Interaction)
}

func normalizeInteractionAction(interaction *goslack.InteractionCallback) (*notify.ActionRequest, error) {
	if interaction == nil {
		return nil, errors.New("missing interaction payload")
	}

	switch interaction.Type {
	case goslack.InteractionTypeBlockActions:
		return normalizeBlockAction(interaction)
	case goslack.InteractionTypeViewSubmission:
		return normalizeViewSubmission(interaction)
	default:
		return nil, errIgnoreEnvelope
	}
}

func normalizeBlockAction(interaction *goslack.InteractionCallback) (*notify.ActionRequest, error) {
	if len(interaction.ActionCallback.BlockActions) == 0 {
		return nil, errors.New("slack block action missing actions")
	}
	actionPayload := interaction.ActionCallback.BlockActions[0]
	action, entityID, ok := core.ParseActionReference(actionPayload.Value)
	if !ok {
		return nil, errIgnoreEnvelope
	}

	channelID := firstNonEmpty(interaction.Channel.ID, interaction.Container.ChannelID)
	replyTarget := &core.ReplyTarget{
		Platform:          liveMetadata.Source,
		ChatID:            channelID,
		ChannelID:         channelID,
		ThreadID:          firstNonEmpty(interaction.Container.ThreadTs, interaction.Container.MessageTs),
		ResponseURL:       strings.TrimSpace(interaction.ResponseURL),
		UseReply:          true,
		PreferredRenderer: string(liveMetadata.Capabilities.StructuredSurface),
	}
	metadata := map[string]string{
		"source":     string(goslack.InteractionTypeBlockActions),
		"trigger_id": strings.TrimSpace(interaction.TriggerID),
	}
	if actionID := strings.TrimSpace(actionPayload.ActionID); actionID != "" {
		metadata["action_id"] = actionID
	}
	if blockID := strings.TrimSpace(actionPayload.BlockID); blockID != "" {
		metadata["block_id"] = blockID
	}
	if callbackID := strings.TrimSpace(interaction.CallbackID); callbackID != "" {
		metadata["callback_id"] = callbackID
	}

	return &notify.ActionRequest{
		Platform:    liveMetadata.Source,
		Action:      action,
		EntityID:    entityID,
		ChatID:      channelID,
		UserID:      strings.TrimSpace(interaction.User.ID),
		ReplyTarget: replyTarget,
		Metadata:    compactMetadata(metadata),
	}, nil
}

func normalizeViewSubmission(interaction *goslack.InteractionCallback) (*notify.ActionRequest, error) {
	action, entityID, ok := core.ParseActionReference(firstNonEmpty(interaction.View.PrivateMetadata, interaction.CallbackID))
	if !ok {
		return nil, errIgnoreEnvelope
	}

	var responseInfo *goslack.ViewSubmissionCallbackResponseURL
	if len(interaction.ViewSubmissionCallback.ResponseURLs) > 0 {
		responseInfo = &interaction.ViewSubmissionCallback.ResponseURLs[0]
	}
	channelID := firstNonEmpty(
		valueOrEmpty(responseInfo, func(v *goslack.ViewSubmissionCallbackResponseURL) string { return v.ChannelID }),
		interaction.Channel.ID,
		interaction.Container.ChannelID,
	)
	replyTarget := &core.ReplyTarget{
		Platform:          liveMetadata.Source,
		ChatID:            channelID,
		ChannelID:         channelID,
		ResponseURL:       firstNonEmpty(valueOrEmpty(responseInfo, func(v *goslack.ViewSubmissionCallbackResponseURL) string { return v.ResponseURL }), interaction.ResponseURL),
		UseReply:          true,
		PreferredRenderer: string(liveMetadata.Capabilities.StructuredSurface),
	}
	metadata := map[string]string{
		"source":     string(goslack.InteractionTypeViewSubmission),
		"trigger_id": strings.TrimSpace(interaction.TriggerID),
		"view_id":    strings.TrimSpace(interaction.View.ID),
		"view_hash":  strings.TrimSpace(interaction.View.Hash),
	}
	if callbackID := strings.TrimSpace(interaction.CallbackID); callbackID != "" {
		metadata["callback_id"] = callbackID
	}
	if responseInfo != nil {
		if blockID := strings.TrimSpace(responseInfo.BlockID); blockID != "" {
			metadata["response_block_id"] = blockID
		}
		if actionID := strings.TrimSpace(responseInfo.ActionID); actionID != "" {
			metadata["response_action_id"] = actionID
		}
	}

	return &notify.ActionRequest{
		Platform:    liveMetadata.Source,
		Action:      action,
		EntityID:    entityID,
		ChatID:      channelID,
		UserID:      strings.TrimSpace(interaction.User.ID),
		ReplyTarget: replyTarget,
		Metadata:    compactMetadata(metadata),
	}, nil
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
		ReplyTarget: &core.ReplyTarget{
			Platform:    liveMetadata.Source,
			ChatID:      command.ChannelID,
			ChannelID:   command.ChannelID,
			ResponseURL: command.ResponseURL,
			UseReply:    true,
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
			ReplyTarget: &core.ReplyTarget{
				Platform:  liveMetadata.Source,
				ChatID:    event.Channel,
				ChannelID: event.Channel,
				ThreadID:  event.ThreadTimeStamp,
				UseReply:  true,
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
			ReplyTarget: &core.ReplyTarget{
				Platform:  liveMetadata.Source,
				ChatID:    event.Channel,
				ChannelID: event.Channel,
				ThreadID:  event.ThreadTimeStamp,
				UseReply:  true,
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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

func valueOrEmpty[T any](value *T, getter func(*T) string) string {
	if value == nil || getter == nil {
		return ""
	}
	return strings.TrimSpace(getter(value))
}
