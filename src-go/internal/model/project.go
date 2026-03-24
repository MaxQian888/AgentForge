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

type ProjectStoredSettings struct {
	CodingAgent CodingAgentSelection `json:"coding_agent,omitempty"`
}

type ProjectSettingsDTO struct {
	CodingAgent CodingAgentSelection `json:"codingAgent"`
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
	Name          *string             `json:"name"`
	Description   *string             `json:"description"`
	RepoURL       *string             `json:"repoUrl"`
	DefaultBranch *string             `json:"defaultBranch"`
	Settings      *ProjectSettingsDTO `json:"settings"`
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
	return ProjectSettingsDTO{
		CodingAgent: p.StoredSettings().CodingAgent,
	}
}

func (p *Project) StoredSettings() ProjectStoredSettings {
	return ParseProjectStoredSettings(p.Settings)
}

func ParseProjectStoredSettings(raw string) ProjectStoredSettings {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ProjectStoredSettings{}
	}

	var settings ProjectStoredSettings
	if err := json.Unmarshal([]byte(trimmed), &settings); err != nil {
		return ProjectStoredSettings{}
	}
	return settings
}

func MergeProjectSettings(raw string, next *ProjectSettingsDTO) (string, error) {
	current := ParseProjectStoredSettings(raw)
	if next != nil {
		current.CodingAgent = next.CodingAgent
	}

	payload, err := json.Marshal(current)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
