// Package middleware — rbac.go declares the canonical project-scoped action
// taxonomy (ActionID), the central action→minimum-role matrix, and the Echo
// middleware that enforces it.
//
// This is the single source of truth for "who can do what" inside a project.
// Both API routes (via Require) and service-layer agent actions (via Authorize)
// MUST consult this matrix; downstream auditing references the same ActionID
// values so every gated write produces a coherent governance record.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appI18n "github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

// ActionID identifies a project-scoped action whose execution is gated by
// the RBAC matrix. ActionIDs are namespaced "<resource>.<verb>".
type ActionID string

// Canonical project-scoped ActionIDs. Audit log MUST reuse this enum verbatim;
// see openspec/specs/project-access-control + project-audit-log.
const (
	// Project lifecycle.
	ActionProjectRead   ActionID = "project.read"
	ActionProjectUpdate ActionID = "project.update"
	ActionProjectDelete ActionID = "project.delete"

	// Members.
	ActionMemberRead       ActionID = "member.read"
	ActionMemberCreate     ActionID = "member.create"
	ActionMemberUpdate     ActionID = "member.update"
	ActionMemberRoleChange ActionID = "member.role.change"
	ActionMemberDelete     ActionID = "member.delete"
	ActionMemberBulkUpdate ActionID = "member.bulk.update"

	// Invitations (human member onboarding path). Accept/decline are
	// top-level protected routes and use the sentinel ActionInvitationAccept /
	// ActionInvitationDecline only for audit emission; gating is via token +
	// identity matching in the service layer.
	ActionInvitationCreate  ActionID = "invitation.create"
	ActionInvitationView    ActionID = "invitation.view"
	ActionInvitationRevoke  ActionID = "invitation.revoke"
	ActionInvitationResend  ActionID = "invitation.resend"
	ActionInvitationAccept  ActionID = "invitation.accept"
	ActionInvitationDecline ActionID = "invitation.decline"

	// Tasks.
	ActionTaskRead       ActionID = "task.read"
	ActionTaskCreate     ActionID = "task.create"
	ActionTaskUpdate     ActionID = "task.update"
	ActionTaskDelete     ActionID = "task.delete"
	ActionTaskAssign     ActionID = "task.assign"
	ActionTaskTransition ActionID = "task.transition"
	ActionTaskDispatch   ActionID = "task.dispatch"
	ActionTaskComment    ActionID = "task.comment.write"
	ActionTaskDecompose  ActionID = "task.decompose"

	// Team runs.
	ActionTeamRunStart  ActionID = "team.run.start"
	ActionTeamRunRetry  ActionID = "team.run.retry"
	ActionTeamRunCancel ActionID = "team.run.cancel"
	ActionTeamUpdate    ActionID = "team.update"
	ActionTeamDelete    ActionID = "team.delete"

	// Workflows.
	ActionWorkflowRead    ActionID = "workflow.read"
	ActionWorkflowWrite   ActionID = "workflow.write"
	ActionWorkflowExecute ActionID = "workflow.execute"
	ActionWorkflowReview  ActionID = "workflow.review.resolve"
	ActionWorkflowCancel  ActionID = "workflow.execution.cancel"

	// Automation.
	ActionAutomationRead    ActionID = "automation.read"
	ActionAutomationWrite   ActionID = "automation.write"
	ActionAutomationTrigger ActionID = "automation.trigger"

	// Settings & dashboards.
	ActionSettingsRead   ActionID = "settings.read"
	ActionSettingsUpdate ActionID = "settings.update"
	ActionDashboardRead  ActionID = "dashboard.read"
	ActionDashboardWrite ActionID = "dashboard.write"

	// Wiki / Knowledge.
	ActionWikiRead   ActionID = "wiki.read"
	ActionWikiWrite  ActionID = "wiki.write"
	ActionWikiDelete ActionID = "wiki.delete"

	// Memory & logs.
	ActionMemoryRead  ActionID = "memory.read"
	ActionMemoryWrite ActionID = "memory.write"
	ActionLogRead     ActionID = "log.read"
	ActionLogWrite    ActionID = "log.write"

	// Custom fields, saved views, forms, milestones, sprints — editor-class.
	ActionCustomFieldRead  ActionID = "custom_field.read"
	ActionCustomFieldWrite ActionID = "custom_field.write"
	ActionSavedViewWrite   ActionID = "saved_view.write"
	ActionFormWrite        ActionID = "form.write"
	ActionMilestoneWrite   ActionID = "milestone.write"
	ActionSprintWrite      ActionID = "sprint.write"

	// Agent control surfaces (project-scoped; non-gated agent CRUD lives elsewhere).
	ActionAgentSpawn  ActionID = "agent.spawn"
	ActionAgentControl ActionID = "agent.control"

	// Audit log (gates the audit-log query API; see add-project-audit-log spec).
	ActionAuditRead ActionID = "audit.read"

	// Project templates. `save_as_template` is a project-scoped action (gated
	// via projectGroup + Require). The CRUD actions on /project-templates are
	// NOT project-scoped — they apply to a user's personal template library —
	// and are enforced by the handler itself, not this matrix.
	ActionProjectSaveAsTemplate ActionID = "project.save_as_template"

	// Emitted only as an audit marker when a new project is created from a
	// template; not mounted on any Route. Present here so the audit service
	// accepts it without being asked to special-case audit-only actions.
	ActionProjectCreatedFromTemplate ActionID = "project.created_from_template"
)

// matrix maps each known ActionID to the minimum project role that may invoke it.
// Note on viewer: viewer is allowed only against pure read actions where the
// existing API surface already returns project-scoped data without side
// effects. Any action that can produce cost, dispatch, or mutation requires
// editor or higher.
var matrix = func() map[ActionID]string {
	return map[ActionID]string{
		// Project lifecycle.
		ActionProjectRead:   model.ProjectRoleViewer,
		ActionProjectUpdate: model.ProjectRoleAdmin,
		ActionProjectDelete: model.ProjectRoleOwner,

		// Members.
		ActionMemberRead:       model.ProjectRoleViewer,
		ActionMemberCreate:     model.ProjectRoleAdmin,
		ActionMemberUpdate:     model.ProjectRoleAdmin,
		ActionMemberRoleChange: model.ProjectRoleAdmin,
		ActionMemberDelete:     model.ProjectRoleAdmin,
		ActionMemberBulkUpdate: model.ProjectRoleAdmin,

		// Invitations — human onboarding path. All project-scoped
		// operations (create/view/revoke/resend) require admin+.
		// Accept/decline are audit-only sentinels (not gated via Require);
		// slotted at viewer so validation accepts them.
		ActionInvitationCreate:  model.ProjectRoleAdmin,
		ActionInvitationView:    model.ProjectRoleAdmin,
		ActionInvitationRevoke:  model.ProjectRoleAdmin,
		ActionInvitationResend:  model.ProjectRoleAdmin,
		ActionInvitationAccept:  model.ProjectRoleViewer,
		ActionInvitationDecline: model.ProjectRoleViewer,

		// Tasks.
		ActionTaskRead:       model.ProjectRoleViewer,
		ActionTaskCreate:     model.ProjectRoleEditor,
		ActionTaskUpdate:     model.ProjectRoleEditor,
		ActionTaskDelete:     model.ProjectRoleEditor,
		ActionTaskAssign:     model.ProjectRoleEditor,
		ActionTaskTransition: model.ProjectRoleEditor,
		ActionTaskDispatch:   model.ProjectRoleEditor,
		ActionTaskComment:    model.ProjectRoleEditor,
		ActionTaskDecompose:  model.ProjectRoleEditor,

		// Teams.
		ActionTeamRunStart:  model.ProjectRoleEditor,
		ActionTeamRunRetry:  model.ProjectRoleEditor,
		ActionTeamRunCancel: model.ProjectRoleEditor,
		ActionTeamUpdate:    model.ProjectRoleAdmin,
		ActionTeamDelete:    model.ProjectRoleAdmin,

		// Workflows.
		ActionWorkflowRead:    model.ProjectRoleViewer,
		ActionWorkflowWrite:   model.ProjectRoleEditor,
		ActionWorkflowExecute: model.ProjectRoleEditor,
		ActionWorkflowReview:  model.ProjectRoleEditor,
		ActionWorkflowCancel:  model.ProjectRoleEditor,

		// Automation.
		ActionAutomationRead:    model.ProjectRoleViewer,
		ActionAutomationWrite:   model.ProjectRoleAdmin,
		ActionAutomationTrigger: model.ProjectRoleEditor,

		// Settings & dashboards.
		ActionSettingsRead:   model.ProjectRoleViewer,
		ActionSettingsUpdate: model.ProjectRoleAdmin,
		ActionDashboardRead:  model.ProjectRoleViewer,
		ActionDashboardWrite: model.ProjectRoleAdmin,

		// Wiki / Knowledge.
		ActionWikiRead:   model.ProjectRoleViewer,
		ActionWikiWrite:  model.ProjectRoleEditor,
		ActionWikiDelete: model.ProjectRoleEditor,

		// Memory & logs.
		ActionMemoryRead:  model.ProjectRoleViewer,
		ActionMemoryWrite: model.ProjectRoleEditor,
		ActionLogRead:     model.ProjectRoleViewer,
		ActionLogWrite:    model.ProjectRoleEditor,

		// Custom fields, saved views, forms, milestones, sprints.
		ActionCustomFieldRead:  model.ProjectRoleViewer,
		ActionCustomFieldWrite: model.ProjectRoleEditor,
		ActionSavedViewWrite:   model.ProjectRoleEditor,
		ActionFormWrite:        model.ProjectRoleEditor,
		ActionMilestoneWrite:   model.ProjectRoleEditor,
		ActionSprintWrite:      model.ProjectRoleEditor,

		// Agent control.
		ActionAgentSpawn:   model.ProjectRoleEditor,
		ActionAgentControl: model.ProjectRoleEditor,

		// Audit.
		ActionAuditRead: model.ProjectRoleAdmin,

		// Project templates — save-as-template is admin+ (design decision
		// #2 in add-project-templates/design.md).
		ActionProjectSaveAsTemplate: model.ProjectRoleAdmin,

		// Audit-only: recorded via audit service, never checked via Require.
		// We slot it at viewer so any ACL-reachable entity can read the event.
		ActionProjectCreatedFromTemplate: model.ProjectRoleViewer,
	}
}()

// MatrixSnapshot returns a defensive copy of the action→minRole mapping.
// Useful for tests and for deriving the per-project permissions response.
func MatrixSnapshot() map[ActionID]string {
	out := make(map[ActionID]string, len(matrix))
	for k, v := range matrix {
		out[k] = v
	}
	return out
}

// AllActions returns every declared ActionID in deterministic order.
func AllActions() []ActionID {
	out := make([]ActionID, 0, len(matrix))
	for k := range matrix {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}

// MinRoleFor returns the minimum role needed for action and a known flag.
func MinRoleFor(action ActionID) (string, bool) {
	role, ok := matrix[action]
	return role, ok
}

// AllowedActionsFor returns the sorted set of ActionIDs the given role can perform.
func AllowedActionsFor(role string) []ActionID {
	out := make([]ActionID, 0)
	for action, min := range matrix {
		if model.ProjectRoleAtLeast(role, min) {
			out = append(out, action)
		}
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}

// MemberLookup is the narrow contract RBAC needs to resolve a caller's
// project role. Implemented by *repository.MemberRepository.
type MemberLookup interface {
	GetByUserAndProject(ctx context.Context, userID, projectID uuid.UUID) (*model.Member, error)
}

// AuditEmission carries the minimum context needed for an audit hook to
// record an allow or deny event. The actual struct lives in this package
// to avoid a service-layer import cycle; the audit service consumes it
// via an emitter callback installed at startup.
type AuditEmission struct {
	ProjectID              uuid.UUID
	ActionID               ActionID
	ActorUserID            *uuid.UUID
	ActorProjectRoleAtTime string
	Allowed                bool
	RequestID              string
	IP                     string
	UserAgent              string
}

// AuditEmitter is invoked from RBAC middleware on every allow/deny pass.
// Wiring is opt-in: when no emitter is registered, RBAC behaves exactly
// as before. Implementations MUST be non-blocking — the typical impl
// enqueues to the audit sink and returns immediately.
type AuditEmitter func(ctx context.Context, e AuditEmission)

// RBAC errors. Returned by Authorize so service callers can surface domain-
// specific responses without depending on the HTTP layer.
var (
	ErrNotAProjectMember       = errors.New("rbac: caller is not a member of this project")
	ErrInsufficientProjectRole = errors.New("rbac: caller does not meet the required project role")
	ErrUnknownAction           = errors.New("rbac: unknown action id")
)

const (
	// CallerProjectRoleContextKey holds the resolved caller's projectRole on
	// the echo.Context after a successful Require() pass. Handlers may read
	// it via GetCallerProjectRole.
	CallerProjectRoleContextKey = "rbac.caller_role"

	// CallerMemberIDContextKey holds the caller's member.ID when known.
	CallerMemberIDContextKey = "rbac.caller_member_id"
)

type rbacRegistry struct {
	mu      sync.RWMutex
	lookup  MemberLookup
	emitter AuditEmitter
}

var defaultRegistry = &rbacRegistry{}

// SetMemberLookup installs the repo used by Require() to resolve project roles.
// Callers wire this once during server startup. Tests may swap with a fake.
func SetMemberLookup(lookup MemberLookup) {
	defaultRegistry.mu.Lock()
	defaultRegistry.lookup = lookup
	defaultRegistry.mu.Unlock()
}

// SetAuditEmitter installs the audit hook. Idempotent; the latest emitter
// wins. Pass nil to disable emission (e.g. in tests).
func SetAuditEmitter(emitter AuditEmitter) {
	defaultRegistry.mu.Lock()
	defaultRegistry.emitter = emitter
	defaultRegistry.mu.Unlock()
}

func currentLookup() MemberLookup {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	return defaultRegistry.lookup
}

func currentEmitter() AuditEmitter {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	return defaultRegistry.emitter
}

// Authorize is the headless authorization check. Returns the caller's resolved
// member row and nil on allow; ErrNotAProjectMember / ErrInsufficientProjectRole
// / ErrUnknownAction on deny.
//
// Callers that need to bypass RBAC for system-initiated paths should NOT call
// Authorize — they should use service-layer Caller{SystemInitiated:true} and
// resolve the configured-by user's role via this same function.
func Authorize(ctx context.Context, lookup MemberLookup, action ActionID, projectID, userID uuid.UUID) (*model.Member, error) {
	minRole, ok := MinRoleFor(action)
	if !ok {
		return nil, ErrUnknownAction
	}
	if lookup == nil {
		return nil, ErrNotAProjectMember
	}
	member, err := lookup.GetByUserAndProject(ctx, userID, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotAProjectMember
		}
		return nil, err
	}
	if member == nil {
		return nil, ErrNotAProjectMember
	}
	if !model.ProjectRoleAtLeast(model.NormalizeProjectRole(member.ProjectRole), minRole) {
		return nil, ErrInsufficientProjectRole
	}
	return member, nil
}

// Require returns an Echo middleware that gates the route on the given action.
// The middleware runs AFTER JWTMiddleware and ProjectMiddleware: it relies on
// the JWT claims for caller identity and the project context for projectID.
//
// On allow, the caller's projectRole and memberID are stashed on the context.
// On deny, a localized JSON error with the appropriate HTTP status is returned.
func Require(action ActionID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			lookup := currentLookup()
			if _, ok := MinRoleFor(action); !ok {
				return c.JSON(http.StatusInternalServerError, model.ErrorResponse{
					Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgUnknownProjectAction),
				})
			}
			claims, err := GetClaims(c)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{
					Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgUnauthorized),
				})
			}
			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, model.ErrorResponse{
					Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgUnauthorized),
				})
			}
			projectID := GetProjectID(c)
			if projectID == uuid.Nil {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{
					Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgProjectIDRequired),
				})
			}
			member, err := Authorize(c.Request().Context(), lookup, action, projectID, userID)
			emitter := currentEmitter()
			if err != nil {
				if emitter != nil {
					emitter(c.Request().Context(), AuditEmission{
						ProjectID:   projectID,
						ActionID:    action,
						ActorUserID: uuidPtr(userID),
						Allowed:     false,
						RequestID:   c.Response().Header().Get(echo.HeaderXRequestID),
						IP:          c.RealIP(),
						UserAgent:   c.Request().UserAgent(),
					})
				}
				return rbacDenyResponse(c, err)
			}
			callerRole := model.NormalizeProjectRole(member.ProjectRole)
			c.Set(CallerProjectRoleContextKey, callerRole)
			c.Set(CallerMemberIDContextKey, member.ID)
			if emitter != nil {
				emitter(c.Request().Context(), AuditEmission{
					ProjectID:              projectID,
					ActionID:               action,
					ActorUserID:            uuidPtr(userID),
					ActorProjectRoleAtTime: callerRole,
					Allowed:                true,
					RequestID:              c.Response().Header().Get(echo.HeaderXRequestID),
					IP:                     c.RealIP(),
					UserAgent:              c.Request().UserAgent(),
				})
			}
			return next(c)
		}
	}
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	out := id
	return &out
}

// rbacDenyResponse maps Authorize errors to HTTP responses. Exposed as a
// helper so other middleware (e.g., audit) can reuse the mapping.
func rbacDenyResponse(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrNotAProjectMember):
		return c.JSON(http.StatusForbidden, model.ErrorResponse{
			Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgNotAProjectMember),
		})
	case errors.Is(err, ErrInsufficientProjectRole):
		return c.JSON(http.StatusForbidden, model.ErrorResponse{
			Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgInsufficientProjectRole),
		})
	case errors.Is(err, ErrUnknownAction):
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgUnknownProjectAction),
		})
	default:
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{
			Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgInternalError),
		})
	}
}

// GetCallerProjectRole returns the role stamped onto the context by Require().
// Empty string when no RBAC pass has run for the request.
func GetCallerProjectRole(c echo.Context) string {
	v, _ := c.Get(CallerProjectRoleContextKey).(string)
	return v
}

// GetCallerMemberID returns the resolved member id stamped onto the context
// by Require(). Returns uuid.Nil when no RBAC pass has run.
func GetCallerMemberID(c echo.Context) uuid.UUID {
	v, _ := c.Get(CallerMemberIDContextKey).(uuid.UUID)
	return v
}
