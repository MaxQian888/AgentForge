package feishu

import (
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderFeishuNativePayload_OmitsBlankTemplateOptionals(t *testing.T) {
	payload, err := renderFeishuNativePayload(&core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:                core.FeishuCardModeTemplate,
			TemplateID:          "ctp_omit",
			TemplateVersionName: "   ",
			TemplateVariable:    map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("renderFeishuNativePayload error: %v", err)
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("payload data = %#v", payload["data"])
	}
	if data["template_id"] != "ctp_omit" {
		t.Fatalf("payload data = %+v", data)
	}
	if _, exists := data["template_version_name"]; exists {
		t.Fatalf("payload data unexpectedly includes template_version_name: %+v", data)
	}
	if _, exists := data["template_variable"]; exists {
		t.Fatalf("payload data unexpectedly includes template_variable: %+v", data)
	}
}

func TestRenderFeishuNativeContent_ReportsMarshalErrors(t *testing.T) {
	_, err := renderFeishuNativeContent(&core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:       core.FeishuCardModeTemplate,
			TemplateID: "ctp_bad",
			TemplateVariable: map[string]any{
				"channel": make(chan int),
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderFeishuNativePayload_RejectsInvalidMessages(t *testing.T) {
	testCases := []struct {
		name    string
		message *core.NativeMessage
		wantErr string
	}{
		{
			name:    "nil message",
			message: nil,
			wantErr: "native message is required",
		},
		{
			name: "missing payload",
			message: &core.NativeMessage{
				Platform: "feishu",
			},
			wantErr: "unsupported native message platform",
		},
		{
			name: "platform mismatch",
			message: &core.NativeMessage{
				Platform: "slack",
				FeishuCard: &core.FeishuCardPayload{
					Mode:       core.FeishuCardModeTemplate,
					TemplateID: "ctp_1",
				},
			},
			wantErr: "does not match payload",
		},
		{
			name: "invalid json payload",
			message: &core.NativeMessage{
				Platform: "feishu",
				FeishuCard: &core.FeishuCardPayload{
					Mode: core.FeishuCardModeJSON,
					JSON: []byte(`{"header":`),
				},
			},
			wantErr: "decode feishu json card payload",
		},
		{
			name: "missing template id",
			message: &core.NativeMessage{
				Platform: "feishu",
				FeishuCard: &core.FeishuCardPayload{
					Mode: core.FeishuCardModeTemplate,
				},
			},
			wantErr: "requires templateId",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := renderFeishuNativePayload(tc.message)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
