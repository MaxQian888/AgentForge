package model

import (
	"time"

	"github.com/google/uuid"
)

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
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	RepoURL       string    `json:"repoUrl"`
	DefaultBranch string    `json:"defaultBranch"`
	CreatedAt     time.Time `json:"createdAt"`
}

type CreateProjectRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Slug        string `json:"slug" validate:"required,min=1,max=50"`
	Description string `json:"description"`
	RepoURL     string `json:"repoUrl"`
}

type UpdateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	RepoURL     *string `json:"repoUrl"`
}

func (p *Project) ToDTO() ProjectDTO {
	return ProjectDTO{
		ID:            p.ID.String(),
		Name:          p.Name,
		Slug:          p.Slug,
		Description:   p.Description,
		RepoURL:       p.RepoURL,
		DefaultBranch: p.DefaultBranch,
		CreatedAt:     p.CreatedAt,
	}
}
