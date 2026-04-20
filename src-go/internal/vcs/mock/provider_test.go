package mock_test

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/vcs"
	"github.com/agentforge/server/internal/vcs/mock"
)

func TestMockProvider_RecordsCalls(t *testing.T) {
	p := mock.New()
	ctx := context.Background()
	repo := vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}

	if _, err := p.PostSummaryComment(ctx, &vcs.PullRequest{Number: 7}, "hello"); err != nil {
		t.Fatalf("PostSummaryComment: %v", err)
	}
	_, _ = p.PostReviewComments(ctx, &vcs.PullRequest{Number: 7}, []vcs.InlineComment{{Path: "a.go", Line: 10, Body: "x", Side: "RIGHT"}})
	_, _ = p.OpenPR(ctx, repo, "main", "fix/abc", "title", "body", vcs.OpenPROpts{Labels: []string{"agentforge:fix"}})

	calls := p.Calls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}
	if calls[0].Op != "PostSummaryComment" || calls[0].Args["body"] != "hello" {
		t.Errorf("call[0] mismatch: %+v", calls[0])
	}
	if calls[2].Op != "OpenPR" {
		t.Errorf("call[2] expected OpenPR, got %s", calls[2].Op)
	}
}

func TestMockProvider_ScriptedError(t *testing.T) {
	p := mock.New()
	p.NextError(vcs.ErrAuthExpired)
	_, err := p.PostSummaryComment(context.Background(), &vcs.PullRequest{Number: 1}, "x")
	if err == nil {
		t.Fatal("expected scripted error")
	}
	if err != vcs.ErrAuthExpired {
		t.Errorf("expected ErrAuthExpired, got %v", err)
	}
	// Subsequent call must succeed (script consumed once).
	if _, err := p.PostSummaryComment(context.Background(), &vcs.PullRequest{Number: 1}, "y"); err != nil {
		t.Errorf("unexpected error after script consumed: %v", err)
	}
}

func TestMockProvider_WebhookSecretIsNotRecorded(t *testing.T) {
	p := mock.New()
	if _, err := p.CreateWebhook(context.Background(),
		vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"},
		"https://example/cb", "super-secret-do-not-leak", []string{"pull_request"}); err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}
	for _, c := range p.Calls() {
		for k, v := range c.Args {
			if s, ok := v.(string); ok && s == "super-secret-do-not-leak" {
				t.Fatalf("secret leaked into recorded call args at key %q", k)
			}
		}
	}
}
