package daemon

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/its-haze/valorant-rpc/pkg/constants"
)

// fakeProcessChecker answers per process name, so a test can have the Riot
// Client running while Valorant is not.
type fakeProcessChecker struct {
	mu      sync.Mutex
	running map[string]bool
	err     error
}

func newFakeProcessChecker() *fakeProcessChecker {
	return &fakeProcessChecker{running: map[string]bool{}}
}

func (f *fakeProcessChecker) IsRunning(names ...string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return false, f.err
	}
	for _, n := range names {
		if f.running[n] {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeProcessChecker) set(name string, running bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running[name] = running
}

func (f *fakeProcessChecker) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// setValorantRunning flips both halves of the game gate at once.
func (f *fakeProcessChecker) setValorantRunning(running bool) {
	f.set(constants.RiotClientProcessName, running)
	f.set(constants.ValorantProcessName, running)
}

func newRiotSupervisor(connector Connector, checker ProcessChecker, opts ...Option) *RiotSupervisor {
	return NewRiotSupervisor(connector, checker, testRetryInterval, testPollInterval, testPollInterval, opts...)
}

func TestRiotSupervisor_GameRunningNeedsBothProcesses(t *testing.T) {
	connector := &fakeConnector{connectFailures: 1 << 30}
	checker := newFakeProcessChecker()
	s := newRiotSupervisor(connector, checker)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go s.Run(ctx)

	if s.GameRunning() {
		t.Fatal("expected GameRunning() to start false")
	}

	// The launcher on its own is a user browsing the Riot Client, not Valorant.
	checker.set(constants.RiotClientProcessName, true)
	time.Sleep(5 * testPollInterval)
	if s.GameRunning() {
		t.Fatal("the Riot Client alone must not count as Valorant running")
	}

	checker.set(constants.ValorantProcessName, true)
	waitFor(t, testTimeout, s.GameRunning)

	checker.set(constants.ValorantProcessName, false)
	waitFor(t, testTimeout, func() bool { return !s.GameRunning() })
}

func TestRiotSupervisor_NeverConnectsWhileValorantNotRunning(t *testing.T) {
	connector := &fakeConnector{}
	checker := newFakeProcessChecker()
	checker.set(constants.RiotClientProcessName, true)
	s := newRiotSupervisor(connector, checker)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go s.Run(ctx)

	time.Sleep(10 * testRetryInterval)
	if got := connector.connectCalls.Load(); got != 0 {
		t.Fatalf("expected no Connect() attempts while Valorant isn't running, got %d", got)
	}

	checker.set(constants.ValorantProcessName, true)
	waitFor(t, testTimeout, s.Connected)
}

func TestRiotSupervisor_ChecksErrorLeavesLastKnownState(t *testing.T) {
	connector := &fakeConnector{connectFailures: 1 << 30}
	checker := newFakeProcessChecker()
	checker.setValorantRunning(true)
	s := newRiotSupervisor(connector, checker)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go s.Run(ctx)

	waitFor(t, testTimeout, s.GameRunning)

	checker.setErr(errors.New("boom"))
	time.Sleep(5 * testPollInterval)

	if !s.GameRunning() {
		t.Fatal("expected last known state to be preserved when the checker errors")
	}
}

func TestRiotSupervisor_DisconnectsWhenValorantExits(t *testing.T) {
	// The Riot Client outlives the game, so the underlying connection stays
	// up. The supervisor has to notice the game process going away.
	connector := &fakeConnector{}
	checker := newFakeProcessChecker()
	checker.setValorantRunning(true)

	var connectCount atomic.Int32
	s := newRiotSupervisor(connector, checker, WithOnConnect(func() { connectCount.Add(1) }))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go s.Run(ctx)

	waitFor(t, testTimeout, func() bool { return connectCount.Load() == 1 })
	waitFor(t, testTimeout, s.Connected)

	checker.set(constants.ValorantProcessName, false)
	waitFor(t, testTimeout, func() bool { return !s.Connected() })

	checker.set(constants.ValorantProcessName, true)
	waitFor(t, testTimeout, func() bool { return connectCount.Load() >= 2 })
	waitFor(t, testTimeout, s.Connected)
}

func TestRiotSupervisor_StillConnectsAndDisconnectsLikeSupervisor(t *testing.T) {
	connector := &fakeConnector{}
	checker := newFakeProcessChecker()
	checker.setValorantRunning(true)

	var connected atomic.Bool
	s := newRiotSupervisor(connector, checker, WithOnConnect(func() { connected.Store(true) }))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	waitFor(t, testTimeout, connected.Load)

	cancel()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("Run did not return after ctx cancellation")
	}

	if connector.disconnectCalls.Load() == 0 {
		t.Error("expected Disconnect to be called on shutdown")
	}
}

// A connector that cannot report a stall is never stalled, which is what the
// fakes throughout these suites rely on.
func TestRiotSupervisor_PresenceStalledFalseForAPlainConnector(t *testing.T) {
	s := newRiotSupervisor(&fakeConnector{}, newFakeProcessChecker())
	if s.PresenceStalled() {
		t.Fatal("a plain Connector must never report a stall")
	}
}

// alwaysStalledConnector connects fine and always claims a stall, so a test
// can check who the supervisor believes about being connected.
type alwaysStalledConnector struct {
	fakeConnector
}

func (c *alwaysStalledConnector) PresenceStalled() bool { return true }

func TestRiotSupervisor_NotStalledWhileTheGameIsClosed(t *testing.T) {
	checker := newFakeProcessChecker()
	s := newRiotSupervisor(&alwaysStalledConnector{}, checker)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go s.Run(ctx)

	// Nothing is connected yet, so there is no connection to be stalled.
	time.Sleep(5 * testPollInterval)
	if s.PresenceStalled() {
		t.Fatal("reported a stall before anything connected")
	}

	checker.setValorantRunning(true)
	waitFor(t, testTimeout, s.Connected)
	waitFor(t, testTimeout, s.PresenceStalled)

	checker.setValorantRunning(false)
	waitFor(t, testTimeout, func() bool { return !s.Connected() })
	if s.PresenceStalled() {
		t.Fatal("a closed game must not report a stalled connection")
	}
}
