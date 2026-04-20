package qianchuanbinding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/adsplatform"
	mockprov "github.com/agentforge/server/internal/adsplatform/mock"
	qianchuanbinding "github.com/agentforge/server/plugins/qianchuan-ads/binding"
	"github.com/google/uuid"
)

type fakeSecrets struct {
	present map[string]string // secret name → plaintext
}

func (f *fakeSecrets) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	if v, ok := f.present[name]; ok {
		return v, nil
	}
	return "", errors.New("secret:not_found")
}

func TestService_Create_RejectsMissingSecret(t *testing.T) {
	repo := newMem()
	secrets := &fakeSecrets{present: map[string]string{"qc.access": "tok"}}
	prov := mockprov.New("qianchuan")
	svc := qianchuanbinding.NewService(repo, secrets, prov)
	_, err := svc.Create(context.Background(), qianchuanbinding.CreateInput{
		ProjectID:             uuid.New(),
		AdvertiserID:          "A1",
		AccessTokenSecretRef:  "qc.access",
		RefreshTokenSecretRef: "qc.refresh-missing", // not in secrets store
		CreatedBy:             uuid.New(),
	})
	if !errors.Is(err, qianchuanbinding.ErrSecretMissing) {
		t.Fatalf("want ErrSecretMissing, got %v", err)
	}
}

func TestService_Create_VerifiesAuthViaProvider(t *testing.T) {
	repo := newMem()
	secrets := &fakeSecrets{present: map[string]string{"qc.access": "tok", "qc.refresh": "ref"}}
	prov := mockprov.New("qianchuan")
	prov.SetMetrics(&adsplatform.MetricSnapshot{Ads: []adsplatform.AdMetric{{AdID: "AD1"}}})
	svc := qianchuanbinding.NewService(repo, secrets, prov)
	rec, err := svc.Create(context.Background(), qianchuanbinding.CreateInput{
		ProjectID:             uuid.New(),
		AdvertiserID:          "A1",
		AccessTokenSecretRef:  "qc.access",
		RefreshTokenSecretRef: "qc.refresh",
		DisplayName:           "店铺A",
		CreatedBy:             uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != qianchuanbinding.StatusActive {
		t.Errorf("status=%s", rec.Status)
	}
}

func TestService_Create_AuthExpiredFailsClosed(t *testing.T) {
	repo := newMem()
	secrets := &fakeSecrets{present: map[string]string{"qc.access": "tok", "qc.refresh": "ref"}}
	prov := mockprov.New("qianchuan")
	prov.SetMetricsError(adsplatform.ErrAuthExpired)
	svc := qianchuanbinding.NewService(repo, secrets, prov)
	_, err := svc.Create(context.Background(), qianchuanbinding.CreateInput{
		ProjectID:             uuid.New(),
		AdvertiserID:          "A1",
		AccessTokenSecretRef:  "qc.access",
		RefreshTokenSecretRef: "qc.refresh",
		CreatedBy:             uuid.New(),
	})
	if !errors.Is(err, adsplatform.ErrAuthExpired) {
		t.Fatalf("want ErrAuthExpired, got %v", err)
	}
}

func TestService_Sync_TouchesLastSyncedAt(t *testing.T) {
	repo := newMem()
	secrets := &fakeSecrets{present: map[string]string{"qc.access": "tok", "qc.refresh": "ref"}}
	prov := mockprov.New("qianchuan")
	svc := qianchuanbinding.NewService(repo, secrets, prov)
	rec, err := svc.Create(context.Background(), qianchuanbinding.CreateInput{
		ProjectID:             uuid.New(),
		AdvertiserID:          "A1",
		AccessTokenSecretRef:  "qc.access",
		RefreshTokenSecretRef: "qc.refresh",
		CreatedBy:             uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Sync(context.Background(), rec.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Get(context.Background(), rec.ID)
	if got.LastSyncedAt == nil {
		t.Fatal("LastSyncedAt should be set after Sync")
	}
}
