package gitlab_test

import (
	"context"
	"errors"
	"testing"

	"github.com/react-go-quick-starter/server/internal/vcs"
	"github.com/react-go-quick-starter/server/internal/vcs/gitlab"
)

func TestStubReturnsUnsupportedFromEveryMethod(t *testing.T) {
	s, err := gitlab.NewStub("gitlab.com", "tok")
	if err != nil {
		t.Fatalf("NewStub: %v", err)
	}
	if s.Name() != "gitlab" {
		t.Errorf("Name(): got %q want %q", s.Name(), "gitlab")
	}
	repo := vcs.RepoRef{Host: "gitlab.com", Owner: "o", Repo: "r"}
	pr := &vcs.PullRequest{Number: 1, URL: "https://gitlab.com/o/r/-/merge_requests/1"}

	cases := []func() error{
		func() error { _, err := s.GetPullRequest(context.Background(), repo, 1); return err },
		func() error { _, err := s.ComparePullRequest(context.Background(), repo, "a", "b"); return err },
		func() error { _, err := s.PostSummaryComment(context.Background(), pr, "x"); return err },
		func() error { return s.EditSummaryComment(context.Background(), pr, "1", "x") },
		func() error { _, err := s.PostReviewComments(context.Background(), pr, nil); return err },
		func() error { return s.EditReviewComment(context.Background(), pr, "1", "x") },
		func() error {
			_, err := s.OpenPR(context.Background(), repo, "main", "f", "t", "b", vcs.OpenPROpts{})
			return err
		},
		func() error { _, err := s.CreateWebhook(context.Background(), repo, "u", "s", nil); return err },
		func() error { return s.DeleteWebhook(context.Background(), repo, "1") },
	}
	for i, fn := range cases {
		if err := fn(); !errors.Is(err, errors.ErrUnsupported) {
			t.Errorf("case %d: expected ErrUnsupported, got %v", i, err)
		}
	}
}
