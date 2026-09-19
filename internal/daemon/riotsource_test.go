package daemon

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/riotchat"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// fakeWatcher records Start calls and can emit presences on demand.
type fakeWatcher struct {
	starts   atomic.Int32
	startErr error
	emit     func(riotchat.Presence)
}

func (w *fakeWatcher) Start(ctx context.Context) error {
	w.starts.Add(1)
	return w.startErr
}

func newTestSource(t *testing.T, connector Connector) (*RiotSource, *state.Manager, *fakeWatcher) {
	t.Helper()
	stateMgr := state.NewManager(zerolog.Nop())
	watcher := &fakeWatcher{}
	source := NewRiotSource(connector, stateMgr, zerolog.Nop(), func(onUpdate func(riotchat.Presence)) presenceWatcher {
		watcher.emit = onUpdate
		return watcher
	})
	return source, stateMgr, watcher
}

func TestRiotSource_ConnectStartsTheWatcher(t *testing.T) {
	connector := &fakeConnector{}
	source, _, watcher := newTestSource(t, connector)

	if err := source.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if watcher.starts.Load() != 1 {
		t.Fatalf("expected the watcher to start once, got %d", watcher.starts.Load())
	}
	if !source.IsConnected() {
		t.Fatal("expected the source to report connected")
	}
}

func TestRiotSource_ConnectTearsDownWhenTheWatcherFails(t *testing.T) {
	connector := &fakeConnector{}
	source, _, watcher := newTestSource(t, connector)
	watcher.startErr = errors.New("no chat session")

	if err := source.Connect(); err == nil {
		t.Fatal("expected Connect to report the watcher's failure")
	}
	if connector.disconnectCalls.Load() == 0 {
		t.Fatal("expected a failed start to disconnect the client again")
	}
}

func TestRiotSource_PresenceLandsOnTheState(t *testing.T) {
	source, stateMgr, watcher := newTestSource(t, &fakeConnector{})
	if err := source.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	watcher.emit(riotchat.Presence{
		GameName:         "Haze",
		Tagline:          "EUW",
		SessionLoopState: "INGAME",
		MatchMap:         "/Game/Maps/Ascent/Ascent",
		QueueID:          "competitive",
		ScoreAllyTeam:    7,
		ScoreEnemyTeam:   5,
		PartySize:        2,
		MaxPartySize:     5,
		CompetitiveTier:  21,
		AccountLevel:     140,
	})

	st := stateMgr.Get()
	if st.RiotID != "Haze" || st.Tagline != "EUW" {
		t.Errorf("riot id = %q#%q, want Haze#EUW", st.RiotID, st.Tagline)
	}
	if got := st.PhaseContext(); got != types.ContextInMatch {
		t.Errorf("context = %q, want %q", got, types.ContextInMatch)
	}
	if st.ScoreAlly != 7 || st.ScoreEnemy != 5 {
		t.Errorf("score = %d-%d, want 7-5", st.ScoreAlly, st.ScoreEnemy)
	}
	if st.CompetitiveTier != 21 || st.AccountLevel != 140 {
		t.Errorf("tier/level = %d/%d, want 21/140", st.CompetitiveTier, st.AccountLevel)
	}
}

func TestRiotSource_IdlePresenceSetsAwayAndClearsAgain(t *testing.T) {
	source, stateMgr, watcher := newTestSource(t, &fakeConnector{})
	if err := source.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	watcher.emit(riotchat.Presence{SessionLoopState: "MENUS", IsIdle: true})
	if got := stateMgr.Get().Availability; got != types.AvailabilityAway {
		t.Errorf("availability = %q, want %q", got, types.AvailabilityAway)
	}

	watcher.emit(riotchat.Presence{SessionLoopState: "MENUS"})
	if got := stateMgr.Get().Availability; got != types.AvailabilityOnline {
		t.Errorf("availability = %q, want %q", got, types.AvailabilityOnline)
	}
}

func TestRiotSource_DisconnectForgetsTheLastPresence(t *testing.T) {
	source, stateMgr, watcher := newTestSource(t, &fakeConnector{})
	if err := source.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	watcher.emit(riotchat.Presence{SessionLoopState: "INGAME", MatchMap: "/Game/Maps/Ascent/Ascent"})
	if err := source.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	st := stateMgr.Get()
	if st.MapID != "" || st.SessionLoopState != "" {
		t.Fatalf("expected a reset state after disconnect, got %+v", st)
	}
}

func TestRiotSource_StallsOnlyWhenNoPresenceEverArrived(t *testing.T) {
	connector := &fakeConnector{}
	source, _, watcher := newTestSource(t, connector)

	var now atomic.Int64
	now.Store(time.Now().UnixNano())
	source.now = func() time.Time { return time.Unix(0, now.Load()) }

	if source.PresenceStalled() {
		t.Fatal("a source that never connected cannot be stalled")
	}

	if err := source.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if source.PresenceStalled() {
		t.Fatal("reported stalled the moment it connected")
	}

	now.Add((presenceStallThreshold - time.Second).Nanoseconds())
	if source.PresenceStalled() {
		t.Fatal("reported stalled before the threshold elapsed")
	}

	now.Add((2 * time.Second).Nanoseconds())
	if !source.PresenceStalled() {
		t.Fatal("expected a stall once the threshold elapsed with nothing read")
	}

	// A presence clears it, and sitting still afterwards must not bring it
	// back: Valorant publishes nothing while a player does nothing.
	watcher.emit(riotchat.Presence{SessionLoopState: "MENUS"})
	now.Add((10 * presenceStallThreshold).Nanoseconds())
	if source.PresenceStalled() {
		t.Fatal("a quiet connection that already read a presence is not stalled")
	}
}

func TestRiotSource_ReconnectRestartsTheStallWindow(t *testing.T) {
	source, _, watcher := newTestSource(t, &fakeConnector{})

	var now atomic.Int64
	now.Store(time.Now().UnixNano())
	source.now = func() time.Time { return time.Unix(0, now.Load()) }

	if err := source.Connect(); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	watcher.emit(riotchat.Presence{SessionLoopState: "MENUS"})

	if err := source.Disconnect(); err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}
	if err := source.Connect(); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}

	now.Add((presenceStallThreshold + time.Second).Nanoseconds())
	if !source.PresenceStalled() {
		t.Fatal("the presence read on the previous connection must not count for this one")
	}
}
