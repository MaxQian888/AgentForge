package discord

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

const (
	interactionTypePing               = 1
	interactionTypeApplicationCommand = 2
	interactionTypeMessageComponent   = 3

	interactionCallbackTypePong                             = 1
	interactionCallbackTypeChannelMessageWithSource         = 4
	interactionCallbackTypeDeferredChannelMessageWithSource = 5
	interactionCallbackTypeDeferredUpdateMessage            = 6

	commandOptionTypeSubCommand      = 1
	commandOptionTypeSubCommandGroup = 2
	commandOptionTypeString          = 3

	componentTypeActionRow = 1
	componentTypeButton    = 2

	componentStylePrimary   = 1
	componentStyleSecondary = 2
	componentStyleDanger    = 4
	componentStyleLink      = 5

	ephemeralMessageFlag = 1 << 6
)

var liveMetadata = core.PlatformMetadata{
	Source: "discord",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:     core.CommandSurfaceInteraction,
		StructuredSurface:  core.StructuredSurfaceComponents,
		AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateFollowUp, core.AsyncUpdateEdit},
		ActionCallbackMode: core.ActionCallbackInteractionToken,
		MessageScopes:      []core.MessageScope{core.MessageScopeInteractionScoped, core.MessageScopeChat},
		NativeSurfaces:     []string{core.NativeSurfaceDiscordEmbed},
		Mutability: core.MutabilitySemantics{
			CanEdit:        true,
			PrefersInPlace: true,
		},
		SupportsDeferredReply: true,
		SupportsSlashCommands: true,
		SupportsRichMessages:  true,
	},
	Rendering: core.RenderingProfile{
		DefaultTextFormat: core.TextFormatPlainText,
		SupportedFormats:  []core.TextFormatMode{core.TextFormatPlainText, core.TextFormatDiscordMD},
		NativeSurfaces:    []string{core.NativeSurfaceDiscordEmbed},
		MaxTextLength:     2000,
		SupportsSegments:  true,
		StructuredSurface: core.StructuredSurfaceComponents,
	},
}

type interaction struct {
	Type          int                 `json:"type"`
	Token         string              `json:"token,omitempty"`
	ApplicationID string              `json:"application_id,omitempty"`
	GuildID       string              `json:"guild_id,omitempty"`
	ChannelID     string              `json:"channel_id,omitempty"`
	Member        *interactionMember  `json:"member,omitempty"`
	User          *interactionUser    `json:"user,omitempty"`
	Data          *interactionData    `json:"data,omitempty"`
	Message       *interactionMessage `json:"message,omitempty"`
}

type interactionMember struct {
	User *interactionUser `json:"user,omitempty"`
}

type interactionUser struct {
	ID         string `json:"id,omitempty"`
	Username   string `json:"username,omitempty"`
	GlobalName string `json:"global_name,omitempty"`
}

type interactionMessage struct {
	ID string `json:"id,omitempty"`
}

type interactionData struct {
	Name          string                  `json:"name,omitempty"`
	Options       []interactionDataOption `json:"options,omitempty"`
	ComponentType int                     `json:"component_type,omitempty"`
	CustomID      string                  `json:"custom_id,omitempty"`
}

type interactionDataOption struct {
	Type        int                     `json:"type"`
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Required    bool                    `json:"required,omitempty"`
	Value       any                     `json:"value,omitempty"`
	Options     []interactionDataOption `json:"options,omitempty"`
}

type applicationCommand struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Options     []interactionDataOption `json:"options,omitempty"`
}

type interactionResponse struct {
	Type int                      `json:"type"`
	Data *interactionResponseData `json:"data,omitempty"`
}

type interactionResponseData struct {
	Content string `json:"content,omitempty"`
	Flags   int    `json:"flags,omitempty"`
}

type replyContext struct {
	InteractionToken   string
	ChannelID          string
	OriginalResponseID string
}

type interactionEnvelope struct {
	Interaction *interaction
	Ack         func(interactionResponse) error
}

type discordComponent struct {
	Type       int                `json:"type"`
	Style      int                `json:"style,omitempty"`
	Label      string             `json:"label,omitempty"`
	CustomID   string             `json:"custom_id,omitempty"`
	URL        string             `json:"url,omitempty"`
	Components []discordComponent `json:"components,omitempty"`
}

type discordOutgoingMessage struct {
	Content    string             `json:"content,omitempty"`
	Flags      int                `json:"flags,omitempty"`
	Embeds     []discordEmbed     `json:"embeds,omitempty"`
	Components []discordComponent `json:"components,omitempty"`
}

type discordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Image       discordEmbedImage   `json:"image,omitempty"`
}

type discordEmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type discordEmbedImage struct {
	URL string `json:"url,omitempty"`
}

type interactionRunner interface {
	Start(ctx context.Context, handler func(context.Context, interactionEnvelope) error) error
	Stop(ctx context.Context) error
}

type followupClient interface {
	SendFollowup(ctx context.Context, appID, token string, message discordOutgoingMessage) error
}

type channelClient interface {
	SendChannelMessage(ctx context.Context, channelID string, message discordOutgoingMessage) error
}

type originalResponseClient interface {
	EditOriginalResponse(ctx context.Context, appID, token string, message discordOutgoingMessage) error
}

type commandRegistrar interface {
	SyncCommands(ctx context.Context, appID, guildID string, commands []applicationCommand) error
}

var _ core.CardSender = (*Live)(nil)
var _ core.TypingIndicator = (*Live)(nil)

type typingClient interface {
	TriggerTyping(ctx context.Context, channelID string) error
}

type LiveOption func(*Live) error

type Live struct {
	appID          string
	botToken       string
	publicKey      string
	port           string
	commandGuildID string

	runner    interactionRunner
	followups followupClient
	channels  channelClient
	originals originalResponseClient
	registrar commandRegistrar
	typing    typingClient

	actionHandler notify.ActionHandler

	startCtx    context.Context
	startCancel context.CancelFunc
	started     bool
	mu          sync.Mutex
}

func NewLive(appID, botToken, publicKey, port string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(botToken) == "" || strings.TrimSpace(publicKey) == "" {
		return nil, errors.New("discord live transport requires app id, bot token, and public key")
	}
	if strings.TrimSpace(port) == "" {
		return nil, errors.New("discord live transport requires interactions port")
	}
	if _, err := decodePublicKey(publicKey); err != nil {
		return nil, err
	}

	apiClient := newDiscordAPIClient(botToken)
	live := &Live{
		appID:     appID,
		botToken:  botToken,
		publicKey: publicKey,
		port:      port,
		runner:    newHTTPInteractionRunner(publicKey, port),
		followups: &discordFollowupClient{client: apiClient},
		channels:  &discordChannelClient{client: apiClient},
		originals: &discordOriginalResponseClient{client: apiClient},
		registrar: &discordCommandRegistrar{client: apiClient},
		typing:    &discordTypingClient{client: apiClient},
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}
	if live.runner == nil {
		return nil, errors.New("discord live transport requires an interaction runner")
	}
	if live.followups == nil {
		return nil, errors.New("discord live transport requires a followup client")
	}
	if live.channels == nil {
		return nil, errors.New("discord live transport requires a channel client")
	}
	if live.originals == nil {
		return nil, errors.New("discord live transport requires an original response client")
	}
	if live.registrar == nil {
		return nil, errors.New("discord live transport requires a command registrar")
	}

	return live, nil
}

func WithInteractionRunner(runner interactionRunner) LiveOption {
	return func(live *Live) error {
		if runner == nil {
			return errors.New("interaction runner cannot be nil")
		}
		live.runner = runner
		return nil
	}
}

func WithFollowupClient(client followupClient) LiveOption {
	return func(live *Live) error {
		if client == nil {
			return errors.New("followup client cannot be nil")
		}
		live.followups = client
		return nil
	}
}

func WithChannelClient(client channelClient) LiveOption {
	return func(live *Live) error {
		if client == nil {
			return errors.New("channel client cannot be nil")
		}
		live.channels = client
		return nil
	}
}

func WithOriginalResponseClient(client originalResponseClient) LiveOption {
	return func(live *Live) error {
		if client == nil {
			return errors.New("original response client cannot be nil")
		}
		live.originals = client
		return nil
	}
}

func WithCommandRegistrar(registrar commandRegistrar) LiveOption {
	return func(live *Live) error {
		if registrar == nil {
			return errors.New("command registrar cannot be nil")
		}
		live.registrar = registrar
		return nil
	}
}

func WithCommandGuildID(guildID string) LiveOption {
	return func(live *Live) error {
		live.commandGuildID = strings.TrimSpace(guildID)
		return nil
	}
}

func (l *Live) Name() string { return "discord-live" }

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
		InteractionToken:   strings.TrimSpace(target.InteractionToken),
		ChannelID:          firstNonEmpty(target.ChannelID, target.ChatID),
		OriginalResponseID: firstNonEmpty(target.OriginalResponseID, "@original"),
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

	if err := l.registrar.SyncCommands(ctx, l.appID, l.commandGuildID, defaultApplicationCommands()); err != nil {
		cancel()
		return err
	}

	if err := l.runner.Start(ctx, func(ctx context.Context, envelope interactionEnvelope) error {
		if envelope.Interaction == nil {
			return errors.New("missing discord interaction payload")
		}

		switch envelope.Interaction.Type {
		case interactionTypePing:
			if envelope.Ack != nil {
				return envelope.Ack(interactionResponse{Type: interactionCallbackTypePong})
			}
			return nil
		case interactionTypeApplicationCommand:
			if envelope.Ack != nil {
				if err := envelope.Ack(interactionResponse{Type: interactionCallbackTypeDeferredChannelMessageWithSource}); err != nil {
					return err
				}
			}
			msg, err := normalizeInteraction(envelope.Interaction)
			if err != nil {
				log.WithField("component", "discord-live").WithError(err).Warn("Ignoring inbound interaction")
				return nil
			}
			handler(l, msg)
			return nil
		case interactionTypeMessageComponent:
			if envelope.Ack != nil {
				if err := envelope.Ack(interactionResponse{Type: interactionCallbackTypeDeferredUpdateMessage}); err != nil {
					return err
				}
			}
			if err := l.handleComponentInteraction(ctx, envelope.Interaction); err != nil {
				log.WithField("component", "discord-live").WithError(err).Warn("Ignoring component interaction")
			}
			return nil
		default:
			if envelope.Ack != nil {
				return envelope.Ack(interactionResponse{
					Type: interactionCallbackTypeChannelMessageWithSource,
					Data: &interactionResponseData{
						Content: "Unsupported interaction type.",
						Flags:   ephemeralMessageFlag,
					},
				})
			}
			return nil
		}
	}); err != nil {
		cancel()
		return err
	}

	l.started = true
	return nil
}

func (l *Live) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	message := discordOutgoingMessage{Content: content}
	if strings.TrimSpace(reply.InteractionToken) != "" {
		return l.followups.SendFollowup(ctx, l.appID, reply.InteractionToken, message)
	}
	if strings.TrimSpace(reply.ChannelID) != "" {
		return l.channels.SendChannelMessage(ctx, reply.ChannelID, message)
	}
	return errors.New("discord reply requires interaction token or channel id")
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	channelID := strings.TrimSpace(chatID)
	if channelID == "" {
		return errors.New("discord send requires channel id")
	}
	return l.channels.SendChannelMessage(ctx, channelID, discordOutgoingMessage{Content: content})
}

func (l *Live) UpdateMessage(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	if strings.TrimSpace(reply.InteractionToken) == "" {
		return errors.New("discord update requires interaction token")
	}
	return l.originals.EditOriginalResponse(ctx, l.appID, reply.InteractionToken, discordOutgoingMessage{Content: content})
}

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	channelID := strings.TrimSpace(chatID)
	if channelID == "" {
		return errors.New("discord send requires channel id")
	}
	outgoing := renderDiscordStructuredMessage(message)
	return l.channels.SendChannelMessage(ctx, channelID, discordOutgoingMessage{
		Content:    outgoing.Content,
		Embeds:     outgoing.Embeds,
		Components: outgoing.Components,
	})
}

func (l *Live) ReplyStructured(ctx context.Context, rawReplyCtx any, message *core.StructuredMessage) error {
	reply := toReplyContext(rawReplyCtx)
	outgoing := renderDiscordStructuredMessage(message)
	if strings.TrimSpace(reply.InteractionToken) != "" {
		return l.followups.SendFollowup(ctx, l.appID, reply.InteractionToken, outgoing)
	}
	if strings.TrimSpace(reply.ChannelID) != "" {
		return l.channels.SendChannelMessage(ctx, reply.ChannelID, outgoing)
	}
	return errors.New("discord reply requires interaction token or channel id")
}

func (l *Live) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	channelID := strings.TrimSpace(chatID)
	if channelID == "" {
		return errors.New("discord send requires channel id")
	}
	outgoing, err := renderDiscordNativeMessage(message)
	if err != nil {
		return err
	}
	return l.channels.SendChannelMessage(ctx, channelID, outgoing)
}

func (l *Live) ReplyNative(ctx context.Context, rawReplyCtx any, message *core.NativeMessage) error {
	reply := toReplyContext(rawReplyCtx)
	outgoing, err := renderDiscordNativeMessage(message)
	if err != nil {
		return err
	}
	if strings.TrimSpace(reply.InteractionToken) != "" {
		return l.followups.SendFollowup(ctx, l.appID, reply.InteractionToken, outgoing)
	}
	if strings.TrimSpace(reply.ChannelID) != "" {
		return l.channels.SendChannelMessage(ctx, reply.ChannelID, outgoing)
	}
	return errors.New("discord reply requires interaction token or channel id")
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	channelID := strings.TrimSpace(chatID)
	if channelID == "" {
		return errors.New("discord send requires channel id")
	}
	outgoing, err := renderDiscordFormattedTextMessage(message)
	if err != nil {
		return err
	}
	return l.channels.SendChannelMessage(ctx, channelID, outgoing)
}

func (l *Live) ReplyFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	reply := toReplyContext(rawReplyCtx)
	outgoing, err := renderDiscordFormattedTextMessage(message)
	if err != nil {
		return err
	}
	if strings.TrimSpace(reply.InteractionToken) != "" {
		return l.followups.SendFollowup(ctx, l.appID, reply.InteractionToken, outgoing)
	}
	if strings.TrimSpace(reply.ChannelID) != "" {
		return l.channels.SendChannelMessage(ctx, reply.ChannelID, outgoing)
	}
	return errors.New("discord reply requires interaction token or channel id")
}

func (l *Live) UpdateFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	reply := toReplyContext(rawReplyCtx)
	if strings.TrimSpace(reply.InteractionToken) == "" {
		return errors.New("discord update requires interaction token")
	}
	outgoing, err := renderDiscordFormattedTextMessage(message)
	if err != nil {
		return err
	}
	return l.originals.EditOriginalResponse(ctx, l.appID, reply.InteractionToken, outgoing)
}

func (l *Live) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	channelID := strings.TrimSpace(chatID)
	if channelID == "" {
		return errors.New("discord send requires channel id")
	}
	outgoing := renderCardToDiscordMessage(card)
	return l.channels.SendChannelMessage(ctx, channelID, outgoing)
}

func (l *Live) ReplyCard(ctx context.Context, rawReplyCtx any, card *core.Card) error {
	reply := toReplyContext(rawReplyCtx)
	outgoing := renderCardToDiscordMessage(card)
	if strings.TrimSpace(reply.InteractionToken) != "" {
		return l.followups.SendFollowup(ctx, l.appID, reply.InteractionToken, outgoing)
	}
	if strings.TrimSpace(reply.ChannelID) != "" {
		return l.channels.SendChannelMessage(ctx, reply.ChannelID, outgoing)
	}
	return errors.New("discord reply requires interaction token or channel id")
}

func (l *Live) StartTyping(ctx context.Context, chatID string) error {
	channelID := strings.TrimSpace(chatID)
	if channelID == "" {
		return nil
	}
	_ = l.typing.TriggerTyping(ctx, channelID)
	return nil
}

func (l *Live) StopTyping(ctx context.Context, chatID string) error {
	return nil // Discord typing auto-expires after ~10 seconds
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

func (l *Live) handleComponentInteraction(ctx context.Context, raw *interaction) error {
	if l.actionHandler == nil {
		return nil
	}
	req, err := normalizeComponentAction(raw)
	if err != nil {
		return err
	}
	result, err := l.actionHandler.HandleAction(ctx, req)
	if err != nil {
		return err
	}
	if result == nil || strings.TrimSpace(result.Result) == "" {
		return nil
	}
	target := req.ReplyTarget
	if result.ReplyTarget != nil {
		target = result.ReplyTarget
	}
	_, err = core.DeliverText(ctx, l, l.Metadata(), target, req.ChatID, result.Result)
	return err
}

type httpInteractionRunner struct {
	publicKey string
	port      string

	server *http.Server
}

func newHTTPInteractionRunner(publicKey, port string) *httpInteractionRunner {
	return &httpInteractionRunner{
		publicKey: publicKey,
		port:      port,
	}
}

func (r *httpInteractionRunner) Start(ctx context.Context, handler func(context.Context, interactionEnvelope) error) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /interactions", func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := validateRequestSignature(
			r.publicKey,
			req.Header.Get("X-Signature-Timestamp"),
			req.Header.Get("X-Signature-Ed25519"),
			body,
		); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		var payload interaction
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		acknowledged := false
		envelope := interactionEnvelope{
			Interaction: &payload,
			Ack: func(response interactionResponse) error {
				if acknowledged {
					return errors.New("discord interaction already acknowledged")
				}
				acknowledged = true
				w.Header().Set("Content-Type", "application/json")
				return json.NewEncoder(w).Encode(response)
			},
		}

		if err := handler(req.Context(), envelope); err != nil {
			if !acknowledged {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			log.WithField("component", "discord-live").WithError(err).Error("Interaction handling failed after acknowledgement")
			return
		}
		if !acknowledged {
			http.Error(w, "discord interaction handler did not acknowledge request", http.StatusInternalServerError)
		}
	})

	r.server = &http.Server{
		Addr:    ":" + r.port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		_ = r.server.Shutdown(context.Background())
	}()
	go func() {
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "discord-live").WithError(err).Error("Interaction server stopped with error")
		}
	}()
	return nil
}

func (r *httpInteractionRunner) Stop(context.Context) error {
	if r.server != nil {
		return r.server.Shutdown(context.Background())
	}
	return nil
}

type discordAPIClient struct {
	baseURL  string
	botToken string
	client   *http.Client
}

func newDiscordAPIClient(botToken string) *discordAPIClient {
	return &discordAPIClient{
		baseURL:  "https://discord.com/api/v10",
		botToken: strings.TrimSpace(botToken),
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *discordAPIClient) doJSON(ctx context.Context, method, path string, body any, withBotAuth bool) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if withBotAuth {
		req.Header.Set("Authorization", "Bot "+c.botToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("discord api error %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
}

type discordFollowupClient struct {
	client *discordAPIClient
}

func (c *discordFollowupClient) SendFollowup(ctx context.Context, appID, token string, message discordOutgoingMessage) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("discord followup requires interaction token")
	}
	return c.client.doJSON(ctx, http.MethodPost, "/webhooks/"+strings.TrimSpace(appID)+"/"+strings.TrimSpace(token), message, false)
}

type discordChannelClient struct {
	client *discordAPIClient
}

func (c *discordChannelClient) SendChannelMessage(ctx context.Context, channelID string, message discordOutgoingMessage) error {
	if strings.TrimSpace(channelID) == "" {
		return errors.New("discord channel send requires channel id")
	}
	return c.client.doJSON(ctx, http.MethodPost, "/channels/"+strings.TrimSpace(channelID)+"/messages", message, true)
}

type discordOriginalResponseClient struct {
	client *discordAPIClient
}

func (c *discordOriginalResponseClient) EditOriginalResponse(ctx context.Context, appID, token string, message discordOutgoingMessage) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("discord original response update requires interaction token")
	}
	return c.client.doJSON(ctx, http.MethodPatch, "/webhooks/"+strings.TrimSpace(appID)+"/"+strings.TrimSpace(token)+"/messages/@original", message, false)
}

type discordCommandRegistrar struct {
	client *discordAPIClient
}

func (c *discordCommandRegistrar) SyncCommands(ctx context.Context, appID, guildID string, commands []applicationCommand) error {
	path := "/applications/" + strings.TrimSpace(appID) + "/commands"
	if strings.TrimSpace(guildID) != "" {
		path = "/applications/" + strings.TrimSpace(appID) + "/guilds/" + strings.TrimSpace(guildID) + "/commands"
	}
	return c.client.doJSON(ctx, http.MethodPut, path, commands, true)
}

type discordTypingClient struct {
	client *discordAPIClient
}

func (c *discordTypingClient) TriggerTyping(ctx context.Context, channelID string) error {
	if strings.TrimSpace(channelID) == "" {
		return errors.New("discord typing requires channel id")
	}
	return c.client.doJSON(ctx, http.MethodPost, "/channels/"+strings.TrimSpace(channelID)+"/typing", nil, true)
}

func WithTypingClient(client typingClient) LiveOption {
	return func(live *Live) error {
		if client == nil {
			return errors.New("typing client cannot be nil")
		}
		live.typing = client
		return nil
	}
}

func defaultApplicationCommands() []applicationCommand {
	return []applicationCommand{
		{
			Name:        "task",
			Description: "Run AgentForge task commands.",
			Options: []interactionDataOption{
				{
					Type:        commandOptionTypeString,
					Name:        "args",
					Description: "Task command arguments, for example: list or status task-123.",
				},
			},
		},
		{
			Name:        "agent",
			Description: "Run AgentForge agent commands.",
			Options: []interactionDataOption{
				{
					Type:        commandOptionTypeString,
					Name:        "args",
					Description: "Agent command arguments, for example: list or spawn task-123.",
				},
			},
		},
		{
			Name:        "cost",
			Description: "Show AgentForge cost statistics.",
		},
		{
			Name:        "help",
			Description: "Show AgentForge IM bridge help.",
		},
	}
}

func normalizeInteraction(raw *interaction) (*core.Message, error) {
	if raw == nil || raw.Data == nil {
		return nil, errors.New("discord interaction missing command data")
	}
	user := interactionUserFromPayload(raw)
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return nil, errors.New("discord interaction missing user id")
	}
	if strings.TrimSpace(raw.ChannelID) == "" {
		return nil, errors.New("discord interaction missing channel id")
	}

	content, err := commandContent(raw.Data)
	if err != nil {
		return nil, err
	}

	return &core.Message{
		Platform:   liveMetadata.Source,
		SessionKey: fmt.Sprintf("%s:%s:%s", liveMetadata.Source, raw.ChannelID, user.ID),
		UserID:     strings.TrimSpace(user.ID),
		UserName:   firstNonEmpty(user.GlobalName, user.Username),
		ChatID:     strings.TrimSpace(raw.ChannelID),
		Content:    content,
		ReplyCtx: replyContext{
			InteractionToken:   strings.TrimSpace(raw.Token),
			ChannelID:          strings.TrimSpace(raw.ChannelID),
			OriginalResponseID: "@original",
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:           liveMetadata.Source,
			ChatID:             strings.TrimSpace(raw.ChannelID),
			ChannelID:          strings.TrimSpace(raw.ChannelID),
			InteractionToken:   strings.TrimSpace(raw.Token),
			OriginalResponseID: "@original",
			UseReply:           true,
			PreferEdit:         true,
		},
		Timestamp: time.Now(),
		IsGroup:   strings.TrimSpace(raw.GuildID) != "",
	}, nil
}

func normalizeComponentAction(raw *interaction) (*notify.ActionRequest, error) {
	if raw == nil || raw.Data == nil {
		return nil, errors.New("discord component interaction missing data")
	}
	user := interactionUserFromPayload(raw)
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return nil, errors.New("discord interaction missing user id")
	}

	customID := strings.TrimSpace(raw.Data.CustomID)
	action, entityID, actionMetadata, ok := core.ParseActionReferenceWithMetadata(customID)
	if !ok {
		return nil, errors.New("discord component interaction missing action reference")
	}

	replyTarget := &core.ReplyTarget{
		Platform:           liveMetadata.Source,
		ChatID:             strings.TrimSpace(raw.ChannelID),
		ChannelID:          strings.TrimSpace(raw.ChannelID),
		MessageID:          valueOrEmpty(raw.Message, func(m *interactionMessage) string { return m.ID }),
		InteractionToken:   strings.TrimSpace(raw.Token),
		OriginalResponseID: "@original",
		UserID:             strings.TrimSpace(user.ID),
		UseReply:           true,
		PreferEdit:         true,
		PreferredRenderer:  string(liveMetadata.Capabilities.StructuredSurface),
	}

	metadata := compactMetadata(map[string]string{
		"source":         "message_component",
		"custom_id":      customID,
		"component_type": fmt.Sprintf("%d", raw.Data.ComponentType),
		"message_id":     valueOrEmpty(raw.Message, func(m *interactionMessage) string { return m.ID }),
	})
	for key, value := range actionMetadata {
		metadata[key] = value
	}

	return &notify.ActionRequest{
		Platform:    liveMetadata.Source,
		Action:      action,
		EntityID:    entityID,
		ChatID:      strings.TrimSpace(raw.ChannelID),
		UserID:      strings.TrimSpace(user.ID),
		ReplyTarget: replyTarget,
		Metadata:    metadata,
	}, nil
}

func interactionUserFromPayload(raw *interaction) *interactionUser {
	if raw == nil {
		return nil
	}
	if raw.Member != nil && raw.Member.User != nil {
		return raw.Member.User
	}
	return raw.User
}

func commandContent(data *interactionData) (string, error) {
	command := strings.ToLower(strings.TrimSpace(data.Name))
	if command == "" {
		return "", errors.New("discord interaction missing command name")
	}

	parts := []string{"/" + command}
	parts = append(parts, flattenCommandOptions(data.Options)...)
	return strings.TrimSpace(strings.Join(parts, " ")), nil
}

func flattenCommandOptions(options []interactionDataOption) []string {
	parts := make([]string, 0)
	for _, option := range options {
		switch option.Type {
		case commandOptionTypeSubCommand, commandOptionTypeSubCommandGroup:
			if name := strings.TrimSpace(option.Name); name != "" {
				parts = append(parts, name)
			}
			parts = append(parts, flattenCommandOptions(option.Options)...)
		default:
			value := optionStringValue(option.Value)
			if value != "" {
				parts = append(parts, value)
			}
		}
	}
	return parts
}

func optionStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
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
		interactionToken := ""
		originalResponseID := ""
		if value.ReplyTarget != nil {
			interactionToken = strings.TrimSpace(value.ReplyTarget.InteractionToken)
			originalResponseID = strings.TrimSpace(value.ReplyTarget.OriginalResponseID)
		}
		return replyContext{InteractionToken: interactionToken, ChannelID: value.ChatID, OriginalResponseID: originalResponseID}
	default:
		return replyContext{}
	}
}

func validateRequestSignature(publicKeyHex, timestamp, signatureHex string, body []byte) error {
	publicKey, err := decodePublicKey(publicKeyHex)
	if err != nil {
		return err
	}
	if strings.TrimSpace(timestamp) == "" {
		return errors.New("missing discord request timestamp")
	}
	signature, err := hex.DecodeString(strings.TrimSpace(signatureHex))
	if err != nil {
		return fmt.Errorf("decode discord signature: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return errors.New("invalid discord signature size")
	}
	if !ed25519.Verify(publicKey, append([]byte(timestamp), body...), signature) {
		return errors.New("discord request signature verification failed")
	}
	return nil
}

func decodePublicKey(raw string) (ed25519.PublicKey, error) {
	decoded, err := hex.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("decode discord public key: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, errors.New("discord public key must be 32 bytes")
	}
	return ed25519.PublicKey(decoded), nil
}

func buildMessageComponents(message *core.StructuredMessage) []discordComponent {
	if message == nil || len(message.Actions) == 0 {
		return nil
	}

	rows := make([]discordComponent, 0, len(message.Actions))
	for _, action := range message.Actions {
		label := strings.TrimSpace(action.Label)
		if label == "" {
			continue
		}
		button := discordComponent{
			Type:  componentTypeButton,
			Label: label,
		}
		switch {
		case strings.TrimSpace(action.URL) != "":
			button.Style = componentStyleLink
			button.URL = strings.TrimSpace(action.URL)
		case strings.TrimSpace(action.ID) != "":
			button.Style = discordComponentStyle(action.Style)
			button.CustomID = strings.TrimSpace(action.ID)
		default:
			continue
		}
		rows = append(rows, discordComponent{
			Type:       componentTypeActionRow,
			Components: []discordComponent{button},
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return rows
}

func structuredFallbackText(message *core.StructuredMessage) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(message.FallbackText())
}

func renderDiscordNativeMessage(message *core.NativeMessage) (discordOutgoingMessage, error) {
	if err := message.Validate(); err != nil {
		return discordOutgoingMessage{}, err
	}
	if message.NormalizedPlatform() != "discord" || message.DiscordEmbed == nil {
		return discordOutgoingMessage{}, errors.New("native message is not a discord embed payload")
	}

	embed := discordEmbed{
		Title:       strings.TrimSpace(message.DiscordEmbed.Title),
		Description: strings.TrimSpace(message.DiscordEmbed.Description),
		Color:       message.DiscordEmbed.Color,
	}
	for _, field := range message.DiscordEmbed.Fields {
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name:   strings.TrimSpace(field.Name),
			Value:  strings.TrimSpace(field.Value),
			Inline: field.Inline,
		})
	}
	components := renderDiscordActionRows(message.DiscordEmbed.Components)

	return discordOutgoingMessage{
		Content:    strings.TrimSpace(message.FallbackText()),
		Embeds:     []discordEmbed{embed},
		Components: components,
	}, nil
}

func renderDiscordFormattedTextMessage(message *core.FormattedText) (discordOutgoingMessage, error) {
	if message == nil {
		return discordOutgoingMessage{}, errors.New("formatted text is required")
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return discordOutgoingMessage{}, errors.New("formatted text content is required")
	}

	switch message.Format {
	case "", core.TextFormatPlainText:
		content = escapeDiscordMarkdown(content)
	case core.TextFormatDiscordMD:
		// keep native markdown untouched
	default:
		content = escapeDiscordMarkdown(content)
	}

	return discordOutgoingMessage{Content: content}, nil
}

func renderDiscordStructuredMessage(message *core.StructuredMessage) discordOutgoingMessage {
	outgoing := discordOutgoingMessage{Content: structuredFallbackText(message)}
	if message != nil && len(message.Sections) > 0 {
		embed, components := renderStructuredSections(message.Sections)
		if !embed.isZero() {
			outgoing.Embeds = []discordEmbed{embed}
		}
		outgoing.Components = components
		return outgoing
	}
	outgoing.Components = buildMessageComponents(message)
	return outgoing
}

func discordComponentStyle(style core.ActionStyle) int {
	switch style {
	case core.ActionStylePrimary:
		return componentStylePrimary
	case core.ActionStyleDanger:
		return componentStyleDanger
	default:
		return componentStyleSecondary
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func valueOrEmpty[T any](value *T, getter func(*T) string) string {
	if value == nil || getter == nil {
		return ""
	}
	return strings.TrimSpace(getter(value))
}

func escapeDiscordMarkdown(content string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		"*", `\*`,
		"_", `\_`,
		"~", `\~`,
		"`", "\\`",
		"|", `\|`,
		">", `\>`,
		"[", `\[`,
		"]", `\]`,
		"(", `\(`,
		")", `\)`,
	)
	return replacer.Replace(content)
}

func (e discordEmbed) isZero() bool {
	return strings.TrimSpace(e.Title) == "" &&
		strings.TrimSpace(e.Description) == "" &&
		len(e.Fields) == 0 &&
		strings.TrimSpace(e.Image.URL) == "" &&
		e.Color == 0
}
