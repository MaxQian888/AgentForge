package model

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const DefaultCodingAgentRuntime = "claude_code"

type CodingAgentSelection struct {
	Runtime  string `json:"runtime,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type BudgetGovernance struct {
	MaxTaskBudgetUsd      float64 `json:"maxTaskBudgetUsd"`
	MaxDailySpendUsd      float64 `json:"maxDailySpendUsd"`
	AlertThresholdPercent float64 `json:"alertThresholdPercent"`
	AutoStopOnExceed      bool    `json:"autoStopOnExceed"`
}

type WebhookConfig struct {
	URL    string   `json:"url,omitempty"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events"`
	Active bool     `json:"active"`
}

type ReviewPolicy struct {
	RequiredLayers          []string `json:"requiredLayers"`
	RequireManualApproval   bool     `json:"requireManualApproval"`
	MinRiskLevelForBlock    string   `json:"minRiskLevelForBlock,omitempty" validate:"omitempty,oneof=critical high medium low"`
	AutoTriggerOnPR         bool     `json:"autoTriggerOnPR"`
	EnabledPluginDimensions []string `json:"enabledPluginDimensions"`
}

func DefaultReviewPolicy() ReviewPolicy {
	return ReviewPolicy{
		RequiredLayers:          []string{},
		RequireManualApproval:   false,
		MinRiskLevelForBlock:    "",
		AutoTriggerOnPR:         false,
		EnabledPluginDimensions: []string{},
	}
}

type ProjectStoredSettings struct {
	CodingAgent      CodingAgentSelection `json:"coding_agent,omitempty"`
	ReviewPolicy     ReviewPolicy         `json:"review_policy,omitempty"`
	BudgetGovernance BudgetGovernance     `json:"budget_governance,omitempty"`
	Webhook          WebhookConfig        `json:"webhook,omitempty"`
}

type ProjectSettingsDTO struct {
	CodingAgent      CodingAgentSelection `json:"codingAgent"`
	ReviewPolicy     ReviewPolicy         `json:"reviewPolicy"`
	BudgetGovernance BudgetGovernance     `json:"budgetGovernance"`
	Webhook          WebhookConfig        `json:"webhook"`
}

type ProjectSettingsPatch struct {
	CodingAgent      *CodingAgentSelection `json:"codingAgent,omitempty"`
	ReviewPolicy     *ReviewPolicy         `json:"reviewPolicy,omitempty"`
	BudgetGovernance *BudgetGovernance     `json:"budgetGovernance,omitempty"`
	Webhook          *WebhookConfig        `json:"webhook,omitempty"`
}

type CodingAgentDiagnosticDTO struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

type CodingAgentRuntimeOptionDTO struct {
	Runtime             string                     `json:"runtime"`
	Label               string                     `json:"label"`
	DefaultProvider     string                     `json:"defaultProvider"`
	CompatibleProviders []string                   `json:"compatibleProviders"`
	DefaultModel        string                     `json:"defaultModel"`
	Available           bool                       `json:"available"`
	Diagnostics         []CodingAgentDiagnosticDTO `json:"diagnostics"`
}

type CodingAgentCatalogDTO struct {
	DefaultRuntime   string                        `json:"defaultRuntime"`
	DefaultSelection CodingAgentSelection          `json:"defaultSelection"`
	Runtimes         []CodingAgentRuntimeOptionDTO `json:"runtimes"`
}

type Project struct {
	ID            uuid.UUID `db:"id"`
	Name          string    `db:"name"`
	Slug          string    `db:"slug"`
	Description   string    `db:"description"`
	RepoURL       string    `db:"repo_url"`
	DefaultBranch string    `db:"default_branch"`
	Settings      string    `db:"settings"` // JSON string
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type ProjectDTO struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	Slug               string                 `json:"slug"`
	Description        string                 `json:"description"`
	RepoURL            string                 `json:"repoUrl"`
	DefaultBranch      string                 `json:"defaultBranch"`
	Settings           ProjectSettingsDTO     `json:"settings"`
	CodingAgentCatalog *CodingAgentCatalogDTO `json:"codingAgentCatalog,omitempty"`
	CreatedAt          time.Time              `json:"createdAt"`
}

type CreateProjectRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Slug        string `json:"slug" validate:"required,min=1,max=50"`
	Description string `json:"description"`
	RepoURL     string `json:"repoUrl"`
}

type UpdateProjectRequest struct {
	Name          *string               `json:"name"`
	Description   *string               `json:"description"`
	RepoURL       *string               `json:"repoUrl"`
	DefaultBranch *string               `json:"defaultBranch"`
	Settings      *ProjectSettingsPatch `json:"settings"`
}

func (p *Project) ToDTO() ProjectDTO {
	return ProjectDTO{
		ID:            p.ID.String(),
		Name:          p.Name,
		Slug:          p.Slug,
		Description:   p.Description,
		RepoURL:       p.RepoURL,
		DefaultBranch: p.DefaultBranch,
		Settings:      p.SettingsDTO(),
		CreatedAt:     p.CreatedAt,
	}
}

func (p *Project) ToDTOWithCatalog(catalog *CodingAgentCatalogDTO) ProjectDTO {
	dto := p.ToDTO()
	dto.CodingAgentCatalog = catalog
	return dto
}

func (p *Project) SettingsDTO() ProjectSettingsDTO {
	settings := p.StoredSettings()
	webhook := settings.Webhook
	webhook.Secret = ""
	return ProjectSettingsDTO{
		CodingAgent:      settings.CodingAgent,
		ReviewPolicy:     settings.ReviewPolicy,
		BudgetGovernance: settings.BudgetGovernance,
		Webhook:          webhook,
	}
}

func (p *Project) StoredSettings() ProjectStoredSettings {
	return ParseProjectStoredSettings(p.Settings)
}

func ParseProjectStoredSettings(raw string) ProjectStoredSettings {
	settingsMap := parseSettingsMap(raw)
	return ProjectStoredSettings{
		CodingAgent:      parseCodingAgentSelection(firstSettingsValue(settingsMap, "coding_agent", "codingAgent")),
		ReviewPolicy:     parseReviewPolicy(firstSettingsValue(settingsMap, "review_policy", "reviewPolicy")),
		BudgetGovernance: parseBudgetGovernance(firstSettingsValue(settingsMap, "budget_governance", "budgetGovernance")),
		Webhook:          parseWebhookConfig(firstSettingsValue(settingsMap, "webhook")),
	}
}

func MergeProjectSettings(raw string, next *ProjectSettingsPatch) (string, error) {
	current := parseSettingsMap(raw)
	if next != nil {
		if next.CodingAgent != nil {
			delete(current, "codingAgent")
			current["coding_agent"] = map[string]any{
				"runtime":  strings.TrimSpace(next.CodingAgent.Runtime),
				"provider": strings.TrimSpace(next.CodingAgent.Provider),
				"model":    strings.TrimSpace(next.CodingAgent.Model),
			}
		}
		if next.ReviewPolicy != nil {
			delete(current, "reviewPolicy")
			current["review_policy"] = map[string]any{
				"requiredLayers":          normalizeStringSliceFromStrings(next.ReviewPolicy.RequiredLayers),
				"requireManualApproval":   next.ReviewPolicy.RequireManualApproval,
				"minRiskLevelForBlock":    strings.TrimSpace(next.ReviewPolicy.MinRiskLevelForBlock),
				"autoTriggerOnPR":         next.ReviewPolicy.AutoTriggerOnPR,
				"enabledPluginDimensions": normalizeStringSliceFromStrings(next.ReviewPolicy.EnabledPluginDimensions),
			}
		}
		if next.BudgetGovernance != nil {
			delete(current, "budgetGovernance")
			current["budget_governance"] = map[string]any{
				"maxTaskBudgetUsd":      next.BudgetGovernance.MaxTaskBudgetUsd,
				"maxDailySpendUsd":      next.BudgetGovernance.MaxDailySpendUsd,
				"alertThresholdPercent": next.BudgetGovernance.AlertThresholdPercent,
				"autoStopOnExceed":      next.BudgetGovernance.AutoStopOnExceed,
			}
		}
		if next.Webhook != nil {
			current["webhook"] = map[string]any{
				"url":    strings.TrimSpace(next.Webhook.URL),
				"secret": next.Webhook.Secret,
				"events": normalizeStringSliceFromStrings(next.Webhook.Events),
				"active": next.Webhook.Active,
			}
		}
	}

	payload, err := json.Marshal(current)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func parseSettingsMap(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}

	settings := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &settings); err != nil {
		return map[string]any{}
	}
	return settings
}

func firstSettingsValue(settings map[string]any, keys ...string) any {
	if len(settings) == 0 {
		return nil
	}
	for _, key := range keys {
		if value, ok := settings[key]; ok {
			return value
		}
	}
	return nil
}

func parseCodingAgentSelection(raw any) CodingAgentSelection {
	record, ok := raw.(map[string]any)
	if !ok {
		return CodingAgentSelection{}
	}

	selection := CodingAgentSelection{}
	if runtime, ok := record["runtime"].(string); ok {
		selection.Runtime = strings.TrimSpace(runtime)
	}
	if provider, ok := record["provider"].(string); ok {
		selection.Provider = strings.TrimSpace(provider)
	}
	if modelName, ok := record["model"].(string); ok {
		selection.Model = strings.TrimSpace(modelName)
	}
	return selection
}

func parseReviewPolicy(raw any) ReviewPolicy {
	policy := DefaultReviewPolicy()
	record, ok := raw.(map[string]any)
	if !ok {
		return policy
	}

	policy.RequiredLayers = normalizeStringSlice(anySlice(record["requiredLayers"]))
	policy.RequireManualApproval = parseBool(record["requireManualApproval"])
	if threshold, ok := record["minRiskLevelForBlock"].(string); ok {
		policy.MinRiskLevelForBlock = strings.TrimSpace(threshold)
	}
	policy.AutoTriggerOnPR = parseBool(record["autoTriggerOnPR"])
	policy.EnabledPluginDimensions = normalizeStringSlice(anySlice(record["enabledPluginDimensions"]))
	return policy
}

func parseBudgetGovernance(raw any) BudgetGovernance {
	record, ok := raw.(map[string]any)
	if !ok {
		return BudgetGovernance{}
	}
	return BudgetGovernance{
		MaxTaskBudgetUsd:      parseFloat64(record["maxTaskBudgetUsd"]),
		MaxDailySpendUsd:      parseFloat64(record["maxDailySpendUsd"]),
		AlertThresholdPercent: parseFloat64(record["alertThresholdPercent"]),
		AutoStopOnExceed:      parseBool(record["autoStopOnExceed"]),
	}
}

func parseWebhookConfig(raw any) WebhookConfig {
	record, ok := raw.(map[string]any)
	if !ok {
		return WebhookConfig{Events: []string{}}
	}
	cfg := WebhookConfig{
		Active: parseBool(record["active"]),
		Events: normalizeStringSlice(anySlice(record["events"])),
	}
	if url, ok := record["url"].(string); ok {
		cfg.URL = strings.TrimSpace(url)
	}
	if secret, ok := record["secret"].(string); ok {
		cfg.Secret = secret
	}
	return cfg
}

func parseFloat64(raw any) float64 {
	switch v := raw.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func parseBool(raw any) bool {
	value, _ := raw.(bool)
	return value
}

func anySlice(raw any) []any {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	return list
}

func normalizeStringSlice(values []any) []string {
	if len(values) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizeStringSliceFromStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
