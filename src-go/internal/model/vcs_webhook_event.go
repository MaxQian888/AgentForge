package model

import (
	"time"

	"github.com/google/uuid"
)

// VCSWebhookEvent records an inbound webhook delivery for deduplication
// and audit (Spec 2B §6.1). The UNIQUE(integration_id, event_id)
// constraint is the single dedup gate.
type VCSWebhookEvent struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	IntegrationID   uuid.UUID  `db:"integration_id" json:"integrationId"`
	EventID         string     `db:"event_id" json:"eventId"`
	EventType       string     `db:"event_type" json:"eventType"`
	PayloadHash     []byte     `db:"payload_hash" json:"-"`
	ReceivedAt      time.Time  `db:"received_at" json:"receivedAt"`
	ProcessedAt     *time.Time `db:"processed_at" json:"processedAt,omitempty"`
	ProcessingError string     `db:"processing_error" json:"processingError,omitempty"`
}
