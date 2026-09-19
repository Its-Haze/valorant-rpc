package daemon

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/its-haze/valorant-rpc/pkg/constants"
)

// fakeGameDetector lets tests control GameRunning() directly.
type fakeGameDetector struct {
	detected atomic.Bool
}

func (f *fakeGameDetector) GameRunning() bool { return f.detected.Load() }

func TestDiscordSupervisor_DetectsDisconnectWhenDiscordProcessDisappears(t *testing.T) {
	// Mirrors the real discord.Client: IsConnected() never flips back to false on its own.
	connector := &fakeConnector{}
	checker := newFakeProcessChecker()
	checker.set(constants.DiscordProcessName, true)
	game := &fakeGameDetector{}
	game.detected.Store(true)

	var connectCount atomic.Int32
	// Gate mirrors production wiring: Connect() only fires while both are up.
	ds := newDiscordSupervisor(connector, checker, game, testRetryInterval, testPollInterval, testPollInterval,
		WithGate(&discordGate{checker: checker, game: game}),
		WithOnConnect(func() { connectCount.Add(1) }))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go ds.Run(ctx)

	waitFor(t, testTimeout, func() bool { return connectCount.Load() == 1 })
	waitFor(t, testTimeout, ds.Connected)

	if !connector.IsConnected() {
		t.Fatal("underlying connector should still report connected, as the real discord.Client would")
	}

	checker.set(constants.DiscordProcessName, false) // Discord's process is killed; connector.IsConnected() never flips
	waitFor(t, testTimeout, func() bool { return !ds.Connected() })

	checker.set(constants.DiscordProcessName, true) // Discord relaunches
	waitFor(t, testTimeout, func() bool { return connectCount.Load() >= 2 })
	waitFor(t, testTimeout, ds.Connected)
}

func TestDiscordSupervisor_NeverConnectsWhileValorantNotDetected(t *testing.T) {
	connector := &fakeConnector{}
	checker := newFakeProcessChecker()
	checker.set(constants.DiscordProcessName, true)
	game := &fakeGameDetector{} // Valorant not running

	ds := newDiscordSupervisor(connector, checker, game, testRetryInterval, testPollInterval, testPollInterval,
		WithGate(&discordGate{checker: checker, game: game}))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go ds.Run(ctx)

	time.Sleep(10 * testRetryInterval)
	if connector.connectCalls.Load() != 0 {
		t.Fatalf("expected no Connect() attempts while Valorant isn't running, got %d", connector.connectCalls.Load())
	}

	game.detected.Store(true) // Valorant launches
	waitFor(t, testTimeout, ds.Connected)
}

func TestDiscordSupervisor_DisconnectsWhenValorantProcessDisappears(t *testing.T) {
	// This is the actual bug: Discord stayed connected and kept showing a
	// stale presence after Valorant closed, since nothing tied the two together.
	connector := &fakeConnector{}
	checker := newFakeProcessChecker()
	checker.set(constants.DiscordProcessName, true)
	game := &fakeGameDetector{}
	game.detected.Store(true)

	var disconnectCount atomic.Int32
	ds := newDiscordSupervisor(connector, checker, game, testRetryInterval, testPollInterval, testPollInterval,
		WithGate(&discordGate{checker: checker, game: game}),
		WithOnDisconnect(func() { disconnectCount.Add(1) }))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go ds.Run(ctx)

	waitFor(t, testTimeout, ds.Connected)
	if !connector.IsConnected() {
		t.Fatal("underlying connector should still report connected, as the real discord.Client would")
	}

	game.detected.Store(false) // Valorant closes; connector.IsConnected() never flips on its own
	waitFor(t, testTimeout, func() bool { return !ds.Connected() })
	waitFor(t, testTimeout, func() bool { return disconnectCount.Load() >= 1 })
}

func TestDiscordSupervisor_ChecksErrorLeavesLastKnownState(t *testing.T) {
	connector := &fakeConnector{}
	checker := newFakeProcessChecker()
	checker.set(constants.DiscordProcessName, true)
	game := &fakeGameDetector{}
	game.detected.Store(true)

	ds := newDiscordSupervisor(connector, checker, game, testRetryInterval, testPollInterval, testPollInterval)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go ds.Run(ctx)

	waitFor(t, testTimeout, ds.Connected)

	checker.setErr(errors.New("boom"))
	waitFor(t, testTimeout, ds.Connected) // stays connected across transient checker errors
}
