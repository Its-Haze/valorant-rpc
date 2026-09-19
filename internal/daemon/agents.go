package daemon

import (
	"context"
	"errors"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/content"
	"github.com/its-haze/valorant-rpc/internal/gamelog"
	"github.com/its-haze/valorant-rpc/internal/state"
)

// CharacterReader reports the agent codename the local player controls.
// *gamelog.Reader satisfies it.
type CharacterReader interface {
	Character() (string, error)
}

// AgentCatalogue resolves a codename to the UUID the presence builders use.
// *content.Cache does not satisfy this directly; GameLogAgents adapts it.
type AgentCatalogue interface {
	Snapshot() *content.Catalogue
}

// GameLogAgents is the AgentLookup backed by Valorant's own log. It needs no
// network call, no token and no request to Riot.
type GameLogAgents struct {
	reader  CharacterReader
	content AgentCatalogue
	state   *state.Manager
	logger  zerolog.Logger

	// missLogged keeps an unresolvable codename to one log line per match
	// rather than one per poll.
	missLogged string
}

// NewGameLogAgents builds the lookup. Every dependency is an interface so the
// daemon tests drive it without a log file.
func NewGameLogAgents(reader CharacterReader, cat AgentCatalogue, stateMgr *state.Manager, logger zerolog.Logger) *GameLogAgents {
	return &GameLogAgents{reader: reader, content: cat, state: stateMgr, logger: logger.With().Str("component", "agents").Logger()}
}

// Lookup reads the log and applies the agent to the state. It is quiet about
func (g *GameLogAgents) Lookup(_ context.Context, _ *state.State) {
	name, err := g.reader.Character()
	if err != nil {
		if !errors.Is(err, gamelog.ErrNoLog) {
			g.logger.Debug().Err(err).Msg("Could not read the game log")
		}
		return
	}
	if name == "" {
		return
	}

	uuid, ok := g.content.Snapshot().AgentUUIDByDeveloperName(name)
	if !ok {
		// Menu and career screens log the same line with a UI class, so this
		// is routine rather than a fault.
		if g.missLogged != name {
			g.missLogged = name
			g.logger.Debug().Str("codename", name).Msg("Game log named something that is not an agent")
		}
		return
	}

	g.missLogged = ""
	g.state.Apply(func(st *state.State) { st.AgentID = uuid })
}
