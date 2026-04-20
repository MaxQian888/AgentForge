package ws

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/knowledge/liveartifact"
	"github.com/agentforge/server/internal/model"
)

// --- test fakes ---

type fakeHub struct {
	mu     sync.Mutex
	frames map[string][][]byte
}

func newFakeHub() *fakeHub {
	return &fakeHub{frames: make(map[string][][]byte)}
}

func (f *fakeHub) SendToClient(clientID string, data []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	dup := make([]byte, len(data))
	copy(dup, data)
	f.frames[clientID] = append(f.frames[clientID], dup)
	return true
}

func (f *fakeHub) framesFor(clientID string) [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]byte, len(f.frames[clientID]))
	copy(out, f.frames[clientID])
	return out
}

// stubProjector publishes a fixed topic list. Project is never called by
// the router so we stub it minimally.
type stubProjector struct {
	kind   liveartifact.LiveArtifactKind
	topics []liveartifact.EventTopic
}

func (s *stubProjector) Kind() liveartifact.LiveArtifactKind { return s.kind }
func (s *stubProjector) RequiredRole() liveartifact.Role     { return liveartifact.RoleViewer }
func (s *stubProjector) Project(
	_ context.Context,
	_ model.PrincipalContext,
	_ uuid.UUID,
	_ json.RawMessage,
	_ json.RawMessage,
) (liveartifact.ProjectionResult, error) {
	return liveartifact.ProjectionResult{Status: liveartifact.StatusOK}, nil
}
func (s *stubProjector) Subscribe(_ json.RawMessage) []liveartifact.EventTopic {
	// Return a copy so callers may mutate safely.
	out := make([]liveartifact.EventTopic, len(s.topics))
	copy(out, s.topics)
	return out
}

// --- test helpers ---

// decodeChanged parses a router-emitted frame into its inner payload.
type changedFrame struct {
	Type    string `json:"type"`
	Payload struct {
		AssetID          string   `json:"asset_id"`
		BlockIDsAffected []string `json:"block_ids_affected"`
	} `json:"payload"`
}

func decodeChanged(t *testing.T, data []byte) changedFrame {
	t.Helper()
	var cf changedFrame
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("decode frame: %v; raw=%s", err, string(data))
	}
	if cf.Type != "knowledge.asset.live_artifacts_changed" {
		t.Fatalf("unexpected frame type %q; raw=%s", cf.Type, string(data))
	}
	return cf
}

// openAssetBlock crafts an asset_open payload with a single block.
func openAssetBlock(assetID, projectID, blockID, liveKind string) []byte {
	msg := map[string]any{
		"type": "asset_open",
		"payload": map[string]any{
			"assetId":   assetID,
			"projectId": projectID,
			"blocks": []map[string]any{
				{
					"blockId":   blockID,
					"liveKind":  liveKind,
					"targetRef": map[string]any{},
				},
			},
		},
	}
	out, _ := json.Marshal(msg)
	return out
}

func buildRegistry(kind liveartifact.LiveArtifactKind, topics []liveartifact.EventTopic) *liveartifact.Registry {
	reg := liveartifact.NewRegistry()
	reg.Register(&stubProjector{kind: kind, topics: topics})
	return reg
}

// waitForFrames polls until at least n frames are recorded or timeout.
func waitForFrames(t *testing.T, hub *fakeHub, clientID string, n int, timeout time.Duration) [][]byte {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		frames := hub.framesFor(clientID)
		if len(frames) >= n {
			return frames
		}
		if time.Now().After(deadline) {
			return frames
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// --- tests ---

func TestRouter_Coalescing(t *testing.T) {
	topics := []liveartifact.EventTopic{{
		Event: "agent.cost_update",
		Scope: map[string]string{"agent_run_id": "X"},
	}}
	reg := buildRegistry(liveartifact.KindAgentRun, topics)
	hub := newFakeHub()
	r := NewLiveArtifactRouter(hub, reg)
	r.SetCoalesceWindow(50 * time.Millisecond)

	clientID := "c1"
	r.OnClientRegister(clientID)
	if err := r.OnClientMessage(clientID, openAssetBlock("A1", "P1", "B1", string(liveartifact.KindAgentRun))); err != nil {
		t.Fatalf("asset_open: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{"agent_run_id": "X"})
	for i := 0; i < 5; i++ {
		r.OnEvent("agent.cost_update", payload)
	}

	frames := waitForFrames(t, hub, clientID, 1, 300*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("want 1 coalesced frame, got %d", len(frames))
	}
	cf := decodeChanged(t, frames[0])
	if cf.Payload.AssetID != "A1" {
		t.Fatalf("asset_id: want A1, got %q", cf.Payload.AssetID)
	}
	if len(cf.Payload.BlockIDsAffected) != 1 || cf.Payload.BlockIDsAffected[0] != "B1" {
		t.Fatalf("block_ids_affected: want [B1], got %v", cf.Payload.BlockIDsAffected)
	}
}

func TestRouter_RateCap(t *testing.T) {
	topics := []liveartifact.EventTopic{{
		Event: "agent.cost_update",
		Scope: map[string]string{"agent_run_id": "X"},
	}}
	reg := buildRegistry(liveartifact.KindAgentRun, topics)
	hub := newFakeHub()
	r := NewLiveArtifactRouter(hub, reg)
	// Use a short coalesce window so the first emission lands fast, but
	// keep the cooldown meaningfully larger so spacing < cooldown is
	// definitely rate-capped. Spec calls for 1 Hz per block; scale down
	// to keep the test fast but preserve the inequality.
	r.SetCoalesceWindow(10 * time.Millisecond)
	r.SetBlockCooldown(250 * time.Millisecond)

	clientID := "c1"
	r.OnClientRegister(clientID)
	if err := r.OnClientMessage(clientID, openAssetBlock("A1", "P1", "B1", string(liveartifact.KindAgentRun))); err != nil {
		t.Fatalf("asset_open: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{"agent_run_id": "X"})

	// Event 1 at t=0 => emits at ~10ms.
	r.OnEvent("agent.cost_update", payload)
	time.Sleep(100 * time.Millisecond)
	// Event 2 at t=100ms => inside cooldown, must be deferred.
	r.OnEvent("agent.cost_update", payload)
	time.Sleep(100 * time.Millisecond)
	// Event 3 at t=200ms => still inside cooldown.
	r.OnEvent("agent.cost_update", payload)

	// By now only one frame should have been sent. Wait past cooldown
	// for the deferred emission.
	frames := hub.framesFor(clientID)
	if len(frames) != 1 {
		t.Fatalf("want 1 frame within cooldown, got %d", len(frames))
	}

	// Wait for deferred flush to fire (cooldown 250ms after first flush).
	frames = waitForFrames(t, hub, clientID, 2, 500*time.Millisecond)
	if len(frames) != 2 {
		t.Fatalf("want 2 frames after cooldown, got %d", len(frames))
	}
}

func TestRouter_FanoutScoping(t *testing.T) {
	topics := []liveartifact.EventTopic{{
		Event: "agent.cost_update",
		Scope: map[string]string{"agent_run_id": "X"},
	}}
	reg := buildRegistry(liveartifact.KindAgentRun, topics)
	// Register a second stub keyed by a different kind so client B
	// explicitly uses a non-matching topic set.
	reg.Register(&stubProjector{
		kind: liveartifact.KindReview,
		topics: []liveartifact.EventTopic{{
			Event: "review.updated",
			Scope: map[string]string{"review_id": "R1"},
		}},
	})
	hub := newFakeHub()
	r := NewLiveArtifactRouter(hub, reg)
	r.SetCoalesceWindow(30 * time.Millisecond)

	r.OnClientRegister("cA")
	r.OnClientRegister("cB")
	if err := r.OnClientMessage("cA", openAssetBlock("A1", "P1", "BA", string(liveartifact.KindAgentRun))); err != nil {
		t.Fatalf("open A: %v", err)
	}
	if err := r.OnClientMessage("cB", openAssetBlock("B1", "P2", "BB", string(liveartifact.KindReview))); err != nil {
		t.Fatalf("open B: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{"agent_run_id": "X"})
	r.OnEvent("agent.cost_update", payload)

	framesA := waitForFrames(t, hub, "cA", 1, 300*time.Millisecond)
	framesB := waitForFrames(t, hub, "cB", 1, 100*time.Millisecond)
	if len(framesA) != 1 {
		t.Fatalf("client A want 1 frame, got %d", len(framesA))
	}
	if len(framesB) != 0 {
		t.Fatalf("client B must not receive frames, got %d", len(framesB))
	}
}

func TestRouter_CleanupOnDisconnect(t *testing.T) {
	topics := []liveartifact.EventTopic{{
		Event: "agent.cost_update",
		Scope: map[string]string{"agent_run_id": "X"},
	}}
	reg := buildRegistry(liveartifact.KindAgentRun, topics)
	hub := newFakeHub()
	r := NewLiveArtifactRouter(hub, reg)
	r.SetCoalesceWindow(30 * time.Millisecond)

	before := runtime.NumGoroutine()

	clientID := "c1"
	r.OnClientRegister(clientID)
	if err := r.OnClientMessage(clientID, openAssetBlock("A1", "P1", "B1", string(liveartifact.KindAgentRun))); err != nil {
		t.Fatalf("asset_open: %v", err)
	}
	r.OnClientUnregister(clientID)

	payload, _ := json.Marshal(map[string]any{"agent_run_id": "X"})
	for i := 0; i < 3; i++ {
		r.OnEvent("agent.cost_update", payload)
	}
	time.Sleep(100 * time.Millisecond)

	if frames := hub.framesFor(clientID); len(frames) != 0 {
		t.Fatalf("want 0 frames after unregister, got %d", len(frames))
	}

	// Allow AfterFunc goroutines to wind down.
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	// Tolerate small fluctuations from the runtime scheduler.
	if after > before+3 {
		t.Fatalf("goroutine leak suspected: before=%d after=%d", before, after)
	}
}

func TestRouter_ScopeSubstitution(t *testing.T) {
	topics := []liveartifact.EventTopic{{
		Event: "agent.cost_update",
		Scope: map[string]string{"project_id": liveartifact.ScopeAssetProject},
	}}
	reg := buildRegistry(liveartifact.KindCostSummary, topics)
	hub := newFakeHub()
	r := NewLiveArtifactRouter(hub, reg)
	r.SetCoalesceWindow(20 * time.Millisecond)

	clientID := "c1"
	r.OnClientRegister(clientID)
	if err := r.OnClientMessage(clientID, openAssetBlock("A1", "P1", "B1", string(liveartifact.KindCostSummary))); err != nil {
		t.Fatalf("asset_open: %v", err)
	}

	// project_id=P2 must NOT match (different from asset's project).
	no, _ := json.Marshal(map[string]any{"project_id": "P2"})
	r.OnEvent("agent.cost_update", no)
	time.Sleep(60 * time.Millisecond)
	if frames := hub.framesFor(clientID); len(frames) != 0 {
		t.Fatalf("unexpected emission for P2: %d frames", len(frames))
	}

	// project_id=P1 matches.
	yes, _ := json.Marshal(map[string]any{"project_id": "P1"})
	r.OnEvent("agent.cost_update", yes)

	frames := waitForFrames(t, hub, clientID, 1, 200*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("want 1 frame for P1, got %d", len(frames))
	}
}

func TestRouter_CamelCasePayloadMatch(t *testing.T) {
	topics := []liveartifact.EventTopic{{
		Event: "agent.cost_update",
		Scope: map[string]string{
			"project_id":   liveartifact.ScopeAssetProject,
			"agent_run_id": "X",
		},
	}}
	reg := buildRegistry(liveartifact.KindAgentRun, topics)
	hub := newFakeHub()
	r := NewLiveArtifactRouter(hub, reg)
	r.SetCoalesceWindow(20 * time.Millisecond)

	clientID := "c1"
	r.OnClientRegister(clientID)
	if err := r.OnClientMessage(clientID, openAssetBlock("A1", "P1", "B1", string(liveartifact.KindAgentRun))); err != nil {
		t.Fatalf("asset_open: %v", err)
	}

	// CamelCase payload keys; scope keys stay snake_case.
	payload, _ := json.Marshal(map[string]any{
		"projectId":  "P1",
		"agentRunId": "X",
	})
	r.OnEvent("agent.cost_update", payload)

	frames := waitForFrames(t, hub, clientID, 1, 200*time.Millisecond)
	if len(frames) != 1 {
		t.Fatalf("want 1 frame for camelCase payload, got %d", len(frames))
	}
}

func TestRouter_UnsupportedLiveKind(t *testing.T) {
	reg := liveartifact.NewRegistry() // no projectors registered
	hub := newFakeHub()
	r := NewLiveArtifactRouter(hub, reg)
	r.SetCoalesceWindow(20 * time.Millisecond)

	clientID := "c1"
	r.OnClientRegister(clientID)
	// live_kind is unknown; router must accept the frame without error.
	if err := r.OnClientMessage(clientID, openAssetBlock("A1", "P1", "B1", "totally_made_up")); err != nil {
		t.Fatalf("asset_open with unknown kind: %v", err)
	}
	// Any event must NOT produce a frame for this block.
	payload, _ := json.Marshal(map[string]any{"agent_run_id": "X"})
	r.OnEvent("agent.cost_update", payload)
	time.Sleep(80 * time.Millisecond)
	if frames := hub.framesFor(clientID); len(frames) != 0 {
		t.Fatalf("unexpected frames for unsupported live_kind: %d", len(frames))
	}
}
