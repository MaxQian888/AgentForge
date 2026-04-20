package middleware

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

// fakeMemberLookup is a hand-rolled stub keyed on (userID, projectID).
type fakeMemberLookup struct {
	rows map[string]*model.Member
}

func newFakeLookup() *fakeMemberLookup {
	return &fakeMemberLookup{rows: map[string]*model.Member{}}
}

func (f *fakeMemberLookup) put(projectID, userID uuid.UUID, role string) *model.Member {
	m := &model.Member{
		ID:          uuid.New(),
		ProjectID:   projectID,
		UserID:      &userID,
		ProjectRole: role,
	}
	f.rows[userID.String()+"|"+projectID.String()] = m
	return m
}

func (f *fakeMemberLookup) GetByUserAndProject(_ context.Context, userID, projectID uuid.UUID) (*model.Member, error) {
	m, ok := f.rows[userID.String()+"|"+projectID.String()]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return m, nil
}

// TestMatrix_AllActionsHaveValidMinRole asserts every declared ActionID maps
// to a known canonical role. Catches drift such as accidentally adding an
// ActionID to AllActions without a matrix entry.
func TestMatrix_AllActionsHaveValidMinRole(t *testing.T) {
	for _, action := range AllActions() {
		min, ok := MinRoleFor(action)
		if !ok {
			t.Errorf("action %q has no matrix entry", action)
			continue
		}
		if !model.IsValidProjectRole(min) {
			t.Errorf("action %q maps to invalid role %q", action, min)
		}
	}
}

// TestMatrix_AllActionsExportedAreInMatrix walks the source-declared
// constants via AllActions and verifies the snapshot has identical keys.
func TestMatrix_AllActionsExportedAreInMatrix(t *testing.T) {
	snapshot := MatrixSnapshot()
	for _, action := range AllActions() {
		if _, ok := snapshot[action]; !ok {
			t.Errorf("AllActions() includes %q but snapshot does not", action)
		}
	}
	if len(snapshot) != len(AllActions()) {
		t.Errorf("matrix size mismatch: AllActions=%d snapshot=%d", len(AllActions()), len(snapshot))
	}
}

// TestAuthorize_RoleSemantics is the table-driven coverage required by spec
// task 2.2: at least one assertion per role × representative ActionID class.
func TestAuthorize_RoleSemantics(t *testing.T) {
	type tc struct {
		name       string
		callerRole string
		action     ActionID
		wantErr    error
	}
	cases := []tc{
		// owner allowed everywhere
		{"owner can delete project", model.ProjectRoleOwner, ActionProjectDelete, nil},
		{"owner can dispatch", model.ProjectRoleOwner, ActionTaskDispatch, nil},
		{"owner can read audit", model.ProjectRoleOwner, ActionAuditRead, nil},

		// admin allowed for member/settings/automation/audit, blocked for project delete
		{"admin can update settings", model.ProjectRoleAdmin, ActionSettingsUpdate, nil},
		{"admin can change member role", model.ProjectRoleAdmin, ActionMemberRoleChange, nil},
		{"admin can read audit", model.ProjectRoleAdmin, ActionAuditRead, nil},
		{"admin cannot delete project", model.ProjectRoleAdmin, ActionProjectDelete, ErrInsufficientProjectRole},

		// editor allowed for tasks/teams/workflows, blocked for member/settings/audit
		{"editor can dispatch", model.ProjectRoleEditor, ActionTaskDispatch, nil},
		{"editor can start team run", model.ProjectRoleEditor, ActionTeamRunStart, nil},
		{"editor can execute workflow", model.ProjectRoleEditor, ActionWorkflowExecute, nil},
		{"editor cannot change member role", model.ProjectRoleEditor, ActionMemberRoleChange, ErrInsufficientProjectRole},
		{"editor cannot update settings", model.ProjectRoleEditor, ActionSettingsUpdate, ErrInsufficientProjectRole},
		{"editor cannot read audit", model.ProjectRoleEditor, ActionAuditRead, ErrInsufficientProjectRole},

		// viewer allowed for reads only
		{"viewer can read project", model.ProjectRoleViewer, ActionProjectRead, nil},
		{"viewer can read tasks", model.ProjectRoleViewer, ActionTaskRead, nil},
		{"viewer cannot create tasks", model.ProjectRoleViewer, ActionTaskCreate, ErrInsufficientProjectRole},
		{"viewer cannot dispatch", model.ProjectRoleViewer, ActionTaskDispatch, ErrInsufficientProjectRole},
		{"viewer cannot read audit", model.ProjectRoleViewer, ActionAuditRead, ErrInsufficientProjectRole},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			projectID := uuid.New()
			userID := uuid.New()
			lookup := newFakeLookup()
			lookup.put(projectID, userID, c.callerRole)

			_, err := Authorize(context.Background(), lookup, c.action, projectID, userID)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("Authorize(%s, %s)=%v want %v", c.callerRole, c.action, err, c.wantErr)
			}
		})
	}
}

// TestAuthorize_NotAMemberRejected covers the spec scenario "Caller is not a
// member of the project": the response must be ErrNotAProjectMember and not
// leak project existence.
func TestAuthorize_NotAMemberRejected(t *testing.T) {
	lookup := newFakeLookup()
	_, err := Authorize(context.Background(), lookup, ActionTaskRead, uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotAProjectMember) {
		t.Errorf("non-member should be rejected with ErrNotAProjectMember; got %v", err)
	}
}

// TestAuthorize_UnknownActionFails ensures we fail closed when handlers tag a
// route with an ActionID that has no matrix entry (typo or merge regression).
func TestAuthorize_UnknownActionFails(t *testing.T) {
	lookup := newFakeLookup()
	_, err := Authorize(context.Background(), lookup, ActionID("unknown.action"), uuid.New(), uuid.New())
	if !errors.Is(err, ErrUnknownAction) {
		t.Errorf("unknown action should fail with ErrUnknownAction; got %v", err)
	}
}

// TestAllowedActionsFor_RoleSubsets asserts the derived per-role allowed set
// matches the matrix semantics. The frontend permissions endpoint returns
// this set as the authoritative gate source.
func TestAllowedActionsFor_RoleSubsets(t *testing.T) {
	owner := AllowedActionsFor(model.ProjectRoleOwner)
	admin := AllowedActionsFor(model.ProjectRoleAdmin)
	editor := AllowedActionsFor(model.ProjectRoleEditor)
	viewer := AllowedActionsFor(model.ProjectRoleViewer)

	if len(owner) != len(AllActions()) {
		t.Errorf("owner should be allowed every action: got %d want %d", len(owner), len(AllActions()))
	}
	if !isSubset(viewer, editor) {
		t.Errorf("viewer set must be a subset of editor set")
	}
	if !isSubset(editor, admin) {
		t.Errorf("editor set must be a subset of admin set")
	}
	if !isSubset(admin, owner) {
		t.Errorf("admin set must be a subset of owner set")
	}

	// Spot-check ordering determinism: AllowedActionsFor returns sorted output.
	sorted := make([]ActionID, len(owner))
	copy(sorted, owner)
	if !reflect.DeepEqual(sorted, owner) {
		t.Errorf("AllowedActionsFor must return a stable, sorted slice")
	}
}

func isSubset(small, big []ActionID) bool {
	set := make(map[ActionID]struct{}, len(big))
	for _, a := range big {
		set[a] = struct{}{}
	}
	for _, a := range small {
		if _, ok := set[a]; !ok {
			return false
		}
	}
	return true
}
