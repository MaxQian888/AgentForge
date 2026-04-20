package feishu

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterCardRenderer("feishu", renderProviderNeutralCard)
}

// renderProviderNeutralCard converts a spec §8 ProviderNeutralCard into a
// Feishu interactive card JSON payload (msg_type=interactive). The card
// header colour is derived from the card's status enum.
func renderProviderNeutralCard(card core.ProviderNeutralCard) (core.RenderedPayload, error) {
	elements := make([]map[string]any, 0, len(card.Fields)+len(card.Actions)+2)

	if s := strings.TrimSpace(card.Summary); s != "" {
		elements = append(elements, map[string]any{
			"tag":  "div",
			"text": map[string]any{"tag": "lark_md", "content": s},
		})
	}

	for _, f := range card.Fields {
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**%s**\n%s", f.Label, f.Value),
			},
		})
	}

	if footer := strings.TrimSpace(card.Footer); footer != "" {
		elements = append(elements, map[string]any{
			"tag":      "note",
			"elements": []map[string]any{{"tag": "plain_text", "content": footer}},
		})
	}

	if len(card.Actions) > 0 {
		actions := make([]map[string]any, 0, len(card.Actions))
		for _, a := range card.Actions {
			btn := map[string]any{
				"tag":  "button",
				"text": map[string]any{"tag": "plain_text", "content": a.Label},
				"type": normalizeFeishuButtonStyle(string(a.Style)),
			}
			switch a.Type {
			case core.CardActionTypeURL:
				btn["url"] = a.URL
			case core.CardActionTypeCallback:
				value := map[string]any{
					"action":            a.ID,
					"correlation_token": a.CorrelationToken,
				}
				if len(a.Payload) > 0 {
					value["payload"] = a.Payload
				}
				btn["value"] = value
			}
			actions = append(actions, btn)
		}
		elements = append(elements, map[string]any{"tag": "action", "actions": actions})
	}

	payload := map[string]any{
		"config":   map[string]any{"wide_screen_mode": true},
		"header":   feishuHeaderForStatus(card.Title, card.Status),
		"elements": elements,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return core.RenderedPayload{}, err
	}
	return core.RenderedPayload{ContentType: "interactive", Body: string(body)}, nil
}

func feishuHeaderForStatus(title string, status core.CardStatus) map[string]any {
	h := map[string]any{
		"title": map[string]any{"tag": "plain_text", "content": title},
	}
	switch status {
	case core.CardStatusSuccess:
		h["template"] = "green"
	case core.CardStatusFailed:
		h["template"] = "red"
	case core.CardStatusRunning:
		h["template"] = "blue"
	case core.CardStatusPending:
		h["template"] = "grey"
	case core.CardStatusInfo:
		h["template"] = "blue"
	}
	return h
}

func normalizeFeishuButtonStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "primary", "danger", "default":
		return strings.ToLower(strings.TrimSpace(style))
	default:
		return "default"
	}
}
