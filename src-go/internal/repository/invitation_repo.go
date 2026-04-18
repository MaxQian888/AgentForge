package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

// InvitationRepository persists `project_invitations` rows.
type InvitationRepository struct {
	db *gorm.DB
}

func NewInvitationRepository(db *gorm.DB) *InvitationRepository {
	return &InvitationRepository{db: db}
}

// InvitationListFilter narrows ListByProject results.
type InvitationListFilter struct {
	Status string // empty = all statuses
}

// Create persists a new invitation row. Caller must populate ID / timestamps.
func (r *InvitationRepository) Create(ctx context.Context, invitation *model.Invitation) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newInvitationRecord(invitation)).Error; err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

// GetByID returns a single invitation by primary key.
func (r *InvitationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record invitationRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get invitation by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// FindByTokenHash returns the pending invitation matching the supplied
// token hash, or ErrNotFound when no active row exists.
func (r *InvitationRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*model.Invitation, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if tokenHash == "" {
		return nil, ErrNotFound
	}
	var record invitationRecord
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		Take(&record).Error
	if err != nil {
		return nil, fmt.Errorf("find invitation by token hash: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// ListByProject returns invitations for a given project. Filter defaults
// to returning all statuses ordered by created_at desc.
func (r *InvitationRepository) ListByProject(ctx context.Context, projectID uuid.UUID, filter InvitationListFilter) ([]*model.Invitation, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	var records []invitationRecord
	if err := q.Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	out := make([]*model.Invitation, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

// ExistsPendingForIdentity reports whether a pending invitation already
// exists for the given project + canonical invited_identity. Used to
// enforce the one-pending-per-identity duplicate guard.
//
// Matching is shallow string compare on the JSONB column content; callers
// must normalise the identity (Canonical) before invoking.
func (r *InvitationRepository) ExistsPendingForIdentity(ctx context.Context, projectID uuid.UUID, identity model.InvitedIdentity) (bool, error) {
	if r.db == nil {
		return false, ErrDatabaseUnavailable
	}
	var n int64
	q := r.db.WithContext(ctx).
		Model(&invitationRecord{}).
		Where("project_id = ? AND status = ?", projectID, model.InvitationStatusPending)

	switch identity.Kind {
	case model.InvitedIdentityKindEmail:
		q = q.Where("invited_identity->>'kind' = ? AND LOWER(invited_identity->>'value') = ?",
			model.InvitedIdentityKindEmail, identity.Email)
	case model.InvitedIdentityKindIM:
		q = q.Where("invited_identity->>'kind' = ? AND invited_identity->>'platform' = ? AND invited_identity->>'userId' = ?",
			model.InvitedIdentityKindIM, identity.Platform, identity.UserID)
	default:
		return false, nil
	}

	if err := q.Count(&n).Error; err != nil {
		return false, fmt.Errorf("exists pending invitation: %w", err)
	}
	return n > 0, nil
}

// UpdateStatus transitions an invitation row. Caller is responsible for
// all state-machine validation; this method only persists.
//
// When transitioning to a terminal state we always clear token_hash to
// make the token single-use and prevent replay.
type InvitationStatusUpdate struct {
	Status                  string
	TokenHash               *string
	AcceptedAt              *time.Time
	InvitedUserID           *uuid.UUID
	DeclineReason           string
	RevokeReason            string
	LastDeliveryStatus      string
	LastDeliveryAttemptedAt *time.Time
	UpdatedAt               time.Time
}

// Update applies the non-zero fields from the given update to the row.
func (r *InvitationRepository) Update(ctx context.Context, id uuid.UUID, upd InvitationStatusUpdate) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{}
	if upd.Status != "" {
		updates["status"] = upd.Status
	}
	if upd.TokenHash != nil {
		if *upd.TokenHash == "" {
			updates["token_hash"] = nil
		} else {
			updates["token_hash"] = *upd.TokenHash
		}
	}
	if upd.AcceptedAt != nil {
		updates["accepted_at"] = *upd.AcceptedAt
	}
	if upd.InvitedUserID != nil {
		updates["invited_user_id"] = *upd.InvitedUserID
	}
	if upd.DeclineReason != "" {
		updates["decline_reason"] = upd.DeclineReason
	}
	if upd.RevokeReason != "" {
		updates["revoke_reason"] = upd.RevokeReason
	}
	if upd.LastDeliveryStatus != "" {
		updates["last_delivery_status"] = upd.LastDeliveryStatus
	}
	if upd.LastDeliveryAttemptedAt != nil {
		updates["last_delivery_attempted_at"] = *upd.LastDeliveryAttemptedAt
	}
	if upd.UpdatedAt.IsZero() {
		updates["updated_at"] = time.Now().UTC()
	} else {
		updates["updated_at"] = upd.UpdatedAt
	}
	if len(updates) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&invitationRecord{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update invitation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// AtomicTransitionFromPending is the linchpin of concurrent-accept safety:
// it flips status from 'pending' to the target value iff token_hash still
// matches and the row is still pending AND unexpired. Returns ErrNotFound
// when the row has already been processed by a competing writer.
func (r *InvitationRepository) AtomicTransitionFromPending(
	ctx context.Context,
	id uuid.UUID,
	tokenHash string,
	newStatus string,
	now time.Time,
	invitedUserID *uuid.UUID,
	acceptedAt *time.Time,
	declineReason string,
	revokeReason string,
) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"status":     newStatus,
		"token_hash": nil,
		"updated_at": now,
	}
	if invitedUserID != nil {
		updates["invited_user_id"] = *invitedUserID
	}
	if acceptedAt != nil {
		updates["accepted_at"] = *acceptedAt
	}
	if declineReason != "" {
		updates["decline_reason"] = declineReason
	}
	if revokeReason != "" {
		updates["revoke_reason"] = revokeReason
	}

	q := r.db.WithContext(ctx).
		Model(&invitationRecord{}).
		Where("id = ? AND status = ?", id, model.InvitationStatusPending)
	if tokenHash != "" {
		q = q.Where("token_hash = ?", tokenHash)
	}
	// Accept/decline paths also require expires_at > now; revoke does not.
	if newStatus == model.InvitationStatusAccepted || newStatus == model.InvitationStatusDeclined {
		q = q.Where("expires_at > ?", now)
	}

	result := q.Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("atomic transition invitation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkExpired flips up to `limit` pending invitations whose expires_at is
// in the past into the expired terminal state. Returns the number of rows
// touched so the scheduler can report progress.
func (r *InvitationRepository) MarkExpired(ctx context.Context, now time.Time, limit int) (int64, error) {
	if r.db == nil {
		return 0, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 500
	}
	// Subquery pattern: select the ids to update, then update them in one
	// statement. Keeps the batch size capped without platform-specific SQL.
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&invitationRecord{}).
		Where("status = ? AND expires_at < ?", model.InvitationStatusPending, now).
		Order("expires_at").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, fmt.Errorf("select expired invitations: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Model(&invitationRecord{}).
		Where("id IN ? AND status = ?", ids, model.InvitationStatusPending).
		Updates(map[string]any{
			"status":     model.InvitationStatusExpired,
			"token_hash": nil,
			"updated_at": now,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("mark invitations expired: %w", result.Error)
	}
	return result.RowsAffected, nil
}
