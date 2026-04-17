package ws

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/knowledge/liveartifact"
)

// Defaults for coalescing and per-block rate cap. Spec §9.5.
const (
	defaultCoalesceWindow   = 250 * time.Millisecond
	defaultPerBlockCooldown = 1 * time.Second
	liveArtifactsChangedEvt = "knowledge.asset.live_artifacts_changed"
)

// HubSender is the slice of *Hub the router actually uses. Keeping it an
// interface makes router tests independent of the real Hub event loop.
type HubSender interface {
	SendToClient(clientID string, payload []byte) bool
}

// LiveArtifactRouter owns per-(client,asset) subscription filter state,
// matches incoming hub events against each open asset's topic union, and
// emits coalesced `knowledge.asset.live_artifacts_changed` frames.
type LiveArtifactRouter struct {
	hub      HubSender
	registry *liveartifact.Registry
	now      func() time.Time
	newTimer func(d time.Duration, fn func()) *time.Timer

	coalesceWindow time.Duration
	blockCooldown  time.Duration

	mu      sync.Mutex
	clients map[string]map[string]*assetFilter // clientID -> assetID -> filter
}

// assetFilter is the open-asset subscription state for one client.
type assetFilter struct {
	clientID  string
	assetID   string
	projectID string

	mu     sync.Mutex
	blocks map[string]*blockSub

	// pending tracks blockIDs whose events arrived in the current
	// coalesce window and are awaiting the next flush.
	pending map[string]struct{}
	// deferred holds blockIDs rate-capped this flush; they re-emit on
	// the next window instead of being dropped outright.
	deferred map[string]struct{}
	timer    *time.Timer
}

type blockSub struct {
	blockID   string
	topics    []liveartifact.EventTopic
	lastFired time.Time
}

// assetOpenMessage matches the inbound control frame documented in §9.2.
type assetOpenMessage struct {
	Type    string `json:"type"`
	Payload struct {
		AssetID   string `json:"assetId"`
		ProjectID string `json:"projectId"`
		Blocks    []struct {
			BlockID   string          `json:"blockId"`
			LiveKind  string          `json:"liveKind"`
			TargetRef json.RawMessage `json:"targetRef"`
		} `json:"blocks"`
	} `json:"payload"`
}

type assetCloseMessage struct {
	Type    string `json:"type"`
	Payload struct {
		AssetID string `json:"assetId"`
	} `json:"payload"`
}

// NewLiveArtifactRouter wires a router against the hub and projector
// registry. Zero-duration fields fall back to defaults (250 ms / 1 s).
func NewLiveArtifactRouter(hub HubSender, registry *liveartifact.Registry) *LiveArtifactRouter {
	r := &LiveArtifactRouter{
		hub:            hub,
		registry:       registry,
		now:            time.Now,
		newTimer:       time.AfterFunc,
		coalesceWindow: defaultCoalesceWindow,
		blockCooldown:  defaultPerBlockCooldown,
		clients:        make(map[string]map[string]*assetFilter),
	}
	return r
}

// SetCoalesceWindow overrides the default 250 ms window. Tests only.
func (r *LiveArtifactRouter) SetCoalesceWindow(d time.Duration) {
	if d > 0 {
		r.coalesceWindow = d
	}
}

// SetBlockCooldown overrides the per-block 1 Hz cap. Tests only.
func (r *LiveArtifactRouter) SetBlockCooldown(d time.Duration) {
	if d > 0 {
		r.blockCooldown = d
	}
}

// setClock swaps the time source and timer factory. Tests only.
func (r *LiveArtifactRouter) setClock(now func() time.Time, newTimer func(d time.Duration, fn func()) *time.Timer) {
	if now != nil {
		r.now = now
	}
	if newTimer != nil {
		r.newTimer = newTimer
	}
}

// OnClientRegister is called by the Hub after a client is registered.
// We pre-create the per-client map so concurrent asset_open messages do
// not race on the top-level map.
func (r *LiveArtifactRouter) OnClientRegister(clientID string) {
	if clientID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clients[clientID]; !ok {
		r.clients[clientID] = make(map[string]*assetFilter)
	}
}

// OnClientUnregister drops all per-asset state for the client and stops
// any pending coalesce timers so they cannot fire against a stale view.
func (r *LiveArtifactRouter) OnClientUnregister(clientID string) {
	if clientID == "" {
		return
	}
	r.mu.Lock()
	assets := r.clients[clientID]
	delete(r.clients, clientID)
	r.mu.Unlock()
	for _, af := range assets {
		af.mu.Lock()
		if af.timer != nil {
			af.timer.Stop()
			af.timer = nil
		}
		af.mu.Unlock()
	}
}

// OnClientMessage parses an inbound control frame. Only asset_open and
// asset_close are handled; other types return nil so existing frames
// (subscribe / unsubscribe) pass through the hub's legacy path.
func (r *LiveArtifactRouter) OnClientMessage(clientID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return fmt.Errorf("liveartifact router: decode frame: %w", err)
	}
	switch probe.Type {
	case "asset_open":
		var msg assetOpenMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			return fmt.Errorf("liveartifact router: decode asset_open: %w", err)
		}
		return r.handleAssetOpen(clientID, msg)
	case "asset_close":
		var msg assetCloseMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			return fmt.Errorf("liveartifact router: decode asset_close: %w", err)
		}
		r.handleAssetClose(clientID, msg.Payload.AssetID)
		return nil
	default:
		return nil
	}
}

func (r *LiveArtifactRouter) handleAssetOpen(clientID string, msg assetOpenMessage) error {
	if clientID == "" || msg.Payload.AssetID == "" {
		return fmt.Errorf("liveartifact router: asset_open missing client or asset id")
	}
	af := &assetFilter{
		clientID:  clientID,
		assetID:   msg.Payload.AssetID,
		projectID: msg.Payload.ProjectID,
		blocks:    make(map[string]*blockSub, len(msg.Payload.Blocks)),
		pending:   make(map[string]struct{}),
		deferred:  make(map[string]struct{}),
	}
	for _, b := range msg.Payload.Blocks {
		if b.BlockID == "" {
			continue
		}
		sub := &blockSub{blockID: b.BlockID}
		proj, ok := r.registry.Lookup(liveartifact.LiveArtifactKind(b.LiveKind))
		if !ok {
			log.WithFields(log.Fields{
				"clientId": clientID,
				"assetId":  msg.Payload.AssetID,
				"blockId":  b.BlockID,
				"liveKind": b.LiveKind,
			}).Warn("liveartifact router: unknown live_kind on asset_open")
		} else {
			sub.topics = proj.Subscribe(b.TargetRef)
		}
		af.blocks[b.BlockID] = sub
	}

	r.mu.Lock()
	assets, ok := r.clients[clientID]
	if !ok {
		assets = make(map[string]*assetFilter)
		r.clients[clientID] = assets
	}
	assets[msg.Payload.AssetID] = af
	r.mu.Unlock()
	return nil
}

func (r *LiveArtifactRouter) handleAssetClose(clientID, assetID string) {
	if clientID == "" || assetID == "" {
		return
	}
	r.mu.Lock()
	assets, ok := r.clients[clientID]
	var af *assetFilter
	if ok {
		af = assets[assetID]
		delete(assets, assetID)
	}
	r.mu.Unlock()
	if af != nil {
		af.mu.Lock()
		if af.timer != nil {
			af.timer.Stop()
			af.timer = nil
		}
		af.mu.Unlock()
	}
}

// OnEvent matches the hub event against every open (client,asset) filter
// and schedules a coalesced emission for every match.
func (r *LiveArtifactRouter) OnEvent(eventType string, payload []byte) {
	if eventType == "" || len(payload) == 0 {
		return
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		// Non-object payloads are not matchable against scope keys.
		return
	}

	// Snapshot the filter set so we don't hold r.mu while touching each
	// filter's own lock.
	r.mu.Lock()
	filters := make([]*assetFilter, 0)
	for _, assets := range r.clients {
		for _, af := range assets {
			filters = append(filters, af)
		}
	}
	r.mu.Unlock()

	for _, af := range filters {
		r.matchFilter(af, eventType, decoded)
	}
}

func (r *LiveArtifactRouter) matchFilter(af *assetFilter, eventType string, payload map[string]any) {
	af.mu.Lock()
	affected := make([]string, 0)
	for _, sub := range af.blocks {
		for _, topic := range sub.topics {
			if topic.Event != eventType {
				continue
			}
			if !scopeMatches(topic.Scope, payload, af.projectID) {
				continue
			}
			affected = append(affected, sub.blockID)
			break
		}
	}
	if len(affected) == 0 {
		af.mu.Unlock()
		return
	}
	if af.pending == nil {
		af.pending = make(map[string]struct{})
	}
	for _, id := range affected {
		af.pending[id] = struct{}{}
	}
	if af.timer == nil {
		af.timer = r.newTimer(r.coalesceWindow, func() { r.flush(af) })
	}
	af.mu.Unlock()
}

// flush emits one frame for the asset. Rate-capped blocks are re-queued
// into `deferred` and re-armed on the next window.
func (r *LiveArtifactRouter) flush(af *assetFilter) {
	af.mu.Lock()
	af.timer = nil
	// Merge deferred from a prior flush into this window's pending set.
	for id := range af.deferred {
		af.pending[id] = struct{}{}
	}
	af.deferred = make(map[string]struct{})
	pending := af.pending
	af.pending = make(map[string]struct{})

	now := r.now()
	emit := make([]string, 0, len(pending))
	for id := range pending {
		sub := af.blocks[id]
		if sub == nil {
			continue
		}
		if !sub.lastFired.IsZero() && now.Sub(sub.lastFired) < r.blockCooldown {
			af.deferred[id] = struct{}{}
			continue
		}
		sub.lastFired = now
		emit = append(emit, id)
	}
	// If any blocks were deferred, re-arm the timer so they emit later.
	if len(af.deferred) > 0 && af.timer == nil {
		// Wait long enough that the oldest deferred block's cooldown
		// has elapsed. Use the full cooldown window to be safe.
		af.timer = r.newTimer(r.blockCooldown, func() { r.flush(af) })
	}
	clientID := af.clientID
	assetID := af.assetID
	af.mu.Unlock()

	if len(emit) == 0 {
		return
	}
	frame := buildChangedFrame(assetID, emit)
	if frame == nil {
		return
	}
	if !r.hub.SendToClient(clientID, frame) {
		log.WithFields(log.Fields{
			"clientId": clientID,
			"assetId":  assetID,
		}).Debug("liveartifact router: SendToClient missed")
	}
}

func buildChangedFrame(assetID string, blockIDs []string) []byte {
	payload := map[string]any{
		"asset_id":           assetID,
		"block_ids_affected": blockIDs,
	}
	frame := map[string]any{
		"type":    liveArtifactsChangedEvt,
		"payload": payload,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		log.WithError(err).Warn("liveartifact router: marshal frame failed")
		return nil
	}
	return data
}

// scopeMatches applies the AND predicate of topic.Scope against payload,
// substituting $asset_project with projectID. Keys are compared in both
// snake_case and camelCase forms against the payload.
func scopeMatches(scope map[string]string, payload map[string]any, projectID string) bool {
	for k, want := range scope {
		if want == liveartifact.ScopeAssetProject {
			want = projectID
		}
		got, ok := lookupScopeKey(payload, k)
		if !ok {
			return false
		}
		if got != want {
			return false
		}
	}
	return true
}

// lookupScopeKey tries snake_case first, then camelCase. Only top-level
// keys are consulted; projectors scope by flat ids today.
func lookupScopeKey(payload map[string]any, key string) (string, bool) {
	if v, ok := payload[key]; ok {
		return stringify(v)
	}
	camel := snakeToCamel(key)
	if camel != key {
		if v, ok := payload[camel]; ok {
			return stringify(v)
		}
	}
	return "", false
}

func stringify(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case nil:
		return "", false
	default:
		return fmt.Sprintf("%v", t), true
	}
}

func snakeToCamel(in string) string {
	out := make([]byte, 0, len(in))
	upper := false
	for i := 0; i < len(in); i++ {
		c := in[i]
		if c == '_' {
			upper = true
			continue
		}
		if upper && c >= 'a' && c <= 'z' {
			out = append(out, c-('a'-'A'))
		} else {
			out = append(out, c)
		}
		upper = false
	}
	return string(out)
}
