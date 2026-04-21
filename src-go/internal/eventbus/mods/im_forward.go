package mods

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/imconfig"
	"github.com/react-go-quick-starter/server/internal/model"
	log "github.com/sirupsen/logrus"
)

// AncestorRootResolver resolves the root ancestor of a task by walking ParentID.
// Implemented by *repository.TaskRepository.
type AncestorRootResolver interface {
	GetAncestorRoot(ctx context.Context, taskID uuid.UUID) (*model.Task, error)
}

// IMForwardControlPlane is the minimal interface subset of *service.IMControlPlane
// required by IMForward. Keeping it narrow avoids importing the service package
// and prevents an import cycle.
type IMForwardControlPlane interface {
	// BoundPlatformForTask returns the IM platform for the given task's binding,
	// or "" if no binding exists.
	BoundPlatformForTask(taskID string) string
	// QueueBoundProgressRaw dispatches a progress event to the IM bridge using
	// the stored binding for the given task. Returns (true, nil) on success.
	QueueBoundProgressRaw(ctx context.Context, taskID, content string, isTerminal bool, metadata map[string]string) (bool, error)
}

// IMForward is the successor to IMForwardLegacy. It subscribes to task/agent/
// review events on the event bus, walks the task's ancestor chain to find the
// root task, and queues a bound progress delivery using the root task's IM
// reply target.
//
// Folding behaviour is controlled by imconfig.FoldingModeFor(platform):
//
//   - nested       – post to IM using the root task's binding. Child events
//     are prefixed with "[sub-agent <short_id>]" so the IM user can identify
//     which sub-agent produced the update.
//   - flat         – post using the event task's own binding if it has one,
//     falling back to the root's binding.
//   - frontend_only – suppress IM posts for child-task events entirely.
//     Root-task events are posted normally.
type IMForward struct {
	tasks        AncestorRootResolver
	controlPlane IMForwardControlPlane
}

// NewIMForward constructs an IMForward observer.
// tasks may be nil, in which case ancestry lookup is skipped and events are
// dispatched directly against the event task's own binding.
// controlPlane may be nil, in which case the observer is a no-op.
func NewIMForward(tasks AncestorRootResolver, controlPlane IMForwardControlPlane) *IMForward {
	return &IMForward{tasks: tasks, controlPlane: controlPlane}
}

func (m *IMForward) Name() string  { return "im.forward" }
func (m *IMForward) Priority() int { return 81 } // runs just after im.forward-legacy (80)
func (m *IMForward) Mode() eb.Mode { return eb.ModeObserve }

// Intercepts declares the event-type patterns this observer wants to receive.
//
// Currently reachable (publishers emit "task:<uuid>" as e.Target):
//   - "task.*" — reserved for task-scoped publishers that set Target via
//     eventbus.MakeTask(taskID). No production publisher does this yet; all
//     current call-sites use PublishLegacy which sets Target to
//     "project:<id>" or "system:broadcast". The pattern is declared here so
//     the observer is wired-up as soon as the first task-targeted publisher
//     lands.
//
// NOTE: "review.*", "agent.*", and "notification" were removed because every
// current publisher emits those events via PublishLegacy, which sets
// e.Target = "project:<id>" (or "system:broadcast"). The Observe dispatch
// guard at line ~74 returns early whenever addr.Scheme != "task", so these
// patterns were silently no-ops. Re-add them here when their publishers
// migrate to eventbus.MakeTask(taskID) targets.
func (m *IMForward) Intercepts() []string {
	return []string{"task.*"}
}

func (m *IMForward) Observe(ctx context.Context, e *eb.Event, _ *eb.PipelineCtx) {
	if m == nil || m.controlPlane == nil || e == nil {
		return
	}

	// Extract task ID from Target (format "task:<uuid>").
	addr, err := eb.ParseAddress(e.Target)
	if err != nil || addr.Scheme != "task" {
		return
	}
	taskID, err := uuid.Parse(addr.Name)
	if err != nil {
		log.WithFields(log.Fields{"event_id": e.ID, "target": e.Target}).
			Warn("im.forward: non-UUID task ID in event target, skipping")
		return
	}

	// Walk the ancestor chain to the root task.
	rootTaskID := taskID
	if m.tasks != nil {
		root, err := m.tasks.GetAncestorRoot(ctx, taskID)
		if err != nil {
			// Task not in DB yet, already deleted, or cycle — skip silently at debug level.
			log.WithFields(log.Fields{"event_id": e.ID, "task_id": taskID}).
				WithError(err).Debug("im.forward: ancestor root lookup failed, skipping")
			return
		}
		rootTaskID = root.ID
	}

	rootIDStr := rootTaskID.String()

	// If the root task has no IM binding the task is not IM-originated.
	// Leave it invisible to IM — unchanged behaviour.
	platform := m.controlPlane.BoundPlatformForTask(rootIDStr)
	if platform == "" {
		return
	}

	isChild := taskID != rootTaskID
	mode := imconfig.FoldingModeFor(platform)

	switch mode {
	case imconfig.FoldingModeFrontendOnly:
		if isChild {
			return
		}
		m.dispatch(ctx, e, rootIDStr, "")

	case imconfig.FoldingModeNested:
		prefix := ""
		if isChild {
			prefix = fmt.Sprintf("[sub-agent %s] ", shortID(taskID.String()))
		}
		m.dispatch(ctx, e, rootIDStr, prefix)

	default: // FoldingModeFlat
		taskIDStr := taskID.String()
		if m.controlPlane.BoundPlatformForTask(taskIDStr) != "" {
			m.dispatch(ctx, e, taskIDStr, "")
		} else {
			m.dispatch(ctx, e, rootIDStr, "")
		}
	}
}

func (m *IMForward) dispatch(ctx context.Context, e *eb.Event, taskID, prefix string) {
	content := prefix + buildIMForwardContent(e)
	metadata := map[string]string{"bridge_event_type": e.Type}
	ok, err := m.controlPlane.QueueBoundProgressRaw(ctx, taskID, content, false, metadata)
	if err != nil {
		log.WithFields(log.Fields{"event_id": e.ID, "event": e.Type, "task_id": taskID}).
			WithError(err).Warn("im.forward: QueueBoundProgress failed")
		return
	}
	if !ok {
		log.WithFields(log.Fields{"event_id": e.ID, "event": e.Type, "task_id": taskID}).
			Debug("im.forward: no binding, skipped")
	}
}

func buildIMForwardContent(e *eb.Event) string {
	if pid := eb.GetString(e, eb.MetaProjectID); pid != "" {
		return fmt.Sprintf("[%s] %s", pid, e.Type)
	}
	return e.Type
}

// shortID returns the first 8 hex characters of a UUID (hyphens removed),
// providing a compact identifier for IM prefix tags.
func shortID(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) > 8 {
		return clean[:8]
	}
	return clean
}
