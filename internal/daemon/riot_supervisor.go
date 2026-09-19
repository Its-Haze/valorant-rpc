package daemon

import (
	"context"
	"sync/atomic"
	"time"
)

// ProcessChecker reports whether any process in names is currently running.
// internal/process.Checker satisfies this.
type ProcessChecker interface {
	IsRunning(names ...string) (bool, error)
}

// RiotSupervisor wraps a Supervisor for the Riot Client connection, and
// separately tracks whether Valorant is actually running.
type RiotSupervisor struct {
	*Supervisor

	checker      ProcessChecker
	source       Connector
	pollInterval time.Duration
	gameUp       atomic.Bool
	connected    atomic.Bool
}

// NewRiotSupervisor builds a RiotSupervisor around connector (typically
// *RiotSource). processPollInterval controls the game-process check cadence.
func NewRiotSupervisor(
	connector Connector,
	checker ProcessChecker,
	retryInterval, connectPollInterval, processPollInterval time.Duration,
	opts ...Option,
) *RiotSupervisor {
	rs := &RiotSupervisor{
		checker:      checker,
		source:       connector,
		pollInterval: processPollInterval,
	}
	rs.Supervisor = NewSupervisor(&gameGatedConnector{Connector: connector, rs: rs}, retryInterval, connectPollInterval,
		append([]Option{WithGate(&gameGate{rs: rs})}, opts...)...)

	// Wrap the caller's callbacks so Connected() tracks the gated connector
	// rather than the raw client, which knows nothing about the game process.
	userOnConnect := rs.Supervisor.onConnect
	userOnDisconnect := rs.Supervisor.onDisconnect
	rs.Supervisor.onConnect = func() {
		rs.connected.Store(true)
		if userOnConnect != nil {
			userOnConnect()
		}
	}
	rs.Supervisor.onDisconnect = func() {
		rs.connected.Store(false)
		if userOnDisconnect != nil {
			userOnDisconnect()
		}
	}
	return rs
}

// Connected reports whether the Riot Client's local API is reachable and
// Valorant is still running, per the last connect/disconnect callback.
func (rs *RiotSupervisor) Connected() bool {
	return rs.connected.Load()
}

// GameRunning reports whether Valorant was running as of the last poll.
func (rs *RiotSupervisor) GameRunning() bool {
	return rs.gameUp.Load()
}

// stallReporter is a Connector that also knows whether it has read any
// presence on the current connection. *RiotSource is one.
type stallReporter interface {
	PresenceStalled() bool
}

// PresenceStalled reports a connection that is up but has produced nothing.
// It reads Connected() too, so the two can never disagree about being up.
func (rs *RiotSupervisor) PresenceStalled() bool {
	if !rs.Connected() {
		return false
	}
	reporter, ok := rs.source.(stallReporter)
	return ok && reporter.PresenceStalled()
}

// gameGate blocks connect attempts until Valorant is running. Dialing the
// Riot Client on its own would connect to a client with no Valorant session.
type gameGate struct {
	rs *RiotSupervisor
}

func (g *gameGate) Ready() (bool, error) { return g.rs.GameRunning(), nil }

// gameGatedConnector drops the connection when Valorant exits. The Riot
// Client outlives the game, so the websocket stays up with nothing to report.
type gameGatedConnector struct {
	Connector
	rs *RiotSupervisor
}

func (c *gameGatedConnector) IsConnected() bool {
	return c.Connector.IsConnected() && c.rs.GameRunning()
}

// Run polls for Valorant's process alongside the underlying Supervisor's
// connect-retry loop, and blocks until ctx is canceled.
func (rs *RiotSupervisor) Run(ctx context.Context) {
	// Prime gameUp before any connect attempt is evaluated, so the gate never
	// sees a stale "not running" default.
	rs.checkGame()
	go rs.pollGame(ctx)
	rs.Supervisor.Run(ctx)
}

func (rs *RiotSupervisor) pollGame(ctx context.Context) {
	ticker := time.NewTicker(rs.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rs.checkGame()
		}
	}
}

// checkGame requires both processes. The Riot Client alone is a launcher
// sitting idle, which is not Valorant running.
func (rs *RiotSupervisor) checkGame() {
	client, err := rs.checker.IsRunning(riotClientProcessNames...)
	if err != nil {
		// Leave the last known state in place rather than flapping to
		// "not running" on a transient process-listing error.
		return
	}
	game, err := rs.checker.IsRunning(valorantProcessNames...)
	if err != nil {
		return
	}
	rs.gameUp.Store(client && game)
}
