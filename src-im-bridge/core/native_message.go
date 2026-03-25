package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

type FeishuCardMode string

const (
	FeishuCardModeJSON     FeishuCardMode = "json"
	FeishuCardModeTemplate FeishuCardMode = "template"
)

// NativeMessage is a typed provider-native payload wrapper for richer message
// surfaces that should not be forced into the shared structured-message model.
type NativeMessage struct {
	Platform   string             `json:"platform,omitempty"`
	FeishuCard *FeishuCardPayload `json:"feishuCard,omitempty"`
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

func (m *NativeMessage) NormalizedPlatform() string {
	if m == nil {
		return ""
	}
	if normalized := NormalizePlatformName(m.Platform); normalized != "" {
		return normalized
	}
	if m.FeishuCard != nil {
		return "feishu"
	}
	return ""
}

func (m *NativeMessage) Validate() error {
	if m == nil {
		return fmt.Errorf("native message is required")
	}
	switch m.NormalizedPlatform() {
	case "feishu":
		if m.FeishuCard == nil {
			return fmt.Errorf("feishu native message requires feishuCard payload")
		}
		return m.FeishuCard.Validate()
	case "":
		return fmt.Errorf("native message platform is required")
	default:
		return fmt.Errorf("unsupported native message platform %q", m.NormalizedPlatform())
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
