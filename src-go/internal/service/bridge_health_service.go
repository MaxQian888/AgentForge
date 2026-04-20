package service

import (
	"context"
	"sync"
	"time"

	bridgeclient "github.com/agentforge/server/internal/bridge"
	log "github.com/sirupsen/logrus"
)

const (
	BridgeStatusReady    = "ready"
	BridgeStatusDegraded = "degraded"
)

type BridgeHealthPool struct {
	Active    int `json:"active"`
	Available int `json:"available"`
	Warm      int `json:"warm"`
}

type BridgeHealthSnapshot struct {
	Status    string           `json:"status"`
	LastCheck time.Time        `json:"last_check"`
	Pool      BridgeHealthPool `json:"pool"`
}

type bridgeHealthClient interface {
	Health(ctx context.Context) error
	GetPoolSummary(ctx context.Context) (*bridgeclient.PoolSummaryResponse, error)
}

type bridgeHealthConfig struct {
	startupAttempts   int
	startupInterval   time.Duration
	heartbeatInterval time.Duration
	failureThreshold  int
}

type BridgeHealthService struct {
	client bridgeHealthClient
	config bridgeHealthConfig

	startOnce sync.Once

	mu                  sync.RWMutex
	status              string
	lastCheck           time.Time
	pool                BridgeHealthPool
	consecutiveFailures int
}

func NewBridgeHealthService(client bridgeHealthClient) *BridgeHealthService {
	return newBridgeHealthServiceWithConfig(client, bridgeHealthConfig{
		startupAttempts:   10,
		startupInterval:   2 * time.Second,
		heartbeatInterval: 30 * time.Second,
		failureThreshold:  3,
	})
}

func newBridgeHealthServiceWithConfig(client bridgeHealthClient, cfg bridgeHealthConfig) *BridgeHealthService {
	if cfg.startupAttempts <= 0 {
		cfg.startupAttempts = 1
	}
	if cfg.startupInterval <= 0 {
		cfg.startupInterval = time.Second
	}
	if cfg.heartbeatInterval <= 0 {
		cfg.heartbeatInterval = 30 * time.Second
	}
	if cfg.failureThreshold <= 0 {
		cfg.failureThreshold = 3
	}
	return &BridgeHealthService{
		client: client,
		config: cfg,
		status: BridgeStatusDegraded,
	}
}

func (s *BridgeHealthService) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		s.runStartupProbe(ctx)
		go s.runHeartbeatLoop(ctx)
	})
}

func (s *BridgeHealthService) Status() string {
	if s == nil {
		return BridgeStatusDegraded
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.status == "" {
		return BridgeStatusDegraded
	}
	return s.status
}

func (s *BridgeHealthService) Snapshot() BridgeHealthSnapshot {
	if s == nil {
		return BridgeHealthSnapshot{Status: BridgeStatusDegraded}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return BridgeHealthSnapshot{
		Status:    s.status,
		LastCheck: s.lastCheck,
		Pool:      s.pool,
	}
}

func (s *BridgeHealthService) runStartupProbe(ctx context.Context) {
	if s.client == nil {
		s.setDegraded(time.Now().UTC(), "bridge client unavailable")
		return
	}
	for attempt := 1; attempt <= s.config.startupAttempts; attempt++ {
		if s.runCheck(ctx) {
			return
		}
		if attempt == s.config.startupAttempts {
			break
		}
		if !sleepWithContext(ctx, s.config.startupInterval) {
			return
		}
	}
}

func (s *BridgeHealthService) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runCheck(ctx)
		}
	}
}

func (s *BridgeHealthService) runCheck(ctx context.Context) bool {
	checkTime := time.Now().UTC()
	if s.client == nil {
		s.setDegraded(checkTime, "bridge client unavailable")
		return false
	}
	if err := s.client.Health(ctx); err != nil {
		s.recordFailure(checkTime, err)
		return false
	}
	s.recordSuccess(ctx, checkTime)
	return true
}

func (s *BridgeHealthService) recordSuccess(ctx context.Context, checkTime time.Time) {
	nextPool := s.currentPool()
	if summary, err := s.client.GetPoolSummary(ctx); err == nil && summary != nil {
		available := summary.Max - summary.Active
		if available < 0 {
			available = 0
		}
		nextPool = BridgeHealthPool{
			Active:    summary.Active,
			Available: available,
			Warm:      summary.WarmTotal,
		}
	}

	s.mu.Lock()
	previous := s.status
	s.status = BridgeStatusReady
	s.lastCheck = checkTime
	s.pool = nextPool
	s.consecutiveFailures = 0
	s.mu.Unlock()

	if previous != BridgeStatusReady {
		log.WithField("status", BridgeStatusReady).Info("bridge health transitioned")
	}
}

func (s *BridgeHealthService) recordFailure(checkTime time.Time, err error) {
	s.mu.Lock()
	s.lastCheck = checkTime
	s.consecutiveFailures++
	previous := s.status
	if previous == BridgeStatusDegraded || s.consecutiveFailures >= s.config.failureThreshold {
		s.status = BridgeStatusDegraded
	}
	current := s.status
	failures := s.consecutiveFailures
	s.mu.Unlock()

	if current == BridgeStatusDegraded && previous != BridgeStatusDegraded {
		log.WithError(err).WithField("consecutiveFailures", failures).Warn("bridge health transitioned to degraded")
	}
}

func (s *BridgeHealthService) setDegraded(checkTime time.Time, reason string) {
	s.mu.Lock()
	s.status = BridgeStatusDegraded
	s.lastCheck = checkTime
	s.consecutiveFailures = s.config.failureThreshold
	s.mu.Unlock()
	if reason != "" {
		log.WithField("reason", reason).Warn("bridge health initialized as degraded")
	}
}

func (s *BridgeHealthService) currentPool() BridgeHealthPool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pool
}

func sleepWithContext(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
