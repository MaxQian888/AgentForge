// Package service — audit_sanitizer.go redacts sensitive fields and bounds
// payload size before audit events hit the database.
//
// Why both:
//   - Redaction prevents secrets (tokens, passwords) from being written to
//     the audit table where they would survive any sensible TTL.
//   - Size cap prevents one rogue payload from forcing the audit table
//     into a row-size or storage-pressure failure mode.
//
// The denylist matches *substring*, case-insensitive, on the JSON object
// key — `accessToken`, `oauth_access_token`, `User_Password` all redact.
package service

import (
	"encoding/json"
	"strings"
)

// AuditPayloadMaxBytes is the hard cap on the serialized JSON payload after
// redaction. Set to 64 KiB per design doc (Decision 4).
const AuditPayloadMaxBytes = 64 * 1024

// AuditRedactionMarker replaces the value of any redacted field. Visible to
// audit readers so they understand a value was intentionally suppressed.
const AuditRedactionMarker = "[REDACTED]"

// AuditTruncationFlagKey marks a payload that was truncated past the size
// cap. Surfaced as a top-level boolean so list views can show a badge.
const AuditTruncationFlagKey = "_truncated"

// auditSensitiveSubstrings — case-insensitive substring matches against
// JSON object keys. Add aggressively; the cost of an over-redaction is a
// missing audit detail, but the cost of a leaked secret is unbounded.
var auditSensitiveSubstrings = []string{
	"secret",
	"token",
	"api_key",
	"apikey",
	"password",
	"access_token",
	"refresh_token",
	"private_key",
	"privatekey",
	"client_secret",
	"clientsecret",
	"authorization",
}

// SanitizeAuditPayload takes a JSON-serializable input (typically
// map[string]any) and returns a JSON string suitable for persistence. On
// invalid input the function returns "{}" rather than failing — audit
// events are best-effort, never blocking.
//
// Output guarantees:
//   - All redacted field values are replaced with AuditRedactionMarker.
//   - If the serialized form exceeds AuditPayloadMaxBytes the result is a
//     compact object containing only `{ _truncated: true, _summary: "..." }`.
func SanitizeAuditPayload(payload any) string {
	if payload == nil {
		return "{}"
	}
	cleaned := redactAuditValue(payload)
	out, err := json.Marshal(cleaned)
	if err != nil {
		return "{}"
	}
	if len(out) <= AuditPayloadMaxBytes {
		return string(out)
	}
	// Truncate by replacing the body with a marker. We deliberately don't
	// try to keep partial fields — the readability cost of a half-object
	// outweighs the value of preserving "the first 64 KB" of arbitrary JSON.
	truncated := map[string]any{
		AuditTruncationFlagKey: true,
		"_originalSizeBytes":   len(out),
		"_summary":             auditPayloadSummary(cleaned),
	}
	final, _ := json.Marshal(truncated)
	return string(final)
}

// SanitizeAuditPayloadJSON is a convenience wrapper for callers that already
// hold a JSON-encoded string. It re-parses, redacts, then re-encodes.
func SanitizeAuditPayloadJSON(payloadJSON string) string {
	if payloadJSON == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(payloadJSON), &v); err != nil {
		// Not valid JSON — treat as opaque string and store under a wrapper.
		return SanitizeAuditPayload(map[string]any{"raw": payloadJSON})
	}
	return SanitizeAuditPayload(v)
}

// redactAuditValue walks any decoded JSON value and substitutes redacted
// markers for sensitive object keys. Slices and nested maps are walked
// recursively; primitives are returned as-is.
func redactAuditValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			if isSensitiveAuditKey(k) {
				out[k] = AuditRedactionMarker
				continue
			}
			out[k] = redactAuditValue(child)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, child := range val {
			out[i] = redactAuditValue(child)
		}
		return out
	default:
		return val
	}
}

func isSensitiveAuditKey(key string) bool {
	lower := strings.ToLower(key)
	for _, needle := range auditSensitiveSubstrings {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// auditPayloadSummary returns a short human-readable hint about the original
// payload shape for the truncation marker. Top-level keys + their type, no
// values. Bounded to 16 keys to keep the summary itself small.
func auditPayloadSummary(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return "non-object payload"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
		if len(keys) >= 16 {
			break
		}
	}
	return map[string]any{"topLevelKeys": keys}
}
