package core

// CommandSurface describes how a platform primarily receives shared commands.
type CommandSurface string

const (
	CommandSurfaceNone          CommandSurface = "none"
	CommandSurfaceSlash         CommandSurface = "slash"
	CommandSurfaceMention       CommandSurface = "mention"
	CommandSurfaceInteraction   CommandSurface = "interaction"
	CommandSurfaceCallbackQuery CommandSurface = "callback_query"
	CommandSurfaceMixed         CommandSurface = "mixed"
)

// StructuredSurface describes the native rich-message surface for a platform.
type StructuredSurface string

const (
	StructuredSurfaceNone           StructuredSurface = "none"
	StructuredSurfaceBlocks         StructuredSurface = "blocks"
	StructuredSurfaceCards          StructuredSurface = "cards"
	StructuredSurfaceInlineKeyboard StructuredSurface = "inline_keyboard"
	StructuredSurfaceActionCard     StructuredSurface = "action_card"
	StructuredSurfaceComponents     StructuredSurface = "components"
)

// AsyncUpdateMode describes how asynchronous progress or terminal updates
// should be routed back to the originating conversation.
type AsyncUpdateMode string

const (
	AsyncUpdateReply              AsyncUpdateMode = "reply"
	AsyncUpdateThreadReply        AsyncUpdateMode = "thread_reply"
	AsyncUpdateEdit               AsyncUpdateMode = "edit"
	AsyncUpdateFollowUp           AsyncUpdateMode = "follow_up"
	AsyncUpdateSessionWebhook     AsyncUpdateMode = "session_webhook"
	AsyncUpdateDeferredCardUpdate AsyncUpdateMode = "deferred_card_update"
)

// ActionCallbackMode describes how interactive callbacks arrive back at the
// bridge.
type ActionCallbackMode string

const (
	ActionCallbackNone             ActionCallbackMode = "none"
	ActionCallbackWebhook          ActionCallbackMode = "webhook"
	ActionCallbackSocketPayload    ActionCallbackMode = "socket_payload"
	ActionCallbackInteractionToken ActionCallbackMode = "interaction_token"
	ActionCallbackQuery            ActionCallbackMode = "callback_query"
)

// MessageScope describes where follow-up messages can stay anchored.
type MessageScope string

const (
	MessageScopeChat              MessageScope = "chat"
	MessageScopeThread            MessageScope = "thread"
	MessageScopeTopic             MessageScope = "topic"
	MessageScopeInteractionScoped MessageScope = "interaction_scoped"
)

// MutabilitySemantics captures whether a platform can update an existing
// message and whether in-place mutation is preferred for noisy progress.
type MutabilitySemantics struct {
	CanEdit        bool `json:"canEdit,omitempty"`
	CanDelete      bool `json:"canDelete,omitempty"`
	PrefersInPlace bool `json:"prefersInPlace,omitempty"`
}

// PlatformCapabilities describes behavior that higher-level bridge components
// can use without hard-coding platform names.
type PlatformCapabilities struct {
	CommandSurface     CommandSurface      `json:"commandSurface,omitempty"`
	StructuredSurface  StructuredSurface   `json:"structuredSurface,omitempty"`
	AsyncUpdateModes   []AsyncUpdateMode   `json:"asyncUpdateModes,omitempty"`
	ActionCallbackMode ActionCallbackMode  `json:"actionCallbackMode,omitempty"`
	MessageScopes      []MessageScope      `json:"messageScopes,omitempty"`
	Mutability         MutabilitySemantics `json:"mutability,omitempty"`

	SupportsRichMessages   bool `json:"supportsRichMessages,omitempty"`
	SupportsDeferredReply  bool `json:"supportsDeferredReply,omitempty"`
	RequiresPublicCallback bool `json:"requiresPublicCallback,omitempty"`
	SupportsSlashCommands  bool `json:"supportsSlashCommands,omitempty"`
	SupportsMentions       bool `json:"supportsMentions,omitempty"`
}

// Matrix returns the structured capability matrix in a transport-friendly form.
func (c PlatformCapabilities) Matrix() map[string]any {
	matrix := map[string]any{
		"commandSurface":     string(c.CommandSurface),
		"structuredSurface":  string(c.StructuredSurface),
		"actionCallbackMode": string(c.ActionCallbackMode),
		"mutability": map[string]bool{
			"canEdit":        c.Mutability.CanEdit,
			"canDelete":      c.Mutability.CanDelete,
			"prefersInPlace": c.Mutability.PrefersInPlace,
		},
	}
	if len(c.AsyncUpdateModes) > 0 {
		modes := make([]string, 0, len(c.AsyncUpdateModes))
		for _, mode := range c.AsyncUpdateModes {
			modes = append(modes, string(mode))
		}
		matrix["asyncUpdateModes"] = modes
	}
	if len(c.MessageScopes) > 0 {
		scopes := make([]string, 0, len(c.MessageScopes))
		for _, scope := range c.MessageScopes {
			scopes = append(scopes, string(scope))
		}
		matrix["messageScopes"] = scopes
	}
	return matrix
}

// HasAsyncUpdateMode reports whether the capability matrix declares the given
// asynchronous update mode.
func (c PlatformCapabilities) HasAsyncUpdateMode(target AsyncUpdateMode) bool {
	for _, mode := range c.AsyncUpdateModes {
		if mode == target {
			return true
		}
	}
	return false
}

// HasMessageScope reports whether the capability matrix declares the given
// message scope.
func (c PlatformCapabilities) HasMessageScope(target MessageScope) bool {
	for _, scope := range c.MessageScopes {
		if scope == target {
			return true
		}
	}
	return false
}

// PlatformMetadata captures the stable source identity and declared
// capabilities of a platform implementation.
type PlatformMetadata struct {
	Source       string
	Capabilities PlatformCapabilities
	Rendering    RenderingProfile
}

// MetadataProvider is an optional interface for platforms that can declare
// metadata explicitly.
type MetadataProvider interface {
	Metadata() PlatformMetadata
}

// NormalizeMetadata normalizes platform metadata using the provided source as a
// fallback when the metadata itself does not declare one.
func NormalizeMetadata(metadata PlatformMetadata, fallbackSource string) PlatformMetadata {
	source := NormalizePlatformName(metadata.Source)
	if source == "" {
		source = NormalizePlatformName(fallbackSource)
	}
	metadata.Source = source
	defaultCapabilities := defaultCapabilitiesForSource(source, nil)
	metadata.Capabilities = normalizeCapabilities(metadata.Capabilities, defaultCapabilities)
	metadata.Rendering = normalizeRenderingProfile(metadata.Rendering, defaultRenderingProfileForSource(source, metadata.Capabilities))
	return metadata
}

// MetadataForPlatform returns normalized metadata for a platform. Platforms can
// override the defaults by implementing MetadataProvider.
func MetadataForPlatform(platform Platform) PlatformMetadata {
	defaults := PlatformMetadata{
		Source:       NormalizePlatformName(platform.Name()),
		Capabilities: defaultCapabilitiesForPlatform(platform),
	}
	if _, ok := platform.(CardSender); ok {
		defaults.Capabilities.SupportsRichMessages = true
		if defaults.Capabilities.StructuredSurface == "" || defaults.Capabilities.StructuredSurface == StructuredSurfaceNone {
			defaults.Capabilities.StructuredSurface = StructuredSurfaceCards
		}
	}
	defaults = NormalizeMetadata(defaults, defaults.Source)

	provider, ok := platform.(MetadataProvider)
	if !ok {
		return defaults
	}

	metadata := provider.Metadata()
	normalized := NormalizeMetadata(metadata, defaults.Source)
	normalized.Capabilities = normalizeCapabilities(normalized.Capabilities, defaults.Capabilities)
	normalized.Rendering = normalizeRenderingProfile(normalized.Rendering, defaultRenderingProfileForSource(normalized.Source, normalized.Capabilities))
	return normalized
}

func defaultCapabilitiesForPlatform(platform Platform) PlatformCapabilities {
	source := NormalizePlatformName(platform.Name())
	return defaultCapabilitiesForSource(source, platform)
}

func defaultCapabilitiesForSource(source string, platform Platform) PlatformCapabilities {
	switch source {
	case "slack":
		return PlatformCapabilities{
			CommandSurface:     CommandSurfaceMixed,
			StructuredSurface:  StructuredSurfaceBlocks,
			AsyncUpdateModes:   []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateThreadReply, AsyncUpdateFollowUp},
			ActionCallbackMode: ActionCallbackSocketPayload,
			MessageScopes:      []MessageScope{MessageScopeChat, MessageScopeThread},
		}
	case "discord":
		return PlatformCapabilities{
			CommandSurface:     CommandSurfaceInteraction,
			StructuredSurface:  StructuredSurfaceComponents,
			AsyncUpdateModes:   []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateFollowUp, AsyncUpdateEdit},
			ActionCallbackMode: ActionCallbackInteractionToken,
			MessageScopes:      []MessageScope{MessageScopeInteractionScoped, MessageScopeChat},
			Mutability: MutabilitySemantics{
				CanEdit:        true,
				PrefersInPlace: true,
			},
		}
	case "telegram":
		return PlatformCapabilities{
			CommandSurface:     CommandSurfaceMixed,
			StructuredSurface:  StructuredSurfaceInlineKeyboard,
			AsyncUpdateModes:   []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateEdit},
			ActionCallbackMode: ActionCallbackQuery,
			MessageScopes:      []MessageScope{MessageScopeChat, MessageScopeTopic},
			Mutability: MutabilitySemantics{
				CanEdit:        true,
				PrefersInPlace: true,
			},
		}
	case "feishu":
		return PlatformCapabilities{
			CommandSurface:     CommandSurfaceMixed,
			StructuredSurface:  StructuredSurfaceCards,
			AsyncUpdateModes:   []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateDeferredCardUpdate},
			ActionCallbackMode: ActionCallbackWebhook,
			MessageScopes:      []MessageScope{MessageScopeChat, MessageScopeThread},
			Mutability: MutabilitySemantics{
				CanEdit:        true,
				PrefersInPlace: true,
			},
		}
	case "dingtalk":
		return PlatformCapabilities{
			CommandSurface:     CommandSurfaceMixed,
			StructuredSurface:  StructuredSurfaceActionCard,
			AsyncUpdateModes:   []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateSessionWebhook},
			ActionCallbackMode: ActionCallbackWebhook,
			MessageScopes:      []MessageScope{MessageScopeChat},
		}
	case "qq":
		return PlatformCapabilities{
			CommandSurface:        CommandSurfaceMixed,
			StructuredSurface:     StructuredSurfaceNone,
			AsyncUpdateModes:      []AsyncUpdateMode{AsyncUpdateReply},
			ActionCallbackMode:    ActionCallbackNone,
			MessageScopes:         []MessageScope{MessageScopeChat},
			SupportsMentions:      true,
			SupportsSlashCommands: true,
		}
	case "qqbot":
		return PlatformCapabilities{
			CommandSurface:         CommandSurfaceMixed,
			StructuredSurface:      StructuredSurfaceNone,
			AsyncUpdateModes:       []AsyncUpdateMode{AsyncUpdateReply},
			ActionCallbackMode:     ActionCallbackWebhook,
			MessageScopes:          []MessageScope{MessageScopeChat},
			RequiresPublicCallback: true,
			SupportsMentions:       true,
			SupportsSlashCommands:  true,
		}
	default:
		capabilities := PlatformCapabilities{
			CommandSurface: CommandSurfaceNone,
			MessageScopes:  []MessageScope{MessageScopeChat},
		}
		if _, ok := platform.(CardSender); ok {
			capabilities.StructuredSurface = StructuredSurfaceCards
			capabilities.SupportsRichMessages = true
		}
		return capabilities
	}
}

func normalizeCapabilities(capabilities PlatformCapabilities, defaults PlatformCapabilities) PlatformCapabilities {
	if capabilities.CommandSurface == "" {
		switch {
		case capabilities.SupportsSlashCommands && capabilities.SupportsMentions:
			capabilities.CommandSurface = CommandSurfaceMixed
		case capabilities.SupportsSlashCommands:
			capabilities.CommandSurface = CommandSurfaceSlash
		case capabilities.SupportsMentions:
			capabilities.CommandSurface = CommandSurfaceMention
		default:
			capabilities.CommandSurface = defaults.CommandSurface
		}
	}

	if capabilities.StructuredSurface == "" {
		switch {
		case capabilities.SupportsRichMessages:
			if defaults.StructuredSurface != "" {
				capabilities.StructuredSurface = defaults.StructuredSurface
			} else {
				capabilities.StructuredSurface = StructuredSurfaceCards
			}
		default:
			capabilities.StructuredSurface = defaults.StructuredSurface
		}
	}
	if capabilities.StructuredSurface == "" {
		capabilities.StructuredSurface = StructuredSurfaceNone
	}

	if len(capabilities.AsyncUpdateModes) == 0 {
		switch {
		case capabilities.SupportsDeferredReply:
			capabilities.AsyncUpdateModes = []AsyncUpdateMode{AsyncUpdateFollowUp}
		case len(defaults.AsyncUpdateModes) > 0:
			capabilities.AsyncUpdateModes = append([]AsyncUpdateMode(nil), defaults.AsyncUpdateModes...)
		}
	}

	if capabilities.ActionCallbackMode == "" {
		switch {
		case capabilities.RequiresPublicCallback:
			capabilities.ActionCallbackMode = ActionCallbackInteractionToken
		case defaults.ActionCallbackMode != "":
			capabilities.ActionCallbackMode = defaults.ActionCallbackMode
		case capabilities.StructuredSurface != StructuredSurfaceNone || capabilities.SupportsRichMessages || capabilities.SupportsSlashCommands || capabilities.SupportsMentions:
			capabilities.ActionCallbackMode = ActionCallbackWebhook
		default:
			capabilities.ActionCallbackMode = ActionCallbackNone
		}
	}

	if len(capabilities.MessageScopes) == 0 {
		if len(defaults.MessageScopes) > 0 {
			capabilities.MessageScopes = append([]MessageScope(nil), defaults.MessageScopes...)
		} else {
			capabilities.MessageScopes = []MessageScope{MessageScopeChat}
		}
	}

	if capabilities.Mutability == (MutabilitySemantics{}) {
		capabilities.Mutability = defaults.Mutability
	}

	capabilities.SupportsDeferredReply = capabilities.SupportsDeferredReply ||
		capabilities.HasAsyncUpdateMode(AsyncUpdateFollowUp) ||
		capabilities.HasAsyncUpdateMode(AsyncUpdateSessionWebhook) ||
		capabilities.HasAsyncUpdateMode(AsyncUpdateDeferredCardUpdate)
	capabilities.RequiresPublicCallback = capabilities.RequiresPublicCallback || capabilities.ActionCallbackMode == ActionCallbackInteractionToken
	capabilities.SupportsSlashCommands = capabilities.SupportsSlashCommands ||
		capabilities.CommandSurface == CommandSurfaceSlash ||
		capabilities.CommandSurface == CommandSurfaceMixed ||
		capabilities.CommandSurface == CommandSurfaceInteraction
	capabilities.SupportsMentions = capabilities.SupportsMentions ||
		capabilities.CommandSurface == CommandSurfaceMention ||
		capabilities.CommandSurface == CommandSurfaceMixed

	return capabilities
}
