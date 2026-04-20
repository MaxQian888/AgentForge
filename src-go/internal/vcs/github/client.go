// Package github implements vcs.Provider for GitHub.com and GitHub
// Enterprise. The constructor takes a base URL (so this package works
// for both github.com and self-hosted) and a PAT plaintext. The PAT is
// resolved at the call site immediately before construction; this
// package MUST NOT cache it beyond the lifetime of the returned Client.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	gogh "github.com/google/go-github/v60/github"

	"github.com/agentforge/server/internal/vcs"
)

// Client wraps go-github's *Client with vcs.Provider semantics.
type Client struct {
	gh *gogh.Client
}

// NewClient builds a Client. baseURL may be "" for github.com, or a
// GitHub Enterprise base ("https://github.acme.corp/api/v3/"). pat is
// the resolved PAT; the constructor wires it via a Bearer token
// round-tripper so every request carries Authorization.
func NewClient(baseURL, pat string) (*Client, error) {
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &bearerTransport{token: pat, base: http.DefaultTransport},
	}
	gh := gogh.NewClient(httpClient)
	if baseURL != "" && baseURL != "https://api.github.com/" {
		// httptest servers and Enterprise both go through this branch;
		// the upload URL shares the same host because httptest serves
		// every path off a single mux.
		var err error
		gh, err = gh.WithEnterpriseURLs(baseURL, baseURL)
		if err != nil {
			return nil, fmt.Errorf("github: enterprise URL: %w", err)
		}
	}
	return &Client{gh: gh}, nil
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+b.token)
	req2.Header.Set("Accept", "application/vnd.github+json")
	return b.base.RoundTrip(req2)
}

// Name implements vcs.Provider.
func (c *Client) Name() string { return "github" }

// GetPullRequest implements vcs.Provider.
func (c *Client) GetPullRequest(ctx context.Context, repo vcs.RepoRef, n int) (*vcs.PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, repo.Owner, repo.Repo, n)
	if err != nil {
		return nil, mapErr(err)
	}
	return toPR(pr), nil
}

// ComparePullRequest implements vcs.Provider.
func (c *Client) ComparePullRequest(ctx context.Context, repo vcs.RepoRef, base, head string) (*vcs.Diff, error) {
	cmp, _, err := c.gh.Repositories.CompareCommits(ctx, repo.Owner, repo.Repo, base, head, nil)
	if err != nil {
		return nil, mapErr(err)
	}
	out := &vcs.Diff{
		BaseSHA: cmp.GetBaseCommit().GetSHA(),
		HeadSHA: head,
	}
	for _, f := range cmp.Files {
		out.ChangedFiles = append(out.ChangedFiles, vcs.ChangedFile{
			Path:      f.GetFilename(),
			Status:    f.GetStatus(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
			Patch:     f.GetPatch(),
		})
	}
	return out, nil
}

// PostSummaryComment implements vcs.Provider. Summary comments live in
// the issue-comment lane on GitHub (not the inline review-comment lane)
// so they appear once at the top of the PR conversation.
func (c *Client) PostSummaryComment(ctx context.Context, pr *vcs.PullRequest, body string) (string, error) {
	owner, repo, err := splitURL(pr.URL)
	if err != nil {
		return "", err
	}
	com, _, err := c.gh.Issues.CreateComment(ctx, owner, repo, pr.Number, &gogh.IssueComment{Body: gogh.String(body)})
	if err != nil {
		return "", mapErr(err)
	}
	return strconv.FormatInt(com.GetID(), 10), nil
}

// EditSummaryComment implements vcs.Provider.
func (c *Client) EditSummaryComment(ctx context.Context, pr *vcs.PullRequest, commentID, body string) error {
	owner, repo, err := splitURL(pr.URL)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(commentID, 10, 64)
	if err != nil {
		return fmt.Errorf("github: invalid commentID %q", commentID)
	}
	_, _, err = c.gh.Issues.EditComment(ctx, owner, repo, id, &gogh.IssueComment{Body: gogh.String(body)})
	return mapErr(err)
}

// PostReviewComments implements vcs.Provider. We post each inline
// comment individually so a single failure does not strand the others;
// the returned ids slice is partial-on-error so callers can resume.
func (c *Client) PostReviewComments(ctx context.Context, pr *vcs.PullRequest, comments []vcs.InlineComment) ([]string, error) {
	owner, repo, err := splitURL(pr.URL)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(comments))
	for _, ic := range comments {
		side := ic.Side
		if side == "" {
			side = "RIGHT"
		}
		review, _, err := c.gh.PullRequests.CreateComment(ctx, owner, repo, pr.Number, &gogh.PullRequestComment{
			CommitID: gogh.String(pr.HeadSHA),
			Path:     gogh.String(ic.Path),
			Line:     gogh.Int(ic.Line),
			Side:     gogh.String(side),
			Body:     gogh.String(ic.Body),
		})
		if err != nil {
			return ids, mapErr(err)
		}
		ids = append(ids, strconv.FormatInt(review.GetID(), 10))
	}
	return ids, nil
}

// EditReviewComment implements vcs.Provider.
func (c *Client) EditReviewComment(ctx context.Context, pr *vcs.PullRequest, commentID, body string) error {
	owner, repo, err := splitURL(pr.URL)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(commentID, 10, 64)
	if err != nil {
		return fmt.Errorf("github: invalid commentID %q", commentID)
	}
	_, _, err = c.gh.PullRequests.EditComment(ctx, owner, repo, id, &gogh.PullRequestComment{Body: gogh.String(body)})
	return mapErr(err)
}

// OpenPR implements vcs.Provider. Labels (when present) are applied via
// a follow-up call because the create endpoint does not accept labels;
// label-add failures are swallowed so a PR is never blocked by a missing
// label — callers see the returned PR and can retry the labelling.
func (c *Client) OpenPR(ctx context.Context, repo vcs.RepoRef, base, head, title, body string, opts vcs.OpenPROpts) (*vcs.PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, repo.Owner, repo.Repo, &gogh.NewPullRequest{
		Title: gogh.String(title),
		Body:  gogh.String(body),
		Base:  gogh.String(base),
		Head:  gogh.String(head),
		Draft: gogh.Bool(opts.Draft),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	if len(opts.Labels) > 0 {
		_, _, _ = c.gh.Issues.AddLabelsToIssue(ctx, repo.Owner, repo.Repo, pr.GetNumber(), opts.Labels)
	}
	return toPR(pr), nil
}

// CreateWebhook implements vcs.Provider. The HMAC secret is passed in
// the config map (per GitHub webhook API); GitHub stores it server-side
// and only echoes a redacted form back on subsequent reads.
func (c *Client) CreateWebhook(ctx context.Context, repo vcs.RepoRef, callbackURL, secret string, events []string) (string, error) {
	hook, _, err := c.gh.Repositories.CreateHook(ctx, repo.Owner, repo.Repo, &gogh.Hook{
		Name:   gogh.String("web"),
		Active: gogh.Bool(true),
		Events: events,
		Config: &gogh.HookConfig{
			URL:         gogh.String(callbackURL),
			ContentType: gogh.String("json"),
			Secret:      gogh.String(secret),
			InsecureSSL: gogh.String("0"),
		},
	})
	if err != nil {
		return "", mapErr(err)
	}
	return strconv.FormatInt(hook.GetID(), 10), nil
}

// DeleteWebhook implements vcs.Provider.
func (c *Client) DeleteWebhook(ctx context.Context, repo vcs.RepoRef, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("github: invalid webhook id %q", id)
	}
	_, err = c.gh.Repositories.DeleteHook(ctx, repo.Owner, repo.Repo, n)
	return mapErr(err)
}

// ---------- helpers ----------

func toPR(pr *gogh.PullRequest) *vcs.PullRequest {
	out := &vcs.PullRequest{
		Number:      pr.GetNumber(),
		Title:       pr.GetTitle(),
		Body:        pr.GetBody(),
		URL:         pr.GetHTMLURL(),
		State:       pr.GetState(),
		BaseBranch:  pr.GetBase().GetRef(),
		BaseSHA:     pr.GetBase().GetSHA(),
		HeadBranch:  pr.GetHead().GetRef(),
		HeadSHA:     pr.GetHead().GetSHA(),
		AuthorLogin: pr.GetUser().GetLogin(),
	}
	if pr.GetMerged() {
		out.State = "merged"
	}
	return out
}

// splitURL extracts (owner, repo) from a PR HTML URL like
// https://github.com/octocat/hello/pull/42. Enterprise hosts go through
// the same shape.
func splitURL(u string) (string, string, error) {
	i := strings.Index(u, "://")
	if i < 0 {
		return "", "", fmt.Errorf("github: bad PR URL %q", u)
	}
	rest := u[i+3:]
	parts := strings.Split(rest, "/")
	if len(parts) < 4 {
		return "", "", fmt.Errorf("github: bad PR URL %q", u)
	}
	// parts: [host, owner, repo, "pull", num]
	return parts[1], parts[2], nil
}

// mapErr translates go-github errors into vcs typed sentinels. Any
// unmapped err is returned as-is so callers can inspect.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var rerr *gogh.ErrorResponse
	if errors.As(err, &rerr) && rerr.Response != nil {
		switch rerr.Response.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return vcs.ErrAuthExpired
		case http.StatusTooManyRequests:
			ra := rerr.Response.Header.Get("Retry-After")
			d, _ := strconv.Atoi(ra)
			return &vcs.RateLimitedError{RetryAfter: time.Duration(d) * time.Second}
		}
		if rerr.Response.StatusCode >= 500 {
			return fmt.Errorf("%w: %s", vcs.ErrTransientFailure, rerr.Message)
		}
	}
	// go-github also returns *RateLimitError for primary rate-limit; treat
	// the same as 429.
	var rl *gogh.RateLimitError
	if errors.As(err, &rl) {
		return &vcs.RateLimitedError{}
	}
	return err
}
