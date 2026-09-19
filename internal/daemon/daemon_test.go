package daemon

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/internal/discord"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

type fakeRunner struct {
	started   atomic.Bool
	stopped   atomic.Bool
	connected atomic.Bool
}

func (r *fakeRunner) Run(ctx context.Context) {
	r.started.Store(true)
	<-ctx.Done()
	r.stopped.Store(true)
}

func (r *fakeRunner) Connected() bool { return r.connected.Load() }

// fakeRiotRunner has directly controllable connection, game and stall
// reporting, skipping a real Supervisor's retry/gating behavior.
type fakeRiotRunner struct {
	fakeRunner
	connected atomic.Bool
	gameUp    atomic.Bool
	stalled   atomic.Bool
}

func (r *fakeRiotRunner) Connected() bool       { return r.connected.Load() }
func (r *fakeRiotRunner) GameRunning() bool     { return r.gameUp.Load() }
func (r *fakeRiotRunner) PresenceStalled() bool { return r.stalled.Load() }

// fakePresenceSender stands in for a real Discord IPC connection; UpdatePresence
// fails while connected is false, like discord.Client when Discord is unreachable.
type fakePresenceSender struct {
	mu        sync.Mutex
	sends     []*discord.RPCData
	cleared   int32
	connected atomic.Bool
}

func (f *fakePresenceSender) UpdatePresence(rpcData *discord.RPCData) error {
	if !f.connected.Load() {
		return errors.New("not connected to Discord RPC")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sends = append(f.sends, rpcData.Copy())
	return nil
}

func (f *fakePresenceSender) ClearPresence() error {
	atomic.AddInt32(&f.cleared, 1)
	return nil
}

func (f *fakePresenceSender) IsConnected() bool { return f.connected.Load() }

func (f *fakePresenceSender) lastSend() *discord.RPCData {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sends) == 0 {
		return nil
	}
	return f.sends[len(f.sends)-1]
}

func (f *fakePresenceSender) sendCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sends)
}

func (f *fakePresenceSender) clearCount() int32 {
	return atomic.LoadInt32(&f.cleared)
}

// fakeAgentLookup records every call, so a test can assert the v0.2 seam
// fires once per entry into agent select or a match.
type fakeAgentLookup struct {
	mu       sync.Mutex
	contexts []types.PresenceContext
}

func (f *fakeAgentLookup) Lookup(ctx context.Context, st *state.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contexts = append(f.contexts, st.PhaseContext())
}

func (f *fakeAgentLookup) seen() []types.PresenceContext {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]types.PresenceContext(nil), f.contexts...)
}

// newTestDaemonDeps builds an Updater over a fake sender and a fresh state
// manager. cfg is applied to the store the Updater reads.
func newTestDaemonDeps(cfg *config.Config) (*discord.Updater, *state.Manager, *fakePresenceSender) {
	logger := zerolog.Nop()
	sender := &fakePresenceSender{}
	sender.connected.Store(true) // Discord connected by default; tests that care override it.
	updater := discord.NewUpdater(sender, config.NewStore(cfg), logger)
	return updater, state.NewManager(logger), sender
}

func defaultTestConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Advanced.UpdateInterval = config.MinUpdateInterval
	return cfg
}

func newTestDaemon(t *testing.T, cfg *config.Config, opts ...DaemonOption) (*Daemon, *fakeRunner, *fakeRiotRunner, *state.Manager, *fakePresenceSender) {
	t.Helper()
	updater, stateMgr, sender := newTestDaemonDeps(cfg)
	discordRunner := &fakeRunner{}
	riotRunner := &fakeRiotRunner{}
	d := New(discordRunner, riotRunner, updater, stateMgr, zerolog.Nop(), testPollInterval, testPollInterval, opts...)
	return d, discordRunner, riotRunner, stateMgr, sender
}

func TestDaemon_RunStartsBothSupervisorsAndBlocksUntilCanceled(t *testing.T) {
	d, discordRunner, riotRunner, _, _ := newTestDaemon(t, defaultTestConfig())

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()

	waitFor(t, testTimeout, func() bool {
		return discordRunner.started.Load() && riotRunner.started.Load()
	})

	select {
	case <-done:
		t.Fatal("Run returned before ctx was canceled")
	default:
	}

	cancel()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("Run did not return after ctx cancellation")
	}

	if !discordRunner.stopped.Load() || !riotRunner.stopped.Load() {
		t.Fatal("expected both supervisors to observe ctx cancellation")
	}
}

func TestDaemon_RunNeverExitsWhileBothSupervisorsFailRepeatedly(t *testing.T) {
	discordSup := NewSupervisor(&fakeConnector{connectFailures: 1 << 30}, testRetryInterval, testPollInterval)
	riotSup := NewRiotSupervisor(&fakeConnector{connectFailures: 1 << 30}, newFakeProcessChecker(), testRetryInterval, testPollInterval, testPollInterval)
	updater, stateMgr, _ := newTestDaemonDeps(defaultTestConfig())
	d := New(discordSup, riotSup, updater, stateMgr, zerolog.Nop(), testPollInterval, testPollInterval)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * testRetryInterval)
	select {
	case <-done:
		t.Fatal("Run returned despite both supervisors failing to connect")
	default:
	}

	cancel()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("Run did not return after ctx cancellation")
	}
}

func TestDaemon_OneSupervisorFailingDoesNotBlockTheOther(t *testing.T) {
	stuck := NewSupervisor(&fakeConnector{connectFailures: 1 << 30}, testRetryInterval, testPollInterval)

	checker := newFakeProcessChecker()
	checker.setValorantRunning(true)
	var healthyConnected atomic.Bool
	healthy := NewRiotSupervisor(&fakeConnector{}, checker, testRetryInterval, testPollInterval, testPollInterval,
		WithOnConnect(func() { healthyConnected.Store(true) }))

	updater, stateMgr, _ := newTestDaemonDeps(defaultTestConfig())
	d := New(stuck, healthy, updater, stateMgr, zerolog.Nop(), testPollInterval, testPollInterval)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	waitFor(t, testTimeout, healthyConnected.Load)
}

func TestDaemon_RealPresenceOnceBothConnected(t *testing.T) {
	d, discordRunner, riotRunner, _, sender := newTestDaemon(t, defaultTestConfig())
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.connected.Store(true)

	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.Details == "In the client"
	})
}

func TestDaemon_PresenceFollowsStateChangesOnceConnected(t *testing.T) {
	d, discordRunner, riotRunner, stateMgr, sender := newTestDaemon(t, defaultTestConfig())
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > 0 })

	stateMgr.Apply(func(st *state.State) { st.SessionLoopState = types.SessionLoopInGame })

	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.State == "In a match"
	})
}

func TestDaemon_ResendsPresenceOnConfigChangeWhileConnected(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	sender := &fakePresenceSender{}
	sender.connected.Store(true)
	store := config.NewStore(defaultTestConfig())
	updater := discord.NewUpdater(sender, store, zerolog.Nop())
	stateMgr := state.NewManager(zerolog.Nop())

	discordRunner := &fakeRunner{}
	discordRunner.connected.Store(true)
	riotRunner := &fakeRiotRunner{}
	d := New(discordRunner, riotRunner, updater, stateMgr, zerolog.Nop(), testPollInterval, testPollInterval)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > 0 })

	// A config change alone, no state change, must trigger an immediate resend.
	before := sender.sendCount()
	next := *defaultTestConfig()
	next.Display.Default.ShowRank = !next.Display.Default.ShowRank
	if err := store.Apply(next); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	waitFor(t, testTimeout, func() bool { return sender.sendCount() > before })
}

func TestDaemon_ClearsPresenceWhenDisconnectedAndValorantNotRunning(t *testing.T) {
	d, discordRunner, riotRunner, _, sender := newTestDaemon(t, defaultTestConfig())
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	waitFor(t, testTimeout, func() bool { return sender.clearCount() > 0 })

	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > 0 })

	before := sender.clearCount()
	riotRunner.connected.Store(false)
	waitFor(t, testTimeout, func() bool { return sender.clearCount() > before })
}

func TestDaemon_NoPlaceholderWhileTheSettingIsOff(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Behavior.ShowPlaceholderPresence = false
	d, discordRunner, riotRunner, _, sender := newTestDaemon(t, cfg)
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	// Valorant is up but nothing has connected, which is placeholder territory.
	riotRunner.gameUp.Store(true)
	time.Sleep(10 * testPollInterval)

	if sender.sendCount() != 0 {
		t.Fatalf("expected no placeholder sends with the setting off, got %d", sender.sendCount())
	}
	waitFor(t, testTimeout, func() bool { return sender.clearCount() > 0 })
}

func TestDaemon_ShowsRotatingPlaceholderWhenTheSettingIsOn(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Behavior.ShowPlaceholderPresence = true
	d, discordRunner, riotRunner, _, sender := newTestDaemon(t, cfg)
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.gameUp.Store(true)

	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.Details == "Launching VALORANT..."
	})

	// More than one send while still launching, so the rotation is running.
	waitFor(t, testTimeout, func() bool { return sender.sendCount() >= 2 })

	// The instant a presence is readable, real presence takes over.
	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.Details != "Launching VALORANT..."
	})
}

func TestDaemon_StalledConnectionFallsOutOfRealPresence(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Behavior.ShowPlaceholderPresence = true
	d, discordRunner, riotRunner, _, sender := newTestDaemon(t, cfg)
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.gameUp.Store(true)
	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.Details == "In the client"
	})

	// Nothing ever arrived on this connection: show the placeholder rather
	// than a state the daemon never actually read.
	riotRunner.stalled.Store(true)
	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.Details == "Launching VALORANT..."
	})

	if !d.PresenceStalled() {
		t.Fatal("PresenceStalled should report the stall to the GUI too")
	}

	riotRunner.stalled.Store(false)
	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.Details == "In the client"
	})
}

func TestDaemon_StalledConnectionClearsWhenThePlaceholderIsOff(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Behavior.ShowPlaceholderPresence = false
	d, discordRunner, riotRunner, _, sender := newTestDaemon(t, cfg)
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.gameUp.Store(true)
	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > 0 })

	before := sender.clearCount()
	riotRunner.stalled.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.clearCount() > before })
}

func TestDaemon_PauseClearsPresenceAndUnpauseResumes(t *testing.T) {
	d, discordRunner, riotRunner, stateMgr, sender := newTestDaemon(t, defaultTestConfig())
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	if d.IsPaused() {
		t.Fatal("a fresh Daemon must start unpaused")
	}

	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > 0 })

	before := sender.clearCount()
	d.SetPaused(true)
	waitFor(t, testTimeout, func() bool { return sender.clearCount() > before })

	sendsWhilePaused := sender.sendCount()
	stateMgr.Apply(func(st *state.State) { st.SessionLoopState = types.SessionLoopInGame })
	time.Sleep(10 * testPollInterval)
	if sender.sendCount() != sendsWhilePaused {
		t.Fatalf("presence sent while paused: before=%d after=%d", sendsWhilePaused, sender.sendCount())
	}

	resumeBefore := sender.sendCount()
	d.SetPaused(false)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > resumeBefore })
}

func TestDaemon_AgentLookupFiresOncePerEntryIntoAMatch(t *testing.T) {
	lookup := &fakeAgentLookup{}
	d, discordRunner, riotRunner, stateMgr, sender := newTestDaemon(t, defaultTestConfig(), WithAgentLookup(lookup))
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > 0 })

	// Sitting in the client is not a lookup.
	if got := lookup.seen(); len(got) != 0 {
		t.Fatalf("expected no lookups in the client, got %v", got)
	}

	stateMgr.Apply(func(st *state.State) { st.SessionLoopState = types.SessionLoopPregame })
	waitFor(t, testTimeout, func() bool { return len(lookup.seen()) == 1 })

	// Another change inside agent select must not fire it again.
	stateMgr.Apply(func(st *state.State) { st.PartySize = 3 })
	time.Sleep(10 * testPollInterval)
	if got := lookup.seen(); len(got) != 1 {
		t.Fatalf("expected one lookup while still in agent select, got %v", got)
	}

	stateMgr.Apply(func(st *state.State) { st.SessionLoopState = types.SessionLoopInGame })
	waitFor(t, testTimeout, func() bool { return len(lookup.seen()) == 2 })

	want := []types.PresenceContext{types.ContextAgentSelect, types.ContextInMatch}
	got := lookup.seen()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lookup contexts = %v, want %v", got, want)
		}
	}
}

func TestDaemon_NoLookupWiredIsNotAPanic(t *testing.T) {
	d, discordRunner, riotRunner, stateMgr, sender := newTestDaemon(t, defaultTestConfig())
	discordRunner.connected.Store(true)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.connected.Store(true)
	waitFor(t, testTimeout, func() bool { return sender.sendCount() > 0 })

	stateMgr.Apply(func(st *state.State) { st.SessionLoopState = types.SessionLoopPregame })
	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.State == "Agent select"
	})
}

// countingWriter counts Write calls, standing in for a log sink so tests can
// assert on log volume without parsing log lines.
type countingWriter struct {
	n atomic.Int32
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n.Add(1)
	return len(p), nil
}

func TestDaemon_LogsOnceWhileWaitingForDiscordThenAgainOnNextEdge(t *testing.T) {
	updater, stateMgr, _ := newTestDaemonDeps(defaultTestConfig())
	writer := &countingWriter{}
	discordRunner := &fakeRunner{} // starts disconnected
	riotRunner := &fakeRiotRunner{}
	d := New(discordRunner, riotRunner, updater, stateMgr, zerolog.New(writer), testPollInterval, testPollInterval)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.gameUp.Store(true) // Valorant up, Discord still unreachable
	waitFor(t, testTimeout, func() bool { return writer.n.Load() >= 1 })

	time.Sleep(10 * testPollInterval)
	if got := writer.n.Load(); got != 1 {
		t.Fatalf("expected exactly one log line while waiting, got %d", got)
	}

	discordRunner.connected.Store(true) // Discord connects; no new log on this edge
	time.Sleep(10 * testPollInterval)
	if got := writer.n.Load(); got != 1 {
		t.Fatalf("expected no additional log line once Discord connects, got %d", got)
	}

	discordRunner.connected.Store(false) // Discord drops again while Valorant still up
	waitFor(t, testTimeout, func() bool { return writer.n.Load() >= 2 })
}

func TestDaemon_ResendsPresenceOnceDiscordConnectsAfterTheRiotClient(t *testing.T) {
	d, discordRunner, riotRunner, _, sender := newTestDaemon(t, defaultTestConfig())
	sender.connected.Store(false) // Discord not reachable yet, like the real Client

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	riotRunner.connected.Store(true)
	time.Sleep(10 * testPollInterval)
	if sender.sendCount() != 0 {
		t.Fatalf("expected no successful sends while Discord isn't connected, got %d", sender.sendCount())
	}

	sender.connected.Store(true)
	discordRunner.connected.Store(true)

	waitFor(t, testTimeout, func() bool {
		last := sender.lastSend()
		return last != nil && last.Details == "In the client"
	})
}

// The content catalogue is optional, so Run has to both start it when it is
// wired and wait for it before returning.
func TestDaemon_RunStartsAndWaitsForTheCatalogue(t *testing.T) {
	catalogue := &fakeRunner{}
	d, _, _, _, _ := newTestDaemon(t, defaultTestConfig(), WithCatalogue(catalogue))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()

	waitFor(t, testTimeout, func() bool { return catalogue.started.Load() })

	cancel()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("Run did not return after cancel")
	}
	if !catalogue.stopped.Load() {
		t.Error("Run returned before the catalogue loop finished")
	}
}

func TestDaemon_RunWithoutACatalogueStillStarts(t *testing.T) {
	d, discordRunner, _, _, _ := newTestDaemon(t, defaultTestConfig())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go d.Run(ctx)

	waitFor(t, testTimeout, func() bool { return discordRunner.started.Load() })
}
