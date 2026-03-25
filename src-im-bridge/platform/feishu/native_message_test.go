package feishu

import (
	"encoding/json"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderFeishuNativeContent_RendersJSONAndTemplateCards(t *testing.T) {
	jsonContent, err := renderFeishuNativeContent(&core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode: core.FeishuCardModeJSON,
			JSON: json.RawMessage(`{"header":{"title":{"tag":"plain_text","content":"Hello"}}}`),
		},
	})
	if err != nil {
		t.Fatalf("renderFeishuNativeContent(json) error: %v", err)
	}

	var jsonPayload map[string]any
	if err := json.Unmarshal([]byte(jsonContent), &jsonPayload); err != nil {
		t.Fatalf("decode jsonContent: %v", err)
	}
	if _, ok := jsonPayload["header"]; !ok {
		t.Fatalf("json payload = %+v, want raw card object", jsonPayload)
	}

	templateContent, err := renderFeishuNativeContent(&core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:                core.FeishuCardModeTemplate,
			TemplateID:          "ctp_123",
			TemplateVersionName: "1.0.0",
			TemplateVariable: map[string]any{
				"title": "AgentForge",
			},
		},
	})
	if err != nil {
		t.Fatalf("renderFeishuNativeContent(template) error: %v", err)
	}

	var templatePayload map[string]any
	if err := json.Unmarshal([]byte(templateContent), &templatePayload); err != nil {
		t.Fatalf("decode templateContent: %v", err)
	}
	if templatePayload["type"] != "template" {
		t.Fatalf("template payload = %+v, want type=template", templatePayload)
	}
	data, ok := templatePayload["data"].(map[string]any)
	if !ok {
		t.Fatalf("template payload data = %#v", templatePayload["data"])
	}
	if data["template_id"] != "ctp_123" {
		t.Fatalf("template data = %+v", data)
	}
	if data["template_version_name"] != "1.0.0" {
		t.Fatalf("template data = %+v", data)
	}
}

func TestRenderFeishuNativePayload_RendersCardUpdatePayload(t *testing.T) {
	payload, err := renderFeishuNativePayload(&core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:       core.FeishuCardModeTemplate,
			TemplateID: "ctp_456",
			TemplateVariable: map[string]any{
				"status": "done",
			},
		},
	})
	if err != nil {
		t.Fatalf("renderFeishuNativePayload error: %v", err)
	}

	if payload["type"] != "template" {
		t.Fatalf("payload = %+v, want type=template", payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("payload data = %#v", payload["data"])
	}
	if data["template_id"] != "ctp_456" {
		t.Fatalf("payload data = %+v", data)
	}
	if _, ok := data["template_variable"].(map[string]any); !ok {
		t.Fatalf("payload data = %+v, want template_variable map", data)
	}
}
