package riotchat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/its-haze/valorant-rpc/internal/riotclient"
)

const sessionBody = `{"federated":false,"game_name":"Haze","game_tag":"EUW","loaded":true,` +
	`"puuid":"11111111-2222-3333-4444-555555555555","region":"eu","state":"connected"}`

// fakeTransport stands in for riotclient.Client: canned GET bodies and a
// captured subscription the test drives by hand.
type fakeTransport struct {
	mu       sync.Mutex
	bodies   map[string]string
	statuses map[string]int
	paths    []string
	ops      []string

	// onGet fires before a path answers, for racing an event against the
	// snapshot fetch.
	onGet map[string]func()

	handlers map[string]riotclient.Handler
	subs     int
	subErr   error
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		bodies:   map[string]string{sessionEndpoint: sessionBody, presencesEndpoint: `{"presences":[]}`},
		statuses: map[string]int{},
		onGet:    map[string]func(){},
		handlers: map[string]riotclient.Handler{},
	}
}

func (f *fakeTransport) Get(_ context.Context, path string) (*http.Response, error) {
	f.mu.Lock()
	hook := f.onGet[path]
	f.mu.Unlock()
	if hook != nil {
		hook()
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.paths = append(f.paths, path)
	f.ops = append(f.ops, "get "+path)
	body, ok := f.bodies[path]
	if !ok {
		return nil, fmt.Errorf("fakeTransport: no canned body for %s", path)
	}
	status := http.StatusOK
	if s, ok := f.statuses[path]; ok {
		status = s
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func (f *fakeTransport) Subscribe(name string, handler riotclient.Handler) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.subErr != nil {
		return f.subErr
	}
	f.subs++
	f.ops = append(f.ops, "subscribe "+name)
	f.handlers[name] = handler
	return nil
}

func (f *fakeTransport) emit(t *testing.T, data []byte) {
	t.Helper()

	f.mu.Lock()
	handler, ok := f.handlers[PresenceEvent]
	f.mu.Unlock()
	if !ok {
		t.Fatal("nothing subscribed to the presence event")
	}
	handler(riotclient.Event{Name: PresenceEvent, EventType: "Update", URI: presencesEndpoint, Data: data})
}

// collector records what the watcher emits.
type collector struct {
	mu   sync.Mutex
	seen []Presence
}

func (c *collector) add(p Presence) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.seen = append(c.seen, p)
}

func (c *collector) all() []Presence {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]Presence(nil), c.seen...)
}

func TestWatcherEmitsAPresencePerEvent(t *testing.T) {
	transport := newFakeTransport()
	seen := &collector{}

	watcher := NewWatcher(transport, discardLogger(), seen.add)
	if err := watcher.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	transport.emit(t, envelope(t, valorantEntry(t, "private_flat.json")))
	transport.emit(t, envelope(t, valorantEntry(t, "private_nested.json")))

	got := seen.all()
	if len(got) != 2 {
		t.Fatalf("emitted %d presences, want 2", len(got))
	}
	for _, p := range got {
		if p != wantFlatPresence() {
			t.Errorf("presence mismatch: %+v", p)
		}
	}
}

func TestWatcherEmitsTheCurrentPresenceOnStart(t *testing.T) {
	transport := newFakeTransport()
	transport.bodies[presencesEndpoint] = string(envelope(t, valorantEntry(t, "private_flat.json")))
	seen := &collector{}

	watcher := NewWatcher(transport, discardLogger(), seen.add)
	if err := watcher.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := seen.all(); len(got) != 1 || got[0] != wantFlatPresence() {
		t.Fatalf("start did not emit the current presence: %+v", got)
	}
}

func TestWatcherStartsWithoutAPresenceYet(t *testing.T) {
	transport := newFakeTransport()
	seen := &collector{}

	watcher := NewWatcher(transport, discardLogger(), seen.add)
	if err := watcher.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := seen.all(); len(got) != 0 {
		t.Fatalf("emitted %d presences with none published", len(got))
	}

	session, ok := watcher.Session()
	if !ok || session.PUUID != selfPUUID || session.GameName != "Haze" || session.Tagline != "EUW" {
		t.Fatalf("session = %+v, ok = %v", session, ok)
	}
}

func TestWatcherNeedsTheChatSession(t *testing.T) {
	transport := newFakeTransport()
	transport.statuses[sessionEndpoint] = http.StatusNotFound

	watcher := NewWatcher(transport, discardLogger(), func(Presence) {})
	if err := watcher.Start(context.Background()); err == nil {
		t.Fatal("Start succeeded without a chat session")
	}
}

func TestWatcherReportsAFailedSubscription(t *testing.T) {
	transport := newFakeTransport()
	transport.subErr = errors.New("socket is gone")

	watcher := NewWatcher(transport, discardLogger(), func(Presence) {})
	if err := watcher.Start(context.Background()); err == nil {
		t.Fatal("Start hid a failed subscription")
	}
}

// A friend's update is a presence event too. It must not reach the handler
// and must not look like an error.
func TestWatcherSkipsEventsWithoutOurPresence(t *testing.T) {
	transport := newFakeTransport()
	seen := &collector{}

	watcher := NewWatcher(transport, discardLogger(), seen.add)
	if err := watcher.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	stranger := valorantEntry(t, "private_flat.json")
	stranger.PUUID = "99999999-9999-9999-9999-999999999999"
	transport.emit(t, envelope(t, stranger))
	transport.emit(t, []byte(`{"presences":[]}`))
	transport.emit(t, []byte(`garbage`))

	if got := seen.all(); len(got) != 0 {
		t.Fatalf("emitted %d presences that were not ours", len(got))
	}
}

// Subscribing after the snapshot would lose any change that lands between
// the two, and the player would sit on a stale presence until the next one.
func TestWatcherSubscribesBeforeFetchingTheSnapshot(t *testing.T) {
	transport := newFakeTransport()
	watcher := NewWatcher(transport, discardLogger(), func(Presence) {})

	if err := watcher.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	want := []string{"get " + sessionEndpoint, "subscribe " + PresenceEvent, "get " + presencesEndpoint}
	if got := transport.ops; !slices.Equal(got, want) {
		t.Fatalf("ops = %v, want %v", got, want)
	}
}

// riotclient replays subscriptions across reconnects, so a second Start must
// not register a second handler or every event is delivered twice.
func TestWatcherSubscribesOnceAcrossReconnects(t *testing.T) {
	transport := newFakeTransport()
	seen := &collector{}
	watcher := NewWatcher(transport, discardLogger(), seen.add)

	for range 3 {
		if err := watcher.Start(context.Background()); err != nil {
			t.Fatalf("Start: %v", err)
		}
	}

	if transport.subs != 1 {
		t.Fatalf("subscribed %d times, want 1", transport.subs)
	}

	transport.emit(t, envelope(t, valorantEntry(t, "private_flat.json")))
	if got := seen.all(); len(got) != 1 {
		t.Fatalf("one event emitted %d presences", len(got))
	}
}

// The snapshot is older than any event that overtakes it. Emitting it anyway
// would put the previous phase back on Discord.
func TestWatcherDropsASnapshotAnEventOvertook(t *testing.T) {
	transport := newFakeTransport()
	transport.bodies[presencesEndpoint] = string(envelope(t, valorantEntry(t, "private_flat.json")))
	seen := &collector{}
	watcher := NewWatcher(transport, discardLogger(), seen.add)

	idle := valorantEntry(t, "private_flat.json")
	idle.Private = base64.StdEncoding.EncodeToString([]byte(`{"sessionLoopState":"MENUS","isIdle":true}`))
	transport.onGet[presencesEndpoint] = func() { transport.emit(t, envelope(t, idle)) }

	if err := watcher.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := seen.all()
	if len(got) != 1 {
		t.Fatalf("emitted %d presences, want only the event", len(got))
	}
	if !got[0].IsIdle {
		t.Errorf("the snapshot overwrote the newer event: %+v", got[0])
	}
}

func TestWatcherRejectsAnOversizedBody(t *testing.T) {
	transport := newFakeTransport()
	transport.bodies[sessionEndpoint] = strings.Repeat("a", maxBodySize+1)

	watcher := NewWatcher(transport, discardLogger(), func(Presence) {})
	err := watcher.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "more than") {
		t.Fatalf("err = %v, want an oversized-body error", err)
	}
}
