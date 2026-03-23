// Package pool manages active agent concurrency.
package pool

import (
	"errors"
	"sync"
)

var (
	ErrPoolFull     = errors.New("agent pool is full")
	ErrNotInPool    = errors.New("agent not in pool")
)

// AgentEntry represents an active agent in the pool.
type AgentEntry struct {
	RunID    string
	TaskID   string
	MemberID string
}

// Pool tracks active agents and enforces max concurrency.
type Pool struct {
	mu       sync.RWMutex
	agents   map[string]*AgentEntry // keyed by run ID
	maxSize  int
}

// NewPool creates an agent pool with the given max concurrency.
func NewPool(maxSize int) *Pool {
	return &Pool{
		agents:  make(map[string]*AgentEntry),
		maxSize: maxSize,
	}
}

// Acquire adds an agent to the pool. Returns ErrPoolFull if at capacity.
func (p *Pool) Acquire(runID, taskID, memberID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.agents) >= p.maxSize {
		return ErrPoolFull
	}

	p.agents[runID] = &AgentEntry{
		RunID:    runID,
		TaskID:   taskID,
		MemberID: memberID,
	}
	return nil
}

// Release removes an agent from the pool.
func (p *Pool) Release(runID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.agents[runID]; !ok {
		return ErrNotInPool
	}
	delete(p.agents, runID)
	return nil
}

// ActiveCount returns the number of active agents.
func (p *Pool) ActiveCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.agents)
}

// Available returns the number of available slots.
func (p *Pool) Available() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxSize - len(p.agents)
}

// List returns all active agent entries.
func (p *Pool) List() []AgentEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entries := make([]AgentEntry, 0, len(p.agents))
	for _, e := range p.agents {
		entries = append(entries, *e)
	}
	return entries
}

// Has checks if a run ID is in the pool.
func (p *Pool) Has(runID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.agents[runID]
	return ok
}
