package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	NativeSurfaceFeishuCard    = "feishu_card"
	NativeSurfaceSlackBlockKit = "slack_block_kit"
	NativeSurfaceDiscordEmbed  = "discord_embed"
	NativeSurfaceTelegramRich  = "telegram_rich"
	NativeSurfaceDingTalkCard  = "dingtalk_card"
	NativeSurfaceWeComCard     = "wecom_card"
	NativeSurfaceQQBotMarkdown = "qqbot_markdown"
)

type FeishuCardMode string

const (
	FeishuCardModeJSON     FeishuCardMode = "json"
	FeishuCardModeTemplate FeishuCardMode = "template"
)

type DingTalkCardType string

const (
	DingTalkCardTypeActionCard DingTalkCardType = "action_card"
	DingTalkCardTypeFeedCard   DingTalkCardType = "feed_card"
)

type WeComCardType string

const (
	WeComCardTypeNews     WeComCardType = "news"
	WeComCardTypeTemplate WeComCardType = "template"
)

// NativeMessage is a typed provider-native payload wrapper for richer message
// surfaces that should not be forced into the shared structured-message model.
type NativeMessage struct {
	Platform      string                `json:"platform,omitempty"`
	FeishuCard    *FeishuCardPayload    `json:"feishuCard,omitempty"`
	SlackBlockKit *SlackBlockKitPayload `json:"slackBlockKit,omitempty"`
	DiscordEmbed  *DiscordEmbedPayload  `json:"discordEmbed,omitempty"`
	TelegramRich  *TelegramRichPayload  `json:"telegramRich,omitempty"`
	DingTalkCard  *DingTalkCardPayload  `json:"dingTalkCard,omitempty"`
	WeComCard     *WeComCardPayload     `json:"weComCard,omitempty"`
	QQBotMarkdown *QQBotMarkdownPayload `json:"qqBotMarkdown,omitempty"`
}

// FeishuCardPayload captures the two supported Feishu interactive card send
// models: raw JSON card content and template-based cards with variables.
type FeishuCardPayload struct {
	Mode                FeishuCardMode  `json:"mode"`
	JSON                json.RawMessage `json:"json,omitempty"`
	TemplateID          string          `json:"templateId,omitempty"`
	TemplateVersionName string          `json:"templateVersionName,omitempty"`
	TemplateVariable    map[string]any  `json:"templateVariable,omitempty"`
}

type SlackBlockKitPayload struct {
	Blocks json.RawMessage `json:"blocks,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type DiscordButton struct {
	Label    string `json:"label,omitempty"`
	CustomID string `json:"customId,omitempty"`
	URL      string `json:"url,omitempty"`
	Style    string `json:"style,omitempty"`
}

type DiscordActionRow struct {
	Buttons []DiscordButton `json:"buttons,omitempty"`
}

type DiscordEmbedPayload struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Components  []DiscordActionRow  `json:"components,omitempty"`
}

type TelegramInlineButton struct {
	Text         string `json:"text,omitempty"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callbackData,omitempty"`
}

type TelegramRichPayload struct {
	Text           string                   `json:"text,omitempty"`
	ParseMode      string                   `json:"parseMode,omitempty"`
	InlineKeyboard [][]TelegramInlineButton `json:"inlineKeyboard,omitempty"`
}

type DingTalkCardButton struct {
	Title     string `json:"title,omitempty"`
	ActionURL string `json:"actionUrl,omitempty"`
}

type DingTalkCardPayload struct {
	CardType DingTalkCardType     `json:"cardType,omitempty"`
	Title    string               `json:"title,omitempty"`
	Markdown string               `json:"markdown,omitempty"`
	Buttons  []DingTalkCardButton `json:"buttons,omitempty"`
}

type WeComArticle struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	PicURL      string `json:"picUrl,omitempty"`
}

type WeComTemplateField struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type WeComCardPayload struct {
	CardType       WeComCardType        `json:"cardType,omitempty"`
	Title          string               `json:"title,omitempty"`
	Description    string               `json:"description,omitempty"`
	URL            string               `json:"url,omitempty"`
	Articles       []WeComArticle       `json:"articles,omitempty"`
	TemplateFields []WeComTemplateField `json:"templateFields,omitempty"`
}

type QQBotKeyboardButton struct {
	Label  string `json:"label,omitempty"`
	URL    string `json:"url,omitempty"`
	Action string `json:"action,omitempty"`
}

type QQBotMarkdownPayload struct {
	Markdown string                  `json:"markdown,omitempty"`
	Keyboard [][]QQBotKeyboardButton `json:"keyboard,omitempty"`
}

func NewFeishuJSONCardMessage(payload map[string]any) (*NativeMessage, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("feishu json card payload is required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode feishu json card payload: %w", err)
	}
	message := &NativeMessage{
		Platform: "feishu",
		FeishuCard: &FeishuCardPayload{
			Mode: FeishuCardModeJSON,
			JSON: body,
		},
	}
	return message, message.Validate()
}

func NewFeishuTemplateCardMessage(templateID, version string, variables map[string]any) (*NativeMessage, error) {
	message := &NativeMessage{
		Platform: "feishu",
		FeishuCard: &FeishuCardPayload{
			Mode:                FeishuCardModeTemplate,
			TemplateID:          strings.TrimSpace(templateID),
			TemplateVersionName: strings.TrimSpace(version),
			TemplateVariable:    variables,
		},
	}
	return message, message.Validate()
}

func NewFeishuMarkdownCardMessage(title, content string) (*NativeMessage, error) {
	payload := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": strings.TrimSpace(title),
			},
		},
		"elements": []map[string]any{
			{
				"tag": "div",
				"text": map[string]any{
					"tag":     "lark_md",
					"content": strings.TrimSpace(content),
				},
			},
		},
	}
	return NewFeishuJSONCardMessage(payload)
}

func NewSlackBlockKitMessage(blocks []map[string]any) (*NativeMessage, error) {
	body, err := json.Marshal(blocks)
	if err != nil {
		return nil, fmt.Errorf("encode slack blocks: %w", err)
	}
	message := &NativeMessage{
		Platform: "slack",
		SlackBlockKit: &SlackBlockKitPayload{
			Blocks: body,
		},
	}
	return message, message.Validate()
}

func NewDiscordEmbedMessage(title, description string, fields []DiscordEmbedField, color int, components []DiscordActionRow) (*NativeMessage, error) {
	message := &NativeMessage{
		Platform: "discord",
		DiscordEmbed: &DiscordEmbedPayload{
			Title:       strings.TrimSpace(title),
			Description: strings.TrimSpace(description),
			Fields:      append([]DiscordEmbedField(nil), fields...),
			Color:       color,
			Components:  append([]DiscordActionRow(nil), components...),
		},
	}
	return message, message.Validate()
}

func NewTelegramRichMessage(text, parseMode string, keyboard [][]TelegramInlineButton) (*NativeMessage, error) {
	message := &NativeMessage{
		Platform: "telegram",
		TelegramRich: &TelegramRichPayload{
			Text:           strings.TrimSpace(text),
			ParseMode:      strings.TrimSpace(parseMode),
			InlineKeyboard: append([][]TelegramInlineButton(nil), keyboard...),
		},
	}
	return message, message.Validate()
}

func NewDingTalkCardMessage(cardType DingTalkCardType, title, markdown string, buttons []DingTalkCardButton) (*NativeMessage, error) {
	if strings.TrimSpace(string(cardType)) == "" {
		cardType = DingTalkCardTypeActionCard
	}
	message := &NativeMessage{
		Platform: "dingtalk",
		DingTalkCard: &DingTalkCardPayload{
			CardType: cardType,
			Title:    strings.TrimSpace(title),
			Markdown: strings.TrimSpace(markdown),
			Buttons:  append([]DingTalkCardButton(nil), buttons...),
		},
	}
	return message, message.Validate()
}

func NewWeComCardMessage(cardType WeComCardType, title, description, url string, articles []WeComArticle, templateFields []WeComTemplateField) (*NativeMessage, error) {
	message := &NativeMessage{
		Platform: "wecom",
		WeComCard: &WeComCardPayload{
			CardType:       cardType,
			Title:          strings.TrimSpace(title),
			Description:    strings.TrimSpace(description),
			URL:            strings.TrimSpace(url),
			Articles:       append([]WeComArticle(nil), articles...),
			TemplateFields: append([]WeComTemplateField(nil), templateFields...),
		},
	}
	return message, message.Validate()
}

func NewQQBotMarkdownMessage(markdown string, keyboard [][]QQBotKeyboardButton) (*NativeMessage, error) {
	message := &NativeMessage{
		Platform: "qqbot",
		QQBotMarkdown: &QQBotMarkdownPayload{
			Markdown: strings.TrimSpace(markdown),
			Keyboard: append([][]QQBotKeyboardButton(nil), keyboard...),
		},
	}
	return message, message.Validate()
}

func (m *NativeMessage) NormalizedPlatform() string {
	if m == nil {
		return ""
	}
	if normalized := NormalizePlatformName(m.Platform); normalized != "" {
		return normalized
	}
	return m.payloadPlatform()
}

func (m *NativeMessage) SurfaceType() string {
	if m == nil {
		return ""
	}
	switch {
	case m.FeishuCard != nil:
		return NativeSurfaceFeishuCard
	case m.SlackBlockKit != nil:
		return NativeSurfaceSlackBlockKit
	case m.DiscordEmbed != nil:
		return NativeSurfaceDiscordEmbed
	case m.TelegramRich != nil:
		return NativeSurfaceTelegramRich
	case m.DingTalkCard != nil:
		return NativeSurfaceDingTalkCard
	case m.WeComCard != nil:
		return NativeSurfaceWeComCard
	case m.QQBotMarkdown != nil:
		return NativeSurfaceQQBotMarkdown
	default:
		return ""
	}
}

func (m *NativeMessage) FallbackText() string {
	if m == nil {
		return ""
	}
	switch {
	case m.FeishuCard != nil:
		return m.FeishuCard.FallbackText()
	case m.SlackBlockKit != nil:
		return m.SlackBlockKit.FallbackText()
	case m.DiscordEmbed != nil:
		return m.DiscordEmbed.FallbackText()
	case m.TelegramRich != nil:
		return m.TelegramRich.FallbackText()
	case m.DingTalkCard != nil:
		return m.DingTalkCard.FallbackText()
	case m.WeComCard != nil:
		return m.WeComCard.FallbackText()
	case m.QQBotMarkdown != nil:
		return m.QQBotMarkdown.FallbackText()
	default:
		return ""
	}
}

func (m *NativeMessage) Validate() error {
	if m == nil {
		return fmt.Errorf("native message is required")
	}
	activePayloads := m.activePayloadCount()
	if activePayloads == 0 {
		if m.NormalizedPlatform() == "" {
			return fmt.Errorf("native message platform is required")
		}
		return fmt.Errorf("unsupported native message platform %q", m.NormalizedPlatform())
	}
	if activePayloads > 1 {
		return fmt.Errorf("native message requires exactly one platform payload")
	}

	payloadPlatform := m.payloadPlatform()
	if normalized := NormalizePlatformName(m.Platform); normalized != "" && normalized != payloadPlatform {
		return fmt.Errorf("native message platform %q does not match payload %q", normalized, payloadPlatform)
	}

	switch payloadPlatform {
	case "feishu":
		return m.FeishuCard.Validate()
	case "slack":
		return m.SlackBlockKit.Validate()
	case "discord":
		return m.DiscordEmbed.Validate()
	case "telegram":
		return m.TelegramRich.Validate()
	case "dingtalk":
		return m.DingTalkCard.Validate()
	case "wecom":
		return m.WeComCard.Validate()
	case "qqbot":
		return m.QQBotMarkdown.Validate()
	case "":
		return fmt.Errorf("native message platform is required")
	default:
		return fmt.Errorf("unsupported native message platform %q", payloadPlatform)
	}
}

func (p *FeishuCardPayload) Validate() error {
	if p == nil {
		return fmt.Errorf("feishu card payload is required")
	}
	switch FeishuCardMode(strings.ToLower(strings.TrimSpace(string(p.Mode)))) {
	case FeishuCardModeJSON:
		if len(p.JSON) == 0 {
			return fmt.Errorf("feishu json card payload is required")
		}
		var decoded any
		if err := json.Unmarshal(p.JSON, &decoded); err != nil {
			return fmt.Errorf("decode feishu json card payload: %w", err)
		}
		if _, ok := decoded.(map[string]any); !ok {
			return fmt.Errorf("feishu json card payload must decode to an object")
		}
		return nil
	case FeishuCardModeTemplate:
		if strings.TrimSpace(p.TemplateID) == "" {
			return fmt.Errorf("feishu template card requires templateId")
		}
		return nil
	default:
		return fmt.Errorf("unsupported feishu card mode %q", p.Mode)
	}
}

func (p *FeishuCardPayload) FallbackText() string {
	if p == nil {
		return ""
	}
	switch FeishuCardMode(strings.ToLower(strings.TrimSpace(string(p.Mode)))) {
	case FeishuCardModeJSON:
		var decoded map[string]any
		if err := json.Unmarshal(p.JSON, &decoded); err != nil {
			return ""
		}
		lines := make([]string, 0, 2)
		if header, ok := decoded["header"].(map[string]any); ok {
			if title, ok := header["title"].(map[string]any); ok {
				appendNonEmptyLine(&lines, title["content"])
			}
		}
		if elements, ok := decoded["elements"].([]any); ok {
			for _, element := range elements {
				if section, ok := element.(map[string]any); ok {
					if text, ok := section["text"].(map[string]any); ok {
						appendNonEmptyLine(&lines, text["content"])
					}
				}
			}
		}
		return strings.Join(lines, "\n")
	case FeishuCardModeTemplate:
		lines := []string{"Feishu card"}
		if templateID := strings.TrimSpace(p.TemplateID); templateID != "" {
			lines[0] = "Feishu template: " + templateID
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func (p *SlackBlockKitPayload) Validate() error {
	if p == nil {
		return fmt.Errorf("slack block kit payload is required")
	}
	blocks, err := decodeObjectArray(p.Blocks, "slack blocks")
	if err != nil {
		return err
	}
	if len(blocks) == 0 {
		return fmt.Errorf("slack block kit payload requires at least one block")
	}
	if len(blocks) > 50 {
		return fmt.Errorf("slack block kit payload exceeds 50 blocks")
	}
	for _, block := range blocks {
		if strings.TrimSpace(asString(block["type"])) == "" {
			return fmt.Errorf("slack block kit block type is required")
		}
	}
	return nil
}

func (p *SlackBlockKitPayload) FallbackText() string {
	blocks, err := decodeObjectArray(p.Blocks, "slack blocks")
	if err != nil {
		return ""
	}
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch strings.ToLower(strings.TrimSpace(asString(block["type"]))) {
		case "section":
			appendNonEmptyLine(&lines, slackTextValue(block["text"]))
			for _, field := range toObjectSlice(block["fields"]) {
				appendNonEmptyLine(&lines, slackTextValue(field))
			}
		case "context":
			values := make([]string, 0)
			for _, element := range toObjectSlice(block["elements"]) {
				if text := slackTextValue(element); text != "" {
					values = append(values, text)
				}
			}
			if len(values) > 0 {
				lines = append(lines, strings.Join(values, " | "))
			}
		case "actions":
			for _, element := range toObjectSlice(block["elements"]) {
				appendNonEmptyLine(&lines, slackTextValue(element["text"]))
			}
		case "image":
			appendNonEmptyLine(&lines, fallbackImageText(asString(block["alt_text"]), asString(block["image_url"])))
		case "divider":
			lines = append(lines, "---")
		}
	}
	return strings.Join(lines, "\n")
}

func (p *DiscordEmbedPayload) Validate() error {
	if p == nil {
		return fmt.Errorf("discord embed payload is required")
	}
	if strings.TrimSpace(p.Title) == "" && strings.TrimSpace(p.Description) == "" {
		return fmt.Errorf("discord embed requires title or description")
	}
	if len(strings.TrimSpace(p.Description)) > 4096 {
		return fmt.Errorf("discord embed description exceeds 4096 characters")
	}
	for _, field := range p.Fields {
		if strings.TrimSpace(field.Name) == "" || strings.TrimSpace(field.Value) == "" {
			return fmt.Errorf("discord embed fields require name and value")
		}
	}
	for _, row := range p.Components {
		for _, button := range row.Buttons {
			if strings.TrimSpace(button.Label) == "" {
				return fmt.Errorf("discord component button label is required")
			}
			if strings.TrimSpace(button.URL) == "" && strings.TrimSpace(button.CustomID) == "" {
				return fmt.Errorf("discord component button requires url or customId")
			}
		}
	}
	return nil
}

func (p *DiscordEmbedPayload) FallbackText() string {
	if p == nil {
		return ""
	}
	lines := make([]string, 0, 2+len(p.Fields))
	appendNonEmptyLine(&lines, p.Title)
	appendNonEmptyLine(&lines, p.Description)
	for _, field := range p.Fields {
		appendNonEmptyLine(&lines, strings.TrimSpace(field.Name)+": "+strings.TrimSpace(field.Value))
	}
	for _, row := range p.Components {
		for _, button := range row.Buttons {
			appendNonEmptyLine(&lines, actionFallbackText(button.Label, button.URL, button.CustomID))
		}
	}
	return strings.Join(lines, "\n")
}

func (p *TelegramRichPayload) Validate() error {
	if p == nil {
		return fmt.Errorf("telegram rich payload is required")
	}
	if strings.TrimSpace(p.Text) == "" {
		return fmt.Errorf("telegram rich payload text is required")
	}
	if len(strings.TrimSpace(p.Text)) > 4096 {
		return fmt.Errorf("telegram rich payload text exceeds 4096 characters")
	}
	for _, row := range p.InlineKeyboard {
		for _, button := range row {
			if strings.TrimSpace(button.Text) == "" {
				return fmt.Errorf("telegram inline button text is required")
			}
		}
	}
	return nil
}

func (p *TelegramRichPayload) FallbackText() string {
	if p == nil {
		return ""
	}
	lines := []string{strings.TrimSpace(stripMarkdown(p.Text))}
	for _, row := range p.InlineKeyboard {
		for _, button := range row {
			appendNonEmptyLine(&lines, strings.TrimSpace(button.Text))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (p *DingTalkCardPayload) Validate() error {
	if p == nil {
		return fmt.Errorf("dingtalk card payload is required")
	}
	cardType := DingTalkCardType(strings.ToLower(strings.TrimSpace(string(p.CardType))))
	if cardType == "" {
		cardType = DingTalkCardTypeActionCard
	}
	switch cardType {
	case DingTalkCardTypeActionCard, DingTalkCardTypeFeedCard:
	default:
		return fmt.Errorf("unsupported dingtalk card type %q", p.CardType)
	}
	if strings.TrimSpace(p.Title) == "" {
		return fmt.Errorf("dingtalk card title is required")
	}
	if strings.TrimSpace(p.Markdown) == "" {
		return fmt.Errorf("dingtalk card markdown body is required")
	}
	for _, button := range p.Buttons {
		if strings.TrimSpace(button.Title) == "" {
			return fmt.Errorf("dingtalk card button title is required")
		}
	}
	return nil
}

func (p *DingTalkCardPayload) FallbackText() string {
	if p == nil {
		return ""
	}
	lines := []string{
		strings.TrimSpace(stripMarkdown(p.Title)),
		strings.TrimSpace(stripMarkdown(p.Markdown)),
	}
	for _, button := range p.Buttons {
		appendNonEmptyLine(&lines, actionFallbackText(button.Title, button.ActionURL, ""))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (p *WeComCardPayload) Validate() error {
	if p == nil {
		return fmt.Errorf("wecom card payload is required")
	}
	switch WeComCardType(strings.ToLower(strings.TrimSpace(string(p.CardType)))) {
	case WeComCardTypeNews:
		if len(p.Articles) == 0 {
			return fmt.Errorf("wecom news card requires at least one article")
		}
		for _, article := range p.Articles {
			if strings.TrimSpace(article.Title) == "" {
				return fmt.Errorf("wecom article title is required")
			}
		}
	case WeComCardTypeTemplate:
		if strings.TrimSpace(p.Title) == "" && len(p.TemplateFields) == 0 {
			return fmt.Errorf("wecom template card requires title or template fields")
		}
	default:
		return fmt.Errorf("unsupported wecom card type %q", p.CardType)
	}
	return nil
}

func (p *WeComCardPayload) FallbackText() string {
	if p == nil {
		return ""
	}
	lines := make([]string, 0, 2+len(p.Articles)+len(p.TemplateFields))
	appendNonEmptyLine(&lines, p.Title)
	appendNonEmptyLine(&lines, p.Description)
	for _, article := range p.Articles {
		appendNonEmptyLine(&lines, actionFallbackText(article.Title, article.URL, ""))
		appendNonEmptyLine(&lines, article.Description)
	}
	for _, field := range p.TemplateFields {
		appendNonEmptyLine(&lines, strings.TrimSpace(field.Key)+": "+strings.TrimSpace(field.Value))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (p *QQBotMarkdownPayload) Validate() error {
	if p == nil {
		return fmt.Errorf("qq bot markdown payload is required")
	}
	if strings.TrimSpace(p.Markdown) == "" {
		return fmt.Errorf("qq bot markdown content is required")
	}
	for _, row := range p.Keyboard {
		for _, button := range row {
			if strings.TrimSpace(button.Label) == "" {
				return fmt.Errorf("qq bot keyboard button label is required")
			}
		}
	}
	return nil
}

func (p *QQBotMarkdownPayload) FallbackText() string {
	if p == nil {
		return ""
	}
	lines := []string{strings.TrimSpace(stripMarkdown(p.Markdown))}
	for _, row := range p.Keyboard {
		for _, button := range row {
			appendNonEmptyLine(&lines, actionFallbackText(button.Label, button.URL, button.Action))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (m *NativeMessage) activePayloadCount() int {
	if m == nil {
		return 0
	}
	count := 0
	if m.FeishuCard != nil {
		count++
	}
	if m.SlackBlockKit != nil {
		count++
	}
	if m.DiscordEmbed != nil {
		count++
	}
	if m.TelegramRich != nil {
		count++
	}
	if m.DingTalkCard != nil {
		count++
	}
	if m.WeComCard != nil {
		count++
	}
	if m.QQBotMarkdown != nil {
		count++
	}
	return count
}

func (m *NativeMessage) payloadPlatform() string {
	switch {
	case m == nil:
		return ""
	case m.FeishuCard != nil:
		return "feishu"
	case m.SlackBlockKit != nil:
		return "slack"
	case m.DiscordEmbed != nil:
		return "discord"
	case m.TelegramRich != nil:
		return "telegram"
	case m.DingTalkCard != nil:
		return "dingtalk"
	case m.WeComCard != nil:
		return "wecom"
	case m.QQBotMarkdown != nil:
		return "qqbot"
	default:
		return ""
	}
}

func decodeObjectArray(raw json.RawMessage, label string) ([]map[string]any, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s is required", label)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	return decoded, nil
}

func appendNonEmptyLine(lines *[]string, value any) {
	if lines == nil {
		return
	}
	text := strings.TrimSpace(stripMarkdown(asString(value)))
	if text != "" {
		*lines = append(*lines, text)
	}
}

func toObjectSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, entry := range typed {
			if object, ok := entry.(map[string]any); ok {
				result = append(result, object)
			}
		}
		return result
	default:
		return nil
	}
}

func slackTextValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		return strings.TrimSpace(stripMarkdown(asString(typed["text"])))
	default:
		return strings.TrimSpace(stripMarkdown(asString(value)))
	}
}

func actionFallbackText(label, url, action string) string {
	trimmedLabel := strings.TrimSpace(stripMarkdown(label))
	trimmedURL := strings.TrimSpace(url)
	trimmedAction := strings.TrimSpace(action)
	switch {
	case trimmedLabel != "" && trimmedURL != "":
		return trimmedLabel + ": " + trimmedURL
	case trimmedLabel != "" && trimmedAction != "":
		return trimmedLabel + ": " + trimmedAction
	case trimmedLabel != "":
		return trimmedLabel
	case trimmedURL != "":
		return trimmedURL
	default:
		return trimmedAction
	}
}

func fallbackImageText(altText, url string) string {
	trimmedAlt := strings.TrimSpace(stripMarkdown(altText))
	trimmedURL := strings.TrimSpace(url)
	switch {
	case trimmedAlt != "" && trimmedURL != "":
		return trimmedAlt + ": " + trimmedURL
	case trimmedAlt != "":
		return trimmedAlt
	default:
		return trimmedURL
	}
}

func stripMarkdown(value string) string {
	replacer := strings.NewReplacer(
		"*", "",
		"_", "",
		"~", "",
		"`", "",
		">", "",
		"[", "",
		"]", "",
		"(", "",
		")", "",
		"!", "",
	)
	lines := strings.Split(strings.TrimSpace(value), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#")
		line = strings.TrimSpace(line)
		lines[i] = replacer.Replace(line)
	}
	return strings.Join(strings.Fields(strings.Join(lines, "\n")), " ")
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}
