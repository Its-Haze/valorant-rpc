package content

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/its-haze/valorant-rpc/pkg/types"
	"time"

	"github.com/rs/zerolog"
)

func discardLogger() zerolog.Logger { return zerolog.New(io.Discard) }

// fakeAPI serves the committed fixtures and can be told to fail one endpoint.
type fakeAPI struct {
	t *testing.T

	mu       sync.Mutex
	requests []string

	// failOn is a path substring; status 0 means fail the transport itself
	// rather than answer with an HTTP error.
	failOn string
	status int
}

func newFakeAPI(t *testing.T) *fakeAPI { return &fakeAPI{t: t} }

func (f *fakeAPI) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := req.URL.Path
	f.requests = append(f.requests, path+"?"+req.URL.RawQuery)

	if f.failOn != "" && strings.Contains(path, f.failOn) {
		if f.status == 0 {
			return nil, errors.New("fake transport failure")
		}
		return &http.Response{StatusCode: f.status, Body: io.NopCloser(strings.NewReader(""))}, nil
	}

	var name string
	switch {
	case strings.HasSuffix(path, "/agents"):
		name = "agents.json"
	case strings.HasSuffix(path, "/maps"):
		name = "maps.json"
	case strings.HasSuffix(path, "/competitivetiers"):
		name = "competitivetiers.json"
	case strings.HasSuffix(path, "/gamemodes"):
		name = "gamemodes.json"
	default:
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(readFixture(f.t, name))),
	}, nil
}

// failTransport makes requests for paths containing substr error out before
// any response, the way a dropped connection does.
func (f *fakeAPI) failTransport(substr string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOn, f.status = substr, 0
}

// failStatus answers those requests with an HTTP error instead.
func (f *fakeAPI) failStatus(substr string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failOn, f.status = substr, status
}

func (f *fakeAPI) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func newTestCache(api *fakeAPI) *Cache {
	return New(Options{Doer: api, Logger: discardLogger()})
}

func TestSnapshotBeforeAnyFetchIsEmptyNotNil(t *testing.T) {
	cache := newTestCache(newFakeAPI(t))

	cat := cache.Snapshot()
	if cat == nil {
		t.Fatal("Snapshot returned nil")
	}
	if !cat.Empty() {
		t.Error("the catalogue is populated before any fetch")
	}
}

func TestRefreshPopulatesEveryTable(t *testing.T) {
	cache := newTestCache(newFakeAPI(t))

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	cat := cache.Snapshot()
	if cat.Empty() {
		t.Fatal("the catalogue is empty after a successful refresh")
	}
	if _, ok := cat.Agent(jettUUID, types.DefaultLocale); !ok {
		t.Error("agents did not load")
	}
	if _, ok := cat.Map(ascentURL, types.DefaultLocale); !ok {
		t.Error("maps did not load")
	}
	if _, ok := cat.Tier(RadiantTier, types.DefaultLocale); !ok {
		t.Error("tiers did not load")
	}
	if _, ok := cat.GameMode(bombMode, types.DefaultLocale); !ok {
		t.Error("game modes did not load")
	}
}

// One fetch per endpoint, each asking for every locale so switching language
// never costs a round trip.
func TestRefreshRequestsEveryLocaleOnce(t *testing.T) {
	api := newFakeAPI(t)
	cache := newTestCache(api)

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	seen := api.seen()
	if len(seen) != 4 {
		t.Fatalf("made %d requests, want 4: %v", len(seen), seen)
	}
	for _, req := range seen {
		if !strings.Contains(req, "language=all") {
			t.Errorf("%q does not ask for every locale", req)
		}
	}
	if !strings.Contains(strings.Join(seen, " "), "isPlayableCharacter=true") {
		t.Errorf("the agent request does not filter to playable characters: %v", seen)
	}
}

func TestATransportFailureKeepsThePreviousCatalogue(t *testing.T) {
	api := newFakeAPI(t)
	cache := newTestCache(api)

	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("first Refresh: %v", err)
	}

	api.failTransport("/maps")
	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("a failing refresh reported success")
	}

	cat := cache.Snapshot()
	if _, ok := cat.Agent(jettUUID, types.DefaultLocale); !ok {
		t.Error("the previous catalogue was dropped on a failed refresh")
	}
	if _, ok := cat.Map(ascentURL, types.DefaultLocale); !ok {
		t.Error("maps were emptied by a failed refresh of the maps endpoint")
	}
}

func TestANonOKStatusIsAFailure(t *testing.T) {
	api := newFakeAPI(t)
	api.failStatus("/competitivetiers", http.StatusServiceUnavailable)
	cache := newTestCache(api)

	if err := cache.Refresh(context.Background()); err == nil {
		t.Fatal("a 503 reported success")
	}
	if !cache.Snapshot().Empty() {
		t.Error("a failed first refresh left a partial catalogue")
	}
}

func TestRunRefreshesOnTheIntervalAndStopsWithTheContext(t *testing.T) {
	api := newFakeAPI(t)
	cache := New(Options{Doer: api, Logger: discardLogger(), Interval: 10 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		cache.Run(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for {
		if len(api.seen()) >= 8 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("only %d requests after 2s; Run is not refreshing", len(api.seen()))
		case <-time.After(time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return when its context was cancelled")
	}
}
