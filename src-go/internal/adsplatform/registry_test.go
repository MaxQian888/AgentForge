package adsplatform_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentforge/server/internal/adsplatform"
)

type stubProvider struct{ name string }

func (s *stubProvider) Name() string { return s.name }
func (*stubProvider) OAuthAuthorizeURL(context.Context, string, string) (string, error) {
	return "", errors.ErrUnsupported
}
func (*stubProvider) OAuthExchange(context.Context, string, string) (*adsplatform.Tokens, error) {
	return nil, errors.ErrUnsupported
}
func (*stubProvider) RefreshToken(context.Context, string) (*adsplatform.Tokens, error) {
	return &adsplatform.Tokens{AccessToken: "a", RefreshToken: "r", ExpiresAt: time.Now()}, nil
}
func (*stubProvider) FetchMetrics(context.Context, adsplatform.BindingRef, adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
	return &adsplatform.MetricSnapshot{}, nil
}
func (*stubProvider) FetchLiveSession(context.Context, adsplatform.BindingRef, string) (*adsplatform.LiveSession, error) {
	return &adsplatform.LiveSession{}, nil
}
func (*stubProvider) FetchMaterialHealth(context.Context, adsplatform.BindingRef, []string) ([]adsplatform.MaterialHealth, error) {
	return nil, nil
}
func (*stubProvider) AdjustBid(context.Context, adsplatform.BindingRef, string, adsplatform.Money) error {
	return nil
}
func (*stubProvider) AdjustBudget(context.Context, adsplatform.BindingRef, string, adsplatform.Money) error {
	return nil
}
func (*stubProvider) PauseAd(context.Context, adsplatform.BindingRef, string) error  { return nil }
func (*stubProvider) ResumeAd(context.Context, adsplatform.BindingRef, string) error { return nil }
func (*stubProvider) ApplyMaterial(context.Context, adsplatform.BindingRef, string, string) error {
	return nil
}

func TestRegistry_RegisterAndResolve(t *testing.T) {
	reg := adsplatform.NewRegistry()
	reg.Register("stub", func() adsplatform.Provider { return &stubProvider{name: "stub"} })
	got, err := reg.Resolve("stub")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Name() != "stub" {
		t.Errorf("got %q, want stub", got.Name())
	}
}

func TestRegistry_ResolveUnknown(t *testing.T) {
	reg := adsplatform.NewRegistry()
	if _, err := reg.Resolve("nope"); !errors.Is(err, adsplatform.ErrProviderNotFound) {
		t.Fatalf("want ErrProviderNotFound, got %v", err)
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	reg := adsplatform.NewRegistry()
	ctor := func() adsplatform.Provider { return &stubProvider{name: "x"} }
	reg.Register("x", ctor)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate register")
		}
	}()
	reg.Register("x", ctor)
}
