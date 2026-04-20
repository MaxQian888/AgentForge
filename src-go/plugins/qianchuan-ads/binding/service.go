package qianchuanbinding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/adsplatform"
)

// ErrSecretMissing is returned when one of the *_secret_ref values does
// not resolve in the secrets store.
var ErrSecretMissing = errors.New("qianchuanbinding: secret_missing")

// SecretsResolver is the narrow contract Service depends on. It mirrors
// the surface of internal/secrets.Service.Resolve so service tests can
// pass an in-memory fake without bootstrapping the cipher.
type SecretsResolver interface {
	Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
}

// CreateInput is the bag passed to Service.Create.
type CreateInput struct {
	ProjectID             uuid.UUID
	AdvertiserID          string
	AwemeID               string
	DisplayName           string
	ActingEmployeeID      *uuid.UUID
	AccessTokenSecretRef  string
	RefreshTokenSecretRef string
	CreatedBy             uuid.UUID
}

// UpdateInput patches one binding's mutable fields.
type UpdateInput struct {
	DisplayName      *string
	Status           *string
	ActingEmployeeID *uuid.UUID
}

// Service composes repository + secrets + provider and exposes the
// operations the HTTP handler invokes.
type Service struct {
	repo     Repository
	secrets  SecretsResolver
	provider adsplatform.Provider
}

// NewService wires the dependencies.
func NewService(repo Repository, secrets SecretsResolver, provider adsplatform.Provider) *Service {
	return &Service{repo: repo, secrets: secrets, provider: provider}
}

// Create validates inputs, ensures both secret refs resolve, runs a
// verification FetchMetrics call, and persists the row in 'active' state.
// Any provider error short-circuits the create — we never persist a
// binding whose tokens we could not validate.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Record, error) {
	if in.AdvertiserID == "" {
		return nil, fmt.Errorf("qianchuanbinding: advertiser_id required")
	}
	access, err := s.secrets.Resolve(ctx, in.ProjectID, in.AccessTokenSecretRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSecretMissing, in.AccessTokenSecretRef)
	}
	if _, err := s.secrets.Resolve(ctx, in.ProjectID, in.RefreshTokenSecretRef); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSecretMissing, in.RefreshTokenSecretRef)
	}
	// Auth probe — we do NOT persist if the token is dead on arrival.
	if _, err := s.provider.FetchMetrics(ctx, adsplatform.BindingRef{
		AdvertiserID: in.AdvertiserID, AwemeID: in.AwemeID, AccessToken: access,
	}, adsplatform.MetricDimensions{Range: "today"}); err != nil {
		return nil, err
	}
	rec := &Record{
		ProjectID:             in.ProjectID,
		AdvertiserID:          in.AdvertiserID,
		AwemeID:               in.AwemeID,
		DisplayName:           in.DisplayName,
		Status:                StatusActive,
		ActingEmployeeID:      in.ActingEmployeeID,
		AccessTokenSecretRef:  in.AccessTokenSecretRef,
		RefreshTokenSecretRef: in.RefreshTokenSecretRef,
		CreatedBy:             in.CreatedBy,
	}
	if err := s.repo.Create(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Get fetches a binding by id.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Record, error) {
	return s.repo.Get(ctx, id)
}

// List lists bindings under a project.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]*Record, error) {
	return s.repo.ListByProject(ctx, projectID)
}

// Update patches mutable fields.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (*Record, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.DisplayName != nil {
		rec.DisplayName = *in.DisplayName
	}
	if in.Status != nil {
		rec.Status = *in.Status
	}
	if in.ActingEmployeeID != nil {
		rec.ActingEmployeeID = in.ActingEmployeeID
	}
	if err := s.repo.Update(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Delete removes a binding (cascades action_logs once Plan 3D lands).
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// Sync resolves the access token and pulls a fresh metrics snapshot,
// updating last_synced_at on success.
func (s *Service) Sync(ctx context.Context, id uuid.UUID) error {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	access, err := s.secrets.Resolve(ctx, rec.ProjectID, rec.AccessTokenSecretRef)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSecretMissing, rec.AccessTokenSecretRef)
	}
	if _, err := s.provider.FetchMetrics(ctx, adsplatform.BindingRef{
		AdvertiserID: rec.AdvertiserID, AwemeID: rec.AwemeID, AccessToken: access,
	}, adsplatform.MetricDimensions{Range: "today"}); err != nil {
		return err
	}
	return s.repo.TouchSync(ctx, id, time.Now().UTC())
}

// Test runs a sample FetchMetrics and returns the result for FE health-check.
func (s *Service) Test(ctx context.Context, id uuid.UUID) (*adsplatform.MetricSnapshot, error) {
	rec, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	access, err := s.secrets.Resolve(ctx, rec.ProjectID, rec.AccessTokenSecretRef)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSecretMissing, rec.AccessTokenSecretRef)
	}
	return s.provider.FetchMetrics(ctx, adsplatform.BindingRef{
		AdvertiserID: rec.AdvertiserID, AwemeID: rec.AwemeID, AccessToken: access,
	}, adsplatform.MetricDimensions{Range: "today"})
}
