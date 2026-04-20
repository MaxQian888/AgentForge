// Package model — vcs_integration.go declares the persisted form of a
// per-(project, repo) VCS link. Plaintext PAT and webhook secret never
// live in this row; only the secrets.name refs do (resolved at outbound
// time via the 1B secrets store).
package model

import (
	"time"

	"github.com/google/uuid"
)

// VCSIntegration is one (project, repo) link to a source-control host.
type VCSIntegration struct {
	ID               uuid.UUID  `json:"id"`
	ProjectID        uuid.UUID  `json:"projectId"`
	Provider         string     `json:"provider"`
	Host             string     `json:"host"`
	Owner            string     `json:"owner"`
	Repo             string     `json:"repo"`
	DefaultBranch    string     `json:"defaultBranch"`
	WebhookID        *string    `json:"webhookId,omitempty"`
	WebhookSecretRef string     `json:"webhookSecretRef"`
	TokenSecretRef   string     `json:"tokenSecretRef"`
	Status           string     `json:"status"`
	ActingEmployeeID *uuid.UUID `json:"actingEmployeeId,omitempty"`
	LastSyncedAt     *time.Time `json:"lastSyncedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}
