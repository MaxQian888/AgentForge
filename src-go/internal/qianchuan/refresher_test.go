package qianchuan

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
	"github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/qianchuanbinding"
)

// --- mocks ---

type mockBindingRepo struct {
	mu       sync.Mutex
	bindings []*qianchuanbinding.Record
	expired  map[uuid.UUID]bool
	expiry   map[uuid.UUID]time.Time
}

func (m *mockBindingRepo) FindDueForRefresh(_ context.Context, before time.Time) ([]*qianchuanbinding.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*qianchuanbinding.Record
	for _, b := range m.bindings {
		if b.Status == qianchuanbinding.StatusActive && b.TokenExpiresAt != nil && b.TokenExpiresAt.Before(before) {
			out = append(out, b)
		}
	}
	return out, nil
}

func (m *mockBindingRepo) UpdateExpiry(_ context.Context, id uuid.UUID, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.expiry == nil {
		m.expiry = make(map[uuid.UUID]time.Time)
	}
	m.expiry[id] = expiresAt
	return nil
}

func (m *mockBindingRepo) MarkAuthExpired(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.expired == nil {
		m.expired = make(map[uuid.UUID]bool)
	}
	m.expired[id] = true
	return nil
}

type mockSecrets struct {
	tokens map[string]string
	rotations []struct {
		Name      string
		Plaintext string
	}
	mu sync.Mutex
}

func (m *mockSecrets) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.tokens[name]; ok {
		return v, nil
	}
	return "", errors.New("secret:not_found")
}

func (m *mockSecrets) RotateSecret(_ context.Context, _ uuid.UUID, name, plaintext string, _ uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rotations = append(m.rotations, struct {
		Name      string
		Plaintext string
	}{name, plaintext})
	m.tokens[name] = plaintext
	return nil
}

type mockProvider struct {
	refreshFunc func(ctx context.Context, rt string) (*adsplatform.Tokens, error)
}

func (p *mockProvider) Name() string { return "qianchuan" }
func (p *mockProvider) OAuthAuthorizeURL(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (p *mockProvider) OAuthExchange(_ context.Context, _, _ string) (*adsplatform.Tokens, error) {
	return nil, nil
}
func (p *mockProvider) RefreshToken(ctx context.Context, rt string) (*adsplatform.Tokens, error) {
	return p.refreshFunc(ctx, rt)
}
func (p *mockProvider) FetchMetrics(_ context.Context, _ adsplatform.BindingRef, _ adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
	return nil, nil
}
func (p *mockProvider) FetchLiveSession(_ context.Context, _ adsplatform.BindingRef, _ string) (*adsplatform.LiveSession, error) {
	return nil, nil
}
func (p *mockProvider) FetchMaterialHealth(_ context.Context, _ adsplatform.BindingRef, _ []string) ([]adsplatform.MaterialHealth, error) {
	return nil, nil
}
func (p *mockProvider) AdjustBid(_ context.Context, _ adsplatform.BindingRef, _ string, _ adsplatform.Money) error {
	return nil
}
func (p *mockProvider) AdjustBudget(_ context.Context, _ adsplatform.BindingRef, _ string, _ adsplatform.Money) error {
	return nil
}
func (p *mockProvider) PauseAd(_ context.Context, _ adsplatform.BindingRef, _ string) error {
	return nil
}
func (p *mockProvider) ResumeAd(_ context.Context, _ adsplatform.BindingRef, _ string) error {
	return nil
}
func (p *mockProvider) ApplyMaterial(_ context.Context, _ adsplatform.BindingRef, _, _ string) error {
	return nil
}

type mockBus struct {
	mu     sync.Mutex
	events []*eventbus.Event
}

func (b *mockBus) Publish(_ context.Context, e *eventbus.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, e)
	return nil
}

type mockAudit struct {
	mu      sync.Mutex
	entries []string
}

func (a *mockAudit) Emit(_ context.Context, _, _, _ uuid.UUID, action, _ string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, action)
}

// --- tests ---

func TestRefresher_PicksDueBindings(t *testing.T) {
	now := time.Now().UTC()
	fiveMin := now.Add(5 * time.Minute)
	thirtyMin := now.Add(30 * time.Minute)

	bindingA := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "A1",
		Status: qianchuanbinding.StatusActive, TokenExpiresAt: &fiveMin,
		AccessTokenSecretRef: "qianchuan.A1.access_token", RefreshTokenSecretRef: "qianchuan.A1.refresh_token",
	}
	bindingB := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "B1",
		Status: qianchuanbinding.StatusActive, TokenExpiresAt: &thirtyMin,
		AccessTokenSecretRef: "qianchuan.B1.access_token", RefreshTokenSecretRef: "qianchuan.B1.refresh_token",
	}
	bindingC := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "C1",
		Status: qianchuanbinding.StatusPaused, TokenExpiresAt: &fiveMin,
		AccessTokenSecretRef: "qianchuan.C1.access_token", RefreshTokenSecretRef: "qianchuan.C1.refresh_token",
	}

	refreshCalled := map[string]bool{}
	var mu sync.Mutex
	prov := &mockProvider{
		refreshFunc: func(_ context.Context, rt string) (*adsplatform.Tokens, error) {
			mu.Lock()
			refreshCalled[rt] = true
			mu.Unlock()
			return &adsplatform.Tokens{
				AccessToken:  "new_at",
				RefreshToken: "new_rt",
				ExpiresAt:    now.Add(2 * time.Hour),
			}, nil
		},
	}
	registry := adsplatform.NewRegistry()
	registry.Register("qianchuan", func() adsplatform.Provider { return prov })

	secrets := &mockSecrets{tokens: map[string]string{
		"qianchuan.A1.refresh_token": "rt_a",
		"qianchuan.B1.refresh_token": "rt_b",
		"qianchuan.C1.refresh_token": "rt_c",
		"qianchuan.A1.access_token":  "at_a",
	}}

	repo := &mockBindingRepo{bindings: []*qianchuanbinding.Record{bindingA, bindingB, bindingC}}

	refresher := &Refresher{
		Bindings:    repo,
		Secrets:     secrets,
		Registry:    registry,
		Bus:         &mockBus{},
		Audit:       &mockAudit{},
		Clock:       func() time.Time { return now },
		EarlyWindow: 10 * time.Minute,
	}

	err := refresher.Tick(context.Background())
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, refreshCalled["rt_a"], "binding A should be refreshed")
	assert.False(t, refreshCalled["rt_b"], "binding B should NOT be refreshed (too far)")
	assert.False(t, refreshCalled["rt_c"], "binding C should NOT be refreshed (paused)")
}

func TestRefresher_TickerStartsAndStops(t *testing.T) {
	repo := &mockBindingRepo{}
	registry := adsplatform.NewRegistry()
	registry.Register("qianchuan", func() adsplatform.Provider { return &mockProvider{} })

	refresher := &Refresher{
		Bindings:    repo,
		Secrets:     &mockSecrets{tokens: map[string]string{}},
		Registry:    registry,
		Bus:         &mockBus{},
		TickEvery:   10 * time.Millisecond,
		EarlyWindow: 10 * time.Minute,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		refresher.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(200 * time.Millisecond):
		t.Fatal("refresher did not stop within 200ms")
	}
}

func TestRefreshOne_Success(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(5 * time.Minute)
	binding := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "AD1",
		Status: qianchuanbinding.StatusActive, TokenExpiresAt: &expiry,
		AccessTokenSecretRef: "qianchuan.AD1.access_token", RefreshTokenSecretRef: "qianchuan.AD1.refresh_token",
	}

	newExpiry := now.Add(2 * time.Hour)
	prov := &mockProvider{
		refreshFunc: func(_ context.Context, _ string) (*adsplatform.Tokens, error) {
			return &adsplatform.Tokens{
				AccessToken:  "AT2",
				RefreshToken: "RT2",
				ExpiresAt:    newExpiry,
			}, nil
		},
	}
	registry := adsplatform.NewRegistry()
	registry.Register("qianchuan", func() adsplatform.Provider { return prov })

	secrets := &mockSecrets{tokens: map[string]string{
		"qianchuan.AD1.access_token":  "AT1",
		"qianchuan.AD1.refresh_token": "RT1",
	}}
	repo := &mockBindingRepo{bindings: []*qianchuanbinding.Record{binding}}
	bus := &mockBus{}
	audit := &mockAudit{}

	refresher := &Refresher{
		Bindings:    repo,
		Secrets:     secrets,
		Registry:    registry,
		Bus:         bus,
		Audit:       audit,
		Clock:       func() time.Time { return now },
		EarlyWindow: 10 * time.Minute,
	}

	refresher.refreshOne(context.Background(), binding)

	// Secrets rotated
	assert.Len(t, secrets.rotations, 2)
	assert.Equal(t, "AT2", secrets.tokens["qianchuan.AD1.access_token"])
	assert.Equal(t, "RT2", secrets.tokens["qianchuan.AD1.refresh_token"])

	// Expiry updated
	repo.mu.Lock()
	assert.Equal(t, newExpiry, repo.expiry[binding.ID])
	repo.mu.Unlock()

	// Audit recorded, no event emitted
	assert.Contains(t, audit.entries, "qianchuan.token.refreshed")
	assert.Empty(t, bus.events)
}

func TestRefreshOne_RefreshInvalid_401(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(5 * time.Minute)
	binding := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "AD1",
		Status: qianchuanbinding.StatusActive, TokenExpiresAt: &expiry,
		AccessTokenSecretRef: "qianchuan.AD1.access_token", RefreshTokenSecretRef: "qianchuan.AD1.refresh_token",
	}

	prov := &mockProvider{
		refreshFunc: func(_ context.Context, _ string) (*adsplatform.Tokens, error) {
			return nil, adsplatform.ErrAuthExpired
		},
	}
	registry := adsplatform.NewRegistry()
	registry.Register("qianchuan", func() adsplatform.Provider { return prov })

	secrets := &mockSecrets{tokens: map[string]string{
		"qianchuan.AD1.refresh_token": "RT1",
	}}
	repo := &mockBindingRepo{bindings: []*qianchuanbinding.Record{binding}}
	bus := &mockBus{}
	audit := &mockAudit{}

	refresher := &Refresher{
		Bindings:    repo,
		Secrets:     secrets,
		Registry:    registry,
		Bus:         bus,
		Audit:       audit,
		Clock:       func() time.Time { return now },
		EarlyWindow: 10 * time.Minute,
	}

	refresher.refreshOne(context.Background(), binding)

	// Binding marked auth_expired
	repo.mu.Lock()
	assert.True(t, repo.expired[binding.ID])
	repo.mu.Unlock()

	// Event published
	bus.mu.Lock()
	assert.Len(t, bus.events, 1)
	assert.Equal(t, eventbus.EventAdsPlatformAuthExpired, bus.events[0].Type)
	bus.mu.Unlock()

	assert.Contains(t, audit.entries, "qianchuan.token.refresh_failed")
}

func TestRefreshOne_TransientError_BelowThreshold(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(5 * time.Minute)
	binding := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "AD1",
		Status: qianchuanbinding.StatusActive, TokenExpiresAt: &expiry,
		AccessTokenSecretRef: "qianchuan.AD1.access_token", RefreshTokenSecretRef: "qianchuan.AD1.refresh_token",
	}

	prov := &mockProvider{
		refreshFunc: func(_ context.Context, _ string) (*adsplatform.Tokens, error) {
			return nil, adsplatform.ErrTransientFailure
		},
	}
	registry := adsplatform.NewRegistry()
	registry.Register("qianchuan", func() adsplatform.Provider { return prov })

	secrets := &mockSecrets{tokens: map[string]string{
		"qianchuan.AD1.refresh_token": "RT1",
	}}
	repo := &mockBindingRepo{bindings: []*qianchuanbinding.Record{binding}}
	bus := &mockBus{}

	refresher := &Refresher{
		Bindings:    repo,
		Secrets:     secrets,
		Registry:    registry,
		Bus:         bus,
		Audit:       &mockAudit{},
		Clock:       func() time.Time { return now },
		EarlyWindow: 10 * time.Minute,
	}

	// First failure — should NOT mark expired
	refresher.refreshOne(context.Background(), binding)
	repo.mu.Lock()
	assert.False(t, repo.expired[binding.ID])
	repo.mu.Unlock()
	assert.Empty(t, bus.events)
}

func TestRefreshOne_TransientError_ThresholdExceeded(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(5 * time.Minute)
	binding := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "AD1",
		Status: qianchuanbinding.StatusActive, TokenExpiresAt: &expiry,
		AccessTokenSecretRef: "qianchuan.AD1.access_token", RefreshTokenSecretRef: "qianchuan.AD1.refresh_token",
	}

	prov := &mockProvider{
		refreshFunc: func(_ context.Context, _ string) (*adsplatform.Tokens, error) {
			return nil, adsplatform.ErrTransientFailure
		},
	}
	registry := adsplatform.NewRegistry()
	registry.Register("qianchuan", func() adsplatform.Provider { return prov })

	secrets := &mockSecrets{tokens: map[string]string{
		"qianchuan.AD1.refresh_token": "RT1",
	}}
	repo := &mockBindingRepo{bindings: []*qianchuanbinding.Record{binding}}
	bus := &mockBus{}

	refresher := &Refresher{
		Bindings:    repo,
		Secrets:     secrets,
		Registry:    registry,
		Bus:         bus,
		Audit:       &mockAudit{},
		Clock:       func() time.Time { return now },
		EarlyWindow: 10 * time.Minute,
	}

	// 3 consecutive failures → marks auth_expired
	refresher.refreshOne(context.Background(), binding)
	refresher.refreshOne(context.Background(), binding)
	refresher.refreshOne(context.Background(), binding)

	repo.mu.Lock()
	assert.True(t, repo.expired[binding.ID])
	repo.mu.Unlock()
}

func TestRefreshOne_SecretsMissing(t *testing.T) {
	now := time.Now().UTC()
	expiry := now.Add(5 * time.Minute)
	binding := &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: uuid.New(), AdvertiserID: "AD1",
		Status: qianchuanbinding.StatusActive, TokenExpiresAt: &expiry,
		AccessTokenSecretRef: "qianchuan.AD1.access_token", RefreshTokenSecretRef: "qianchuan.AD1.refresh_token",
	}

	registry := adsplatform.NewRegistry()
	registry.Register("qianchuan", func() adsplatform.Provider { return &mockProvider{} })

	secrets := &mockSecrets{tokens: map[string]string{}} // no tokens
	repo := &mockBindingRepo{bindings: []*qianchuanbinding.Record{binding}}
	bus := &mockBus{}

	refresher := &Refresher{
		Bindings:    repo,
		Secrets:     secrets,
		Registry:    registry,
		Bus:         bus,
		Audit:       &mockAudit{},
		Clock:       func() time.Time { return now },
		EarlyWindow: 10 * time.Minute,
	}

	refresher.refreshOne(context.Background(), binding)

	repo.mu.Lock()
	assert.True(t, repo.expired[binding.ID])
	repo.mu.Unlock()
}
