// Package model — invitation.go declares the `project_invitations` aggregate.
// See openspec/specs/member-invitation-flow/spec.md for the state machine.
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Invitation status enum. Must stay in lockstep with the DB CHECK
// constraint in migration 061.
const (
	InvitationStatusPending  = "pending"
	InvitationStatusAccepted = "accepted"
	InvitationStatusDeclined = "declined"
	InvitationStatusExpired  = "expired"
	InvitationStatusRevoked  = "revoked"
)

// IsValidInvitationStatus reports whether v is one of the canonical values.
func IsValidInvitationStatus(v string) bool {
	switch v {
	case InvitationStatusPending, InvitationStatusAccepted, InvitationStatusDeclined,
		InvitationStatusExpired, InvitationStatusRevoked:
		return true
	}
	return false
}

// IsTerminalInvitationStatus reports whether v is a terminal state — any
// transition out of a terminal state is rejected.
func IsTerminalInvitationStatus(v string) bool {
	switch v {
	case InvitationStatusAccepted, InvitationStatusDeclined,
		InvitationStatusExpired, InvitationStatusRevoked:
		return true
	}
	return false
}

// InvitedIdentityKind enumerates the supported identity shapes.
const (
	InvitedIdentityKindEmail = "email"
	InvitedIdentityKindIM    = "im"
)

// ErrInvalidInvitedIdentity is returned by ParseInvitedIdentity when the
// JSON blob cannot be coerced into a known shape.
var ErrInvalidInvitedIdentity = errors.New("invalid invited_identity")

// InvitedIdentity is the typed form of the `invited_identity` JSON column.
//
// Exactly one of {Email} or {Platform, UserID} must be populated; other
// fields depend on kind.
type InvitedIdentity struct {
	Kind string `json:"kind"`

	// Email-kind fields.
	Email string `json:"value,omitempty"`

	// IM-kind fields.
	Platform    string `json:"platform,omitempty"`
	UserID      string `json:"userId,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// Validate enforces the shape rules described on the type.
func (i InvitedIdentity) Validate() error {
	switch i.Kind {
	case InvitedIdentityKindEmail:
		if strings.TrimSpace(i.Email) == "" {
			return fmt.Errorf("%w: email value required", ErrInvalidInvitedIdentity)
		}
	case InvitedIdentityKindIM:
		if strings.TrimSpace(i.Platform) == "" || strings.TrimSpace(i.UserID) == "" {
			return fmt.Errorf("%w: im identity requires platform and userId", ErrInvalidInvitedIdentity)
		}
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidInvitedIdentity, i.Kind)
	}
	return nil
}

// Canonical returns a normalized form of the identity used for uniqueness
// comparisons (case-insensitive email, trimmed IM tuple).
func (i InvitedIdentity) Canonical() InvitedIdentity {
	switch i.Kind {
	case InvitedIdentityKindEmail:
		return InvitedIdentity{
			Kind:  InvitedIdentityKindEmail,
			Email: strings.ToLower(strings.TrimSpace(i.Email)),
		}
	case InvitedIdentityKindIM:
		return InvitedIdentity{
			Kind:        InvitedIdentityKindIM,
			Platform:    strings.TrimSpace(i.Platform),
			UserID:      strings.TrimSpace(i.UserID),
			DisplayName: i.DisplayName,
		}
	}
	return i
}

// JSON serialises the identity into the shape persisted in the JSONB column.
func (i InvitedIdentity) JSON() string {
	b, err := json.Marshal(i.Canonical())
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ParseInvitedIdentity parses a JSON payload into an InvitedIdentity and
// validates it. Empty / blank input returns ErrInvalidInvitedIdentity.
func ParseInvitedIdentity(raw string) (InvitedIdentity, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return InvitedIdentity{}, fmt.Errorf("%w: empty payload", ErrInvalidInvitedIdentity)
	}
	var id InvitedIdentity
	if err := json.Unmarshal([]byte(raw), &id); err != nil {
		return InvitedIdentity{}, fmt.Errorf("%w: %v", ErrInvalidInvitedIdentity, err)
	}
	if err := id.Validate(); err != nil {
		return InvitedIdentity{}, err
	}
	return id.Canonical(), nil
}

// Invitation is the domain aggregate for a pending-or-resolved invitation
// to a project. One invitation materialises into zero or one `member` rows.
type Invitation struct {
	ID                       uuid.UUID       `db:"id"`
	ProjectID                uuid.UUID       `db:"project_id"`
	InviterUserID            uuid.UUID       `db:"inviter_user_id"`
	InvitedIdentity          InvitedIdentity `db:"invited_identity"`
	InvitedUserID            *uuid.UUID      `db:"invited_user_id"`
	ProjectRole              string          `db:"project_role"`
	Status                   string          `db:"status"`
	TokenHash                string          `db:"token_hash"`
	Message                  string          `db:"message"`
	ExpiresAt                time.Time       `db:"expires_at"`
	CreatedAt                time.Time       `db:"created_at"`
	UpdatedAt                time.Time       `db:"updated_at"`
	AcceptedAt               *time.Time      `db:"accepted_at"`
	DeclineReason            string          `db:"decline_reason"`
	RevokeReason             string          `db:"revoke_reason"`
	LastDeliveryStatus       string          `db:"last_delivery_status"`
	LastDeliveryAttemptedAt  *time.Time      `db:"last_delivery_attempted_at"`
}

// InvitationDTO is the JSON shape returned to API consumers. Sensitive
// fields (token hash, raw token) are never surfaced here; the plaintext
// `AcceptToken` only appears in the Create response via InvitationCreateResponse.
type InvitationDTO struct {
	ID                      string                 `json:"id"`
	ProjectID               string                 `json:"projectId"`
	InviterUserID           string                 `json:"inviterUserId"`
	InvitedIdentity         InvitedIdentity        `json:"invitedIdentity"`
	InvitedUserID           *string                `json:"invitedUserId,omitempty"`
	ProjectRole             string                 `json:"projectRole"`
	Status                  string                 `json:"status"`
	Message                 string                 `json:"message,omitempty"`
	ExpiresAt               string                 `json:"expiresAt"`
	CreatedAt               string                 `json:"createdAt"`
	UpdatedAt               string                 `json:"updatedAt"`
	AcceptedAt              *string                `json:"acceptedAt,omitempty"`
	DeclineReason           string                 `json:"declineReason,omitempty"`
	RevokeReason            string                 `json:"revokeReason,omitempty"`
	LastDeliveryStatus      string                 `json:"lastDeliveryStatus,omitempty"`
	LastDeliveryAttemptedAt *string                `json:"lastDeliveryAttemptedAt,omitempty"`
}

func (i *Invitation) ToDTO() InvitationDTO {
	dto := InvitationDTO{
		ID:                 i.ID.String(),
		ProjectID:          i.ProjectID.String(),
		InviterUserID:      i.InviterUserID.String(),
		InvitedIdentity:    i.InvitedIdentity,
		ProjectRole:        NormalizeProjectRole(i.ProjectRole),
		Status:             i.Status,
		Message:            i.Message,
		ExpiresAt:          i.ExpiresAt.UTC().Format(time.RFC3339),
		CreatedAt:          i.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          i.UpdatedAt.UTC().Format(time.RFC3339),
		DeclineReason:      i.DeclineReason,
		RevokeReason:       i.RevokeReason,
		LastDeliveryStatus: i.LastDeliveryStatus,
	}
	if i.InvitedUserID != nil {
		s := i.InvitedUserID.String()
		dto.InvitedUserID = &s
	}
	if i.AcceptedAt != nil {
		s := i.AcceptedAt.UTC().Format(time.RFC3339)
		dto.AcceptedAt = &s
	}
	if i.LastDeliveryAttemptedAt != nil {
		s := i.LastDeliveryAttemptedAt.UTC().Format(time.RFC3339)
		dto.LastDeliveryAttemptedAt = &s
	}
	return dto
}

// InvitationPublicPreview is the narrow payload returned from
// `GET /invitations/by-token/:token`. It intentionally excludes the
// project UUID, inviter UUID, and anything that could help guess other
// invitations; only data the invitee needs for the "do I accept?" decision.
type InvitationPublicPreview struct {
	ProjectName    string `json:"projectName"`
	ProjectRole    string `json:"projectRole"`
	InviterName    string `json:"inviterName,omitempty"`
	InviterEmail   string `json:"inviterEmail,omitempty"`
	Message        string `json:"message,omitempty"`
	ExpiresAt      string `json:"expiresAt"`
	Status         string `json:"status"`
	IdentityKind   string `json:"identityKind"`
	IdentityHint   string `json:"identityHint,omitempty"`
}

// CreateInvitationRequest is the body of `POST /projects/:pid/invitations`.
type CreateInvitationRequest struct {
	InvitedIdentity InvitedIdentity `json:"invitedIdentity" validate:"required"`
	ProjectRole     string          `json:"projectRole" validate:"required,oneof=owner admin editor viewer"`
	Message         string          `json:"message" validate:"omitempty,max=1000"`
	// ExpiresAt is an optional explicit override. If empty, server defaults
	// to now + DefaultInvitationExpiry. Clamped to [MinInvitationExpiry,
	// MaxInvitationExpiry] range.
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// InvitationCreateResponse is the 201 response; `acceptToken` and
// `acceptURL` are ONLY present here so the admin can manually copy them
// when automatic delivery fails. Subsequent reads never include them.
type InvitationCreateResponse struct {
	Invitation  InvitationDTO `json:"invitation"`
	AcceptToken string        `json:"acceptToken"`
	AcceptURL   string        `json:"acceptUrl"`
}

// AcceptInvitationRequest is the body of `POST /invitations/accept`.
type AcceptInvitationRequest struct {
	Token string `json:"token" validate:"required"`
}

// DeclineInvitationRequest is the body of `POST /invitations/decline`.
type DeclineInvitationRequest struct {
	Token  string `json:"token" validate:"required"`
	Reason string `json:"reason" validate:"omitempty,max=500"`
}

// RevokeInvitationRequest is the body of `POST /projects/:pid/invitations/:id/revoke`.
type RevokeInvitationRequest struct {
	Reason string `json:"reason" validate:"omitempty,max=500"`
}

// Invitation expiry bounds.
const (
	DefaultInvitationExpiry = 7 * 24 * time.Hour
	MinInvitationExpiry     = time.Hour
	MaxInvitationExpiry     = 30 * 24 * time.Hour
)

// ClampInvitationExpiry returns a valid expiry duration within bounds.
// An input of zero or below returns the default.
func ClampInvitationExpiry(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultInvitationExpiry
	}
	if d < MinInvitationExpiry {
		return MinInvitationExpiry
	}
	if d > MaxInvitationExpiry {
		return MaxInvitationExpiry
	}
	return d
}
