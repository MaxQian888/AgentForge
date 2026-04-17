// Package audit provides structured audit logging for the IM Bridge.
// Events are emitted as append-only JSONL to a configurable directory so
// operators can reconstruct the timeline of every delivery, callback and
// action the bridge processes. The primary storage is a local file; an
// optional control-plane shipper can mirror events to the backend.
package audit

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// SchemaVersion is the current audit event schema. Incompatible changes
// MUST bump this number so downstream consumers can discriminate.
const SchemaVersion = 1

// Direction classifies an event relative to the bridge boundary.
type Direction string

const (
	DirectionIngress  Direction = "ingress"
	DirectionEgress   Direction = "egress"
	DirectionAction   Direction = "action"
	DirectionInternal Direction = "internal"
)

// Status is the outcome of the audited operation.
type Status string

const (
	StatusDelivered   Status = "delivered"
	StatusDuplicate   Status = "duplicate"
	StatusRejected    Status = "rejected"
	StatusRateLimited Status = "rate_limited"
	StatusDowngraded  Status = "downgraded"
	StatusFailed      Status = "failed"
	StatusInfo        Status = "info"
)

// Event is the JSONL-serialized audit record. Field names are stable and
// additive; consumers MUST tolerate unknown future fields.
type Event struct {
	V               int               `json:"v"`
	Ts              time.Time         `json:"ts"`
	Direction       Direction         `json:"direction"`
	Surface         string            `json:"surface"`
	DeliveryID      string            `json:"deliveryId,omitempty"`
	Platform        string            `json:"platform,omitempty"`
	BridgeID        string            `json:"bridgeId,omitempty"`
	TenantID        string            `json:"tenantId,omitempty"`
	ChatIDHash      string            `json:"chatIdHash,omitempty"`
	UserIDHash      string            `json:"userIdHash,omitempty"`
	Action          string            `json:"action,omitempty"`
	Status          Status            `json:"status,omitempty"`
	DeliveryMethod  string            `json:"deliveryMethod,omitempty"`
	FallbackReason  string            `json:"fallbackReason,omitempty"`
	LatencyMs       int64             `json:"latencyMs,omitempty"`
	SignatureSource string            `json:"signatureSource,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// HashID returns an HMAC-SHA256(salt, raw) truncated to 16 hex characters.
// Empty raw values yield the empty string, so absent PII stays absent and
// can't be retroactively compared against present values.
func HashID(salt, raw string) string {
	if raw == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(salt))
	_, _ = mac.Write([]byte(raw))
	sum := mac.Sum(nil)
	return hex.EncodeToString(sum[:8])
}

// GenerateSalt returns a cryptographically random 32-byte hex string
// suitable for use as IM_AUDIT_HASH_SALT.
func GenerateSalt() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
