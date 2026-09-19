package riotclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

var errNoRiotClient = errors.New("riotclient: no running Riot Client found")

// credentialSource is one way of finding the local API's credentials.
type credentialSource struct {
	name string
	find func() (Credentials, error)
}

// discovery finds credentials, preferring the lockfile and falling back to
// the process table when a non-default install defeats it.
type discovery struct {
	lockfilePath string
	lister       ProcessLister
	healthy      func(context.Context, Credentials) error
	interval     time.Duration
	attempts     int
	logger       zerolog.Logger
}

func (d discovery) find(ctx context.Context) (Credentials, error) {
	sources := []credentialSource{
		{"lockfile", func() (Credentials, error) { return readLockfile(d.lockfilePath) }},
		{"process", func() (Credentials, error) { return credentialsFromProcesses(d.lister) }},
	}

	lastErr := errNoRiotClient
	for attempt := range d.attempts {
		// A lockfile caught mid-write, or a client still starting up, both
		// resolve on their own within a tick or two.
		if attempt > 0 && !sleep(ctx, d.interval) {
			return Credentials{}, ctx.Err()
		}

		for _, source := range sources {
			creds, err := source.find()
			if err != nil {
				lastErr = err
				d.logger.Debug().Err(err).Str("source", source.name).Msg("Riot Client credential lookup failed")
				continue
			}

			// A crashed client leaves a stale lockfile behind, so credentials
			// only count once the API answers on them.
			if err := d.healthy(ctx, creds); err != nil {
				lastErr = err
				d.logger.Debug().Err(err).Str("source", source.name).Int("port", creds.Port).
					Msg("Riot Client credentials failed the health check")
				continue
			}

			d.logger.Debug().Str("source", source.name).Int("port", creds.Port).
				Msg("Found the Riot Client local API")
			return creds, nil
		}
	}

	return Credentials{}, fmt.Errorf("riotclient: gave up after %d discovery attempts: %w", d.attempts, lastErr)
}

// sleep waits for d, reporting false if ctx is canceled first.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
