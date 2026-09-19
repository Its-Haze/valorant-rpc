// Package daemon implements the Daemon's core lifecycle.
// See ADR-0002.
package daemon

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/discord"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// runner is anything Daemon can start and supervise for the duration of a
// run. *Supervisor and *RiotSupervisor both satisfy this.
type runner interface {
	Run(ctx context.Context)
}

// discordRunner also reports whether Discord is connected, so Daemon can
// resend presence the moment it reconnects. *Supervisor satisfies this.
type discordRunner interface {
	runner
	Connected() bool
}

// riotRunner also reports the connection, the game process and the stall,
// the inputs Daemon needs to pick real presence, placeholder, or cleared.
type riotRunner interface {
	runner
	Connected() bool
	GameRunning() bool
	PresenceStalled() bool
}

// CatalogueRefresher keeps the content catalogue warm for the duration of a
// run. *content.Cache satisfies it.
type CatalogueRefresher interface {
	Run(ctx context.Context)
}

// AgentLookup resolves which agent the player is on. It fires once per entry
// into agent select or a match, and v0.1 wires nothing into it.
type AgentLookup interface {
	Lookup(ctx context.Context, st *state.State)
}

// presenceMode is what Daemon currently shows on Discord. See ADR-0002.
type presenceMode int

const (
	modeUnknown presenceMode = iota
	modeConnected
	modePlaceholder
	modeCleared
)

// DaemonOption configures a Daemon.
type DaemonOption func(*Daemon)

// WithCatalogue wires the content catalogue's refresh loop into Run.
func WithCatalogue(c CatalogueRefresher) DaemonOption {
	return func(d *Daemon) { d.catalogue = c }
}

// WithAgentLookup wires the v0.2 agent lookup into the context-entry hook.
func WithAgentLookup(a AgentLookup) DaemonOption {
	return func(d *Daemon) { d.agents = a }
}

// Daemon owns the Discord and Riot Connection Supervisors and drives Discord
// presence off their state. Run is the single seam here. See ADR-0002.
type Daemon struct {
	discord discordRunner
	riot    riotRunner
	updater *discord.Updater
	state   *state.Manager
	agents  AgentLookup
	logger  zerolog.Logger

	// catalogue is optional: a Daemon without one still drives presence,
	// with every content lookup missing.
	catalogue CatalogueRefresher

	presencePollInterval time.Duration
	placeholderInterval  time.Duration

	// paused is a runtime flag, never persisted to Config. It starts false
	// on every Daemon and clears presence for as long as it is set.
	paused      atomic.Bool
	pauseSignal chan struct{}
}

// New builds a Daemon that drives presence from stateMgr through updater.
func New(
	discordSup discordRunner,
	riotSup riotRunner,
	updater *discord.Updater,
	stateMgr *state.Manager,
	logger zerolog.Logger,
	presencePollInterval, placeholderInterval time.Duration,
	opts ...DaemonOption,
) *Daemon {
	d := &Daemon{
		discord:              discordSup,
		riot:                 riotSup,
		updater:              updater,
		state:                stateMgr,
		logger:               logger,
		presencePollInterval: presencePollInterval,
		placeholderInterval:  placeholderInterval,
		pauseSignal:          make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// SetPaused sets the runtime pause flag and wakes the presence loop so it
// takes effect at once. Paused clears presence; unpausing resumes it.
func (d *Daemon) SetPaused(paused bool) {
	d.paused.Store(paused)
	select {
	case d.pauseSignal <- struct{}{}:
	default:
	}
}

// IsPaused reports the pause flag. A fresh Daemon is always unpaused.
func (d *Daemon) IsPaused() bool { return d.paused.Load() }

// DiscordConnected reports whether Discord IPC is currently reachable.
func (d *Daemon) DiscordConnected() bool { return d.discord.Connected() }

// RiotConnected reports whether the Riot Client's local API is reachable.
func (d *Daemon) RiotConnected() bool { return d.riot.Connected() }

// GameRunning reports whether Valorant itself is running.
func (d *Daemon) GameRunning() bool { return d.riot.GameRunning() }

// PresenceStalled reports a live connection that has never read a presence,
// which the GUI status view shows instead of a state it cannot vouch for.
func (d *Daemon) PresenceStalled() bool { return d.riot.PresenceStalled() }

// LastSent returns the presence the Updater last pushed to Discord.
func (d *Daemon) LastSent() discord.LastSent { return d.updater.LastSent() }

// SubscribeState returns a fresh channel of state changes for the status
// bridge, independent of the presence loop's own subscription.
func (d *Daemon) SubscribeState() <-chan *state.State { return d.state.Subscribe() }

// Run starts both supervisors and the presence loop, and blocks until ctx
// is canceled; a connection failure on either side never returns early.
func (d *Daemon) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(4)

	if d.catalogue != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.catalogue.Run(ctx)
		}()
	}

	go func() {
		defer wg.Done()
		d.discord.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		d.riot.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		d.updater.Run(ctx)
	}()
	go func() {
		defer wg.Done()
		d.presenceLoop(ctx)
	}()

	wg.Wait()
}

// presenceLoop shows real presence once a presence has been read, a
// placeholder while Valorant is starting, or clears presence. See ADR-0002.
func (d *Daemon) presenceLoop(ctx context.Context) {
	ticker := time.NewTicker(d.presencePollInterval)
	defer ticker.Stop()

	cfgUpdates := d.updater.ConfigChanges()

	mode := modeUnknown
	discordConnected := false
	waitingForDiscordLogged := false
	stallLogged := false

	var placeholderTicker *time.Ticker
	var placeholderC <-chan time.Time
	var placeholderStart int64

	stopPlaceholder := func() {
		if placeholderTicker != nil {
			placeholderTicker.Stop()
			placeholderTicker = nil
			placeholderC = nil
		}
	}
	defer stopPlaceholder()

	sendPlaceholder := func() {
		// Skip while Discord isn't connected yet; the reconnect handler below resends once it is.
		if !d.discord.Connected() {
			return
		}
		d.updater.UpdateLaunchingPlaceholder(placeholderStart)
	}

	// The agent lookup runs only in a match, never in agent select: the game
	// log names the agent once its pawn spawns, which is after the match has
	// started. It keeps running until one resolves, because the first reads
	// of a match legitimately find nothing.
	agentResolved := false
	fireLookup := func(st *state.State) {
		if st.PhaseContext() != types.ContextInMatch {
			if agentResolved || st.AgentID != "" {
				d.state.Apply(func(s *state.State) { s.AgentID = "" })
			}
			agentResolved = false
			return
		}
		if agentResolved || d.agents == nil {
			return
		}
		d.agents.Lookup(ctx, st)
		agentResolved = d.state.Get().AgentID != ""
	}

	// placeholderAllowed keeps the launching presence behind its setting, for
	// users who would rather show nothing until the game reports something.
	placeholderAllowed := func() bool {
		return d.updater.Config().Behavior.ShowPlaceholderPresence
	}

	// reconcile picks the presence mode from the current connection and pause
	// state. It runs on every poll tick and on any pause-flag change.
	reconcile := func() {
		// Pause is a runtime flag: while set, hold presence cleared the same
		// way Valorant-not-running does, and skip the rest of the decision.
		if d.paused.Load() {
			stopPlaceholder()
			if mode != modeCleared {
				d.updater.ClearPresence()
				mode = modeCleared
			}
			return
		}

		// Discord can connect after the Riot Client already has; resend the
		// current mode's presence instead of waiting for a state change.
		nowConnected := d.discord.Connected()
		if nowConnected && !discordConnected {
			switch mode {
			case modeConnected:
				d.updater.ImmediateUpdate(d.state.Get())
			case modePlaceholder:
				sendPlaceholder()
			}
		}
		discordConnected = nowConnected

		// Valorant is up but Discord isn't reachable yet: log it once per
		// edge, not on every poll tick, so it isn't spammy.
		if d.riot.GameRunning() && !nowConnected {
			if !waitingForDiscordLogged {
				d.logger.Info().Msg("Valorant is running, waiting for Discord to be reachable")
				waitingForDiscordLogged = true
			}
		} else {
			waitingForDiscordLogged = false
		}

		// A stalled connection is a connection with nothing behind it. Fall
		// through to the placeholder rather than show a state we never read.
		stalled := d.riot.PresenceStalled()
		if stalled && !stallLogged {
			d.logger.Warn().Msg("Connected to the Riot Client but no Valorant presence has arrived; presence is unknown")
			stallLogged = true
		}
		if !stalled {
			stallLogged = false
		}

		switch {
		case d.riot.Connected() && !stalled:
			st := d.state.Get()
			if mode != modeConnected {
				stopPlaceholder()
				// Skip while Discord isn't connected yet; the reconnect handler above resends once it is.
				if d.discord.Connected() {
					d.updater.ImmediateUpdate(st)
				}
				mode = modeConnected
			}
			fireLookup(st)

		case d.riot.GameRunning() && placeholderAllowed():
			if mode != modePlaceholder {
				mode = modePlaceholder
				placeholderStart = time.Now().Unix()
				sendPlaceholder()
				placeholderTicker = time.NewTicker(d.placeholderInterval)
				placeholderC = placeholderTicker.C
			}

		default:
			if mode != modeCleared {
				stopPlaceholder()
				d.updater.ClearPresence()
				mode = modeCleared
			}
		}

		// Losing the connection drops the agent with it: the next match has
		// to resolve its own rather than inherit the last one.
		if mode != modeConnected {
			agentResolved = false
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case st, ok := <-d.state.Updates():
			if !ok {
				return
			}
			if mode == modeConnected {
				d.updater.DelayUpdate(st)
				fireLookup(st)
			}

		case <-cfgUpdates:
			// Display settings may have changed; reflect them now instead of
			// waiting for the next real state change or poll tick.
			if mode == modeConnected {
				d.updater.ImmediateUpdate(d.state.Get())
			}

		case <-placeholderC:
			sendPlaceholder()

		case <-d.pauseSignal:
			reconcile()

		case <-ticker.C:
			reconcile()
		}
	}
}
