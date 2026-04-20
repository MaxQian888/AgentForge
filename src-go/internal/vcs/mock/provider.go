// Package mock supplies a recording vcs.Provider for tests. Every
// outbound method appends a Call to the recording so tests can assert
// on the full interaction sequence. Use NextError to script a single
// failure for the next call (resets after one consumption).
package mock

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/react-go-quick-starter/server/internal/vcs"
)

// Call captures one provider invocation.
type Call struct {
	Op   string
	Args map[string]any
}

// Provider is a recording, in-memory vcs.Provider.
type Provider struct {
	mu        sync.Mutex
	calls     []Call
	nextErr   atomic.Value // error
	commentID atomic.Int64
	prID      atomic.Int64
}

// New returns an empty recorder.
func New() *Provider { return &Provider{} }

// Calls returns a snapshot of recorded calls.
func (p *Provider) Calls() []Call {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Call, len(p.calls))
	copy(out, p.calls)
	return out
}

// NextError scripts err to be returned by the next outbound call.
func (p *Provider) NextError(err error) { p.nextErr.Store(errBox{err: err}) }

// errBox lets us distinguish "no scripted error" from "scripted nil"
// via atomic.Value (which refuses inconsistent types and rejects nil).
type errBox struct{ err error }

func (p *Provider) consumeErr() error {
	v := p.nextErr.Swap(errBox{})
	if v == nil {
		return nil
	}
	box, ok := v.(errBox)
	if !ok {
		return nil
	}
	return box.err
}

func (p *Provider) record(op string, args map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, Call{Op: op, Args: args})
}

// Name implements vcs.Provider.
func (p *Provider) Name() string { return "mock" }

// GetPullRequest implements vcs.Provider.
func (p *Provider) GetPullRequest(ctx context.Context, repo vcs.RepoRef, n int) (*vcs.PullRequest, error) {
	p.record("GetPullRequest", map[string]any{"repo": repo.String(), "n": n})
	if err := p.consumeErr(); err != nil {
		return nil, err
	}
	return &vcs.PullRequest{
		Number:     n,
		BaseBranch: "main",
		BaseSHA:    "base",
		HeadSHA:    "head",
		State:      "open",
		URL:        "https://mock/pr/" + repo.String(),
	}, nil
}

// ComparePullRequest implements vcs.Provider.
func (p *Provider) ComparePullRequest(ctx context.Context, repo vcs.RepoRef, base, head string) (*vcs.Diff, error) {
	p.record("ComparePullRequest", map[string]any{"repo": repo.String(), "base": base, "head": head})
	if err := p.consumeErr(); err != nil {
		return nil, err
	}
	return &vcs.Diff{BaseSHA: base, HeadSHA: head}, nil
}

// PostSummaryComment implements vcs.Provider.
func (p *Provider) PostSummaryComment(ctx context.Context, pr *vcs.PullRequest, body string) (string, error) {
	p.record("PostSummaryComment", map[string]any{"pr": pr.Number, "body": body})
	if err := p.consumeErr(); err != nil {
		return "", err
	}
	id := p.commentID.Add(1)
	return "summary-" + strconv.FormatInt(id, 10), nil
}

// EditSummaryComment implements vcs.Provider.
func (p *Provider) EditSummaryComment(ctx context.Context, pr *vcs.PullRequest, id, body string) error {
	p.record("EditSummaryComment", map[string]any{"pr": pr.Number, "id": id, "body": body})
	return p.consumeErr()
}

// PostReviewComments implements vcs.Provider.
func (p *Provider) PostReviewComments(ctx context.Context, pr *vcs.PullRequest, comments []vcs.InlineComment) ([]string, error) {
	p.record("PostReviewComments", map[string]any{"pr": pr.Number, "count": len(comments)})
	if err := p.consumeErr(); err != nil {
		return nil, err
	}
	out := make([]string, len(comments))
	for i := range comments {
		out[i] = "inline-" + strconv.FormatInt(p.commentID.Add(1), 10)
	}
	return out, nil
}

// EditReviewComment implements vcs.Provider.
func (p *Provider) EditReviewComment(ctx context.Context, pr *vcs.PullRequest, id, body string) error {
	p.record("EditReviewComment", map[string]any{"pr": pr.Number, "id": id, "body": body})
	return p.consumeErr()
}

// OpenPR implements vcs.Provider.
func (p *Provider) OpenPR(ctx context.Context, repo vcs.RepoRef, base, head, title, body string, opts vcs.OpenPROpts) (*vcs.PullRequest, error) {
	p.record("OpenPR", map[string]any{
		"repo":   repo.String(),
		"base":   base,
		"head":   head,
		"title":  title,
		"labels": opts.Labels,
	})
	if err := p.consumeErr(); err != nil {
		return nil, err
	}
	n := int(p.prID.Add(1))
	return &vcs.PullRequest{
		Number:     n,
		Title:      title,
		BaseBranch: base,
		HeadBranch: head,
		State:      "open",
		URL:        "https://mock/pr/" + strconv.Itoa(n),
	}, nil
}

// CreateWebhook implements vcs.Provider. The raw secret value is never
// recorded so test assertions cannot accidentally print it.
func (p *Provider) CreateWebhook(ctx context.Context, repo vcs.RepoRef, cb, secret string, events []string) (string, error) {
	_ = secret
	p.record("CreateWebhook", map[string]any{"repo": repo.String(), "callback": cb, "events": events})
	if err := p.consumeErr(); err != nil {
		return "", err
	}
	return "hook-" + strconv.FormatInt(p.commentID.Add(1), 10), nil
}

// DeleteWebhook implements vcs.Provider.
func (p *Provider) DeleteWebhook(ctx context.Context, repo vcs.RepoRef, id string) error {
	p.record("DeleteWebhook", map[string]any{"repo": repo.String(), "id": id})
	return p.consumeErr()
}
