package slack

import (
	"encoding/json"

	"github.com/agentforge/im-bridge/core"
	gs "github.com/slack-go/slack"
)

func init() {
	core.RegisterCardRenderer("slack", renderProviderNeutralCard)
}

// renderProviderNeutralCard converts a spec §8 ProviderNeutralCard into a
// Slack blocks payload (chat.postMessage compatible). The wire shape is
// {"blocks":[...]}; ContentType is "blocks" so the platform send path can
// route it to the blocks-aware sender.
func renderProviderNeutralCard(card core.ProviderNeutralCard) (core.RenderedPayload, error) {
	blocks := []gs.Block{
		gs.NewHeaderBlock(gs.NewTextBlockObject(gs.PlainTextType, card.Title, false, false)),
	}
	if card.Summary != "" {
		blocks = append(blocks, gs.NewSectionBlock(
			gs.NewTextBlockObject(gs.MarkdownType, card.Summary, false, false), nil, nil))
	}
	if len(card.Fields) > 0 {
		fields := make([]*gs.TextBlockObject, 0, len(card.Fields))
		for _, f := range card.Fields {
			fields = append(fields, gs.NewTextBlockObject(gs.MarkdownType,
				"*"+f.Label+"*\n"+f.Value, false, false))
		}
		blocks = append(blocks, gs.NewSectionBlock(nil, fields, nil))
	}
	if len(card.Actions) > 0 {
		elems := make([]gs.BlockElement, 0, len(card.Actions))
		for _, a := range card.Actions {
			btn := gs.NewButtonBlockElement("act-"+a.ID, a.ID,
				gs.NewTextBlockObject(gs.PlainTextType, a.Label, false, false))
			switch a.Type {
			case core.CardActionTypeURL:
				btn.URL = a.URL
			case core.CardActionTypeCallback:
				valueJSON, _ := json.Marshal(map[string]any{
					"action_id":         a.ID,
					"correlation_token": a.CorrelationToken,
					"payload":           a.Payload,
				})
				btn.Value = string(valueJSON)
			}
			btn.Style = gs.Style(slackButtonStyle(string(a.Style)))
			elems = append(elems, btn)
		}
		blocks = append(blocks, gs.NewActionBlock("af-actions", elems...))
	}
	if card.Footer != "" {
		blocks = append(blocks, gs.NewContextBlock("af-footer",
			gs.NewTextBlockObject(gs.MarkdownType, card.Footer, false, false)))
	}
	body, err := json.Marshal(map[string]any{"blocks": blocks})
	if err != nil {
		return core.RenderedPayload{}, err
	}
	return core.RenderedPayload{ContentType: "blocks", Body: string(body)}, nil
}

func slackButtonStyle(s string) string {
	switch s {
	case "primary", "danger":
		return s
	}
	return ""
}
