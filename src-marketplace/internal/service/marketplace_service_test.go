package service_test

import (
	"testing"

	"github.com/agentforge/marketplace/internal/service"
)

// TestErrorVarsExported verifies that all sentinel errors are exported and non-nil.
func TestErrorVarsExported(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotItemOwner", service.ErrNotItemOwner},
		{"ErrSlugTaken", service.ErrSlugTaken},
		{"ErrInvalidSemver", service.ErrInvalidSemver},
		{"ErrVersionYanked", service.ErrVersionYanked},
		{"ErrVersionNotFound", service.ErrVersionNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Errorf("%s should be non-nil", tc.name)
			}
		})
	}
}

// TestErrorMessages verifies that sentinel errors have distinct, non-empty messages.
func TestErrorMessages(t *testing.T) {
	seen := map[string]bool{}
	errs := []error{
		service.ErrNotItemOwner,
		service.ErrSlugTaken,
		service.ErrInvalidSemver,
		service.ErrVersionYanked,
		service.ErrVersionNotFound,
	}
	for _, e := range errs {
		msg := e.Error()
		if msg == "" {
			t.Errorf("error %T has empty message", e)
		}
		if seen[msg] {
			t.Errorf("duplicate error message: %q", msg)
		}
		seen[msg] = true
	}
}
