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

func TestMergeProjectSettingsPersistsBudgetGovernance(t *testing.T) {
	raw := `{"coding_agent":{"runtime":"claude_code"}}`
	next := &ProjectSettingsPatch{
		BudgetGovernance: &BudgetGovernance{
			MaxTaskBudgetUsd:      10.5,
			MaxDailySpendUsd:      100,
			AlertThresholdPercent: 80,
			AutoStopOnExceed:      true,
		},
	}
	merged, err := MergeProjectSettings(raw, next)
	if err != nil {
		t.Fatalf("MergeProjectSettings() error = %v", err)
	}
	settings := ParseProjectStoredSettings(merged)
	if settings.BudgetGovernance.MaxTaskBudgetUsd != 10.5 {
		t.Fatalf("MaxTaskBudgetUsd = %v, want 10.5", settings.BudgetGovernance.MaxTaskBudgetUsd)
	}
	if settings.BudgetGovernance.AlertThresholdPercent != 80 {
		t.Fatalf("AlertThresholdPercent = %v, want 80", settings.BudgetGovernance.AlertThresholdPercent)
	}
	if !settings.BudgetGovernance.AutoStopOnExceed {
		t.Fatal("AutoStopOnExceed = false, want true")
	}
	// Coding agent should be preserved
	if settings.CodingAgent.Runtime != "claude_code" {
		t.Fatalf("CodingAgent.Runtime = %q, want claude_code", settings.CodingAgent.Runtime)
	}
}

func TestMergeProjectSettingsPersistsWebhookConfig(t *testing.T) {
	raw := `{"coding_agent":{"runtime":"claude_code"}}`
	next := &ProjectSettingsPatch{
		Webhook: &WebhookConfig{
			URL:    "https://example.com/hook",
			Secret: "s3cr3t",
			Events: []string{"push", "pr_merged"},
			Active: true,
		},
	}
	merged, err := MergeProjectSettings(raw, next)
	if err != nil {
		t.Fatalf("MergeProjectSettings() error = %v", err)
	}
	settings := ParseProjectStoredSettings(merged)
	if settings.Webhook.URL != "https://example.com/hook" {
		t.Fatalf("Webhook.URL = %q, want https://example.com/hook", settings.Webhook.URL)
	}
	if !settings.Webhook.Active {
		t.Fatal("Webhook.Active = false, want true")
	}
	if len(settings.Webhook.Events) != 2 {
		t.Fatalf("Webhook.Events = %v, want 2 items", settings.Webhook.Events)
	}
}

func TestReviewPolicyIncludesAutoTriggerOnPR(t *testing.T) {
	raw := `{}`
	next := &ProjectSettingsPatch{
		ReviewPolicy: &ReviewPolicy{
			RequiredLayers:          []string{"layer1"},
			RequireManualApproval:   true,
			MinRiskLevelForBlock:    "high",
			AutoTriggerOnPR:         true,
			EnabledPluginDimensions: []string{"security", "style"},
		},
	}
	merged, err := MergeProjectSettings(raw, next)
	if err != nil {
		t.Fatalf("MergeProjectSettings() error = %v", err)
	}
	settings := ParseProjectStoredSettings(merged)
	if !settings.ReviewPolicy.AutoTriggerOnPR {
		t.Fatal("AutoTriggerOnPR = false, want true")
	}
	if len(settings.ReviewPolicy.EnabledPluginDimensions) != 2 {
		t.Fatalf("EnabledPluginDimensions = %v, want 2 items", settings.ReviewPolicy.EnabledPluginDimensions)
	}
}

func TestParseBudgetGovernanceCamelCaseFallback(t *testing.T) {
	raw := `{"budgetGovernance":{"maxTaskBudgetUsd":5,"maxDailySpendUsd":50,"alertThresholdPercent":90,"autoStopOnExceed":false}}`
	settings := ParseProjectStoredSettings(raw)
	if settings.BudgetGovernance.MaxTaskBudgetUsd != 5 {
		t.Fatalf("MaxTaskBudgetUsd = %v, want 5", settings.BudgetGovernance.MaxTaskBudgetUsd)
	}
	if settings.BudgetGovernance.AlertThresholdPercent != 90 {
		t.Fatalf("AlertThresholdPercent = %v, want 90", settings.BudgetGovernance.AlertThresholdPercent)
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

func TestProjectSettingsDTORedactsWebhookSecret(t *testing.T) {
	project := &Project{
		Settings: `{"webhook":{"url":"https://example.com/hook","secret":"super-secret","events":["push"],"active":true}}`,
	}

	dto := project.SettingsDTO()
	if dto.Webhook.URL != "https://example.com/hook" {
		t.Fatalf("Webhook.URL = %q, want https://example.com/hook", dto.Webhook.URL)
	}
	if dto.Webhook.Secret != "" {
		t.Fatalf("Webhook.Secret = %q, want redacted empty value", dto.Webhook.Secret)
	}
	if !dto.Webhook.Active {
		t.Fatal("Webhook.Active = false, want true")
	}
	if len(dto.Webhook.Events) != 1 || dto.Webhook.Events[0] != "push" {
		t.Fatalf("Webhook.Events = %#v, want [push]", dto.Webhook.Events)
	}
}
