package dingtalk

import (
	"encoding/json"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func init() {
	core.RegisterCardRenderer("dingtalk", renderProviderNeutralCard)
}

// renderProviderNeutralCard converts a spec §8 ProviderNeutralCard into a
// DingTalk ActionCard payload. ActionCard supports URL buttons only; callback
// actions degrade to a markdown line so the user still sees the labels.
func renderProviderNeutralCard(card core.ProviderNeutralCard) (core.RenderedPayload, error) {
	var md strings.Builder
	if card.Status != "" {
		md.WriteString("**[")
		md.WriteString(strings.ToUpper(string(card.Status)))
		md.WriteString("]** ")
	}
	if card.Summary != "" {
		md.WriteString(card.Summary)
		md.WriteString("\n\n")
	}
	for _, f := range card.Fields {
		md.WriteString("**" + f.Label + "**: " + f.Value + "  \n")
	}
	if card.Footer != "" {
		md.WriteString("\n_" + card.Footer + "_")
	}

	buttons := make([]map[string]string, 0, len(card.Actions))
	for _, a := range card.Actions {
		if a.Type == core.CardActionTypeURL && a.URL != "" {
			buttons = append(buttons, map[string]string{"title": a.Label, "actionURL": a.URL})
		} else if a.Type == core.CardActionTypeCallback {
			md.WriteString("\n• [" + a.Label + "] (使用 AgentForge 客户端响应)")
		}
	}
	payload := map[string]any{
		"card_type": "ActionCard",
		"title":     card.Title,
		"markdown":  md.String(),
		"buttons":   buttons,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return core.RenderedPayload{}, err
	}
	return core.RenderedPayload{ContentType: "actioncard", Body: string(body)}, nil
}
