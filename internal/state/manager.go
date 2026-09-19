package state

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// updateBuffer is how far the shared Updates() channel may run ahead of its
// reader before changes start being dropped.
const updateBuffer = 100

// Manager guards the state and fans changes out. Every writer goes through
// Apply, so the context entry stamp is the only derived field to maintain.
type Manager struct {
	mu      sync.RWMutex
	current *State
	logger  zerolog.Logger

	// updates is created on first Updates() call. A daemon that only ever
	// subscribes would otherwise fill it and warn on every change after.
	updates chan *State
	closed  bool

	// Fan-out channels handed to Subscribe callers. Each gets its own copy
	// of every change; a slow reader only ever misses intermediate states.
	subs []chan *State
}

// NewManager returns a manager holding the default state.
func NewManager(logger zerolog.Logger) *Manager {
	return &Manager{current: NewState(), logger: logger}
}

// Get returns a copy of the current state.
func (m *Manager) Get() *State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.current.Copy()
}

// Apply mutates a copy of the state and publishes it if anything changed.
// The mutator runs under the write lock, so it must not call back in.
func (m *Manager) Apply(mutate func(*State)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	next := m.current.Copy()
	mutate(next)

	if next.PhaseContext() != m.current.PhaseContext() {
		next.ContextEnteredAt = time.Now()
	}
	if m.current.Equals(next) {
		return
	}

	m.logger.Debug().
		Str("context", string(next.PhaseContext())).
		Str("queue", string(next.QueueID)).
		Msg("State updated")

	m.current = next
	m.broadcast()
}

// Updates returns the shared change channel, creating it on first use.
func (m *Manager) Updates() <-chan *State {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updates == nil {
		m.updates = make(chan *State, updateBuffer)
		if m.closed {
			close(m.updates)
		}
	}
	return m.updates
}

// Subscribe returns a fresh channel that receives a copy of the state on
// every change, independent of Updates() and of any other subscriber.
func (m *Manager) Subscribe() <-chan *State {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *State, 1)
	// Subscribing during shutdown hands back a channel nothing will ever
	// close, and a caller ranging over it would block there forever.
	if m.closed {
		close(ch)
		return ch
	}
	m.subs = append(m.subs, ch)
	return ch
}

// broadcast pushes the current state to Updates() and to every subscriber.
// Callers must hold m.mu.
func (m *Manager) broadcast() {
	if m.updates != nil {
		select {
		case m.updates <- m.current.Copy():
		default:
			m.logger.Warn().Msg("State update channel full, dropping update")
		}
	}

	for _, ch := range m.subs {
		// Coalesce: drop a stale pending value, then push the latest. Both
		// sends are non-blocking so a slow subscriber never stalls a writer.
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- m.current.Copy():
		default:
		}
	}
}

// Close closes the update channels. Applying after this panics, so the
// daemon closes the manager only once everything feeding it has stopped.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}
	m.closed = true

	if m.updates != nil {
		close(m.updates)
	}
	for _, ch := range m.subs {
		close(ch)
	}
	m.subs = nil
}
