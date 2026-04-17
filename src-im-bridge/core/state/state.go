// Package state provides durable SQLite-backed storage for the IM Bridge's
// security-critical state: delivery dedupe, nonce consumption, rate limit
// counters, and small key-value settings. State persists across bridge
// restarts so idempotency and rate decisions remain truthful under rolling
// deploys or multi-replica topologies.
package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS dedupe (
  delivery_id TEXT PRIMARY KEY,
  surface     TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS dedupe_expires ON dedupe(expires_at);

CREATE TABLE IF NOT EXISTS nonce (
  nonce      TEXT NOT NULL,
  scope      TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  PRIMARY KEY(nonce, scope)
);
CREATE INDEX IF NOT EXISTS nonce_expires ON nonce(expires_at);

CREATE TABLE IF NOT EXISTS rate (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  scope_key   TEXT NOT NULL,
  policy_id   TEXT NOT NULL,
  occurred_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS rate_query     ON rate(policy_id, scope_key, occurred_at);
CREATE INDEX IF NOT EXISTS rate_eviction  ON rate(occurred_at);

CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

// Config controls where and how the state store lives.
type Config struct {
	// Path to the SQLite database file. Parent directory must be writable.
	Path string
	// CleanupInterval controls how often expired dedupe/nonce rows are pruned.
	// Zero uses 30s. Negative disables background cleanup.
	CleanupInterval time.Duration
	// RateRetention controls how long rate events are retained beyond the
	// longest active policy window. Zero uses 1h.
	RateRetention time.Duration
	// Now is an injectable clock used by tests. Production code leaves it nil.
	Now func() time.Time
}

// Store wraps the underlying SQLite connection and exposes the security-state
// stores used by the IM Bridge.
type Store struct {
	db       *sql.DB
	cfg      Config
	now      func() time.Time
	cleanup  chan struct{}
	cleanDone chan struct{}
}

// Open initializes or opens a state store at cfg.Path. It applies schema,
// configures WAL + busy_timeout, and spawns a background cleanup goroutine
// unless CleanupInterval is negative.
func Open(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		return nil, errors.New("state: Config.Path is required")
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 30 * time.Second
	}
	if cfg.RateRetention == 0 {
		cfg.RateRetention = time.Hour
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("state: create dir: %w", err)
	}

	// _busy_timeout must be in ms; _journal_mode=WAL enables better concurrency.
	dsn := cfg.Path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("state: open: %w", err)
	}
	// SQLite performs best with a single writer; serialize writes.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("state: ping: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("state: apply schema: %w", err)
	}

	s := &Store{
		db:  db,
		cfg: cfg,
		now: now,
	}
	if cfg.CleanupInterval > 0 {
		s.cleanup = make(chan struct{})
		s.cleanDone = make(chan struct{})
		go s.cleanupLoop()
	}
	return s, nil
}

// Close stops the background cleanup goroutine and closes the database.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	if s.cleanup != nil {
		close(s.cleanup)
		<-s.cleanDone
	}
	return s.db.Close()
}

// DB returns the underlying *sql.DB for callers that need custom queries
// (e.g. session-persistence from other change sets). Prefer the typed
// accessors below when possible.
func (s *Store) DB() *sql.DB { return s.db }

// --- DedupeStore --------------------------------------------------------

// Seen records that the given delivery id was observed on `surface` and
// returns true if the same id had already been recorded within its TTL.
// Expired records are transparently recycled; the operation is atomic under
// SQLite's WAL write serialization.
func (s *Store) Seen(id, surface string, ttl time.Duration) (bool, error) {
	if id == "" {
		return false, errors.New("state: delivery id is required")
	}
	if ttl <= 0 {
		return false, errors.New("state: ttl must be positive")
	}
	now := s.now().Unix()
	expires := now + int64(ttl.Seconds())

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("state: begin dedupe tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var prevExpires int64
	err = tx.QueryRow("SELECT expires_at FROM dedupe WHERE delivery_id = ?", id).Scan(&prevExpires)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// new record
	case err != nil:
		return false, fmt.Errorf("state: dedupe query: %w", err)
	case prevExpires > now:
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return true, nil
	}

	if _, err := tx.Exec(
		"INSERT OR REPLACE INTO dedupe(delivery_id, surface, created_at, expires_at) VALUES (?, ?, ?, ?)",
		id, surface, now, expires,
	); err != nil {
		return false, fmt.Errorf("state: dedupe insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return false, nil
}

// --- NonceStore ---------------------------------------------------------

// Consume attempts to register a (nonce, scope) pair for the first time.
// It returns true on first-time consumption and false when the nonce is a
// replay within its TTL window. Expired records are recycled.
func (s *Store) Consume(nonce, scope string, ttl time.Duration) (bool, error) {
	if nonce == "" {
		return false, errors.New("state: nonce is required")
	}
	if scope == "" {
		return false, errors.New("state: scope is required")
	}
	if ttl <= 0 {
		return false, errors.New("state: ttl must be positive")
	}
	now := s.now().Unix()
	expires := now + int64(ttl.Seconds())

	tx, err := s.db.Begin()
	if err != nil {
		return false, fmt.Errorf("state: begin nonce tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var prevExpires int64
	err = tx.QueryRow("SELECT expires_at FROM nonce WHERE nonce = ? AND scope = ?", nonce, scope).Scan(&prevExpires)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return false, fmt.Errorf("state: nonce query: %w", err)
	case prevExpires > now:
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}

	if _, err := tx.Exec(
		"INSERT OR REPLACE INTO nonce(nonce, scope, created_at, expires_at) VALUES (?, ?, ?, ?)",
		nonce, scope, now, expires,
	); err != nil {
		return false, fmt.Errorf("state: nonce insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// --- RateStore ---------------------------------------------------------

// Record logs a single rate event for the (scopeKey, policyId) pair at
// the provided timestamp. The caller supplies the timestamp so tests can
// inject a deterministic clock.
func (s *Store) Record(scopeKey, policyID string, ts time.Time) error {
	if scopeKey == "" || policyID == "" {
		return errors.New("state: scopeKey and policyID are required")
	}
	_, err := s.db.Exec(
		"INSERT INTO rate(scope_key, policy_id, occurred_at) VALUES (?, ?, ?)",
		scopeKey, policyID, ts.Unix(),
	)
	if err != nil {
		return fmt.Errorf("state: rate insert: %w", err)
	}
	return nil
}

// Count returns the number of rate events for the given (scopeKey, policyId)
// that occurred at or after `since`.
func (s *Store) Count(scopeKey, policyID string, since time.Time) (int, error) {
	if scopeKey == "" || policyID == "" {
		return 0, errors.New("state: scopeKey and policyID are required")
	}
	var n int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM rate WHERE policy_id = ? AND scope_key = ? AND occurred_at >= ?",
		policyID, scopeKey, since.Unix(),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("state: rate count: %w", err)
	}
	return n, nil
}

// --- Settings ---------------------------------------------------------

// SettingsGet returns the value for the given key, or ("", false, nil)
// when the key is absent.
func (s *Store) SettingsGet(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("state: settings query: %w", err)
	}
	return value, true, nil
}

// SettingsPut writes a key-value pair, overwriting any existing value.
func (s *Store) SettingsPut(key, value string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO settings(key, value) VALUES (?, ?)",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("state: settings put: %w", err)
	}
	return nil
}

// --- cleanup -----------------------------------------------------------

func (s *Store) cleanupLoop() {
	defer close(s.cleanDone)
	ticker := time.NewTicker(s.cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.cleanup:
			return
		case <-ticker.C:
			_ = s.Cleanup(context.Background())
		}
	}
}

// Cleanup prunes expired dedupe/nonce rows and rate events older than the
// configured retention window. Exposed for tests and operator ad-hoc runs.
func (s *Store) Cleanup(ctx context.Context) error {
	now := s.now().Unix()
	rateCutoff := s.now().Add(-s.cfg.RateRetention).Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("state: cleanup begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM dedupe WHERE expires_at < ?", now); err != nil {
		return fmt.Errorf("state: cleanup dedupe: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM nonce WHERE expires_at < ?", now); err != nil {
		return fmt.Errorf("state: cleanup nonce: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM rate WHERE occurred_at < ?", rateCutoff); err != nil {
		return fmt.Errorf("state: cleanup rate: %w", err)
	}
	return tx.Commit()
}

// WriteMu exposes a sentinel mutex for callers wanting to serialize work
// around state writes at the application layer; the store itself is safe
// for concurrent use.
var WriteMu sync.Mutex
