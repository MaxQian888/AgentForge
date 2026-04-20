package qianchuanbinding_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/qianchuanbinding"
)

type memRepo struct{ rows map[uuid.UUID]*qianchuanbinding.Record }

func newMem() *memRepo { return &memRepo{rows: map[uuid.UUID]*qianchuanbinding.Record{}} }

func (m *memRepo) Create(_ context.Context, r *qianchuanbinding.Record) error {
	for _, ex := range m.rows {
		if ex.ProjectID == r.ProjectID && ex.AdvertiserID == r.AdvertiserID && ex.AwemeID == r.AwemeID {
			return qianchuanbinding.ErrAdvertiserAlreadyBound
		}
	}
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.Status == "" {
		r.Status = qianchuanbinding.StatusActive
	}
	cp := *r
	m.rows[r.ID] = &cp
	return nil
}

func (m *memRepo) Get(_ context.Context, id uuid.UUID) (*qianchuanbinding.Record, error) {
	r, ok := m.rows[id]
	if !ok {
		return nil, qianchuanbinding.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (m *memRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*qianchuanbinding.Record, error) {
	out := []*qianchuanbinding.Record{}
	for _, r := range m.rows {
		if r.ProjectID == projectID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memRepo) Update(_ context.Context, r *qianchuanbinding.Record) error {
	if _, ok := m.rows[r.ID]; !ok {
		return qianchuanbinding.ErrNotFound
	}
	cp := *r
	m.rows[r.ID] = &cp
	return nil
}

func (m *memRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.rows[id]; !ok {
		return qianchuanbinding.ErrNotFound
	}
	delete(m.rows, id)
	return nil
}

func (m *memRepo) TouchSync(_ context.Context, id uuid.UUID, when time.Time) error {
	r, ok := m.rows[id]
	if !ok {
		return qianchuanbinding.ErrNotFound
	}
	r.LastSyncedAt = &when
	return nil
}

func TestRepo_Contract(t *testing.T) {
	var _ qianchuanbinding.Repository = newMem()
}

func TestRepo_DuplicateRejected(t *testing.T) {
	m := newMem()
	ctx := context.Background()
	proj := uuid.New()
	r := &qianchuanbinding.Record{ProjectID: proj, AdvertiserID: "A1", AwemeID: "W1", AccessTokenSecretRef: "s.access", RefreshTokenSecretRef: "s.refresh", CreatedBy: uuid.New(), Status: "active"}
	if err := m.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := m.Create(ctx, &qianchuanbinding.Record{ProjectID: proj, AdvertiserID: "A1", AwemeID: "W1", AccessTokenSecretRef: "s.access", RefreshTokenSecretRef: "s.refresh", CreatedBy: uuid.New(), Status: "active"}); err != qianchuanbinding.ErrAdvertiserAlreadyBound {
		t.Fatalf("want ErrAdvertiserAlreadyBound, got %v", err)
	}
}
