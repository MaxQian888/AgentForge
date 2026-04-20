// Package service — invitation_service.go is the state-machine owner for
// `project_invitations`. See openspec/specs/member-invitation-flow/spec.md.
//
// Transitions:
//
//	pending ──► accepted   (accept path, identity matches, unexpired)
//	pending ──► declined   (decline path, token-only)
//	pending ──► revoked    (admin revoke)
//	pending ──► expired    (sweeper or accept-time check)
//
// All non-pending states are terminal. The token_hash is nulled on every
// terminal transition so a leaked token becomes inert after first use.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

// Sentinel errors surface the domain-level failure modes the handler maps
// to HTTP status codes.
var (
	ErrInvitationNotFound               = errors.New("invitation: not found")
	ErrInvitationAlreadyPendingForIdent = errors.New("invitation: a pending invitation already exists for this identity")
	ErrInvitationAlreadyProcessed       = errors.New("invitation: already processed")
	ErrInvitationExpired                = errors.New("invitation: expired")
	ErrInvitationIdentityMismatch       = errors.New("invitation: caller identity does not match invited identity")
	ErrInvitationInvalidIdentity        = errors.New("invitation: invalid invited identity")
	ErrInvitationInvalidRole            = errors.New("invitation: invalid project role")
	ErrInvitationInvalidTransition      = errors.New("invitation: invalid state transition")
)

// InvitationRepoContract is the narrow repository contract the service uses.
// A minimal interface keeps tests ergonomic without pulling in the full repo.
type InvitationRepoContract interface {
	Create(ctx context.Context, invitation *model.Invitation) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error)
	FindByTokenHash(ctx context.Context, tokenHash string) (*model.Invitation, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, filter repository.InvitationListFilter) ([]*model.Invitation, error)
	ExistsPendingForIdentity(ctx context.Context, projectID uuid.UUID, identity model.InvitedIdentity) (bool, error)
	Update(ctx context.Context, id uuid.UUID, upd repository.InvitationStatusUpdate) error
	AtomicTransitionFromPending(
		ctx context.Context,
		id uuid.UUID,
		tokenHash string,
		newStatus string,
		now time.Time,
		invitedUserID *uuid.UUID,
		acceptedAt *time.Time,
		declineReason string,
		revokeReason string,
	) error
	MarkExpired(ctx context.Context, now time.Time, limit int) (int64, error)
}

// MemberContract is the subset of MemberRepository the invitation service
// calls into for materialisation + IM identity checks.
type MemberContract interface {
	Create(ctx context.Context, member *model.Member) error
	GetByUserAndProject(ctx context.Context, userID, projectID uuid.UUID) (*model.Member, error)
	HasIMIdentity(ctx context.Context, userID uuid.UUID, platform, imUserID string) (bool, error)
}

// UserLookup resolves a caller's email for identity matching. Implemented
// by *repository.UserRepository.
type UserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

// ProjectLookup fetches minimum project metadata for invitation previews.
type ProjectLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
}

// InvitationAuditEmitter is the contract the service uses to push audit
// events for invitation state transitions. A nil emitter is permitted —
// the service then skips emission (useful in tests before audit is wired).
type InvitationAuditEmitter interface {
	RecordEvent(ctx context.Context, event *model.AuditEvent) error
}

// InvitationDelivery is the async delivery-dispatcher contract. Returned
// errors are recorded on the invitation row but do NOT fail Create.
type InvitationDelivery interface {
	Deliver(ctx context.Context, invitation *model.Invitation, plaintextToken string)
}

// InvitationService bundles state-machine transitions + delivery handoff.
type InvitationService struct {
	invitations InvitationRepoContract
	members     MemberContract
	users       UserLookup
	projects    ProjectLookup
	audit       InvitationAuditEmitter
	delivery    InvitationDelivery
	acceptURL   string // e.g. "https://app/invitations/accept"
	now         func() time.Time
}

// InvitationServiceConfig captures construction-time options.
type InvitationServiceConfig struct {
	AcceptURLBase string // used to build the plaintext accept URL returned on create
	Now           func() time.Time
}

// NewInvitationService constructs the service. members, invitations, users
// and projects are required; audit / delivery are optional.
func NewInvitationService(
	invitations InvitationRepoContract,
	members MemberContract,
	users UserLookup,
	projects ProjectLookup,
	cfg InvitationServiceConfig,
) *InvitationService {
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	acceptURL := cfg.AcceptURLBase
	if acceptURL == "" {
		acceptURL = "/invitations/accept"
	}
	return &InvitationService{
		invitations: invitations,
		members:     members,
		users:       users,
		projects:    projects,
		acceptURL:   acceptURL,
		now:         now,
	}
}

// WithAuditEmitter wires an audit emitter for state transitions.
func (s *InvitationService) WithAuditEmitter(emitter InvitationAuditEmitter) *InvitationService {
	s.audit = emitter
	return s
}

// WithDelivery wires the async delivery dispatcher.
func (s *InvitationService) WithDelivery(d InvitationDelivery) *InvitationService {
	s.delivery = d
	return s
}

// CreateInput is the user-facing form of create arguments, pre-validated.
type CreateInput struct {
	ProjectID       uuid.UUID
	InviterUserID   uuid.UUID
	InvitedIdentity model.InvitedIdentity
	ProjectRole     string
	Message         string
	ExpiresAt       time.Time // zero => default
	RequestID       string
	IP              string
	UserAgent       string
}

// CreateResult bundles the persisted invitation with the plaintext token
// and the admin-visible accept URL. The plaintext is ONLY produced here
// and must not be surfaced on any other path.
type CreateResult struct {
	Invitation     *model.Invitation
	PlaintextToken string
	AcceptURL      string
}

// Create issues a new pending invitation for the given project + identity.
func (s *InvitationService) Create(ctx context.Context, in CreateInput) (*CreateResult, error) {
	if err := in.InvitedIdentity.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvitationInvalidIdentity, err)
	}
	if !model.IsValidProjectRole(in.ProjectRole) {
		return nil, ErrInvitationInvalidRole
	}
	identity := in.InvitedIdentity.Canonical()

	exists, err := s.invitations.ExistsPendingForIdentity(ctx, in.ProjectID, identity)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrInvitationAlreadyPendingForIdent
	}

	now := s.now()
	expiresAt := in.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = now.Add(model.DefaultInvitationExpiry)
	} else {
		d := expiresAt.Sub(now)
		d = model.ClampInvitationExpiry(d)
		expiresAt = now.Add(d)
	}

	plaintext, hash, err := generateInvitationToken()
	if err != nil {
		return nil, fmt.Errorf("generate invitation token: %w", err)
	}

	invitation := &model.Invitation{
		ID:              uuid.New(),
		ProjectID:       in.ProjectID,
		InviterUserID:   in.InviterUserID,
		InvitedIdentity: identity,
		ProjectRole:     model.NormalizeProjectRole(in.ProjectRole),
		Status:          model.InvitationStatusPending,
		TokenHash:       hash,
		Message:         strings.TrimSpace(in.Message),
		ExpiresAt:       expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.invitations.Create(ctx, invitation); err != nil {
		return nil, err
	}

	s.emitAudit(ctx, invitation, "invitation.create", &in.InviterUserID, in.RequestID, in.IP, in.UserAgent, nil)

	if s.delivery != nil {
		s.delivery.Deliver(ctx, invitation, plaintext)
	}

	return &CreateResult{
		Invitation:     invitation,
		PlaintextToken: plaintext,
		AcceptURL:      s.buildAcceptURL(plaintext),
	}, nil
}

// List returns invitations for a project, filtered optionally by status.
func (s *InvitationService) List(ctx context.Context, projectID uuid.UUID, status string) ([]*model.Invitation, error) {
	filter := repository.InvitationListFilter{}
	if status != "" {
		if !model.IsValidInvitationStatus(status) {
			return nil, fmt.Errorf("invitation: invalid status filter %q", status)
		}
		filter.Status = status
	}
	return s.invitations.ListByProject(ctx, projectID, filter)
}

// Revoke transitions a pending invitation to revoked. Target must exist
// and be pending.
func (s *InvitationService) Revoke(ctx context.Context, invitationID uuid.UUID, actorUserID uuid.UUID, reason, requestID, ip, userAgent string) (*model.Invitation, error) {
	invitation, err := s.invitations.GetByID(ctx, invitationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	if invitation.Status != model.InvitationStatusPending {
		return nil, ErrInvitationAlreadyProcessed
	}
	now := s.now()
	if err := s.invitations.AtomicTransitionFromPending(
		ctx,
		invitation.ID,
		"", // revoke does not validate token
		model.InvitationStatusRevoked,
		now,
		nil,
		nil,
		"",
		reason,
	); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvitationAlreadyProcessed
		}
		return nil, err
	}
	invitation.Status = model.InvitationStatusRevoked
	invitation.TokenHash = ""
	invitation.RevokeReason = reason
	invitation.UpdatedAt = now

	s.emitAudit(ctx, invitation, "invitation.revoke", &actorUserID, requestID, ip, userAgent, map[string]string{"reason": reason})
	return invitation, nil
}

// Resend re-enqueues delivery for a pending invitation without rotating
// the token. The service cannot recover the plaintext (only a hash is
// stored), so it hands delivery a nil plaintext; the delivery
// implementation must tolerate this by falling back to a manual-copy
// warning or regenerating a preview link it can reconstruct.
func (s *InvitationService) Resend(ctx context.Context, invitationID uuid.UUID, actorUserID uuid.UUID, requestID, ip, userAgent string) (*model.Invitation, error) {
	invitation, err := s.invitations.GetByID(ctx, invitationID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	if invitation.Status != model.InvitationStatusPending {
		return nil, ErrInvitationAlreadyProcessed
	}
	now := s.now()
	attempt := now
	if err := s.invitations.Update(ctx, invitation.ID, repository.InvitationStatusUpdate{
		LastDeliveryStatus:      "resend_requested",
		LastDeliveryAttemptedAt: &attempt,
		UpdatedAt:               now,
	}); err != nil {
		return nil, err
	}
	invitation.LastDeliveryStatus = "resend_requested"
	invitation.LastDeliveryAttemptedAt = &attempt
	invitation.UpdatedAt = now

	if s.delivery != nil {
		// Plaintext is not available on resend; delivery layer must handle
		// the nil-token case by sending a redirect or a manual-copy notice.
		s.delivery.Deliver(ctx, invitation, "")
	}
	s.emitAudit(ctx, invitation, "invitation.resend", &actorUserID, requestID, ip, userAgent, nil)
	return invitation, nil
}

// PreviewByToken returns the narrow public-preview payload for the
// `GET /invitations/by-token/:token` endpoint. No authentication required.
func (s *InvitationService) PreviewByToken(ctx context.Context, plaintextToken string) (*model.InvitationPublicPreview, *model.Invitation, error) {
	invitation, err := s.resolveByToken(ctx, plaintextToken)
	if err != nil {
		return nil, nil, err
	}
	preview := &model.InvitationPublicPreview{
		ProjectRole:  invitation.ProjectRole,
		Message:      invitation.Message,
		ExpiresAt:    invitation.ExpiresAt.UTC().Format(time.RFC3339),
		Status:       invitation.Status,
		IdentityKind: invitation.InvitedIdentity.Kind,
	}
	// Identity hint: obfuscate email, pass IM displayName if present.
	switch invitation.InvitedIdentity.Kind {
	case model.InvitedIdentityKindEmail:
		preview.IdentityHint = maskEmail(invitation.InvitedIdentity.Email)
	case model.InvitedIdentityKindIM:
		preview.IdentityHint = invitation.InvitedIdentity.DisplayName
	}
	if s.projects != nil {
		if p, err := s.projects.GetByID(ctx, invitation.ProjectID); err == nil && p != nil {
			preview.ProjectName = p.Name
		}
	}
	if s.users != nil {
		if u, err := s.users.GetByID(ctx, invitation.InviterUserID); err == nil && u != nil {
			preview.InviterName = u.Name
			preview.InviterEmail = u.Email
		}
	}
	return preview, invitation, nil
}

// AcceptInput captures the caller context for the accept path.
type AcceptInput struct {
	PlaintextToken string
	CallerUserID   uuid.UUID
	RequestID      string
	IP             string
	UserAgent      string
}

// Accept matches identity + token and materialises a human member.
func (s *InvitationService) Accept(ctx context.Context, in AcceptInput) (*model.Invitation, *model.Member, error) {
	invitation, err := s.resolveByToken(ctx, in.PlaintextToken)
	if err != nil {
		return nil, nil, err
	}
	now := s.now()
	if !invitation.ExpiresAt.After(now) {
		return nil, nil, ErrInvitationExpired
	}

	user, err := s.users.GetByID(ctx, in.CallerUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("load caller user: %w", err)
	}
	matches, err := s.identityMatches(ctx, invitation.InvitedIdentity, user)
	if err != nil {
		return nil, nil, err
	}
	if !matches {
		return nil, nil, ErrInvitationIdentityMismatch
	}

	// If the caller is already a member of this project, re-accepting
	// just returns 200 with the existing member (idempotent no-op).
	existing, err := s.members.GetByUserAndProject(ctx, in.CallerUserID, invitation.ProjectID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, nil, err
	}
	acceptedAt := now
	hash := hashInvitationToken(in.PlaintextToken)
	if err := s.invitations.AtomicTransitionFromPending(
		ctx,
		invitation.ID,
		hash,
		model.InvitationStatusAccepted,
		now,
		&in.CallerUserID,
		&acceptedAt,
		"",
		"",
	); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, ErrInvitationAlreadyProcessed
		}
		return nil, nil, err
	}
	invitation.Status = model.InvitationStatusAccepted
	invitation.InvitedUserID = &in.CallerUserID
	invitation.AcceptedAt = &acceptedAt
	invitation.TokenHash = ""
	invitation.UpdatedAt = now

	member := existing
	if existing == nil {
		member = &model.Member{
			ID:          uuid.New(),
			ProjectID:   invitation.ProjectID,
			UserID:      &in.CallerUserID,
			Name:        userDisplayName(user),
			Type:        model.MemberTypeHuman,
			ProjectRole: invitation.ProjectRole,
			Status:      model.MemberStatusActive,
			Email:       user.Email,
			IsActive:    true,
		}
		if invitation.InvitedIdentity.Kind == model.InvitedIdentityKindIM {
			member.IMPlatform = invitation.InvitedIdentity.Platform
			member.IMUserID = invitation.InvitedIdentity.UserID
			if invitation.InvitedIdentity.DisplayName != "" {
				member.Name = invitation.InvitedIdentity.DisplayName
			}
		}
		if err := s.members.Create(ctx, member); err != nil {
			return nil, nil, fmt.Errorf("materialise member: %w", err)
		}
	}
	s.emitAudit(ctx, invitation, "invitation.accept", &in.CallerUserID, in.RequestID, in.IP, in.UserAgent, nil)
	return invitation, member, nil
}

// DeclineInput captures the decline path arguments. CallerUserID is
// optional (decline can be invoked anonymously — just requires the token).
type DeclineInput struct {
	PlaintextToken string
	CallerUserID   *uuid.UUID
	Reason         string
	RequestID      string
	IP             string
	UserAgent      string
}

// Decline transitions a pending invitation to declined.
func (s *InvitationService) Decline(ctx context.Context, in DeclineInput) (*model.Invitation, error) {
	invitation, err := s.resolveByToken(ctx, in.PlaintextToken)
	if err != nil {
		return nil, err
	}
	now := s.now()
	hash := hashInvitationToken(in.PlaintextToken)
	if err := s.invitations.AtomicTransitionFromPending(
		ctx,
		invitation.ID,
		hash,
		model.InvitationStatusDeclined,
		now,
		nil,
		nil,
		in.Reason,
		"",
	); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvitationAlreadyProcessed
		}
		return nil, err
	}
	invitation.Status = model.InvitationStatusDeclined
	invitation.DeclineReason = in.Reason
	invitation.TokenHash = ""
	invitation.UpdatedAt = now

	s.emitAudit(ctx, invitation, "invitation.decline", in.CallerUserID, in.RequestID, in.IP, in.UserAgent, map[string]string{"reason": in.Reason})
	return invitation, nil
}

// ExpireSweep is the scheduler handler entrypoint.
func (s *InvitationService) ExpireSweep(ctx context.Context) (int64, error) {
	return s.invitations.MarkExpired(ctx, s.now(), 500)
}

// resolveByToken loads the invitation keyed by hashing the plaintext token.
// The only place `plaintext => hash` translation happens.
func (s *InvitationService) resolveByToken(ctx context.Context, plaintext string) (*model.Invitation, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return nil, ErrInvitationNotFound
	}
	hash := hashInvitationToken(plaintext)
	invitation, err := s.invitations.FindByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	return invitation, nil
}

// identityMatches implements the spec's identity-matching rules:
//   - email kind: case-insensitive compare against user's primary email.
//   - im kind: any member row bound to (platform, userId) for this user.
func (s *InvitationService) identityMatches(ctx context.Context, identity model.InvitedIdentity, user *model.User) (bool, error) {
	if user == nil {
		return false, nil
	}
	switch identity.Kind {
	case model.InvitedIdentityKindEmail:
		return strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(identity.Email)), nil
	case model.InvitedIdentityKindIM:
		return s.members.HasIMIdentity(ctx, user.ID, identity.Platform, identity.UserID)
	}
	return false, nil
}

func (s *InvitationService) buildAcceptURL(plaintext string) string {
	base := strings.TrimRight(s.acceptURL, "/")
	return fmt.Sprintf("%s?token=%s", base, plaintext)
}

func (s *InvitationService) emitAudit(
	ctx context.Context,
	invitation *model.Invitation,
	actionID string,
	actorUserID *uuid.UUID,
	requestID, ip, userAgent string,
	extraPayload map[string]string,
) {
	if s.audit == nil {
		return
	}
	payload := map[string]string{
		"invitationId": invitation.ID.String(),
		"projectRole":  invitation.ProjectRole,
		"status":       invitation.Status,
		"identityKind": invitation.InvitedIdentity.Kind,
	}
	for k, v := range extraPayload {
		if v != "" {
			payload[k] = v
		}
	}
	event := &model.AuditEvent{
		ProjectID:           invitation.ProjectID,
		OccurredAt:          s.now(),
		ActorUserID:         actorUserID,
		ActionID:            actionID,
		ResourceType:        model.AuditResourceTypeInvitation,
		ResourceID:          invitation.ID.String(),
		PayloadSnapshotJSON: marshalAuditPayload(payload),
		RequestID:           requestID,
		IP:                  ip,
		UserAgent:           userAgent,
	}
	_ = s.audit.RecordEvent(ctx, event)
}

func generateInvitationToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plaintext := hex.EncodeToString(buf)
	return plaintext, hashInvitationToken(plaintext), nil
}

func hashInvitationToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 1 {
		return email
	}
	local := email[:at]
	if len(local) <= 2 {
		return local[:1] + "***" + email[at:]
	}
	return local[:1] + "***" + local[len(local)-1:] + email[at:]
}

func userDisplayName(user *model.User) string {
	if user == nil {
		return ""
	}
	if strings.TrimSpace(user.Name) != "" {
		return user.Name
	}
	return user.Email
}

// marshalAuditPayload produces a deterministic JSON rendering of the
// supplied key/value map. Kept local so the service does not take a
// transitive dependency on encoding/json mesh with the sanitizer.
func marshalAuditPayload(payload map[string]string) string {
	if len(payload) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	// stable order for deterministic audit payload hashing
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(jsonEscape(k))
		b.WriteString(`":"`)
		b.WriteString(jsonEscape(payload[k]))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

func jsonEscape(s string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		`"`, `\"`,
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	)
	return replacer.Replace(s)
}
