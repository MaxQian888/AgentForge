package state

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// sessionSchema extends the state store with the three tables backing
// session history, intent cache, and reply-target bindings. It is applied
// lazily from the first call into NewSessionStore so existing deployments
// upgrade in place.
const sessionSchema = `
CREATE TABLE IF NOT EXISTS session_history (
  tenant_id   TEXT NOT NULL,
  session_key TEXT NOT NULL,
  content     TEXT NOT NULL,
  occurred_at INTEGER NOT NULL,
  PRIMARY KEY(tenant_id, session_key, occurred_at)
);
CREATE INDEX IF NOT EXISTS session_history_recent ON session_history(tenant_id, session_key, occurred_at DESC);

CREATE TABLE IF NOT EXISTS intent_cache (
  tenant_id  TEXT NOT NULL,
  text_hash  TEXT NOT NULL,
  intent     TEXT NOT NULL,
  confidence REAL NOT NULL,
  cached_at  INTEGER NOT NULL,
  PRIMARY KEY(tenant_id, text_hash)
);
CREATE INDEX IF NOT EXISTS intent_cache_expiry ON intent_cache(cached_at);

CREATE TABLE IF NOT EXISTS reply_target_binding (
  tenant_id    TEXT NOT NULL,
  binding_id   TEXT NOT NULL,
  reply_target TEXT NOT NULL,
  created_at   INTEGER NOT NULL,
  expires_at   INTEGER NOT NULL,
  PRIMARY KEY(tenant_id, binding_id)
);
CREATE INDEX IF NOT EXISTS reply_target_binding_expires ON reply_target_binding(expires_at);
`

// SessionStore persists NLU conversation history per tenant. It implements
// both persistent (SQLite-backed) and in-memory fallback modes transparently
// so callers don't branch. When `store` is nil, history is kept in a
// process-local map and lost on restart (matching legacy behaviour).
type SessionStore struct {
	store *Store

	// Configuration knobs. Operators may tune these via env in main.go;
	// default values here match the spec's expectations.
	historyLimit  int
	intentTTL     time.Duration
	replyDefault  time.Duration

	memMu       sync.Mutex
	memHistory  map[string][]historyEntry
	memIntents  map[string]intentEntry
	memBindings map[string]bindingEntry
}

type historyEntry struct {
	content    string
	occurredAt int64
}

type intentEntry struct {
	intent     string
	confidence float64
	cachedAt   int64
}

type bindingEntry struct {
	replyTarget string
	createdAt   int64
	expiresAt   int64
}

// NewSessionStore returns a SessionStore backed by the given *state.Store.
// When store is nil the returned store falls back to an in-process map so
// callers get the same API contract in both modes.
func NewSessionStore(store *Store) *SessionStore {
	s := &SessionStore{
		store:         store,
		historyLimit:  100,
		intentTTL:     24 * time.Hour,
		replyDefault:  7 * 24 * time.Hour,
		memHistory:    map[string][]historyEntry{},
		memIntents:    map[string]intentEntry{},
		memBindings:   map[string]bindingEntry{},
	}
	if store != nil {
		if _, err := store.db.Exec(sessionSchema); err != nil {
			// Leave the store alive but fall back to in-memory so the
			// bridge still serves traffic while the operator investigates.
			s.store = nil
		}
	}
	return s
}

// SetHistoryLimit overrides the LRU retention count.
func (s *SessionStore) SetHistoryLimit(n int) {
	if s == nil || n <= 0 {
		return
	}
	s.historyLimit = n
}

// SetIntentTTL overrides the intent cache TTL.
func (s *SessionStore) SetIntentTTL(d time.Duration) {
	if s == nil || d <= 0 {
		return
	}
	s.intentTTL = d
}

// SetReplyBindingTTL overrides the default reply-target binding TTL.
func (s *SessionStore) SetReplyBindingTTL(d time.Duration) {
	if s == nil || d <= 0 {
		return
	}
	s.replyDefault = d
}

// Append records a new message for the (tenantID, sessionKey) session. It
// prunes old entries to keep the stored length at or below historyLimit.
func (s *SessionStore) Append(tenantID, sessionKey, content string) error {
	if s == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	if s.store != nil {
		tx, err := s.store.db.Begin()
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.Exec(
			"INSERT INTO session_history(tenant_id, session_key, content, occurred_at) VALUES (?, ?, ?, ?)",
			tenantID, sessionKey, content, now,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`DELETE FROM session_history
			 WHERE tenant_id = ? AND session_key = ?
			   AND occurred_at < (
			     SELECT MIN(occurred_at) FROM (
			       SELECT occurred_at FROM session_history
			       WHERE tenant_id = ? AND session_key = ?
			       ORDER BY occurred_at DESC LIMIT ?))`,
			tenantID, sessionKey, tenantID, sessionKey, s.historyLimit,
		); err != nil {
			return err
		}
		return tx.Commit()
	}
	// In-memory fallback.
	s.memMu.Lock()
	defer s.memMu.Unlock()
	key := tenantID + "\x00" + sessionKey
	entries := append(s.memHistory[key], historyEntry{content: content, occurredAt: now})
	if len(entries) > s.historyLimit {
		entries = entries[len(entries)-s.historyLimit:]
	}
	s.memHistory[key] = entries
	return nil
}

// Recent returns up to n most-recent entries for the session (oldest first).
func (s *SessionStore) Recent(tenantID, sessionKey string, n int) []string {
	if s == nil || n <= 0 {
		return nil
	}
	if s.store != nil {
		rows, err := s.store.db.Query(
			"SELECT content FROM session_history WHERE tenant_id = ? AND session_key = ? ORDER BY occurred_at DESC LIMIT ?",
			tenantID, sessionKey, n,
		)
		if err != nil {
			return nil
		}
		defer rows.Close()
		out := make([]string, 0, n)
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err == nil {
				out = append(out, c)
			}
		}
		// Reverse to oldest-first.
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		return out
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	key := tenantID + "\x00" + sessionKey
	entries := s.memHistory[key]
	if len(entries) > n {
		entries = entries[len(entries)-n:]
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.content)
	}
	return out
}

// IntentCacheGet returns the cached intent classification for `(tenant,
// textHash)` when it is within the TTL window. Expired entries return
// ("", 0, false).
func (s *SessionStore) IntentCacheGet(tenantID, textHash string) (string, float64, bool) {
	if s == nil {
		return "", 0, false
	}
	now := time.Now().UnixMilli()
	cutoff := now - s.intentTTL.Milliseconds()
	if s.store != nil {
		var intent string
		var confidence float64
		var cachedAt int64
		err := s.store.db.QueryRow(
			"SELECT intent, confidence, cached_at FROM intent_cache WHERE tenant_id = ? AND text_hash = ?",
			tenantID, textHash,
		).Scan(&intent, &confidence, &cachedAt)
		if err != nil || cachedAt < cutoff {
			return "", 0, false
		}
		return intent, confidence, true
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	key := tenantID + "\x00" + textHash
	e, ok := s.memIntents[key]
	if !ok || e.cachedAt < cutoff {
		return "", 0, false
	}
	return e.intent, e.confidence, true
}

// IntentCacheSet records an intent classification.
func (s *SessionStore) IntentCacheSet(tenantID, textHash, intent string, confidence float64) error {
	if s == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	if s.store != nil {
		_, err := s.store.db.Exec(
			"INSERT OR REPLACE INTO intent_cache(tenant_id, text_hash, intent, confidence, cached_at) VALUES (?, ?, ?, ?, ?)",
			tenantID, textHash, intent, confidence, now,
		)
		return err
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	s.memIntents[tenantID+"\x00"+textHash] = intentEntry{intent: intent, confidence: confidence, cachedAt: now}
	return nil
}

// ReplyBindingPut persists a reply target for (tenant, bindingId). The
// serialized reply target is produced by the caller; the store is opaque.
// When ttl is zero the default TTL is used.
func (s *SessionStore) ReplyBindingPut(tenantID, bindingID, serialized string, ttl time.Duration) error {
	if s == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = s.replyDefault
	}
	now := time.Now().UnixMilli()
	expires := now + ttl.Milliseconds()
	if s.store != nil {
		_, err := s.store.db.Exec(
			"INSERT OR REPLACE INTO reply_target_binding(tenant_id, binding_id, reply_target, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
			tenantID, bindingID, serialized, now, expires,
		)
		return err
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	s.memBindings[tenantID+"\x00"+bindingID] = bindingEntry{replyTarget: serialized, createdAt: now, expiresAt: expires}
	return nil
}

// ReplyBindingGet fetches a serialized reply target. Missing or expired
// bindings return "", false.
func (s *SessionStore) ReplyBindingGet(tenantID, bindingID string) (string, bool) {
	if s == nil {
		return "", false
	}
	now := time.Now().UnixMilli()
	if s.store != nil {
		var serialized string
		var expiresAt int64
		err := s.store.db.QueryRow(
			"SELECT reply_target, expires_at FROM reply_target_binding WHERE tenant_id = ? AND binding_id = ?",
			tenantID, bindingID,
		).Scan(&serialized, &expiresAt)
		if err != nil || expiresAt <= now {
			return "", false
		}
		return serialized, true
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	e, ok := s.memBindings[tenantID+"\x00"+bindingID]
	if !ok || e.expiresAt <= now {
		return "", false
	}
	return e.replyTarget, true
}

// ReplyBindingDelete explicitly removes a binding (e.g. when the owning
// entity completes).
func (s *SessionStore) ReplyBindingDelete(tenantID, bindingID string) error {
	if s == nil {
		return nil
	}
	if s.store != nil {
		_, err := s.store.db.Exec(
			"DELETE FROM reply_target_binding WHERE tenant_id = ? AND binding_id = ?",
			tenantID, bindingID,
		)
		return err
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	delete(s.memBindings, tenantID+"\x00"+bindingID)
	return nil
}

// SessionSweepNow prunes expired intent-cache and reply-binding rows. The
// store runs this periodically; callers may invoke it manually from tests
// or operator maintenance scripts.
func (s *SessionStore) SessionSweepNow(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}
	now := time.Now().UnixMilli()
	intentCutoff := now - s.intentTTL.Milliseconds()
	_, err := s.store.db.ExecContext(ctx, "DELETE FROM intent_cache WHERE cached_at < ?", intentCutoff)
	if err != nil {
		return fmt.Errorf("session sweep intent: %w", err)
	}
	_, err = s.store.db.ExecContext(ctx, "DELETE FROM reply_target_binding WHERE expires_at < ?", now)
	if err != nil {
		return fmt.Errorf("session sweep binding: %w", err)
	}
	return nil
}

// Counts reports approximate row counts for each session table. Useful for
// operator diagnostics.
func (s *SessionStore) Counts() (history int, intents int, bindings int) {
	if s == nil {
		return 0, 0, 0
	}
	if s.store != nil {
		_ = s.store.db.QueryRow("SELECT COUNT(*) FROM session_history").Scan(&history)
		_ = s.store.db.QueryRow("SELECT COUNT(*) FROM intent_cache").Scan(&intents)
		_ = s.store.db.QueryRow("SELECT COUNT(*) FROM reply_target_binding").Scan(&bindings)
		return
	}
	s.memMu.Lock()
	defer s.memMu.Unlock()
	for _, entries := range s.memHistory {
		history += len(entries)
	}
	intents = len(s.memIntents)
	bindings = len(s.memBindings)
	return
}

// ErrSessionClosed is returned when operations are attempted after the
// backing store is closed.
var ErrSessionClosed = errors.New("session store closed")
