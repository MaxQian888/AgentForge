package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/vcs"
)

const summaryBodyHardLimit = 50 * 1024 // 50KB safety cap

// VCSOutboundReviewReader is the narrow read-only surface on ReviewService
// the dispatcher needs.
type VCSOutboundReviewReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
}

// VCSOutboundIntegrationLoader loads an integration by ID.
type VCSOutboundIntegrationLoader interface {
	Get(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error)
}

// VCSOutboundSecretsResolver resolves secret by project + name.
type VCSOutboundSecretsResolver interface {
	Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
}

// VCSOutboundDispatcher subscribes to EventReviewCompleted and posts
// review results as PR comments via the vcs.Provider interface.
type VCSOutboundDispatcher struct {
	reviews      VCSOutboundReviewReader
	integrations VCSOutboundIntegrationLoader
	secrets      VCSOutboundSecretsResolver
	registry     *vcs.Registry
	bus          eb.Publisher
	feBaseURL    string
	delays       [3]time.Duration
}

func NewVCSOutboundDispatcher(
	r VCSOutboundReviewReader,
	integ VCSOutboundIntegrationLoader,
	s VCSOutboundSecretsResolver,
	reg *vcs.Registry,
	bus eb.Publisher,
	feBaseURL string,
) *VCSOutboundDispatcher {
	return &VCSOutboundDispatcher{
		reviews:      r,
		integrations: integ,
		secrets:      s,
		registry:     reg,
		bus:          bus,
		feBaseURL:    feBaseURL,
		delays:       [3]time.Duration{1 * time.Second, 4 * time.Second, 16 * time.Second},
	}
}

// SetRetryDelays overrides exponential backoff delays (for tests).
func (d *VCSOutboundDispatcher) SetRetryDelays(d1, d2, d3 time.Duration) {
	d.delays = [3]time.Duration{d1, d2, d3}
}

// --- eventbus.Mod interface ---

func (d *VCSOutboundDispatcher) Name() string         { return "service.vcs-outbound-dispatcher" }
func (d *VCSOutboundDispatcher) Intercepts() []string { return []string{eb.EventReviewCompleted} }
func (d *VCSOutboundDispatcher) Priority() int        { return 90 }
func (d *VCSOutboundDispatcher) Mode() eb.Mode        { return eb.ModeObserve }

type reviewCompletedPayload struct {
	ReviewID string `json:"reviewId"`
	ID       string `json:"id"`
}

// Observe implements eventbus.ObserveMod.
func (d *VCSOutboundDispatcher) Observe(ctx context.Context, e *eb.Event, _ *eb.PipelineCtx) {
	var p reviewCompletedPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		log.WithError(err).Warn("vcs_outbound_dispatcher: payload decode")
		return
	}
	idStr := p.ReviewID
	if idStr == "" {
		idStr = p.ID
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		log.WithError(err).WithField("raw", idStr).Warn("vcs_outbound_dispatcher: parse review id")
		return
	}
	// Fire-and-forget in background goroutine to not block the pipeline.
	go d.HandleReviewCompleted(context.Background(), id)
}

// HandleReviewCompleted is exported for direct test invocation.
func (d *VCSOutboundDispatcher) HandleReviewCompleted(ctx context.Context, reviewID uuid.UUID) {
	rev, err := d.reviews.GetByID(ctx, reviewID)
	if err != nil || rev == nil {
		log.WithError(err).WithField("reviewId", reviewID).Warn("vcs_dispatcher: load review")
		return
	}
	if rev.IntegrationID == nil {
		log.WithField("reviewId", reviewID).Debug("vcs_dispatcher: skip (no integration_id)")
		return
	}

	integ, err := d.integrations.Get(ctx, *rev.IntegrationID)
	if err != nil {
		log.WithError(err).Warn("vcs_dispatcher: load integration")
		return
	}

	pat, err := d.secrets.Resolve(ctx, integ.ProjectID, integ.TokenSecretRef)
	if err != nil || pat == "" {
		log.WithError(err).Warn("vcs_dispatcher: resolve PAT")
		return
	}

	prov, err := d.registry.Resolve(integ.Provider, integ.Host, pat)
	if err != nil {
		log.WithError(err).Warn("vcs_dispatcher: provider")
		return
	}

	pr := &vcs.PullRequest{
		Number:  rev.PRNumber,
		HeadSHA: rev.HeadSHA,
	}

	findings := rev.Findings
	body := d.buildSummaryBody(rev, findings)

	if err := d.deliverSummary(ctx, prov, pr, rev, body); err != nil {
		d.emitFailure(ctx, rev, "summary", err)
		return
	}
	d.deliverInline(ctx, prov, pr, rev, findings)
}

func (d *VCSOutboundDispatcher) deliverSummary(ctx context.Context, prov vcs.Provider, pr *vcs.PullRequest, rev *model.Review, body string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(d.delays[attempt-1])
		}
		if rev.SummaryCommentID != "" {
			if err := prov.EditSummaryComment(ctx, pr, rev.SummaryCommentID, body); err == nil {
				return nil
			} else {
				lastErr = err
			}
		} else {
			id, err := prov.PostSummaryComment(ctx, pr, body)
			if err == nil {
				rev.SummaryCommentID = id
				// TODO: persist summary_comment_id on the review row via repo.
				return nil
			}
			lastErr = err
		}
		log.WithError(lastErr).WithField("attempt", attempt+1).Warn("vcs_dispatcher: summary failed")
	}
	return lastErr
}

func (d *VCSOutboundDispatcher) deliverInline(ctx context.Context, prov vcs.Provider, pr *vcs.PullRequest, rev *model.Review, findings []model.ReviewFinding) {
	var fresh []vcs.InlineComment
	for _, f := range findings {
		if f.File == "" || f.Line <= 0 {
			continue
		}
		body := fmt.Sprintf("**[%s]** %s", strings.ToUpper(f.Severity), f.Message)
		if f.SuggestedPatch != "" {
			body += "\n```suggestion\n" + f.SuggestedPatch + "\n```"
		}
		ic := vcs.InlineComment{Path: f.File, Line: f.Line, Body: body, Side: "RIGHT"}
		if f.InlineCommentID != "" {
			_ = retryFn(d.delays, func() error { return prov.EditReviewComment(ctx, pr, f.InlineCommentID, body) })
			continue
		}
		fresh = append(fresh, ic)
	}
	if len(fresh) == 0 {
		return
	}
	_, err := prov.PostReviewComments(ctx, pr, fresh)
	if err != nil {
		d.emitFailure(ctx, rev, "inline", err)
	}
}

func retryFn(delays [3]time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (d *VCSOutboundDispatcher) buildSummaryBody(rev *model.Review, findings []model.ReviewFinding) string {
	var crit, warn int
	for _, f := range findings {
		switch strings.ToLower(f.Severity) {
		case "critical", "error":
			crit++
		case "warning", "warn":
			warn++
		}
	}
	var b strings.Builder
	b.WriteString("## AgentForge Review\n\n")
	fmt.Fprintf(&b, "- %d critical / %d warnings\n", crit, warn)
	fmt.Fprintf(&b, "- [Open full review](%s/reviews/%s)\n\n", strings.TrimRight(d.feBaseURL, "/"), rev.ID)
	if len(findings) > 0 {
		b.WriteString("## Findings\n\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "- **[%s]** `%s:%d` - %s\n", strings.ToUpper(f.Severity), f.File, f.Line, f.Message)
		}
	}
	out := b.String()
	if len(out) > summaryBodyHardLimit {
		cut := summaryBodyHardLimit - 200
		if cut < 0 {
			cut = 0
		}
		out = out[:cut] + fmt.Sprintf("\n\n...truncated; see full review at %s/reviews/%s\n", strings.TrimRight(d.feBaseURL, "/"), rev.ID)
	}
	return out
}

func (d *VCSOutboundDispatcher) emitFailure(ctx context.Context, rev *model.Review, op string, lastErr error) {
	if d.bus == nil {
		return
	}
	msg := ""
	if lastErr != nil {
		msg = lastErr.Error()
	}
	projectID := ""
	if rev.ProjectID != uuid.Nil {
		projectID = rev.ProjectID.String()
	}
	_ = eb.PublishLegacy(ctx, d.bus, eb.EventVCSDeliveryFailed, projectID, map[string]any{
		"review_id": rev.ID.String(),
		"op":        op,
		"error":     msg,
	})
}
