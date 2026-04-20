package vcs_test

import (
	"testing"

	"github.com/react-go-quick-starter/server/internal/vcs"
)

func TestRepoRefStringIsStable(t *testing.T) {
	r := vcs.RepoRef{Host: "github.com", Owner: "octocat", Repo: "hello"}
	if r.String() != "github.com/octocat/hello" {
		t.Errorf("unexpected RepoRef.String(): %q", r.String())
	}
}

func TestInlineCommentDefaults(t *testing.T) {
	c := vcs.InlineComment{Path: "a.go", Line: 10, Body: "x"}
	if c.Side != "" {
		t.Errorf("expected zero-value Side, got %q", c.Side)
	}
}
