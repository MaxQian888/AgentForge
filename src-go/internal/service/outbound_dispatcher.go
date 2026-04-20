package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
)

// ExecutionLoader is the repository surface the dispatcher needs.
type ExecutionLoader interface {
	GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error)
}

// WorkflowDefinitionLoader optionally resolves the workflow display name used
// in the default card title. When nil, the dispatcher falls back to the
// workflow id. The signature matches repository.WorkflowDefinitionRepository
// so the existing repo can be passed in directly.
type WorkflowDefinitionLoader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
}

// OutboundDispatcher subscribes to terminal workflow execution events and
// delivers the default reply card to the IM Bridge unless the workflow
// explicitly took over delivery via system_metadata.im_dispatched. Spec §5.
type OutboundDispatcher struct {
	execRepo  ExecutionLoader
	wfRepo    WorkflowDefinitionLoader
	bridgeURL string
	feBaseURL string
	bus       eb.Publisher
	client    *http.Client
	delays    [3]time.Duration
}

// NewOutboundDispatcher constructs a dispatcher. wfRepo may be nil; bus may
// be nil during unit tests that don't assert on emission of the failure
// event.
func NewOutboundDispatcher(repo ExecutionLoader, bridgeURL, feBaseURL string, bus eb.Publisher) *OutboundDispatcher {
	return &OutboundDispatcher{
		execRepo:  repo,
		bridgeURL: strings.TrimRight(bridgeURL, "/"),
		feBaseURL: strings.TrimRight(feBaseURL, "/"),
		bus:       bus,
		client:    &http.Client{Timeout: 10 * time.Second},
		delays:    [3]time.Duration{1 * time.Second, 4 * time.Second, 16 * time.Second},
	}
}

// SetWorkflowLoader allows late-binding of the workflow definition repo so
// the card title reflects the workflow name when available.
func (d *OutboundDispatcher) SetWorkflowLoader(loader WorkflowDefinitionLoader) {
	d.wfRepo = loader
}

// SetRetryDelays is for tests; production keeps 1s/4s/16s.
func (d *OutboundDispatcher) SetRetryDelays(d1, d2, d3 time.Duration) {
	d.delays = [3]time.Duration{d1, d2, d3}
}

// --- eventbus.Mod interface ---

func (d *OutboundDispatcher) Name() string         { return "service.outbound-dispatcher" }
func (d *OutboundDispatcher) Intercepts() []string { return []string{eb.EventWorkflowExecutionCompleted} }
func (d *OutboundDispatcher) Priority() int        { return 80 }
func (d *OutboundDispatcher) Mode() eb.Mode        { return eb.ModeObserve }

type completedPayload struct {
	ExecutionID string `json:"executionId"`
	WorkflowID  string `json:"workflowId"`
	Status      string `json:"status"`
}

// Observe is invoked synchronously by the bus. It decodes the payload,
// fast-paths cancelled runs, and otherwise hands off to dispatch on a
// goroutine so retries don't block the event pipeline.
func (d *OutboundDispatcher) Observe(ctx context.Context, e *eb.Event, _ *eb.PipelineCtx) {
	if e == nil {
		return
	}
	var p completedPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		log.WithError(err).Warn("outbound_dispatcher: payload decode")
		return
	}
	// Only dispatch on completed/failed; cancelled is silent.
	if p.Status != model.WorkflowExecStatusCompleted && p.Status != model.WorkflowExecStatusFailed {
		return
	}
	execID, err := uuid.Parse(p.ExecutionID)
	if err != nil {
		return
	}
	go d.dispatch(context.Background(), execID, p.Status)
}

func (d *OutboundDispatcher) dispatch(ctx context.Context, execID uuid.UUID, status string) {
	exec, err := d.execRepo.GetExecution(ctx, execID)
	if err != nil || exec == nil {
		log.WithError(err).WithField("executionId", execID).Warn("outbound_dispatcher: load exec")
		return
	}
	sm := decodeSystemMetadata(exec.SystemMetadata)
	if v, _ := sm["im_dispatched"].(bool); v {
		log.WithField("executionId", execID).Info("outbound_dispatcher: skipped (explicit im_send)")
		return
	}
	target := decodeReplyTarget(sm["reply_target"])
	if target == nil {
		log.WithField("executionId", execID).Info("outbound_dispatcher: skipped (no reply target)")
		return
	}

	card := d.buildDefaultCard(ctx, exec, status, sm)
	body, _ := json.Marshal(map[string]any{
		"platform":    target.Platform,
		"chat_id":     target.ChatID,
		"replyTarget": target,
		"card":        card,
	})

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(d.delays[attempt-1])
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.bridgeURL+"/im/send", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := d.client.Do(req)
		if err == nil && resp != nil && resp.StatusCode < 400 {
			resp.Body.Close()
			return
		}
		if resp != nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		} else {
			lastErr = err
		}
		log.WithError(lastErr).WithField("attempt", attempt+1).Warn("outbound_dispatcher: send failed")
	}
	d.emitFailure(ctx, exec, lastErr)
}

func (d *OutboundDispatcher) emitFailure(ctx context.Context, exec *model.WorkflowExecution, lastErr error) {
	if d.bus == nil {
		return
	}
	msg := ""
	if lastErr != nil {
		msg = lastErr.Error()
	}
	_ = eb.PublishLegacy(ctx, d.bus, eb.EventOutboundDeliveryFailed, exec.ProjectID.String(), map[string]any{
		"executionId": exec.ID.String(),
		"lastError":   msg,
		"attempts":    3,
	})
}
