package liveartifact

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubProjector struct {
	kind LiveArtifactKind
}

func (s stubProjector) Kind() LiveArtifactKind { return s.kind }
func (s stubProjector) RequiredRole() Role     { return RoleViewer }
func (s stubProjector) Project(
	_ context.Context,
	_ model.PrincipalContext,
	_ uuid.UUID,
	_ json.RawMessage,
	_ json.RawMessage,
) (ProjectionResult, error) {
	return ProjectionResult{Status: StatusOK}, nil
}
func (s stubProjector) Subscribe(_ json.RawMessage) []EventTopic { return nil }

func TestRegistryLookup(t *testing.T) {
	r := NewRegistry()
	p := stubProjector{kind: KindAgentRun}
	r.Register(p)

	got, ok := r.Lookup(KindAgentRun)
	if !ok {
		t.Fatal("expected projector to be found")
	}
	if got.Kind() != KindAgentRun {
		t.Fatalf("kind mismatch: %s", got.Kind())
	}
}

func TestRegistryLookupMiss(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Lookup(KindReview); ok {
		t.Fatal("expected miss on empty registry")
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(stubProjector{kind: KindAgentRun})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register(stubProjector{kind: KindAgentRun})
}

func TestRegistryNilPanics(t *testing.T) {
	r := NewRegistry()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil projector")
		}
	}()
	r.Register(nil)
}

func TestPrincipalHasRole(t *testing.T) {
	cases := []struct {
		role     string
		required Role
		want     bool
	}{
		{"viewer", RoleViewer, true},
		{"viewer", RoleEditor, false},
		{"viewer", RoleAdmin, false},
		{"editor", RoleViewer, true},
		{"editor", RoleEditor, true},
		{"editor", RoleAdmin, false},
		{"admin", RoleAdmin, true},
		{"owner", RoleAdmin, true},
		{"", RoleViewer, false},
	}
	for _, tc := range cases {
		pc := model.PrincipalContext{ProjectRole: tc.role}
		if got := PrincipalHasRole(pc, tc.required); got != tc.want {
			t.Errorf("role=%s required=%s: got %v want %v", tc.role, tc.required, got, tc.want)
		}
	}
}
