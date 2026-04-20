package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// QianchuanMetricSnapshot is the in-memory representation of one
// qianchuan_metric_snapshots row. Minute-bucketed time-series data
// per binding for metrics charts and strategy evaluation.
type QianchuanMetricSnapshot struct {
	ID           int64           `db:"id" json:"id"`
	BindingID    uuid.UUID       `db:"binding_id" json:"bindingId"`
	MinuteBucket time.Time       `db:"minute_bucket" json:"minuteBucket"`
	Payload      json.RawMessage `db:"payload" json:"payload"`
	CreatedAt    time.Time       `db:"created_at" json:"createdAt"`
}
