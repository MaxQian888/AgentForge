package plugin_test

import (
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
)

func boolPtr(b bool) *bool { return &b }

func TestValidateConfig_NilSchemaAcceptsAnything(t *testing.T) {
	if err := plugin.ValidateConfig(nil, map[string]any{"anything": 42}); err != nil {
		t.Fatalf("nil schema must accept any config: %v", err)
	}
}

func TestValidateConfig_RejectsMissingRequiredField(t *testing.T) {
	schema := &model.PluginConfigSchema{
		Type:     "object",
		Required: []string{"apiKey"},
	}
	err := plugin.ValidateConfig(schema, map[string]any{"endpoint": "https://x"})
	if err == nil {
		t.Fatal("expected missing-required error")
	}
}

func TestValidateConfig_RejectsWrongType(t *testing.T) {
	schema := &model.PluginConfigSchema{
		Type: "object",
		Properties: map[string]*model.PluginConfigSchema{
			"timeout": {Type: "integer"},
		},
	}
	err := plugin.ValidateConfig(schema, map[string]any{"timeout": "thirty seconds"})
	if err == nil {
		t.Fatal("expected type-mismatch error for non-integer timeout")
	}
}

func TestValidateConfig_RejectsUnknownFieldWhenAdditionalFalse(t *testing.T) {
	schema := &model.PluginConfigSchema{
		Type: "object",
		Properties: map[string]*model.PluginConfigSchema{
			"apiKey": {Type: "string"},
		},
		AdditionalProperties: boolPtr(false),
	}
	err := plugin.ValidateConfig(schema, map[string]any{
		"apiKey": "sk-123",
		"secret": "oops",
	})
	if err == nil {
		t.Fatal("expected unknown-field error with additionalProperties: false")
	}
}

func TestValidateConfig_AcceptsValidObject(t *testing.T) {
	schema := &model.PluginConfigSchema{
		Type:     "object",
		Required: []string{"apiKey", "endpoint"},
		Properties: map[string]*model.PluginConfigSchema{
			"apiKey":   {Type: "string"},
			"endpoint": {Type: "string"},
			"timeout":  {Type: "integer"},
			"enabled":  {Type: "boolean"},
			"region":   {Type: "string", Enum: []any{"us", "eu", "ap"}},
		},
	}
	cfg := map[string]any{
		"apiKey":   "sk-123",
		"endpoint": "https://api.example.com",
		"timeout":  30,
		"enabled":  true,
		"region":   "eu",
	}
	if err := plugin.ValidateConfig(schema, cfg); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestValidateConfig_EnumRejectsNonMember(t *testing.T) {
	schema := &model.PluginConfigSchema{
		Type: "object",
		Properties: map[string]*model.PluginConfigSchema{
			"region": {Type: "string", Enum: []any{"us", "eu"}},
		},
	}
	if err := plugin.ValidateConfig(schema, map[string]any{"region": "mars"}); err == nil {
		t.Fatal("expected enum rejection")
	}
}
