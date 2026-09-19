package riotclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog"
)

// ErrNotConnected reports a request made before Connect succeeded.
var ErrNotConnected = errors.New("riotclient: not connected to the Riot Client")

// healthEndpoint answers as soon as the Riot Client is up, with no product
// launched. Gating on a Valorant endpoint would mistake the client for dead.
const healthEndpoint = "/riotclient/region-locale"

const (
	defaultDiscoveryInterval = 500 * time.Millisecond
	defaultDiscoveryAttempts = 20
	defaultRequestTimeout    = 5 * time.Second
	defaultMinBackoff        = time.Second
	defaultMaxBackoff        = 30 * time.Second

	// A presence frame carries every friend on the list, well past the
	// websocket library's 32 KiB default.
	defaultReadLimit = 4 << 20
)

// Options configures a Client. The zero value points at the real Riot Client
// with sensible timeouts.
type Options struct {
	Logger zerolog.Logger

	// LockfilePath overrides %LOCALAPPDATA%'s lockfile, for tests.
	LockfilePath string
	// Lister overrides the real process table, for tests.
	Lister ProcessLister

	DiscoveryInterval time.Duration
	DiscoveryAttempts int
	RequestTimeout    time.Duration
	ReadLimit         int64
	MinBackoff        time.Duration
	MaxBackoff        time.Duration
}

func (o Options) withDefaults() Options {
	if o.LockfilePath == "" {
		// A missing LOCALAPPDATA leaves the path empty, which discovery then
		// reports as a partial lockfile and retries past into the process scan.
		o.LockfilePath, _ = DefaultLockfilePath()
	}
	if o.Lister == nil {
		o.Lister = gopsutilLister{}
	}
	if o.DiscoveryInterval <= 0 {
		o.DiscoveryInterval = defaultDiscoveryInterval
	}
	if o.DiscoveryAttempts <= 0 {
		o.DiscoveryAttempts = defaultDiscoveryAttempts
	}
	if o.RequestTimeout <= 0 {
		o.RequestTimeout = defaultRequestTimeout
	}
	if o.ReadLimit <= 0 {
		o.ReadLimit = defaultReadLimit
	}
	if o.MinBackoff <= 0 {
		o.MinBackoff = defaultMinBackoff
	}
	if o.MaxBackoff < o.MinBackoff {
		o.MaxBackoff = max(defaultMaxBackoff, o.MinBackoff)
	}
	return o
}

// Client is a connection to the Riot Client's local API. It carries no
// product types: callers decode Event.Data themselves.
type Client struct {
	opts   Options
	logger zerolog.Logger
	http   *http.Client

	// subMu pairs every registry change with the frame that announces it.
	subMu sync.Mutex
	subs  registry

	mu        sync.RWMutex
	base      context.Context
	creds     Credentials
	haveCreds bool
	conn      *websocket.Conn
	cancel    context.CancelFunc
	closed    chan struct{}

	connected atomic.Bool
}

// New builds a Client. Nothing connects until Connect or Run is called.
func New(opts Options) *Client {
	opts = opts.withDefaults()
	return &Client{
		opts:   opts,
		logger: opts.Logger,
		http:   newHTTPClient(),
		base:   context.Background(),
	}
}

// newHTTPClient trusts the Riot Client's self-signed loopback certificate.
// It carries no Timeout: the websocket dial rejects one, so every call sets
// its own context deadline instead.
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// Connect discovers credentials, confirms the local API answers on them and
// opens the event websocket. It satisfies the daemon's Connector interface.
func (c *Client) Connect() error {
	ctx := c.baseContext()

	if c.IsConnected() {
		_ = c.Disconnect()
	}
	// A previous listener has to be gone before a new one starts, or its
	// deferred flag reset lands on the new connection and pins it false.
	c.waitClosed(ctx)

	creds, err := c.discovery().find(ctx)
	if err != nil {
		return err
	}

	conn, err := c.dial(ctx, creds)
	if err != nil {
		return err
	}

	listenCtx, cancel := context.WithCancel(ctx)
	closed := make(chan struct{})

	// subMu covers publishing the socket as well as the replay: a Subscribe
	// seeing the new connection first would send its frame twice.
	c.subMu.Lock()
	c.mu.Lock()
	c.creds, c.haveCreds = creds, true
	c.conn, c.cancel, c.closed = conn, cancel, closed
	c.mu.Unlock()

	err = c.replaySubscriptions(listenCtx, conn)
	c.subMu.Unlock()
	if err != nil {
		cancel()
		_ = conn.Close(websocket.StatusInternalError, "subscribe failed")
		// The listener never started, so this is the only close of the channel.
		close(closed)
		c.clearConnection()
		return err
	}

	c.connected.Store(true)
	go c.listen(listenCtx, conn, closed)

	c.logger.Info().Int("port", creds.Port).Msg("Connected to the Riot Client")
	return nil
}

// Disconnect tears the websocket down. The Client stays reusable.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	conn, cancel := c.conn, c.cancel
	c.conn, c.cancel = nil, nil
	c.creds, c.haveCreds = Credentials{}, false
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
	c.connected.Store(false)
	return nil
}

// clearConnection drops a half-built connection's state. The caller has
// already canceled its context and closed the socket.
func (c *Client) clearConnection() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn, c.cancel = nil, nil
	c.creds, c.haveCreds = Credentials{}, false
}

// IsConnected reports whether the websocket is live. It flips back to false
// on its own when the socket drops, with no polling by the caller.
func (c *Client) IsConnected() bool { return c.connected.Load() }

// Credentials returns the credentials the live connection was made with.
func (c *Client) Credentials() (Credentials, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.creds, c.haveCreds
}

// Get performs a GET against the local API and hands back the response with
// its body unread. GET is the only method exposed: nothing writes to Riot.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	creds, ok := c.Credentials()
	if !ok {
		return nil, ErrNotConnected
	}
	return c.get(ctx, creds, path)
}

func (c *Client) get(ctx context.Context, creds Credentials, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, creds.BaseURL()+path, nil)
	if err != nil {
		return nil, fmt.Errorf("riotclient: building request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", creds.AuthHeader())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("riotclient: requesting %s: %w", path, err)
	}
	return resp, nil
}

func (c *Client) discovery() discovery {
	return discovery{
		lockfilePath: c.opts.LockfilePath,
		lister:       c.opts.Lister,
		healthy:      c.checkHealth,
		interval:     c.opts.DiscoveryInterval,
		attempts:     c.opts.DiscoveryAttempts,
		logger:       c.logger,
	}
}

// checkHealth accepts any answer but an auth rejection: the endpoint proves
// the API is up, and a stale lockfile fails the dial or the auth instead.
func (c *Client) checkHealth(ctx context.Context, creds Credentials) error {
	ctx, cancel := context.WithTimeout(ctx, c.opts.RequestTimeout)
	defer cancel()

	resp, err := c.get(ctx, creds, healthEndpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("riotclient: the local API rejected these credentials with %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) baseContext() context.Context {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.base
}

// setBaseContext scopes every later connection to ctx. Run calls it so the
// context-free Connector interface still shuts down cleanly.
func (c *Client) setBaseContext(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.base = ctx
}
