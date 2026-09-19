package riotclient

import (
	"bytes"
	"encoding/json"
	"slices"
	"sync"

	"github.com/rs/zerolog"
)

// WAMP opcodes the Riot Client speaks. Only these two are ever sent or read.
const (
	opcodeSubscribe = 5
	opcodeEvent     = 8
)

// Event is one inbound `[8, name, {eventType, uri, data}]` frame. Data stays
// raw so this package carries no product-specific types.
type Event struct {
	Name      string
	EventType string
	URI       string
	Data      json.RawMessage
}

// Handler receives events for one subscription. Handlers run in order on the
// listener goroutine, so a slow handler delays every later event.
type Handler func(Event)

// maxLoggedFrame caps how much of a dropped frame reaches the log.
const maxLoggedFrame = 512

// parseFrame decodes an inbound frame, reporting ok=false for anything that
// is not a well-formed event. Nothing here type-asserts, so nothing panics.
func parseFrame(raw []byte) (Event, bool) {
	var frame []json.RawMessage
	if err := json.Unmarshal(raw, &frame); err != nil || len(frame) < 3 {
		return Event{}, false
	}

	var opcode int
	if err := json.Unmarshal(frame[0], &opcode); err != nil || opcode != opcodeEvent {
		return Event{}, false
	}

	var name string
	if err := json.Unmarshal(frame[1], &name); err != nil || name == "" {
		return Event{}, false
	}

	// A JSON null unmarshals into a struct without error, so the payload has
	// to be confirmed as an object before it counts as an event.
	if !bytes.HasPrefix(bytes.TrimSpace(frame[2]), []byte("{")) {
		return Event{}, false
	}

	var payload struct {
		EventType string          `json:"eventType"`
		URI       string          `json:"uri"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(frame[2], &payload); err != nil {
		return Event{}, false
	}

	return Event{Name: name, EventType: payload.EventType, URI: payload.URI, Data: payload.Data}, true
}

// registry holds the subscriptions, which outlive any single connection and
// are replayed after every reconnect.
type registry struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	order    []string
}

// add registers h, reporting whether name had no handlers before.
func (r *registry) add(name string, h Handler) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.handlers == nil {
		r.handlers = make(map[string][]Handler)
	}
	_, seen := r.handlers[name]
	if !seen {
		r.order = append(r.order, name)
	}
	r.handlers[name] = append(r.handlers[name], h)
	return !seen
}

// names returns the subscribed event names in registration order.
func (r *registry) names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return append([]string(nil), r.order...)
}

// removeLast undoes the most recent add for name. A failed subscribe must
// leave nothing behind, or the retry becomes a silent no-op.
func (r *registry) removeLast(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	handlers := r.handlers[name]
	switch len(handlers) {
	case 0:
		return
	case 1:
		delete(r.handlers, name)
		r.order = slices.DeleteFunc(r.order, func(n string) bool { return n == name })
	default:
		r.handlers[name] = handlers[:len(handlers)-1]
	}
}

func (r *registry) handlersFor(name string) []Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return append([]Handler(nil), r.handlers[name]...)
}

// dispatch routes one raw frame to its handlers. A malformed frame is logged
// and dropped; it never reaches a handler and never kills the listener.
func (r *registry) dispatch(raw []byte, logger zerolog.Logger) {
	// The Riot Client acknowledges a subscribe with an empty frame. That is
	// normal traffic, not something worth logging.
	if len(bytes.TrimSpace(raw)) == 0 {
		return
	}

	event, ok := parseFrame(raw)
	if !ok {
		logger.Debug().Str("frame", truncate(raw, maxLoggedFrame)).Msg("Dropped a malformed websocket frame")
		return
	}

	for _, h := range r.handlersFor(event.Name) {
		h(event)
	}
}

func truncate(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
