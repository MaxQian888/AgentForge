package qianchuan

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
	"github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/plugins/qianchuan-ads/binding"
)

// systemActor is used for secret rotations performed by the background refresher.
var systemActor = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// BindingRefresherRepo is the subset of the binding repository the refresher needs.
type BindingRefresherRepo interface {
	FindDueForRefresh(ctx context.Context, before time.Time) ([]*qianchuanbinding.Record, error)
	UpdateExpiry(ctx context.Context, id uuid.UUID, expiresAt time.Time) error
	MarkAuthExpired(ctx context.Context, id uuid.UUID) error
}

// SecretsService is the narrow seam for resolving and rotating secrets.
type SecretsService interface {
	Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
	RotateSecret(ctx context.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error
}

// AuditRecorder is the narrow seam for recording audit events.
type AuditRecorder interface {
	Emit(ctx context.Context, projectID, actorUserID, resourceID uuid.UUID, action, payload string)
}

// Refresher polls for bindings whose access tokens are about to expire
// and refreshes them via the adsplatform.Provider.RefreshToken method.
// Single-instance only in v1.
type Refresher struct {
	Bindings    BindingRefresherRepo
	Secrets     SecretsService
	Registry    *adsplatform.Registry
	Bus         eventbus.Publisher
	Audit       AuditRecorder
	Clock       func() time.Time
	TickEvery   time.Duration
	EarlyWindow time.Duration

	failures   sync.Map // uuid.UUID -> int (consecutive transient failure counter)
}

// Run blocks until ctx is cancelled, ticking every TickEvery.
// TODO(spec3-multi-instance): leader-elect this loop before scaling backend horizontally.
func (r *Refresher) Run(ctx context.Context) {
	if r.TickEvery <= 0 {
		r.TickEvery = 60 * time.Second
	}
	if r.EarlyWindow <= 0 {
		r.EarlyWindow = 10 * time.Minute
	}
	if r.Clock == nil {
		r.Clock = func() time.Time { return time.Now().UTC() }
	}

	log.WithFields(log.Fields{"tick": r.TickEvery, "earlyWindow": r.EarlyWindow}).
		Info("qianchuan refresher started")

	ticker := time.NewTicker(r.TickEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("qianchuan refresher stopped")
			return
		case <-ticker.C:
			if err := r.tick(ctx); err != nil {
				log.WithError(err).Warn("qianchuan refresher tick failed")
			}
		}
	}
}

// Tick exposes the internal tick for testing.
func (r *Refresher) Tick(ctx context.Context) error {
	if r.Clock == nil {
		r.Clock = func() time.Time { return time.Now().UTC() }
	}
	if r.EarlyWindow <= 0 {
		r.EarlyWindow = 10 * time.Minute
	}
	return r.tick(ctx)
}

func (r *Refresher) tick(ctx context.Context) error {
	due, err := r.Bindings.FindDueForRefresh(ctx, r.Clock().Add(r.EarlyWindow))
	if err != nil {
		return err
	}
	for _, b := range due {
		r.refreshOne(ctx, b)
	}
	return nil
}

func (r *Refresher) refreshOne(ctx context.Context, b *qianchuanbinding.Record) {
	provider, err := r.Registry.Resolve("qianchuan")
	if err != nil {
		log.WithError(err).WithField("binding_id", b.ID).Error("refresher: resolve provider")
		return
	}

	rt, err := r.Secrets.Resolve(ctx, b.ProjectID, b.RefreshTokenSecretRef)
	if err != nil {
		log.WithError(err).WithField("binding_id", b.ID).Warn("refresher: resolve refresh secret")
		r.markAuthExpired(ctx, b, "refresh_secret_missing")
		return
	}

	tokens, err := provider.RefreshToken(ctx, rt)
	if err != nil {
		if isAuthInvalid(err) {
			r.markAuthExpired(ctx, b, "refresh_invalid")
			return
		}
		r.bumpTransientFailure(b.ID)
		if r.transientFailures(b.ID) >= 3 {
			r.markAuthExpired(ctx, b, "transient_threshold_exceeded")
		} else {
			log.WithError(err).WithField("binding_id", b.ID).Warn("refresher: transient refresh failure")
		}
		return
	}
	r.resetTransientFailure(b.ID)

	// Rotate access token secret.
	if err := r.Secrets.RotateSecret(ctx, b.ProjectID, b.AccessTokenSecretRef, tokens.AccessToken, systemActor); err != nil {
		log.WithError(err).WithField("binding_id", b.ID).Error("refresher: rotate access token")
		return
	}
	// Rotate refresh token if it changed (OAuth2 with rotation).
	if tokens.RefreshToken != "" && tokens.RefreshToken != rt {
		if err := r.Secrets.RotateSecret(ctx, b.ProjectID, b.RefreshTokenSecretRef, tokens.RefreshToken, systemActor); err != nil {
			log.WithError(err).WithField("binding_id", b.ID).Error("refresher: rotate refresh token")
			return
		}
	}
	// Update expiry on binding.
	if err := r.Bindings.UpdateExpiry(ctx, b.ID, tokens.ExpiresAt); err != nil {
		log.WithError(err).WithField("binding_id", b.ID).Warn("refresher: update expiry")
	}

	// Audit (never include plaintext).
	if r.Audit != nil {
		payload, _ := json.Marshal(map[string]any{
			"binding_id":        b.ID,
			"advertiser_id":     b.AdvertiserID,
			"access_expires_at": tokens.ExpiresAt,
			"refresh_rotated":   tokens.RefreshToken != "" && tokens.RefreshToken != rt,
		})
		r.Audit.Emit(ctx, b.ProjectID, systemActor, b.ID, "qianchuan.token.refreshed", string(payload))
	}
}

func (r *Refresher) markAuthExpired(ctx context.Context, b *qianchuanbinding.Record, reason string) {
	if err := r.Bindings.MarkAuthExpired(ctx, b.ID); err != nil {
		log.WithError(err).WithField("binding_id", b.ID).Error("refresher: mark auth_expired")
	}

	payload := AuthExpiredPayload{
		BindingID:    b.ID,
		ProjectID:    b.ProjectID,
		EmployeeID:   b.ActingEmployeeID,
		ProviderID:   "qianchuan",
		AdvertiserID: b.AdvertiserID,
		Reason:       reason,
		DetectedAt:   r.Clock(),
	}
	payloadJSON, _ := json.Marshal(payload)
	evt := eventbus.NewEvent(
		eventbus.EventAdsPlatformAuthExpired,
		"qianchuan.refresher",
		"project:"+b.ProjectID.String(),
	)
	evt.Payload = payloadJSON
	if err := r.Bus.Publish(ctx, evt); err != nil {
		log.WithError(err).WithField("binding_id", b.ID).Warn("refresher: publish auth_expired event")
	}

	if r.Audit != nil {
		auditPayload, _ := json.Marshal(map[string]any{"binding_id": b.ID, "reason": reason})
		r.Audit.Emit(ctx, b.ProjectID, systemActor, b.ID, "qianchuan.token.refresh_failed", string(auditPayload))
	}
}

// isAuthInvalid checks whether the error indicates a terminal auth failure.
func isAuthInvalid(err error) bool {
	return errors.Is(err, adsplatform.ErrAuthExpired)
}

func (r *Refresher) bumpTransientFailure(id uuid.UUID) {
	val, _ := r.failures.LoadOrStore(id, 0)
	r.failures.Store(id, val.(int)+1)
}

func (r *Refresher) transientFailures(id uuid.UUID) int {
	val, ok := r.failures.Load(id)
	if !ok {
		return 0
	}
	return val.(int)
}

func (r *Refresher) resetTransientFailure(id uuid.UUID) {
	r.failures.Delete(id)
}
