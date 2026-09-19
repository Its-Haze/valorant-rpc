package content

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	// baseURL is valorant-api.com's v1 root.
	baseURL = "https://valorant-api.com/v1"

	// DefaultInterval is how often the catalogue refetches. Agents and maps
	// change on patch days, so this only has to beat a user's session length.
	DefaultInterval = 6 * time.Hour

	requestTimeout = 10 * time.Second

	// maxBodyBytes caps a response read with generous headroom over the
	// agent payload, which is the largest of the four. Exceeding it errors.
	maxBodyBytes = 64 << 20
)

// HTTPDoer is anything that can execute an *http.Request, satisfied by
// *http.Client. Injected so tests never reach valorant-api.com.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Options configures a Cache. Every field has a working default.
type Options struct {
	Doer     HTTPDoer
	Logger   zerolog.Logger
	Interval time.Duration
}

// Cache holds the most recent good Catalogue and refetches on an interval.
// A failed refresh keeps what is already there.
type Cache struct {
	doer     HTTPDoer
	logger   zerolog.Logger
	interval time.Duration

	mu  sync.RWMutex
	cat *Catalogue
}

// New builds a Cache. Nothing is fetched until Refresh or Run is called.
func New(opts Options) *Cache {
	doer := opts.Doer
	if doer == nil {
		doer = &http.Client{Timeout: requestTimeout}
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}

	return &Cache{
		doer:     doer,
		logger:   opts.Logger.With().Str("component", "content").Logger(),
		interval: interval,
		cat:      &Catalogue{},
	}
}

// Snapshot is the current catalogue, never nil. Before the first successful
// refresh it is empty and every lookup misses, which callers already handle.
func (c *Cache) Snapshot() *Catalogue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cat
}

// Refresh fetches all four endpoints and swaps in a new catalogue. Every
func (c *Cache) Refresh(ctx context.Context) error {
	agents, err := c.get(ctx, agentsPath)
	if err != nil {
		return c.logFailure(err)
	}
	maps, err := c.get(ctx, mapsPath)
	if err != nil {
		return c.logFailure(err)
	}
	tiers, err := c.get(ctx, tiersPath)
	if err != nil {
		return c.logFailure(err)
	}
	modes, err := c.get(ctx, modesPath)
	if err != nil {
		return c.logFailure(err)
	}

	cat, err := parseCatalogue(agents, maps, tiers, modes)
	if err != nil {
		return c.logFailure(err)
	}

	c.mu.Lock()
	c.cat = cat
	c.mu.Unlock()

	c.logger.Info().
		Int("agents", len(cat.agents)).
		Int("maps", len(cat.maps)).
		Int("tiers", len(cat.tiers)).
		Int("game_modes", len(cat.modes)).
		Msg("content catalogue refreshed")
	return nil
}

// logFailure reports a failed refresh, saying whether anything usable is
// still cached, and hands the error straight back.
func (c *Cache) logFailure(err error) error {
	c.mu.RLock()
	stale := !c.cat.Empty()
	c.mu.RUnlock()

	c.logger.Warn().Err(err).Bool("serving_previous", stale).Msg("content catalogue refresh failed")
	return err
}

// Run refreshes immediately and then on the interval until ctx is done.
func (c *Cache) Run(ctx context.Context) {
	_ = c.Refresh(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.Refresh(ctx)
		}
	}
}

func (c *Cache) get(ctx context.Context, path string) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain the error body so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		return nil, fmt.Errorf("content: %s returned status %d", path, resp.StatusCode)
	}

	// One byte past the cap tells a truncated read from a merely large one.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("content: %s exceeded %d bytes", path, maxBodyBytes)
	}
	return body, nil
}
