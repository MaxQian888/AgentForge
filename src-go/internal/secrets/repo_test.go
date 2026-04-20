package secrets_test

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/secrets"
	"github.com/google/uuid"
)

// memRepo is the in-memory test double used here AND by service_test.go.
// Real DB coverage lives in repo_integration_test.go (build tag).
type memRepo struct{ rows map[string]*secrets.Record }

func newMemRepo() *memRepo { return &memRepo{rows: map[string]*secrets.Record{}} }

func key(p uuid.UUID, n string) string { return p.String() + "|" + n }

func (m *memRepo) Create(_ context.Context, r *secrets.Record) error {
	if _, ok := m.rows[key(r.ProjectID, r.Name)]; ok {
		return secrets.ErrNameConflict
	}
	cp := *r
	m.rows[key(r.ProjectID, r.Name)] = &cp
	return nil
}
func (m *memRepo) Get(_ context.Context, p uuid.UUID, n string) (*secrets.Record, error) {
	r, ok := m.rows[key(p, n)]
	if !ok {
		return nil, secrets.ErrNotFound
	}
	cp := *r
	return &cp, nil
}
func (m *memRepo) List(_ context.Context, p uuid.UUID) ([]*secrets.Record, error) {
	var out []*secrets.Record
	for _, r := range m.rows {
		if r.ProjectID == p {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (m *memRepo) Update(_ context.Context, r *secrets.Record) error {
	k := key(r.ProjectID, r.Name)
	if _, ok := m.rows[k]; !ok {
		return secrets.ErrNotFound
	}
	cp := *r
	m.rows[k] = &cp
	return nil
}
func (m *memRepo) Delete(_ context.Context, p uuid.UUID, n string) error {
	delete(m.rows, key(p, n))
	return nil
}
func (m *memRepo) TouchLastUsed(_ context.Context, p uuid.UUID, n string, when time.Time) error {
	r, ok := m.rows[key(p, n)]
	if !ok {
		return secrets.ErrNotFound
	}
	r.LastUsedAt = &when
	return nil
}

func TestMemRepoContractCompiles(t *testing.T) {
	var _ secrets.Repository = newMemRepo()
}
