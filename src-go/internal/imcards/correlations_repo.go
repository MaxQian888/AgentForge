package imcards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrCorrelationNotFound is returned by Lookup when no row matches the
// supplied token. Distinct from a query error so the router can map it to
// the "fall through to trigger handler" branch.
var ErrCorrelationNotFound = errors.New("imcards: correlation not found")

// Correlation mirrors the card_action_correlations row shape. Payload is
// already-parsed JSON; callers receive a typed map and need not unmarshal.
type Correlation struct {
	Token       uuid.UUID
	ExecutionID uuid.UUID
	NodeID      string
	ActionID    string
	Payload     map[string]any
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
	CreatedAt   time.Time
}

// CorrelationInput is the Create signature. ExpiresAt MUST be set by the
// caller -- repo refuses to default it so the lifetime policy stays in the
// im_send applier where the operator can audit it.
type CorrelationInput struct {
	ExecutionID uuid.UUID
	NodeID      string
	ActionID    string
	Payload     map[string]any
	ExpiresAt   time.Time
}

type CorrelationsRepo struct{ db *gorm.DB }

func NewCorrelationsRepo(db *gorm.DB) *CorrelationsRepo { return &CorrelationsRepo{db: db} }

// Create inserts a new correlation and returns the freshly minted token.
func (r *CorrelationsRepo) Create(ctx context.Context, in *CorrelationInput) (uuid.UUID, error) {
	if in == nil {
		return uuid.Nil, fmt.Errorf("nil input")
	}
	if in.ExpiresAt.IsZero() {
		return uuid.Nil, fmt.Errorf("ExpiresAt is required")
	}
	var payloadBytes []byte
	if in.Payload != nil {
		b, err := json.Marshal(in.Payload)
		if err != nil {
			return uuid.Nil, fmt.Errorf("marshal payload: %w", err)
		}
		payloadBytes = b
	}
	var token uuid.UUID
	result := r.db.WithContext(ctx).Raw(`
		INSERT INTO card_action_correlations
			(execution_id, node_id, action_id, payload, expires_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING token`,
		in.ExecutionID, in.NodeID, in.ActionID, payloadBytes, in.ExpiresAt,
	).Scan(&token)
	if result.Error != nil {
		return uuid.Nil, fmt.Errorf("insert correlation: %w", result.Error)
	}
	return token, nil
}

// Lookup returns the row matching token. It does NOT enforce expiry --
// callers (the router) compare against `time.Now()` so the failure mode
// is distinguishable from a missing token.
func (r *CorrelationsRepo) Lookup(ctx context.Context, token uuid.UUID) (*Correlation, error) {
	type row struct {
		Token       uuid.UUID  `gorm:"column:token"`
		ExecutionID uuid.UUID  `gorm:"column:execution_id"`
		NodeID      string     `gorm:"column:node_id"`
		ActionID    string     `gorm:"column:action_id"`
		Payload     []byte     `gorm:"column:payload"`
		ExpiresAt   time.Time  `gorm:"column:expires_at"`
		ConsumedAt  *time.Time `gorm:"column:consumed_at"`
		CreatedAt   time.Time  `gorm:"column:created_at"`
	}
	var r2 row
	result := r.db.WithContext(ctx).Raw(`
		SELECT token, execution_id, node_id, action_id, payload,
			   expires_at, consumed_at, created_at
		FROM card_action_correlations
		WHERE token = ?`, token).Scan(&r2)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrCorrelationNotFound
		}
		return nil, fmt.Errorf("scan correlation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrCorrelationNotFound
	}
	c := &Correlation{
		Token:       r2.Token,
		ExecutionID: r2.ExecutionID,
		NodeID:      r2.NodeID,
		ActionID:    r2.ActionID,
		ExpiresAt:   r2.ExpiresAt,
		ConsumedAt:  r2.ConsumedAt,
		CreatedAt:   r2.CreatedAt,
	}
	if len(r2.Payload) > 0 {
		if err := json.Unmarshal(r2.Payload, &c.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	return c, nil
}

// MarkConsumed stamps consumed_at = now() exactly once. A second call
// succeeds but does NOT overwrite the original timestamp -- uniqueness is
// enforced by the WHERE clause.
func (r *CorrelationsRepo) MarkConsumed(ctx context.Context, token uuid.UUID) error {
	result := r.db.WithContext(ctx).Exec(`
		UPDATE card_action_correlations
		   SET consumed_at = now()
		 WHERE token = ?
		   AND consumed_at IS NULL`, token)
	return result.Error
}
