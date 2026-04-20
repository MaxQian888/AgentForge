package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ErrPushHandlerNotImplemented is a sentinel returned by RouteEvent for
// push / pull_request{synchronize} actions. Plan 2C replaces this body
// with the diff-of-diff pipeline — keep the sentinel stable.
var ErrPushHandlerNotImplemented = errors.New("vcs_webhook_router: push/synchronize handler is owned by Plan 2C")

// ReviewTrigger is the narrow surface of ReviewService this router needs.
type ReviewTrigger interface {
	Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error)
}

// VCSWebhookRouter dispatches parsed webhook payloads to the appropriate
// handler by event type. It is the extensibility seam for Plan 2C (push
// events) and future event types.
type VCSWebhookRouter struct{ reviews ReviewTrigger }

func NewVCSWebhookRouter(rt ReviewTrigger) *VCSWebhookRouter {
	return &VCSWebhookRouter{reviews: rt}
}

type prPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		Head    struct{ SHA string `json:"sha"` } `json:"head"`
		Base    struct{ SHA string `json:"sha"` } `json:"base"`
	} `json:"pull_request"`
}

// RouteEvent dispatches an inbound webhook by event type + action.
func (r *VCSWebhookRouter) RouteEvent(ctx context.Context, integ *model.VCSIntegration,
	eventType, deliveryID string, body []byte) error {

	switch eventType {
	case "pull_request":
		var p prPayload
		if err := json.Unmarshal(body, &p); err != nil {
			return err
		}
		switch p.Action {
		case "opened", "reopened", "ready_for_review":
			return r.triggerReview(ctx, integ, p)
		case "synchronize":
			// Plan 2C plugs in here.
			return ErrPushHandlerNotImplemented
		default:
			log.WithFields(log.Fields{
				"integration": integ.ID, "delivery": deliveryID, "action": p.Action,
			}).Debug("vcs_webhook_router: ignoring pull_request action")
			return nil
		}
	case "push":
		// Plan 2C plugs in here.
		return ErrPushHandlerNotImplemented
	default:
		log.WithFields(log.Fields{
			"integration": integ.ID, "delivery": deliveryID, "event": eventType,
		}).Debug("vcs_webhook_router: ignoring event type")
		return nil
	}
}

func (r *VCSWebhookRouter) triggerReview(ctx context.Context, integ *model.VCSIntegration, p prPayload) error {
	replyTarget := map[string]any{
		"kind":           "vcs_pr_thread",
		"integration_id": integ.ID.String(),
		"pr_number":      p.PullRequest.Number,
		"host":           integ.Host,
		"owner":          integ.Owner,
		"repo":           integ.Repo,
	}
	req := &model.TriggerReviewRequest{
		Trigger:       "vcs_webhook",
		PRURL:         strings.TrimSpace(p.PullRequest.HTMLURL),
		PRNumber:      p.PullRequest.Number,
		ProjectID:     integ.ProjectID.String(),
		IntegrationID: integ.ID.String(),
		HeadSHA:       p.PullRequest.Head.SHA,
		BaseSHA:       p.PullRequest.Base.SHA,
		ReplyTarget:   replyTarget,
	}
	if integ.ActingEmployeeID != nil {
		req.ActingEmployeeID = integ.ActingEmployeeID.String()
	}
	_, err := r.reviews.Trigger(ctx, req)
	return err
}
