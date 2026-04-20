package vcs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
)

// ErrSecretNotResolvable signals the (project, secret_ref) tuple did
// not resolve. The handler maps it to a 4xx so the user can fix the
// ref before persisting the integration.
var ErrSecretNotResolvable = errors.New("vcs:secret_not_resolvable")

// ErrPublicBaseURLNotConfigured is returned by Service.Create when the
// server has no AGENTFORGE_PUBLIC_BASE_URL — without it we cannot tell
// the host where to send webhook callbacks.
var ErrPublicBaseURLNotConfigured = errors.New("vcs:public_base_url_not_configured")

// Repo is the narrow persistence contract the Service consumes. The
// concrete implementation is repository.VCSIntegrationRepo.
type Repo interface {
	Create(ctx context.Context, r *model.VCSIntegration) error
	Get(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error)
	Update(ctx context.Context, r *model.VCSIntegration) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// SecretsResolver is the narrow seam the Service uses to resolve
// secret refs. Implemented by an adapter that forwards into the 1B
// secrets.Service. Plaintext is consumed inside the same call frame
// and never persisted on the Service struct.
type SecretsResolver interface {
	Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
}

// AuditRecorder mirrors the secrets-subsystem audit seam so vcs.* events
// can flow into the same project_audit_events stream. Payloads MUST NOT
// contain tokens or webhook IDs — only {provider, host, owner, repo, op}.
type AuditRecorder interface {
	Record(ctx context.Context, projectID uuid.UUID, action, resourceID, payload string, actor *uuid.UUID)
}

// CreateInput is the request payload accepted by Service.Create.
type CreateInput struct {
	ProjectID        uuid.UUID
	Provider         string
	Host             string
	Owner            string
	Repo             string
	DefaultBranch    string
	TokenSecretRef   string
	WebhookSecretRef string
	ActingEmployeeID *uuid.UUID
	Actor            *uuid.UUID
}

// PatchInput captures the fields PATCH /vcs-integrations/:id may modify.
type PatchInput struct {
	Status           *string
	TokenSecretRef   *string
	ActingEmployeeID *uuid.UUID
	Actor            *uuid.UUID
}

// Service orchestrates registry + repo + secrets + webhook lifecycle.
type Service struct {
	repo           Repo
	registry       *Registry
	secrets        SecretsResolver
	publicCallback string
	audit          AuditRecorder
}

// NewService wires the Service. callbackURL is the absolute, fully-
// qualified webhook destination handed to the host (e.g.
// https://agentforge.example/api/v1/vcs/github/webhook). audit may be
// nil for unit tests that don't care about audit emission.
func NewService(repo Repo, reg *Registry, secrets SecretsResolver, callbackURL string, audit AuditRecorder) *Service {
	return &Service{repo: repo, registry: reg, secrets: secrets, publicCallback: callbackURL, audit: audit}
}

// Create validates the configuration end-to-end then persists.
//
//  1. registry must know the provider
//  2. both secret refs must resolve in the project's secret scope
//  3. PAT must validate against the host (sentinel GetPullRequest call;
//     ErrAuthExpired short-circuits, anything else is treated as PAT-OK)
//  4. CreateWebhook on the host
//  5. persist row with returned webhook_id
//
// Failure between steps 4 and 5 triggers a best-effort DeleteWebhook so
// orphan webhooks do not pile up on the host side.
func (s *Service) Create(ctx context.Context, in CreateInput) (*model.VCSIntegration, error) {
	if s.publicCallback == "" {
		return nil, ErrPublicBaseURLNotConfigured
	}
	if in.Provider == "" || in.Host == "" || in.Owner == "" || in.Repo == "" ||
		in.TokenSecretRef == "" || in.WebhookSecretRef == "" {
		return nil, errors.New("vcs:invalid_input")
	}

	token, err := s.secrets.Resolve(ctx, in.ProjectID, in.TokenSecretRef)
	if err != nil {
		return nil, fmt.Errorf("%w: token ref %q", ErrSecretNotResolvable, in.TokenSecretRef)
	}
	whSecret, err := s.secrets.Resolve(ctx, in.ProjectID, in.WebhookSecretRef)
	if err != nil {
		return nil, fmt.Errorf("%w: webhook ref %q", ErrSecretNotResolvable, in.WebhookSecretRef)
	}

	prov, err := s.registry.Resolve(in.Provider, in.Host, token)
	if err != nil {
		return nil, err
	}

	repoRef := RepoRef{Host: in.Host, Owner: in.Owner, Repo: in.Repo}

	// Sentinel auth-validation call. We expect either a 404 (PR doesn't
	// exist) which proves auth works, OR ErrAuthExpired which we pass
	// back. Any other error is treated as PAT-OK so transient host-side
	// problems do not block onboarding.
	if _, err := prov.GetPullRequest(ctx, repoRef, 0); errors.Is(err, ErrAuthExpired) {
		return nil, ErrAuthExpired
	}

	hookID, err := prov.CreateWebhook(ctx, repoRef, s.publicCallback, whSecret, []string{"pull_request", "push"})
	if err != nil {
		return nil, fmt.Errorf("vcs: create webhook: %w", err)
	}

	rec := &model.VCSIntegration{
		ID:               uuid.New(),
		ProjectID:        in.ProjectID,
		Provider:         in.Provider,
		Host:             in.Host,
		Owner:            in.Owner,
		Repo:             in.Repo,
		DefaultBranch:    coalesce(in.DefaultBranch, "main"),
		WebhookID:        &hookID,
		WebhookSecretRef: in.WebhookSecretRef,
		TokenSecretRef:   in.TokenSecretRef,
		Status:           "active",
		ActingEmployeeID: in.ActingEmployeeID,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		// best-effort cleanup so orphan webhooks don't pile up on host
		_ = prov.DeleteWebhook(ctx, repoRef, hookID)
		return nil, err
	}
	s.emitAudit(ctx, rec, "vcs.integration.create", in.Actor)
	return rec, nil
}

// Patch updates mutable fields. If TokenSecretRef changed, we re-run
// the sentinel GetPullRequest call so a misconfigured rotation surfaces
// immediately rather than at first webhook delivery.
func (s *Service) Patch(ctx context.Context, id uuid.UUID, in PatchInput) (*model.VCSIntegration, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Status != nil {
		rec.Status = *in.Status
	}
	if in.ActingEmployeeID != nil {
		rec.ActingEmployeeID = in.ActingEmployeeID
	}
	if in.TokenSecretRef != nil {
		token, err := s.secrets.Resolve(ctx, rec.ProjectID, *in.TokenSecretRef)
		if err != nil {
			return nil, ErrSecretNotResolvable
		}
		prov, err := s.registry.Resolve(rec.Provider, rec.Host, token)
		if err != nil {
			return nil, err
		}
		if _, err := prov.GetPullRequest(ctx, RepoRef{Host: rec.Host, Owner: rec.Owner, Repo: rec.Repo}, 0); errors.Is(err, ErrAuthExpired) {
			return nil, ErrAuthExpired
		}
		rec.TokenSecretRef = *in.TokenSecretRef
	}
	if err := s.repo.Update(ctx, rec); err != nil {
		return nil, err
	}
	s.emitAudit(ctx, rec, "vcs.integration.update", in.Actor)
	return rec, nil
}

// Delete removes the host-side webhook first, then deletes the row.
// Webhook deletion is best-effort so a host-side 404 (already gone)
// does not strand the local row.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, actor *uuid.UUID) error {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if rec.WebhookID != nil && *rec.WebhookID != "" {
		token, terr := s.secrets.Resolve(ctx, rec.ProjectID, rec.TokenSecretRef)
		if terr == nil {
			if prov, perr := s.registry.Resolve(rec.Provider, rec.Host, token); perr == nil {
				_ = prov.DeleteWebhook(ctx, RepoRef{Host: rec.Host, Owner: rec.Owner, Repo: rec.Repo}, *rec.WebhookID)
			}
		}
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.emitAudit(ctx, rec, "vcs.integration.delete", actor)
	return nil
}

// List returns all integrations for the project.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error) {
	return s.repo.ListByProject(ctx, projectID)
}

// QueueSync stamps last_synced_at and returns the row. The actual
// background pull of open PRs lives in 2B; this v1 returns 202 to the
// handler so the FE can stop spinning.
func (s *Service) QueueSync(ctx context.Context, id uuid.UUID, actor *uuid.UUID) (*model.VCSIntegration, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	rec.LastSyncedAt = &now
	if err := s.repo.Update(ctx, rec); err != nil {
		return nil, err
	}
	s.emitAudit(ctx, rec, "vcs.integration.sync", actor)
	return rec, nil
}

func (s *Service) emitAudit(ctx context.Context, rec *model.VCSIntegration, action string, actor *uuid.UUID) {
	if s.audit == nil || rec == nil {
		return
	}
	// Payload contains ONLY non-sensitive identifiers. Tokens, webhook
	// secret values, and webhook IDs are deliberately omitted.
	payload := fmt.Sprintf(
		`{"provider":%q,"host":%q,"owner":%q,"repo":%q,"op":%q}`,
		rec.Provider, rec.Host, rec.Owner, rec.Repo, action,
	)
	s.audit.Record(ctx, rec.ProjectID, action, rec.ID.String(), payload, actor)
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
