package feishu

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func renderFeishuNativeContent(message *core.NativeMessage) (string, error) {
	payload, err := renderFeishuNativePayload(message)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func renderFeishuNativePayload(message *core.NativeMessage) (map[string]any, error) {
	if err := message.Validate(); err != nil {
		return nil, err
	}
	if message.NormalizedPlatform() != "feishu" || message.FeishuCard == nil {
		return nil, fmt.Errorf("native message is not a feishu card payload")
	}

	card := message.FeishuCard
	switch core.FeishuCardMode(strings.ToLower(strings.TrimSpace(string(card.Mode)))) {
	case core.FeishuCardModeJSON:
		var decoded map[string]any
		if err := json.Unmarshal(card.JSON, &decoded); err != nil {
			return nil, fmt.Errorf("decode feishu native card payload: %w", err)
		}
		return decoded, nil
	case core.FeishuCardModeTemplate:
		data := map[string]any{
			"template_id": card.TemplateID,
		}
		if version := strings.TrimSpace(card.TemplateVersionName); version != "" {
			data["template_version_name"] = version
		}
		if len(card.TemplateVariable) > 0 {
			data["template_variable"] = card.TemplateVariable
		}
		return map[string]any{
			"type": "template",
			"data": data,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported feishu card mode %q", card.Mode)
	}
}
