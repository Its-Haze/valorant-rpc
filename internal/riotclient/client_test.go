package riotclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const presenceEvent = "OnJsonApiEvent_chat_v4_presences"

func TestConnectDeliversSubscribedEvents(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	events := make(chan Event, 4)
	if err := client.Subscribe(presenceEvent, func(e Event) { events <- e }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	conn := riot.nextSocket()
	if name := riot.nextSubscribe(); name != presenceEvent {
		t.Errorf("subscribed to %q, want %q", name, presenceEvent)
	}

	riot.push(conn, presenceFrame)

	select {
	case event := <-events:
		if event.Name != presenceEvent || event.EventType != "Update" {
			t.Errorf("got %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the pushed event never reached the handler")
	}
}

// A malformed frame must be dropped without taking the listener down, so a
// good frame behind it still arrives.
func TestMalformedFramesDoNotKillTheListener(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	events := make(chan Event, 4)
	if err := client.Subscribe(presenceEvent, func(e Event) { events <- e }); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	conn := riot.nextSocket()

	for _, frame := range []string{"{{{", "", "[]", "[8,42,{}]", `[8,"` + presenceEvent + `",null]`} {
		riot.push(conn, frame)
	}
	riot.push(conn, presenceFrame)

	select {
	case event := <-events:
		if event.Name != presenceEvent {
			t.Errorf("got %+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the listener died on a malformed frame")
	}
	if !client.IsConnected() {
		t.Error("IsConnected() = false after malformed frames")
	}
}

func TestConnectFailsWhenTheAPIRejectsTheCredentials(t *testing.T) {
	riot := newFakeRiot(t)
	riot.refuseAuth()
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err == nil {
		t.Fatal("Connect succeeded against an API that rejects the credentials")
	}
	if client.IsConnected() {
		t.Error("IsConnected() = true after a failed Connect")
	}
}

// A crashed client leaves the lockfile behind, so nothing must connect on
// credentials whose port answers nothing at all.
func TestConnectFailsOnAStaleLockfile(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)
	riot.server.Close()

	if err := client.Connect(); err == nil {
		t.Fatal("Connect succeeded against a dead port")
	}
}

func TestGetBeforeConnectReportsNotConnected(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	_, err := client.Get(context.Background(), healthEndpoint)
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("got %v, want ErrNotConnected", err)
	}
}

func TestGetSendsBasicAuthAgainstTheLocalAPI(t *testing.T) {
	const token = `{"subject":"11111111-2222-3333-4444-555555555555"}`

	riot := newFakeRiot(t)
	riot.route("/entitlements/v1/token", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, token)
	})
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	resp, err := client.Get(context.Background(), "/entitlements/v1/token")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: the auth header did not survive", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != token {
		t.Errorf("body = %s", body)
	}
}

func TestIsConnectedFlipsFalseWhenTheSocketDrops(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !client.IsConnected() {
		t.Fatal("IsConnected() = false right after Connect")
	}

	conn := riot.nextSocket()
	_ = conn.Close(websocket.StatusGoingAway, "client exited")

	waitFor(t, "IsConnected() to go false", func() bool { return !client.IsConnected() })
}

func TestCredentialsReportTheLivePort(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if _, ok := client.Credentials(); ok {
		t.Error("Credentials() reported credentials before Connect")
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	creds, ok := client.Credentials()
	if !ok {
		t.Fatal("Credentials() reported none after Connect")
	}
	if creds.Port != riot.port {
		t.Errorf("Port = %d, want %d", creds.Port, riot.port)
	}

	_ = client.Disconnect()
	if _, ok := client.Credentials(); ok {
		t.Error("Credentials() survived Disconnect")
	}
}

func TestSubscribeAfterConnectSendsTheFrame(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	riot.nextSocket()

	if err := client.Subscribe(presenceEvent, func(Event) {}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if name := riot.nextSubscribe(); name != presenceEvent {
		t.Errorf("subscribed to %q", name)
	}
}

// Subscribing a second handler to a live name must not send another frame:
// the Riot Client would then deliver every event twice.
func TestSecondHandlerForOneNameSendsNoExtraFrame(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, nil)

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	riot.nextSocket()

	for range 2 {
		if err := client.Subscribe(presenceEvent, func(Event) {}); err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
	}
	if name := riot.nextSubscribe(); name != presenceEvent {
		t.Fatalf("subscribed to %q", name)
	}

	select {
	case extra := <-riot.subscribes:
		t.Errorf("a second subscribe frame was sent for %q", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSubscribeRejectsUnusableRegistrations(t *testing.T) {
	client := New(Options{Logger: discardLogger()})

	if err := client.Subscribe("", func(Event) {}); err == nil {
		t.Error("Subscribe accepted an empty event name")
	}
	if err := client.Subscribe(presenceEvent, nil); err == nil {
		t.Error("Subscribe accepted a nil handler")
	}
}

func TestDiscoveryFallsBackToTheProcessTable(t *testing.T) {
	riot := newFakeRiot(t)
	client := newTestClient(t, riot, func(o *Options) {
		// An empty path would default back to the real %LOCALAPPDATA%
		// lockfile and connect the test to a running Riot Client.
		o.LockfilePath = filepath.Join(t.TempDir(), "absent-lockfile")
		o.Lister = fakeLister{procs: []ProcessInfo{{PID: 4242, Cmdline: riot.cmdline()}}}
	})

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	creds, _ := client.Credentials()
	if creds.Port != riot.port {
		t.Errorf("Port = %d, want %d from the command line", creds.Port, riot.port)
	}
}

// The Riot Client writes the lockfile non-atomically, so discovery has to
// wait the partial write out rather than fail on it.
func TestDiscoveryWaitsOutAPartiallyWrittenLockfile(t *testing.T) {
	riot := newFakeRiot(t)
	path := riot.lockfile()
	complete, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, complete[:len(complete)/2], 0o600); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(t, riot, func(o *Options) {
		o.LockfilePath = path
		o.DiscoveryInterval = 5 * time.Millisecond
		o.DiscoveryAttempts = 200
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(path, complete, 0o600)
	}()

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect never waited out the partial lockfile: %v", err)
	}
}

func TestDisconnectIsSafeWithoutAConnection(t *testing.T) {
	client := New(Options{Logger: discardLogger()})

	if err := client.Disconnect(); err != nil {
		t.Errorf("Disconnect: %v", err)
	}
	if err := client.Disconnect(); err != nil {
		t.Errorf("second Disconnect: %v", err)
	}
}
