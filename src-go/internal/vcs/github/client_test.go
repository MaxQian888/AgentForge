package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/vcs"
	ghimpl "github.com/agentforge/server/internal/vcs/github"
)

func newServer(t *testing.T, h http.Handler) (*httptest.Server, *ghimpl.Client) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := ghimpl.NewClient(srv.URL+"/", "test-pat")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return srv, c
}

func TestGetPullRequest_Happy(t *testing.T) {
	_, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/repos/o/r/pulls/42") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-pat" {
			t.Errorf("expected bearer auth, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number":   42,
			"title":    "T",
			"state":    "open",
			"html_url": "https://github.com/o/r/pull/42",
			"base":     map[string]any{"ref": "main", "sha": "BASE"},
			"head":     map[string]any{"ref": "feat", "sha": "HEAD"},
			"user":     map[string]any{"login": "alice"},
		})
	}))
	pr, err := c.GetPullRequest(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}, 42)
	if err != nil {
		t.Fatalf("GetPullRequest: %v", err)
	}
	if pr.Number != 42 || pr.BaseSHA != "BASE" || pr.HeadSHA != "HEAD" || pr.AuthorLogin != "alice" {
		t.Errorf("unexpected PR mapping: %+v", pr)
	}
}

func TestGetPullRequest_AuthExpiredMaps401(t *testing.T) {
	_, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	_, err := c.GetPullRequest(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}, 1)
	if !errors.Is(err, vcs.ErrAuthExpired) {
		t.Fatalf("expected ErrAuthExpired, got %v", err)
	}
}

func TestPostSummaryComment_ReturnsID(t *testing.T) {
	_, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 9911})
	}))
	id, err := c.PostSummaryComment(context.Background(), &vcs.PullRequest{Number: 42, URL: "https://github.com/o/r/pull/42"}, "summary")
	if err != nil {
		t.Fatalf("PostSummaryComment: %v", err)
	}
	if id != "9911" {
		t.Errorf("expected id=9911, got %q", id)
	}
}

func TestRateLimitedMaps429WithRetryAfter(t *testing.T) {
	_, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "37")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	_, err := c.GetPullRequest(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}, 1)
	if !errors.Is(err, vcs.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	var rl *vcs.RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitedError, got %v", err)
	}
	if rl.RetryAfter.Seconds() != 37 {
		t.Errorf("expected 37s RetryAfter, got %v", rl.RetryAfter)
	}
}

func TestCreateWebhook_PassesSecret(t *testing.T) {
	var bodyJSON map[string]any
	_, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&bodyJSON)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 4242})
	}))
	id, err := c.CreateWebhook(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"},
		"https://agentforge.acme.corp/api/v1/vcs/github/webhook", "shh", []string{"pull_request", "push"})
	if err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}
	if id != "4242" {
		t.Errorf("expected hook id 4242, got %q", id)
	}
	cfg, _ := bodyJSON["config"].(map[string]any)
	if cfg["secret"] != "shh" || cfg["url"] == "" {
		t.Errorf("expected config to carry secret + url; got %+v", cfg)
	}
}

func TestDeleteWebhook_HitsCorrectPath(t *testing.T) {
	var seen string
	_, c := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Method + " " + r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := c.DeleteWebhook(context.Background(), vcs.RepoRef{Host: "github.com", Owner: "o", Repo: "r"}, "9999"); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}
	if !strings.Contains(seen, "DELETE") || !strings.Contains(seen, "/repos/o/r/hooks/9999") {
		t.Errorf("expected DELETE /repos/o/r/hooks/9999, got %q", seen)
	}
}
