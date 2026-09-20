// Package daemon implements the Daemon's core lifecycle.
// See ADR-0002.
package daemon

import (
	"context"
	"strings"
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
// the inputs Daemon needs to pick real presence or cleared.
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

// MenuLookup resolves which section of the client is open, which is the only
// way to tell a lobby the player opened from the one Valorant gave them.
type MenuLookup interface {
	Read()
}

// presenceMode is what Daemon currently shows on Discord. See ADR-0002.
type presenceMode int

const (
	modeUnknown presenceMode = iota
	modeConnected
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

// WithMenuLookup wires the menu-screen reader into the presence loop.
func WithMenuLookup(m MenuLookup) DaemonOption {
	return func(d *Daemon) { d.screens = m }
}

// Daemon owns the Discord and Riot Connection Supervisors and drives Discord
// presence off their state. Run is the single seam here. See ADR-0002.
type Daemon struct {
	discord discordRunner
	riot    riotRunner
	updater *discord.Updater
	state   *state.Manager
	agents  AgentLookup
	screens MenuLookup
	logger  zerolog.Logger

	// catalogue is optional: a Daemon without one still drives presence,
	// with every content lookup missing.
	catalogue CatalogueRefresher

	presencePollInterval time.Duration

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
	presencePollInterval time.Duration,
	opts ...DaemonOption,
) *Daemon {
	d := &Daemon{
		discord:              discordSup,
		riot:                 riotSup,
		updater:              updater,
		state:                stateMgr,
		logger:               logger,
		presencePollInterval: presencePollInterval,
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

// presenceLoop shows real presence once a presence has been read, and clears
// presence the rest of the time. See ADR-0002.
func (d *Daemon) presenceLoop(ctx context.Context) {
	ticker := time.NewTicker(d.presencePollInterval)
	defer ticker.Stop()

	cfgUpdates := d.updater.ConfigChanges()

	mode := modeUnknown
	discordConnected := false
	waitingForDiscordLogged := false
	stallLogged := false

	// The menu screen is only read in the menus. In a match the log grows
	// fast and the answer could not change the context anyway.
	fireScreen := func() {
		if d.screens == nil {
			return
		}
		if loop := d.state.Get().SessionLoopState; !isMenus(loop) {
			return
		}
		d.screens.Read()
	}

	// The agent lookup runs only in a match, never in agent select: the log
	// names the agent once its pawn spawns, after the match has started.
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

	// reconcile picks the presence mode from the current connection and pause
	// state. It runs on every poll tick and on any pause-flag change.
	reconcile := func() {
		// Pause is a runtime flag: while set, hold presence cleared the same
		// way Valorant-not-running does, and skip the rest of the decision.
		if d.paused.Load() {
			if mode != modeCleared {
				d.updater.ClearPresence()
				mode = modeCleared
			}
			return
		}

		// Discord can connect after the Riot Client already has; resend the
		// current mode's presence instead of waiting for a state change.
		nowConnected := d.discord.Connected()
		if nowConnected && !discordConnected && mode == modeConnected {
			d.updater.ImmediateUpdate(d.state.Get())
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

		// A stalled connection is a connection with nothing behind it. Clear
		// presence rather than show a state we never read.
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
			// Before the state is read, so entering the client builds its
			// presence from the right half of the menus the first time.
			fireScreen()
			st := d.state.Get()
			if mode != modeConnected {
				// Skip while Discord isn't connected yet; the reconnect handler above resends once it is.
				if d.discord.Connected() {
					d.updater.ImmediateUpdate(st)
				}
				mode = modeConnected
			}
			fireLookup(st)

		default:
			if mode != modeCleared {
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
				fireScreen()
			}

		case <-cfgUpdates:
			// Display settings may have changed; reflect them now instead of
			// waiting for the next real state change or poll tick.
			if mode == modeConnected {
				d.updater.ImmediateUpdate(d.state.Get())
			}

		case <-d.pauseSignal:
			reconcile()

		case <-ticker.C:
			reconcile()
		}
	}
}

// isMenus reports the out-of-game loop state, where an empty value counts:
// a connection that has read nothing yet is not in a match.
func isMenus(loop types.SessionLoopState) bool {
	return loop == "" || strings.EqualFold(string(loop), string(types.SessionLoopMenus))
}
