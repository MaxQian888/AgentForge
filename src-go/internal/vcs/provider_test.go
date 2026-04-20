package vcs_test

import (
	"errors"
	"testing"

	"github.com/react-go-quick-starter/server/internal/vcs"
)

func TestErrorSentinelsAreDistinct(t *testing.T) {
	if errors.Is(vcs.ErrAuthExpired, vcs.ErrRateLimited) {
		t.Fatal("ErrAuthExpired and ErrRateLimited must be distinct")
	}
	if vcs.ErrAuthExpired.Error() != "vcs:auth_expired" {
		t.Errorf("ErrAuthExpired must serialize as vcs:auth_expired, got %q", vcs.ErrAuthExpired.Error())
	}
	if vcs.ErrRateLimited.Error() != "vcs:rate_limited" {
		t.Errorf("ErrRateLimited must serialize as vcs:rate_limited, got %q", vcs.ErrRateLimited.Error())
	}
	if vcs.ErrTransientFailure.Error() != "vcs:transient_failure" {
		t.Errorf("ErrTransientFailure must serialize as vcs:transient_failure, got %q", vcs.ErrTransientFailure.Error())
	}
}
