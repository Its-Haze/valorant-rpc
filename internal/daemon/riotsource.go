package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/riotchat"
	"github.com/its-haze/valorant-rpc/internal/state"
)

// presenceStallThreshold is how long a live connection may go without a
// single presence before the state is treated as unknown.
const presenceStallThreshold = 30 * time.Second

// startTimeout bounds the session fetch and the presence snapshot. Connect
// runs outside the supervisor's context, so it cannot wait forever.
const startTimeout = 15 * time.Second

// presenceWatcher resolves who the player is, subscribes and emits their
// presence. *riotchat.Watcher satisfies this.
type presenceWatcher interface {
	Start(ctx context.Context) error
}

// RiotSource is the Connector the Riot supervisor drives: it owns the local
// API connection and feeds every presence it reads into the state manager.
type RiotSource struct {
	client  Connector
	watcher presenceWatcher
	state   *state.Manager
	logger  zerolog.Logger

	// now is overridden in tests to drive the stall threshold.
	now func() time.Time

	mu          sync.Mutex
	connectedAt time.Time
	presenceAt  time.Time
}

// NewRiotSource builds a source over client. newWatcher is handed the
// callback that every decoded presence arrives on.
func NewRiotSource(client Connector, stateMgr *state.Manager, logger zerolog.Logger, newWatcher func(onUpdate func(riotchat.Presence)) presenceWatcher) *RiotSource {
	s := &RiotSource{client: client, state: stateMgr, logger: logger}
	s.watcher = newWatcher(s.onPresence)
	return s
}

// Connect opens the local API connection, then resolves the player and their
// current presence. A failure on either half leaves nothing connected.
func (s *RiotSource) Connect() error {
	if err := s.client.Connect(); err != nil {
		return err
	}

	s.mark(s.clock(), time.Time{})

	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	if err := s.watcher.Start(ctx); err != nil {
		_ = s.client.Disconnect()
		return err
	}
	return nil
}

// Disconnect closes the connection and forgets the presence it was showing,
// so a reconnect does not start from whatever was last true.
func (s *RiotSource) Disconnect() error {
	s.mark(time.Time{}, time.Time{})
	s.state.Apply(func(st *state.State) { *st = *state.NewState() })
	return s.client.Disconnect()
}

// IsConnected reports the underlying client's connection.
func (s *RiotSource) IsConnected() bool { return s.client.IsConnected() }

// PresenceStalled reports a live connection that has never read a presence.
// One arriving ends it for good: a still player publishes nothing at all.
func (s *RiotSource) PresenceStalled() bool {
	if !s.IsConnected() {
		return false
	}

	s.mu.Lock()
	connectedAt, presenceAt := s.connectedAt, s.presenceAt
	s.mu.Unlock()

	if connectedAt.IsZero() || !presenceAt.IsZero() {
		return false
	}
	return s.clock().Sub(connectedAt) >= presenceStallThreshold
}

// onPresence is called from the websocket listener goroutine and from
// Connect, so it must neither block nor assume a goroutine.
func (s *RiotSource) onPresence(p riotchat.Presence) {
	s.mu.Lock()
	s.presenceAt = s.clock()
	s.mu.Unlock()

	s.state.Apply(applyPresence(p))
}

func (s *RiotSource) mark(connectedAt, presenceAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedAt, s.presenceAt = connectedAt, presenceAt
}

func (s *RiotSource) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}
