package notify

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// DefaultStagingTTL is the lifetime of a staged attachment before the worker
// deletes it. Operators can override via IM_BRIDGE_ATTACHMENT_TTL.
const DefaultStagingTTL = time.Hour

// DefaultStagingCapacityBytes is the default total-size threshold that
// triggers oldest-first garbage collection. 2 GB matches the design.
const DefaultStagingCapacityBytes int64 = 2 * 1024 * 1024 * 1024

// StagingStore manages the on-disk staging directory for inbound/outbound
// attachments. It owns the directory lifecycle: startup cleanup, TTL-based
// deletion, and capacity-threshold GC.
type StagingStore struct {
	dir         string
	ttl         time.Duration
	capacity    int64
	now         func() time.Time
	mu          sync.Mutex
	meta        map[string]stagingEntry
	cancel      chan struct{}
	done        chan struct{}
}

type stagingEntry struct {
	path      string
	createdAt time.Time
	sizeBytes int64
}

// NewStagingStore creates a staging store rooted at `dir`. The directory is
// created if missing. Passing "" returns nil (caller should treat attachments
// as unsupported).
func NewStagingStore(dir string) (*StagingStore, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}
	store := &StagingStore{
		dir:      dir,
		ttl:      DefaultStagingTTL,
		capacity: DefaultStagingCapacityBytes,
		now:      time.Now,
		meta:     make(map[string]stagingEntry),
		cancel:   make(chan struct{}),
		done:     make(chan struct{}),
	}
	store.cleanupOnStartup()
	return store, nil
}

// SetTTL overrides the per-attachment lifetime.
func (s *StagingStore) SetTTL(ttl time.Duration) {
	if s == nil || ttl <= 0 {
		return
	}
	s.ttl = ttl
}

// SetCapacity overrides the total-size threshold in bytes.
func (s *StagingStore) SetCapacity(bytes int64) {
	if s == nil || bytes <= 0 {
		return
	}
	s.capacity = bytes
}

// Dir returns the absolute staging directory path.
func (s *StagingStore) Dir() string {
	if s == nil {
		return ""
	}
	return s.dir
}

// Stage writes payload to a new UUID-named file and returns the staged
// attachment ID and absolute path. The caller is responsible for recording
// the path in the Attachment.ContentRef.
func (s *StagingStore) Stage(filename string, reader io.Reader, declaredSize int64) (id string, path string, size int64, err error) {
	if s == nil {
		return "", "", 0, errors.New("staging disabled")
	}
	id = uuid.NewString()
	safeName := sanitizeStagingName(filename)
	path = filepath.Join(s.dir, id+"-"+safeName)
	f, ferr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if ferr != nil {
		return "", "", 0, fmt.Errorf("create staging file: %w", ferr)
	}
	n, copyErr := io.Copy(f, reader)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", "", 0, fmt.Errorf("write staging file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", "", 0, fmt.Errorf("close staging file: %w", closeErr)
	}
	if declaredSize > 0 && n != declaredSize {
		_ = os.Remove(path)
		return "", "", 0, fmt.Errorf("staging size mismatch: declared=%d wrote=%d", declaredSize, n)
	}
	s.mu.Lock()
	s.meta[id] = stagingEntry{path: path, createdAt: s.now(), sizeBytes: n}
	s.mu.Unlock()
	s.enforceCapacityLocked()
	return id, path, n, nil
}

// Remove deletes a staged file by ID.
func (s *StagingStore) Remove(id string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	entry, ok := s.meta[id]
	delete(s.meta, id)
	s.mu.Unlock()
	if !ok {
		return nil
	}
	if err := os.Remove(entry.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Lookup returns the absolute path for a staged ID.
func (s *StagingStore) Lookup(id string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.meta[id]
	if !ok {
		return "", false
	}
	return entry.path, true
}

// cleanupOnStartup removes stale files from a previous process crash.
func (s *StagingStore) cleanupOnStartup() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		log.WithField("component", "notify.staging").WithError(err).Warn("staging startup read failed")
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(s.dir, e.Name()))
	}
}

// StartWorker kicks off a goroutine that deletes TTL-expired staging files
// every `interval`. Stop via Stop().
func (s *StagingStore) StartWorker(interval time.Duration) {
	if s == nil {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.cancel:
				return
			case <-ticker.C:
				s.collectExpired()
			}
		}
	}()
}

// Stop signals the worker to exit and waits briefly for shutdown.
func (s *StagingStore) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.cancel:
		return
	default:
	}
	close(s.cancel)
	<-s.done
}

func (s *StagingStore) collectExpired() {
	cutoff := s.now().Add(-s.ttl)
	var drop []string
	s.mu.Lock()
	for id, entry := range s.meta {
		if entry.createdAt.Before(cutoff) {
			drop = append(drop, id)
		}
	}
	s.mu.Unlock()
	for _, id := range drop {
		if err := s.Remove(id); err != nil {
			log.WithField("component", "notify.staging").WithField("id", id).WithError(err).Warn("remove expired staging file")
		}
	}
}

// enforceCapacityLocked deletes oldest-first until the total size is under the
// configured capacity threshold.
func (s *StagingStore) enforceCapacityLocked() {
	s.mu.Lock()
	var total int64
	type idx struct {
		id        string
		createdAt time.Time
	}
	all := make([]idx, 0, len(s.meta))
	for id, e := range s.meta {
		total += e.sizeBytes
		all = append(all, idx{id, e.createdAt})
	}
	if total <= s.capacity {
		s.mu.Unlock()
		return
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].createdAt.Before(all[j].createdAt)
	})
	var drop []string
	for _, entry := range all {
		if total <= s.capacity {
			break
		}
		drop = append(drop, entry.id)
		total -= s.meta[entry.id].sizeBytes
	}
	s.mu.Unlock()
	for _, id := range drop {
		if err := s.Remove(id); err != nil {
			log.WithField("component", "notify.staging").WithField("id", id).WithError(err).Warn("capacity GC remove")
		}
	}
}

func sanitizeStagingName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "attachment.bin"
	}
	// Strip path separators and keep the base name.
	name = filepath.Base(name)
	// Replace any remaining non-filename-safe characters.
	replacer := strings.NewReplacer("/", "_", "\\", "_", "\x00", "_")
	name = replacer.Replace(name)
	if name == "." || name == ".." {
		return "attachment.bin"
	}
	// Cap length to avoid absurd filenames.
	if len(name) > 128 {
		name = name[:128]
	}
	return name
}
