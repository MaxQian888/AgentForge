package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
)

type fakeBridgeHealthClient struct {
	mu            sync.Mutex
	healthResults []error
	poolSummary   *bridgeclient.PoolSummaryResponse
}

func (f *fakeBridgeHealthClient) Health(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.healthResults) == 0 {
		return nil
	}
	result := f.healthResults[0]
	if len(f.healthResults) > 1 {
		f.healthResults = f.healthResults[1:]
	}
	return result
}

func (f *fakeBridgeHealthClient) GetPoolSummary(_ context.Context) (*bridgeclient.PoolSummaryResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.poolSummary == nil {
		return nil, nil
	}
	copy := *f.poolSummary
	return &copy, nil
}

func TestBridgeHealthService_StartMarksReadyAfterSuccessfulProbe(t *testing.T) {
	t.Parallel()

	svc := newBridgeHealthServiceWithConfig(&fakeBridgeHealthClient{
		healthResults: []error{nil},
		poolSummary: &bridgeclient.PoolSummaryResponse{
			Active:        2,
			Max:           4,
			WarmTotal:     1,
			WarmAvailable: 1,
		},
	}, bridgeHealthConfig{
		startupAttempts:   1,
		startupInterval:   time.Millisecond,
		heartbeatInterval: time.Hour,
		failureThreshold:  3,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)

	snapshot := svc.Snapshot()
	if snapshot.Status != BridgeStatusReady {
		t.Fatalf("status = %q, want %q", snapshot.Status, BridgeStatusReady)
	}
	if snapshot.LastCheck.IsZero() {
		t.Fatal("expected last check to be recorded")
	}
	if snapshot.Pool.Active != 2 || snapshot.Pool.Available != 2 || snapshot.Pool.Warm != 1 {
		t.Fatalf("unexpected pool snapshot: %+v", snapshot.Pool)
	}
}

func TestBridgeHealthService_StartMarksDegradedWhenStartupProbeNeverSucceeds(t *testing.T) {
	t.Parallel()

	svc := newBridgeHealthServiceWithConfig(&fakeBridgeHealthClient{
		healthResults: []error{errors.New("dial tcp"), errors.New("dial tcp")},
	}, bridgeHealthConfig{
		startupAttempts:   2,
		startupInterval:   time.Millisecond,
		heartbeatInterval: time.Hour,
		failureThreshold:  3,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)

	if got := svc.Snapshot().Status; got != BridgeStatusDegraded {
		t.Fatalf("status = %q, want %q", got, BridgeStatusDegraded)
	}
}

func TestBridgeHealthService_TransitionsToDegradedAfterConsecutiveHeartbeatFailures(t *testing.T) {
	t.Parallel()

	svc := newBridgeHealthServiceWithConfig(&fakeBridgeHealthClient{
		healthResults: []error{
			nil,
			errors.New("heartbeat failed"),
			errors.New("heartbeat failed"),
			errors.New("heartbeat failed"),
		},
	}, bridgeHealthConfig{
		startupAttempts:   1,
		startupInterval:   time.Millisecond,
		heartbeatInterval: 5 * time.Millisecond,
		failureThreshold:  3,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)

	waitForBridgeHealthStatus(t, svc, BridgeStatusDegraded)
}

func TestBridgeHealthService_RecoversToReadyAfterHeartbeatSucceeds(t *testing.T) {
	t.Parallel()

	svc := newBridgeHealthServiceWithConfig(&fakeBridgeHealthClient{
		healthResults: []error{
			errors.New("startup failed"),
			nil,
		},
		poolSummary: &bridgeclient.PoolSummaryResponse{
			Active:        1,
			Max:           3,
			WarmTotal:     1,
			WarmAvailable: 0,
		},
	}, bridgeHealthConfig{
		startupAttempts:   1,
		startupInterval:   time.Millisecond,
		heartbeatInterval: 5 * time.Millisecond,
		failureThreshold:  3,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.Start(ctx)
	if got := svc.Snapshot().Status; got != BridgeStatusDegraded {
		t.Fatalf("initial status = %q, want %q", got, BridgeStatusDegraded)
	}

	waitForBridgeHealthStatus(t, svc, BridgeStatusReady)
}

func TestNewBridgeHealthServiceDefaultsAndStatusHelpers(t *testing.T) {
	svc := NewBridgeHealthService(nil)
	if svc == nil {
		t.Fatal("NewBridgeHealthService() returned nil")
	}
	if svc.config.startupAttempts != 10 || svc.config.failureThreshold != 3 {
		t.Fatalf("default config = %+v", svc.config)
	}
	if got := svc.Status(); got != BridgeStatusDegraded {
		t.Fatalf("Status() = %q, want degraded", got)
	}

	var nilSvc *BridgeHealthService
	if got := nilSvc.Status(); got != BridgeStatusDegraded {
		t.Fatalf("nil Status() = %q, want degraded", got)
	}
	if snapshot := nilSvc.Snapshot(); snapshot.Status != BridgeStatusDegraded {
		t.Fatalf("nil Snapshot() = %+v, want degraded snapshot", snapshot)
	}
}

func TestBridgeHealthService_RunCheckAndSetDegradedWithNilClient(t *testing.T) {
	svc := newBridgeHealthServiceWithConfig(nil, bridgeHealthConfig{
		startupAttempts:   1,
		startupInterval:   time.Millisecond,
		heartbeatInterval: time.Second,
		failureThreshold:  2,
	})
	if ok := svc.runCheck(context.Background()); ok {
		t.Fatal("runCheck(nil client) = true, want false")
	}
	if got := svc.Status(); got != BridgeStatusDegraded {
		t.Fatalf("Status() after runCheck = %q, want degraded", got)
	}

	checkTime := time.Date(2026, 3, 30, 20, 0, 0, 0, time.UTC)
	svc.setDegraded(checkTime, "manual override")
	snapshot := svc.Snapshot()
	if !snapshot.LastCheck.Equal(checkTime) {
		t.Fatalf("LastCheck = %v, want %v", snapshot.LastCheck, checkTime)
	}
	if svc.consecutiveFailures != svc.config.failureThreshold {
		t.Fatalf("consecutiveFailures = %d, want %d", svc.consecutiveFailures, svc.config.failureThreshold)
	}
}

func waitForBridgeHealthStatus(t *testing.T, svc *BridgeHealthService, want string) {
	t.Helper()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if got := svc.Snapshot().Status; got == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for bridge health status %q, last snapshot = %+v", want, svc.Snapshot())
}
