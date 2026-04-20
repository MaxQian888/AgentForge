package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

// invitationDeliveryUserLookup matches the invitation service's UserLookup
// without forcing a cycle; we locally alias so the delivery package can
// express its dependency cleanly.
type invitationDeliveryUserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
}

// invitationDeliveryProjectLookup is the narrow contract used to fetch the
// project's display name for delivery message bodies.
type invitationDeliveryProjectLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
}

// InvitationNotificationDelivery is the default invitation delivery that
// drops an in-app notification onto the invited user's notification feed
// when the invited identity resolves to an existing account. When it
// cannot resolve a user (or delivery fails), it records a status on the
// invitation row so the admin UI can surface the manual-copy fallback.
type InvitationNotificationDelivery struct {
	notifications *NotificationService
	invitations   *repository.InvitationRepository
	users         invitationDeliveryUserLookup
	projects      invitationDeliveryProjectLookup
	acceptURLBase string
}

// NewInvitationNotificationDelivery wires the default delivery pipeline.
func NewInvitationNotificationDelivery(
	notifications *NotificationService,
	invitations *repository.InvitationRepository,
	users invitationDeliveryUserLookup,
	projects invitationDeliveryProjectLookup,
	acceptURLBase string,
) *InvitationNotificationDelivery {
	return &InvitationNotificationDelivery{
		notifications: notifications,
		invitations:   invitations,
		users:         users,
		projects:      projects,
		acceptURLBase: acceptURLBase,
	}
}

// Deliver implements InvitationDelivery. Failures never bubble up — they
// are recorded on the invitation row so the admin can fall back to the
// "copy accept link" flow surfaced by the Create response.
func (d *InvitationNotificationDelivery) Deliver(ctx context.Context, invitation *model.Invitation, plaintextToken string) {
	if d == nil || invitation == nil {
		return
	}
	status := "dispatched"
	attempt := time.Now().UTC()

	// Resolve the target user when possible. Email identities look up by
	// email; IM identities fall back to invited_user_id when populated.
	var target *model.User
	switch invitation.InvitedIdentity.Kind {
	case model.InvitedIdentityKindEmail:
		if d.users != nil && invitation.InvitedIdentity.Email != "" {
			if u, err := d.users.GetByEmail(ctx, invitation.InvitedIdentity.Email); err == nil {
				target = u
			}
		}
	case model.InvitedIdentityKindIM:
		if invitation.InvitedUserID != nil && d.users != nil {
			if u, err := d.users.GetByID(ctx, *invitation.InvitedUserID); err == nil {
				target = u
			}
		}
	}

	if target == nil {
		status = "manual_copy_required"
	} else if d.notifications != nil {
		projectName := invitation.ProjectID.String()
		if d.projects != nil {
			if p, err := d.projects.GetByID(ctx, invitation.ProjectID); err == nil && p != nil {
				projectName = p.Name
			}
		}
		title := fmt.Sprintf("You have been invited to %s", projectName)
		acceptURL := d.acceptURLBase
		if plaintextToken != "" {
			base := strings.TrimRight(d.acceptURLBase, "/")
			acceptURL = fmt.Sprintf("%s?token=%s", base, plaintextToken)
		}
		body := fmt.Sprintf("You have been invited to join %s as %s. Accept: %s",
			projectName, invitation.ProjectRole, acceptURL)
		if _, err := d.notifications.Create(ctx, target.ID, "invitation", title, body, ""); err != nil {
			status = "failed"
		}
	}

	if d.invitations != nil {
		_ = d.invitations.Update(ctx, invitation.ID, repository.InvitationStatusUpdate{
			LastDeliveryStatus:      status,
			LastDeliveryAttemptedAt: &attempt,
			UpdatedAt:               attempt,
		})
	}
}
