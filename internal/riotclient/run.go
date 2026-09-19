package riotclient

import (
	"context"
	"time"
)

// stableConnection is how long a connection must last before its backoff
// counts as spent and resets.
const stableConnection = 10 * time.Second

// Run keeps the connection up until ctx is canceled, reconnecting with
// exponential backoff whenever the socket drops or the Riot Client exits.
func (c *Client) Run(ctx context.Context) error {
	c.setBaseContext(ctx)
	// The base context outlives Run, so leaving a canceled one behind would
	// make every later Connect fail on a dead context.
	defer c.setBaseContext(context.Background())
	defer func() { _ = c.Disconnect() }()

	backoff := c.opts.MinBackoff
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := c.Connect(); err != nil {
			c.logger.Debug().Err(err).Dur("retry_in", backoff).Msg("Could not reach the Riot Client")
			if !sleep(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff, c.opts.MaxBackoff)
			continue
		}

		connectedAt := time.Now()
		c.waitClosed(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		c.logger.Info().Msg("Lost the Riot Client connection, reconnecting")
		_ = c.Disconnect()

		// A socket accepted and dropped straight away has to escalate too,
		// or Run spins at the minimum interval for as long as that lasts.
		if time.Since(connectedAt) >= stableConnection {
			backoff = c.opts.MinBackoff
		} else {
			backoff = nextBackoff(backoff, c.opts.MaxBackoff)
		}
		if !sleep(ctx, backoff) {
			return ctx.Err()
		}
	}
}

// waitClosed blocks until the current connection's listener exits.
func (c *Client) waitClosed(ctx context.Context) {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed == nil {
		return
	}
	select {
	case <-closed:
	case <-ctx.Done():
	}
}

func nextBackoff(current, limit time.Duration) time.Duration {
	if next := current * 2; next < limit {
		return next
	}
	return limit
}
