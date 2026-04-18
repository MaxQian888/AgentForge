// Package middleware — archived_guard.go implements the lifecycle-guard
// middleware layer that complements RBAC.
//
// Layer ordering (intentional, asserted by routes_wiring_test):
//
//     JWT → Project → RBAC → ArchivedGuard → handler
//
// RBAC answers "does this caller have the right role". ArchivedGuard answers
// "is this project currently writable". Separating them keeps the role matrix
// 2-dimensional (action × role) instead of 3-dimensional (action × role ×
// status). A viewer who hits an archived project hits RBAC first — that's
// the "your role is too low" signal. An owner who hits an archived project
// gets past RBAC and is then stopped here.
//
// The guard consults the canonical read-only whitelist declared in this file.
// Any action ending in `.read`, plus a short list of explicit view-class
// actions (project lifecycle, audit), are allowed on archived projects.
// Anything else returns 409 `project_archived` with archivedAt/by in the
// body so the UI can render a "this project is archived" banner instead of
// guessing why the write was refused.
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appI18n "github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
)

// archivedReadOnlyWhitelist is the small, explicitly-enumerated set of
// write-class ActionIDs allowed on archived projects. Any `.read` action is
// allowed implicitly by the suffix check and is NOT listed here.
//
// Rationale for each entry:
//   * project.unarchive : owner needs to flip the project back to active
//   * project.delete    : owner needs to physically delete the archived project
//   * audit.read        : covered by the suffix check below, but noted
//   * settings.read     : covered by the suffix check
//
// If you add a new write-class action that must work on archived projects,
// it belongs here — NOT as a RBAC matrix tweak.
var archivedReadOnlyWhitelist = map[ActionID]struct{}{
	ActionProjectUnarchive: {},
	ActionProjectDelete:      {},
}

// ActionProjectArchive / ActionProjectUnarchive are declared here to avoid a
// circular dependency between this file and rbac.go's matrix declaration
// (both need the other's symbol). They are registered into the main matrix
// via the init() below.
const (
	ActionProjectArchive   ActionID = "project.archive"
	ActionProjectUnarchive ActionID = "project.unarchive"
)

func init() {
	// Register archive/unarchive into the canonical RBAC matrix. Both are
	// owner-only per design.md §1.
	matrix[ActionProjectArchive] = model.ProjectRoleOwner
	matrix[ActionProjectUnarchive] = model.ProjectRoleOwner
}

// IsArchivedProjectReadOnlyAction reports whether the action is permitted on
// an archived project. Any `.read` / `.view` suffix is allowed as a blanket
// read surface; the explicit whitelist above covers the non-read exceptions.
func IsArchivedProjectReadOnlyAction(action ActionID) bool {
	if _, ok := archivedReadOnlyWhitelist[action]; ok {
		return true
	}
	a := string(action)
	if strings.HasSuffix(a, ".read") || strings.HasSuffix(a, ".view") {
		return true
	}
	return false
}

// archivedProjectErrorBody is the canonical 409 payload when the guard
// refuses a write on an archived project. UI consumes ErrorCode to decide
// whether to render the "archived" banner vs a generic error toast.
type archivedProjectErrorBody struct {
	Message          string     `json:"message"`
	Code             int        `json:"code"`
	ErrorCode        string     `json:"errorCode"`
	ArchivedAt       *time.Time `json:"archivedAt,omitempty"`
	ArchivedByUserID *string    `json:"archivedByUserId,omitempty"`
}

// ArchivedGuard returns an Echo middleware that denies the given action
// when the resolved project is archived. Must be registered AFTER
// ProjectMiddleware (so the project is in context) and AFTER Require()
// (so role gating runs first). Missing project context fails open — the
// guard is no-op for non-project-scoped routes.
func ArchivedGuard(action ActionID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			project := GetProject(c)
			if project == nil {
				return next(c)
			}
			if !project.IsArchived() {
				return next(c)
			}
			if IsArchivedProjectReadOnlyAction(action) {
				return next(c)
			}
			return writeArchivedProjectError(c, project)
		}
	}
}

// ArchivedProjectWriteGuard is a project-group-level middleware that blocks
// any non-read HTTP request on an archived project. Reads (GET/HEAD/OPTIONS)
// always pass. Writes pass only when the path matches a whitelisted
// lifecycle path (archive/unarchive), which the caller wires via
// WithWhitelistedPaths. This is the group-level fallback when per-route
// ActionID-based guards would be too verbose; per-route ArchivedGuard is
// still preferred when the route author knows the ActionID up front.
type ArchivedProjectWriteGuardConfig struct {
	// WhitelistedSuffixes lists path suffixes (relative to the project
	// group) that are exempt from the write guard. Example: "/unarchive"
	// permits POST /projects/:pid/unarchive on archived projects.
	WhitelistedSuffixes []string
}

// ArchivedProjectWriteGuard returns the group-level guard configured with
// the supplied whitelist.
func ArchivedProjectWriteGuard(cfg ArchivedProjectWriteGuardConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			project := GetProject(c)
			if project == nil || !project.IsArchived() {
				return next(c)
			}
			method := c.Request().Method
			switch method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}
			path := c.Path()
			for _, suffix := range cfg.WhitelistedSuffixes {
				if strings.HasSuffix(path, suffix) {
					return next(c)
				}
			}
			return writeArchivedProjectError(c, project)
		}
	}
}

func writeArchivedProjectError(c echo.Context, project *model.Project) error {
	body := archivedProjectErrorBody{
		Message:   appI18n.Localize(GetLocalizer(c), appI18n.MsgProjectArchived),
		Code:      http.StatusConflict,
		ErrorCode: "project_archived",
	}
	if project.ArchivedAt != nil {
		t := *project.ArchivedAt
		body.ArchivedAt = &t
	}
	if project.ArchivedByUserID != nil {
		id := project.ArchivedByUserID.String()
		body.ArchivedByUserID = &id
	}
	return c.JSON(http.StatusConflict, body)
}

// IsArchivedProjectForRequest is a helper that service-layer callers can use
// to re-check project status before performing a write. Prefer the middleware
// for HTTP paths; use this for scheduler/automation/internal-caller paths
// that bypass HTTP (see design.md §4 "belt and suspenders").
func IsArchivedProjectForRequest(project *model.Project) bool {
	return project != nil && project.Status == model.ProjectStatusArchived
}

// ProjectArchivedError is the error type service entry-points return when
// they reject a write because the project is archived. Distinct from the
// middleware guard so internal callers can errors.Is / errors.As against it.
type ProjectArchivedError struct {
	ProjectID        uuid.UUID
	ArchivedAt       *time.Time
	ArchivedByUserID *uuid.UUID
}

func (e *ProjectArchivedError) Error() string {
	return "project is archived (write rejected by lifecycle guard)"
}

// NewProjectArchivedError builds a ProjectArchivedError from a project
// snapshot.
func NewProjectArchivedError(project *model.Project) *ProjectArchivedError {
	if project == nil {
		return &ProjectArchivedError{}
	}
	return &ProjectArchivedError{
		ProjectID:        project.ID,
		ArchivedAt:       project.ArchivedAt,
		ArchivedByUserID: project.ArchivedByUserID,
	}
}
