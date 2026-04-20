package vcs

import (
	"context"
	"errors"
	"time"
)

// Provider is the provider-neutral surface for source-control hosts.
// Implementations MUST resolve credentials at call time, never cache
// plaintext PAT on the struct, and translate host-specific errors
// through the typed sentinels declared below.
type Provider interface {
	Name() string

	// PR lifecycle.
	GetPullRequest(ctx context.Context, repo RepoRef, number int) (*PullRequest, error)
	ComparePullRequest(ctx context.Context, repo RepoRef, base, head string) (*Diff, error)

	// Summary comment (one per review) — kept editable for diff-of-diff.
	PostSummaryComment(ctx context.Context, pr *PullRequest, body string) (commentID string, err error)
	EditSummaryComment(ctx context.Context, pr *PullRequest, commentID string, body string) error

	// Inline review comments (one per finding).
	PostReviewComments(ctx context.Context, pr *PullRequest, comments []InlineComment) (ids []string, err error)
	EditReviewComment(ctx context.Context, pr *PullRequest, commentID string, body string) error

	// Fix-PR opening (used by 2E).
	OpenPR(ctx context.Context, repo RepoRef, base, head, title, body string, opts OpenPROpts) (*PullRequest, error)

	// Webhook lifecycle.
	CreateWebhook(ctx context.Context, repo RepoRef, callbackURL, secret string, events []string) (id string, err error)
	DeleteWebhook(ctx context.Context, repo RepoRef, id string) error
}

// ErrAuthExpired indicates the credential resolved at call time is no
// longer accepted by the host (401/403). Callers should mark
// vcs_integrations.status = 'auth_expired' and pause downstream work.
var ErrAuthExpired = errors.New("vcs:auth_expired")

// ErrRateLimited indicates the host returned 429. Wrap with
// RateLimitedError to convey Retry-After hints.
var ErrRateLimited = errors.New("vcs:rate_limited")

// ErrTransientFailure indicates a 5xx or network-level failure that the
// caller should retry with backoff.
var ErrTransientFailure = errors.New("vcs:transient_failure")

// ErrProviderUnsupported is returned by the registry when a stubbed
// provider (gitlab/gitea pre-impl) is requested.
var ErrProviderUnsupported = errors.New("vcs:provider_unsupported")

// RateLimitedError wraps ErrRateLimited with the Retry-After hint
// returned by the host. Callers may use errors.As to extract.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string { return ErrRateLimited.Error() }
func (e *RateLimitedError) Unwrap() error { return ErrRateLimited }
