package strategy

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type mockSeedRepo struct {
	mu          sync.Mutex
	rows        []*QianchuanStrategy
	insertErr   error
	overrideErr error
	insertCount int
}

func (m *mockSeedRepo) FindByProjectAndName(_ context.Context, projectID *uuid.UUID, name string) ([]*QianchuanStrategy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.overrideErr != nil {
		return nil, m.overrideErr
	}
	out := []*QianchuanStrategy{}
	for _, r := range m.rows {
		matchProj := (projectID == nil && r.ProjectID == nil) ||
			(projectID != nil && r.ProjectID != nil && *projectID == *r.ProjectID)
		if matchProj && r.Name == name {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *mockSeedRepo) Insert(_ context.Context, s *QianchuanStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.insertErr != nil {
		return m.insertErr
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	cp := *s
	m.rows = append(m.rows, &cp)
	m.insertCount++
	return nil
}

func TestSeedSystemStrategiesIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := &mockSeedRepo{}

	// First run inserts both seeds.
	if err := SeedSystemStrategies(ctx, repo); err != nil {
		t.Fatalf("SeedSystemStrategies (first): %v", err)
	}
	if got := len(repo.rows); got != 2 {
		t.Fatalf("after first run: got %d rows want 2", got)
	}
	for _, r := range repo.rows {
		if r.ProjectID != nil {
			t.Errorf("seed row has non-nil project id: %+v", r)
		}
		if r.Status != StatusPublished {
			t.Errorf("seed row status: got %q want published", r.Status)
		}
	}

	// Second run is a no-op.
	beforeCount := repo.insertCount
	if err := SeedSystemStrategies(ctx, repo); err != nil {
		t.Fatalf("SeedSystemStrategies (second): %v", err)
	}
	if repo.insertCount != beforeCount {
		t.Errorf("second run inserted %d new rows; expected 0", repo.insertCount-beforeCount)
	}
	if len(repo.rows) != 2 {
		t.Errorf("after second run: got %d rows want 2", len(repo.rows))
	}
}

func TestSeedNamesMatchEmbeddedFiles(t *testing.T) {
	ctx := context.Background()
	repo := &mockSeedRepo{}
	if err := SeedSystemStrategies(ctx, repo); err != nil {
		t.Fatalf("seed: %v", err)
	}
	names := map[string]bool{}
	for _, r := range repo.rows {
		names[r.Name] = true
	}
	expected := []string{"system:monitor-only", "system:conservative-bid-optimizer"}
	for _, want := range expected {
		if !names[want] {
			t.Errorf("missing seed %q (have %+v)", want, names)
		}
		if !strings.HasPrefix(want, "system:") {
			t.Errorf("seed name not system:-prefixed: %s", want)
		}
	}
}
