// Package mock provides an in-memory adsplatform.Provider that records
// every call. Used by handler/integration tests to avoid live HTTP.
package mock

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/agentforge/server/internal/adsplatform"
)

// Call is one recorded Provider invocation.
type Call struct {
	Method     string
	AdID       string
	AwemeID    string
	Money      adsplatform.Money
	MaterialID string
}

// Provider is a configurable test double.
type Provider struct {
	name string

	mu          sync.Mutex
	calls       []Call
	metrics     *adsplatform.MetricSnapshot
	metricsErr  error
	liveSession *adsplatform.LiveSession
	material    []adsplatform.MaterialHealth
	nextErr     error
	tokens      *adsplatform.Tokens
}

// New returns a Provider that reports the given name.
func New(name string) *Provider { return &Provider{name: name} }

// Name returns the configured provider name.
func (p *Provider) Name() string { return p.name }

// SetMetrics installs the snapshot returned by FetchMetrics.
func (p *Provider) SetMetrics(s *adsplatform.MetricSnapshot) { p.metrics = s }

// SetMetricsError configures FetchMetrics to return err on every call.
func (p *Provider) SetMetricsError(err error) { p.metricsErr = err }

// SetLiveSession installs the response returned by FetchLiveSession.
func (p *Provider) SetLiveSession(s *adsplatform.LiveSession) { p.liveSession = s }

// SetMaterialHealth installs the response returned by FetchMaterialHealth.
func (p *Provider) SetMaterialHealth(m []adsplatform.MaterialHealth) { p.material = m }

// SetTokens installs the response returned by OAuthExchange / RefreshToken.
func (p *Provider) SetTokens(t *adsplatform.Tokens) { p.tokens = t }

// FailNext makes the next mutating call return err (one-shot).
func (p *Provider) FailNext(err error) { p.nextErr = err }

// Calls returns a copy of the recorded call log.
func (p *Provider) Calls() []Call {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Call, len(p.calls))
	copy(out, p.calls)
	return out
}

func (p *Provider) record(c Call) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, c)
	if p.nextErr != nil {
		err := p.nextErr
		p.nextErr = nil
		return err
	}
	return nil
}

// OAuthAuthorizeURL returns a fixed URL for the configured name.
func (p *Provider) OAuthAuthorizeURL(_ context.Context, state, redirectURI string) (string, error) {
	return "https://mock/" + p.name + "/oauth?state=" + state + "&redirect_uri=" + redirectURI, nil
}

// OAuthExchange returns the configured tokens or a sentinel default.
func (p *Provider) OAuthExchange(_ context.Context, _ string, _ string) (*adsplatform.Tokens, error) {
	if p.tokens != nil {
		return p.tokens, nil
	}
	return &adsplatform.Tokens{AccessToken: "mock-access", RefreshToken: "mock-refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// RefreshToken returns the configured tokens or a default fresh pair.
func (p *Provider) RefreshToken(_ context.Context, _ string) (*adsplatform.Tokens, error) {
	if p.tokens != nil {
		return p.tokens, nil
	}
	return &adsplatform.Tokens{AccessToken: "mock-access", RefreshToken: "mock-refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// FetchMetrics returns the configured snapshot.
func (p *Provider) FetchMetrics(_ context.Context, _ adsplatform.BindingRef, _ adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
	if p.metricsErr != nil {
		return nil, p.metricsErr
	}
	if p.metrics == nil {
		return &adsplatform.MetricSnapshot{}, nil
	}
	return p.metrics, nil
}

// FetchLiveSession returns the configured live-session response.
func (p *Provider) FetchLiveSession(_ context.Context, _ adsplatform.BindingRef, awemeID string) (*adsplatform.LiveSession, error) {
	if p.liveSession == nil {
		return &adsplatform.LiveSession{AwemeID: awemeID}, nil
	}
	return p.liveSession, nil
}

// FetchMaterialHealth returns the configured material health.
func (p *Provider) FetchMaterialHealth(_ context.Context, _ adsplatform.BindingRef, _ []string) ([]adsplatform.MaterialHealth, error) {
	return p.material, nil
}

// AdjustBid records the call.
func (p *Provider) AdjustBid(_ context.Context, _ adsplatform.BindingRef, adID string, money adsplatform.Money) error {
	return p.record(Call{Method: "AdjustBid", AdID: adID, Money: money})
}

// AdjustBudget records the call.
func (p *Provider) AdjustBudget(_ context.Context, _ adsplatform.BindingRef, adID string, money adsplatform.Money) error {
	return p.record(Call{Method: "AdjustBudget", AdID: adID, Money: money})
}

// PauseAd records the call.
func (p *Provider) PauseAd(_ context.Context, _ adsplatform.BindingRef, adID string) error {
	return p.record(Call{Method: "PauseAd", AdID: adID})
}

// ResumeAd records the call.
func (p *Provider) ResumeAd(_ context.Context, _ adsplatform.BindingRef, adID string) error {
	return p.record(Call{Method: "ResumeAd", AdID: adID})
}

// ApplyMaterial records the call.
func (p *Provider) ApplyMaterial(_ context.Context, _ adsplatform.BindingRef, adID, materialID string) error {
	return p.record(Call{Method: "ApplyMaterial", AdID: adID, MaterialID: materialID})
}

// compile-time check: Provider satisfies adsplatform.Provider.
var _ adsplatform.Provider = (*Provider)(nil)

// ErrSimulatedAuth is the sentinel mock callers use when injecting
// an auth_expired-shaped failure.
var ErrSimulatedAuth = errors.New("mock: simulated auth_expired")
