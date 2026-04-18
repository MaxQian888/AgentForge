package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeAuditPayload_RedactsKnownSensitiveFields(t *testing.T) {
	in := map[string]any{
		"user":         "alice",
		"accessToken":  "secret-abc",
		"refresh_token": "secret-def",
		"api_key":       "secret-ghi",
		"PASSWORD":      "secret-jkl",
		"client_secret": "secret-mno",
		"nested": map[string]any{
			"oauth_token": "secret-pqr",
			"keep_me":     "ok",
		},
		"items": []any{
			map[string]any{"token": "deep-secret", "name": "first"},
			map[string]any{"name": "second"},
		},
	}
	out := SanitizeAuditPayload(in)

	// All sensitive values must be redacted.
	for _, needle := range []string{"secret-abc", "secret-def", "secret-ghi", "secret-jkl", "secret-mno", "secret-pqr", "deep-secret"} {
		if strings.Contains(out, needle) {
			t.Errorf("sanitized payload still contains %q: %s", needle, out)
		}
	}
	// Non-sensitive sibling values must survive.
	for _, needle := range []string{"alice", "ok", "first", "second"} {
		if !strings.Contains(out, needle) {
			t.Errorf("sanitized payload should preserve %q: %s", needle, out)
		}
	}
	// Marker is present.
	if !strings.Contains(out, AuditRedactionMarker) {
		t.Errorf("expected redaction marker %q in output: %s", AuditRedactionMarker, out)
	}
}

func TestSanitizeAuditPayload_TruncatesPastSizeCap(t *testing.T) {
	// Generate ~80 KB of innocent data so the cap kicks in.
	bigSlice := make([]string, 0, 4000)
	for i := 0; i < 4000; i++ {
		bigSlice = append(bigSlice, "blob-of-text-blob-of-text-blob-of-text")
	}
	in := map[string]any{"data": bigSlice}

	out := SanitizeAuditPayload(in)

	if len(out) > AuditPayloadMaxBytes+512 {
		t.Errorf("expected truncated payload near 64 KB, got %d bytes", len(out))
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("truncated payload is not valid JSON: %v", err)
	}
	if v, ok := parsed[AuditTruncationFlagKey].(bool); !ok || !v {
		t.Errorf("expected %s=true in truncated payload, got %#v", AuditTruncationFlagKey, parsed[AuditTruncationFlagKey])
	}
}

func TestSanitizeAuditPayload_NilInput(t *testing.T) {
	if got := SanitizeAuditPayload(nil); got != "{}" {
		t.Errorf("expected `{}`, got %q", got)
	}
}

func TestSanitizeAuditPayloadJSON_HandlesInvalidJSON(t *testing.T) {
	out := SanitizeAuditPayloadJSON("not-json")
	if !strings.Contains(out, "raw") {
		t.Errorf("expected raw wrapper for invalid JSON; got %s", out)
	}
}

func TestIsSensitiveAuditKey_CaseInsensitive(t *testing.T) {
	for _, key := range []string{"Token", "API_KEY", "ClientSecret", "user_password", "RefreshToken"} {
		if !isSensitiveAuditKey(key) {
			t.Errorf("expected %q to be flagged sensitive", key)
		}
	}
	for _, key := range []string{"name", "email", "user_id", "project"} {
		if isSensitiveAuditKey(key) {
			t.Errorf("expected %q NOT to be flagged sensitive", key)
		}
	}
}
