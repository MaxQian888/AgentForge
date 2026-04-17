package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core/state"
)

// applySignedHeadersAt signs a request with an explicit timestamp, letting
// tests exercise skew-window rejection.
func applySignedHeadersAt(req *http.Request, path, deliveryID string, body []byte, secret string, ts time.Time) {
	timestamp := ts.UTC().Format(time.RFC3339)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join([]string{
		req.Method, path, deliveryID, timestamp, string(body),
	}, "|")))
	req.Header.Set("X-AgentForge-Delivery-Id", deliveryID)
	req.Header.Set("X-AgentForge-Delivery-Timestamp", timestamp)
	req.Header.Set("X-AgentForge-Signature", hex.EncodeToString(mac.Sum(nil)))
}

func newSecurityReceiver(t *testing.T) *Receiver {
	t.Helper()
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")
	r.SetSharedSecret("shared-secret")
	return r
}

func newSecurityReceiverWithStore(t *testing.T) (*Receiver, *state.Store) {
	t.Helper()
	r := newSecurityReceiver(t)
	store, err := state.Open(state.Config{
		Path:            filepath.Join(t.TempDir(), "state.db"),
		CleanupInterval: -1,
	})
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	r.SetDedupeStore(store)
	return r, store
}

func sendBody(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(SendRequest{Platform: "slack", ChatID: "chat-1", Content: "hi"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return body
}

func TestSkew_RejectsOldTimestamp(t *testing.T) {
	r := newSecurityReceiver(t)
	r.SetSignatureSkew(5 * time.Minute)

	body := sendBody(t)
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeadersAt(req, "/im/send", "d-old", body, "shared-secret",
		time.Now().Add(-30*time.Minute))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestTimeout)
	}
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if got, _ := payload["error"].(string); got != string(reasonTimestampOutOfWindow) {
		t.Fatalf("error = %v, want %s", payload["error"], reasonTimestampOutOfWindow)
	}
	if retryable, _ := payload["retryable"].(bool); retryable {
		t.Fatalf("retryable should be false for out-of-window timestamp")
	}
}

func TestSkew_RejectsFutureTimestamp(t *testing.T) {
	r := newSecurityReceiver(t)
	r.SetSignatureSkew(5 * time.Minute)

	body := sendBody(t)
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeadersAt(req, "/im/send", "d-future", body, "shared-secret",
		time.Now().Add(30*time.Minute))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestTimeout)
	}
}

func TestSkew_AcceptsWithinWindow(t *testing.T) {
	r := newSecurityReceiver(t)
	r.SetSignatureSkew(5 * time.Minute)

	body := sendBody(t)
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeadersAt(req, "/im/send", "d-ok", body, "shared-secret",
		time.Now().Add(-1*time.Minute))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestSkew_UnsetSkewAllowsAny(t *testing.T) {
	r := newSecurityReceiver(t)
	r.SetSignatureSkew(0) // fallback to default 5min — still rejects 30min ago

	// with default skew, 30min ago is out-of-window
	body := sendBody(t)
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeadersAt(req, "/im/send", "d1", body, "shared-secret",
		time.Now().Add(-30*time.Minute))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)
	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("default skew should still reject old: got %d", rec.Code)
	}
}

func TestInvalidSignatureClassifiedReject(t *testing.T) {
	r := newSecurityReceiver(t)

	body := sendBody(t)
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	req.Header.Set("X-AgentForge-Delivery-Id", "d1")
	req.Header.Set("X-AgentForge-Delivery-Timestamp", time.Now().UTC().Format(time.RFC3339))
	req.Header.Set("X-AgentForge-Signature", "deadbeef")
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if got, _ := payload["error"].(string); got != string(reasonInvalidSignature) {
		t.Fatalf("error = %v, want %s", payload["error"], reasonInvalidSignature)
	}
}

func TestMissingHeadersClassifiedReject(t *testing.T) {
	r := newSecurityReceiver(t)

	body := sendBody(t)
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var payload map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if got, _ := payload["error"].(string); got != string(reasonMissingHeaders) {
		t.Fatalf("error = %v, want %s", payload["error"], reasonMissingHeaders)
	}
}

func TestDurableDedupe_DuplicateReturns409(t *testing.T) {
	r, _ := newSecurityReceiverWithStore(t)

	body := sendBody(t)
	// First delivery
	req1 := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeaders(req1, "/im/send", "d-dupe", body, "shared-secret")
	rec1 := httptest.NewRecorder()
	r.handleSend(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first: got %d", rec1.Code)
	}

	// Duplicate within TTL → 409
	req2 := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeaders(req2, "/im/send", "d-dupe", body, "shared-secret")
	rec2 := httptest.NewRecorder()
	r.handleSend(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("duplicate: got %d, want 409", rec2.Code)
	}
	var payload map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &payload)
	if got, _ := payload["error"].(string); got != string(reasonDuplicateDelivery) {
		t.Fatalf("error = %v, want %s", payload["error"], reasonDuplicateDelivery)
	}
	if got, _ := payload["status"].(string); got != "duplicate" {
		t.Fatalf("status = %v, want duplicate", payload["status"])
	}
}

func TestDurableDedupe_SurvivesReceiverRecreation(t *testing.T) {
	// Share the same state store between two fresh receivers, simulating restart.
	dbPath := filepath.Join(t.TempDir(), "state.db")
	store1, err := state.Open(state.Config{Path: dbPath, CleanupInterval: -1})
	if err != nil {
		t.Fatal(err)
	}

	r1 := newSecurityReceiver(t)
	r1.SetDedupeStore(store1)

	body := sendBody(t)
	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeaders(req, "/im/send", "d-restart", body, "shared-secret")
	rec := httptest.NewRecorder()
	r1.handleSend(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first: got %d", rec.Code)
	}
	_ = store1.Close()

	// Reopen the same DB on disk (simulates restart).
	store2, err := state.Open(state.Config{Path: dbPath, CleanupInterval: -1})
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()
	r2 := newSecurityReceiver(t)
	r2.SetDedupeStore(store2)

	req2 := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeaders(req2, "/im/send", "d-restart", body, "shared-secret")
	rec2 := httptest.NewRecorder()
	r2.handleSend(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("post-restart duplicate: got %d, want 409", rec2.Code)
	}
}

func TestDurableDedupe_ConcurrentFirstOnlyAccepted(t *testing.T) {
	r, _ := newSecurityReceiverWithStore(t)

	body := sendBody(t)
	const workers = 8
	var wg sync.WaitGroup
	var okCount, dupCount int
	var mu sync.Mutex
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
			applySignedHeaders(req, "/im/send", "d-race", body, "shared-secret")
			rec := httptest.NewRecorder()
			r.handleSend(rec, req)
			mu.Lock()
			switch rec.Code {
			case http.StatusOK:
				okCount++
			case http.StatusConflict:
				dupCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("okCount = %d, want 1; dupCount = %d", okCount, dupCount)
	}
}
