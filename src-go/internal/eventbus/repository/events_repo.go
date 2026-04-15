package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
	eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type EventsRepository struct {
	db *sqlx.DB
}

func NewEventsRepository(db *sqlx.DB) *EventsRepository {
	return &EventsRepository{db: db}
}

const insertSQL = `
INSERT INTO events (id, type, source, target, visibility, payload, metadata, project_id, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO NOTHING
`

func (r *EventsRepository) Insert(ctx context.Context, e *eb.Event) error {
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	var projectID sql.NullString
	if pid := eb.GetString(e, eb.MetaProjectID); pid != "" {
		projectID.String = pid
		projectID.Valid = true
	}
	_, err = r.db.ExecContext(ctx, insertSQL,
		e.ID, e.Type, e.Source, e.Target, string(e.Visibility),
		[]byte(e.Payload), meta, projectID, e.Timestamp,
	)
	return err
}

const findByIDSQL = `SELECT id, type, source, target, visibility, payload, metadata, occurred_at FROM events WHERE id = $1`

type eventRow struct {
	ID         string `db:"id"`
	Type       string `db:"type"`
	Source     string `db:"source"`
	Target     string `db:"target"`
	Visibility string `db:"visibility"`
	Payload    []byte `db:"payload"`
	Metadata   []byte `db:"metadata"`
	OccurredAt int64  `db:"occurred_at"`
}

func (r *EventsRepository) FindByID(ctx context.Context, id string) (*eb.Event, error) {
	var rr eventRow
	if err := r.db.GetContext(ctx, &rr, findByIDSQL, id); err != nil {
		return nil, err
	}
	e := &eb.Event{
		ID: rr.ID, Type: rr.Type, Source: rr.Source, Target: rr.Target,
		Visibility: eb.Visibility(rr.Visibility),
		Payload:    json.RawMessage(rr.Payload),
		Timestamp:  rr.OccurredAt,
	}
	if len(rr.Metadata) > 0 {
		_ = json.Unmarshal(rr.Metadata, &e.Metadata)
	}
	return e, nil
}
