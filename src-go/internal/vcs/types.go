// Package vcs is the provider-neutral seam for source-control hosts
// (GitHub, GitLab, Gitea, ...). Concrete implementations live under
// internal/vcs/<provider>/. Tests use internal/vcs/mock.
//
// Spec reference: docs/superpowers/specs/2026-04-20-code-reviewer-employee-design.md
//   §5 S2-G architecture, §8 Provider interface, §11 Security.
package vcs

import "fmt"

// RepoRef identifies one repository on one host.
type RepoRef struct {
	Host  string
	Owner string
	Repo  string
}

func (r RepoRef) String() string {
	return fmt.Sprintf("%s/%s/%s", r.Host, r.Owner, r.Repo)
}

// PullRequest is the provider-neutral PR snapshot. number is the
// host-side numeric identifier (#42).
type PullRequest struct {
	Number      int
	Title       string
	Body        string
	BaseBranch  string
	BaseSHA     string
	HeadBranch  string
	HeadSHA     string
	State       string // "open" | "closed" | "merged"
	URL         string
	AuthorLogin string
}

// Diff is a coarse PR-level diff used by ComparePullRequest. File
// entries follow GitHub's compare-API shape but the field names are
// provider-neutral so adapters can populate them directly.
type Diff struct {
	BaseSHA      string
	HeadSHA      string
	ChangedFiles []ChangedFile
}

// ChangedFile is one file's worth of compare-API metadata. patch is
// the unified-diff hunk for that file (may be empty for binary files
// or large changes that the host elides).
type ChangedFile struct {
	Path      string
	Status    string // "added" | "modified" | "removed" | "renamed"
	Additions int
	Deletions int
	Patch     string
}

// InlineComment is one PR-line comment. Side is "RIGHT" for added
// lines and "LEFT" for removed lines (matches GitHub's review-comment
// semantics; other providers map equivalently).
type InlineComment struct {
	Path string
	Line int
	Body string
	Side string
}

// OpenPROpts holds optional PR-creation flags. Zero value = non-draft,
// no auto-merge, no labels.
type OpenPROpts struct {
	Draft     bool
	AutoMerge bool
	Labels    []string
}
