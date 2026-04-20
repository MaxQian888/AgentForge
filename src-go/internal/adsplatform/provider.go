package adsplatform

import "context"

// Provider is the platform-neutral contract. ALL methods are scope-limited
// to safe, auditable primitives. Adding a method requires a code-review
// gate per Spec 3 §11 (no raw passthrough).
type Provider interface {
	Name() string

	// OAuth.
	OAuthAuthorizeURL(ctx context.Context, state, redirectURI string) (string, error)
	OAuthExchange(ctx context.Context, code, redirectURI string) (*Tokens, error)
	RefreshToken(ctx context.Context, refreshToken string) (*Tokens, error)

	// Metrics (fetch-only, no side effects).
	FetchMetrics(ctx context.Context, b BindingRef, dims MetricDimensions) (*MetricSnapshot, error)
	FetchLiveSession(ctx context.Context, b BindingRef, awemeID string) (*LiveSession, error)
	FetchMaterialHealth(ctx context.Context, b BindingRef, materialIDs []string) ([]MaterialHealth, error)

	// Actions — exactly five. Each maps to a single ad-platform mutation.
	AdjustBid(ctx context.Context, b BindingRef, adID string, newBid Money) error
	AdjustBudget(ctx context.Context, b BindingRef, adID string, newBudget Money) error
	PauseAd(ctx context.Context, b BindingRef, adID string) error
	ResumeAd(ctx context.Context, b BindingRef, adID string) error
	ApplyMaterial(ctx context.Context, b BindingRef, adID string, materialID string) error
}
