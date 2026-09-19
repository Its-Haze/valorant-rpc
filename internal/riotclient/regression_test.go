package riotclient

import (
	"context"
	"testing"
	"time"
)

// Run scopes connections to its own context, so the Client has to be usable
// again once Run has returned.
func TestRunLeavesTheClientReusable(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = client.Run(ctx)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect after Run: %v", err)
	}
	if !client.IsConnected() {
		t.Error("IsConnected() = false after reconnecting past a finished Run")
	}
}

// The previous listener's deferred flag reset must not land on the new
// connection, which would report a healthy socket as disconnected forever.
func TestReconnectingDoesNotPinIsConnectedFalse(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	for range 3 {
		if err := client.Connect(); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		riot.nextSocket()
	}

	// The old listeners exit asynchronously, so give them room to misbehave.
	time.Sleep(100 * time.Millisecond)
	if !client.IsConnected() {
		t.Error("IsConnected() = false on a live socket after reconnecting")
	}
}

func TestReconnectingReplaysSubscriptionsExactlyOnce(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Subscribe(presenceEvent, func(Event) {}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	for range 2 {
		if err := client.Connect(); err != nil {
			t.Fatalf("Connect: %v", err)
		}
		riot.nextSocket()
		if name := riot.nextSubscribe(); name != presenceEvent {
			t.Fatalf("subscribed to %q", name)
		}
	}

	select {
	case extra := <-riot.subscribes:
		t.Errorf("a duplicate subscribe frame was sent for %q", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

// A failed subscribe must leave no registration behind, or the retry is a
// silent no-op and the next reconnect double-delivers.
func TestRegistryRemoveLastUndoesAdd(t *testing.T) {
	var reg registry

	reg.add(presenceEvent, func(Event) {})
	reg.removeLast(presenceEvent)

	if names := reg.names(); len(names) != 0 {
		t.Errorf("names() = %v, want none", names)
	}
	if handlers := reg.handlersFor(presenceEvent); len(handlers) != 0 {
		t.Errorf("%d handlers survived removeLast", len(handlers))
	}
	if isNew := reg.add(presenceEvent, func(Event) {}); !isNew {
		t.Error("re-adding after removeLast did not report the name as new")
	}
}

func TestRegistryRemoveLastKeepsEarlierHandlers(t *testing.T) {
	var reg registry

	reg.add(presenceEvent, func(Event) {})
	reg.add(presenceEvent, func(Event) {})
	reg.removeLast(presenceEvent)

	if handlers := reg.handlersFor(presenceEvent); len(handlers) != 1 {
		t.Errorf("got %d handlers, want 1", len(handlers))
	}
	if names := reg.names(); len(names) != 1 {
		t.Errorf("names() = %v, want the name to survive", names)
	}
}

func TestRegistryRemoveLastOnAnUnknownNameIsSafe(t *testing.T) {
	var reg registry
	reg.removeLast(presenceEvent)
}

// A half-built connection must not look connected: Get would otherwise hit
// the API on a socket-less port instead of reporting ErrNotConnected.
func TestClearConnectionLeavesNothingUsable(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	client.clearConnection()

	if _, ok := client.Credentials(); ok {
		t.Error("Credentials() survived clearConnection")
	}
	if _, ok := client.currentConn(); ok {
		t.Error("currentConn() survived clearConnection")
	}
}
