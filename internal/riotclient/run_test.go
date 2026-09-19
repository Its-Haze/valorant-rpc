package riotclient

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// A dropped socket has to bring back every subscription: the Riot Client
// keeps none of them across connections.
func TestRunReconnectsAndReplaysSubscriptions(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	events := make(chan Event, 8)
	if err := client.Subscribe(presenceEvent, func(e Event) { events <- e }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()

	first := riot.nextSocket()
	if name := riot.nextSubscribe(); name != presenceEvent {
		t.Fatalf("first subscribe was %q", name)
	}
	_ = first.Close(websocket.StatusGoingAway, "the Riot Client exited")

	second := riot.nextSocket()
	if name := riot.nextSubscribe(); name != presenceEvent {
		t.Fatalf("the subscription was not replayed, got %q", name)
	}

	// The replayed subscription has to actually deliver, not just be sent.
	riot.push(second, presenceFrame)
	select {
	case event := <-events:
		if event.Name != presenceEvent {
			t.Errorf("got %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no event arrived on the reconnected socket")
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was canceled")
	}
}

// Run is started before the Riot Client is, so it has to keep retrying until
// the lockfile shows up.
func TestRunWaitsForTheRiotClientToAppear(t *testing.T) {
	riot := newFakeRiot(t)
	path := riot.lockfile()
	complete, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(t, riot, func(o *Options) {
		o.LockfilePath = path
		o.DiscoveryAttempts = 1
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = client.Run(ctx) }()

	time.Sleep(20 * time.Millisecond)
	if client.IsConnected() {
		t.Fatal("Run connected with no lockfile on disk")
	}

	if err := os.WriteFile(path, complete, 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "Run to connect once the lockfile appears", client.IsConnected)
}

func TestRunReturnsImmediatelyOnACanceledContext(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := client.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Run returned %v, want context.Canceled", err)
	}
}

func TestNextBackoffDoublesUpToTheLimit(t *testing.T) {
	cases := []struct {
		current, limit, want time.Duration
	}{
		{time.Second, 30 * time.Second, 2 * time.Second},
		{16 * time.Second, 30 * time.Second, 30 * time.Second},
		{30 * time.Second, 30 * time.Second, 30 * time.Second},
	}
	for _, c := range cases {
		if got := nextBackoff(c.current, c.limit); got != c.want {
			t.Errorf("nextBackoff(%s, %s) = %s, want %s", c.current, c.limit, got, c.want)
		}
	}
}

func TestOptionsDefaultsAreUsable(t *testing.T) {
	opts := Options{}.withDefaults()

	if opts.Lister == nil {
		t.Error("Lister defaulted to nil")
	}
	if opts.ReadLimit != defaultReadLimit {
		t.Errorf("ReadLimit = %d, want %d", opts.ReadLimit, defaultReadLimit)
	}
	if opts.MaxBackoff < opts.MinBackoff {
		t.Errorf("MaxBackoff %s is below MinBackoff %s", opts.MaxBackoff, opts.MinBackoff)
	}
}

// A MinBackoff above the default ceiling must not produce a MaxBackoff that
// sits below it, which would make the backoff run backwards.
func TestOptionsKeepMaxBackoffAboveMin(t *testing.T) {
	opts := Options{MinBackoff: time.Minute}.withDefaults()

	if opts.MaxBackoff != time.Minute {
		t.Errorf("MaxBackoff = %s, want %s", opts.MaxBackoff, time.Minute)
	}
}
