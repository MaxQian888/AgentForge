package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
)

type spawnVerificationBridge struct {
	mu          sync.Mutex
	statusCalls int
	statusResp  *BridgeStatusResponse
}

func (b *spawnVerificationBridge) Execute(context.Context, BridgeExecuteRequest) (*BridgeExecuteResponse, error) {
	return nil, nil
}

func (b *spawnVerificationBridge) GetStatus(_ context.Context, _ string) (*BridgeStatusResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.statusCalls++
	if b.statusResp != nil {
		return b.statusResp, nil
	}
	return &BridgeStatusResponse{State: "running"}, nil
}

func (b *spawnVerificationBridge) GetPoolSummary(context.Context) (*bridgeclient.PoolSummaryResponse, error) {
	return nil, nil
}

func (b *spawnVerificationBridge) Health(context.Context) error {
	return nil
}

func (b *spawnVerificationBridge) Cancel(context.Context, string, string) error {
	return nil
}

func (b *spawnVerificationBridge) Pause(context.Context, string, string) (*BridgePauseResponse, error) {
	return nil, nil
}

func (b *spawnVerificationBridge) Resume(context.Context, BridgeExecuteRequest) (*BridgeResumeResponse, error) {
	return nil, nil
}

func (b *spawnVerificationBridge) calls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.statusCalls
}

func TestVerifySpawnStartedSkipsFallbackWhenBridgeActivityArrives(t *testing.T) {
	bridge := &spawnVerificationBridge{}
	svc := NewAgentService(nil, nil, nil, nil, bridge, nil)
	taskID := uuid.New()

	done := make(chan struct{})
	go func() {
		svc.verifySpawnStarted(taskID, uuid.New(), time.Now().UTC())
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	svc.noteBridgeActivity(taskID, time.Now().UTC())

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for bridge activity waiter")
	}

	if bridge.calls() != 0 {
		t.Fatalf("GetStatus() calls = %d, want 0", bridge.calls())
	}
}

func TestVerifySpawnStartedFallsBackToGetStatusWithoutBridgeActivity(t *testing.T) {
	bridge := &spawnVerificationBridge{}
	svc := NewAgentService(nil, nil, nil, nil, bridge, nil)

	svc.verifySpawnStarted(uuid.New(), uuid.New(), time.Now().UTC())

	if bridge.calls() != 1 {
		t.Fatalf("GetStatus() calls = %d, want 1", bridge.calls())
	}
}
