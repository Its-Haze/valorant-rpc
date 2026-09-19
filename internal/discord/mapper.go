package discord

import (
	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// contextLabels is the stand-in text for each context until ticket 07 lands
// the real builders, the token tables and the content catalogue.
var contextLabels = map[types.PresenceContext]string{
	types.ContextInClient:    "In the client",
	types.ContextInQueue:     "In queue",
	types.ContextCustomGame:  "Custom game",
	types.ContextAgentSelect: "Agent select",
	types.ContextInMatch:     "In a match",
}

// MapStateToPresence routes state to the builder for its context. Ticket 07
// replaces the single stand-in builder with five real ones.
func MapStateToPresence(st *state.State, cfg *config.Config) *RPCData {
	if st == nil {
		return &RPCData{}
	}
	return buildStandInPresence(st)
}

// buildStandInPresence renders the context name and nothing else, so the
// daemon has something transmittable to reconcile against before ticket 07.
func buildStandInPresence(st *state.State) *RPCData {
	label, ok := contextLabels[st.PhaseContext()]
	if !ok {
		label = string(st.PhaseContext())
	}
	return &RPCData{Details: "VALORANT", State: label}
}

// ShouldClearPresence reports whether presence should be cleared rather than
// updated, which today is only the show-in-client toggle turned off.
func ShouldClearPresence(st *state.State, cfg *config.Config) bool {
	return !cfg.Presence.ShowInClient && st.PhaseContext() == types.ContextInClient
}
