package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Writer emits audit events. Implementations MUST be safe for concurrent
// use. Emit MUST NOT block longer than necessary on the caller; the bridge
// path is latency-sensitive.
type Writer interface {
	Emit(event Event) error
	Close() error
}

// NopWriter discards all events. Suitable for tests and explicit opt-out.
type NopWriter struct{}

func (NopWriter) Emit(Event) error { return nil }
func (NopWriter) Close() error     { return nil }

// FileWriter appends JSONL events to a single file. Not recommended for
// production use on its own; wrap with RotatingWriter for size/age caps.
type FileWriter struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// OpenFile opens or creates `path` for append-only writes and returns a
// FileWriter backed by it.
func OpenFile(path string) (*FileWriter, error) {
	if path == "" {
		return nil, errors.New("audit: file path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("audit: create dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: open file: %w", err)
	}
	return &FileWriter{path: path, f: f}, nil
}

func (w *FileWriter) Emit(event Event) error {
	if event.V == 0 {
		event.V = SchemaVersion
	}
	if event.Ts.IsZero() {
		event.Ts = time.Now().UTC()
	} else {
		event.Ts = event.Ts.UTC()
	}
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("audit: marshal: %w", err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}
	return nil
}

// Size returns the current size of the backing file in bytes.
func (w *FileWriter) Size() (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	info, err := w.f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// RotatingConfig controls RotatingWriter's rollover and retention.
type RotatingConfig struct {
	Dir              string        // required; directory to hold audit files
	BaseName         string        // default "audit.jsonl"
	MaxSizeBytes     int64         // 0 disables size rotation
	RotateAt         time.Duration // 0 disables time rotation; e.g. 24h for daily
	RetainDuration   time.Duration // 0 disables retention cleanup
	Now              func() time.Time
	// DisableWriter, when true, yields a Writer that discards events. Used
	// so callers can centralize creation and respect env overrides.
	DisableWriter bool
}

// RotatingWriter wraps a FileWriter with size+age rotation plus retention.
type RotatingWriter struct {
	mu         sync.Mutex
	cfg        RotatingConfig
	current    *FileWriter
	openedAt   time.Time
	lastRotate time.Time
	now        func() time.Time
}

// New returns a Writer selected from cfg. If DisableWriter is set, a
// NopWriter is returned. Otherwise a RotatingWriter backed by a FileWriter.
func New(cfg RotatingConfig) (Writer, error) {
	if cfg.DisableWriter {
		return NopWriter{}, nil
	}
	if cfg.Dir == "" {
		return nil, errors.New("audit: Dir is required")
	}
	if cfg.BaseName == "" {
		cfg.BaseName = "audit.jsonl"
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	w := &RotatingWriter{cfg: cfg, now: now}
	if err := w.openCurrent(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RotatingWriter) openCurrent() error {
	path := filepath.Join(w.cfg.Dir, w.cfg.BaseName)
	f, err := OpenFile(path)
	if err != nil {
		return err
	}
	w.current = f
	w.openedAt = w.now()
	return nil
}

func (w *RotatingWriter) Emit(event Event) error {
	w.mu.Lock()
	if err := w.maybeRotateLocked(); err != nil {
		// Rotation failure is serious but should not drop events; emit a
		// self-audit entry then keep writing to the (now stale) file.
		w.mu.Unlock()
		_ = w.current.Emit(Event{
			V:         SchemaVersion,
			Ts:        w.now().UTC(),
			Direction: DirectionInternal,
			Surface:   "audit/rotate",
			Action:    "rotate_failed",
			Status:    StatusFailed,
			Metadata:  map[string]string{"error": err.Error()},
		})
		return w.current.Emit(event)
	}
	cur := w.current
	w.mu.Unlock()
	return cur.Emit(event)
}

func (w *RotatingWriter) maybeRotateLocked() error {
	if w.cfg.MaxSizeBytes > 0 {
		size, err := w.current.Size()
		if err == nil && size >= w.cfg.MaxSizeBytes {
			return w.rotateLocked()
		}
	}
	if w.cfg.RotateAt > 0 && w.now().Sub(w.openedAt) >= w.cfg.RotateAt {
		return w.rotateLocked()
	}
	return nil
}

func (w *RotatingWriter) rotateLocked() error {
	if w.current == nil {
		return nil
	}
	oldPath := w.current.path
	if err := w.current.Close(); err != nil {
		return fmt.Errorf("audit: close current: %w", err)
	}
	ts := w.now().UTC().Format("2006-01-02-1504")
	base := w.cfg.BaseName
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	rotated := filepath.Join(w.cfg.Dir, fmt.Sprintf("%s.%s%s", stem, ts, ext))
	// If `rotated` already exists, append a short suffix; avoids clobber
	// when rotating more than once per minute.
	if _, err := os.Stat(rotated); err == nil {
		rotated = filepath.Join(w.cfg.Dir, fmt.Sprintf("%s.%s.%d%s", stem, ts, w.now().Nanosecond(), ext))
	}
	if err := os.Rename(oldPath, rotated); err != nil {
		return fmt.Errorf("audit: rename: %w", err)
	}
	w.lastRotate = w.now()
	w.pruneLocked()
	return w.openCurrent()
}

// pruneLocked deletes rotated files older than RetainDuration. It never
// deletes the current `audit.jsonl`.
func (w *RotatingWriter) pruneLocked() {
	if w.cfg.RetainDuration <= 0 {
		return
	}
	entries, err := os.ReadDir(w.cfg.Dir)
	if err != nil {
		return
	}
	cutoff := w.now().Add(-w.cfg.RetainDuration)
	base := w.cfg.BaseName
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	rotatedPrefix := stem + "."
	for _, e := range entries {
		name := e.Name()
		if name == base {
			continue
		}
		if !strings.HasPrefix(name, rotatedPrefix) || !strings.HasSuffix(name, ext) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(w.cfg.Dir, name))
		}
	}
}

// ListRotated returns the rotated files in the audit directory, sorted by
// modification time (oldest first). Exposed for tests and tooling.
func (w *RotatingWriter) ListRotated() ([]string, error) {
	entries, err := os.ReadDir(w.cfg.Dir)
	if err != nil {
		return nil, err
	}
	base := w.cfg.BaseName
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	prefix := stem + "."
	type named struct {
		name    string
		modTime time.Time
	}
	var out []named
	for _, e := range entries {
		name := e.Name()
		if name == base {
			continue
		}
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, named{name, info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].modTime.Before(out[j].modTime) })
	names := make([]string, len(out))
	for i, n := range out {
		names[i] = n.name
	}
	return names, nil
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current == nil {
		return nil
	}
	return w.current.Close()
}
