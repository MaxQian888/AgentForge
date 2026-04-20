package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

// --- Fakes ---------------------------------------------------------------

type fakeInvitationRepo struct {
	mu          sync.Mutex
	byID        map[uuid.UUID]*model.Invitation
	byTokenHash map[string]uuid.UUID
	createErr   error
}

func newFakeInvitationRepo() *fakeInvitationRepo {
	return &fakeInvitationRepo{
		byID:        map[uuid.UUID]*model.Invitation{},
		byTokenHash: map[string]uuid.UUID{},
	}
}

func (r *fakeInvitationRepo) Create(_ context.Context, invitation *model.Invitation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	clone := *invitation
	r.byID[clone.ID] = &clone
	if clone.TokenHash != "" {
		r.byTokenHash[clone.TokenHash] = clone.ID
	}
	return nil
}

func (r *fakeInvitationRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	inv, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	clone := *inv
	return &clone, nil
}

func (r *fakeInvitationRepo) FindByTokenHash(_ context.Context, hash string) (*model.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byTokenHash[hash]
	if !ok {
		return nil, repository.ErrNotFound
	}
	inv, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	clone := *inv
	return &clone, nil
}

func (r *fakeInvitationRepo) ListByProject(_ context.Context, projectID uuid.UUID, filter repository.InvitationListFilter) ([]*model.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*model.Invitation, 0, len(r.byID))
	for _, inv := range r.byID {
		if inv.ProjectID != projectID {
			continue
		}
		if filter.Status != "" && inv.Status != filter.Status {
			continue
		}
		clone := *inv
		out = append(out, &clone)
	}
	return out, nil
}

func (r *fakeInvitationRepo) ExistsPendingForIdentity(_ context.Context, projectID uuid.UUID, identity model.InvitedIdentity) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, inv := range r.byID {
		if inv.ProjectID != projectID || inv.Status != model.InvitationStatusPending {
			continue
		}
		if inv.InvitedIdentity.Kind != identity.Kind {
			continue
		}
		if identity.Kind == model.InvitedIdentityKindEmail &&
			inv.InvitedIdentity.Email == identity.Email {
			return true, nil
		}
		if identity.Kind == model.InvitedIdentityKindIM &&
			inv.InvitedIdentity.Platform == identity.Platform &&
			inv.InvitedIdentity.UserID == identity.UserID {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeInvitationRepo) Update(_ context.Context, id uuid.UUID, upd repository.InvitationStatusUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inv, ok := r.byID[id]
	if !ok {
		return repository.ErrNotFound
	}
	if upd.Status != "" {
		inv.Status = upd.Status
	}
	if upd.TokenHash != nil {
		inv.TokenHash = *upd.TokenHash
		// Token hash cleared; drop mapping.
		for k, v := range r.byTokenHash {
			if v == id && inv.TokenHash == "" {
				delete(r.byTokenHash, k)
			}
		}
	}
	if upd.AcceptedAt != nil {
		inv.AcceptedAt = upd.AcceptedAt
	}
	if upd.InvitedUserID != nil {
		inv.InvitedUserID = upd.InvitedUserID
	}
	if upd.DeclineReason != "" {
		inv.DeclineReason = upd.DeclineReason
	}
	if upd.RevokeReason != "" {
		inv.RevokeReason = upd.RevokeReason
	}
	if upd.LastDeliveryStatus != "" {
		inv.LastDeliveryStatus = upd.LastDeliveryStatus
	}
	if upd.LastDeliveryAttemptedAt != nil {
		inv.LastDeliveryAttemptedAt = upd.LastDeliveryAttemptedAt
	}
	if !upd.UpdatedAt.IsZero() {
		inv.UpdatedAt = upd.UpdatedAt
	}
	return nil
}

func (r *fakeInvitationRepo) AtomicTransitionFromPending(
	_ context.Context,
	id uuid.UUID,
	tokenHash string,
	newStatus string,
	now time.Time,
	invitedUserID *uuid.UUID,
	acceptedAt *time.Time,
	declineReason string,
	revokeReason string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	inv, ok := r.byID[id]
	if !ok {
		return repository.ErrNotFound
	}
	if inv.Status != model.InvitationStatusPending {
		return repository.ErrNotFound
	}
	if tokenHash != "" && inv.TokenHash != tokenHash {
		return repository.ErrNotFound
	}
	if (newStatus == model.InvitationStatusAccepted || newStatus == model.InvitationStatusDeclined) &&
		!inv.ExpiresAt.After(now) {
		return repository.ErrNotFound
	}
	inv.Status = newStatus
	inv.TokenHash = ""
	inv.UpdatedAt = now
	if invitedUserID != nil {
		inv.InvitedUserID = invitedUserID
	}
	if acceptedAt != nil {
		inv.AcceptedAt = acceptedAt
	}
	if declineReason != "" {
		inv.DeclineReason = declineReason
	}
	if revokeReason != "" {
		inv.RevokeReason = revokeReason
	}
	for k, v := range r.byTokenHash {
		if v == id {
			delete(r.byTokenHash, k)
		}
	}
	return nil
}

func (r *fakeInvitationRepo) MarkExpired(_ context.Context, now time.Time, limit int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, inv := range r.byID {
		if inv.Status != model.InvitationStatusPending {
			continue
		}
		if inv.ExpiresAt.Before(now) {
			inv.Status = model.InvitationStatusExpired
			inv.TokenHash = ""
			inv.UpdatedAt = now
			n++
			for k, v := range r.byTokenHash {
				if v == inv.ID {
					delete(r.byTokenHash, k)
				}
			}
			if limit > 0 && n >= int64(limit) {
				break
			}
		}
	}
	return n, nil
}

type fakeMemberRepo struct {
	mu         sync.Mutex
	byUserProj map[string]*model.Member
	imIdent    map[string]bool
	created    []*model.Member
}

func newFakeMemberRepo() *fakeMemberRepo {
	return &fakeMemberRepo{
		byUserProj: map[string]*model.Member{},
		imIdent:    map[string]bool{},
	}
}

func memberKey(userID, projectID uuid.UUID) string { return userID.String() + "|" + projectID.String() }

func (m *fakeMemberRepo) Create(_ context.Context, member *model.Member) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *member
	m.created = append(m.created, &clone)
	if member.UserID != nil {
		m.byUserProj[memberKey(*member.UserID, member.ProjectID)] = &clone
	}
	return nil
}

func (m *fakeMemberRepo) GetByUserAndProject(_ context.Context, userID, projectID uuid.UUID) (*model.Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.byUserProj[memberKey(userID, projectID)]; ok {
		clone := *v
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

func (m *fakeMemberRepo) HasIMIdentity(_ context.Context, userID uuid.UUID, platform, imUserID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.imIdent[userID.String()+"|"+platform+"|"+imUserID], nil
}

type fakeUserLookup struct {
	users map[uuid.UUID]*model.User
}

func (f *fakeUserLookup) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	if u, ok := f.users[id]; ok {
		clone := *u
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

type fakeProjectLookup struct {
	projects map[uuid.UUID]*model.Project
}

func (f *fakeProjectLookup) GetByID(_ context.Context, id uuid.UUID) (*model.Project, error) {
	if p, ok := f.projects[id]; ok {
		clone := *p
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

// --- helpers -------------------------------------------------------------

func newTestInvitationService(t *testing.T, now time.Time) (
	*InvitationService,
	*fakeInvitationRepo,
	*fakeMemberRepo,
	*fakeUserLookup,
	*fakeProjectLookup,
) {
	t.Helper()
	invRepo := newFakeInvitationRepo()
	memRepo := newFakeMemberRepo()
	userLookup := &fakeUserLookup{users: map[uuid.UUID]*model.User{}}
	projectLookup := &fakeProjectLookup{projects: map[uuid.UUID]*model.Project{}}
	svc := NewInvitationService(
		invRepo,
		memRepo,
		userLookup,
		projectLookup,
		InvitationServiceConfig{
			AcceptURLBase: "http://test/invitations/accept",
			Now:           func() time.Time { return now },
		},
	)
	return svc, invRepo, memRepo, userLookup, projectLookup
}

func TestInvitationService_Create_HappyPath_EmailIdentity(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, repo, _, _, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()

	result, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "X@Example.com"},
		ProjectRole:     model.ProjectRoleEditor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if result.PlaintextToken == "" {
		t.Fatal("plaintext token should be returned once")
	}
	if result.Invitation.Status != model.InvitationStatusPending {
		t.Fatalf("status = %q, want pending", result.Invitation.Status)
	}
	if result.Invitation.InvitedIdentity.Email != "x@example.com" {
		t.Fatalf("email not canonicalised: %q", result.Invitation.InvitedIdentity.Email)
	}
	if result.AcceptURL == "" || !containsSubstring(result.AcceptURL, result.PlaintextToken) {
		t.Fatalf("accept url missing token: %q", result.AcceptURL)
	}
	if len(repo.byID) != 1 {
		t.Fatalf("expected 1 persisted invitation, got %d", len(repo.byID))
	}
}

func TestInvitationService_Create_DuplicatePendingReturns409(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, _, _, _, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()
	input := CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "dup@example.com"},
		ProjectRole:     model.ProjectRoleEditor,
	}
	if _, err := svc.Create(context.Background(), input); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := svc.Create(context.Background(), input); !errors.Is(err, ErrInvitationAlreadyPendingForIdent) {
		t.Fatalf("second create err = %v, want ErrInvitationAlreadyPendingForIdent", err)
	}
}

func TestInvitationService_Accept_IdentityMatchMaterialisesMember(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, _, memRepo, userLookup, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()
	caller := uuid.New()
	userLookup.users[caller] = &model.User{ID: caller, Email: "caller@example.com", Name: "Caller"}

	result, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "caller@example.com"},
		ProjectRole:     model.ProjectRoleAdmin,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	invitation, member, err := svc.Accept(context.Background(), AcceptInput{
		PlaintextToken: result.PlaintextToken,
		CallerUserID:   caller,
	})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if invitation.Status != model.InvitationStatusAccepted {
		t.Fatalf("status = %q, want accepted", invitation.Status)
	}
	if member == nil || member.UserID == nil || *member.UserID != caller {
		t.Fatalf("member not materialised for caller")
	}
	if member.ProjectRole != model.ProjectRoleAdmin {
		t.Fatalf("member role = %q, want admin", member.ProjectRole)
	}
	if len(memRepo.created) != 1 {
		t.Fatalf("expected member create to be called once, got %d", len(memRepo.created))
	}
}

func TestInvitationService_Accept_IdentityMismatchRejected(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, _, _, userLookup, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()
	caller := uuid.New()
	userLookup.users[caller] = &model.User{ID: caller, Email: "other@example.com"}

	result, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "intended@example.com"},
		ProjectRole:     model.ProjectRoleEditor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, _, err = svc.Accept(context.Background(), AcceptInput{
		PlaintextToken: result.PlaintextToken,
		CallerUserID:   caller,
	})
	if !errors.Is(err, ErrInvitationIdentityMismatch) {
		t.Fatalf("accept err = %v, want ErrInvitationIdentityMismatch", err)
	}
}

func TestInvitationService_Accept_ExpiredReturnsExpired(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, invRepo, _, userLookup, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()
	caller := uuid.New()
	userLookup.users[caller] = &model.User{ID: caller, Email: "caller@example.com"}

	result, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "caller@example.com"},
		ProjectRole:     model.ProjectRoleEditor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Simulate expiry by rewinding expires_at (sweeper hasn't run yet).
	for _, inv := range invRepo.byID {
		inv.ExpiresAt = now.Add(-time.Hour)
	}
	_, _, err = svc.Accept(context.Background(), AcceptInput{
		PlaintextToken: result.PlaintextToken,
		CallerUserID:   caller,
	})
	if !errors.Is(err, ErrInvitationExpired) {
		t.Fatalf("accept err = %v, want ErrInvitationExpired", err)
	}
}

func TestInvitationService_ConcurrentAccept_OnlyOneSucceeds(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, _, _, userLookup, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()
	caller := uuid.New()
	userLookup.users[caller] = &model.User{ID: caller, Email: "caller@example.com"}

	result, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "caller@example.com"},
		ProjectRole:     model.ProjectRoleEditor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var firstErr, secondErr error
	var wg sync.WaitGroup
	wg.Add(2)
	input := AcceptInput{PlaintextToken: result.PlaintextToken, CallerUserID: caller}
	go func() {
		defer wg.Done()
		_, _, firstErr = svc.Accept(context.Background(), input)
	}()
	go func() {
		defer wg.Done()
		_, _, secondErr = svc.Accept(context.Background(), input)
	}()
	wg.Wait()

	successes := 0
	losers := 0
	for _, err := range []error{firstErr, secondErr} {
		if err == nil {
			successes++
			continue
		}
		// The losing caller surfaces either AlreadyProcessed (transition
		// raced the winner) or NotFound (token hash was cleared before the
		// losing caller's resolveByToken). Both are valid outcomes — the
		// invariant is "exactly one winner".
		if errors.Is(err, ErrInvitationAlreadyProcessed) || errors.Is(err, ErrInvitationNotFound) {
			losers++
		}
	}
	if successes != 1 || losers != 1 {
		t.Fatalf("expected exactly one success and one loser, got successes=%d losers=%d first=%v second=%v",
			successes, losers, firstErr, secondErr)
	}
}

func TestInvitationService_Revoke_TransitionsPendingOnly(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, _, _, _, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()

	result, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "rev@example.com"},
		ProjectRole:     model.ProjectRoleEditor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	revoked, err := svc.Revoke(context.Background(), result.Invitation.ID, inviter, "no longer needed", "", "", "")
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked.Status != model.InvitationStatusRevoked {
		t.Fatalf("status = %q, want revoked", revoked.Status)
	}
	// Second revoke on terminal row is rejected.
	if _, err := svc.Revoke(context.Background(), result.Invitation.ID, inviter, "", "", "", ""); !errors.Is(err, ErrInvitationAlreadyProcessed) {
		t.Fatalf("second revoke err = %v, want ErrInvitationAlreadyProcessed", err)
	}
}

func TestInvitationService_ExpireSweep_MarksPendingExpired(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, repo, _, _, _ := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()

	res1, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "exp@example.com"},
		ProjectRole:     model.ProjectRoleEditor,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Rewind expiry so the sweeper picks it up.
	repo.byID[res1.Invitation.ID].ExpiresAt = now.Add(-time.Hour)

	expired, err := svc.ExpireSweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expired count = %d, want 1", expired)
	}
	if repo.byID[res1.Invitation.ID].Status != model.InvitationStatusExpired {
		t.Fatalf("status = %q, want expired", repo.byID[res1.Invitation.ID].Status)
	}
}

func TestInvitationService_PreviewByToken_ReturnsPreview(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	svc, _, _, userLookup, projectLookup := newTestInvitationService(t, now)
	projectID := uuid.New()
	inviter := uuid.New()
	userLookup.users[inviter] = &model.User{ID: inviter, Email: "admin@example.com", Name: "Admin"}
	projectLookup.projects[projectID] = &model.Project{ID: projectID, Name: "Shipyard"}

	result, err := svc.Create(context.Background(), CreateInput{
		ProjectID:       projectID,
		InviterUserID:   inviter,
		InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "a@b.com"},
		ProjectRole:     model.ProjectRoleAdmin,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	preview, _, err := svc.PreviewByToken(context.Background(), result.PlaintextToken)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.ProjectName != "Shipyard" {
		t.Fatalf("project name = %q, want Shipyard", preview.ProjectName)
	}
	if preview.InviterName != "Admin" {
		t.Fatalf("inviter name = %q, want Admin", preview.InviterName)
	}
	if preview.IdentityHint == "" {
		t.Fatal("expected masked email hint")
	}
}

// TestInvitationService_StateMachine_TerminalTransitionsRejected walks every
// terminal status and confirms subsequent state-change attempts return
// ErrInvitationAlreadyProcessed rather than silently succeeding.
func TestInvitationService_StateMachine_TerminalTransitionsRejected(t *testing.T) {
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	terminals := []string{
		model.InvitationStatusAccepted,
		model.InvitationStatusDeclined,
		model.InvitationStatusExpired,
		model.InvitationStatusRevoked,
	}
	for _, terminal := range terminals {
		t.Run(terminal, func(t *testing.T) {
			svc, invRepo, _, userLookup, _ := newTestInvitationService(t, now)
			projectID := uuid.New()
			inviter := uuid.New()
			caller := uuid.New()
			userLookup.users[caller] = &model.User{ID: caller, Email: "caller@example.com"}

			result, err := svc.Create(context.Background(), CreateInput{
				ProjectID:       projectID,
				InviterUserID:   inviter,
				InvitedIdentity: model.InvitedIdentity{Kind: model.InvitedIdentityKindEmail, Email: "caller@example.com"},
				ProjectRole:     model.ProjectRoleEditor,
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			id := result.Invitation.ID

			// Drive the row into the terminal status directly via the fake.
			invRepo.byID[id].Status = terminal
			invRepo.byID[id].TokenHash = ""
			// Drop token mapping so re-resolution is impossible.
			for k, v := range invRepo.byTokenHash {
				if v == id {
					delete(invRepo.byTokenHash, k)
				}
			}

			if _, err := svc.Revoke(context.Background(), id, inviter, "", "", "", ""); err == nil {
				t.Fatalf("revoke on %s should fail", terminal)
			}
			if _, _, err := svc.Accept(context.Background(), AcceptInput{
				PlaintextToken: result.PlaintextToken,
				CallerUserID:   caller,
			}); err == nil {
				t.Fatalf("accept on %s should fail", terminal)
			}
			if _, err := svc.Decline(context.Background(), DeclineInput{
				PlaintextToken: result.PlaintextToken,
			}); err == nil {
				t.Fatalf("decline on %s should fail", terminal)
			}
			if _, err := svc.Resend(context.Background(), id, inviter, "", "", ""); err == nil {
				t.Fatalf("resend on %s should fail", terminal)
			}
		})
	}
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
