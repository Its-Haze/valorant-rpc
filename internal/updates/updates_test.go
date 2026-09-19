package updates

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	wupdater "github.com/wailsapp/wails/v3/pkg/updater"
)

// fakeEngine is a scripted stand-in for *wupdater.Updater. Everything is
// guarded: a check that finds a release starts autoDownload on its own
// goroutine, which then drives this engine while the test is still asserting.
type fakeEngine struct {
	mu         sync.Mutex
	rel        *wupdater.Release
	checkErr   error
	dlErr      error
	checkCalls int
	dlCalls    int
	restarts   int
}

func (f *fakeEngine) Check(context.Context) (*wupdater.Release, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.checkCalls++
	return f.rel, f.checkErr
}

func (f *fakeEngine) DownloadAndInstall(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.dlCalls++
	return f.dlErr
}

func (f *fakeEngine) Restart(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.restarts++
	return nil
}

// failCheck makes every later Check report err and no release.
func (f *fakeEngine) failCheck(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.rel, f.checkErr = nil, err
}

func (f *fakeEngine) checks() int    { return f.count(&f.checkCalls) }
func (f *fakeEngine) downloads() int { return f.count(&f.dlCalls) }
func (f *fakeEngine) restartCount() int {
	return f.count(&f.restarts)
}

func (f *fakeEngine) count(n *int) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return *n
}

// statusRecorder captures every onChange call in order. Check fires one and
// the auto-download behind it fires more, so a bare variable both races and
// loses the one the test cares about.
type statusRecorder struct {
	mu   sync.Mutex
	seen []Status
}

func (r *statusRecorder) record(s Status) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seen = append(r.seen, s)
}

// first is the callback Check itself made, before any download started.
func (r *statusRecorder) first(t *testing.T) Status {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.seen) == 0 {
		t.Fatal("onChange never fired")
	}
	return r.seen[0]
}

func newTestCoordinator(eng engine) *Coordinator {
	return New(eng, doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("no network in test")
	}), false, zerolog.Nop())
}

func TestCheck_NoUpdate(t *testing.T) {
	c := newTestCoordinator(&fakeEngine{rel: nil})

	rec := &statusRecorder{}
	c.OnChange(rec.record)

	s, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if s.Available {
		t.Fatalf("Available = true for a nil release, want false")
	}
	if rec.first(t).Available {
		t.Fatal("onChange saw Available = true, want false")
	}
}

func TestCheck_UpdateAvailable(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "2.0.0", Notes: "big changes"}}
	c := newTestCoordinator(eng)

	rec := &statusRecorder{}
	c.OnChange(rec.record)

	s, err := c.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !s.Available || s.Version != "2.0.0" || s.Notes != "big changes" {
		t.Fatalf("status = %+v, want available 2.0.0 with notes", s)
	}
	if got := rec.first(t); got != s {
		t.Fatalf("onChange status %+v != returned %+v", got, s)
	}

	// Only the release identity is stable here. Check starts the
	// auto-download before returning, and that flips Downloading then Ready
	// underneath; TestCheck_TriggersAutoDownloadInTheBackground owns those.
	held := c.Status()
	if !held.Available || held.Version != s.Version || held.Notes != s.Notes {
		t.Fatalf("Status() %+v does not hold the release Check found, %+v", held, s)
	}
}

func TestCheck_ErrorKeepsPriorAvailability(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "2.0.0"}}
	c := newTestCoordinator(eng)

	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("first Check: %v", err)
	}

	eng.failCheck(errors.New("network down"))
	s, err := c.Check(context.Background())
	if err == nil {
		t.Fatal("expected the check error to propagate")
	}
	if !s.Available || s.Version != "2.0.0" {
		t.Fatalf("status = %+v, want the prior available release retained", s)
	}
	if s.LastError == "" {
		t.Fatal("LastError not recorded")
	}
}

func TestDownload_NoPendingRelease(t *testing.T) {
	c := newTestCoordinator(&fakeEngine{rel: nil})
	err := c.Download(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no update available") {
		t.Fatalf("Download err = %v, want 'no update available'", err)
	}
}

func TestDownload_HappyPathDoesNotRestart(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "2.0.0"}}
	c := newTestCoordinator(eng)

	if err := c.Download(context.Background()); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if eng.downloads() != 1 {
		t.Fatalf("DownloadAndInstall called %d times, want 1", eng.downloads())
	}
	if eng.restartCount() != 0 {
		t.Fatal("Download must not restart; that is a separate confirmed step")
	}

	if err := c.Restart(context.Background()); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if eng.restartCount() != 1 {
		t.Fatalf("Restart called %d times, want 1", eng.restartCount())
	}
}

func TestDevBuild_ChecksAndActionsDisabled(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "9.9.9"}}
	c := New(eng, nil, true, zerolog.Nop())

	// Run returns at once and never touches the engine.
	done := make(chan struct{})
	go func() { c.Run(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return promptly for a dev build")
	}

	s, err := c.Check(context.Background())
	if err != nil || s.Available {
		t.Fatalf("dev Check = (%+v, %v), want empty status and no error", s, err)
	}
	if err := c.Download(context.Background()); err == nil {
		t.Fatal("dev Download should refuse")
	}
	if eng.checks() != 0 || eng.downloads() != 0 {
		t.Fatal("dev build must not call the engine at all")
	}
}

func TestRun_LaunchCheckThenStops(t *testing.T) {
	eng := &fakeEngine{rel: nil}
	c := newTestCoordinator(eng)
	c.interval = time.Hour // keep the ticker out of the way

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	// The launch check should land quickly.
	deadline := time.After(2 * time.Second)
	for eng.checks() == 0 {
		select {
		case <-deadline:
			t.Fatal("no launch check within 2s")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop on context cancel")
	}
}

// waitFor polls cond every 5ms until it's true or the 2s deadline passes,
// for asserting on state a background goroutine (autoDownload) will reach.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for !cond() {
		select {
		case <-deadline:
			t.Fatal(msg)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestCheck_TriggersAutoDownloadInTheBackground(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "2.0.0"}}
	c := newTestCoordinator(eng)

	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}

	waitFor(t, func() bool { return eng.downloads() == 1 }, "auto-download never ran")

	s := c.Status()
	if !s.Ready {
		t.Fatalf("status = %+v, want Ready after a successful auto-download", s)
	}
}

func TestAutoDownload_SkipsOnceAlreadyReady(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "2.0.0"}}
	c := newTestCoordinator(eng)

	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("first Check: %v", err)
	}
	waitFor(t, func() bool { return eng.downloads() == 1 }, "auto-download never ran")
	waitFor(t, func() bool { return c.Status().Ready }, "never reached Ready")

	// A later check (e.g. the next periodic tick) must not download again.
	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("second Check: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if eng.downloads() != 1 {
		t.Fatalf("DownloadAndInstall called %d times, want 1 once already Ready", eng.downloads())
	}
}

func TestAutoDownload_FirstFailureStaysSilent(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "2.0.0"}, dlErr: errors.New("network blip")}
	c := newTestCoordinator(eng)

	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	waitFor(t, func() bool { return eng.downloads() == 1 }, "auto-download never ran")
	waitFor(t, func() bool { return !c.Status().Downloading }, "download never settled")

	if s := c.Status(); s.LastError != "" {
		t.Fatalf("LastError = %q after one failure, want empty (stays silent)", s.LastError)
	}
}

func TestAutoDownload_SecondConsecutiveFailureSurfaces(t *testing.T) {
	eng := &fakeEngine{rel: &wupdater.Release{Version: "2.0.0"}, dlErr: errors.New("network blip")}
	c := newTestCoordinator(eng)

	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("first Check: %v", err)
	}
	waitFor(t, func() bool { return eng.downloads() == 1 }, "first auto-download never ran")
	waitFor(t, func() bool { return !c.Status().Downloading }, "first download never settled")

	if _, err := c.Check(context.Background()); err != nil {
		t.Fatalf("second Check: %v", err)
	}
	waitFor(t, func() bool { return eng.downloads() == 2 }, "second auto-download never ran")

	waitFor(t, func() bool { return c.Status().LastError != "" }, "LastError never surfaced after two failures")
}

// doerFunc adapts a function to HTTPDoer.
type doerFunc func(*http.Request) (*http.Response, error)

func (d doerFunc) Do(req *http.Request) (*http.Response, error) { return d(req) }

// jsonResponse is a helper for changelog tests.
func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
