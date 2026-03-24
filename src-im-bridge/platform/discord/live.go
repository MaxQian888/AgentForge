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
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
)

const (
	interactionTypePing               = 1
	interactionTypeApplicationCommand = 2

	interactionCallbackTypePong                             = 1
	interactionCallbackTypeChannelMessageWithSource         = 4
	interactionCallbackTypeDeferredChannelMessageWithSource = 5

	commandOptionTypeSubCommand      = 1
	commandOptionTypeSubCommandGroup = 2
	commandOptionTypeString          = 3

	ephemeralMessageFlag = 1 << 6
)

var liveMetadata = core.PlatformMetadata{
	Source: "discord",
	Capabilities: core.PlatformCapabilities{
		SupportsDeferredReply: true,
		SupportsSlashCommands: true,
	},
}

type interaction struct {
	Type          int                     `json:"type"`
	Token         string                  `json:"token,omitempty"`
	ApplicationID string                  `json:"application_id,omitempty"`
	GuildID       string                  `json:"guild_id,omitempty"`
	ChannelID     string                  `json:"channel_id,omitempty"`
	Member        *interactionMember      `json:"member,omitempty"`
	User          *interactionUser        `json:"user,omitempty"`
	Data          *applicationCommandData `json:"data,omitempty"`
}

type interactionMember struct {
	User *interactionUser `json:"user,omitempty"`
}

type interactionUser struct {
	ID         string `json:"id,omitempty"`
	Username   string `json:"username,omitempty"`
	GlobalName string `json:"global_name,omitempty"`
}

type applicationCommandData struct {
	Name    string                     `json:"name"`
	Options []applicationCommandOption `json:"options,omitempty"`
}

type applicationCommandOption struct {
	Type        int                        `json:"type"`
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Required    bool                       `json:"required,omitempty"`
	Value       any                        `json:"value,omitempty"`
	Options     []applicationCommandOption `json:"options,omitempty"`
}

type applicationCommand struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Options     []applicationCommandOption `json:"options,omitempty"`
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
	InteractionToken string
	ChannelID        string
}

type interactionEnvelope struct {
	Interaction *interaction
	Ack         func(interactionResponse) error
}

type interactionRunner interface {
	Start(ctx context.Context, handler func(context.Context, interactionEnvelope) error) error
	Stop(ctx context.Context) error
}

type followupClient interface {
	SendFollowup(ctx context.Context, appID, token, content string) error
}

type channelClient interface {
	SendChannelMessage(ctx context.Context, channelID, content string) error
}

type commandRegistrar interface {
	SyncCommands(ctx context.Context, appID, guildID string, commands []applicationCommand) error
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
	registrar commandRegistrar

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
		registrar: &discordCommandRegistrar{client: apiClient},
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

func (l *Live) Metadata() core.PlatformMetadata { return liveMetadata }

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	return replyContext{
		InteractionToken: strings.TrimSpace(target.InteractionToken),
		ChannelID:        firstNonEmpty(target.ChannelID, target.ChatID),
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
				log.Printf("[discord-live] Ignoring inbound interaction: %v", err)
				return nil
			}
			handler(l, msg)
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
	if strings.TrimSpace(reply.InteractionToken) != "" {
		return l.followups.SendFollowup(ctx, l.appID, reply.InteractionToken, content)
	}
	if strings.TrimSpace(reply.ChannelID) != "" {
		return l.channels.SendChannelMessage(ctx, reply.ChannelID, content)
	}
	return errors.New("discord reply requires interaction token or channel id")
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	channelID := strings.TrimSpace(chatID)
	if channelID == "" {
		return errors.New("discord send requires channel id")
	}
	return l.channels.SendChannelMessage(ctx, channelID, content)
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
			log.Printf("[discord-live] interaction handling failed after acknowledgement: %v", err)
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
			log.Printf("[discord-live] interaction server stopped with error: %v", err)
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

func (c *discordFollowupClient) SendFollowup(ctx context.Context, appID, token, content string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("discord followup requires interaction token")
	}
	return c.client.doJSON(ctx, http.MethodPost, "/webhooks/"+strings.TrimSpace(appID)+"/"+strings.TrimSpace(token), map[string]string{
		"content": content,
	}, false)
}

type discordChannelClient struct {
	client *discordAPIClient
}

func (c *discordChannelClient) SendChannelMessage(ctx context.Context, channelID, content string) error {
	if strings.TrimSpace(channelID) == "" {
		return errors.New("discord channel send requires channel id")
	}
	return c.client.doJSON(ctx, http.MethodPost, "/channels/"+strings.TrimSpace(channelID)+"/messages", map[string]string{
		"content": content,
	}, true)
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

func defaultApplicationCommands() []applicationCommand {
	return []applicationCommand{
		{
			Name:        "task",
			Description: "Run AgentForge task commands.",
			Options: []applicationCommandOption{
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
			Options: []applicationCommandOption{
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
			InteractionToken: strings.TrimSpace(raw.Token),
			ChannelID:        strings.TrimSpace(raw.ChannelID),
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:         liveMetadata.Source,
			ChatID:           strings.TrimSpace(raw.ChannelID),
			ChannelID:        strings.TrimSpace(raw.ChannelID),
			InteractionToken: strings.TrimSpace(raw.Token),
			UseReply:         true,
		},
		Timestamp: time.Now(),
		IsGroup:   strings.TrimSpace(raw.GuildID) != "",
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

func commandContent(data *applicationCommandData) (string, error) {
	command := strings.ToLower(strings.TrimSpace(data.Name))
	if command == "" {
		return "", errors.New("discord interaction missing command name")
	}

	parts := []string{"/" + command}
	parts = append(parts, flattenCommandOptions(data.Options)...)
	return strings.TrimSpace(strings.Join(parts, " ")), nil
}

func flattenCommandOptions(options []applicationCommandOption) []string {
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
		return replyContext{ChannelID: value.ChatID}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
