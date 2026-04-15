package repository

import (
	"context"
	"encoding/json"

	"github.com/jmoiron/sqlx"
	eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

type DeadLetterRepository struct {
	db *sqlx.DB
}

func NewDeadLetterRepository(db *sqlx.DB) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

const dlqInsertSQL = `INSERT INTO events_dead_letter (event_id, envelope, last_error, retry_count) VALUES ($1, $2, $3, $4)`

func (r *DeadLetterRepository) Record(ctx context.Context, e *eb.Event, err error, retries int) error {
	env, jerr := json.Marshal(e)
	if jerr != nil {
		return jerr
	}
	_, dberr := r.db.ExecContext(ctx, dlqInsertSQL, e.ID, env, err.Error(), retries)
	return dberr
}
