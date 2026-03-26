package model

import (
	"encoding/json"
	"testing"
)

func TestMergeProjectSettingsPreservesReviewPolicyWhenUpdatingCodingAgentOnly(t *testing.T) {
	raw := `{"coding_agent":{"runtime":"claude_code","provider":"anthropic","model":"claude-sonnet-4-6"},"review_policy":{"requiredLayers":["layer1","layer2"],"requireManualApproval":true,"minRiskLevelForBlock":"high"},"webhook":{"active":true}}`

	next := &ProjectSettingsPatch{
		CodingAgent: &CodingAgentSelection{
			Runtime:  "codex",
			Provider: "openai",
			Model:    "gpt-5-codex",
		},
	}

	merged, err := MergeProjectSettings(raw, next)
	if err != nil {
		t.Fatalf("MergeProjectSettings() error = %v", err)
	}

	settings := ParseProjectStoredSettings(merged)
	if settings.CodingAgent.Runtime != "codex" {
		t.Fatalf("coding agent runtime = %q, want codex", settings.CodingAgent.Runtime)
	}
	if !settings.ReviewPolicy.RequireManualApproval {
		t.Fatalf("review policy manual flag = %v, want true", settings.ReviewPolicy.RequireManualApproval)
	}
	if settings.ReviewPolicy.MinRiskLevelForBlock != "high" {
		t.Fatalf("min risk threshold = %q, want high", settings.ReviewPolicy.MinRiskLevelForBlock)
	}
	if len(settings.ReviewPolicy.RequiredLayers) != 2 {
		t.Fatalf("required layers = %#v, want 2 values", settings.ReviewPolicy.RequiredLayers)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(merged), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := decoded["webhook"]; !ok {
		t.Fatalf("merged settings should preserve unrelated sections: %#v", decoded)
	}
}

func TestParseProjectStoredSettingsAcceptsCamelCaseFallback(t *testing.T) {
	raw := `{"codingAgent":{"runtime":"codex","provider":"openai","model":"gpt-5-codex"},"reviewPolicy":{"requiredLayers":["layer2"],"requireManualApproval":false,"minRiskLevelForBlock":"critical"}}`
	settings := ParseProjectStoredSettings(raw)

	if settings.CodingAgent.Runtime != "codex" {
		t.Fatalf("coding agent runtime = %q, want codex", settings.CodingAgent.Runtime)
	}
	if settings.ReviewPolicy.MinRiskLevelForBlock != "critical" {
		t.Fatalf("min risk threshold = %q, want critical", settings.ReviewPolicy.MinRiskLevelForBlock)
	}
	if len(settings.ReviewPolicy.RequiredLayers) != 1 || settings.ReviewPolicy.RequiredLayers[0] != "layer2" {
		t.Fatalf("required layers = %#v, want [layer2]", settings.ReviewPolicy.RequiredLayers)
	}
}

func TestProjectSettingsDTOReturnsDefaultReviewPolicyWhenMissing(t *testing.T) {
	project := &Project{
		Settings: `{"coding_agent":{"runtime":"claude_code"}}`,
	}

	dto := project.SettingsDTO()
	if dto.ReviewPolicy.RequireManualApproval {
		t.Fatalf("RequireManualApproval = %v, want false", dto.ReviewPolicy.RequireManualApproval)
	}
	if dto.ReviewPolicy.MinRiskLevelForBlock != "" {
		t.Fatalf("MinRiskLevelForBlock = %q, want empty", dto.ReviewPolicy.MinRiskLevelForBlock)
	}
	if len(dto.ReviewPolicy.RequiredLayers) != 0 {
		t.Fatalf("RequiredLayers = %#v, want empty", dto.ReviewPolicy.RequiredLayers)
	}
}
