package vcs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/react-go-quick-starter/server/internal/vcs"
)

type stubProvider struct{ name string }

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) GetPullRequest(context.Context, vcs.RepoRef, int) (*vcs.PullRequest, error) {
	return nil, nil
}
func (s *stubProvider) ComparePullRequest(context.Context, vcs.RepoRef, string, string) (*vcs.Diff, error) {
	return nil, nil
}
func (s *stubProvider) PostSummaryComment(context.Context, *vcs.PullRequest, string) (string, error) {
	return "", nil
}
func (s *stubProvider) EditSummaryComment(context.Context, *vcs.PullRequest, string, string) error {
	return nil
}
func (s *stubProvider) PostReviewComments(context.Context, *vcs.PullRequest, []vcs.InlineComment) ([]string, error) {
	return nil, nil
}
func (s *stubProvider) EditReviewComment(context.Context, *vcs.PullRequest, string, string) error {
	return nil
}
func (s *stubProvider) OpenPR(context.Context, vcs.RepoRef, string, string, string, string, vcs.OpenPROpts) (*vcs.PullRequest, error) {
	return nil, nil
}
func (s *stubProvider) CreateWebhook(context.Context, vcs.RepoRef, string, string, []string) (string, error) {
	return "", nil
}
func (s *stubProvider) DeleteWebhook(context.Context, vcs.RepoRef, string) error { return nil }

func TestRegistry_RegisterAndResolve(t *testing.T) {
	reg := vcs.NewRegistry()
	reg.Register("github", func(host, token string) (vcs.Provider, error) {
		return &stubProvider{name: "github"}, nil
	})
	p, err := reg.Resolve("github", "github.com", "tok")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if p.Name() != "github" {
		t.Errorf("expected name=github, got %q", p.Name())
	}
}

func TestRegistry_UnknownProvider(t *testing.T) {
	reg := vcs.NewRegistry()
	_, err := reg.Resolve("svn", "x", "")
	if !errors.Is(err, vcs.ErrProviderUnsupported) {
		t.Fatalf("expected ErrProviderUnsupported, got %v", err)
	}
}

func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	reg := vcs.NewRegistry()
	reg.Register("x", func(string, string) (vcs.Provider, error) { return nil, nil })
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate Register")
		}
	}()
	reg.Register("x", func(string, string) (vcs.Provider, error) { return nil, nil })
}
