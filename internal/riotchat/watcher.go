package riotchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/riotclient"
)

// maxBodySize caps a local API response. The presence list carries every
// friend, so it is generous, but it is not unbounded.
const maxBodySize = 8 << 20

// Transport is the part of riotclient.Client this package uses. It reads and
// subscribes; there is no write method to grow into.
type Transport interface {
	Get(ctx context.Context, path string) (*http.Response, error)
	Subscribe(eventName string, handler riotclient.Handler) error
}

// Session is the local player's chat identity, read once per connection.
type Session struct {
	PUUID    string
	GameName string
	Tagline  string
}

// Watcher turns presence events into normalized Presence values. onUpdate is
// called from the websocket listener goroutine and from Start, so it must
// neither block nor assume a goroutine.
type Watcher struct {
	transport Transport
	logger    zerolog.Logger
	onUpdate  func(Presence)

	mu          sync.RWMutex
	session     Session
	haveSession bool
	subscribed  bool
	sawEvent    bool
}

// NewWatcher builds a Watcher. Nothing is fetched until Start.
func NewWatcher(transport Transport, logger zerolog.Logger, onUpdate func(Presence)) *Watcher {
	return &Watcher{transport: transport, logger: logger, onUpdate: onUpdate}
}

// Start resolves who we are, subscribes and emits the presence already
// published. Call it once per connection: the subscription itself survives
// reconnects, so a second call only refreshes the session and the snapshot.
func (w *Watcher) Start(ctx context.Context) error {
	session, err := w.fetchSession(ctx)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.session, w.haveSession = session, true
	w.sawEvent = false
	needsSubscribe := !w.subscribed
	w.mu.Unlock()

	w.logger.Info().Str("riot_id", session.GameName+"#"+session.Tagline).Msg("Reading Valorant presence")

	// Subscribing first means a change during the snapshot fetch arrives as
	// an event instead of falling into the gap between the two.
	if needsSubscribe {
		if err := w.transport.Subscribe(PresenceEvent, w.handleEvent); err != nil {
			return fmt.Errorf("riotchat: subscribing to presence: %w", err)
		}
		w.mu.Lock()
		w.subscribed = true
		w.mu.Unlock()
	}

	// An event only fires on the next change, so the presence already
	// published would otherwise stay invisible until the player moves.
	if err := w.fetchCurrent(ctx); err != nil {
		w.logger.Warn().Err(err).Msg("Could not read the current presence")
	}
	return nil
}

// Session returns the chat identity Start resolved.
func (w *Watcher) Session() (Session, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.session, w.haveSession
}

func (w *Watcher) puuid() string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.session.PUUID
}

func (w *Watcher) handleEvent(event riotclient.Event) {
	presence, err := Decode(event.Data, w.puuid(), w.logger)
	switch {
	case errors.Is(err, ErrNoPresence):
		// Every friend's update arrives here too. Routine, not a problem.
		w.logger.Debug().Msg("Presence event carried nothing of ours")
		return
	case err != nil:
		w.logger.Warn().Err(err).Msg("Could not decode a presence event")
		return
	}

	w.mu.Lock()
	w.sawEvent = true
	w.mu.Unlock()

	w.onUpdate(presence)
}

func (w *Watcher) fetchCurrent(ctx context.Context) error {
	body, err := w.read(ctx, presencesEndpoint)
	if err != nil {
		return err
	}

	presence, err := Decode(body, w.puuid(), w.logger)
	if errors.Is(err, ErrNoPresence) {
		w.logger.Debug().Msg("No Valorant presence published yet")
		return nil
	}
	if err != nil {
		return err
	}

	// An event that landed while this was in flight is newer than the
	// snapshot, so emitting it now would put stale state back on Discord.
	w.mu.RLock()
	stale := w.sawEvent
	w.mu.RUnlock()
	if stale {
		w.logger.Debug().Msg("Dropped the presence snapshot, an event overtook it")
		return nil
	}

	w.onUpdate(presence)
	return nil
}

func (w *Watcher) fetchSession(ctx context.Context) (Session, error) {
	body, err := w.read(ctx, sessionEndpoint)
	if err != nil {
		return Session{}, err
	}

	var payload struct {
		PUUID    string `json:"puuid"`
		GameName string `json:"game_name"`
		GameTag  string `json:"game_tag"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Session{}, fmt.Errorf("riotchat: decoding the chat session: %w", err)
	}
	if payload.PUUID == "" {
		return Session{}, errors.New("riotchat: the chat session carries no puuid")
	}

	return Session{PUUID: payload.PUUID, GameName: payload.GameName, Tagline: payload.GameTag}, nil
}

func (w *Watcher) read(ctx context.Context, path string) ([]byte, error) {
	resp, err := w.transport.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("riotchat: %s answered %d", path, resp.StatusCode)
	}

	// One byte past the cap, so an oversized body reports itself instead of
	// arriving truncated and failing later as malformed JSON.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize+1))
	if err != nil {
		return nil, fmt.Errorf("riotchat: reading %s: %w", path, err)
	}
	if len(body) > maxBodySize {
		return nil, fmt.Errorf("riotchat: %s returned more than %d bytes", path, maxBodySize)
	}
	return body, nil
}
