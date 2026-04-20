// Package gitlab is a placeholder implementation of vcs.Provider for
// GitLab. Every method returns errors.ErrUnsupported. The stub is
// registered with the vcs.Registry so configurations referring to
// "gitlab" surface a clean error rather than a nil-pointer panic.
package gitlab

import (
	"context"
	"errors"

	"github.com/react-go-quick-starter/server/internal/vcs"
)

// Stub satisfies vcs.Provider with errors.ErrUnsupported on every method.
type Stub struct{ host string }

// NewStub is the registry Constructor entry point.
func NewStub(host, _ string) (vcs.Provider, error) { return &Stub{host: host}, nil }

// Name implements vcs.Provider.
func (s *Stub) Name() string { return "gitlab" }

// GetPullRequest implements vcs.Provider.
func (s *Stub) GetPullRequest(context.Context, vcs.RepoRef, int) (*vcs.PullRequest, error) {
	return nil, errors.ErrUnsupported
}

// ComparePullRequest implements vcs.Provider.
func (s *Stub) ComparePullRequest(context.Context, vcs.RepoRef, string, string) (*vcs.Diff, error) {
	return nil, errors.ErrUnsupported
}

// PostSummaryComment implements vcs.Provider.
func (s *Stub) PostSummaryComment(context.Context, *vcs.PullRequest, string) (string, error) {
	return "", errors.ErrUnsupported
}

// EditSummaryComment implements vcs.Provider.
func (s *Stub) EditSummaryComment(context.Context, *vcs.PullRequest, string, string) error {
	return errors.ErrUnsupported
}

// PostReviewComments implements vcs.Provider.
func (s *Stub) PostReviewComments(context.Context, *vcs.PullRequest, []vcs.InlineComment) ([]string, error) {
	return nil, errors.ErrUnsupported
}

// EditReviewComment implements vcs.Provider.
func (s *Stub) EditReviewComment(context.Context, *vcs.PullRequest, string, string) error {
	return errors.ErrUnsupported
}

// OpenPR implements vcs.Provider.
func (s *Stub) OpenPR(context.Context, vcs.RepoRef, string, string, string, string, vcs.OpenPROpts) (*vcs.PullRequest, error) {
	return nil, errors.ErrUnsupported
}

// CreateWebhook implements vcs.Provider.
func (s *Stub) CreateWebhook(context.Context, vcs.RepoRef, string, string, []string) (string, error) {
	return "", errors.ErrUnsupported
}

// DeleteWebhook implements vcs.Provider.
func (s *Stub) DeleteWebhook(context.Context, vcs.RepoRef, string) error {
	return errors.ErrUnsupported
}
