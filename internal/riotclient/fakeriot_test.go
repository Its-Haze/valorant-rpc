package riotclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeRiot is a stand-in for the Riot Client's local API: a loopback TLS
// server with the same basic auth, health endpoint and WAMP socket.
type fakeRiot struct {
	t        *testing.T
	server   *httptest.Server
	password string
	port     int

	// sockets receives every accepted websocket, so a test can push frames
	// down the exact connection the client is reading.
	sockets chan *websocket.Conn
	// subscribes receives every event name the client subscribes to.
	subscribes chan string

	mu         sync.Mutex
	routes     map[string]http.HandlerFunc
	rejectAuth bool
}

func newFakeRiot(t *testing.T) *fakeRiot {
	t.Helper()

	f := &fakeRiot{
		t:          t,
		password:   "YyaTUtjvBvJQzZ1H0fUXPw",
		sockets:    make(chan *websocket.Conn, 8),
		subscribes: make(chan string, 16),
		routes:     map[string]http.HandlerFunc{},
	}
	f.server = httptest.NewTLSServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)

	parsed, err := url.Parse(f.server.URL)
	if err != nil {
		t.Fatalf("parsing the fake server URL: %v", err)
	}
	f.port, err = strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("parsing the fake server port: %v", err)
	}
	return f
}

func (f *fakeRiot) route(path string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.routes[path] = h
}

func (f *fakeRiot) refuseAuth() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.rejectAuth = true
}

func (f *fakeRiot) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	reject := f.rejectAuth
	route := f.routes[r.URL.Path]
	f.mu.Unlock()

	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:"+f.password))
	if reject || r.Header.Get("Authorization") != want {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		f.serveSocket(w, r)
		return
	}
	if route != nil {
		route(w, r)
		return
	}
	if r.URL.Path == healthEndpoint {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"country":"gb","locale":"en-GB"}`)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func (f *fakeRiot) serveSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:       []string{"wamp"},
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	select {
	case f.sockets <- conn:
	default:
	}

	// The handler goroutine owns the socket for its lifetime, echoing every
	// subscription back to the test.
	for {
		_, raw, err := conn.Read(context.Background())
		if err != nil {
			return
		}

		var frame []json.RawMessage
		if err := json.Unmarshal(raw, &frame); err != nil || len(frame) < 2 {
			continue
		}
		var name string
		if err := json.Unmarshal(frame[1], &name); err != nil {
			continue
		}
		select {
		case f.subscribes <- name:
		default:
		}
	}
}

// lockfile writes a lockfile pointing at the fake server, the way the real
// Riot Client publishes its port and password.
func (f *fakeRiot) lockfile() string {
	f.t.Helper()

	path := filepath.Join(f.t.TempDir(), "lockfile")
	line := fmt.Sprintf("Riot Client:4242:%d:%s:https", f.port, f.password)
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		f.t.Fatalf("writing the fake lockfile: %v", err)
	}
	return path
}

func (f *fakeRiot) cmdline() string {
	return fmt.Sprintf(`"C:\Riot Games\Riot Client\RiotClientServices.exe" --app-port=%d --remoting-auth-token=%s`,
		f.port, f.password)
}

// nextSocket returns the connection the client most recently opened.
func (f *fakeRiot) nextSocket() *websocket.Conn {
	f.t.Helper()

	select {
	case conn := <-f.sockets:
		return conn
	case <-time.After(5 * time.Second):
		f.t.Fatal("the client never opened a websocket")
		return nil
	}
}

func (f *fakeRiot) nextSubscribe() string {
	f.t.Helper()

	select {
	case name := <-f.subscribes:
		return name
	case <-time.After(5 * time.Second):
		f.t.Fatal("the client never sent a subscribe frame")
		return ""
	}
}

func (f *fakeRiot) push(conn *websocket.Conn, frame string) {
	f.t.Helper()

	if err := conn.Write(context.Background(), websocket.MessageText, []byte(frame)); err != nil {
		f.t.Fatalf("pushing a frame: %v", err)
	}
}

// newTestClient points a Client at the fake, with timings short enough that
// retries and backoff finish inside a test.
func newTestClient(t *testing.T, f *fakeRiot, mutate func(*Options)) *Client {
	t.Helper()

	opts := Options{
		Logger:            discardLogger(),
		LockfilePath:      f.lockfile(),
		Lister:            fakeLister{err: errNoRiotClient},
		DiscoveryInterval: time.Millisecond,
		DiscoveryAttempts: 3,
		RequestTimeout:    5 * time.Second,
		MinBackoff:        time.Millisecond,
		MaxBackoff:        10 * time.Millisecond,
	}
	if mutate != nil {
		mutate(&opts)
	}

	client := New(opts)
	t.Cleanup(func() { _ = client.Disconnect() })
	return client
}

// waitFor polls until cond holds, failing the test if it never does.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
